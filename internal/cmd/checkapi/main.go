// Command checkapi checks offline catalog integrity without claiming API acceptance.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/foae/plane-cli/internal/catalogsource"
	"github.com/foae/plane-cli/internal/cli"
	"github.com/foae/plane-cli/internal/spec"
)

type source struct {
	ID                  string `json:"id"`
	Version             string `json:"version"`
	Family              string `json:"family"`
	Packet              string `json:"packet"`
	Disposition         string `json:"disposition"`
	EvidenceStatus      string `json:"evidence_status"`
	URL                 string `json:"url"`
	ReviewResolution    string `json:"review_resolution"`
	AliasOf             string `json:"alias_of"`
	ProvenanceID        string `json:"provenance_id"`
	CompletionException string `json:"completion_exception,omitempty"`
}

type operation struct {
	ID              string            `json:"id"`
	Packet          string            `json:"packet"`
	Version         string            `json:"version"`
	CommandPath     []string          `json:"command_path"`
	Scope           string            `json:"scope"`
	Method          string            `json:"method"`
	Action          string            `json:"action"`
	SideEffect      string            `json:"side_effect"`
	Pagination      string            `json:"pagination"`
	FeatureEvidence string            `json:"feature_evidence"`
	Availability    map[string]string `json:"availability"`
	Path            string            `json:"path"`
	Lifecycle       string            `json:"lifecycle"`
	ProvenanceID    string            `json:"provenance_id"`
	SourceURL       string            `json:"source_url"`
	RequestSchema   json.RawMessage   `json:"request_schema"`
	ResponseSchema  json.RawMessage   `json:"response_schema"`
	Sensitivity     json.RawMessage   `json:"sensitivity"`
	AcceptanceIDs   []string          `json:"acceptance_scenario_ids"`
	Responses       []struct {
		Status      int             `json:"status"`
		ContentType string          `json:"content_type"`
		Schema      json.RawMessage `json:"schema"`
	} `json:"responses"`
	ErrorContract struct {
		Schema       json.RawMessage `json:"schema"`
		ContentType  string          `json:"content_type"`
		ProvenanceID string          `json:"provenance_id"`
		Statuses     []int           `json:"documented_statuses"`
	} `json:"error_contract"`
	Evidence struct {
		Operation         string `json:"operation"`
		ExpectedFixtures  string `json:"expected_fixtures"`
		AcceptanceCommand string `json:"acceptance_command"`
		AcceptanceResult  string `json:"acceptance_result"`
		PlannerApproval   string `json:"planner_approval"`
	} `json:"verification"`
}

type provenanceRecord struct {
	ID             string `json:"id"`
	URL            string `json:"url"`
	FetchedAt      string `json:"fetched_at"`
	SourceRevision string `json:"source_revision"`
	SHA256         string `json:"sha256"`
	FetchedBytes   int    `json:"fetched_bytes"`
	Permission     string `json:"permission"`
	EvidencePath   string `json:"evidence_path"`
	EvidenceSHA256 string `json:"evidence_sha256"`
	EvidenceKind   string `json:"evidence_kind"`
}

type catalog struct {
	SchemaVersion int                        `json:"schema_version"`
	Sources       []source                   `json:"sources"`
	Operations    []operation                `json:"operations"`
	Resources     map[string]json.RawMessage `json:"resources"`
	FamilyOwners  map[string]string          `json:"family_owners"`
	Supplements   []struct {
		Family string `json:"family"`
		Packet string `json:"packet"`
	} `json:"supplemental_families"`
	Completion map[string]struct {
		IDs    []string `json:"completed_operation_ids"`
		Status string   `json:"status"`
	} `json:"packet_completion"`
}

func main() {
	mode := flag.String("mode", "packet", "packet or release; metadata contract: api/README.md")
	packet := flag.String("packet", "P00", "Packet to check")
	release := flag.Bool("release", false, "Require complete verified coverage")
	flag.Parse()
	if *release {
		*mode = "release"
	}
	if err := catalogsource.Check("api/catalog", "api/catalog.json"); err != nil {
		fmt.Fprintln(os.Stderr, "checkapi:", err)
		os.Exit(1)
	}
	if err := run(*mode, *packet); err != nil {
		fmt.Fprintln(os.Stderr, "checkapi:", err)
		os.Exit(1)
	}
}

