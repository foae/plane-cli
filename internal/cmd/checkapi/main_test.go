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
	for _, name := range []string{"api", "docs", "internal", "Makefile"} {
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
			if rel != "Makefile" && !strings.HasSuffix(path, ".json") && !strings.HasSuffix(path, ".tsv") && !strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".md") {
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

func TestRejectUnknownAcceptanceScenario(t *testing.T) {
	fixture(t)
	setScenario := func(id string) {
		t.Helper()
		rewrite(t, "api/catalog.json", func(c map[string]any) {
			for _, value := range c["operations"].([]any) {
				op := value.(map[string]any)
				if op["lifecycle"] == "implemented" || op["lifecycle"] == "verified" {
					op["acceptance_scenario_ids"] = []any{id}
					return
				}
			}
			t.Fatal("fixture has no callable operation")
		})
	}
	for _, id := range []string{"TestDoesNotExist", "TestProjectWireOperations/v1/does-not-exist", "TestProjectWireOperations/v1/bulk-create", ""} {
		setScenario(id)
		if err := run("packet", "P00"); err == nil || !strings.Contains(err.Error(), "unknown acceptance scenario") {
			t.Fatalf("reference %q: got %v", id, err)
		}
	}
	setScenario("TestProjectWireOperations/v2/bulk-create")
	if err := run("packet", "P00"); err != nil {
		t.Fatalf("real scenario rejected: %v", err)
	}
}

func TestAcceptanceScenarioDiscovery(t *testing.T) {
	root := t.TempDir()
	source := `package scenarios
import "testing"
func TestReal(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		cases := []struct { action string }{{"get"}}
		if version == "v1" {
			cases = append(cases, struct { action string }{"legacy"})
		} else {
			cases = append(cases, struct { action string }{"bulk"})
		}
		for _, row := range cases {
			t.Run(version+"/"+row.action, func(t *testing.T) {
				t.Run("nested", func(t *testing.T) {})
			})
		}
	}
	rows := []struct { version, scope, action string }{{"v2", "", "list"}}
	for _, row := range rows {
		t.Run(row.version+"/"+row.scope+"/"+row.action, func(t *testing.T) {})
	}
	t.Run("literal", func(t *testing.T) {})
}
func Testhelper() {}
`
	if err := os.WriteFile(filepath.Join(root, "scenario_test.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	// An ordinary source file and a non-test helper are not runnable scenarios.
	if err := os.WriteFile(filepath.Join(root, "ordinary.go"), []byte("package scenarios\nfunc TestNotATestFile() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	names, err := acceptanceScenarios(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"TestReal", "TestReal/v1/get", "TestReal/v2/get", "TestReal/v1/legacy", "TestReal/v2/bulk", "TestReal/v2/bulk/nested", "TestReal/v2//list", "TestReal/literal"} {
		if !names[name] {
			t.Errorf("real scenario missing: %q", name)
		}
	}
	for _, name := range []string{"TestBogus", "Testhelper", "TestNotATestFile", "TestReal/bogus", "TestReal/v1/bulk", "TestReal/v2/legacy", "TestReal/v2//bogus"} {
		if names[name] {
			t.Errorf("bogus scenario discovered: %q", name)
		}
	}
}

func TestRejectIncompleteRelease(t *testing.T) {
	fixture(t)
	rewrite(t, "api/catalog.json", func(c map[string]any) {
		for _, value := range c["sources"].([]any) {
			source := value.(map[string]any)
			if source["evidence_status"] == "verified" {
				source["evidence_status"] = "reviewed"
				return
			}
		}
		t.Fatal("fixture has no verified source")
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

func TestRejectUnsubstantiatedVerification(t *testing.T) {
	fixture(t)
	rewrite(t, "api/catalog.json", func(c map[string]any) {
		op := c["operations"].([]any)[0].(map[string]any)
		op["lifecycle"] = "verified"
		op["acceptance_scenario_ids"] = []any{"TestPacketCatalog"}
		delete(op, "verification")
	})
	if err := run("packet", "P00"); err == nil {
		t.Fatal("accepted verified status without verification evidence")
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
		command := discoveryCommand(c)
		command["local"] = false
		op := c["operations"].([]any)[0].(map[string]any)
		op["command_path"] = command["path"]
		op["lifecycle"] = "verified"
		op["acceptance_scenario_ids"] = []any{"TestPacketCatalog"}
		op["verification"] = map[string]any{"operation": "api/sources/v1-project-get.json", "expected_fixtures": "api/sources/v1-project-get.json", "acceptance_command": "api/sources/v1-project-get.json", "acceptance_result": "api/sources/v1-project-get.json"}
	})
	if err := run("packet", "P00"); err == nil || !strings.Contains(err.Error(), "verification ") {
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

func TestApprovedUpstreamException(t *testing.T) {
	for _, disposition := range []string{"operation-source", "evidence-conflict", "supplemental-evidence"} {
		s := source{ID: "gap", Disposition: disposition, EvidenceStatus: "reviewed-partial", ProvenanceID: "evidence", ReviewResolution: "upstream-contract-gap", CompletionException: "docs/reference/api-contract-gaps.md"}
		if !resolvedSource(s, nil, map[string]bool{}) {
			t.Fatalf("documented %s exception did not close reconciliation", disposition)
		}
		s.CompletionException = ""
		if resolvedSource(s, nil, map[string]bool{}) {
			t.Fatal("undocumented gap closed reconciliation")
		}
		s.CompletionException = "docs/reference/api-contract-gaps.md"
		s.EvidenceStatus = "pending"
		if resolvedSource(s, nil, map[string]bool{}) {
			t.Fatal("unreviewed source closed reconciliation")
		}
	}
}

func TestRejectMissingExceptionDocument(t *testing.T) {
	fixture(t)
	rewrite(t, "api/catalog.json", func(c map[string]any) {
		for _, value := range c["sources"].([]any) {
			s := value.(map[string]any)
			if s["disposition"] == "operation-source" && s["provenance_id"] != nil {
				s["evidence_status"] = "reviewed"
				s["review_resolution"] = "upstream-contract-gap"
				s["completion_exception"] = "docs/missing-exception.md"
				return
			}
		}
		t.Fatal("fixture has no reviewed operation source")
	})
	if err := run("packet", "P00"); err == nil || !strings.Contains(err.Error(), "completion exception") {
		t.Fatalf("missing exception document accepted: %v", err)
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
			var completed []any
			rewrite(t, "api/catalog.json", func(c map[string]any) {
				completion := c["packet_completion"].(map[string]any)["P03"].(map[string]any)
				completed = completion["completed_operation_ids"].([]any)
				if len(completed) == 0 {
					t.Fatal("fixture has no completed P03 operations")
				}
				// Release mode selects P00 but must still reconcile P03.
				completion["completed_operation_ids"] = completed[1:]
			})
			packet := "P03"
			if mode == "release" {
				packet = "P00"
			}
			if err := run(mode, packet); err == nil || !strings.Contains(err.Error(), "verified operation missing from completed set") {
				t.Fatalf("omitted verified operation accepted: %v", err)
			}
			rewrite(t, "api/catalog.json", func(c map[string]any) {
				c["packet_completion"].(map[string]any)["P03"].(map[string]any)["completed_operation_ids"] = completed
			})
			if err := run(mode, packet); err != nil {
				t.Fatalf("exact verified set rejected: %v", err)
			}
		})
	}
}

func TestReleaseRequiresEveryCompletionDeclaration(t *testing.T) {
	fixture(t)
	rewrite(t, "api/catalog.json", func(c map[string]any) {
		delete(c["packet_completion"].(map[string]any), "P01")
	})
	if err := run("release", "P00"); err == nil || !strings.Contains(err.Error(), "packet completion declaration missing: P01") {
		t.Fatalf("missing release declaration accepted: %v", err)
	}
}

func discoveryCommand(c map[string]any) map[string]any {
	for _, value := range c["commands"].([]any) {
		command := value.(map[string]any)
		if command["handler"] == "commands" {
			return command
		}
	}
	panic("fixture has no commands descriptor")
}
