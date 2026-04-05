package ssh

import (
	"testing"

	"lesiw.io/command"
)

// Run with -parallel=1: fuzz workers are separate processes that share the
// same SSH container, and concurrent sshpass invocations race each other
// (SSH askpass fallback produces spurious auth failures).
func FuzzMachineArgQuoting(f *testing.F) {
	sshm := sshMachine(f)
	f.Fuzz(func(t *testing.T, arg string) {
		for i := range len(arg) {
			switch arg[i] {
			case 0:
				// POSIX argv can't carry null bytes.
				t.Skip("null byte")
			case '\r':
				// Stripped by SSH/terminal layer.
				t.Skip("carriage return")
			case '\n':
				// Trailing newlines stripped by Read.
				t.Skip("newline")
			}
		}
		got, err := command.Read(t.Context(), sshm, "printf", "%s", arg)
		if err != nil {
			t.Errorf("command.Read(printf, %%s, %q) err: %v", arg, err)
		}
		if got != arg {
			t.Errorf("got %q, want %q", got, arg)
		}
	})
}