func readJSON(path string, target any) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Repository metadata is not a runtime request body and grows with coverage.
	if _, err := spec.DecodeWithLimit(data, len(data)); err != nil {
		return nil, fmt.Errorf("%s: invalid or duplicate-key JSON", path)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(target); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: trailing JSON", path)
	}
	return data, nil
}

func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func validHash(value string) bool {
	b, err := hex.DecodeString(value)
	return err == nil && len(b) == sha256.Size
}
func validURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && u.Scheme == "https" && u.Host == "developers.plane.so" && u.User == nil
}
func validPacket(packet string) bool {
	return len(packet) == 3 && packet[0] == 'P' && packet[1] >= '0' && packet[1] <= '1' &&
		packet[2] >= '0' && packet[2] <= '9' && packet <= "P10"
}

// Guides and overviews may close without invented operations only after an
// explicit reviewed non-operation resolution backed by retained provenance.
func resolvedNonOperation(s source) bool {
	return (s.Disposition == "general-guide" || s.Disposition == "overview") &&
		s.EvidenceStatus == "reviewed" && s.ReviewResolution == "non-operation" &&
		s.ProvenanceID != ""
}

func resolvedSource(s source, sources map[string]source, visiting map[string]bool) bool {
	if s.EvidenceStatus == "verified" || resolvedNonOperation(s) {
		return true
	}
	// Approved upstream gaps close source reconciliation, not API coverage.
	if s.CompletionException != "" && s.ProvenanceID != "" &&
		(s.EvidenceStatus == "reviewed" || s.EvidenceStatus == "reviewed-partial") {
		switch s.Disposition {
		case "operation-source", "evidence-conflict", "supplemental-evidence":
			return true
		}
	}
	if s.Disposition != "duplicate-alias" || s.EvidenceStatus != "reviewed" ||
		s.ReviewResolution != "duplicate-alias" || s.ProvenanceID == "" || visiting[s.ID] {
		return false
	}
	target, ok := sources[s.AliasOf]
	if !ok || target.ID == s.ID || target.Version != s.Version {
		return false
	}
	visiting[s.ID] = true
	defer delete(visiting, s.ID)
	return resolvedSource(target, sources, visiting)
}
func localFile(path string) error {
	if path == "" || filepath.IsAbs(path) || filepath.Clean(path) == ".." || strings.HasPrefix(filepath.Clean(path), ".."+string(filepath.Separator)) {
		return fmt.Errorf("invalid evidence path %q", path)
	}
	root, err := filepath.EvalSymlinks(".")
	if err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("evidence path escapes repository: %q", path)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular evidence file: %s", path)
	}
	return nil
}

// Sensitivity is an explicit review contract, not merely a present JSON value.
func validSensitivity(raw json.RawMessage) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return false
	}
	for _, name := range []string{"review", "extension_fields"} {
		var value string
		if json.Unmarshal(fields[name], &value) != nil || strings.TrimSpace(value) == "" {
			return false
		}
	}
	var delivery *bool
	if json.Unmarshal(fields["intentional_secret_delivery"], &delivery) != nil || delivery == nil {
		return false
	}
	for _, name := range []string{"input", "output", "ordinary_read_redaction", "signed_target_paths", "signed_form_paths"} {
		var paths []string
		if json.Unmarshal(fields[name], &paths) != nil || paths == nil {
			return false
		}
		for _, path := range paths {
			if !strings.HasPrefix(path, "/") {
				return false
			}
		}
	}
	return true
}

