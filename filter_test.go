package command_test

import (
	"io"
	"strings"
	"testing"

	"lesiw.io/command"
	"lesiw.io/command/mem"
)

func TestFilterReadFromClosesStdin(t *testing.T) {
	m, ctx := mem.Machine(), t.Context()

	// Create a command that reads from stdin and writes to stdout
	filter := command.NewFilter(ctx, m, "cat")

	// Use io.Copy which should trigger ReadFrom and auto-close stdin
	src := strings.NewReader("test data")
	n, err := io.Copy(filter, src)
	if err != nil {
		t.Fatalf("io.Copy() error = %v, want nil", err)
	}
	if got, want := n, int64(9); got != want {
		t.Errorf("io.Copy() = %d bytes, want %d", got, want)
	}

	// Now read from the command - should get the data and EOF
	// (indicating stdin was closed properly)
	buf, err := io.ReadAll(filter)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v, want nil", err)
	}
	if got, want := string(buf), "test data"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
