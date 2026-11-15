package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExplicitEmptySuccess(t *testing.T) {
	for _, tt := range []struct {
		name              string
		status, permitted int
		body, contentType string
		wantError         bool
	}{
		{name: "documented empty creation", status: 201, permitted: 201},
		{name: "ordinary empty creation rejected", status: 201, wantError: true},
		{name: "different empty status rejected", status: 200, permitted: 201, wantError: true},
		{name: "whitespace is not empty", status: 201, permitted: 201, body: " ", contentType: "application/json", wantError: true},
		{name: "malformed body is not ignored", status: 201, permitted: 201, body: "broken", contentType: "application/json", wantError: true},
		{name: "error status is never permitted", status: 403, permitted: 403, wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.contentType != "" {
					w.Header().Set("Content-Type", tt.contentType)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			client, err := New(server.URL, time.Second, "")
			if err != nil {
				t.Fatal(err)
			}
			client.EmptySuccessStatus = tt.permitted
			response, err := client.Do(context.Background(), http.MethodPost, []string{"values"}, nil, nil, "", "")
			if tt.wantError {
				var apiError *Error
				if !errors.As(err, &apiError) || apiError.Status != tt.status {
					t.Fatalf("response=%+v error=%v", response, err)
				}
				if tt.status < 300 && (apiError.Code != "invalid_response" || !apiError.OutcomeUnknown) {
					t.Fatalf("error=%+v", apiError)
				}
			} else if err != nil || response.Status != tt.status || string(response.Data) != "null" {
				t.Fatalf("response=%+v error=%v", response, err)
			}
		})
	}
}
