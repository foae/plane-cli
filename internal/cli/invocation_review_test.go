package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type invocationBlockedReader struct {
	started chan struct{}
	release chan struct{}
}

func (r *invocationBlockedReader) Read([]byte) (int, error) {
	close(r.started)
	<-r.release
	return 0, io.EOF
}

func TestConfiguredInvocationTimeoutIncludesStdin(t *testing.T) {
	transportEnv(t)
	// Ambiguous credentials must not be inspected before body validation.
	t.Setenv("PLANE_CLI_API_KEY", "test-key")
	t.Setenv("PLANE_CLI_ACCESS_TOKEN", "test-token")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()
	reader := &invocationBlockedReader{started: make(chan struct{}), release: make(chan struct{})}
	defer close(reader.release)
	var out, diagnostic bytes.Buffer
	catalog := workItemCatalog(t)
	args := []string{"work-item", "advanced-search", "--api-version", "v1", "--base-url", server.URL, "--workspace", "team", "--body-file", "-", "--timeout", "40ms"}
	result := make(chan int, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() {
		result <- run(ctx, args, Streams{In: reader, Out: &out, Err: &diagnostic}, Build{}, catalog, nil)
	}()
	select {
	case <-reader.started:
	case <-ctx.Done():
		t.Fatal("body read did not start")
	}
	select {
	case code := <-result:
		if ctx.Err() != nil || code != 4 || !strings.Contains(diagnostic.String(), `"code":"deadline_exceeded"`) || out.Len() != 0 || calls.Load() != 0 {
			t.Fatalf("code=%d calls=%d stdout=%s stderr=%s parent=%v", code, calls.Load(), &out, &diagnostic, ctx.Err())
		}
	case <-ctx.Done():
		<-result
		t.Fatal("configured timeout did not stop stdin read")
	}
}

func TestReadOnlyPostFailureHasNoMutationMarkers(t *testing.T) {
	catalog := workItemCatalog(t)
	descriptor, ok := catalog.Find([]string{"work-item", "advanced-search"})
	if !ok || descriptor.SideEffect != "read" {
		t.Fatalf("embedded advanced-search side effect=%q found=%v", descriptor.SideEffect, ok)
	}
	for _, failure := range []string{"http", "protocol", "output"} {
		t.Run(failure, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "test-key")
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodPost {
					t.Errorf("method=%s", r.Method)
				}
				w.Header().Set("Content-Type", "application/json")
				if failure == "http" {
					w.WriteHeader(http.StatusInternalServerError)
				}
				body := `[]`
				if failure == "protocol" {
					body = `invalid`
				}
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			args := []string{"work-item", "advanced-search", "--api-version", "v1", "--base-url", server.URL, "--workspace", "team", "--body-file", workItemBodyFile(t, `{"query":"","filters":{"priority":["high"],"unknown_extension":false},"limit":0,"workspace_search":false,"project_id":"project"}`)}
			var out, diagnostic bytes.Buffer
			var writer io.Writer = &out
			want := 5
			if failure == "output" {
				writer = brokenWriter{}
				want = 1
			}
			code := run(context.Background(), args, Streams{In: strings.NewReader(""), Out: writer, Err: &diagnostic}, Build{}, catalog, nil)
			if code != want || calls.Load() != 1 || out.Len() != 0 || strings.Contains(diagnostic.String(), `"outcome_unknown":true`) || strings.Contains(diagnostic.String(), `"remote_changed":true`) {
				t.Fatalf("code=%d calls=%d stdout=%s stderr=%s", code, calls.Load(), &out, &diagnostic)
			}
		})
	}
}
