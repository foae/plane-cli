package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/foae/plane-cli/internal/output"
	"github.com/foae/plane-cli/internal/spec"
	"github.com/spf13/cobra"
)

func invoke(t *testing.T, c spec.Catalog, args []string, handlers map[string]handler) (int, string, string) {
	t.Helper()
	var out, stderr bytes.Buffer
	code := run(context.Background(), args, Streams{In: strings.NewReader(""), Out: &out, Err: &stderr}, Build{Version: "test"}, c, handlers)
	return code, out.String(), stderr.String()
}

func fixtureCatalog(t *testing.T) spec.Catalog {
	t.Helper()
	// This nonlocal descriptor exercises dispatch without performing HTTP.
	t.Setenv("PLANE_CLI_BASE_URL", "http://fixture.invalid")
	c, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	c.Commands = append(c.Commands, spec.Command{Path: []string{"fixture", "nested", "create"}, Handler: "fixture", Summary: "Create fixture", SideEffect: "create", Examples: []string{"fixture nested create"}, Versions: map[string]spec.Variant{"v2": {
		Schema: json.RawMessage(`{"type":"object","properties":{"arguments":{"type":"array","maxItems":0},"flags":{"type":"object","properties":{"api-version":{"type":"string","enum":["v1","v2"]}},"required":["api-version"],"additionalProperties":false},"body":{"type":"object","properties":{"count":{"type":"integer","minimum":9007199254740993},"enabled":{"type":"boolean"}},"required":["count"],"additionalProperties":false}},"required":["arguments","flags","body"],"additionalProperties":false}`),
		Flags:  []spec.Flag{{Name: "api-version", Type: "string", Required: true, Binding: "/flags/api-version", Description: "Version"}, {Name: "count", Type: "integer", Required: true, Binding: "/body/count", Description: "Count"}, {Name: "enabled", Type: "boolean", Binding: "/body/enabled", Description: "Enabled"}},
	}}})
	return c
}

func TestNestedDiscoveryAndAvailability(t *testing.T) {
	c := fixtureCatalog(t)
	h := map[string]handler{"fixture": func(cmd *cobra.Command, _ []string, input map[string]any) error {
		return output.Success(cmd.OutOrStdout(), commandName(cmd), input["body"])
	}}
	for _, tc := range []struct {
		args     []string
		code     int
		contains string
	}{
		{[]string{"schema", "fixture", "nested", "create", "--api-version", "v2"}, 0, `"minimum":9007199254740993`},
		{[]string{"schema", "fixture", "nested", "--api-version", "v2"}, 2, "usage_error"},
		{[]string{"fixture", "nested", "create", "--api-version", "v1"}, 7, "operation_unavailable"},
		{[]string{"schema", "fixture", "nested", "create", "--api-version", "v1"}, 7, "operation_unavailable"},
		{[]string{"fixture", "nested", "unknown"}, 2, "usage_error"},
		{[]string{"fixture", "nested", "create", "--api-version", "v2", "--count", "9007199254740993", "--enabled=false"}, 0, `"count":9007199254740993,"enabled":false`},
		{[]string{"fixture", "nested", "create", "--api-version", "v2", "--count", "18446744073709551616"}, 0, `"count":18446744073709551616`},
		{[]string{"fixture", "nested", "create", "--api-version", "v2", "--count", "9007199254740992"}, 2, "usage_error"},
		{[]string{"fixture", "nested", "--help"}, 0, "create"},
		{[]string{"help", "fixture", "nested", "create"}, 0, "--count integer"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			code, out, err := invoke(t, c, tc.args, h)
			if code != tc.code || !strings.Contains(out+err, tc.contains) {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, out, err)
			}
			if code != 0 && out != "" {
				t.Fatal("failure wrote stdout")
			}
		})
	}
}

func TestFreshInvocationsAndSafeErrors(t *testing.T) {
	c, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"commands", "--api-version", "v2"}, {"version"}, {"--version"}} {
		code, out, stderr := invoke(t, c, args, nil)
		if code != 0 || stderr != "" || !json.Valid([]byte(out)) {
			t.Fatalf("code=%d stdout=%s stderr=%s", code, out, stderr)
		}
	}
	for _, args := range [][]string{{"commands"}, {"version", "--secret=PRIVATE"}, {"PRIVATE"}, {"schema", "version", "--api-version", "PRIVATE"}, {"help", "PRIVATE"}, {"help", "version", "PRIVATE"}, {"schema", string([]byte{0xff}), "--api-version", "v2"}} {
		code, out, stderr := invoke(t, c, args, nil)
		if code != 2 || out != "" || !json.Valid([]byte(stderr)) || strings.Contains(stderr, "PRIVATE") || strings.Count(stderr, "\n") != 1 {
			t.Fatalf("code=%d stdout=%s stderr=%s", code, out, stderr)
		}
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("PRIVATE writer detail") }

