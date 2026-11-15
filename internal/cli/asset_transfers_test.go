package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/foae/plane-cli/internal/spec"
)

const transferID = "11111111-1111-4111-8111-111111111111"

func transferCatalog(t *testing.T) spec.Catalog {
	t.Helper()
	transportEnv(t)
	t.Setenv("PLANE_CLI_API_KEY", "private-api-key")
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}
func transferFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func transferParents(group string) []string {
	switch group {
	case "attachment":
		return []string{"--workspace", "team", "--project", transferID, "--work-item", transferID}
	case "workspace-page-attachment":
		return []string{"--workspace", "team", "--page", transferID}
	case "workspace-asset":
		return []string{"--workspace", "team"}
	}
	return nil
}

func TestAssetTransferUpload(t *testing.T) {
	for _, tc := range []struct{ version, group, path, confirm string }{
		{"v2", "attachment", "/api/v2/workspaces/team/projects/" + transferID + "/work-items/" + transferID + "/attachments/", ""},
		{"v2", "user-asset", "/api/v2/users/me/assets/", ""},
		{"v2", "workspace-asset", "/api/v2/workspaces/team/assets/", ""},
		{"v1", "workspace-asset", "/api/v1/workspaces/team/assets/", ""},
		{"v2", "workspace-page-attachment", "/api/v2/workspaces/team/pages/" + transferID + "/attachments/", ""},
		{"v1", "workspace-page-attachment", "/api/v1/workspaces/team/assets/", "/api/v1/workspaces/team/pages/" + transferID + "/attachments/" + transferID + "/"},
	} {
		t.Run(tc.version+"/"+tc.group, func(t *testing.T) {
			catalog := transferCatalog(t)
			t.Setenv("PLANE_CLI_API_VERSION", tc.version)
			storageCalls := 0
			storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				storageCalls++
				if r.Method != "POST" || r.Header.Get("X-API-Key") != "" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Error("storage received auth or wrong method")
				}
				reader, err := r.MultipartReader()
				if err != nil {
					t.Error(err)
					return
				}
				fields := map[string]string{}
				last := ""
				for {
					part, err := reader.NextPart()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Error(err)
						return
					}
					raw, _ := io.ReadAll(part)
					last = part.FormName()
					if last == "file" {
						if string(raw) != "file-bytes" || part.FileName() != "payload.txt" || part.Header.Get("Content-Type") != "text/plain" {
							t.Error("file part changed")
						}
					} else {
						fields[last] = string(raw)
					}
				}
				if last != "file" || fields["policy"] != "signed-policy" || fields["Content-Type"] != "text/plain" || fields["key"] != "object-key" {
					t.Error("signed form fields changed")
				}
				if tc.group == "attachment" {
					time.Sleep(500 * time.Millisecond)
				}
				w.WriteHeader(204)
			}))
			defer storage.Close()
			apiCalls := 0
			api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				apiCalls++
				if r.Header.Get("X-API-Key") != "private-api-key" {
					t.Error("missing API auth")
				}
				w.Header().Set("Content-Type", "application/json")
				if r.Method == "POST" {
					if r.URL.Path != tc.path {
						t.Errorf("metadata route %s", r.URL.Path)
					}
					var body map[string]any
					decoder := json.NewDecoder(r.Body)
					decoder.UseNumber()
					_ = decoder.Decode(&body)
					if body["name"] != "payload.txt" || body["size"] != json.Number("10") {
						t.Errorf("metadata %#v", body)
					}
					extension, ok := body["extension"].(map[string]any)
					if !ok || extension["integer"] != json.Number("9007199254740993") {
						t.Errorf("native metadata rounded: %#v", body["extension"])
					}
					if tc.version == "v1" && tc.group == "workspace-page-attachment" && (body["entity_type"] != "PAGE_DESCRIPTION" || body["entity_identifier"] != transferID) {
						t.Error("wiki metadata missing")
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"asset_id": transferID, "upload_data": map[string]any{"url": storage.URL, "fields": map[string]string{"policy": "signed-policy", "Content-Type": "text/plain", "key": "object-key"}}})
					return
				}
				expected := tc.confirm
				if expected == "" {
					expected = tc.path + transferID + "/"
				}
				if r.Method != "PATCH" || r.URL.Path != expected {
					t.Errorf("confirm %s %s", r.Method, r.URL.Path)
				}
				body, _ := io.ReadAll(r.Body)
				empty := tc.version == "v2" && (tc.group == "user-asset" || tc.group == "workspace-asset")
				if (empty && len(body) != 0) || (!empty && string(body) != `{"is_uploaded":true}`) {
					t.Errorf("confirmation body %s", body)
				}
				if tc.version == "v1" {
					w.WriteHeader(204)
				} else {
					_, _ = io.WriteString(w, `{"id":"`+transferID+`","asset_url":"signed-result","is_uploaded":true}`)
				}
			}))
			defer api.Close()
			ca := transferFile(t, "ca.pem", string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: api.Certificate().Raw})))
			args := append([]string{tc.group, "upload", "--base-url", api.URL, "--ca-file", ca, "--file", transferFile(t, "payload.txt", "file-bytes")}, transferParents(tc.group)...)
			metadata := `{"extension":{"integer":9007199254740993}}`
			if tc.group == "user-asset" {
				metadata = `{"entity_type":"USER_AVATAR","extension":{"integer":9007199254740993}}`
			}
			args = append(args, "--body-file", transferFile(t, "body.json", metadata))
			if tc.group == "attachment" {
				args = append(args, "--request-timeout", "250ms", "--timeout", "5s")
			}
			code, out, stderr := invoke(t, catalog, args, nil)
			if code != 0 || apiCalls != 2 || storageCalls != 1 || stderr != "" || strings.Contains(out, "signed-") {
				t.Fatalf("code=%d api=%d storage=%d out=%s err=%s", code, apiCalls, storageCalls, out, stderr)
			}
		})
	}
}

