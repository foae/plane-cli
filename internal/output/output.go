package output

import (
	"encoding/json"
	"io"
)

type Meta struct {
	Command string `json:"command"`
	Status  string `json:"status"`
}

// PaginationProgress describes staged pages without exposing continuation data.
type PaginationProgress struct {
	Mode     string `json:"mode"`
	Pages    int    `json:"pages"`
	Complete bool   `json:"complete"`
}

type Error struct {
	Code              string              `json:"code"`
	Message           string              `json:"message"`
	Command           string              `json:"command"`
	Exit              int                 `json:"-"`
	APIVersion        string              `json:"api_version,omitempty"`
	HTTPStatus        int                 `json:"http_status,omitempty"`
	RetryAfterSeconds *int                `json:"retry_after_seconds,omitempty"`
	OutcomeUnknown    bool                `json:"outcome_unknown,omitempty"`
	RemoteChanged     bool                `json:"remote_changed,omitempty"`
	SecretDelivered   *bool               `json:"secret_delivered,omitempty"`
	AssetID           string              `json:"asset_id,omitempty"`
	Phase             string              `json:"phase,omitempty"`
	Recovery          string              `json:"recovery,omitempty"`
	Progress          *PaginationProgress `json:"progress,omitempty"`
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
