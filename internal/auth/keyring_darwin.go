package auth

import (
	"context"
	"io"
	"os/exec"
)

func securityCommand(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "/usr/bin/security", args...)
}

// DispatchKeyringHelper is unused on macOS, which invokes security directly.
func DispatchKeyringHelper(_ []string, _ io.Reader, _ io.Writer) (bool, int) {
	return false, 0
}

// NativeKeyring selects the macOS security tool without a storage fallback.
func NativeKeyring() Keyring {
	return securityKeyring{command: securityCommand}
}
