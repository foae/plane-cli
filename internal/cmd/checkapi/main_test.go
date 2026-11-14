package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foae/plane-cli/internal/spec"
)

// fixture copies real reviewed inputs, so failures exercise reconciliation rather
// than a second hand-maintained catalog that could accidentally omit a boundary.
func fixture(t *testing.T) {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	for _, name := range []string{"api", "docs/reference", "internal/spec"} {
		if err := filepath.WalkDir(filepath.Join(root, name), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			target := filepath.Join(dest, rel)
			if entry.IsDir() {
				return os.MkdirAll(target, 0755)
			}
			if !strings.HasSuffix(path, ".json") && !strings.HasSuffix(path, ".tsv") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, data, 0600)
		}); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dest)
}

func rewrite(t *testing.T, path string, change func(map[string]any)) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	change(object)
	data, err = json.MarshalIndent(object, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	if path == "api/catalog.json" {
		// Keep generation current so each mutation reaches its intended coverage gate.
		generated, err := spec.Generate(data)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile("internal/spec/catalog.gen.json", generated, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPacketCatalog(t *testing.T) {
	fixture(t)
	if err := run("packet", "P00"); err != nil {
		t.Fatal(err)
	}
}

func TestRejectIncompleteRelease(t *testing.T) {
	fixture(t)
	rewrite(t, "api/catalog.json", func(c map[string]any) {
		completion := c["packet_completion"].(map[string]any)
		for _, packet := range []string{"P01", "P02", "P03", "P04", "P05", "P06", "P07", "P08", "P09", "P10"} {
			completion[packet] = map[string]any{"completed_operation_ids": []any{}, "status": "pending"}
		}
	})
	if err := run("release", "P00"); err == nil || !strings.Contains(err.Error(), "release coverage incomplete") {
		t.Fatalf("got %v", err)
	}
}

func TestRejectCoverageAndEvidenceFailures(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		change func(map[string]any)
		want   string
	}{
		{"missing-row", "api/catalog.json", func(c map[string]any) { c["sources"] = c["sources"].([]any)[1:] }, "uncovered index row"},
		{"missing-owner", "api/catalog.json", func(c map[string]any) { delete(c["family_owners"].(map[string]any), "projects") }, "family ownership mismatch"},
		{"invalid-source-hash", "api/provenance.json", func(c map[string]any) { c["sources"].([]any)[0].(map[string]any)["sha256"] = "not-a-hash" }, "malformed provenance"},
		{"invalid-evidence-hash", "api/provenance.json", func(c map[string]any) {
			c["sources"].([]any)[0].(map[string]any)["evidence_sha256"] = strings.Repeat("0", 64)
		}, "evidence hash mismatch"},
		{"missing-evidence-file", "api/provenance.json", func(c map[string]any) {
			c["sources"].([]any)[0].(map[string]any)["evidence_path"] = "api/sources/missing.json"
		}, "missing.json"},
		{"invented-verified-status", "api/catalog.json", func(c map[string]any) { c["operations"].([]any)[0].(map[string]any)["lifecycle"] = "verified" }, "lacks callable descriptor"},
		{"duplicate-operation", "api/catalog.json", func(c map[string]any) {
			ops := c["operations"].([]any)
			var op map[string]any
			b, _ := json.Marshal(ops[0])
			if err := json.Unmarshal(b, &op); err != nil {
				panic(err)
			}
			op["id"] = "duplicate-route"
			c["operations"] = append(ops, op)
		}, "duplicate route/command ownership"},
		{"remote-schema-reference", "api/catalog.json", func(c map[string]any) {
			c["operations"].([]any)[0].(map[string]any)["request_schema"] = map[string]any{"$ref": "https://example.invalid/schema.json"}
		}, "operation schema"},
		{"invalid-source-packet", "api/catalog.json", func(c map[string]any) {
			c["sources"].([]any)[0].(map[string]any)["packet"] = "P0z"
		}, "malformed source"},
		{"source-userinfo", "api/catalog.json", func(c map[string]any) {
			c["sources"].([]any)[0].(map[string]any)["url"] = "https://private@developers.plane.so/page"
		}, "malformed source"},
		{"null-sensitivity", "api/catalog.json", func(c map[string]any) {
			c["operations"].([]any)[0].(map[string]any)["sensitivity"] = nil
		}, "invalid sensitivity"},
		{"nonobject-sensitivity", "api/catalog.json", func(c map[string]any) {
			c["operations"].([]any)[0].(map[string]any)["sensitivity"] = []any{}
		}, "invalid sensitivity"},
		{"missing-sensitivity-field", "api/catalog.json", func(c map[string]any) {
			delete(c["operations"].([]any)[0].(map[string]any)["sensitivity"].(map[string]any), "input")
		}, "invalid sensitivity"},
		{"null-sensitivity-paths", "api/catalog.json", func(c map[string]any) {
			c["operations"].([]any)[0].(map[string]any)["sensitivity"].(map[string]any)["output"] = nil
		}, "invalid sensitivity"},
		{"invalid-sensitivity-delivery", "api/catalog.json", func(c map[string]any) {
			c["operations"].([]any)[0].(map[string]any)["sensitivity"].(map[string]any)["intentional_secret_delivery"] = "yes"
		}, "invalid sensitivity"},
		{"invalid-completion-status", "api/catalog.json", func(c map[string]any) {
			c["packet_completion"].(map[string]any)["P00"].(map[string]any)["status"] = "verified"
		}, "invalid completion status"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture(t)
			rewrite(t, test.path, test.change)
			if err := run("packet", "P00"); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want %q, got %v", test.want, err)
			}
		})
	}
}

