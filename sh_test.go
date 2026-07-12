package command_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"lesiw.io/command"
	"lesiw.io/command/mem"
)

func TestShellConcurrentUse(t *testing.T) {
	ctx := t.Context()
	sh := command.Shell(mem.Machine())
	sh.Handle("echo", mem.Machine())

	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = sh.OS(ctx)
			_ = sh.Arch(ctx)
			_ = sh.FS()
			_, err := sh.Read(ctx, "echo", "hi")
			if err != nil {
				t.Errorf("Read: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestShellBasic(t *testing.T) {
	ctx, sh := context.Background(), command.Shell(mem.Machine())
	sh.Handle("echo", mem.Machine())
	out, err := sh.Read(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("echo failed: %v", err)
	}
	if got, want := strings.TrimSpace(string(out)), "hello"; got != want {
		t.Errorf("echo output = %q, want %q", got, want)
	}
	err = sh.Exec(ctx, "cat", "file")
	if err == nil {
		t.Fatal("expected error for unregistered command")
	}
	if !strings.Contains(err.Error(), "command not found") {
		t.Errorf("error should mention 'command not found', got: %v", err)
	}
}

func TestShellMultipleRoutes(t *testing.T) {
	ctx := context.Background()
	m1, m2 := mem.Machine(), mem.Machine()
	sh := command.Shell(mem.Machine()).
		Handle("echo", m1).
		Handle("cat", m2)
	out, err := sh.Read(ctx, "echo", "test")
	if err != nil {
		t.Fatalf("echo failed: %v", err)
	}
	if got, want := out, "test"; got != want {
		t.Errorf("echo output = %q, want %q", got, want)
	}
	_, _ = sh.Read(ctx, "cat")
	err = sh.Exec(ctx, "tr")
	if err == nil {
		t.Fatal("expected error for unregistered tr")
	}
}

func TestHandleCreateShell(t *testing.T) {
	ctx, m := context.Background(), mem.Machine()
	m = command.Handle(m, "echo", mem.Machine())
	out, err := command.Read(ctx, m, "echo", "registered")
	if err != nil {
		t.Fatalf("echo failed: %v", err)
	}
	if got, want := out, "registered"; got != want {
		t.Errorf("echo output = %q, want %q", got, want)
	}
	out, err = command.Read(ctx, m, "cat")
	if err != nil {
		t.Fatalf("cat should fall back to mem.Machine: %v", err)
	}
	if got, want := out, ""; got != want {
		t.Errorf("cat output = %q, want %q", got, want)
	}
}

func TestHandleExistingShell(t *testing.T) {
	ctx := context.Background()
	m1, m2 := mem.Machine(), mem.Machine()
	sh := command.Shell(mem.Machine()).
		Handle("echo", m1).
		Handle("cat", m2)
	out, err := sh.Read(ctx, "echo", "from m1")
	if err != nil {
		t.Fatalf("echo failed: %v", err)
	}
	if !strings.Contains(string(out), "from m1") {
		t.Errorf("echo should route to m1")
	}
	_, _ = sh.Read(ctx, "cat")
}

func TestHandleChaining(t *testing.T) {
	ctx, m := context.Background(), mem.Machine()
	m = command.Handle(m, "echo", mem.Machine())
	m = command.Handle(m, "tee", mem.Machine())
	out, err := command.Read(ctx, m, "echo", "test")
	if err != nil {
		t.Fatalf("echo failed: %v", err)
	}
	if got, want := out, "test"; got != want {
		t.Errorf("echo output = %q, want %q", got, want)
	}
	err = command.Do(ctx, m, "cat")
	if err != nil {
		t.Fatalf("cat should fall back to mem.Machine: %v", err)
	}
}

func TestHandleFuncBasic(t *testing.T) {
	ctx, m := context.Background(), mem.Machine()
	m = command.HandleFunc(m, "echo",
		func(ctx context.Context, args ...string) command.Buffer {
			m := mem.Machine()
			if len(args) > 1 {
				args[1] = "PREFIX: " + args[1]
			}
			return m.Command(ctx, args...)
		})
	out, err := command.Read(ctx, m, "echo", "test")
	if err != nil {
		t.Fatalf("echo failed: %v", err)
	}
	if got, want := out, "PREFIX: test"; got != want {
		t.Errorf("echo output = %q, want %q", got, want)
	}
}

func TestHandleFuncWithFunc(t *testing.T) {
	ctx := context.Background()
	greeter := command.MachineFunc(
		func(ctx context.Context, args ...string) command.Buffer {
			if len(args) > 1 {
				return strings.NewReader("Hello, " + args[1])
			}
			return strings.NewReader("Hello, World")
		})
	m := command.Handle(mem.Machine(), "greet", greeter)
	out, err := command.Read(ctx, m, "greet", "Alice")
	if err != nil {
		t.Fatalf("greet failed: %v", err)
	}
	if got, want := out, "Hello, Alice"; got != want {
		t.Errorf("greet output = %q, want %q", got, want)
	}
}

func TestUnshellBasic(t *testing.T) {
	ctx, core := t.Context(), mem.Machine()
	shell := command.Shell(core)
	unshelled := command.Unshell(shell)
	out, err := command.Read(ctx, unshelled, "echo", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := out, "hello"; got != want {
		t.Errorf("Unshell() fallback got %q, want %q", got, want)
	}
	result := command.Unshell(core)
	if result != core {
		t.Error("Unshell() of non-Unsheller machine should return itself")
	}
}

func TestUnshellNested(t *testing.T) {
	core := mem.Machine()
	shell1 := command.Shell(core)
	shell2 := command.Shell(shell1)
	unshelled := command.Unshell(shell2)
	if unshelled != shell1 {
		t.Error("Unshell(shell2) should return shell1")
	}
	unshelled2 := command.Unshell(unshelled)
	if unshelled2 != core {
		t.Error("Unshell(shell1) should return core")
	}
}

func TestUnshellWhitelisting(t *testing.T) {
	ctx, sh := t.Context(), command.Shell(mem.Machine())
	sh = sh.Handle("echo", sh.Unshell())
	out, err := command.Read(ctx, sh, "echo", "hello")
	if err != nil {
		t.Fatalf("echo command should work: %v", err)
	}
	if got, want := out, "hello"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	_, err = command.Read(ctx, sh, "cat")
	if err == nil {
		t.Error("cat command should fail - not whitelisted")
	}
	if !command.NotFound(err) {
		t.Errorf("expected NotFound error, got: %v", err)
	}
}

func TestProbesOS_PiercesShell(t *testing.T) {
	ctx := t.Context()
	core := command.MachineFunc(func(
		_ context.Context, args ...string,
	) command.Buffer {
		if len(args) == 2 && args[0] == "uname" && args[1] == "-s" {
			return strings.NewReader("Linux\n")
		}
		return command.Fail(&command.Error{
			Err: fmt.Errorf("command not found: %s", args[0]),
		})
	})
	sh := command.Shell(core)
	if got, want := command.OS(ctx, sh), "linux"; got != want {
		t.Errorf("OS() should pierce Shell, got %q, want %q", got, want)
	}
}

func TestProbesArch_PiercesShell(t *testing.T) {
	ctx := t.Context()
	core := command.MachineFunc(func(
		_ context.Context, args ...string,
	) command.Buffer {
		if len(args) == 2 && args[0] == "uname" && args[1] == "-m" {
			return strings.NewReader("x86_64\n")
		}
		return command.Fail(&command.Error{
			Err: fmt.Errorf("command not found: %s", args[0]),
		})
	})
	sh := command.Shell(core)
	if got, want := command.Arch(ctx, sh), "amd64"; got != want {
		t.Errorf("Arch() should pierce Shell, got %q, want %q", got, want)
	}
}

func TestProbesEnv_PiercesShell(t *testing.T) {
	ctx := t.Context()
	core := command.MachineFunc(func(
		_ context.Context, args ...string,
	) command.Buffer {
		if len(args) == 2 && args[0] == "printenv" && args[1] == "HOME" {
			return strings.NewReader("/\n")
		}
		return command.Fail(&command.Error{
			Err: fmt.Errorf("command not found: %s", args[0]),
		})
	})
	sh := command.Shell(core)
	// Env() should call printenv on sh.m, not sh itself.
	if got, want := command.Env(ctx, sh, "HOME"), "/"; got != want {
		t.Errorf("Env() should pierce Shell, got %q, want %q", got, want)
	}
}

func TestNotFoundDetectsCommandNotFound(t *testing.T) {
	ctx, sh := t.Context(), command.Shell(mem.Machine())
	_, err := command.Read(ctx, sh, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
	if !command.NotFound(err) {
		t.Errorf("NotFound() should return true for command not found error")
	}
	sh = sh.Handle("fail", command.MachineFunc(
		func(ctx context.Context, args ...string) command.Buffer {
			return command.Fail(fmt.Errorf("regular error"))
		},
	))
	_, err = command.Read(ctx, sh, "fail")
	if err == nil {
		t.Fatal("expected error from fail command")
	}
	if command.NotFound(err) {
		t.Error("NotFound() should return false for regular errors")
	}
}
