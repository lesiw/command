package command

import (
	"errors"
	"io"
)

// ErrClosed is returned when attempting to read from or write to a closed
// reader or writer.
var ErrClosed = errors.New("command: write to closed buffer")

// ErrReadOnly is returned when attempting to write to a read-only command.
var ErrReadOnly = errors.New("command: write to read-only buffer")

// Buffer represents a command's execution.
// Buffers provide read access to command output.
// Reading drives execution and returns output until the command completes.
//
// Buffers may implement additional interfaces for extended capabilities:
//   - [AttachBuffer] - connect to controlling terminal
//   - [LogBuffer] - capture diagnostic output
//   - [WriteBuffer] - provide input to the command
type Buffer interface {
	// Read reads output from the command.
	// The command starts on first Read and completes at EOF.
	// Implementations must return EOF when the command terminates.
	// Multiple reads may be required to consume all output.
	io.Reader
}

// WriteBuffer is an optional interface for buffers that accept input.
// Buffers that accept input allow writing to the command's stdin.
//
// Note that Close closes stdin, not stdout.
// The buffer must still be read to EOF to observe command completion.
type WriteBuffer interface {
	Buffer

	// Write writes data to the command's stdin.
	// The command starts on first Write if it hasn't started from Read.
	// Implementations should buffer or stream data to the command's stdin.
	io.Writer

	// Close closes the command's stdin, signaling EOF to the command.
	// This does NOT close stdout - the buffer must still be read to EOF.
	// After Close, subsequent Write calls must return an error.
	// Implementations should wait for stdin close to propagate to the command.
	io.Closer
}

// AttachBuffer is an optional interface for terminal-attached buffers.
// Terminal-attached buffers connect the command directly to the terminal
// for interactive use.
//
// After calling Attach, the buffer must still be readable exactly once
// to observe command completion, but the Read must return 0 bytes and EOF.
type AttachBuffer interface {
	Buffer

	// Attach connects the command to the controlling terminal.
	// Both stdin and stdout must be connected to allow interactive use.
	// The command should start immediately if not already started.
	//
	// After Attach returns, the buffer must remain readable exactly once.
	// The single Read call must block until the command completes,
	// then return 0 bytes read and io.EOF.
	//
	// Implementations must handle terminal control sequences, raw mode,
	// and proper cleanup of terminal state.
	Attach() error
}

// LogBuffer is an optional interface for buffers with diagnostic output.
// Buffers with diagnostic output can capture stderr separately from stdout.
type LogBuffer interface {
	Buffer

	// Log sets the destination for diagnostic output (stderr).
	// Implementations must write all stderr output to w.
	// This is typically called before any Read to ensure stderr is captured.
	// Multiple Log calls should replace the previous destination.
	Log(io.Writer)
}

// Attach attaches buf to the controlling terminal if it implements
// [AttachBuffer].
// Does nothing if buf does not implement AttachBuffer.
func Attach(buf Buffer) error {
	if a, ok := buf.(AttachBuffer); ok {
		return a.Attach()
	}
	return nil
}

// Log sets the log destination for buf if it implements [LogBuffer].
// Does nothing if buf does not implement LogBuffer.
func Log(buf Buffer, w io.Writer) {
	if l, ok := buf.(LogBuffer); ok {
		l.Log(w)
	}
}
