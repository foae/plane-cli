package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/foae/plane-cli/internal/spec"
)

func TestReadBodyPreservesRawInput(t *testing.T) {
	for _, input := range []string{
		" \n {\"id\":9007199254740993,\"nullable\":null,\"false\":false,\"zero\":0,\"empty\":\"\",\"list\":[]}\t\n",
		"null",
		"[9007199254740993, false, null]",
	} {
		got, err := readBody(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != input {
			t.Fatalf("body = %q; want exact bytes %q", got, input)
		}
	}
}

func TestReadBodyRejectsInvalidJSON(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"whitespace", " \n\t"},
		{"invalid", `{"x":`},
		{"duplicate", `{"x":1,"x":2}`},
		{"nested duplicate", `{"a":[{"x":1,"x":2}]}`},
		{"escaped duplicate", `{"x":1,"\u0078":2}`},
		{"trailing object", `{} {}`},
		{"trailing scalar", `{} true`},
		{"trailing junk", `{} !`},
		{"invalid UTF-8", "{\"x\":\"\xff\"}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := readBody(strings.NewReader(tc.input))
			if !errors.Is(err, spec.ErrInput) || body != nil {
				t.Fatalf("body = %q; error = %v", body, err)
			}
		})
	}
}

func TestReadBodySizeBoundary(t *testing.T) {
	input := `"` + strings.Repeat("a", spec.MaxInputBytes-2) + `"`
	got, err := readBody(strings.NewReader(input))
	if err != nil || !bytes.Equal(got, []byte(input)) {
		t.Fatalf("at limit: body length = %d; error = %v", len(got), err)
	}
	reader := strings.NewReader(input + "  extra bytes remain unread")
	body, err := readBody(reader)
	if !errors.Is(err, spec.ErrInput) || !strings.Contains(err.Error(), "8 MiB") || body != nil {
		t.Fatalf("over limit: body length = %d; error = %v", len(body), err)
	}
	if reader.Len() != len(" extra bytes remain unread") {
		t.Fatalf("reader consumed beyond limit probe: remaining bytes = %d", reader.Len())
	}
}

type bodyErrorReader struct {
	err error
}

func (r bodyErrorReader) Read(p []byte) (int, error) {
	copy(p, "{}")
	return 2, r.err
}

func TestReadBodyReadFailure(t *testing.T) {
	broken := errors.New("input unavailable")
	body, err := readBody(io.MultiReader(strings.NewReader(" "), bodyErrorReader{broken}))
	if !errors.Is(err, broken) || body != nil {
		t.Fatalf("body = %q; error = %v", body, err)
	}
}
