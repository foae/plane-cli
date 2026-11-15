package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/foae/plane-cli/internal/httpx"
	"github.com/foae/plane-cli/internal/output"
	"github.com/spf13/cobra"
)

func TestAPITransportErrorSafeUpstreamCode(t *testing.T) {
	command := &cobra.Command{Use: "update"}
	seconds := 7
	for _, code := range []string{"workflow_state_orphans", ""} {
		remote := &httpx.Error{Code: "api_error", Message: "API request rejected", Status: 409, Exit: 5, UpstreamCode: code, RetryAfterSeconds: &seconds, OutcomeUnknown: true}
		err := apiTransportError(command, "v2", remote)
		var normalized *output.Error
		if !errors.As(err, &normalized) || normalized.UpstreamCode != code || normalized.Code != remote.Code || normalized.Exit != remote.Exit || normalized.HTTPStatus != remote.Status || normalized.APIVersion != "v2" || !normalized.OutcomeUnknown || normalized.RetryAfterSeconds != &seconds {
			t.Fatalf("error fields not preserved: %#v", err)
		}
		var stderr bytes.Buffer
		if err := output.Failure(&stderr, normalized); err != nil {
			t.Fatal(err)
		}
		var envelope struct {
			Error map[string]any `json:"error"`
		}
		if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		got, present := envelope.Error["upstream_code"]
		if code == "" {
			if present {
				t.Fatalf("empty upstream code serialized: %s", stderr.Bytes())
			}
		} else if got != code {
			t.Fatalf("missing safe upstream code: %s", stderr.Bytes())
		}
		for _, forbidden := range []string{"detail", "type", "errors", "body", "exit"} {
			if _, ok := envelope.Error[forbidden]; ok {
				t.Fatalf("unexpected field %q: %s", forbidden, stderr.Bytes())
			}
		}
	}
}
