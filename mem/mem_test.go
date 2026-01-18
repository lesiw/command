package mem

import (
	"io"
	"testing"

	"lesiw.io/command"
	"lesiw.io/fs"
)

func TestCommandNotFound(t *testing.T) {
	m, ctx := Machine(), t.Context()

	_, err := command.Read(ctx, m, "nonexistent-command-xyz")
	if err == nil {
		t.Fatal("Read() error = nil, want error")
	}

	if got, want := command.NotFound(err), true; got != want {
		t.Errorf("NotFound() = %v, want %v", got, want)
	}
}

func TestCommandSuccess(t *testing.T) {
	m, ctx := Machine(), t.Context()

	if err := command.Exec(ctx, m, "echo", "hello"); err != nil {
		t.Errorf("Exec() error = %v, want nil", err)
	}
}

func TestEcho(t *testing.T) {
	m, ctx := Machine(), t.Context()

	cmd := m.Command(ctx, "echo", "hello", "world")
	out, err := io.ReadAll(cmd)
	if err != nil {
		t.Fatalf("echo failed: %v", err)
	}

	if got, want := string(out), "hello world\n"; got != want {
		t.Errorf("echo output = %q, want %q", got, want)
	}
}

func TestTee(t *testing.T) {
	m, ctx := Machine(), t.Context()

	if err := fs.MkdirAll(ctx, command.FS(m), "/tmp"); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	teeCmd := command.NewFilter(ctx, m, "tee", "/tmp/test.txt")

	go func() {
		_, _ = teeCmd.Write([]byte("test content"))
		_ = teeCmd.Close()
	}()

	out, err := io.ReadAll(teeCmd)
	if err != nil {
		t.Fatalf("tee read failed: %v", err)
	}

	if got, want := string(out), "test content"; got != want {
		t.Errorf("tee output = %q, want %q", got, want)
	}

	buf := m.Command(ctx, "cat", "/tmp/test.txt")
	catOut, err := io.ReadAll(buf)
	if err != nil {
		t.Fatalf("cat failed: %v", err)
	}

	if got, want := string(catOut), "test content"; got != want {
		t.Errorf("cat output = %q, want %q", got, want)
	}
}

func TestCopy(t *testing.T) {
	m, ctx := Machine(), t.Context()

	if err := fs.MkdirAll(ctx, command.FS(m), "/tmp"); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	_, err := io.Copy(
		command.NewWriter(ctx, m, "tee", "/tmp/cmdio_test.txt"),
		command.NewReader(ctx, m, "echo", "hello world"),
	)
	if err != nil {
		t.Fatalf("copy failed: %v", err)
	}

	buf := m.Command(ctx, "cat", "/tmp/cmdio_test.txt")
	out, err := io.ReadAll(buf)
	if err != nil {
		t.Fatalf("cat failed: %v", err)
	}

	if got, want := string(out), "hello world\n"; got != want {
		t.Errorf("cat output = %q, want %q", got, want)
	}
}

func TestWorkDir(t *testing.T) {
	m, ctx := Machine(), t.Context()

	if err := fs.MkdirAll(ctx, command.FS(m), "/app/data"); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	ctx = fs.WithWorkDir(ctx, "/app")

	_, err := io.Copy(
		command.NewWriter(ctx, m, "tee", "test.txt"),
		command.NewReader(ctx, m, "echo", "hello from /app"),
	)
	if err != nil {
		t.Fatalf("copy failed: %v", err)
	}

	buf := m.Command(t.Context(), "cat", "/app/test.txt")
	out, err := io.ReadAll(buf)
	if err != nil {
		t.Fatalf("cat failed: %v", err)
	}

	if got, want := string(out), "hello from /app\n"; got != want {
		t.Errorf("cat output = %q, want %q", got, want)
	}

	ctx = fs.WithWorkDir(ctx, "/app/data")

	_, err = io.Copy(
		command.NewWriter(ctx, m, "tee", "nested.txt"),
		command.NewReader(ctx, m, "echo", "hello from /app/data"),
	)
	if err != nil {
		t.Fatalf("copy failed: %v", err)
	}

	buf = m.Command(t.Context(), "cat", "/app/data/nested.txt")
	out, err = io.ReadAll(buf)
	if err != nil {
		t.Fatalf("cat nested file failed: %v", err)
	}

	if got, want := string(out), "hello from /app/data\n"; got != want {
		t.Errorf("cat nested output = %q, want %q", got, want)
	}

	buf = m.Command(ctx, "cat", "nested.txt")
	out, err = io.ReadAll(buf)
	if err != nil {
		t.Fatalf("cat with relative path failed: %v", err)
	}

	if got, want := string(out), "hello from /app/data\n"; got != want {
		t.Errorf("cat relative output = %q, want %q", got, want)
	}
}
