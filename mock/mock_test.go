package mock_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"lesiw.io/command"
	"lesiw.io/command/mock"
)

func TestMachineSingleQueue(t *testing.T) {
	m, ctx := new(mock.Machine), context.Background()
	m.Return(strings.NewReader("world"), "hello")

	want := "world"
	if got, err := command.Read(ctx, m, "hello"); err != nil {
		t.Fatal(err)
	} else if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMachineQueueRepeatsLast(t *testing.T) {
	m, ctx := new(mock.Machine), context.Background()
	m.Return(strings.NewReader("first"), "echo")
	m.Return(strings.NewReader("second"), "echo")

	// First call returns first queued value.
	want := "first"
	if got, err := command.Read(ctx, m, "echo", "test"); err != nil {
		t.Fatal(err)
	} else if got != want {
		t.Errorf("first call = %q, want %q", got, want)
	}

	// Second call returns second queued value.
	want = "second"
	if got, err := command.Read(ctx, m, "echo", "test"); err != nil {
		t.Fatal(err)
	} else if got != want {
		t.Errorf("second call = %q, want %q", got, want)
	}

	// Third call repeats last value.
	if got, err := command.Read(ctx, m, "echo", "test"); err != nil {
		t.Fatal(err)
	} else if got != want {
		t.Errorf("third call = %q, want %q", got, want)
	}
}

func TestMachineNoQueueReturnsEmpty(t *testing.T) {
	m, ctx := new(mock.Machine), context.Background()

	want := ""
	if got, err := command.Read(ctx, m, "foo"); err != nil {
		t.Fatalf("expected quiet success, got error: %v", err)
	} else if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMachineTracksInvocations(t *testing.T) {
	m, ctx := new(mock.Machine), context.Background()
	m.Return(strings.NewReader("main\n"), "git")

	gitArgs := []string{"git", "branch", "--show-current"}
	if err := command.Do(ctx, m, gitArgs...); err != nil {
		t.Fatal(err)
	}

	if got, want := len(m.Calls), 1; got != want {
		t.Fatalf("call count = %d, want %d", got, want)
	}

	call := m.Calls[0]
	if got, want := len(call.Args), 3; got != want {
		t.Fatalf("arg count = %d, want %d", got, want)
	}
	if got, want := call.Args[0], "git"; got != want {
		t.Errorf("args[0] = %q, want %q", got, want)
	}
	if got, want := call.Args[1], "branch"; got != want {
		t.Errorf("args[1] = %q, want %q", got, want)
	}
	if got, want := call.Args[2], "--show-current"; got != want {
		t.Errorf("args[2] = %q, want %q", got, want)
	}
}

func TestMachineTracksInput(t *testing.T) {
	m, ctx := new(mock.Machine), context.Background()
	m.Return(strings.NewReader("echoed\n"), "tee")

	cmd := command.NewFilter(ctx, m, "tee", "output.txt")
	if _, err := cmd.Write([]byte("test input")); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(cmd); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Close(); err != nil {
		t.Fatal(err)
	}

	if got, want := len(m.Calls), 1; got != want {
		t.Fatalf("call count = %d, want %d", got, want)
	}

	if got, want := string(m.Calls[0].Got), "test input"; got != want {
		t.Errorf("input = %q, want %q", got, want)
	}
}

func TestMachineTracksEnvironment(t *testing.T) {
	ctx := command.WithEnv(context.Background(), map[string]string{
		"FOO": "bar",
		"BAZ": "qux",
	})
	m := new(mock.Machine)
	m.Return(strings.NewReader(""), "env")

	if err := command.Do(ctx, m, "env"); err != nil {
		t.Fatal(err)
	}

	if got, want := len(m.Calls), 1; got != want {
		t.Fatalf("call count = %d, want %d", got, want)
	}

	env := m.Calls[0].Env
	if got, want := env["FOO"], "bar"; got != want {
		t.Errorf("env[FOO] = %q, want %q", got, want)
	}
	if got, want := env["BAZ"], "qux"; got != want {
		t.Errorf("env[BAZ] = %q, want %q", got, want)
	}
}

func TestMachineDifferentCommands(t *testing.T) {
	m, ctx := new(mock.Machine), context.Background()
	m.Return(strings.NewReader("main\n"), "git")
	m.Return(strings.NewReader("1.2.3\n"), "npm")

	wantGit := "main"
	if got, err := command.Read(ctx, m, "git", "branch"); err != nil {
		t.Fatal(err)
	} else if got != wantGit {
		t.Errorf("git = %q, want %q", got, wantGit)
	}

	wantNpm := "1.2.3"
	if got, err := command.Read(ctx, m, "npm", "--version"); err != nil {
		t.Fatal(err)
	} else if got != wantNpm {
		t.Errorf("npm = %q, want %q", got, wantNpm)
	}

	if got, want := len(m.Calls), 2; got != want {
		t.Fatalf("call count = %d, want %d", got, want)
	}
}

func TestMachineCustomHandler(t *testing.T) {
	m, ctx := new(mock.Machine), context.Background()

	// Override specific command with custom logic.
	sh := command.Shell(m)
	sh = sh.HandleFunc("greet",
		func(_ context.Context, args ...string) command.Buffer {
			if len(args) > 1 {
				return strings.NewReader("Hello, " + args[1] + "!")
			}
			return strings.NewReader("Hello, World!")
		})

	want := "Hello, Alice!"
	if got, err := command.Read(ctx, sh, "greet", "Alice"); err != nil {
		t.Fatal(err)
	} else if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	want = "Hello, World!"
	if got, err := command.Read(ctx, sh, "greet"); err != nil {
		t.Fatal(err)
	} else if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMachineInputCapture(t *testing.T) {
	m, ctx := new(mock.Machine), context.Background()
	m.Return(strings.NewReader("output"), "cmd")

	cmd := command.NewFilter(ctx, m, "cmd", "arg")
	if _, err := cmd.Write([]byte("input data")); err != nil {
		t.Fatal(err)
	}

	// Read to EOF then close to trigger recording.
	if _, err := io.ReadAll(cmd); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Close(); err != nil {
		t.Fatal(err)
	}

	if got, want := len(m.Calls), 1; got != want {
		t.Fatalf("call count = %d, want %d", got, want)
	}

	if got, want := string(m.Calls[0].Got), "input data"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCallsDirect(t *testing.T) {
	m, ctx := new(mock.Machine), context.Background()
	m.Return(strings.NewReader("main\n"), "git")
	m.Return(strings.NewReader("1.0.0\n"), "npm")

	if err := command.Do(ctx, m, "git", "branch"); err != nil {
		t.Fatal(err)
	}
	if err := command.Do(ctx, m, "npm", "--version"); err != nil {
		t.Fatal(err)
	}

	calls := mock.Calls(m)
	if got, want := len(calls), 2; got != want {
		t.Fatalf("call count = %d, want %d", got, want)
	}
}

func TestCallsThroughShell(t *testing.T) {
	m, ctx := new(mock.Machine), context.Background()
	m.Return(strings.NewReader("main\n"), "git")
	m.Return(strings.NewReader("1.0.0\n"), "npm")

	sh := command.Shell(m)
	sh = sh.Handle("git", sh.Unshell())
	sh = sh.Handle("npm", sh.Unshell())

	if err := command.Do(ctx, sh, "git", "branch"); err != nil {
		t.Fatal(err)
	}
	if err := command.Do(ctx, sh, "npm", "--version"); err != nil {
		t.Fatal(err)
	}

	// Should be able to get calls through the shell wrapper.
	calls := mock.Calls(sh)
	if len(calls) < 2 {
		t.Fatalf("call count = %d, want at least 2", len(calls))
	}

	// Verify we got git and npm calls.
	var hasGit, hasNpm bool
	for _, call := range calls {
		if len(call.Args) > 0 {
			if call.Args[0] == "git" {
				hasGit = true
			}
			if call.Args[0] == "npm" {
				hasNpm = true
			}
		}
	}
	if !hasGit {
		t.Error("expected git call")
	}
	if !hasNpm {
		t.Error("expected npm call")
	}
}

func TestCallsForDirect(t *testing.T) {
	m, ctx := new(mock.Machine), context.Background()
	m.Return(strings.NewReader("main\n"), "git")
	m.Return(strings.NewReader("1.0.0\n"), "npm")

	if err := command.Do(ctx, m, "git", "branch"); err != nil {
		t.Fatal(err)
	}
	if err := command.Do(ctx, m, "git", "status"); err != nil {
		t.Fatal(err)
	}
	if err := command.Do(ctx, m, "npm", "--version"); err != nil {
		t.Fatal(err)
	}

	gitCalls := mock.Calls(m, "git")
	if got, want := len(gitCalls), 2; got != want {
		t.Fatalf("git call count = %d, want %d", got, want)
	}
	if got, want := gitCalls[0].Args[1], "branch"; got != want {
		t.Errorf("first call = %q, want %q", got, want)
	}
	if got, want := gitCalls[1].Args[1], "status"; got != want {
		t.Errorf("second call = %q, want %q", got, want)
	}

	npmCalls := mock.Calls(m, "npm")
	if got, want := len(npmCalls), 1; got != want {
		t.Fatalf("npm call count = %d, want %d", got, want)
	}
}

func TestCallsForThroughShell(t *testing.T) {
	m, ctx := new(mock.Machine), context.Background()
	m.Return(strings.NewReader("main\n"), "git")
	m.Return(strings.NewReader("1.0.0\n"), "npm")

	sh := command.Shell(m)
	sh = sh.Handle("git", sh.Unshell())
	sh = sh.Handle("npm", sh.Unshell())

	if err := command.Do(ctx, sh, "git", "branch"); err != nil {
		t.Fatal(err)
	}
	if err := command.Do(ctx, sh, "git", "status"); err != nil {
		t.Fatal(err)
	}
	if err := command.Do(ctx, sh, "npm", "--version"); err != nil {
		t.Fatal(err)
	}

	// Should be able to filter calls through the shell wrapper.
	gitCalls := mock.Calls(sh, "git")
	if got, want := len(gitCalls), 2; got != want {
		t.Fatalf("git call count = %d, want %d", got, want)
	}
}

func TestCallsWithPattern(t *testing.T) {
	m, ctx := new(mock.Machine), context.Background()
	m.Return(strings.NewReader("main\n"), "git")

	if err := command.Do(ctx, m, "git", "branch"); err != nil {
		t.Fatal(err)
	}
	err := command.Do(ctx, m, "git", "branch", "--show-current")
	if err != nil {
		t.Fatal(err)
	}
	if err = command.Do(ctx, m, "git", "status"); err != nil {
		t.Fatal(err)
	}

	// Filter by command only.
	gitCalls := mock.Calls(m, "git")
	if got, want := len(gitCalls), 3; got != want {
		t.Fatalf("git call count = %d, want %d", got, want)
	}

	// Filter by command and subcommand.
	branchCalls := mock.Calls(m, "git", "branch")
	if got, want := len(branchCalls), 2; got != want {
		t.Fatalf("git branch call count = %d, want %d", got, want)
	}
	if got, want := branchCalls[0].Args[1], "branch"; got != want {
		t.Errorf("first call = %q, want %q", got, want)
	}
	if got, want := branchCalls[1].Args[1], "branch"; got != want {
		t.Errorf("second call = %q, want %q", got, want)
	}

	// More specific pattern.
	specificCalls := mock.Calls(m, "git", "branch", "--show-current")
	if got, want := len(specificCalls), 1; got != want {
		t.Fatalf("specific call count = %d, want %d", got, want)
	}
}

func TestCallsNonMock(t *testing.T) {
	// Should return nil for non-mock machines.
	if calls := mock.Calls(nil); calls != nil {
		t.Errorf("Calls(nil) = %v, want nil", calls)
	}
}

func TestMachineDo(t *testing.T) {
	m, ctx := new(mock.Machine), context.Background()

	// Register custom handler that returns command not found.
	m.Do(func(_ context.Context, args ...string) command.Buffer {
		return command.Fail(&command.Error{Err: io.EOF})
	}, "uname")

	if err := command.Do(ctx, m, "uname", "-s"); !command.NotFound(err) {
		t.Errorf("error = %v, want NotFound", err)
	}
}

func TestMachineDoConditional(t *testing.T) {
	m, ctx := new(mock.Machine), context.Background()

	// Register conditional handler based on args.
	m.Do(func(_ context.Context, args ...string) command.Buffer {
		if len(args) > 1 && args[1] == "hello" {
			return strings.NewReader("world\n")
		}
		return strings.NewReader("unknown\n")
	}, "echo")

	want := "world"
	if got, err := command.Read(ctx, m, "echo", "hello"); err != nil {
		t.Fatal(err)
	} else if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	want = "unknown"
	if got, err := command.Read(ctx, m, "echo", "goodbye"); err != nil {
		t.Fatal(err)
	} else if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMachineDoOverridesReturn(t *testing.T) {
	m, ctx := new(mock.Machine), context.Background()

	// Set up Return for echo command.
	m.Return(strings.NewReader("from Return\n"), "echo")

	// Do() should override Return().
	m.Do(func(_ context.Context, args ...string) command.Buffer {
		return strings.NewReader("from Do\n")
	}, "echo")

	want := "from Do"
	if got, err := command.Read(ctx, m, "echo", "test"); err != nil {
		t.Fatal(err)
	} else if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMachineRepeatsResetOnNewQueue(t *testing.T) {
	m, ctx := new(mock.Machine), context.Background()

	// Queue first reader.
	m.Return(strings.NewReader("first\n"), "test")

	// Call twice - should repeat.
	want := "first"
	if got, err := command.Read(ctx, m, "test"); err != nil {
		t.Fatal(err)
	} else if got != want {
		t.Errorf("first call = %q, want %q", got, want)
	}

	if got, err := command.Read(ctx, m, "test"); err != nil {
		t.Fatal(err)
	} else if got != want {
		t.Errorf("second call (repeat) = %q, want %q", got, want)
	}

	// Queue another reader.
	m.Return(strings.NewReader("second\n"), "test")

	// Call twice again - should repeat the new value.
	want = "second"
	if got, err := command.Read(ctx, m, "test"); err != nil {
		t.Fatal(err)
	} else if got != want {
		t.Errorf("third call = %q, want %q", got, want)
	}

	if got, err := command.Read(ctx, m, "test"); err != nil {
		t.Fatal(err)
	} else if got != want {
		t.Errorf("fourth call (repeat) = %q, want %q", got, want)
	}
}

func TestMachineErrorPreservation(t *testing.T) {
	m, ctx := new(mock.Machine), context.Background()

	testErr := fmt.Errorf("test error from reader")
	m.Return(iotest.ErrReader(testErr), "fail")

	if err := command.Do(ctx, m, "fail"); err == nil {
		t.Fatal("expected error, got nil")
	} else if got, want := err.Error(), testErr.Error(); got != want {
		t.Errorf("error = %q, want %q", got, want)
	}

	if err := command.Do(ctx, m, "fail"); err == nil {
		t.Fatal("expected error on repeat, got nil")
	} else if got, want := err.Error(), testErr.Error(); got != want {
		t.Errorf("repeat error = %q, want %q", got, want)
	}
}

//ignore:errcheck
func TestMachineConcurrentRecordCall(t *testing.T) {
	for range 1000 {
		m := new(mock.Machine)
		m.Return(strings.NewReader("x"), "cmd")
		command.Copy(
			io.Discard,
			strings.NewReader("x"),
			command.NewFilter(t.Context(), m, "cmd"),
		)
	}
}
