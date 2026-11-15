package main

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"

	"github.com/foae/plane-cli/internal/auth"
	"github.com/foae/plane-cli/internal/cli"
)

var version = "dev"
var commit = "unknown"

func main() { os.Exit(run()) }

func run() int {
	if handled, status := auth.DispatchKeyringHelper(os.Args[1:], os.Stdin, os.Stdout); handled {
		return status
	}
	// Go otherwise terminates on writes to a closed stdout pipe before cleanup.
	pipe := make(chan os.Signal, 1)
	signal.Notify(pipe, syscall.SIGPIPE)
	defer signal.Stop(pipe)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	info, _ := debug.ReadBuildInfo()
	buildVersion, buildCommit := buildIdentity(version, commit, info)
	return cli.Run(ctx, os.Args[1:], cli.Streams{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}, cli.Build{
		Version: buildVersion, Commit: buildCommit, GoVersion: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH,
	})
}
