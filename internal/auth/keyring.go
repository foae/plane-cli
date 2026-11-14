package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"os/exec"
	"strings"
)

// Keyring is the explicitly selected native credential store. Implementations
// never fall back to a file or treat a locked/unavailable store as missing.
type Keyring interface {
	Get(context.Context, string) (string, error)
	Set(context.Context, string, string) error
	Delete(context.Context, string) error
}

// ErrCredentialNotFound means the native store positively reported no item.
var ErrCredentialNotFound = errors.New("credential not found")

const (
	keyringService        = "foae-plane-cli"
	keyringEncodingPrefix = "foae-plane-cli-base64-v1:"
	securityLineLimit     = 4096
	// errSecItemNotFound (-25300), represented by a Unix process exit status.
	securityItemNotFound = 44
)

func encodeKeyringSecret(secret string) string {
	return keyringEncodingPrefix + base64.StdEncoding.EncodeToString([]byte(secret))
}

func decodeKeyringSecret(encoded string) (string, error) {
	if !strings.HasPrefix(encoded, keyringEncodingPrefix) {
		return "", errors.New("keyring credential has an unsupported encoding")
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(strings.TrimPrefix(encoded, keyringEncodingPrefix))
	if err != nil {
		return "", errors.New("keyring credential has an invalid encoding")
	}
	return string(decoded), nil
}

// The subprocess adapter lives here so its lifecycle and protocol can be tested
// on either supported OS without touching a developer's native keychain.
type securityKeyring struct {
	command func(context.Context, ...string) *exec.Cmd
}

func (k securityKeyring) Get(ctx context.Context, key string) (string, error) {
	if err := validateSecurityKey(key); err != nil {
		return "", err
	}
	out, err := k.run(ctx, []string{"find-generic-password", "-s", keyringService, "-wa", key}, "", true)
	if err != nil {
		return "", err
	}
	// security appends a newline. Encoded values themselves contain none.
	return decodeKeyringSecret(strings.TrimSuffix(string(out), "\n"))
}

func (k securityKeyring) Set(ctx context.Context, key, secret string) error {
	if err := validateSecurityKey(key); err != nil {
		return err
	}
	// Bound allocation before base64 expansion and interactive quoting.
	if len(key) >= securityLineLimit || len(secret) >= securityLineLimit {
		return errors.New("credential is too large for the macOS keyring")
	}
	// Quote for security's own interactive parser, not a shell. In particular,
	// shell-style adjacent single-quoted fragments are separate arguments there.
	command := "add-generic-password -U -s " + quoteSecurityArgument(keyringService) +
		" -a " + quoteSecurityArgument(key) + " -w " + quoteSecurityArgument(encodeKeyringSecret(secret)) + "\n"
	// The 4096-byte input buffer needs room for NUL as well as the newline.
	// Reject before Run: a truncated command can otherwise execute a second line.
	if len(command) >= securityLineLimit {
		return errors.New("credential is too large for the macOS keyring")
	}
	_, err := k.run(ctx, []string{"-i"}, command, false)
	if err != nil {
		return err
	}
	// security -i can exit zero after an interactive command failed. Only a
	// read-back of the exact value proves that the requested write succeeded.
	stored, err := k.Get(ctx, key)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil || stored != secret {
		return errors.New("macOS keyring write could not be verified")
	}
	return nil
}

func (k securityKeyring) Delete(ctx context.Context, key string) error {
	if err := validateSecurityKey(key); err != nil {
		return err
	}
	_, err := k.run(ctx, []string{"delete-generic-password", "-s", keyringService, "-a", key}, "", true)
	return err
}

func validateSecurityKey(key string) error {
	if strings.ContainsAny(key, "\x00\r\n") {
		return errors.New("keyring key contains invalid characters")
	}
	return nil
}

func quoteSecurityArgument(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	return "\"" + strings.ReplaceAll(value, "\"", "\\\"") + "\""
}

func (k securityKeyring) run(ctx context.Context, args []string, input string, allowMissing bool) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cmd := k.command(ctx, args...)
	cmd.Stdin = strings.NewReader(input)
	out := boundedKeyringOutput{limit: securityLineLimit}
	cmd.Stdout = &out
	// Never retain diagnostics: security may reflect credentials in its errors.
	cmd.Stderr = io.Discard
	// Run owns stdin copying, close, cancellation and Wait on every path.
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		var exit *exec.ExitError
		if allowMissing && errors.As(err, &exit) && exit.ExitCode() == securityItemNotFound {
			return nil, ErrCredentialNotFound
		}
		return nil, errors.New("macOS keyring command failed")
	}
	if out.overflow {
		return nil, errors.New("macOS keyring response is too large")
	}
	return out.data, nil
}

// Continue draining after the cap so child output cannot deadlock cleanup.
type boundedKeyringOutput struct {
	data     []byte
	limit    int
	overflow bool
}

func (b *boundedKeyringOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := b.limit - len(b.data)
	if len(p) > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	b.data = append(b.data, p...)
	return n, nil
}
