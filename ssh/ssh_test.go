package ssh

import (
	"io"
	"strings"
	"testing"
	"time"

	"lesiw.io/command"
	"lesiw.io/command/ctr"
	"lesiw.io/command/mock"
	"lesiw.io/command/sys"
)

// sshMachine returns an ssh.Machine connected to an SSH container on
// localhost:2222. If no container is reachable, one is started and torn
// down at test end. An already-running container is reused (without
// cleanup) so concurrent fuzz workers share a single container.
// Tests are skipped when sshpass is unavailable.
func sshMachine(tb testing.TB) command.Machine {
	tb.Helper()
	err := command.Do(tb.Context(), sys.Machine(), "sshpass", "--version")
	if command.NotFound(err) {
		tb.Skip("sshpass not available")
	}
	err = command.Do(tb.Context(), sys.Machine(),
		"sshpass", "-p", "test",
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=1",
		"-p", "2222",
		"testuser@localhost", "echo", "ready",
	)
	if err != nil {
		ctr := ctr.Machine(sys.Machine(),
			"lscr.io/linuxserver/openssh-server:latest",
			"-e", "PASSWORD_ACCESS=true",
			"-e", "USER_PASSWORD=test",
			"-e", "USER_NAME=testuser",
			"-p", "2222:2222",
		)
		tb.Cleanup(func() { _ = command.Shutdown(tb.Context(), ctr) })
		err := command.Do(tb.Context(), ctr, "true")
		if err != nil {
			tb.Fatalf("command.Do(true) err: %v", err)
		}
		var lastErr error
		for range 30 {
			lastErr = command.Do(tb.Context(), sys.Machine(),
				"sshpass", "-p", "test",
				"ssh",
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "ConnectTimeout=1",
				"-p", "2222",
				"testuser@localhost", "echo", "ready",
			)
			if lastErr == nil {
				break
			}
			time.Sleep(time.Second)
		}
		if lastErr != nil {
			tb.Fatalf("SSH container not ready: %v", lastErr)
		}
	}
	return Machine(sys.Machine(),
		"sshpass", "-p", "test",
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-p", "2222",
		"testuser@localhost",
	)
}

func TestMachineEnvVars_Unix_Mock(t *testing.T) {
	testHookOS = func() string { return "linux" }
	t.Cleanup(func() { testHookOS = nil })

	m := new(mock.Machine)
	sshm := Machine(m, "user@host")

	ctx := command.WithEnv(t.Context(), map[string]string{
		"FOO": "bar",
		"BAZ": "qux",
	})

	cmd := sshm.Command(ctx, "printenv", "FOO")
	_, _ = io.ReadAll(cmd)

	calls := mock.Calls(m)
	if len(calls) == 0 {
		t.Fatal("expected at least one call")
	}

	// Last call should be our command (earlier calls are OS probes)
	args := calls[len(calls)-1].Args

	// Should have: user@host FOO=bar BAZ=qux printenv FOO
	if len(args) < 4 {
		t.Fatalf("expected at least 4 args, got %v", args)
	}

	if args[0] != "user@host" {
		t.Errorf("expected user@host as first arg, got %v", args[0])
	}

	// Check for env vars
	foundFoo := false
	foundBaz := false
	for _, arg := range args {
		if arg == "FOO=bar" {
			foundFoo = true
		}
		if arg == "BAZ=qux" {
			foundBaz = true
		}
	}

	if !foundFoo {
		t.Errorf("FOO=bar not found in args: %v", args)
	}
	if !foundBaz {
		t.Errorf("BAZ=qux not found in args: %v", args)
	}
}

func TestMachineEnvVars_Windows_Mock(t *testing.T) {
	testHookOS = func() string { return "windows" }
	t.Cleanup(func() { testHookOS = nil })

	m := new(mock.Machine)
	sshm := Machine(m, "user@host")

	ctx := command.WithEnv(t.Context(), map[string]string{
		"FOO": "bar",
		"BAZ": "qux",
	})

	cmd := sshm.Command(ctx, "printenv.exe", "FOO")
	_, _ = io.ReadAll(cmd)

	calls := mock.Calls(m)
	if len(calls) == 0 {
		t.Fatal("expected at least one call")
	}

	// Last call should be our command
	args := calls[len(calls)-1].Args

	if len(args) < 2 {
		t.Fatalf("expected at least 2 args, got %v", args)
	}

	if args[0] != "user@host" {
		t.Errorf("expected user@host as first arg, got %v", args[0])
	}

	// Second arg should contain "set VAR=value&" prefixes
	secondArg := args[1]
	if !strings.Contains(secondArg, "set FOO=bar&") {
		t.Errorf("second arg missing 'set FOO=bar&': %s", secondArg)
	}
	if !strings.Contains(secondArg, "set BAZ=qux&") {
		t.Errorf("second arg missing 'set BAZ=qux&': %s", secondArg)
	}
	if !strings.Contains(secondArg, "printenv.exe") {
		t.Errorf("second arg missing command: %s", secondArg)
	}
}

