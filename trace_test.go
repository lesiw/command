package command

import (
	"context"
	"strings"
	"testing"
)

func traceMachine(line string) Machine {
	return MachineFunc(func(_ context.Context, _ ...string) Buffer {
		return &readStringer{strings.NewReader(""), line}
	})
}

func TestCmdTraceOn(t *testing.T) {
	t.Setenv("CMDTRACE", "on")
	var buf strings.Builder
	old := Trace
	Trace = &buf
	t.Cleanup(func() { Trace = old })

	err := Do(t.Context(), traceMachine("FOO=bar echo hi"), "echo", "hi")
	if err != nil {
		t.Fatal(err)
	}

	if got, want := buf.String(), "echo hi\n"; got != want {
		t.Errorf("trace = %q, want %q", got, want)
	}
}

func TestCmdTraceFull(t *testing.T) {
	t.Setenv("CMDTRACE", "full")
	var buf strings.Builder
	old := Trace
	Trace = &buf
	t.Cleanup(func() { Trace = old })

	err := Do(t.Context(), traceMachine("FOO=bar echo hi"), "echo", "hi")
	if err != nil {
		t.Fatal(err)
	}

	if got, want := buf.String(), "FOO=bar echo hi\n"; got != want {
		t.Errorf("trace = %q, want %q", got, want)
	}
}

func TestCmdTraceUnrecognized(t *testing.T) {
	t.Setenv("CMDTRACE", "definitely-not-a-mode")
	var buf strings.Builder
	old := Trace
	Trace = &buf
	t.Cleanup(func() { Trace = old })

	err := Do(t.Context(), traceMachine(""), "echo", "hi")
	if err != nil {
		t.Fatal(err)
	}

	if got := buf.String(); got != "" {
		t.Errorf("trace = %q, want no output", got)
	}
}

func TestTraceSilentWhenDisabled(t *testing.T) {
	t.Setenv("CMDTRACE", "")
	var buf strings.Builder
	old := Trace
	Trace = &buf
	t.Cleanup(func() { Trace = old })

	err := Do(t.Context(), traceMachine("FOO=bar echo hi"), "echo", "hi")
	if err != nil {
		t.Fatal(err)
	}

	if got := buf.String(); got != "" {
		t.Errorf("trace = %q, want no output", got)
	}
}
