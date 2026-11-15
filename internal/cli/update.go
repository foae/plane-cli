package cli

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/foae/plane-cli/internal/output"
	"github.com/foae/plane-cli/internal/semver"
	"github.com/foae/plane-cli/internal/spec"
	"github.com/spf13/cobra"
)

const updateAPI = "https://api.github.com/repos/foae/plane-cli/releases"
const updatePageSize = 100
const updateMaxPages = 10
const updateMaxBytes = 2 << 20

// No configuration, credential provider or Plane transport participates here.
func updateHTTPClient() *http.Client {
	return &http.Client{Transport: &http.Transport{
		Proxy:                  http.ProxyFromEnvironment,
		DialContext:            (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout:    10 * time.Second,
		ResponseHeaderTimeout:  10 * time.Second,
		MaxResponseHeaderBytes: 1 << 20,
		DisableKeepAlives:      true,
		TLSNextProto:           map[string]func(string, *tls.Conn) http.RoundTripper{},
	}}
}

type updateResult struct {
	CurrentVersion  string  `json:"current_version"`
	LatestVersion   *string `json:"latest_version"`
	UpdateAvailable *bool   `json:"update_available"`
	Channel         string  `json:"channel"`
	ReleaseURL      *string `json:"release_url"`
	Status          string  `json:"status"`
}

func updateCheckHandler(build Build, out io.Writer, injected *http.Client) handler {
	return func(cmd *cobra.Command, _ []string, _ map[string]any) error {
		total := 30 * time.Second
		if cmd.Flags().Changed("timeout") {
			raw, _ := cmd.Flags().GetString("timeout")
			var err error
			total, err = time.ParseDuration(raw)
			if err != nil || total <= 0 {
				return &output.Error{Code: "usage_error", Message: "--timeout must be a positive duration", Command: commandName(cmd), Exit: 2}
			}
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), total)
		defer cancel()
		client := injected
		if client == nil {
			client = updateHTTPClient()
		}
		// Even an injected client cannot relax the no-redirect policy.
		bounded := *client
		bounded.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
		include, _ := cmd.Flags().GetBool("include-prerelease")
		result, err := checkUpdate(ctx, &bounded, build.Version, include)
		if err != nil {
			return err
		}
		return output.Success(out, commandName(cmd), result)
	}
}

func updateFailure(code, message string, status, exit int) *output.Error {
	return &output.Error{Code: code, Message: message, Command: "update check", HTTPStatus: status, Exit: exit}
}

func updateProtocol(status int) error {
	return updateFailure("invalid_response", "GitHub returned invalid release metadata", status, 5)
}

func updateTransportError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return updateFailure("transport_error", "GitHub update request failed", 0, 4)
}