func TestMachineNoEnvVars_Mock(t *testing.T) {
	testHookOS = func() string { return "linux" }
	t.Cleanup(func() { testHookOS = nil })

	m := new(mock.Machine)

	sshm := Machine(m, "user@host")

	cmd := sshm.Command(t.Context(), "echo", "hello")
	_, _ = io.ReadAll(cmd)

	calls := mock.Calls(m)
	if len(calls) == 0 {
		t.Fatal("expected at least one call")
	}

	// Last call should be our command
	args := calls[len(calls)-1].Args

	// Should have: user@host <single quoted remote command>
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %v", args)
	}
	if args[0] != "user@host" {
		t.Errorf("args[0] = %q, want %q", args[0], "user@host")
	}
	want := `sh -c 'exec "$@"' sh echo hello`
	if args[1] != want {
		t.Errorf("args[1] = %q, want %q", args[1], want)
	}
}

func TestMachineSSHOptions_Mock(t *testing.T) {
	testHookOS = func() string { return "linux" }
	t.Cleanup(func() { testHookOS = nil })

	m := new(mock.Machine)

	sshm := Machine(
		m, "-p", "2222",
		"-o", "StrictHostKeyChecking=no", "user@host",
	)

	cmd := sshm.Command(t.Context(), "echo", "hello")
	_, _ = io.ReadAll(cmd)

	calls := mock.Calls(m)
	if len(calls) == 0 {
		t.Fatal("expected at least one call")
	}

	// Last call should be our command
	args := calls[len(calls)-1].Args

	// SSH options are passed as separate args, remote command is one string.
	want := []string{
		"-p", "2222", "-o", "StrictHostKeyChecking=no",
		"user@host", `sh -c 'exec "$@"' sh echo hello`,
	}
	if got := len(args); got != len(want) {
		t.Fatalf("arg count = %d, want %d: %v", got, len(want), args)
	}

	for i, w := range want {
		if got := args[i]; got != w {
			t.Errorf("arg[%d] = %q, want %q", i, got, w)
		}
	}
}

func TestMachineRealSSH(t *testing.T) {
	sshm := sshMachine(t)
	ctx := command.WithEnv(t.Context(), map[string]string{
		"TEST_VAR": "hello_ssh",
	})
	result, err := command.Read(ctx, sshm, "printenv", "TEST_VAR")
	if err != nil {
		t.Errorf("command.Read(printenv, TEST_VAR) err: %v", err)
	}
	if result != "hello_ssh" {
		t.Errorf("got %q, want %q", result, "hello_ssh")
	}
}

func TestMachineStreaming(t *testing.T) {
	sshm := sshMachine(t)
	var out strings.Builder
	_, err := command.Copy(
		&out, strings.NewReader("hello world"),
		command.NewFilter(t.Context(), sshm, "tr", "a-z", "A-Z"),
	)
	if err != nil {
		t.Errorf("command.Copy() err: %v", err)
	}
	if got, want := out.String(), "HELLO WORLD"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMachineArgQuoting(t *testing.T) {
	sshm := sshMachine(t)
	tests := []struct {
		name string
		arg  string
	}{
		{"double quotes", `hello "world"`},
		{"single quotes", "it's"},
		{"spaces", "hello   world"},
		{"dollar sign", "$notavar"},
		{"backticks", "`echo hi`"},
		{"glob", "[abc]*"},
		{"semicolon", "a; echo injected"},
		{"pipe", "a | cat"},
		{"command substitution", "$(cat /etc/passwd)"},
		{"backslash", `\`},
		{"empty", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := command.Read(
				t.Context(), sshm, "printf", "%s", tt.arg,
			)
			if err != nil {
				t.Errorf("command.Read(printf, %%s, %q) err: %v", tt.arg, err)
			}
			if got != tt.arg {
				t.Errorf("got %q, want %q", got, tt.arg)
			}
		})
	}
}
