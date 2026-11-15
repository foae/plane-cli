package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/foae/plane-cli/internal/spec"
)

type p07CustomerFixture struct {
	ID       string            `json:"id"`
	Version  string            `json:"version"`
	Args     []string          `json:"args"`
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Query    map[string]string `json:"query"`
	Body     json.RawMessage   `json:"body"`
	Status   int               `json:"status"`
	Response json.RawMessage   `json:"response"`
}

func p07CustomerFixtures(t *testing.T) []p07CustomerFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/p07-customers-operations.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []p07CustomerFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func p07CustomerArgs(t *testing.T, fixture p07CustomerFixture, base string) []string {
	t.Helper()
	args := append([]string{}, fixture.Args...)
	args = append(args, "--api-version", fixture.Version, "--base-url", base)
	if string(fixture.Body) != "null" && len(fixture.Body) != 0 {
		file := filepath.Join(t.TempDir(), "body.json")
		if err := os.WriteFile(file, fixture.Body, 0600); err != nil {
			t.Fatal(err)
		}
		args = append(args, "--body-file", file)
	}
	return args
}

// Every documented operation has an independent wire fixture. The selected
// schema, constructor registration and HTTP transport are exercised together.
func TestP07CustomerOperations(t *testing.T) {
	for _, fixture := range p07CustomerFixtures(t) {
		t.Run(fixture.ID, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				query := url.Values{}
				for key, value := range fixture.Query {
					query.Set(key, value)
				}
				if r.Method != fixture.Method || r.URL.Path != fixture.Path || r.URL.RawQuery != query.Encode() || r.Header.Get("X-API-Key") != "fixture-key" {
					t.Errorf("request %s %s does not match %s %s?%s", r.Method, r.URL, fixture.Method, fixture.Path, query.Encode())
				}
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if string(fixture.Body) == "null" {
					if len(data) != 0 {
						t.Errorf("unexpected request body: %s", data)
					}
				} else {
					var got, want any
					if err := json.Unmarshal(data, &got); err != nil {
						t.Error(err)
					}
					if err := json.Unmarshal(fixture.Body, &want); err != nil {
						t.Error(err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Errorf("request body %s != %s", data, fixture.Body)
					}
				}
				if calls == 2 {
					w.Header().Set("Content-Type", "application/problem+json")
					w.WriteHeader(402)
					_, _ = io.WriteString(w, `{"type":"payment_required","code":"payment_required","detail":"Customers are unavailable on this plan"}`)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(fixture.ID, "upsert-customer") {
					w.Header().Set("X-Plane-Upsert", "updated")
				}
				w.WriteHeader(fixture.Status)
				if string(fixture.Response) != "null" {
					_, _ = w.Write(fixture.Response)
				}
			}))
			defer server.Close()
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			args := p07CustomerArgs(t, fixture, server.URL)
			code, out, stderr := invoke(t, catalog, args, nil)
			if code != 0 || stderr != "" || calls != 1 {
				t.Fatalf("success code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
			}
			if strings.Contains(string(fixture.Response), "9007199254740993") && !strings.Contains(out, "9007199254740993") {
				t.Fatalf("native precision or extension field lost: %s", out)
			}
			if strings.Contains(fixture.ID, "property-values") && fixture.Method == "GET" && strings.Contains(string(fixture.Response), "empty") {
				for _, value := range []string{`"text":null`, `"empty":[]`, `"false":false`, `"zero":0`, `"blank":""`} {
					if !strings.Contains(strings.ReplaceAll(out, " ", ""), value) {
						t.Errorf("property value %s lost: %s", value, out)
					}
				}
			}
			code, out, stderr = invoke(t, catalog, args, nil)
			if code != 5 || out != "" || calls != 2 || !strings.Contains(stderr, "402") {
				t.Fatalf("402 not preserved: code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
			}
		})
	}
}

func TestP07CustomerRemovalConfirmation(t *testing.T) {
	for _, fixture := range p07CustomerFixtures(t) {
		if !strings.Contains(strings.Join(fixture.Args, " "), "--yes") {
			continue
		}
		t.Run(fixture.ID, func(t *testing.T) {
			transportEnv(t)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(204)
			}))
			defer server.Close()
			args := p07CustomerArgs(t, fixture, server.URL)
			filtered := make([]string, 0, len(args))
			for _, arg := range args {
				if arg != "--yes" {
					filtered = append(filtered, arg)
				}
			}
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			code, out, stderr := invoke(t, catalog, filtered, nil)
			if code != 2 || out != "" || calls != 0 || !strings.Contains(stderr, "confirmation_required") {
				t.Fatalf("confirmation bypassed: code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
			}
		})
	}
}

