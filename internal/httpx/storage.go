package httpx

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"sort"
	"strings"
	"sync/atomic"
)

func storageURL(target string) (*url.URL, error) {
	u, err := url.Parse(target)
	if err != nil || u.Host == "" || u.Opaque != "" || u.User != nil || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("invalid storage URL")
	}
	return u, nil
}

type uploadCounter struct {
	reader io.Reader
	bytes  atomic.Int64
}

func (r *uploadCounter) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.bytes.Add(int64(n))
	return n, err
}

type downloadWriter struct {
	writer io.Writer
	err    error
}

func (w *downloadWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if err == nil && n < len(p) {
		err = io.ErrShortWrite
	}
	if err != nil {
		w.err = err
	}
	return n, err
}

// DownloadLocation intercepts only the documented 302 wiki download phase.
// The returned signed destination must never be included in diagnostics.
func (c *Client) DownloadLocation(ctx context.Context, segments []string, header, token string) (string, error) {
	response, err := c.do(ctx, http.MethodGet, segments, nil, nil, header, token, false, true)
	if err != nil {
		return "", err
	}
	if response.Status != http.StatusFound || response.Location == "" {
		return "", &Error{Code: "invalid_response", Message: "Expected a download redirect", Status: response.Status, Exit: 5}
	}
	return response.Location, nil
}

// Upload streams one signed multipart POST without API credentials or redirects.
// Call this on a separate client constructed with the storage transport policy.
func (c *Client) Upload(ctx context.Context, target string, fields map[string]string, name string, file io.Reader, size int64) error {
	u, err := storageURL(target)
	if err != nil {
		return &Error{Code: "invalid_response", Message: "Invalid upload destination", Exit: 5}
	}
	if size < 0 || name == "" || strings.ContainsAny(name, "/\\\r\n") {
		return &Error{Code: "usage_error", Message: "Invalid upload filename or size", Exit: 2}
	}
	var framing bytes.Buffer
	writer := multipart.NewWriter(&framing)
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if key == "file" || strings.ContainsAny(key, "\r\n") {
			return &Error{Code: "invalid_response", Message: "Invalid signed upload fields", Exit: 5}
		}
		if err := writer.WriteField(key, fields[key]); err != nil {
			return &Error{Code: "invalid_response", Message: "Invalid signed upload fields", Exit: 5}
		}
	}
	contentType := fields["Content-Type"]
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if _, _, err = mime.ParseMediaType(contentType); err != nil || strings.ContainsAny(contentType, "\r\n") {
		return &Error{Code: "invalid_response", Message: "Invalid signed content type", Exit: 5}
	}
	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": name}))
	partHeader.Set("Content-Type", contentType)
	if _, err = writer.CreatePart(partHeader); err != nil {
		return &Error{Code: "invalid_request", Message: "Cannot construct upload", Exit: 2}
	}
	prefixSize := framing.Len()
	if err = writer.Close(); err != nil {
		return &Error{Code: "invalid_request", Message: "Cannot construct upload", Exit: 2}
	}
	limited := &uploadCounter{reader: io.LimitReader(file, size)}
	body := io.MultiReader(bytes.NewReader(framing.Bytes()[:prefixSize]), limited, bytes.NewReader(framing.Bytes()[prefixSize:]))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), body)
	if err != nil {
		return &Error{Code: "invalid_request", Message: "Cannot construct upload", Exit: 2}
	}
	req.ContentLength = int64(framing.Len()) + size
	req.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := c.http.Do(req)
	if err != nil {
		return requestError(ctx, err, true, 0)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &Error{Code: "storage_error", Message: "Storage rejected the upload", Status: response.StatusCode, Exit: 5, OutcomeUnknown: response.StatusCode >= 500}
	}
	if limited.bytes.Load() != size {
		return &Error{Code: "upload_incomplete", Message: "Storage did not consume the complete upload", Exit: 5, OutcomeUnknown: true}
	}
	return nil
}

// Download streams a credential-free storage response. Expected size is -1 when
// no authoritative length is available. Content-Disposition never chooses paths.
func (c *Client) Download(ctx context.Context, target string, destination io.Writer, expectedSize int64) (int64, error) {
	u, err := storageURL(target)
	if err != nil {
		return 0, &Error{Code: "invalid_response", Message: "Invalid download destination", Exit: 5}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, &Error{Code: "invalid_request", Message: "Cannot construct download", Exit: 2}
	}
	// Preserve stored bytes instead of allowing the transport to auto-decompress gzip.
	req.Header.Set("Accept-Encoding", "identity")
	response, err := c.http.Do(req)
	if err != nil {
		return 0, requestError(ctx, err, false, 0)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return 0, &Error{Code: "storage_error", Message: "Storage rejected the download", Status: response.StatusCode, Exit: 5}
	}
	if expectedSize >= 0 && response.ContentLength >= 0 && expectedSize != response.ContentLength {
		return 0, &Error{Code: "size_mismatch", Message: "Download size does not match metadata", Exit: 5}
	}
	var reader io.Reader = response.Body
	if expectedSize >= 0 {
		reader = io.LimitReader(reader, expectedSize+1)
	}
	writer := &downloadWriter{writer: destination}
	size, err := io.Copy(writer, reader)
	if err != nil {
		if writer.err != nil {
			return size, &Error{Code: "output_error", Message: "Cannot write download output", Exit: 1}
		}
		return size, requestError(ctx, err, false, response.StatusCode)
	}
	if (expectedSize >= 0 && size != expectedSize) || (response.ContentLength >= 0 && size != response.ContentLength) {
		return size, &Error{Code: "size_mismatch", Message: "Download size does not match declared length", Exit: 5}
	}
	return size, nil
}