func checkUpdate(ctx context.Context, client *http.Client, current string, include bool) (updateResult, error) {
	result := updateResult{CurrentVersion: current, Channel: "stable", Status: "no_release"}
	if include {
		result.Channel = "prerelease"
	}
	installed, currentErr := semver.Parse(strings.TrimPrefix(current, "v"))
	if currentErr == nil {
		available := false
		result.UpdateAvailable = &available
	}
	var latest semver.Version
	for page := 1; ; page++ {
		requestURL := updateAPI + "?per_page=" + strconv.Itoa(updatePageSize) + "&page=" + strconv.Itoa(page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return result, updateProtocol(0)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Set("User-Agent", "plane-cli-update-check")
		resp, err := client.Do(req)
		if err != nil {
			return result, updateTransportError(ctx, err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, updateMaxBytes+1))
		closeErr := resp.Body.Close()
		if readErr != nil {
			return result, updateTransportError(ctx, readErr)
		}
		if closeErr != nil {
			return result, updateTransportError(ctx, closeErr)
		}
		if resp.StatusCode != http.StatusOK {
			exit, code := 5, "api_error"
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				exit, code = 3, "auth_error"
			}
			if resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode == http.StatusForbidden && (resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.Header.Get("Retry-After") != "")) {
				exit, code = 5, "rate_limited"
			}
			return result, updateFailure(code, "GitHub rejected the update request", resp.StatusCode, exit)
		}
		decoded, err := spec.DecodeWithLimit(body, updateMaxBytes)
		if err != nil {
			return result, updateProtocol(resp.StatusCode)
		}
		releases, ok := decoded.([]any)
		if !ok || len(releases) > updatePageSize {
			return result, updateProtocol(resp.StatusCode)
		}
		for _, entry := range releases {
			release, ok := entry.(map[string]any)
			if !ok {
				return result, updateProtocol(resp.StatusCode)
			}
			draft, ok := release["draft"].(bool)
			if !ok {
				return result, updateProtocol(resp.StatusCode)
			}
			if draft {
				continue
			}
			tag, ok := release["tag_name"].(string)
			if !ok {
				return result, updateProtocol(resp.StatusCode)
			}
			version := strings.TrimPrefix(tag, "v")
			candidate, err := semver.Parse(version)
			if err != nil {
				continue
			}
			prerelease, ok := release["prerelease"].(bool)
			published, publishedOK := release["published_at"].(string)
			_, dateErr := time.Parse(time.RFC3339, published)
			releaseURL, urlOK := release["html_url"].(string)
			if !ok || prerelease != (candidate.Prerelease != "") || !publishedOK || dateErr != nil || !urlOK || releaseURL != "https://github.com/foae/plane-cli/releases/tag/"+url.PathEscape(tag) {
				return result, updateProtocol(resp.StatusCode)
			}
			if prerelease && !include {
				continue
			}
			if result.LatestVersion == nil || semver.Compare(candidate, latest) > 0 {
				latest = candidate
				result.LatestVersion, result.ReleaseURL = &version, &releaseURL
			}
		}
		next, err := updateNextPage(resp.Header.Values("Link"), page)
		if err != nil {
			return result, updateProtocol(resp.StatusCode)
		}
		// Without a Link header, a full page cannot prove the scan complete.
		if !next && (len(releases) < updatePageSize || len(resp.Header.Values("Link")) != 0) {
			break
		}
		if page == updateMaxPages {
			return result, updateFailure("invalid_response", "GitHub release scan exceeded the 10-page limit; latest release is unknown", resp.StatusCode, 5)
		}
	}
	if currentErr != nil {
		result.Status = "unknown_current"
	} else if result.LatestVersion != nil {
		available := semver.Compare(latest, installed) > 0
		result.UpdateAvailable = &available
		result.Status = "up_to_date"
		if available {
			result.Status = "update_available"
		}
	}
	return result, nil
}

// Links are validated, never used as request destinations.
func updateNextPage(headers []string, page int) (bool, error) {
	next := false
	for _, header := range headers {
		for _, part := range strings.Split(header, ",") {
			if strings.TrimSpace(part) == "" {
				continue
			}
			link, relation, ok := strings.Cut(strings.TrimSpace(part), ";")
			if !ok || !strings.HasPrefix(link, "<") || !strings.HasSuffix(link, ">") {
				return false, errors.New("invalid pagination")
			}
			u, err := url.Parse(strings.TrimSuffix(strings.TrimPrefix(link, "<"), ">"))
			if err != nil || u.Scheme != "https" || u.Host != "api.github.com" || u.Path != "/repos/foae/plane-cli/releases" || u.RawPath != "" || u.User != nil || u.Fragment != "" {
				return false, errors.New("invalid pagination")
			}
			q, err := url.ParseQuery(u.RawQuery)
			p, parseErr := strconv.Atoi(q.Get("page"))
			if err != nil || parseErr != nil || p < 1 || len(q) != 2 || len(q["page"]) != 1 || len(q["per_page"]) != 1 || q.Get("per_page") != "100" {
				return false, errors.New("invalid pagination")
			}
			switch strings.TrimSpace(relation) {
			case `rel="next"`:
				if next || p != page+1 {
					return false, errors.New("invalid pagination")
				}
				next = true
			case `rel="first"`, `rel="prev"`, `rel="last"`:
			default:
				return false, errors.New("invalid pagination")
			}
		}
	}
	return next, nil
}
