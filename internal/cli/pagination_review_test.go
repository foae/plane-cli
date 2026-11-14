package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/foae/plane-cli/internal/httpx"
	"github.com/spf13/cobra"
)

const paginationReviewToken = "pagination-review-secret-token"
const paginationReviewCursor = "pagination-review-secret-cursor+="

func paginationReviewArgs(version, base string) []string {
	return []string{"work-item", "list", "--api-version", version, "--base-url", base, "--workspace", "team", "--project", workItemTestProject, "--scope", "project", "--all"}
}

func paginationReviewFailure(t *testing.T, code int, stdout, stderr, wantCode string, wantExit, pages int) map[string]any {
	t.Helper()
	if code != wantExit || stdout != "" || strings.Count(stderr, "\n") != 1 {
		t.Fatalf("aggregate failure: exit=%d stdout bytes=%d stderr=%s", code, len(stdout), stderr)
	}
	for _, secret := range []string{paginationReviewToken, paginationReviewCursor} {
		if strings.Contains(stderr, secret) {
			t.Fatal("aggregate error leaked secret-bearing continuation or credential")
		}
	}
	var envelope struct {
		SchemaVersion int            `json:"schema_version"`
		Error         map[string]any `json:"error"`
	}
	if err := json.Unmarshal([]byte(stderr), &envelope); err != nil {
		t.Fatal(err)
	}
	wantProgress := map[string]any{"mode": "all", "pages": float64(pages), "complete": false}
	if envelope.SchemaVersion != 1 || envelope.Error["code"] != wantCode || envelope.Error["command"] != "work-item list" || !reflect.DeepEqual(envelope.Error["progress"], wantProgress) {
		t.Fatalf("aggregate diagnostic: %s", stderr)
	}
	return envelope.Error
}

func TestPaginationReviewBooleanContinuation(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		for _, value := range []string{"missing", "null", `"false"`, "0", "[]", "{}", "false"} {
			t.Run(version+"/"+value, func(t *testing.T) {
				transportEnv(t)
				t.Setenv("PLANE_CLI_API_KEY", paginationReviewToken)
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls++
					w.Header().Set("Content-Type", "application/json")
					field, prefix := "next_page_results", `{"results":[],"next_cursor":"`+paginationReviewCursor+`"`
					if version == "v2" {
						field, prefix = "has_more", `{"data":[],"pagination":{"style":"cursor"},"next_cursor":"`+paginationReviewCursor+`"`
					}
					if value != "missing" {
						prefix += `,"` + field + `":` + value
					}
					_, _ = io.WriteString(w, prefix+"}")
				}))
				defer server.Close()
				args := paginationReviewArgs(version, server.URL)
				if version == "v2" {
					args = append(args, "--paginate", "cursor", "--order-by", "created_at")
				}
				code, stdout, stderr := invoke(t, workItemCatalog(t), args, workItemHandlers())
				if value == "false" {
					if code != 0 || stderr != "" || !strings.Contains(stdout, `"complete":true`) {
						t.Fatalf("boolean false did not terminate: exit=%d stderr=%s", code, stderr)
					}
				} else {
					paginationReviewFailure(t, code, stdout, stderr, "invalid_pagination", 5, 1)
				}
				if calls != 1 {
					t.Fatalf("requests=%d; want 1", calls)
				}
			})
		}
	}
}