func TestAssetTransferFailurePhases(t *testing.T) {
	for _, phase := range []string{"storage", "confirmation", "size_limit", "missing_file", "unsafe_name", "fifo", "delivery"} {
		t.Run(phase, func(t *testing.T) {
			catalog := transferCatalog(t)
			t.Setenv("PLANE_CLI_API_VERSION", "v2")
			apiCalls, storageCalls := 0, 0
			storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				storageCalls++
				_, _ = io.Copy(io.Discard, r.Body)
				if phase == "storage" {
					w.WriteHeader(500)
					_, _ = io.WriteString(w, "signed-secret")
				} else {
					w.WriteHeader(204)
				}
			}))
			defer storage.Close()
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				apiCalls++
				w.Header().Set("Content-Type", "application/json")
				if r.Method == "PATCH" {
					if phase == "delivery" {
						_, _ = io.WriteString(w, `{"is_uploaded":true}`)
						return
					}
					w.WriteHeader(409)
					_, _ = io.WriteString(w, `{"detail":"signed-secret"}`)
					return
				}
				response := map[string]any{"asset_id": transferID, "upload_data": map[string]any{"url": storage.URL + "?token=signed-secret", "fields": map[string]string{"policy": "signed-secret"}}}
				if phase == "size_limit" {
					response["attachment"] = map[string]any{"size": 1}
				}
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer api.Close()
			name := "payload"
			if phase == "unsafe_name" {
				name = "unsafe\nname"
			}
			file := transferFile(t, name, "file-bytes")
			if phase == "missing_file" {
				file += "-missing"
			}
			if phase == "fifo" {
				file += "-fifo"
				if err := syscall.Mkfifo(file, 0600); err != nil {
					t.Fatal(err)
				}
			}
			args := append([]string{"workspace-page-attachment", "upload", "--base-url", api.URL, "--file", file}, transferParents("workspace-page-attachment")...)
			var code int
			var out, stderr string
			if phase == "delivery" {
				var diagnostic bytes.Buffer
				code = Run(context.Background(), args, Streams{Out: failedAssetWriter{}, Err: &diagnostic}, Build{})
				stderr = diagnostic.String()
				if !strings.Contains(stderr, "Upload confirmed") || strings.Contains(stderr, "incomplete") {
					t.Fatalf("unsafe recovery guidance: %s", stderr)
				}
			} else {
				code, out, stderr = invoke(t, catalog, args, nil)
			}
			if phase == "missing_file" || phase == "unsafe_name" || phase == "fifo" {
				if code != 2 || apiCalls != 0 || storageCalls != 0 {
					t.Fatalf("preflight code=%d calls=%d/%d", code, apiCalls, storageCalls)
				}
				return
			}
			expectedAPI := 1
			if phase == "confirmation" || phase == "delivery" {
				expectedAPI = 2
			}
			expectedStorage := 1
			if phase == "size_limit" {
				expectedStorage = 0
			}
			if code != 6 || out != "" || apiCalls != expectedAPI || storageCalls != expectedStorage || !strings.Contains(stderr, `"remote_changed":true`) || !strings.Contains(stderr, `"phase":"`+phase+`"`) || strings.Contains(stderr, "signed-secret") {
				t.Fatalf("code=%d calls=%d/%d out=%s err=%s", code, apiCalls, storageCalls, out, stderr)
			}
		})
	}
}

