package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foae/plane-cli/internal/spec"
)

func TestP08WebhookDeliveryFailure(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"hook","secret_key":"undelivered-secret"}`)
	}))
	defer server.Close()
	args := []string{"webhook", "regenerate-secret", "--api-version", "v2", "--workspace", "fixture", "--id", "11111111-1111-4111-8111-111111111111", "--base-url", server.URL, "--yes"}
	var diagnostic bytes.Buffer
	code := Run(context.Background(), args, Streams{Out: failedAssetWriter{}, Err: &diagnostic}, Build{})
	stderr := diagnostic.String()
	if code != 6 || calls != 1 || !strings.Contains(stderr, `"remote_changed":true`) || !strings.Contains(stderr, `"secret_delivered":false`) || strings.Contains(stderr, "undelivered-secret") {
		t.Fatalf("delivery failure code=%d calls=%d err=%s", code, calls, stderr)
	}
}

func TestP08WebhookLogPaginationRedaction(t *testing.T) {
	for _, bound := range []string{"1", "2"} {
		t.Run(bound, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Query().Get("paginate") != "cursor" || r.URL.Query().Get("fields") != "id,request_headers" {
					t.Errorf("lost filters: %s", r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				if calls == 1 {
					_, _ = io.WriteString(w, `{"data":[{"id":"one","request_headers":{"Authorization":"hidden-first"}}],"next_cursor":"next-token","has_more":true,"pagination":{"style":"cursor"}}`)
				} else {
					if r.URL.Query().Get("cursor") != "next-token" {
						t.Errorf("cursor not forwarded: %s", r.URL)
					}
					_, _ = io.WriteString(w, `{"data":[{"id":"two","request_headers":{"X-API-Key":"hidden-second"}}],"next_cursor":null,"has_more":false,"pagination":{"style":"cursor"}}`)
				}
			}))
			defer server.Close()
			args := []string{"webhook", "log", "list", "--api-version", "v2", "--workspace", "fixture", "--webhook", "11111111-1111-4111-8111-111111111111", "--base-url", server.URL, "--paginate", "cursor", "--fields", "id,request_headers", "--all", "--max-pages", bound}
			code, out, stderr := invoke(t, catalog, args, nil)
			if strings.Contains(out+stderr, "hidden-") {
				t.Fatal("credential leaked")
			}
			if bound == "1" {
				if code != 1 || calls != 1 || out != "" || !strings.Contains(stderr, "page_limit") {
					t.Fatalf("bound code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
				}
			} else if code != 0 || calls != 2 || !strings.Contains(out, `"id":"two"`) || !strings.Contains(out, "[REDACTED]") {
				t.Fatalf("aggregate code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
			}
		})
	}
}

func TestP08GroupSyncMutationUncertainty(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		for _, status := range []int{200, 500} {
			t.Run(version+"/"+http.StatusText(status), func(t *testing.T) {
				transportEnv(t)
				t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
				catalog, err := spec.Load()
				if err != nil {
					t.Fatal(err)
				}
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(status)
					_, _ = io.WriteString(w, `{"incomplete":`)
				}))
				defer server.Close()
				code, out, diagnostic := invoke(t, catalog, []string{"group-sync", "config", "get", "--api-version", version, "--workspace", "fixture", "--base-url", server.URL, "--yes"}, nil)
				unknown := strings.Contains(diagnostic, `"outcome_unknown":true`)
				if code != 5 || calls != 1 || out != "" || unknown != (version == "v1") {
					t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, diagnostic)
				}
			})
		}
	}
}

func TestP08WebhookMissingSecret(t *testing.T) {
	for _, response := range []string{`null`, `{}`, `{"secret_key":null}`, `{"secret_key":""}`} {
		for _, file := range []bool{false, true} {
			t.Run(response+map[bool]string{false: "/stdout", true: "/file"}[file], func(t *testing.T) {
				transportEnv(t)
				t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
				catalog, err := spec.Load()
				if err != nil {
					t.Fatal(err)
				}
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, response)
				}))
				defer server.Close()
				path := filepath.Join(t.TempDir(), "secret.json")
				args := []string{"webhook", "regenerate-secret", "--api-version", "v2", "--workspace", "fixture", "--id", "11111111-1111-4111-8111-111111111111", "--base-url", server.URL, "--yes"}
				if file {
					args = append(args, "--secret-output-file", path)
				}
				code, out, diagnostic := invoke(t, catalog, args, nil)
				if code != 6 || calls != 1 || out != "" || !strings.Contains(diagnostic, `"secret_delivered":false`) || !strings.Contains(diagnostic, `"remote_changed":true`) {
					t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, diagnostic)
				}
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("missing credential installed: %v", err)
				}
			})
		}
	}
}

func TestP08WebhookReceiptAndSingleReadRedaction(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"hook","secret_key":"canary-top","nested":{"secret_key":"canary-nested"},"request_headers":"Authorization: canary-text","response_headers":{"X-Plane-Signature":"canary-signature"}}`)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "secret.json")
	common := []string{"--api-version", "v2", "--workspace", "fixture", "--id", "11111111-1111-4111-8111-111111111111", "--base-url", server.URL}
	args := append([]string{"webhook", "regenerate-secret"}, common...)
	args = append(args, "--yes", "--secret-output-file", path)
	code, out, diagnostic := invoke(t, catalog, args, nil)
	if code != 0 || strings.Contains(out+diagnostic, "canary-") || !strings.Contains(out, `"id":"hook"`) {
		t.Fatalf("receipt code=%d out=%s err=%s", code, out, diagnostic)
	}
	protected, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(protected, []byte("canary-top")) || !bytes.Contains(protected, []byte("canary-nested")) {
		t.Fatalf("protected result lost credential: %v", err)
	}
	code, out, diagnostic = invoke(t, catalog, append([]string{"webhook", "get"}, common...), nil)
	if code != 0 || strings.Contains(out+diagnostic, "canary-") || !strings.Contains(out, `"id":"hook"`) {
		t.Fatalf("read code=%d out=%s err=%s", code, out, diagnostic)
	}
}

func TestP08WebhookAggregateRedactionExpansion(t *testing.T) {
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	payload := `{"data":[{"request_body":"` + strings.Repeat("<", 12<<20) + `"}],"next_cursor":null,"has_more":false,"pagination":{"style":"cursor"}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, payload)
	}))
	defer server.Close()
	code, out, diagnostic := invoke(t, catalog, []string{"webhook", "log", "list", "--api-version", "v2", "--workspace", "fixture", "--webhook", "11111111-1111-4111-8111-111111111111", "--base-url", server.URL, "--all"}, nil)
	if code != 1 || out != "" || !strings.Contains(diagnostic, `"code":"output_limit"`) {
		t.Fatalf("code=%d stdout bytes=%d err=%s", code, len(out), diagnostic)
	}
}

func TestP08MembershipRejectsNonBareEmail(t *testing.T) {
	transportEnv(t)
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range [][]string{{"invitation", "create"}, {"member", "workspace", "remove"}} {
		code, out, diagnostic := invoke(t, catalog, append(command, "--api-version", "v2", "--base-url", "https://example.invalid", "--workspace", "fixture", "--body-file", workItemBodyFile(t, `{"email":"<a@example.invalid>"}`), "--dry-run"), nil)
		if code != 2 || out != "" {
			t.Fatalf("code=%d out=%s err=%s", code, out, diagnostic)
		}
	}
}
