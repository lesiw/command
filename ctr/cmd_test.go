package ctr

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"lesiw.io/command"
	"lesiw.io/command/sys"
)

func alpine(t *testing.T) (m command.Machine) {
	t.Helper()

	m = Machine(sys.Machine(), "alpine")
	if _, err := command.Read(t.Context(), m, "echo", "test"); err != nil {
		t.Skipf("alpine container not available: %v", err)
	}
	t.Cleanup(func() {
		if err := command.Shutdown(t.Context(), m); err != nil {
			t.Fatalf("Machine.Shutdown() err: %v", err)
		}
	})
	return
}

func swap[T any](t *testing.T, ptr *T, val T) {
	t.Helper()
	old := *ptr
	*ptr = val
	t.Cleanup(func() { *ptr = old })
}

func TestCmdExecute(t *testing.T) {
	r := alpine(t).Command(t.Context(), "echo", "hello")

	if err := iotest.TestReader(r, []byte("hello\n")); err != nil {
		t.Errorf("iotest.TestReader(%v) err: %v", r, err)
	}
}

func TestCmdAttach(t *testing.T) {
	var (
		m    = alpine(t)
		file = ".command-attach-test"
		rm   = func() {
			_, err := command.Read(context.WithoutCancel(t.Context()),
				m, "rm", "-f", file,
			)
			if err != nil {
				t.Fatalf("command.Read(rm, -f, %v) err: %v", file, err)
			}
		}
		out bytes.Buffer
	)
	rm()
	t.Cleanup(rm)
	swap[io.Writer](t, &sys.Stdout, &out)

	r := m.Command(t.Context(), "sh", "-c", "echo attached | tee "+file)

	if a, ok := r.(command.AttachBuffer); !ok {
		t.Fatal("r is not a command.AttachBuffer")
	} else if err := a.Attach(); err != nil {
		t.Errorf("Attach() err: %v", err)
	}
	if buf, err := io.ReadAll(r); err != nil {
		t.Errorf("io.ReadAll(%v) err: %v", r, err)
	} else if got, want := string(buf), ""; got != want {
		t.Errorf("io.ReadAll(%v) = %q, want %q", r, got, want)
	}
	if got, want := out.String(), "attached\n"; got != want {
		t.Errorf("sys.Stdout = %q, want %q", got, want)
	}
	if txt, err := command.Read(t.Context(), m, "cat", file); err != nil {
		t.Errorf("command.Read(cat, %v) err: %v", file, err)
	} else if got, want := txt, "attached"; got != want {
		t.Errorf("file content = %q, want %q", got, want)
	}
}

func TestCmdString(t *testing.T) {
	r := alpine(t).Command(t.Context(), "echo", "test")

	if s, ok := r.(fmt.Stringer); !ok {
		t.Fatal("r is not a fmt.Stringer")
	} else if str := s.String(); !strings.Contains(str, "echo") {
		t.Errorf("r.String() = %q, should contain 'echo'", str)
	}
}

func TestCmdLog(t *testing.T) {
	r := alpine(t).Command(t.Context(),
		"sh", "-c", "echo error >&2; echo output",
	)
	var log strings.Builder
	if l, ok := r.(command.LogBuffer); !ok {
		t.Fatal("r is not a command.LogBuffer")
	} else {
		l.Log(&log)
	}

	if buf, err := io.ReadAll(r); err != nil {
		t.Fatalf("io.ReadAll(%v) err: %v", r, err)
	} else if got, want := string(buf), "output\n"; got != want {
		t.Errorf("io.ReadAll(%v) stdout: %q, want %q", r, got, want)
	}
	if got, want := log.String(), "error\n"; got != want {
		t.Errorf("r log = %q, want %q", got, want)
	}
}

func TestCmdClose(t *testing.T) {
	buf := command.NewFilter(t.Context(), alpine(t), "cat")

	if _, err := buf.Write([]byte("test\n")); err != nil {
		t.Fatalf("buf.Write(%q) err: %v", "test\n", err)
	}
	if err := buf.Close(); err != nil {
		t.Errorf("buf.Close() err: %v", err)
	}
}
