package cli

import (
	"context"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/foae/plane-cli/internal/output"
	"github.com/foae/plane-cli/internal/spec"
)

type Streams struct {
	In       io.Reader
	Out, Err io.Writer
}

type checkedWriter struct {
	io.Writer
	err error
}

func (w *checkedWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.Writer.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	w.err = err
	return n, err
}

// Run owns error rendering; callers own resource cleanup and the process exit.
func Run(ctx context.Context, args []string, streams Streams, build Build) int {
	catalog, err := spec.Load()
	if err != nil {
		return report(streams.Err, &output.Error{Code: "local_error", Message: "Embedded command catalog is invalid", Command: "plane-cli", Exit: 1})
	}
	return run(ctx, args, streams, build, catalog, nil)
}

func run(ctx context.Context, args []string, streams Streams, build Build, catalog spec.Catalog, extra map[string]handler) int {
	out := &checkedWriter{Writer: streams.Out}
	root, err := newRoot(catalog, build, out, extra) //nolint:contextcheck // Handlers inherit ctx through Cobra ExecuteContextC and cmd.Context().
	if err != nil {
		return report(streams.Err, &output.Error{Code: "local_error", Message: "Embedded command catalog is invalid", Command: "plane-cli", Exit: 1})
	}
	root.SetIn(streams.In)
	root.SetOut(out)
	// Cobra usage/errors are silenced; deliberate structured warnings use stderr.
	root.SetErr(streams.Err)
	root.SetArgs(args)
	cmd := root
	err = ctx.Err()
	if err == nil {
		for _, arg := range args {
			if !utf8.ValidString(arg) {
				return report(streams.Err, &output.Error{Code: "usage_error", Message: "Arguments must be valid UTF-8", Command: "plane-cli", Exit: 2})
			}
		}
		cmd, err = root.ExecuteContextC(ctx)
	}
	var safe *output.Error
	if errors.As(err, &safe) && safe.Exit == 6 {
		return report(streams.Err, safe)
	}
	if out.err != nil {
		if errors.As(err, &safe) && safe.Code == "output_error" {
			return report(streams.Err, safe)
		}
		return report(streams.Err, &output.Error{Code: "output_error", Message: "Could not write command output", Command: commandName(cmd), Exit: 1})
	}
	// Completed output is authoritative; a late cancellation cannot undo success.
	if err == nil {
		return 0
	}
	if !errors.As(err, &safe) && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		exit, code, message := 130, "interrupted", "Command interrupted"
		if errors.Is(err, context.DeadlineExceeded) {
			exit, code, message = 4, "deadline_exceeded", "Command deadline exceeded"
		}
		return report(streams.Err, &output.Error{Code: code, Message: message, Command: commandName(cmd), Exit: exit})
	}
	// Preserve mutation uncertainty even when the parent context was cancelled.
	if errors.As(err, &safe) {
		return report(streams.Err, safe)
	}
	if errors.Is(err, spec.ErrSchema) {
		return report(streams.Err, &output.Error{Code: "local_error", Message: "Embedded command schema is invalid", Command: commandName(cmd), Exit: 1})
	}
	message := "Invalid command or arguments; use --help to list valid commands and flags"
	if len(args) != 0 {
		if suggestions := root.SuggestionsFor(args[0]); len(suggestions) != 0 {
			message += "; did you mean: " + strings.Join(suggestions, ", ")
		}
	}
	return report(streams.Err, &output.Error{Code: "usage_error", Message: message, Command: commandName(cmd), Exit: 2})
}

func report(w io.Writer, err *output.Error) int {
	// A failed diagnostic stream cannot be repaired by writing another value.
	if output.Failure(w, err) != nil {
		return 1
	}
	return err.Exit
}
