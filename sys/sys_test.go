package sys_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lesiw.io/command"
	"lesiw.io/command/sys"
)

func testBinary(t *testing.T) string {
	binary, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatalf("could not resolve binary path: %v", err)
	}
	return binary
}

func TestExecSuccess(t *testing.T) {
	m, ctx := sys.Machine(), t.Context()

	if os.Getenv("CMD_TEST_PROC") == "1" {
		os.Exit(0)
	}
	t.Setenv("CMD_TEST_PROC", "1")

	err := command.Exec(ctx, m, testBinary(t), "-test.run=TestExecSuccess")
	if err != nil {
		t.Errorf("Exec() = %q, want <nil>", err)
	}
}

func TestExecFailure(t *testing.T) {
	m, ctx := sys.Machine(), t.Context()

	if os.Getenv("CMD_TEST_PROC") == "1" {
		os.Exit(42)
	}
	t.Setenv("CMD_TEST_PROC", "1")

	err := command.Exec(ctx, m, testBinary(t), "-test.run=TestExecFailure")
	if err == nil {
		t.Errorf("Exec() = <nil>, want error")
	}

	var cmdErr *command.Error
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected *command.Error, got %T", err)
	}

	if cmdErr.Code != 42 {
		t.Errorf("Code = %d, want 42", cmdErr.Code)
	}
}

func TestExecBadCommand(t *testing.T) {
	m, ctx := sys.Machine(), t.Context()

	err := command.Exec(ctx, m, "this-command-does-not-exist")
	if err == nil {
		t.Errorf("Exec() = <nil>, want exec.ErrNotFound")
	} else if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("Exec() = %q, want exec.ErrNotFound", err.Error())
	}
}

func TestCallFailure(t *testing.T) {
	m, ctx := sys.Machine(), t.Context()

	if os.Getenv("CMD_TEST_PROC") == "1" {
		fmt.Println("hello world")
		fmt.Fprintln(os.Stderr, "hello stderr")
		os.Exit(42)
	}
	t.Setenv("CMD_TEST_PROC", "1")

	out, err := command.Read(ctx, m, testBinary(t),
		"-test.run=TestCallFailure",
	)
	if err == nil {
		t.Errorf("Read().error = <nil>, want error")
	}

	got := strings.TrimSpace(out)
	want := "hello world"
	if got != want {
		t.Errorf("Read().output = %q, want %q", got, want)
	}

	var cmdErr *command.Error
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected *command.Error, got %T", err)
	}

	if cmdErr.Code != 42 {
		t.Errorf("Code = %d, want 42", cmdErr.Code)
	}

	log := strings.TrimSpace(string(cmdErr.Log))
	wantLog := "hello stderr"
	if log != wantLog {
		t.Errorf("Log = %q, want %q", log, wantLog)
	}
}

func TestCallBadCommand(t *testing.T) {
	m, ctx := sys.Machine(), t.Context()

	_, err := command.Read(ctx, m, "this-command-does-not-exist")
	if err == nil {
		t.Errorf("Read().error = <nil>, want error")
	} else if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("Read().error = %q, want exec.ErrNotFound", err.Error())
	}
}

func TestContextCancellation(t *testing.T) {
	m, ctx := sys.Machine(), t.Context()

	if os.Getenv("CMD_TEST_PROC") == "1" {
		fmt.Println("READY")
		_ = os.Stdout.Sync()
		// Block forever (or until killed)
		select {}
	}
	t.Setenv("CMD_TEST_PROC", "1")

	ctx, cancel := context.WithCancel(ctx)
	cmd := m.Command(ctx, testBinary(t), "-test.run=TestContextCancellation")

	// Read output to detect when subprocess is ready
	buf := make([]byte, 1024)
	n, err := cmd.Read(buf)
	if err != nil {
		t.Fatalf("failed to read from command: %v", err)
	}
	if !strings.Contains(string(buf[:n]), "READY") {
		t.Fatalf("subprocess didn't signal ready, got: %q", buf[:n])
	}

	// Now cancel context
	cancel()

	// Command should terminate with error within timeout
	done := make(chan error, 1)
	go func() {
		_, err := io.ReadAll(cmd)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected error after context cancellation")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("command did not terminate within 10 seconds")
	}
}
