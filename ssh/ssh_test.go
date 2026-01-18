package ssh

import (
	"io"
	"strings"
	"testing"

	"lesiw.io/command"
	"lesiw.io/command/ctr"
	"lesiw.io/command/mock"
	"lesiw.io/command/sys"
)

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

	// Should just have: user@host echo hello
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %v", args)
	}
	if args[0] != "user@host" || args[1] != "echo" || args[2] != "hello" {
		t.Errorf("unexpected args: %v", args)
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

	want := []string{
		"-p", "2222", "-o", "StrictHostKeyChecking=no",
		"user@host", "echo", "hello",
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
	// Check if sshpass is available
	_, err := command.Read(
		t.Context(), sys.Machine(), "sshpass", "--version",
	)
	if command.NotFound(err) {
		t.Skip("sshpass not available")
	}

	// Start an SSH container using ctr.Machine
	sshContainer := ctr.Machine(
		sys.Machine(),
		"lscr.io/linuxserver/openssh-server:latest",
		"-e", "PASSWORD_ACCESS=true",
		"-e", "USER_PASSWORD=test",
		"-e", "USER_NAME=testuser",
		"-p", "2222:2222",
	)
	t.Cleanup(func() {
		if err := command.Shutdown(t.Context(), sshContainer); err != nil {
			t.Logf("Close() error: %v", err)
		}
	})

	// Initialize container (triggers lazy start)
	if _, err := command.Read(t.Context(), sshContainer, "true"); err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	// Wait for SSH to be ready
	var sshReady bool
	for range 30 {
		if _, err := command.Read(t.Context(), sys.Machine(),
			"sshpass", "-p", "test",
			"ssh", "-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "ConnectTimeout=1",
			"-p", "2222",
			"testuser@localhost", "echo", "ready"); err == nil {
			sshReady = true
			break
		}
		_, _ = command.Read(t.Context(), sys.Machine(), "sleep", "1")
	}
	if !sshReady {
		t.Skip("SSH container did not become ready in time")
	}

	// Create ssh.Machine with sshpass for authentication
	sshm := Machine(sys.Machine(),
		"sshpass", "-p", "test",
		"ssh", "-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-p", "2222",
		"testuser@localhost")

	// Test env var handling
	ctx := command.WithEnv(t.Context(), map[string]string{
		"TEST_VAR": "hello_ssh",
	})

	result, err := command.Read(ctx, sshm, "printenv", "TEST_VAR")
	if err != nil {
		t.Fatalf("printenv failed: %v", err)
	}

	if result != "hello_ssh" {
		t.Errorf("expected %q, got %q", "hello_ssh", result)
	}
}

func TestMachineStreaming(t *testing.T) {
	// Check if sshpass is available
	_, err := command.Read(
		t.Context(), sys.Machine(), "sshpass", "--version",
	)
	if command.NotFound(err) {
		t.Skip("sshpass not available")
	}

	// Start an SSH container
	sshContainer := ctr.Machine(
		sys.Machine(),
		"lscr.io/linuxserver/openssh-server:latest",
		"-e", "PASSWORD_ACCESS=true",
		"-e", "USER_PASSWORD=test",
		"-e", "USER_NAME=testuser",
		"-p", "2222:2222",
	)
	t.Cleanup(func() {
		if err := command.Shutdown(t.Context(), sshContainer); err != nil {
			t.Logf("Close() error: %v", err)
		}
	})

	// Initialize container (triggers lazy start)
	if _, err := command.Read(t.Context(), sshContainer, "true"); err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	// Wait for SSH to be ready
	var sshReady bool
	for range 30 {
		if _, err := command.Read(t.Context(), sys.Machine(),
			"sshpass", "-p", "test",
			"ssh", "-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "ConnectTimeout=1",
			"-p", "2222",
			"testuser@localhost", "echo", "ready"); err == nil {
			sshReady = true
			break
		}
		_, _ = command.Read(t.Context(), sys.Machine(), "sleep", "1")
	}
	if !sshReady {
		t.Skip("SSH container did not become ready in time")
	}

	// Create ssh.Machine
	sshm := Machine(sys.Machine(),
		"sshpass", "-p", "test",
		"ssh", "-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-p", "2222",
		"testuser@localhost")

	var out strings.Builder

	_, err = command.Copy(
		&out, strings.NewReader("hello world"),
		command.NewFilter(t.Context(), sshm, "tr", "a-z", "A-Z"),
	)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if got, want := out.String(), "HELLO WORLD"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