func TestPaginationReviewMalformedContinuation(t *testing.T) {
	for _, tc := range []struct {
		name, body string
	}{
		{"missing-pagination", `{"data":[],"next":null}`},
		{"unknown-style", `{"data":[],"next":null,"pagination":{"style":"unknown"}}`},
		{"missing-offset", `{"data":[],"pagination":{"style":"offset"}}`},
		{"offset-url", `{"data":[],"next":"https://example.invalid/secret","pagination":{"style":"offset"}}`},
		{"offset-negative", `{"data":[],"next":-1,"pagination":{"style":"offset"}}`},
		{"offset-fractional", `{"data":[],"next":1.5,"pagination":{"style":"offset"}}`},
		{"offset-nonprogressing", `{"data":[],"next":0,"pagination":{"style":"offset"}}`},
		{"offset-limit", `{"data":[],"next":10001,"pagination":{"style":"offset"}}`},
		{"cursor-missing", `{"data":[],"has_more":true,"pagination":{"style":"cursor"}}`},
		{"cursor-null", `{"data":[],"has_more":true,"next_cursor":null,"pagination":{"style":"cursor"}}`},
		{"cursor-number", `{"data":[],"has_more":true,"next_cursor":2,"pagination":{"style":"cursor"}}`},
		{"cursor-empty", `{"data":[],"has_more":true,"next_cursor":"","pagination":{"style":"cursor"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", paginationReviewToken)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			code, stdout, stderr := invoke(t, workItemCatalog(t), paginationReviewArgs("v2", server.URL), workItemHandlers())
			paginationReviewFailure(t, code, stdout, stderr, "invalid_pagination", 5, 1)
			if calls != 1 {
				t.Fatalf("requests=%d; want 1", calls)
			}
		})
	}
}

func TestPaginationReviewCyclesAndPageLimit(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		for _, limit := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/limit=%t", version, limit), func(t *testing.T) {
				transportEnv(t)
				t.Setenv("PLANE_CLI_API_KEY", paginationReviewToken)
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					wantCursor := []string{"", paginationReviewCursor, "second-private-cursor"}
					if calls > len(wantCursor) {
						t.Error("cycle made an extra request")
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					if r.URL.Query().Get("cursor") != wantCursor[calls-1] || r.URL.Query().Get("fields") != "id,name" {
						t.Error("pagination changed continuation or filter")
					}
					next := paginationReviewCursor
					if calls == 2 {
						next = "second-private-cursor"
					}
					w.Header().Set("Content-Type", "application/json")
					if version == "v1" {
						_, _ = fmt.Fprintf(w, `{"results":[],"next_page_results":true,"next_cursor":%q}`, next)
					} else {
						_, _ = fmt.Fprintf(w, `{"data":[],"pagination":{"style":"cursor"},"has_more":true,"next_cursor":%q}`, next)
					}
				}))
				defer server.Close()
				args := append(paginationReviewArgs(version, server.URL), "--fields", "id,name")
				if version == "v2" {
					args = append(args, "--paginate", "cursor", "--order-by", "created_at")
				}
				wantCode, wantExit, pages := "invalid_pagination", 5, 3
				if limit {
					args = append(args, "--max-pages", "2")
					wantCode, wantExit, pages = "page_limit", 1, 2
				}
				code, stdout, stderr := invoke(t, workItemCatalog(t), args, workItemHandlers())
				paginationReviewFailure(t, code, stdout, stderr, wantCode, wantExit, pages)
				if calls != pages {
					t.Fatalf("requests=%d; want %d", calls, pages)
				}
				if limit && !strings.Contains(stderr, "manual pagination") {
					t.Fatal("page limit omitted recovery guidance")
				}
			})
		}
	}
}

func TestPaginationReviewLaterHTTPFailure(t *testing.T) {
	for _, table := range []bool{false, true} {
		t.Run(fmt.Sprint(table), func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", paginationReviewToken)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				if calls == 1 {
					_, _ = fmt.Fprintf(w, `{"data":[{"id":"private-read-result"}],"pagination":{"style":"cursor"},"has_more":true,"next_cursor":%q}`, paginationReviewCursor)
					return
				}
				if r.URL.Query().Get("cursor") != paginationReviewCursor {
					t.Error("cursor changed")
				}
				w.Header().Set("Retry-After", "7")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = fmt.Fprintf(w, `{"detail":%q}`, paginationReviewToken+paginationReviewCursor)
			}))
			defer server.Close()
			args := append(paginationReviewArgs("v2", server.URL), "--paginate", "cursor", "--order-by", "created_at")
			if table {
				args = append(args, "--output", "table")
			}
			code, stdout, stderr := invoke(t, workItemCatalog(t), args, workItemHandlers())
			failure := paginationReviewFailure(t, code, stdout, stderr, "api_error", 5, 1)
			if calls != 2 || failure["http_status"] != float64(429) || failure["retry_after_seconds"] != float64(7) || strings.Contains(stderr, "private-read-result") {
				t.Fatalf("later HTTP failure: calls=%d diagnostic=%s", calls, stderr)
			}
		})
	}
}

func TestPaginationReviewAggregateSizeCap(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", paginationReviewToken)
	// Each native response is below the independent 16 MiB response limit.
	// Four such pages exceed the aggregate bound, including its envelope reserve.
	padding := strings.Repeat("x", (16<<20)-1024)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"data":[],"padding":"%s","next":%d,"pagination":{"style":"offset"}}`, padding, calls)
	}))
	defer server.Close()
	code, stdout, stderr := invoke(t, workItemCatalog(t), paginationReviewArgs("v2", server.URL), workItemHandlers())
	paginationReviewFailure(t, code, stdout, stderr, "output_limit", 1, 3)
	if calls != 4 || !strings.Contains(stderr, "64 MiB") || !strings.Contains(stderr, "manual pagination") {
		t.Fatalf("aggregate cap: calls=%d diagnostic=%s", calls, stderr)
	}
}

