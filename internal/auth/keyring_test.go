package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fakeSecurityCommand(mode, path, output string, status int) func(context.Context, ...string) *exec.Cmd {
	return func(ctx context.Context, args ...string) *exec.Cmd {
		argv := append([]string{"-test.run=^TestSecurityKeyringHelperProcess$", "--"}, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], argv...)
		cmd.Env = append(os.Environ(),
			"PLANE_CLI_TEST_SECURITY_HELPER=1",
			"PLANE_CLI_TEST_SECURITY_MODE="+mode,
			"PLANE_CLI_TEST_SECURITY_PATH="+path,
			"PLANE_CLI_TEST_SECURITY_OUTPUT="+output,
			"PLANE_CLI_TEST_SECURITY_STATUS="+strconv.Itoa(status))
		return cmd
	}
}

// This child executable models security's actual argv/stdin boundary, consumes
// stdin to EOF and persists encoded items between separate process invocations.
func TestSecurityKeyringHelperProcess(t *testing.T) {
	if os.Getenv("PLANE_CLI_TEST_SECURITY_HELPER") != "1" {
		return
	}
	mode := os.Getenv("PLANE_CLI_TEST_SECURITY_MODE")
	if mode == "block" {
		ready := os.NewFile(3, "ready")
		_, _ = ready.Write([]byte{1})
		_ = ready.Close()
		interrupts := make(chan os.Signal, 1)
		signal.Notify(interrupts, os.Interrupt)
		<-interrupts
		os.Exit(1)
	}
	if mode == "output" {
		_, _ = io.WriteString(os.Stdout, os.Getenv("PLANE_CLI_TEST_SECURITY_OUTPUT"))
		_, _ = io.WriteString(os.Stderr, "reflected-fixture-secret: could not be found")
		status, _ := strconv.Atoi(os.Getenv("PLANE_CLI_TEST_SECURITY_STATUS"))
		os.Exit(status)
	}
	fail := func(err error) {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) < 2 {
		fail(errors.New("missing command"))
	}
	args = args[1:]
	path := os.Getenv("PLANE_CLI_TEST_SECURITY_PATH")
	type item struct{ Key, Value string }
	if reflect.DeepEqual(args, []string{"-i"}) {
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			fail(err)
		}
		if len(input) >= 4096 {
			fail(errors.New("interactive command overflow"))
		}
		quoted := `("(?:[^"\\]|\\.)*")`
		pattern := regexp.MustCompile(`^add-generic-password -U -s ` + quoted + ` -a ` + quoted + ` -w ` + quoted + "\n$")
		parts := pattern.FindStringSubmatch(string(input))
		if len(parts) != 4 {
			fail(errors.New("invalid interactive command"))
		}
		values := make([]string, 3)
		for i := range values {
			values[i], err = strconv.Unquote(parts[i+1])
			if err != nil {
				fail(err)
			}
		}
		if values[0] != "foae-plane-cli" || !strings.HasPrefix(values[2], "foae-plane-cli-base64-v1:") {
			fail(errors.New("incorrect service or encoding"))
		}
		if mode == "interactive-error" {
			// security's interactive loop reports the subcommand error but can
			// still exit zero when it subsequently reaches stdin EOF.
			_, _ = io.WriteString(os.Stderr, "SecKeychainItemModifyAttributesAndData: reflected-fixture-secret")
			os.Exit(0)
		}
		data, _ := json.Marshal(item{Key: values[1], Value: values[2]})
		if err := os.WriteFile(path, data, 0600); err != nil {
			fail(err)
		}
		os.Exit(0)
	}
	if len(args) != 5 || args[1] != "-s" || args[2] != "foae-plane-cli" {
		fail(errors.New("incorrect command arguments"))
	}
	if mode == "verify-error" && args[0] == "find-generic-password" {
		_, _ = io.WriteString(os.Stderr, "locked: reflected-fixture-secret")
		os.Exit(1)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		os.Exit(44)
	}
	if err != nil {
		fail(err)
	}
	var stored item
	if err := json.Unmarshal(data, &stored); err != nil {
		fail(err)
	}
	if stored.Key != args[4] {
		os.Exit(44)
	}
	switch {
	case args[0] == "find-generic-password" && args[3] == "-wa":
		_, _ = fmt.Fprintln(os.Stdout, stored.Value)
	case args[0] == "delete-generic-password" && args[3] == "-a":
		if err := os.Remove(path); err != nil {
			fail(err)
		}
	default:
		fail(errors.New("incorrect operation"))
	}
	os.Exit(0)
}

func TestSecurityKeyringRoundTripAndDeletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "item")
	runner := fakeSecurityCommand("store", path, "", 0)
	secret := "fixture-secret\x00\n\r unicode: 雪 ' \" \\"
	key := "opaque ' \"quoted\" \\ key"
	k := securityKeyring{command: func(ctx context.Context, args ...string) *exec.Cmd {
		if strings.Contains(strings.Join(args, " "), secret) || strings.Contains(strings.Join(args, " "), base64.StdEncoding.EncodeToString([]byte(secret))) {
			t.Fatal("secret appeared in subprocess argv")
		}
		return runner(ctx, args...)
	}}
	ctx := context.Background()
	if _, err := k.Get(ctx, key); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("initial Get error = %v", err)
	}
	if err := k.Set(ctx, key, secret); err != nil {
		t.Fatal(err)
	}
	got, err := k.Get(ctx, key)
	if err != nil || got != secret {
		t.Fatalf("round trip failed: equal=%v error=%v", got == secret, err)
	}
	if err := k.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Get(ctx, key); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("Get after Delete error = %v", err)
	}
	if err := k.Delete(ctx, key); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("Delete missing error = %v", err)
	}
}

func TestSecurityKeyringSetVerifiesInteractiveSuccess(t *testing.T) {
	for _, scenario := range []struct {
		name, initial, mode string
	}{
		{name: "missing", mode: "interactive-error"},
		{name: "stale", initial: encodeKeyringSecret("old-fixture-secret"), mode: "interactive-error"},
		{name: "corrupt", initial: "corrupt-fixture-secret", mode: "interactive-error"},
		{name: "read-denied", mode: "verify-error"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "item")
			if scenario.initial != "" {
				data, err := json.Marshal(struct{ Key, Value string }{"key", scenario.initial})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			k := securityKeyring{command: fakeSecurityCommand(scenario.mode, path, "", 0)}
			err := k.Set(context.Background(), "key", "new-fixture-secret")
			if err == nil || errors.Is(err, ErrCredentialNotFound) || strings.Contains(err.Error(), "fixture-secret") {
				t.Fatalf("unverified write succeeded or leaked/classified diagnostics: %v", err)
			}
			if scenario.name == "stale" {
				// An unsuccessful update must not delete the prior credential.
				k.command = fakeSecurityCommand("store", path, "", 0)
				got, err := k.Get(context.Background(), "key")
				if err != nil || got != "old-fixture-secret" {
					t.Fatalf("failed update damaged prior credential: %v", err)
				}
			}
		})
	}
}

func TestSecurityKeyringSetCancellationDuringVerification(t *testing.T) {
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
	store := fakeSecurityCommand("store", filepath.Join(t.TempDir(), "item"), "", 0)
	block := fakeSecurityCommand("block", "", "", 0)
	var child *exec.Cmd
	k := securityKeyring{command: func(ctx context.Context, args ...string) *exec.Cmd {
		if reflect.DeepEqual(args, []string{"-i"}) {
			return store(ctx, args...)
		}
		child = block(ctx, args...)
		child.ExtraFiles = []*os.File{readyWrite}
		return child
	}}
	completed := make(chan error, 1)
	go func() { completed <- k.Set(ctx, "key", "fixture-secret") }()
	ready := make(chan error, 1)
	go func() { var b [1]byte; _, err := io.ReadFull(readyRead, b[:]); ready <- err }()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case err := <-completed:
		t.Fatalf("Set returned before verification: %v", err)
	case <-watchdog.Done():
		t.Fatal("verification did not become ready")
	}
	cancel()
	select {
	case err := <-completed:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("verification cancellation error = %v", err)
		}
	case <-watchdog.Done():
		t.Fatal("cancellation did not stop verification")
	}
	if child.ProcessState == nil {
		t.Fatal("Set returned before reaping verification child")
	}
	if _, err := child.Process.Wait(); err == nil {
		t.Fatal("verification child was not already reaped")
	}
}

