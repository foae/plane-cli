package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/foae/plane-cli/internal/httpx"
	"github.com/foae/plane-cli/internal/output"
	"github.com/spf13/cobra"
)

func aggregatePages(cmd *cobra.Command, ctx context.Context, client *httpx.Client, segments []string, query url.Values, header, token, version string, first httpx.Response) (result error) {
	pages := 0
	defer func() {
		var failure *output.Error
		if errors.As(result, &failure) {
			failure.Progress = &output.PaginationProgress{Mode: "all", Pages: pages, Complete: false}
		}
	}()
	fail := func(code, message string, exit int) error {
		return &output.Error{Code: code, Message: message, Command: commandName(cmd), Exit: exit, APIVersion: version}
	}
	contextError := func(err error) error {
		if errors.Is(err, context.DeadlineExceeded) {
			return fail("deadline_exceeded", "Aggregate deadline exceeded", 4)
		}
		return fail("interrupted", "Aggregate interrupted", 130)
	}
	maxPages := 1000
	if cmd.Flags().Changed("max-pages") {
		value, _ := cmd.Flags().GetString("max-pages")
		maxPages, _ = strconv.Atoi(value)
	}
	spool, err := os.CreateTemp("", "plane-cli-pages-*")
	if err != nil {
		return fail("output_error", "Cannot create aggregate spool", 1)
	}
	defer func() { _ = spool.Close(); _ = os.Remove(spool.Name()) }()
	format, _ := cmd.Flags().GetString("output")
	table := format == "table"
	columns := strings.Split(cmd.Annotations["table_columns"], ",")
	if !table {
		if _, err = io.WriteString(spool, "{\"schema_version\":1,\"data\":["); err != nil {
			return fail("output_error", "Cannot write aggregate spool", 1)
		}
	}
	response := first
	seen := map[string]bool{query.Encode(): true}
	size := int64(28)
	for {
		if err = ctx.Err(); err != nil {
			return contextError(err)
		}
		size += int64(len(response.Data) + 1)
		if size > (64<<20)-4096 {
			return fail("output_limit", "Aggregate exceeds 64 MiB; use manual pagination", 1)
		}
		if table {
			if err = tablePage(spool, response.Data, columns, pages == 0); err != nil {
				return fail("invalid_response", "Cannot render table response", 5)
			}
		} else {
			if pages > 0 {
				if _, err = io.WriteString(spool, ","); err != nil {
					return fail("output_error", "Cannot write aggregate spool", 1)
				}
			}
			if _, err = spool.Write(response.Data); err != nil {
				return fail("output_error", "Cannot write aggregate spool", 1)
			}
		}
		pages++
		if cmd.Annotations["pagination"] == "none" {
			break
		}
		next, more, nextErr := nextPage(response.Data, cmd.Annotations["pagination"], query)
		if nextErr != nil {
			return fail("invalid_pagination", nextErr.Error(), 5)
		}
		if !more {
			break
		}
		if pages >= maxPages {
			return fail("page_limit", "Maximum pages reached; use manual pagination or raise --max-pages", 1)
		}
		if seen[next.Encode()] {
			return fail("invalid_pagination", "Repeated pagination continuation", 5)
		}
		seen[next.Encode()] = true
		query = next
		response, err = client.Do(ctx, "GET", segments, query, nil, header, token)
		if err != nil {
			return apiTransportError(cmd, version, err)
		}
	}
	if !table {
		meta := map[string]any{"command": commandName(cmd), "api_version": version, "http_status": response.Status, "status": "completed", "pagination": map[string]any{"mode": "all", "pages": pages, "complete": true}}
		encoded, _ := json.Marshal(meta)
		if _, err = io.WriteString(spool, "],\"meta\":"+string(encoded)+"}\n"); err != nil {
			return fail("output_error", "Cannot finish aggregate spool", 1)
		}
	}
	if _, err = spool.Seek(0, io.SeekStart); err != nil {
		return fail("output_error", "Cannot read aggregate spool", 1)
	}
	if err = ctx.Err(); err != nil {
		return contextError(err)
	}
	if _, err = io.Copy(cmd.OutOrStdout(), spool); err != nil {
		return fail("output_error", "Cannot deliver aggregate result", 1)
	}
	return nil
}
