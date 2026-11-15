package httpx

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestStorageTransferCancellation(t *testing.T) {
	for _, upload := range []bool{false, true} {
		name := "download"
		if upload {
			name = "upload"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if upload {
					_, _ = io.Copy(io.Discard, r.Body)
				}
				cancel()
				<-r.Context().Done()
			}))
			defer server.Close()
			client := &Client{http: server.Client()}
			var err error
			if upload {
				err = client.Upload(ctx, server.URL+"?secret=signed-secret", nil, "file.txt", strings.NewReader("bytes"), 5)
			} else {
				_, err = client.Download(ctx, server.URL+"?secret=signed-secret", io.Discard, -1)
			}
			var transportErr *Error
			if !errors.As(err, &transportErr) {
				t.Fatalf("expected structured cancellation error, got %v", err)
			}
			if strings.Contains(err.Error(), "signed-secret") {
				t.Fatal("signed target leaked through cancellation diagnostic")
			}
			if ctx.Err() != context.Canceled {
				t.Fatalf("request context was not cancelled: %v", ctx.Err())
			}
		})
	}
}

func TestStorageDownloadPreservesGzipBytes(t *testing.T) {
	var compressed bytes.Buffer
	encoder := gzip.NewWriter(&compressed)
	if _, err := encoder.Write([]byte("stored gzip content")); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatal(err)
	}
	payload := compressed.Bytes()
	for _, expectedSize := range []int64{int64(len(payload)), -1} {
		t.Run(strconv.FormatInt(expectedSize, 10), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Accept-Encoding"); got != "identity" {
					t.Errorf("Accept-Encoding = %q, want identity", got)
				}
				w.Header().Set("Content-Encoding", "gzip")
				w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
				_, _ = w.Write(payload)
			}))
			defer server.Close()
			client := &Client{http: server.Client()}
			var destination bytes.Buffer
			size, err := client.Download(context.Background(), server.URL, &destination, expectedSize)
			if err != nil {
				t.Fatal(err)
			}
			if size != int64(len(payload)) || !bytes.Equal(destination.Bytes(), payload) {
				t.Fatalf("download did not preserve stored bytes: size = %d, bytes = %x", size, destination.Bytes())
			}
		})
	}
}

type storageFailingWriter struct {
	err error
}

func (w storageFailingWriter) Write(p []byte) (int, error) {
	return len(p) / 2, w.err
}

func TestStorageDownloadOutputErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "write failure", err: errors.New("private destination details")},
		{name: "short write"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, "stored bytes")
			}))
			defer server.Close()
			client := &Client{http: server.Client()}
			size, err := client.Download(context.Background(), server.URL+"?secret=signed-secret", storageFailingWriter{err: test.err}, -1)
			var outputErr *Error
			if !errors.As(err, &outputErr) {
				t.Fatalf("expected structured output error, got %v", err)
			}
			if outputErr.Code != "output_error" || outputErr.Exit != 1 {
				t.Fatalf("expected output_error with exit 1, got %+v", outputErr)
			}
			if size != int64(len("stored bytes")/2) {
				t.Fatalf("written size = %d, want %d", size, len("stored bytes")/2)
			}
			if strings.Contains(err.Error(), "private destination details") || strings.Contains(err.Error(), "signed-secret") {
				t.Fatal("private details leaked through output diagnostic")
			}
		})
	}
}