func TestP07CustomerInvalidBodies(t *testing.T) {
	for _, tc := range []struct {
		version string
		args    []string
		body    string
	}{
		{"v2", []string{"customer", "upsert"}, `{"name":"Acme"}`},
		{"v2", []string{"customer", "create"}, `{"name":"Acme","description":{}}`},
		{"v1", []string{"customer", "create"}, `{"name":"Acme","description":"v2 string"}`},
		{"v2", []string{"customer", "property", "create"}, `{"display_name":"Tier","property_type":"OPTION","options":["not an object"]}`},
		{"v2", []string{"customer", "property", "create"}, `{"display_name":"Tier","property_type":"UNKNOWN"}`},
		{"v2", []string{"customer", "property-value", "set", "--customer", "customer-id"}, `{"values":[]}`},
		{"v1", []string{"customer", "property-value", "update", "--customer", "customer-id", "--property", "property-id"}, `{"values":{}}`},
	} {
		t.Run(tc.version+"/"+strings.Join(tc.args, " ")+"/"+tc.body, func(t *testing.T) {
			transportEnv(t)
			fixture := p07CustomerFixture{Version: tc.version, Args: append(tc.args, "--workspace", "example"), Body: json.RawMessage(tc.body)}
			args := p07CustomerArgs(t, fixture, "http://127.0.0.1:1")
			args = append(args, "--dry-run")
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			code, out, stderr := invoke(t, catalog, args, nil)
			if code != 2 || out != "" {
				t.Fatalf("invalid body accepted: code=%d out=%s stderr=%s", code, out, stderr)
			}
		})
	}
}

func TestP07CustomerUpsertOutcome(t *testing.T) {
	for _, outcome := range []string{"created", "updated"} {
		t.Run(outcome, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Plane-Upsert", outcome)
				if outcome == "created" {
					w.WriteHeader(201)
				}
				_, _ = io.WriteString(w, `{"id":"customer-id","name":"Acme"}`)
			}))
			defer server.Close()
			fixture := p07CustomerFixture{Version: "v2", Args: []string{"customer", "upsert", "--workspace", "example"}, Body: json.RawMessage(`{"name":"Acme","external_source":"crm","external_id":"acme"}`)}
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			code, out, stderr := invoke(t, catalog, p07CustomerArgs(t, fixture, server.URL), nil)
			if code != 0 || stderr != "" || !strings.Contains(out, `"upsert":"`+outcome+`"`) {
				t.Fatalf("upsert outcome lost: code=%d out=%s stderr=%s", code, out, stderr)
			}
		})
	}
}

func TestP07CustomerAssociationPresence(t *testing.T) {
	for _, tc := range []struct {
		id   string
		body string
	}{
		{"v2.customer-requests.update-customer-request", `{"name":"Renamed","description":null,"link":""}`},
		{"v2.customers.manage-customer-work-items", `{"add":["44444444-4444-4444-8444-444444444444"]}`},
		{"v2.customers.manage-customer-work-items", `{"remove":[]}`},
		{"v2.customer-properties.update-customer-property", `{"description":null,"default_value":[],"options":[],"is_active":false}`},
		{"v2.customer-property-values.set-customer-property-values", `{"values":{}}`},
	} {
		t.Run(tc.id+"/"+tc.body, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			var fixture p07CustomerFixture
			for _, candidate := range p07CustomerFixtures(t) {
				if candidate.ID == tc.id {
					fixture = candidate
					break
				}
			}
			if fixture.ID == "" {
				t.Fatal("missing operation fixture")
			}
			fixture.Body = json.RawMessage(tc.body)
			fixture.Args = append([]string{}, fixture.Args...)
			for index, arg := range fixture.Args {
				if arg == "--yes" {
					fixture.Args = append(fixture.Args[:index], fixture.Args[index+1:]...)
					break
				}
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				var got, want any
				if err := json.Unmarshal(data, &got); err != nil {
					t.Error(err)
				}
				if err := json.Unmarshal([]byte(tc.body), &want); err != nil {
					t.Error(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("presence/null/empty values changed: %s != %s", data, tc.body)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(fixture.Status)
				if string(fixture.Response) != "null" {
					_, _ = w.Write(fixture.Response)
				}
			}))
			defer server.Close()
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			code, out, stderr := invoke(t, catalog, p07CustomerArgs(t, fixture, server.URL), nil)
			if code != 0 || stderr != "" || calls != 1 {
				t.Fatalf("non-removal rejected: code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
			}
		})
	}
}

func TestP07CustomerTables(t *testing.T) {
	for _, fixture := range p07CustomerFixtures(t) {
		if fixture.ID != "v1.customer.list-customers" && fixture.ID != "v1.customer.get-customer-detail" && fixture.ID != "v2.customers.list-customers" && fixture.ID != "v2.customers.get-customer" {
			continue
		}
		t.Run(fixture.ID, func(t *testing.T) {
			transportEnv(t)
			t.Setenv("PLANE_CLI_API_KEY", "fixture-key")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(fixture.Response)
			}))
			defer server.Close()
			catalog, err := spec.Load()
			if err != nil {
				t.Fatal(err)
			}
			args := append(p07CustomerArgs(t, fixture, server.URL), "--output", "table")
			code, out, stderr := invoke(t, catalog, args, nil)
			if code != 0 || stderr != "" || calls != 1 || !strings.Contains(out, "Native") || !strings.Contains(out, "native-id") {
				t.Fatalf("table projection failed: code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
			}
		})
	}
}