// scenarioValue evaluates only static test names and table rows, never test code.
// Unsupported expressions remain unknown, so they cannot certify a reference.
func scenarioValue(expr ast.Expr, env map[string]any, expected ast.Expr) any {
	switch expr := expr.(type) {
	case *ast.BasicLit:
		if expr.Kind == token.STRING {
			value, err := strconv.Unquote(expr.Value)
			if err == nil {
				return value
			}
		}
	case *ast.Ident:
		return env[expr.Name]
	case *ast.SelectorExpr:
		if row, ok := scenarioValue(expr.X, env, nil).(map[string]any); ok {
			return row[expr.Sel.Name]
		}
	case *ast.BinaryExpr:
		left, lok := scenarioValue(expr.X, env, nil).(string)
		right, rok := scenarioValue(expr.Y, env, nil).(string)
		if lok && rok {
			switch expr.Op {
			case token.ADD:
				return left + right
			case token.EQL:
				return left == right
			case token.NEQ:
				return left != right
			}
		}
	case *ast.CompositeLit:
		typ := expr.Type
		if typ == nil {
			typ = expected
		}
		if name, ok := typ.(*ast.Ident); ok {
			typ, _ = env[name.Name].(ast.Expr)
		}
		if array, ok := typ.(*ast.ArrayType); ok {
			var values []any
			for _, elt := range expr.Elts {
				values = append(values, scenarioValue(elt, env, array.Elt))
			}
			return values
		}
		if record, ok := typ.(*ast.StructType); ok {
			var fields []string
			for _, field := range record.Fields.List {
				for _, name := range field.Names {
					fields = append(fields, name.Name)
				}
			}
			row := map[string]any{}
			for i, elt := range expr.Elts {
				if pair, ok := elt.(*ast.KeyValueExpr); ok {
					if key, ok := pair.Key.(*ast.Ident); ok {
						row[key.Name] = scenarioValue(pair.Value, env, nil)
					}
				} else if i < len(fields) {
					row[fields[i]] = scenarioValue(elt, env, nil)
				}
			}
			return row
		}
	case *ast.CallExpr:
		if name, ok := expr.Fun.(*ast.Ident); ok && name.Name == "append" && len(expr.Args) > 1 {
			values, ok := scenarioValue(expr.Args[0], env, nil).([]any)
			if !ok {
				return nil
			}
			values = append([]any(nil), values...)
			for _, arg := range expr.Args[1:] {
				values = append(values, scenarioValue(arg, env, nil))
			}
			return values
		}
	}
	return nil
}

// collectScenarios supports literal and table-driven t.Run names, including
// conditional append tables and slash-containing names used by the catalog.
func collectScenarios(statements []ast.Stmt, env map[string]any, receiver, prefix string, names map[string]bool) {
	for _, statement := range statements {
		switch statement := statement.(type) {
		case *ast.DeclStmt:
			if declaration, ok := statement.Decl.(*ast.GenDecl); ok {
				for _, declaration := range declaration.Specs {
					switch declaration := declaration.(type) {
					case *ast.TypeSpec:
						env[declaration.Name.Name] = declaration.Type
					case *ast.ValueSpec:
						for i, name := range declaration.Names {
							if i < len(declaration.Values) {
								env[name.Name] = scenarioValue(declaration.Values[i], env, declaration.Type)
							}
						}
					}
				}
			}
		case *ast.AssignStmt:
			for i, target := range statement.Lhs {
				if name, ok := target.(*ast.Ident); ok && i < len(statement.Rhs) {
					env[name.Name] = scenarioValue(statement.Rhs[i], env, nil)
				}
			}
		case *ast.RangeStmt:
			values, _ := scenarioValue(statement.X, env, nil).([]any)
			for _, value := range values {
				child := make(map[string]any, len(env)+1)
				for key, value := range env {
					child[key] = value
				}
				if name, ok := statement.Value.(*ast.Ident); ok {
					child[name.Name] = value
				}
				collectScenarios(statement.Body.List, child, receiver, prefix, names)
			}
		case *ast.IfStmt:
			if condition, ok := scenarioValue(statement.Cond, env, nil).(bool); ok {
				if condition {
					collectScenarios(statement.Body.List, env, receiver, prefix, names)
				} else if block, ok := statement.Else.(*ast.BlockStmt); ok {
					collectScenarios(block.List, env, receiver, prefix, names)
				}
			}
		case *ast.ExprStmt:
			call, ok := statement.X.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				continue
			}
			method, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || method.Sel.Name != "Run" {
				continue
			}
			target, ok := method.X.(*ast.Ident)
			if !ok || target.Name != receiver {
				continue
			}
			name, ok := scenarioValue(call.Args[0], env, nil).(string)
			callback, callbackOK := call.Args[1].(*ast.FuncLit)
			if !ok || name == "" || !callbackOK || len(callback.Type.Params.List) != 1 || len(callback.Type.Params.List[0].Names) != 1 {
				continue
			}
			full := prefix + "/" + name
			names[full] = true
			child := make(map[string]any, len(env))
			for key, value := range env {
				child[key] = value
			}
			collectScenarios(callback.Body.List, child, callback.Type.Params.List[0].Names[0].Name, full, names)
		}
	}
}