func TestOutputFailureAndCleanup(t *testing.T) {
	c := fixtureCatalog(t)
	cleaned := false
	h := map[string]handler{"fixture": func(cmd *cobra.Command, _ []string, _ map[string]any) error {
		defer func() { cleaned = true }()
		return output.Success(cmd.OutOrStdout(), commandName(cmd), json.Number("9007199254740993"))
	}}
	var stderr bytes.Buffer
	code := run(context.Background(), []string{"fixture", "nested", "create", "--api-version", "v2", "--count", "9007199254740993"}, Streams{In: strings.NewReader(""), Out: brokenWriter{}, Err: &stderr}, Build{}, c, h)
	if code != 1 || !cleaned || !strings.Contains(stderr.String(), "output_error") || strings.Contains(stderr.String(), "PRIVATE") {
		t.Fatalf("code=%d cleaned=%v stderr=%s", code, cleaned, &stderr)
	}
	for _, args := range [][]string{{"--help"}, {"help"}, {"help", "version"}} {
		stderr.Reset()
		code = run(context.Background(), args, Streams{In: strings.NewReader(""), Out: brokenWriter{}, Err: &stderr}, Build{}, c, h)
		if code != 1 || !strings.Contains(stderr.String(), "output_error") {
			t.Fatalf("help writer failure returned %d: %s", code, &stderr)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, args := range [][]string{{"version"}, {"--version"}, {"help"}, {"--help"}} {
		var stdout bytes.Buffer
		code = run(ctx, args, Streams{In: strings.NewReader(""), Out: &stdout, Err: &stderr}, Build{}, c, h)
		if code != 130 || stdout.Len() != 0 {
			t.Fatalf("cancellation %v returned %d, stdout=%q", args, code, stdout.String())
		}
	}
}

func TestCompletedOutputWinsOverLateCancellation(t *testing.T) {
	c := fixtureCatalog(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := map[string]handler{"fixture": func(cmd *cobra.Command, _ []string, _ map[string]any) error {
		err := output.Success(cmd.OutOrStdout(), commandName(cmd), "completed")
		cancel()
		return err
	}}
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"fixture", "nested", "create", "--api-version", "v2", "--count", "9007199254740993"}, Streams{Out: &stdout, Err: &stderr}, Build{}, c, h)
	if code != 0 || stderr.Len() != 0 || !json.Valid(stdout.Bytes()) {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, &stdout, &stderr)
	}
}

func TestSafeUsageRecovery(t *testing.T) {
	c := fixtureCatalog(t)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"versio"}, "version"},
		{[]string{"fixture", "nested", "create", "--api-version", "v2"}, "--count"},
	} {
		code, out, stderr := invoke(t, c, tc.args, map[string]handler{"fixture": func(*cobra.Command, []string, map[string]any) error {
			t.Fatal("invalid input reached handler")
			return nil
		}})
		if code != 2 || out != "" || !json.Valid([]byte(stderr)) || !strings.Contains(stderr, tc.want) {
			t.Fatalf("code=%d stdout=%s stderr=%s", code, out, stderr)
		}
	}
}

func TestInvalidEmbeddedSchemaIsLocalError(t *testing.T) {
	c := fixtureCatalog(t)
	d := &c.Commands[len(c.Commands)-1]
	v := d.Versions["v2"]
	v.Schema = json.RawMessage(`{"$ref":"https://invalid.example/schema"}`)
	d.Versions["v2"] = v
	code, out, stderr := invoke(t, c, []string{"fixture", "nested", "create", "--api-version", "v2", "--count", "9007199254740993"}, map[string]handler{"fixture": func(*cobra.Command, []string, map[string]any) error {
		t.Fatal("invalid schema reached handler")
		return nil
	}})
	if code != 1 || out != "" || !json.Valid([]byte(stderr)) || !strings.Contains(stderr, "local_error") {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, out, stderr)
	}
}
