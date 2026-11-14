package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fakeLinuxKeyringCommand(mode, path string, status int) func(context.Context, ...string) (*exec.Cmd, error) {
	return func(ctx context.Context, args ...string) (*exec.Cmd, error) {
		argv := append([]string{"-test.run=^TestLinuxKeyringHelperProcess$", "--"}, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], argv...)
		cmd.Env = append(os.Environ(),
			"PLANE_CLI_TEST_LINUX_KEYRING_HELPER=1",
			"PLANE_CLI_TEST_LINUX_KEYRING_MODE="+mode,
			"PLANE_CLI_TEST_LINUX_KEYRING_PATH="+path,
			"PLANE_CLI_TEST_LINUX_KEYRING_STATUS="+strconv.Itoa(status))
		return cmd, nil
	}
}

func TestLinuxKeyringHelperProcess(t *testing.T) {
	if os.Getenv("PLANE_CLI_TEST_LINUX_KEYRING_HELPER") != "1" {
		return
	}
	fail := func(err error) {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) != 3 || args[1] != linuxKeyringHelper {
		fail(errors.New("incorrect helper arguments"))
	}
	args = args[1:]
	mode := os.Getenv("PLANE_CLI_TEST_LINUX_KEYRING_MODE")
	if mode == "dispatch" {
		handled, status := DispatchKeyringHelper(args, os.Stdin, os.Stdout)
		if !handled {
			fail(errors.New("helper invocation was not handled"))
		}
		os.Exit(status)
	}
	var request linuxKeyringRequest
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fail(err)
	}
	path := os.Getenv("PLANE_CLI_TEST_LINUX_KEYRING_PATH")
	if mode == "block" {
		ready := os.NewFile(3, "ready")
		_, _ = ready.Write([]byte{1})
		_ = ready.Close()
		permit := os.NewFile(4, "permit")
		var b [1]byte
		if _, err := io.ReadFull(permit, b[:]); err != nil {
			fail(err)
		}
		if err := os.WriteFile(path, []byte("late backend mutation"), 0600); err != nil {
			fail(err)
		}
		os.Exit(0)
	}
	if mode == "output" {
		_, _ = io.WriteString(os.Stdout, "reflected-fixture-secret")
		_, _ = io.WriteString(os.Stderr, "locked: reflected-fixture-secret")
		status, _ := strconv.Atoi(os.Getenv("PLANE_CLI_TEST_LINUX_KEYRING_STATUS"))
		os.Exit(status)
	}
	items := make(map[string]string)
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fail(err)
	}
	if err == nil {
		if err := json.Unmarshal(data, &items); err != nil {
			fail(err)
		}
	}
	switch args[1] {
	case "get":
		value, ok := items[request.Key]
		if !ok {
			os.Exit(linuxKeyringNotFound)
		}
		_, _ = io.WriteString(os.Stdout, value)
		os.Exit(0)
	case "set":
		if !strings.HasPrefix(request.Secret, keyringEncodingPrefix) {
			fail(errors.New("incorrect secret encoding"))
		}
		items[request.Key] = request.Secret
	case "delete":
		if _, ok := items[request.Key]; !ok {
			os.Exit(linuxKeyringNotFound)
		}
		delete(items, request.Key)
	default:
		fail(errors.New("unknown operation"))
	}
	data, err = json.Marshal(items)
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		fail(err)
	}
	os.Exit(0)
}