func acceptanceScenarios(root string) (map[string]bool, error) {
	names := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" || entry.Name() == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		testingName := ""
		for _, imported := range file.Imports {
			if imported.Path.Value == `"testing"` {
				testingName = "testing"
				if imported.Name != nil {
					testingName = imported.Name.Name
				}
			}
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || function.Body == nil || !strings.HasPrefix(function.Name.Name, "Test") || len(function.Name.Name) == 4 || (function.Name.Name[4] >= 'a' && function.Name.Name[4] <= 'z') || function.Type.Results != nil || len(function.Type.Params.List) != 1 {
				continue
			}
			parameter := function.Type.Params.List[0]
			pointer, ok := parameter.Type.(*ast.StarExpr)
			if !ok || len(parameter.Names) != 1 {
				continue
			}
			typ, ok := pointer.X.(*ast.SelectorExpr)
			if !ok || typ.Sel.Name != "T" {
				continue
			}
			pkg, ok := typ.X.(*ast.Ident)
			if !ok || testingName == "" || pkg.Name != testingName {
				continue
			}
			names[function.Name.Name] = true
			collectScenarios(function.Body.List, map[string]any{}, parameter.Names[0].Name, function.Name.Name, names)
		}
		return nil
	})
	return names, err
}

func run(mode, packet string) (result error) {
	if mode != "packet" && mode != "release" {
		return fmt.Errorf("unknown mode %q", mode)
	}
	if !validPacket(packet) {
		return fmt.Errorf("invalid packet %q", packet)
	}
	var c catalog
	raw, err := readJSON("api/catalog.json", &c)
	if err != nil {
		return err
	}
	if c.SchemaVersion != 1 {
		return errors.New("catalog schema_version must be 1")
	}
	generated, err := spec.Generate(raw)
	if err != nil {
		return err
	}
	// Materialize regeneration outside the source tree before comparing exact bytes.
	dir, err := os.MkdirTemp("", "plane-checkapi-*")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, os.RemoveAll(dir)) }()
	path := filepath.Join(dir, "catalog.gen.json")
	if err := os.WriteFile(path, generated, 0600); err != nil {
		return err
	}
	generated, err = os.ReadFile(path)
	if err != nil {
		return err
	}
	committed, err := os.ReadFile("internal/spec/catalog.gen.json")
	if err != nil {
		return err
	}
	if !bytes.Equal(generated, committed) {
		return errors.New("stale embedded catalog; run go run ./internal/cmd/genspec")
	}
	var public spec.Catalog
	if err := json.Unmarshal(generated, &public); err != nil {
		return err
	}
	if err := spec.CheckCatalog(public); err != nil {
		return err
	}
	for uri, schema := range c.Resources {
		if err := spec.Compile(schema, c.Resources); err != nil {
			return fmt.Errorf("resource %s: %w", uri, err)
		}
	}
	handlers := map[string]bool{}
	for _, id := range cli.HandlerIDs() {
		if handlers[id] {
			return fmt.Errorf("duplicate registered handler %s", id)
		}
		handlers[id] = true
	}
	for _, command := range public.Commands {
		if !handlers[command.Handler] {
			return fmt.Errorf("descriptor without registered handler: %s", command.Handler)
		}
		delete(handlers, command.Handler)
		for _, variant := range command.Versions {
			if err := spec.Compile(variant.Schema, c.Resources); err != nil {
				return fmt.Errorf("command schema %v: %w", command.Path, err)
			}
		}
	}
	if len(handlers) != 0 {
		return fmt.Errorf("registered handlers without descriptors: %v", handlers)
	}
	var p struct {
		SchemaVersion int `json:"schema_version"`
		Index         struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
			Rows   int    `json:"rows"`
		} `json:"index"`
		Sources []provenanceRecord `json:"sources"`
	}
	if _, err := readJSON("api/provenance.json", &p); err != nil {
		return err
	}
	if p.SchemaVersion != 1 {
		return errors.New("provenance schema_version must be 1")
	}
	if p.Index.Path != "docs/reference/api-index.tsv" {
		return errors.New("unexpected source index path")
	}
	index, err := os.ReadFile(p.Index.Path)
	if err != nil {
		return err
	}
	if digest(index) != p.Index.SHA256 {
		return errors.New("source index hash mismatch")
	}
	provenance := map[string]provenanceRecord{}
	for _, record := range p.Sources {
		if record.ID == "" || provenance[record.ID].ID != "" {
			return fmt.Errorf("empty/duplicate provenance ID %q", record.ID)
		}
		if !validURL(record.URL) || !validHash(record.SHA256) || !validHash(record.EvidenceSHA256) || record.FetchedBytes <= 0 || record.FetchedAt == "" || record.SourceRevision == "" || record.Permission == "" || record.EvidenceKind == "" {
			return fmt.Errorf("malformed provenance %s", record.ID)
		}
		if err := localFile(record.EvidencePath); err != nil {
			return err
		}
		evidence, err := os.ReadFile(record.EvidencePath)
		if err != nil {
			return err
		}
		if digest(evidence) != record.EvidenceSHA256 {
			return fmt.Errorf("evidence hash mismatch: %s", record.ID)
		}
		var document struct {
			SourceURL string `json:"source_url"`
		}
		if err := json.Unmarshal(evidence, &document); err != nil {
			return err
		}
		if document.SourceURL != record.URL {
			return fmt.Errorf("evidence URL mismatch: %s", record.ID)
		}
		provenance[record.ID] = record
	}
	sources := map[string]source{}
	pending := map[string]int{}
	for _, s := range c.Sources {
		if s.ID == "" || sources[s.ID].ID != "" {
			return fmt.Errorf("empty/duplicate source ID %q", s.ID)
		}
		if (s.Version != "v1" && s.Version != "v2") || !validPacket(s.Packet) || s.Family == "" || !validURL(s.URL) {
			return fmt.Errorf("malformed source %s", s.ID)
		}
		switch s.Disposition {
		case "operation-source", "general-guide", "overview", "duplicate-alias", "evidence-conflict", "supplemental-evidence":
		default:
			return fmt.Errorf("unknown disposition %s", s.ID)
		}
		switch s.EvidenceStatus {
		case "pending", "reviewed-partial", "reviewed", "verified":
		default:
			return fmt.Errorf("unknown evidence status %s", s.ID)
		}
		if s.ProvenanceID != "" {
			r, ok := provenance[s.ProvenanceID]
			if !ok || r.URL != s.URL {
				return fmt.Errorf("source provenance mismatch: %s", s.ID)
			}
		} else if s.EvidenceStatus != "pending" {
			return fmt.Errorf("source evidence missing: %s", s.ID)
		}
		if s.CompletionException != "" {
			if s.ReviewResolution == "" || (s.EvidenceStatus != "reviewed" && s.EvidenceStatus != "reviewed-partial") {
				return fmt.Errorf("unreviewed completion exception: %s", s.ID)
			}
			switch s.Disposition {
			case "operation-source", "evidence-conflict", "supplemental-evidence":
			default:
				return fmt.Errorf("invalid completion exception disposition: %s", s.ID)
			}
			if err := localFile(s.CompletionException); err != nil {
				return fmt.Errorf("completion exception %s: %w", s.ID, err)
			}
		}
		sources[s.ID] = s
	}
	for _, s := range c.Sources {
		if !resolvedSource(s, sources, map[string]bool{}) {
			pending[s.Packet]++
		}
	}
	reader := csv.NewReader(bytes.NewReader(index))
	reader.Comma = '\t'
	reader.FieldsPerRecord = 5
	if _, err := reader.Read(); err != nil {
		return err
	}
	seen := map[string]bool{}
	count := 0
	families := map[string]bool{}
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		id := strings.Join(row[:3], ":")
		if seen[id] {
			return fmt.Errorf("duplicate index row: %s", id)
		}
		seen[id] = true
		s, ok := sources[id]
		if !ok || s.URL != row[4] || s.Version != row[0] || s.Family != row[1] {
			return fmt.Errorf("uncovered index row: %s", id)
		}
		if c.FamilyOwners[row[1]] != s.Packet {
			return fmt.Errorf("family ownership mismatch: %s", id)
		}
		families[row[1]] = true
		count++
	}
	if count != p.Index.Rows {
		return fmt.Errorf("index row count mismatch: %d", count)
	}
	if len(families) != len(c.FamilyOwners) {
		return errors.New("unused or missing family ownership entries")
	}
	for family, owner := range c.FamilyOwners {
		if !families[family] || !validPacket(owner) {
			return fmt.Errorf("invalid family owner %s", family)
		}
	}
	requiredSupplements := map[string]string{"ai-agents-runs-activities": "P09", "integration-jobs": "P09", "server-upload-variants": "P06", "core-searches-grouped-counts-lite-lists": "P03", "scheduling-collaboration-equivalents": "P04", "approvals-state-transfers": "P09"}
	for _, supplement := range c.Supplements {
		if requiredSupplements[supplement.Family] != supplement.Packet {
			return fmt.Errorf("unexpected supplemental family %s", supplement.Family)
		}
		id := "supplement:v1:" + supplement.Family
		s, ok := sources[id]
		if !ok || s.Packet != supplement.Packet || s.Disposition != "supplemental-evidence" {
			return fmt.Errorf("missing supplemental owner: %s", supplement.Family)
		}
		seen[id] = true
		delete(requiredSupplements, supplement.Family)
	}
	if len(requiredSupplements) != 0 {
		return errors.New("required supplemental families missing")
	}
	if len(seen) != len(sources) {
		return errors.New("source ledger contains entries outside index and declared supplements")
	}
	opIDs := map[string]operation{}
	routes := map[string]bool{}
	mappings := map[string]bool{}
	verified := []string{}
	scenarios, err := acceptanceScenarios(".")
	if err != nil {
		return fmt.Errorf("acceptance scenarios: %w", err)
	}
	for _, op := range c.Operations {
		if op.ID == "" || opIDs[op.ID].ID != "" {
			return fmt.Errorf("duplicate/empty operation ID %q", op.ID)
		}
		apiPath := strings.HasPrefix(op.Path, "/api/"+op.Version+"/") && strings.HasSuffix(op.Path, "/")
		// Storage targets come from signed credentials, never from an invented API route.
		storagePath := op.Scope == "storage" && op.Path == "{upload_url}" && op.Method == "POST" && op.Action == "upload-file"
		if !validPacket(op.Packet) || (op.Version != "v1" && op.Version != "v2") || len(op.CommandPath) == 0 || op.Scope == "" || (!apiPath && !storagePath) {
			return fmt.Errorf("malformed operation %s", op.ID)
		}
		switch op.Method {
		case "GET", "POST", "PUT", "PATCH", "DELETE":
		default:
			return fmt.Errorf("invalid method %s", op.ID)
		}
		key := op.Version + " " + op.Method + " " + op.Path
		mapping := op.Version + " " + op.Scope + " " + strings.Join(op.CommandPath, " ")
		if routes[key] || mappings[mapping] {
			return fmt.Errorf("duplicate route/command ownership: %s", op.ID)
		}
		routes[key] = true
		mappings[mapping] = true
		r, ok := provenance[op.ProvenanceID]
		if !ok || r.URL != op.SourceURL {
			return fmt.Errorf("missing operation provenance: %s", op.ID)
		}
		if !validSensitivity(op.Sensitivity) || len(op.AcceptanceIDs) == 0 {
			return fmt.Errorf("invalid sensitivity/scenarios: %s", op.ID)
		}
		// Documented operations may name planned acceptance; implementation and
		// verification must instead link runnable repository test scenarios.
		if op.Lifecycle == "implemented" || op.Lifecycle == "verified" {
			for _, id := range op.AcceptanceIDs {
				if !scenarios[id] {
					return fmt.Errorf("unknown acceptance scenario %q: %s", id, op.ID)
				}
			}
		}
		if op.Action == "" || op.SideEffect == "" || op.Pagination == "" || op.FeatureEvidence == "" || op.Availability[op.Version] == "" {
			return fmt.Errorf("missing required operation metadata: %s", op.ID)
		}
		for _, schema := range []json.RawMessage{op.RequestSchema, op.ResponseSchema} {
			if err := spec.Compile(schema, c.Resources); err != nil {
				return fmt.Errorf("operation schema %s: %w", op.ID, err)
			}
		}
		if op.Version == "v2" {
			if op.ErrorContract.ContentType != "application/problem+json" || len(op.ErrorContract.Statuses) == 0 || provenance[op.ErrorContract.ProvenanceID].ID == "" {
				return fmt.Errorf("missing v2 error evidence: %s", op.ID)
			}
			if err := spec.Compile(op.ErrorContract.Schema, c.Resources); err != nil {
				return fmt.Errorf("error schema %s: %w", op.ID, err)
			}
		}
		if len(op.Responses) == 0 {
			return fmt.Errorf("missing response contracts: %s", op.ID)
		}
		responseStatuses := map[int]bool{}
		for _, response := range op.Responses {
			if response.Status < 100 || response.Status > 599 || (response.ContentType == "" && response.Status != 204) || responseStatuses[response.Status] {
				return fmt.Errorf("invalid response contract: %s", op.ID)
			}
			responseStatuses[response.Status] = true
			if err := spec.Compile(response.Schema, c.Resources); err != nil {
				return fmt.Errorf("response schema %s: %w", op.ID, err)
			}
		}
		ownedSource := false
		for _, s := range c.Sources {
			if s.URL == op.SourceURL && s.Packet == op.Packet && s.Version == op.Version {
				ownedSource = true
			}
		}
		if !ownedSource {
			return fmt.Errorf("operation lacks owned source row: %s", op.ID)
		}
		switch op.Lifecycle {
		case "documented":
		case "implemented", "verified":
			found := false
			for _, command := range public.Commands {
				if !command.Local && strings.Join(command.Path, " ") == strings.Join(op.CommandPath, " ") {
					_, found = command.Versions[op.Version]
				}
			}
			if !found {
				return fmt.Errorf("implemented operation lacks callable descriptor: %s", op.ID)
			}
		default:
			return fmt.Errorf("invalid lifecycle %s", op.ID)
		}
		if op.Lifecycle == "verified" {
			for _, path := range []string{op.Evidence.Operation, op.Evidence.ExpectedFixtures, op.Evidence.AcceptanceCommand, op.Evidence.AcceptanceResult, op.Evidence.PlannerApproval} {
				if err := localFile(path); err != nil {
					return fmt.Errorf("verification %s: %w", op.ID, err)
				}
			}
			verified = append(verified, op.ID)
		}
		opIDs[op.ID] = op
	}
	for _, s := range c.Sources {
		if s.EvidenceStatus != "verified" {
			continue
		}
		matched := false
		for _, op := range c.Operations {
			if op.SourceURL != s.URL {
				continue
			}
			matched = true
			if op.Lifecycle != "verified" {
				return fmt.Errorf("source claims verified before operation: %s", s.ID)
			}
		}
		if !matched {
			return fmt.Errorf("verified source lacks operation evidence: %s", s.ID)
		}
	}
	checkPackets := []string{packet}
	if mode == "release" {
		checkPackets = nil
		for i := 0; i <= 10; i++ {
			checkPackets = append(checkPackets, fmt.Sprintf("P%02d", i))
		}
	}
	for key := range c.Completion {
		if !validPacket(key) {
			return fmt.Errorf("invalid completion packet %q", key)
		}
	}
	for _, selected := range checkPackets {
		completion, ok := c.Completion[selected]
		if !ok {
			return fmt.Errorf("packet completion declaration missing: %s", selected)
		}
		if strings.TrimSpace(completion.Status) == "" || (selected == "P00" && completion.Status != "metadata-only") {
			return fmt.Errorf("invalid completion status: %s", selected)
		}
		declared := map[string]bool{}
		for _, id := range completion.IDs {
			op, ok := opIDs[id]
			if !ok || op.Packet != selected || op.Lifecycle != "verified" || declared[id] {
				return fmt.Errorf("invalid completed operation %s", id)
			}
			declared[id] = true
		}
		for _, op := range c.Operations {
			if op.Packet == selected && op.Lifecycle == "verified" && !declared[op.ID] {
				return fmt.Errorf("verified operation missing from completed set: %s", op.ID)
			}
		}
	}
	completed := append([]string{}, c.Completion[packet].IDs...)
	sort.Strings(completed)
	if packet == "P00" && len(completed) != 0 {
		return errors.New("P00 must declare zero completed API operations")
	}
	packets := make([]string, 0, len(pending))
	total := 0
	for p := range pending {
		packets = append(packets, p)
	}
	sort.Strings(packets)
	fmt.Printf("packet=%s completed_api_operations=%d ids=%v indexed_sources=%d supplemental_sources=%d documented_operations=%d verified_operations=%d\n", packet, len(completed), completed, count, len(c.Supplements), len(c.Operations), len(verified))
	exceptions := 0
	for _, s := range c.Sources {
		if s.CompletionException != "" {
			exceptions++
		}
	}
	fmt.Printf("approved_upstream_source_exceptions=%d (not implemented API operations)\n", exceptions)
	for _, p := range packets {
		fmt.Printf("pending_sources packet=%s count=%d\n", p, pending[p])
		total += pending[p]
	}
	if mode == "release" && (total != 0 || len(verified) != len(c.Operations)) {
		return fmt.Errorf("release coverage incomplete: %d pending sources; %d/%d API operations verified", total, len(verified), len(c.Operations))
	}
	return nil
}