func TestAssetTransferDownload(t *testing.T) {
	for _, mode := range []string{"success", "v1", "workspace_asset", "exists", "overwrite", "mismatch", "redirect", "install_race", "slow_storage"} {
		t.Run(mode, func(t *testing.T) {
			catalog := transferCatalog(t)
			version := "v2"
			if mode == "v1" || mode == "workspace_asset" {
				version = "v1"
			}
			t.Setenv("PLANE_CLI_API_VERSION", version)
			destination := filepath.Join(t.TempDir(), "download.txt")
			if mode == "exists" || mode == "overwrite" {
				if err := os.WriteFile(destination, []byte("original"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			storageCalls, apiCalls := 0, 0
			storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				storageCalls++
				if r.Header.Get("X-API-Key") != "" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Error("storage auth leak")
				}
				if mode == "install_race" {
					_ = os.WriteFile(destination, []byte("racer"), 0600)
				}
				if mode == "redirect" {
					w.Header().Set("Location", "http://127.0.0.1:1/?secret=storage-secret")
					w.WriteHeader(302)
					return
				}
				if mode == "slow_storage" {
					time.Sleep(500 * time.Millisecond)
				}
				w.Header().Set("Content-Disposition", `attachment; filename="../../unsafe"`)
				_, _ = io.WriteString(w, "file-bytes")
			}))
			defer storage.Close()
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				apiCalls++
				if strings.HasSuffix(r.URL.Path, "/download/") {
					w.Header().Set("Location", storage.URL+"?secret=storage-secret")
					w.WriteHeader(302)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				size := 10
				if mode == "mismatch" {
					size = 2
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"size": size, "asset_url": storage.URL + "?secret=storage-secret"})
			}))
			defer api.Close()
			group := "workspace-page-attachment"
			if mode == "workspace_asset" {
				group = "workspace-asset"
			}
			args := append([]string{group, "download", "--base-url", api.URL, "--id", transferID, "--output-file", destination}, transferParents(group)...)
			if mode == "overwrite" {
				args = append(args, "--overwrite", "--yes")
			}
			if mode == "slow_storage" {
				args = append(args, "--request-timeout", "250ms", "--timeout", "5s")
			}
			code, out, stderr := invoke(t, catalog, args, nil)
			content, readErr := os.ReadFile(destination)
			if strings.Contains(out+stderr, "storage-secret") {
				t.Fatal("signed target leaked")
			}
			switch mode {
			case "exists":
				if code != 1 || apiCalls != 0 || storageCalls != 0 || string(content) != "original" {
					t.Fatalf("preflight failed %d %d/%d %q", code, apiCalls, storageCalls, content)
				}
			case "install_race":
				if code != 1 || string(content) != "racer" {
					t.Fatalf("clobbered raced destination: %d %q", code, content)
				}
			case "mismatch", "redirect":
				if code != 5 || !errors.Is(readErr, os.ErrNotExist) {
					t.Fatalf("failed download left destination: %d %s %v", code, stderr, readErr)
				}
			default:
				if code != 0 || string(content) != "file-bytes" || storageCalls != 1 {
					t.Fatalf("download: %d %s %s %q", code, out, stderr, content)
				}
				info, _ := os.Stat(destination)
				if info.Mode().Perm() != 0600 {
					t.Errorf("mode %v", info.Mode())
				}
			}
			files, _ := filepath.Glob(filepath.Join(filepath.Dir(destination), ".plane-cli-transfer-*"))
			if len(files) != 0 {
				t.Fatal("temporary output leaked")
			}
		})
	}
}