func TestPaginationReviewDerivedContextErrors(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(fmt.Sprint(deadline), func(t *testing.T) {
			transportEnv(t)
			handlers := workItemHandlers()
			// Keep the command's parent alive: the aggregate must classify its
			// own derived cancellation rather than relying on run's parent.
			handlers["work-item-list"] = func(cmd *cobra.Command, _ []string, _ map[string]any) error {
				ctx, cancel := context.WithCancel(cmd.Context())
				if deadline {
					cancel()
					ctx, cancel = context.WithDeadline(cmd.Context(), time.Now().Add(-time.Second))
				}
				cancel()
				return aggregatePages(cmd, ctx, nil, nil, url.Values{}, "", "", "v2", httpx.Response{Data: json.RawMessage(`{"data":[],"next":null,"pagination":{"style":"offset"}}`), Status: 200})
			}
			code, stdout, stderr := invoke(t, workItemCatalog(t), paginationReviewArgs("v2", "https://example.invalid"), handlers)
			wantCode, wantExit := "interrupted", 130
			if deadline {
				wantCode, wantExit = "deadline_exceeded", 4
			}
			paginationReviewFailure(t, code, stdout, stderr, wantCode, wantExit, 0)
		})
	}
}

func TestPaginationReviewLaterDeadline(t *testing.T) {
	for _, total := range []bool{false, true} {
		t.Run(fmt.Sprintf("total=%t", total), func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", paginationReviewToken)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls > 1 {
					<-r.Context().Done()
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"data":[],"pagination":{"style":"cursor"},"has_more":true,"next_cursor":%q}`, paginationReviewCursor)
			}))
			defer server.Close()
			args := append(paginationReviewArgs("v2", server.URL), "--paginate", "cursor", "--order-by", "created_at")
			if total {
				args = append(args, "--timeout", "500ms", "--request-timeout", "2s")
			} else {
				args = append(args, "--request-timeout", "200ms")
			}
			code, stdout, stderr := invoke(t, workItemCatalog(t), args, workItemHandlers())
			paginationReviewFailure(t, code, stdout, stderr, "deadline_exceeded", 4, 1)
			if calls != 2 {
				t.Fatalf("requests=%d; want 2", calls)
			}
		})
	}
}

func TestPaginationReviewSpoolCreationFailure(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", paginationReviewToken)
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[],"next":null,"pagination":{"style":"offset"}}`)
	}))
	defer server.Close()
	code, stdout, stderr := invoke(t, workItemCatalog(t), paginationReviewArgs("v2", server.URL), workItemHandlers())
	paginationReviewFailure(t, code, stdout, stderr, "output_error", 1, 0)
}

func TestPaginationReviewSpoolCleanup(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", paginationReviewToken)
	spools := t.TempDir()
	t.Setenv("TMPDIR", spools)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[],"has_more":null,"pagination":{"style":"cursor"}}`)
	}))
	defer server.Close()
	code, stdout, stderr := invoke(t, workItemCatalog(t), paginationReviewArgs("v2", server.URL), workItemHandlers())
	paginationReviewFailure(t, code, stdout, stderr, "invalid_pagination", 5, 1)
	entries, err := os.ReadDir(spools)
	if err != nil || len(entries) != 0 {
		t.Fatalf("aggregate left temporary spools: entries=%d error=%v", len(entries), err)
	}
}

func TestPaginationReviewDeliveryFailure(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", paginationReviewToken)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[],"next":null,"pagination":{"style":"offset"}}`)
	}))
	defer server.Close()
	var stderr bytes.Buffer
	code := run(context.Background(), paginationReviewArgs("v2", server.URL), Streams{In: strings.NewReader(""), Out: brokenWriter{}, Err: &stderr}, Build{}, workItemCatalog(t), workItemHandlers())
	paginationReviewFailure(t, code, "", stderr.String(), "output_error", 1, 1)
}
