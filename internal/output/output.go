package output

import (
	"encoding/json"
	"io"
)

type Meta struct {
	Command string `json:"command"`
	Status  string `json:"status"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Command string `json:"command"`
	Exit    int    `json:"-"`
}

func (e *Error) Error() string { return e.Message }

func Success(w io.Writer, command string, data any) error {
	return json.NewEncoder(w).Encode(struct {
		SchemaVersion int  `json:"schema_version"`
		Data          any  `json:"data"`
		Meta          Meta `json:"meta"`
	}{1, data, Meta{Command: command, Status: "completed"}})
}

func Failure(w io.Writer, err *Error) error {
	return json.NewEncoder(w).Encode(struct {
		SchemaVersion int    `json:"schema_version"`
		Error         *Error `json:"error"`
	}{1, err})
}