func TestSecurityKeyringRejectsOversizeBeforeLaunch(t *testing.T) {
	launches := 0
	k := securityKeyring{command: func(context.Context, ...string) *exec.Cmd {
		launches++
		return nil
	}}
	base := len("add-generic-password -U -s \"foae-plane-cli\" -a \"\" -w \"foae-plane-cli-base64-v1:\"\n")
	for _, key := range []string{strings.Repeat("k", 4096-base), strings.Repeat("k", 4097-base)} {
		if err := k.Set(context.Background(), key, ""); err == nil {
			t.Fatal("oversized command accepted")
		}
	}
	if err := k.Set(context.Background(), "key", strings.Repeat("secret", 4096)); err == nil {
		t.Fatal("oversized secret accepted")
	}
	if launches != 0 {
		t.Fatalf("oversized input launched %d processes", launches)
	}
	k.command = fakeSecurityCommand("store", filepath.Join(t.TempDir(), "item"), "", 0)
	if err := k.Set(context.Background(), strings.Repeat("k", 4095-base), ""); err != nil {
		t.Fatalf("maximum safe command failed: %v", err)
	}
}

func TestSecurityKeyringRejectsCommandInjectionBeforeLaunch(t *testing.T) {
	k := securityKeyring{command: func(context.Context, ...string) *exec.Cmd {
		t.Fatal("invalid key launched process")
		return nil
	}}
	for _, key := range []string{"key\ncommand", "key\rcommand", "key\x00command"} {
		if _, err := k.Get(context.Background(), key); err == nil {
			t.Fatal("Get accepted invalid key")
		}
		if err := k.Set(context.Background(), key, "secret"); err == nil {
			t.Fatal("Set accepted invalid key")
		}
		if err := k.Delete(context.Background(), key); err == nil {
			t.Fatal("Delete accepted invalid key")
		}
	}
}

func TestSecurityKeyringErrorsNeverReflectOutput(t *testing.T) {
	for _, status := range []int{1, 44, 128} {
		for _, operation := range []string{"get", "set", "delete"} {
			t.Run(fmt.Sprintf("%s/%d", operation, status), func(t *testing.T) {
				k := securityKeyring{command: fakeSecurityCommand("output", "", "reflected-fixture-secret: could not be found", status)}
				var err error
				switch operation {
				case "get":
					var value string
					value, err = k.Get(context.Background(), "key")
					if value != "" {
						t.Fatal("failed Get returned output")
					}
				case "set":
					err = k.Set(context.Background(), "key", "fixture-secret")
				case "delete":
					err = k.Delete(context.Background(), "key")
				}
				missing := status == 44 && operation != "set"
				if err == nil || errors.Is(err, ErrCredentialNotFound) != missing || strings.Contains(err.Error(), "fixture-secret") || strings.Contains(err.Error(), "could not be found") {
					t.Fatalf("unsafe or incorrectly classified failure: %v", err)
				}
			})
		}
	}
	for _, output := range []string{"plaintext-fixture-secret\n", "foae-plane-cli-base64-v1:not-base64\n", strings.Repeat("fixture-secret", 1000)} {
		k := securityKeyring{command: fakeSecurityCommand("output", "", output, 0)}
		got, err := k.Get(context.Background(), "key")
		if got != "" || err == nil || errors.Is(err, ErrCredentialNotFound) || strings.Contains(err.Error(), "fixture-secret") {
			t.Fatalf("invalid output accepted or reflected: %v", err)
		}
	}
	k := securityKeyring{command: func(ctx context.Context, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, filepath.Join(t.TempDir(), "reflected-fixture-secret"))
	}}
	if _, err := k.Get(context.Background(), "key"); err == nil || strings.Contains(err.Error(), "fixture-secret") {
		t.Fatalf("start failure accepted or reflected: %v", err)
	}
}

func TestSecurityKeyringCancellationReapsChild(t *testing.T) {
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
	var child *exec.Cmd
	runner := fakeSecurityCommand("block", "", "", 0)
	k := securityKeyring{command: func(ctx context.Context, args ...string) *exec.Cmd {
		child = runner(ctx, args...)
		child.ExtraFiles = []*os.File{readyWrite}
		return child
	}}
	completed := make(chan error, 1)
	go func() { completed <- k.Set(ctx, "key", "fixture-secret") }()
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
		t.Fatal("Run returned without waiting for child")
	}
	if _, err := child.Process.Wait(); err == nil {
		t.Fatal("child was not already reaped")
	}
	k.command = func(context.Context, ...string) *exec.Cmd {
		t.Fatal("pre-cancelled operation launched process")
		return nil
	}
	if err := k.Set(ctx, "key", "fixture-secret"); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancelled Set error = %v", err)
	}
}
