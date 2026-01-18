package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"lesiw.io/fs"
)

// A [Machine] executes commands.
//
// Machines may implement additional interfaces for extended capabilities:
//   - [ArchMachine] - architecture detection
//   - [FSMachine] - filesystem access
//   - [OSMachine] - OS detection
//   - [ShutdownMachine] - graceful shutdown
type Machine interface {
	// Command instantiates a command with the given context and arguments.
	// Environment variables are extracted from ctx using Envs.
	// The returned Buffer represents the command's execution.
	// Reading to EOF drives command execution to completion.
	Command(ctx context.Context, arg ...string) Buffer
}

// FSMachine is an optional interface that allows a Machine to provide its own
// filesystem implementation.
//
// When FS() returns a non-nil fs.FS, the command.FS() function will use this
// filesystem instead of creating a command-based one.
type FSMachine interface {
	Machine

	// FS returns a fs.FS for this Machine.
	FS() fs.FS
}

// OSMachine is an optional interface that allows a Machine to provide its own
// OS detection implementation.
//
// When OS() returns a non-empty string, the command.OS() function will use
// this value instead of probing the machine with commands.
type OSMachine interface {
	Machine

	// OS returns the operating system this Machine is running.
	OS(ctx context.Context) string
}

// ArchMachine is an optional interface that allows a Machine to provide
// its own architecture detection implementation.
//
// When Arch() returns a non-empty string, the command.Arch() function will use
// this value instead of probing the machine with commands.
type ArchMachine interface {
	Machine

	// Arch returns the architecture type of this Machine.
	Arch(ctx context.Context) string
}

// ShutdownMachine is an optional interface that allows a Machine to provide
// cleanup functionality.
//
// When a Machine implements ShutdownMachine, the command.Shutdown() function
// will call Shutdown(ctx) to clean up resources.
//
// This is useful for machines that hold resources like network connections
// or container instances that need explicit cleanup.
type ShutdownMachine interface {
	Machine

	// Shutdown releases any resources held by the machine.
	//
	// The context passed to Shutdown may lack cancellation, as the
	// command.Shutdown helper derives a context using context.WithoutCancel
	// to ensure cleanup can complete even after the parent context is
	// canceled.
	//
	// Because the context may not be canceled, implementations must ensure
	// all operations performed by Shutdown do not block indefinitely by
	// deriving their own context with an appropriate timeout.
	Shutdown(ctx context.Context) error
}

// Shutdown shuts down the machine if it implements [ShutdownMachine].
// Returns nil if the machine does not implement ShutdownMachine.
//
// The context passed to Shutdown is derived using context.WithoutCancel to
// ensure cleanup can complete even after the parent context is canceled.
func Shutdown(ctx context.Context, m Machine) error {
	if closer, ok := m.(ShutdownMachine); ok {
		return closer.Shutdown(context.WithoutCancel(ctx))
	}
	return nil
}

// Exec executes a command and waits for it to complete.
// The command's output is attached to the controlling terminal.
//
// Unlike Read, errors returned by Exec will not include log output.
func Exec(ctx context.Context, m Machine, args ...string) error {
	r := m.Command(ctx, args...)
	trace(r)
	return exec(r)
}

// Read executes a command and returns its output as a string.
// Trailing newlines are stripped from the output.
// For exact output, use [io.ReadAll].
//
// If the command fails, the error will contain an exit code and log output.
func Read(ctx context.Context, m Machine, args ...string) (string, error) {
	r := m.Command(ctx, args...)

	var buf bytes.Buffer
	if logger, ok := r.(LogBuffer); ok {
		logger.Log(&buf)
	}

	trace(r)
	out, err := io.ReadAll(r)

	// Convert to string then strip trailing newlines
	// (like shell $() behavior)
	result := string(out)
	result = strings.TrimRight(result, "\r\n")

	if e := new(Error); err != nil && buf.Len() > 0 && errors.As(err, &e) {
		e.Log = buf.Bytes()
	}

	return result, err
}

// Do executes a command for its side effects, discarding output.
// Only the error status is returned.
//
// If the command fails, the error will contain exit code and log output.
func Do(ctx context.Context, m Machine, args ...string) error {
	r := m.Command(ctx, args...)

	var buf bytes.Buffer
	if logger, ok := r.(LogBuffer); ok {
		logger.Log(&buf)
	}

	trace(r)
	_, err := io.Copy(io.Discard, r)

	if e := new(Error); err != nil && buf.Len() > 0 && errors.As(err, &e) {
		e.Log = buf.Bytes()
	}

	return err
}

// exec attaches a command to the controlling terminal.
// If the command implements AttachBuffer, it calls that method.
// Otherwise, it streams both stdout and stderr.
func exec(buf Buffer) error {
	if attacher, ok := buf.(AttachBuffer); ok {
		if err := attacher.Attach(); err != nil {
			return err
		}
	}
	if logger, ok := buf.(LogBuffer); ok {
		logger.Log(stderr)
	}
	_, err := io.Copy(stdout, buf)
	return err
}

func trace(buf Buffer) {
	if stringer, ok := buf.(fmt.Stringer); ok {
		s := stringer.String()
		s = strings.TrimRight(s, "\n")
		if s != "" {
			_, _ = fmt.Fprintf(Trace, "%s\n", s)
		}
	} else {
		_, _ = fmt.Fprintf(Trace, "%v\n", buf)
	}
}

// probeRead executes a command using Read, automatically unshelling the
// machine and retrying if the command is not found. This loops through
// machine layers until either:
//   - The command succeeds
//   - The command fails with a non-NotFound error
//   - The machine doesn't implement Unsheller (reached bottom layer)
func probeRead(
	ctx context.Context, m Machine, args ...string,
) (output string, err error) {
	for {
		output, err = Read(ctx, m, args...)
		if err == nil || !NotFound(err) {
			return
		}
		if u, ok := m.(Unsheller); ok {
			if unshelled := u.Unshell(); unshelled != nil {
				m = unshelled
				continue
			}
		}
		return
	}
}
