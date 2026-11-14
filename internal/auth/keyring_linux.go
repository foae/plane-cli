package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"

	"github.com/zalando/go-keyring"
)

const (
	linuxKeyringHelper   = "--plane-cli-internal-keyring-v1"
	linuxKeyringNotFound = 44
)

type linuxKeyring struct {
	command func(context.Context, ...string) (*exec.Cmd, error)
}

type linuxKeyringRequest struct {
	Key    string
	Secret string
}

// DispatchKeyringHelper is the private subprocess entry point, called by main
// before normal CLI setup. Secrets travel over pipes, never argv or environment.
func DispatchKeyringHelper(args []string, input io.Reader, output io.Writer) (handled bool, status int) {
	if len(args) != 2 || args[0] != linuxKeyringHelper {
		return false, 0
	}
	if args[1] != "get" && args[1] != "set" && args[1] != "delete" {
		return true, 1
	}
	var request linuxKeyringRequest
	if err := json.NewDecoder(input).Decode(&request); err != nil {
		return true, 1
	}
	var err error
	switch args[1] {
	case "get":
		var value string
		value, err = keyring.Get(keyringService, request.Key)
		if err == nil {
			_, err = io.WriteString(output, value)
		}
	case "set":
		err = keyring.Set(keyringService, request.Key, request.Secret)
	case "delete":
		err = keyring.Delete(keyringService, request.Key)
	}
	if errors.Is(err, keyring.ErrNotFound) {
		return true, linuxKeyringNotFound
	}
	if err != nil {
		return true, 1
	}
	return true, 0
}

// NativeKeyring selects Linux Secret Service without any persistence fallback.
// The synchronous upstream API runs in a killable, reaped subprocess rather than
// a goroutine that could issue writes after the connection lock is released.
func NativeKeyring() Keyring {
	return linuxKeyring{command: func(ctx context.Context, args ...string) (*exec.Cmd, error) {
		executable, err := os.Executable()
		if err != nil {
			return nil, err
		}
		return exec.CommandContext(ctx, executable, args...), nil
	}}
}

func (k linuxKeyring) Get(ctx context.Context, key string) (string, error) {
	encoded, err := k.run(ctx, "get", linuxKeyringRequest{Key: key})
	if err != nil {
		return "", err
	}
	return decodeKeyringSecret(encoded)
}

func (k linuxKeyring) Set(ctx context.Context, key, secret string) error {
	_, err := k.run(ctx, "set", linuxKeyringRequest{Key: key, Secret: encodeKeyringSecret(secret)})
	return err
}

func (k linuxKeyring) Delete(ctx context.Context, key string) error {
	_, err := k.run(ctx, "delete", linuxKeyringRequest{Key: key})
	return err
}

func (k linuxKeyring) run(ctx context.Context, operation string, request linuxKeyringRequest) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	cmd, err := k.command(ctx, linuxKeyringHelper, operation)
	if err != nil {
		return "", errors.New("linux keyring operation failed")
	}
	input, err := json.Marshal(request)
	if err != nil {
		return "", errors.New("linux keyring operation failed")
	}
	cmd.Stdin = bytes.NewReader(input)
	var out bytes.Buffer
	cmd.Stdout = io.Discard
	if operation == "get" {
		cmd.Stdout = &out
	}
	// Secret Service diagnostics can reflect stored secrets.
	cmd.Stderr = io.Discard
	// Run owns pipe cleanup, cancellation and Wait. A daemon-accepted mutation
	// may still complete: cancellation is uncertain, never a rollback guarantee.
	err = cmd.Run()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == linuxKeyringNotFound {
			return "", ErrCredentialNotFound
		}
		return "", errors.New("linux keyring operation failed")
	}
	return out.String(), nil
}
