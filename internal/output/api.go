package output

import (
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

const maxAPIOutputBytes = 64 << 20

// API stages a complete native API response before writing its JSON envelope.
func API(w io.Writer, command, version string, status int, data json.RawMessage) error {
	if status == 204 {
		data = json.RawMessage("null")
	}
	if len(data) > maxAPIOutputBytes {
		return errors.New("output exceeds the 64 MiB limit; use manual pagination")
	}
	if !utf8.Valid(data) || !json.Valid(data) {
		return errors.New("API response is not valid JSON")
	}
	completion := "completed"
	if status == 202 {
		completion = "accepted"
	}
	type apiMeta struct {
		Command    string `json:"command"`
		APIVersion string `json:"api_version"`
		HTTPStatus int    `json:"http_status"`
		Status     string `json:"status"`
	}
	payload, err := json.Marshal(struct {
		SchemaVersion int             `json:"schema_version"`
		Data          json.RawMessage `json:"data"`
		Meta          apiMeta         `json:"meta"`
	}{1, data, apiMeta{command, version, status, completion}})
	if err != nil {
		return err
	}
	if len(payload) >= maxAPIOutputBytes {
		return errors.New("output exceeds the 64 MiB limit; use manual pagination")
	}
	return writeJSONLine(w, payload)
}

// Warning writes one structured diagnostic without changing the command status.
func Warning(w io.Writer, command, code, message string) error {
	type warning struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Command string `json:"command"`
	}
	payload, err := json.Marshal(struct {
		SchemaVersion int     `json:"schema_version"`
		Warning       warning `json:"warning"`
	}{1, warning{code, message, command}})
	if err != nil {
		return err
	}
	return writeJSONLine(w, payload)
}

func writeJSONLine(w io.Writer, payload []byte) error {
	payload = append(payload, '\n')
	n, err := w.Write(payload)
	if err != nil {
		return err
	}
	if n != len(payload) {
		return io.ErrShortWrite
	}
	return nil
}
