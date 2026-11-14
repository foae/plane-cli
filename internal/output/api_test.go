package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestAPINativeResponse(t *testing.T) {
	var out bytes.Buffer
	data := json.RawMessage(`{"id":9007199254740993,"unknown":{"items":[false,null,0,"",[]]}}`)
	if err := API(&out, "work-item get", "v2", 200, data); err != nil {
		t.Fatal(err)
	}
	want := "{\"schema_version\":1,\"data\":" + string(data) + ",\"meta\":{\"command\":\"work-item get\",\"api_version\":\"v2\",\"http_status\":200,\"status\":\"completed\"}}\n"
	if out.String() != want {
		t.Fatalf("output = %s; want %s", out.String(), want)
	}
}

func TestAPIResponseStatus(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		data   string
		want   string
	}{
		{"JSON null", 200, "null", `"data":null,"meta":{"command":"work-item get","api_version":"v1","http_status":200,"status":"completed"}`},
		{"no content", 204, "", `"data":null,"meta":{"command":"work-item get","api_version":"v1","http_status":204,"status":"completed"}`},
		{"accepted", 202, `{"poll":"opaque"}`, `"data":{"poll":"opaque"},"meta":{"command":"work-item get","api_version":"v1","http_status":202,"status":"accepted"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := API(&out, "work-item get", "v1", tc.status, json.RawMessage(tc.data)); err != nil {
				t.Fatal(err)
			}
			if out.String() != `{"schema_version":1,`+tc.want+"}\n" {
				t.Fatalf("output = %s", out.String())
			}
		})
	}
}

func TestAPIStagesBeforeWriting(t *testing.T) {
	for _, data := range []json.RawMessage{
		nil,
		json.RawMessage(`{"unfinished":`),
		json.RawMessage(`{} {}`),
		{'"', 0xff, '"'},
	} {
		var out bytes.Buffer
		if err := API(&out, "work-item get", "v2", 200, data); err == nil || out.Len() != 0 {
			t.Fatalf("error = %v; output length = %d", err, out.Len())
		}
	}
}

func TestAPITotalOutputLimit(t *testing.T) {
	// A valid payload under the limit must still fail when its envelope exceeds it.
	data := json.RawMessage(`"` + strings.Repeat("a", maxAPIOutputBytes-3) + `"`)
	var out bytes.Buffer
	err := API(&out, "work-item get", "v2", 200, data)
	if err == nil || !strings.Contains(err.Error(), "64 MiB") || !strings.Contains(err.Error(), "manual pagination") || out.Len() != 0 {
		t.Fatalf("error = %v; output length = %d", err, out.Len())
	}
}

type apiFailWriter struct {
	err error
}

func (w apiFailWriter) Write(p []byte) (int, error) {
	return len(p) / 2, w.err
}

func TestStructuredOutputWriterFailures(t *testing.T) {
	broken := errors.New("broken output")
	for _, render := range []struct {
		name string
		call func(io.Writer) error
	}{
		{"API", func(w io.Writer) error { return API(w, "work-item get", "v2", 200, json.RawMessage(`{}`)) }},
		{"warning", func(w io.Writer) error {
			return Warning(w, "connection add", "http_connection", "HTTP does not encrypt traffic")
		}},
	} {
		t.Run(render.name, func(t *testing.T) {
			if err := render.call(apiFailWriter{err: broken}); !errors.Is(err, broken) {
				t.Fatalf("writer error = %v", err)
			}
			if err := render.call(apiFailWriter{}); !errors.Is(err, io.ErrShortWrite) {
				t.Fatalf("short writer error = %v", err)
			}
		})
	}
}

func TestWarningEnvelope(t *testing.T) {
	var out bytes.Buffer
	if err := Warning(&out, "connection add", "http_connection", "HTTP does not encrypt traffic"); err != nil {
		t.Fatal(err)
	}
	want := "{\"schema_version\":1,\"warning\":{\"code\":\"http_connection\",\"message\":\"HTTP does not encrypt traffic\",\"command\":\"connection add\"}}\n"
	if out.String() != want {
		t.Fatalf("output = %s", out.String())
	}
}