type failedAssetWriter struct{}

func (failedAssetWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestAssetProtectedDelivery(t *testing.T) {
	for _, mode := range []string{"protected", "preflight", "stdout_failure", "install_failure"} {
		t.Run(mode, func(t *testing.T) {
			catalog := transferCatalog(t)
			t.Setenv("PLANE_CLI_API_VERSION", "v2")
			target := filepath.Join(t.TempDir(), "credentials.json")
			calls := 0
			if mode == "preflight" {
				_ = os.WriteFile(target, []byte("original"), 0600)
			}
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if mode == "install_failure" {
					_ = os.WriteFile(target, []byte("racer"), 0600)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"asset_id":"`+transferID+`","upload_data":{"url":"https://storage.invalid/?secret=signed-secret","fields":{"policy":"signed-secret"}}}`)
			}))
			defer api.Close()
			args := []string{"workspace-asset", "create-upload", "--workspace", "team", "--base-url", api.URL, "--body-file", transferFile(t, "body.json", `{"name":"file","size":10}`)}
			if mode != "stdout_failure" {
				args = append(args, "--secret-output-file", target)
			}
			var code int
			var out, stderr string
			if mode == "stdout_failure" {
				var diagnostic bytes.Buffer
				code = Run(context.Background(), args, Streams{Out: failedAssetWriter{}, Err: &diagnostic}, Build{})
				stderr = diagnostic.String()
			} else {
				code, out, stderr = invoke(t, catalog, args, nil)
			}
			if strings.Contains(out+stderr, "signed-secret") {
				t.Fatal("signed secret leaked into ordinary output")
			}
			if mode == "protected" {
				raw, err := os.ReadFile(target)
				info, _ := os.Stat(target)
				if code != 0 || err != nil || !bytes.Contains(raw, []byte("signed-secret")) || info.Mode().Perm() != 0600 {
					t.Fatalf("protected output %d %s %v", code, stderr, err)
				}
			} else if mode == "preflight" {
				if code != 1 || calls != 0 {
					t.Fatalf("preflight sent mutation %d/%d", code, calls)
				}
			} else if code != 6 || calls != 1 || !strings.Contains(stderr, `"secret_delivered":false`) || !strings.Contains(stderr, `"remote_changed":true`) {
				t.Fatalf("delivery failure %d %d %s", code, calls, stderr)
			}
		})
	}
}

func TestAssetTransferLocalValidation(t *testing.T) {
	catalog := transferCatalog(t)
	t.Setenv("PLANE_CLI_API_KEY", "")
	t.Setenv("PLANE_CLI_BASE_URL", "http://127.0.0.1:1")
	file := transferFile(t, "payload", "bytes")
	for _, body := range []string{`[]`, `null`, `{"name":"override"}`, `{"size":99}`, `{"entity_type":"OTHER"}`, `{"entity_identifier":"other"}`} {
		args := append([]string{"workspace-page-attachment", "upload", "--api-version", "v1", "--file", file, "--body-file", transferFile(t, "body.json", body)}, transferParents("workspace-page-attachment")...)
		code, out, stderr := invoke(t, catalog, args, nil)
		if code != 2 || out != "" || !strings.Contains(stderr, `"code":"usage_error"`) {
			t.Fatalf("body=%s code=%d out=%s err=%s", body, code, out, stderr)
		}
	}
	destination := filepath.Join(t.TempDir(), "download")
	args := append([]string{"workspace-page-attachment", "download", "--api-version", "v2", "--id", transferID, "--output-file", destination, "--overwrite", "--dry-run"}, transferParents("workspace-page-attachment")...)
	code, out, stderr := invoke(t, catalog, args, nil)
	if code != 0 || stderr != "" || !strings.Contains(out, `"requires_confirmation":true`) {
		t.Fatalf("dry-run: %d %s %s", code, out, stderr)
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run created destination: %v", err)
	}
}