func TestLinuxKeyringRoundTripWithoutNativeService(t *testing.T) {
	path := filepath.Join(t.TempDir(), "items")
	runner := fakeLinuxKeyringCommand("store", path, 0)
	secret := "fixture-secret\x00\n\r雪"
	k := linuxKeyring{command: func(ctx context.Context, args ...string) (*exec.Cmd, error) {
		for _, arg := range args {
			if strings.Contains(arg, "fixture-secret") || strings.Contains(arg, encodeKeyringSecret(secret)) {
				t.Fatal("secret appeared in subprocess argv")
			}
		}
		return runner(ctx, args...)
	}}
	ctx := context.Background()
	if _, err := k.Get(ctx, "binding-a"); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("missing Get error = %v", err)
	}
	if err := k.Set(ctx, "binding-a", secret); err != nil {
		t.Fatal(err)
	}
	if err := k.Set(ctx, "binding-b", "other-fixture-secret"); err != nil {
		t.Fatal(err)
	}
	got, err := k.Get(ctx, "binding-a")
	if err != nil || got != secret {
		t.Fatalf("round trip failed: equal=%v error=%v", got == secret, err)
	}
	if err := k.Delete(ctx, "binding-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Get(ctx, "binding-a"); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("Get after Delete error = %v", err)
	}
	if err := k.Delete(ctx, "binding-a"); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("Delete missing error = %v", err)
	}
	if got, err := k.Get(ctx, "binding-b"); err != nil || got != "other-fixture-secret" {
		t.Fatalf("deleting one binding damaged another: %v", err)
	}
	data, err := json.Marshal(map[string]string{"binding-b": "corrupt-fixture-secret"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := k.Get(ctx, "binding-b"); got != "" || err == nil || errors.Is(err, ErrCredentialNotFound) || strings.Contains(err.Error(), "fixture-secret") {
		t.Fatalf("corruption was hidden or reflected: %v", err)
	}
}

func TestLinuxKeyringOnlyMapsMissingExitStatus(t *testing.T) {
	for _, status := range []int{1, linuxKeyringNotFound, 128} {
		k := linuxKeyring{command: fakeLinuxKeyringCommand("output", "", status)}
		got, getErr := k.Get(context.Background(), "binding")
		if got != "" {
			t.Fatal("failed Get exposed a credential")
		}
		for _, err := range []error{getErr, k.Set(context.Background(), "binding", "fixture-secret"), k.Delete(context.Background(), "binding")} {
			if err == nil || errors.Is(err, ErrCredentialNotFound) != (status == linuxKeyringNotFound) || strings.Contains(err.Error(), "fixture-secret") {
				t.Fatalf("incorrect or unsafe error mapping: %v", err)
			}
		}
	}
	k := linuxKeyring{command: fakeLinuxKeyringCommand("output", "", 0)}
	if got, err := k.Get(context.Background(), "binding"); got != "" || err == nil || errors.Is(err, ErrCredentialNotFound) || strings.Contains(err.Error(), "fixture-secret") {
		t.Fatalf("invalid response was accepted or reflected: %v", err)
	}
}

func TestLinuxKeyringStartErrorsStaySafe(t *testing.T) {
	for _, runner := range []func(context.Context, ...string) (*exec.Cmd, error){
		func(context.Context, ...string) (*exec.Cmd, error) {
			return nil, errors.New("reflected-fixture-secret")
		},
		func(ctx context.Context, _ ...string) (*exec.Cmd, error) {
			return exec.CommandContext(ctx, filepath.Join(t.TempDir(), "reflected-fixture-secret")), nil
		},
	} {
		k := linuxKeyring{command: runner}
		if got, err := k.Get(context.Background(), "binding"); got != "" || err == nil || errors.Is(err, ErrCredentialNotFound) || strings.Contains(err.Error(), "fixture-secret") {
			t.Fatalf("start failure was accepted or reflected: %v", err)
		}
	}
}

func TestLinuxKeyringPreCancelledContextDoesNotAccessBackend(t *testing.T) {
	k := linuxKeyring{command: func(context.Context, ...string) (*exec.Cmd, error) {
		t.Fatal("pre-cancelled operation launched process")
		return nil, nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, getErr := k.Get(ctx, "binding")
	for _, err := range []error{getErr, k.Set(ctx, "binding", "fixture-secret"), k.Delete(ctx, "binding")} {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled operation error = %v", err)
		}
	}
}

func TestLinuxKeyringCancellationReapsChildBeforeReturning(t *testing.T) {
	for _, operation := range []string{"get", "set", "delete"} {
		t.Run(operation, func(t *testing.T) {
			watchdog, stop := context.WithTimeout(context.Background(), 10*time.Second)
			defer stop()
			ctx, cancel := context.WithCancel(watchdog)
			defer cancel()
			readyRead, readyWrite, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = readyRead.Close() }()
			defer func() { _ = readyWrite.Close() }()
			permitRead, permitWrite, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = permitRead.Close() }()
			defer func() { _ = permitWrite.Close() }()
			path := filepath.Join(t.TempDir(), "late-mutation")
			runner := fakeLinuxKeyringCommand("block", path, 0)
			var child *exec.Cmd
			k := linuxKeyring{command: func(ctx context.Context, args ...string) (*exec.Cmd, error) {
				var err error
				child, err = runner(ctx, args...)
				if err == nil {
					child.ExtraFiles = []*os.File{readyWrite, permitRead}
				}
				return child, err
			}}
			completed := make(chan error, 1)
			go func() {
				switch operation {
				case "get":
					_, err := k.Get(ctx, "binding")
					completed <- err
				case "set":
					completed <- k.Set(ctx, "binding", "fixture-secret")
				case "delete":
					completed <- k.Delete(ctx, "binding")
				}
			}()
			ready := make(chan error, 1)
			go func() { var b [1]byte; _, err := io.ReadFull(readyRead, b[:]); ready <- err }()
			select {
			case err := <-ready:
				if err != nil {
					t.Fatal(err)
				}
			case err := <-completed:
				t.Fatalf("child failed before readiness: %v", err)
			case <-watchdog.Done():
				t.Fatal("child did not become ready")
			}
			cancel()
			select {
			case err := <-completed:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation error = %v", err)
				}
			case <-watchdog.Done():
				t.Fatal("cancellation did not stop and reap child")
			}
			if child.ProcessState == nil {
				t.Fatal("operation returned before waiting for child")
			}
			if _, err := child.Process.Wait(); err == nil {
				t.Fatal("child was not already reaped")
			}
			// Releasing the simulated backend after return cannot cause a write:
			// there is no surviving child to consume this permission.
			_, _ = permitWrite.Write([]byte{1})
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("cancelled backend performed a late mutation: %v", err)
			}
		})
	}
}

func TestLinuxKeyringHelperDispatchRejectsInvalidInput(t *testing.T) {
	for _, args := range [][]string{nil, {"auth", "status"}, {linuxKeyringHelper}, {linuxKeyringHelper, "get", "extra"}} {
		if handled, status := DispatchKeyringHelper(args, strings.NewReader(""), io.Discard); handled || status != 0 {
			t.Fatalf("ordinary invocation was intercepted: handled=%v status=%d", handled, status)
		}
	}
	for _, operation := range []string{"get", "set", "delete", "unknown"} {
		runner := fakeLinuxKeyringCommand("dispatch", "", 0)
		cmd, err := runner(context.Background(), linuxKeyringHelper, operation)
		if err != nil {
			t.Fatal(err)
		}
		cmd.Stdin = strings.NewReader("{")
		if err := cmd.Run(); err == nil {
			t.Fatal("malformed helper invocation succeeded")
		} else {
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatalf("helper dispatch failed unexpectedly: %v", err)
			}
		}
	}
}