func TestRejectStaleEmbeddedCatalog(t *testing.T) {
	fixture(t)
	if err := os.WriteFile("internal/spec/catalog.gen.json", []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run("packet", "P00"); err == nil || !strings.Contains(err.Error(), "stale embedded catalog") {
		t.Fatalf("got %v", err)
	}
}

func TestRejectVerificationWithoutApproval(t *testing.T) {
	fixture(t)
	// A real handler/descriptor link alone cannot certify an operation.
	rewrite(t, "api/catalog.json", func(c map[string]any) {
		command := c["commands"].([]any)[1].(map[string]any)
		command["local"] = false
		op := c["operations"].([]any)[0].(map[string]any)
		op["command_path"] = command["path"]
		op["lifecycle"] = "verified"
		op["verification"] = map[string]any{"operation": "api/sources/v1-project-get.json", "expected_fixtures": "api/sources/v1-project-get.json", "acceptance_command": "api/sources/v1-project-get.json", "acceptance_result": "api/sources/v1-project-get.json"}
	})
	if err := run("packet", "P00"); err == nil || !strings.Contains(err.Error(), "verification v1-project-get") {
		t.Fatalf("got %v", err)
	}
}

func TestNonOperationReleaseResolution(t *testing.T) {
	for _, disposition := range []string{"general-guide", "overview"} {
		t.Run(disposition, func(t *testing.T) {
			resolved := source{ID: "guide", Version: "v2", Disposition: disposition, EvidenceStatus: "reviewed", ReviewResolution: "non-operation", ProvenanceID: "reviewed-evidence"}
			if !resolvedSource(resolved, map[string]source{}, map[string]bool{}) {
				t.Fatal("explicitly reviewed non-operation must not require an invented API operation")
			}
			pending := resolved
			pending.EvidenceStatus = "pending"
			if resolvedSource(pending, map[string]source{}, map[string]bool{}) {
				t.Fatal("pending overview was excluded")
			}
			unresolved := resolved
			unresolved.ReviewResolution = ""
			if resolvedSource(unresolved, map[string]source{}, map[string]bool{}) {
				t.Fatal("overview without extraction resolution was excluded")
			}
			noEvidence := resolved
			noEvidence.ProvenanceID = ""
			if resolvedSource(noEvidence, map[string]source{}, map[string]bool{}) {
				t.Fatal("non-operation without evidence was excluded")
			}
		})
	}
	for _, disposition := range []string{"operation-source", "evidence-conflict", "supplemental-evidence", "duplicate-alias"} {
		s := source{ID: "unresolved", Version: "v2", Disposition: disposition, EvidenceStatus: "reviewed", ReviewResolution: "non-operation", ProvenanceID: "evidence"}
		if resolvedSource(s, map[string]source{}, map[string]bool{}) {
			t.Fatalf("unresolved %s was excluded", disposition)
		}
	}
}

func TestAliasRequiresResolvedTarget(t *testing.T) {
	target := source{ID: "canonical", Version: "v2", EvidenceStatus: "verified"}
	alias := source{ID: "alias", Version: "v2", Disposition: "duplicate-alias", EvidenceStatus: "reviewed", ReviewResolution: "duplicate-alias", ProvenanceID: "alias-evidence", AliasOf: "canonical"}
	sources := map[string]source{"canonical": target, "alias": alias}
	if !resolvedSource(alias, sources, map[string]bool{}) {
		t.Fatal("reviewed alias to verified canonical source should resolve without duplicate operation")
	}
	target.EvidenceStatus = "pending"
	sources["canonical"] = target
	if resolvedSource(alias, sources, map[string]bool{}) {
		t.Fatal("alias masked pending target")
	}
	target = alias
	target.ID = "canonical"
	target.AliasOf = "alias"
	sources["canonical"] = target
	if resolvedSource(alias, sources, map[string]bool{}) {
		t.Fatal("cyclic aliases masked missing coverage")
	}
}

func TestPacketAndURLBoundaries(t *testing.T) {
	for _, packet := range []string{"P00", "P01", "P09", "P10"} {
		if !validPacket(packet) {
			t.Fatalf("rejected %q", packet)
		}
	}
	for _, packet := range []string{"P0z", "P0:", "P0/", "P11", "P000", ""} {
		if validPacket(packet) {
			t.Fatalf("accepted %q", packet)
		}
	}
	if !validURL("https://developers.plane.so/page") || validURL("https://user:secret@developers.plane.so/page") {
		t.Fatal("official URL userinfo boundary not enforced")
	}
}

func TestEvidenceSymlinkContainment(t *testing.T) {
	fixture(t)
	outside := filepath.Join(t.TempDir(), "evidence.json")
	if err := os.WriteFile(outside, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, "api/sources/external.json"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("v1-project-get.json", "api/sources/internal.json"); err != nil {
		t.Fatal(err)
	}
	if err := localFile("api/sources/internal.json"); err != nil {
		t.Fatalf("repository-local symlink rejected: %v", err)
	}
	rewrite(t, "api/provenance.json", func(c map[string]any) {
		c["sources"].([]any)[0].(map[string]any)["evidence_path"] = "api/sources/external.json"
	})
	if err := run("packet", "P00"); err == nil || !strings.Contains(err.Error(), "escapes repository") {
		t.Fatalf("external evidence accepted: %v", err)
	}
}

func TestCompletionReconciliation(t *testing.T) {
	for _, mode := range []string{"packet", "release"} {
		t.Run(mode, func(t *testing.T) {
			fixture(t)
			rewrite(t, "api/catalog.json", func(c map[string]any) {
				command := c["commands"].([]any)[1].(map[string]any)
				command["local"] = false
				op := c["operations"].([]any)[0].(map[string]any)
				op["command_path"] = command["path"]
				op["lifecycle"] = "verified"
				evidence := "api/sources/v1-project-get.json"
				op["verification"] = map[string]any{"operation": evidence, "expected_fixtures": evidence, "acceptance_command": evidence, "acceptance_result": evidence, "planner_approval": evidence}
				// The selected packet remains P00 in release mode: P03 must still reconcile.
				if mode == "packet" {
					c["packet_completion"].(map[string]any)["P03"] = map[string]any{"completed_operation_ids": []any{}, "status": "pending"}
				} else {
					for _, packet := range []string{"P01", "P02", "P03", "P04", "P05", "P06", "P07", "P08", "P09", "P10"} {
						c["packet_completion"].(map[string]any)[packet] = map[string]any{"completed_operation_ids": []any{}, "status": "pending"}
					}
				}
			})
			packet := "P03"
			if mode == "release" {
				packet = "P00"
			}
			if err := run(mode, packet); err == nil || !strings.Contains(err.Error(), "verified operation missing from completed set") {
				t.Fatalf("omitted verified operation accepted: %v", err)
			}
			rewrite(t, "api/catalog.json", func(c map[string]any) {
				c["packet_completion"].(map[string]any)["P03"].(map[string]any)["completed_operation_ids"] = []any{"v1-project-get"}
			})
			if mode == "packet" {
				if err := run(mode, packet); err != nil {
					t.Fatalf("exact verified set rejected: %v", err)
				}
			} else if err := run(mode, packet); err == nil || !strings.Contains(err.Error(), "release coverage incomplete") {
				t.Fatalf("exact sets did not reach remaining release coverage gate: %v", err)
			}
		})
	}
}

func TestReleaseRequiresEveryCompletionDeclaration(t *testing.T) {
	fixture(t)
	if err := run("release", "P00"); err == nil || !strings.Contains(err.Error(), "packet completion declaration missing: P01") {
		t.Fatalf("missing release declaration accepted: %v", err)
	}
}
