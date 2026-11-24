package cli

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestReadOnlyPOSTRetryOn429(t *testing.T) {
	const body = `{"query":"example","filters":{"priority":["high"]},"workspace_search":true}`
	for _, tc := range []struct {
		name  string
		flag  string
		calls int32
	}{
		{name: "default", calls: 1},
		{name: "explicit-false", flag: "--retry-on-429=false", calls: 1},
		{name: "opt-in", flag: "--retry-on-429", calls: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "retry-key")
			var calls atomic.Int32
			bodies := make(chan string, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				call := calls.Add(1)
				if r.Method != http.MethodPost || r.RequestURI != "/prefix/api/v1/workspaces/team/work-items/advanced-search/" || r.Header.Get("X-Api-Key") != "retry-key" {
					t.Error("retry changed the method, request URI, or credentials")
				}
				raw, err := io.ReadAll(r.Body)
				if err != nil || !strings.Contains(string(raw), `"query":"example"`) {
					t.Errorf("unexpected search body %q: %v", raw, err)
				}
				if call <= 2 {
					bodies <- string(raw)
				}
				if call == 1 {
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `[[{"id":"match"}]]`)
			}))
			defer server.Close()
			args := []string{"work-item", "advanced-search", "--api-version", "v1", "--base-url", server.URL + "/prefix", "--workspace", "team", "--body-file", workItemBodyFile(t, body)}
			if tc.flag != "" {
				args = append(args, tc.flag)
			}
			code, stdout, stderr := invoke(t, workItemCatalog(t), args, workItemHandlers())
			if calls.Load() != tc.calls {
				t.Fatalf("requests=%d; want %d (exit=%d stderr=%s)", calls.Load(), tc.calls, code, stderr)
			}
			if tc.calls == 1 {
				if code != 5 || stdout != "" || !strings.Contains(stderr, `"http_status":429`) || !strings.Contains(stderr, `"retry_after_seconds":0`) {
					t.Fatalf("unexpected default output: exit=%d stdout=%s stderr=%s", code, stdout, stderr)
				}
				return
			}
			if first, second := <-bodies, <-bodies; first != second {
				t.Fatalf("request body changed: first=%q retry=%q", first, second)
			}
			if code != 0 || stderr != "" || !strings.Contains(stdout, `"data":[[{"id":"match"}]]`) {
				t.Fatalf("retry failed: exit=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
		})
	}
}

func TestPaginationRetryOn429(t *testing.T) {
	for _, exhaust := range []bool{false, true} {
		t.Run(fmt.Sprintf("exhaust=%t", exhaust), func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", paginationReviewToken)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				call := calls.Add(1)
				if call > 4 {
					t.Error("pagination made an extra request")
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				wantCursor := ""
				if call > 2 {
					wantCursor = paginationReviewCursor
				}
				if r.Method != http.MethodGet || r.URL.Query().Get("cursor") != wantCursor || r.URL.Query().Get("fields") != "id,name" || r.Header.Get("X-Api-Key") != paginationReviewToken {
					t.Error("pagination retry changed cursor, filters, method, or credentials")
				}
				switch {
				case call == 1 || call == 3 || (call == 4 && exhaust):
					w.Header().Set("Retry-After", "0")
					if call == 4 {
						w.Header().Set("Content-Type", "application/problem+json")
					}
					w.WriteHeader(http.StatusTooManyRequests)
					if call == 4 {
						_, _ = fmt.Fprintf(w, `{"type":"rate_limited","code":"rate_limited","detail":%q}`, paginationReviewToken)
					}
				case call == 2:
					w.Header().Set("Content-Type", "application/json")
					_, _ = fmt.Fprintf(w, `{"data":[{"id":"first-page"}],"pagination":{"style":"cursor"},"has_more":true,"next_cursor":%q}`, paginationReviewCursor)
				default:
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"data":[{"id":"second-page"}],"pagination":{"style":"cursor"},"has_more":false}`)
				}
			}))
			defer server.Close()
			args := append(paginationReviewArgs("v2", server.URL), "--paginate", "cursor", "--order-by", "created_at", "--fields", "id,name", "--max-pages", "2", "--retry-on-429")
			code, stdout, stderr := invoke(t, workItemCatalog(t), args, workItemHandlers())
			if calls.Load() != 4 {
				t.Fatalf("requests=%d; want 4 (exit=%d stderr=%s)", calls.Load(), code, stderr)
			}
			if exhaust {
				failure := paginationReviewFailure(t, code, stdout, stderr, "api_error", 5, 1)
				if failure["http_status"] != float64(429) || failure["upstream_code"] != "rate_limited" || failure["retry_after_seconds"] != float64(0) {
					t.Fatalf("missing final rate-limit diagnostic: %s", stderr)
				}
				return
			}
			if code != 0 || stderr != "" || !strings.Contains(stdout, `"complete":true`) || !strings.Contains(stdout, `"pages":2`) || strings.Count(stdout, `"id":"first-page"`) != 1 || strings.Count(stdout, `"id":"second-page"`) != 1 {
				t.Fatalf("unexpected aggregate: exit=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
		})
	}
}

func TestAssetMetadataRetryOn429(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "retry-key")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.RequestURI != "/api/v2/workspaces/team/assets/"+transferID+"/" {
			t.Errorf("unexpected asset metadata request %s %s", r.Method, r.RequestURI)
		}
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"asset-id","asset_url":"signed-private-url"}`)
	}))
	defer server.Close()
	args := []string{"workspace-asset", "get", "--api-version", "v2", "--base-url", server.URL, "--workspace", "team", "--id", transferID, "--retry-on-429"}
	code, stdout, stderr := invoke(t, workItemCatalog(t), args, contentAssetsHandlers())
	if calls.Load() != 2 || code != 0 || stderr != "" || strings.Contains(stdout, "signed-private-url") || !strings.Contains(stdout, `"asset_url":"[REDACTED]"`) {
		t.Fatalf("metadata retry: requests=%d exit=%d stdout=%s stderr=%s", calls.Load(), code, stdout, stderr)
	}
}

func TestIneligibleCommandsDoNotRetryOn429(t *testing.T) {
	for _, tc := range []struct {
		name, version string
		args          []string
		handlers      func() map[string]handler
	}{
		{"get-with-side-effects", "v2", []string{"release", "changelog", "get", "--workspace", "team", "--release", transferID}, portfolioReleaseHandlers},
		{"download", "v1", []string{"workspace-asset", "download", "--workspace", "team", "--id", transferID, "--output-file", filepath.Join(t.TempDir(), "asset")}, assetTransferHandlers},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "retry-key")
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
			}))
			defer server.Close()
			catalog, handlers := workItemCatalog(t), tc.handlers()
			args := append(tc.args, "--api-version", tc.version, "--base-url", server.URL)
			code, stdout, stderr := invoke(t, catalog, args, handlers)
			if calls.Load() != 1 || code != 5 || stdout != "" || !strings.Contains(stderr, `"http_status":429`) {
				t.Fatalf("non-retry operation: requests=%d exit=%d stdout=%s stderr=%s", calls.Load(), code, stdout, stderr)
			}
			code, stdout, stderr = invoke(t, catalog, append(args, "--retry-on-429"), handlers)
			if calls.Load() != 1 || code != 2 || stdout != "" || !strings.Contains(stderr, `"code":"usage_error"`) {
				t.Fatalf("retry flag was not rejected before HTTP: requests=%d exit=%d stdout=%s stderr=%s", calls.Load(), code, stdout, stderr)
			}
		})
	}
}
