// Package mock provides a Machine implementation for testing that tracks
// invocations and allows queuing responses.
//
// Unlike testing individual commands, mock.Machine operates at the Machine
// level, tracking all command invocations and allowing response queuing with
// argument pattern matching.
//
// The mock Machine tracks every command invocation in the Calls slice,
// including arguments, environment, and input written to the command.
// Tests can inspect Calls using cmp.Diff or direct comparison.
//
// Responses are queued using Return() with optional argument patterns.
// Matching is done from most to least specific. When the queue is
// exhausted, the last response repeats indefinitely (lesiw.io/moxie).
//
//	m := new(mock.Machine)
//	m.Return(strings.NewReader("hello\n"), "echo")
//	m.Return(strings.NewReader("Linux\n"), "uname", "-s")
//	m.Return(command.Fail(&command.Error{Err: io.EOF}))  // Default for all
//
// For complex conditional behavior based on arguments, use Do() to register
// custom handlers:
//
//	m.Do(func(_ context.Context, args ...string) io.ReadWriter {
//	    if len(args) > 1 {
//	        return command.FromReader(strings.NewReader("Hello, " + args[1]))
//	    }
//	    return command.FromReader(strings.NewReader("Hello, World!"))
//	}, "greet")
//
// # Accessing Calls Through Shell
//
// When wrapping a mock Machine in a Shell, use the package-level Calls()
// function to access invocations:
//
//	m := new(mock.Machine)
//	sh := command.Shell(m)
//	// ... use sh ...
//	calls := mock.Calls(sh)            // All invocations
//	gitCalls := mock.Calls(sh, "git")  // Just git invocations
package mock

import (
	"bytes"
	"context"
	"io"
	"sync"

	"lesiw.io/command"
	"lesiw.io/fs"
	"lesiw.io/fs/memfs"
)

// Call represents a single command invocation captured by the mock Machine.
type Call struct {
	Args []string
	Env  map[string]string
	Got  []byte
}

type mockResponse struct {
	args    []string
	readers []io.Reader
}

type mockHandler struct {
	args []string
	fn   func(context.Context, ...string) command.Buffer
}

// Machine is a mock implementation of command.Machine that tracks invocations
// and allows queuing responses.
// It also provides an in-memory filesystem and controllable OS/Arch detection
// via the FSMachine, OSMachine, and ArchMachine interfaces.
type Machine struct {
	mu        sync.Mutex
	once      sync.Once
	Calls     []Call
	responses []mockResponse
	captured  map[string][]byte
	handlers  []mockHandler
	fsys      fs.FS
	os        string
	arch      string
}

// init initializes captured map and fsys if they are nil.
func (m *Machine) init() {
	m.once.Do(func() {
		m.captured = make(map[string][]byte)
		m.fsys = memfs.New()
	})
}

// FS implements the command.FSMachine interface.
// Returns the in-memory filesystem, or nil if SetFS was called with nil.
func (m *Machine) FS() fs.FS {
	m.init()
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.fsys
}

// SetFS sets the filesystem returned by FS().
// Pass nil to trigger fallback to command-based filesystem.
func (m *Machine) SetFS(fsys fs.FS) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fsys = fsys
}

// OS implements the command.OSMachine interface.
// Returns the OS set by SetOS, or "" if not set (triggers probe-based
// detection).
func (m *Machine) OS(ctx context.Context) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.os
}

// SetOS sets the operating system returned by OS().
// Pass "" to trigger fallback to probe-based detection.
func (m *Machine) SetOS(os string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.os = os
}

// Arch implements the command.ArchMachine interface.
// Returns the architecture set by SetArch, or "" if not set (triggers
// probe-based detection).
func (m *Machine) Arch(ctx context.Context) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.arch
}

// SetArch sets the architecture returned by Arch().
// Pass "" to trigger fallback to probe-based detection.
func (m *Machine) SetArch(arch string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.arch = arch
}

// Return adds a reader to the queue with optional argument matching.
// When Command() is called, responses are matched from most to least specific.
// After the queue is exhausted, the last reader's output repeats indefinitely.
//
// Return() is implemented as a special case of Do() - it registers a handler
// that manages a queue of readers for the given argument pattern.
//
// If no args are provided, the reader becomes the default for all:
//
//	m.Return(command.Fail(&command.Error{Err: errors.New("bad command")}))
//
// For specific command matching:
//
//	m.Return(strings.NewReader("hello\n"), "echo")
//	m.Return(strings.NewReader("Linux\n"), "uname", "-s")
func (m *Machine) Return(reader io.Reader, arg ...string) {
	m.init()
	m.mu.Lock()

	argCopy := append([]string(nil), arg...)
	key := argsKey(argCopy)

	for i := range m.responses {
		if argsEqual(m.responses[i].args, arg) {
			if _, ok := m.captured[key]; ok {
				delete(m.captured, key)
				m.responses[i].readers = []io.Reader{reader}
			} else {
				m.responses[i].readers = append(m.responses[i].readers, reader)
			}
			m.mu.Unlock()
			return
		}
	}

	m.responses = append(m.responses, mockResponse{
		args:    argCopy,
		readers: []io.Reader{reader},
	})

	m.mu.Unlock()

	m.Do(m.makeQueueHandler(argCopy), arg...)
}

// makeQueueHandler creates a handler function that manages the queue for
// the given argument pattern.
func (m *Machine) makeQueueHandler(arg []string) func(
	context.Context, ...string,
) command.Buffer {
	return func(ctx context.Context, args ...string) command.Buffer {
		m.mu.Lock()

		key := argsKey(arg)

		var resp *mockResponse
		for i := range m.responses {
			if argsEqual(m.responses[i].args, arg) {
				resp = &m.responses[i]
				break
			}
		}

		var reader io.Reader
		var captureBuf *bytes.Buffer

		if resp != nil {
			if len(resp.readers) > 1 {
				reader = resp.readers[0]
				resp.readers = resp.readers[1:]
				delete(m.captured, key)
			} else if captured, ok := m.captured[key]; ok {
				reader = bytes.NewReader(captured)
			} else if len(resp.readers) == 1 {
				// Set up TeeReader to capture output for repeating.
				captureBuf = &bytes.Buffer{}
				reader = io.TeeReader(resp.readers[0], captureBuf)
			}
		} else if captured, ok := m.captured[key]; ok {
			reader = bytes.NewReader(captured)
		}

		m.mu.Unlock()

		if reader == nil {
			reader = bytes.NewReader(nil)
		}

		return &mockCmd{
			machine: m,
			call: Call{
				Args: append([]string{}, args...),
				Env:  command.Envs(ctx),
			},
			reader:     reader,
			captureBuf: captureBuf,
			key:        key,
		}
	}
}

// Do registers a custom command handler with optional argument matching.
// The handler function receives the context and arguments, and must return
// a command.Buffer representing the command's behavior.
//
// Do allows mocking complex behavior like failures, conditional
// responses, or stateful commands. For simple responses, use Return.
//
// If no args are provided, the handler becomes the default for ALL commands:
//
//	m.Do(func(_ context.Context, args ...string) command.Buffer {
//	    return command.Fail(&command.Error{Err: io.EOF}) // Default for all
//	})
//
// For specific command matching:
//
//	m.Do(func(_ context.Context, args ...string) command.Buffer {
//	    return command.FromReader(strings.NewReader("Linux\n"))
//	}, "uname", "-s")
func (m *Machine) Do(
	fn func(context.Context, ...string) command.Buffer, arg ...string,
) {
	m.init()
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.handlers {
		if argsEqual(m.handlers[i].args, arg) {
			m.handlers[i].fn = fn
			return
		}
	}

	m.handlers = append(m.handlers, mockHandler{
		args: append([]string(nil), arg...),
		fn:   fn,
	})
}

// Command implements the command.Machine interface.
// All commands are routed through handlers registered via Do() or Return().
// Return() is implemented as a special case of Do() that manages queues.
func (m *Machine) Command(ctx context.Context, args ...string) command.Buffer {
	if len(args) == 0 {
		return command.Fail(io.ErrUnexpectedEOF)
	}

	m.init()
	m.mu.Lock()
	var bestHandler *mockHandler
	var bestHandlerLen int = -1

	for i := range m.handlers {
		h := &m.handlers[i]
		if argsMatch(args, h.args) {
			matchLen := len(h.args)
			if matchLen > bestHandlerLen {
				bestHandler = h
				bestHandlerLen = matchLen
			}
		}
	}

	m.mu.Unlock()

	if bestHandler != nil {
		return bestHandler.fn(ctx, args...)
	}

	return &mockCmd{
		machine: m,
		call: Call{
			Args: append([]string{}, args...),
			Env:  command.Envs(ctx),
		},
		reader: bytes.NewReader(nil),
	}
}

type mockCmd struct {
	sync.Mutex
	once       sync.Once
	machine    *Machine
	call       Call
	reader     io.Reader
	captureBuf *bytes.Buffer
	key        string
	input      []byte
	callIndex  int
}

func (c *mockCmd) Read(p []byte) (n int, err error) {
	n, err = c.reader.Read(p)
	if err != nil {
		// Only capture output for repetition on EOF (successful completion).
		// For other errors, keep the reader in the queue for retry/repetition.
		if err == io.EOF && c.captureBuf != nil {
			c.machine.init()
			c.machine.mu.Lock()
			c.machine.captured[c.key] = c.captureBuf.Bytes()
			c.machine.mu.Unlock()
		}
		c.recordCall()
	}
	return n, err
}

func (c *mockCmd) Write(p []byte) (n int, err error) {
	c.Lock()
	c.input = append(c.input, p...)
	c.Unlock()
	return len(p), nil
}

func (c *mockCmd) Close() error {
	c.recordCall()
	return nil
}

func (c *mockCmd) recordCall() {
	c.Lock()
	if len(c.input) > 0 {
		c.call.Got = append([]byte{}, c.input...)
	}
	call := c.call
	c.Unlock()

	c.once.Do(func() {
		c.machine.mu.Lock()
		c.machine.Calls = append(c.machine.Calls, Call{})
		c.callIndex = len(c.machine.Calls) - 1
		c.machine.mu.Unlock()
	})

	c.machine.mu.Lock()
	c.machine.Calls[c.callIndex] = call
	c.machine.mu.Unlock()
}

// Calls returns invocations tracked by m, or nil if m is not a mock.Machine.
// If m is a Shell, Calls will unwrap it automatically.
//
// With no arguments, returns all invocations:
//
//	calls := mock.Calls(m)
//
// With arguments, returns only invocations matching that prefix:
//
//	gitCalls := mock.Calls(m, "git")              // All git commands
//	branchCalls := mock.Calls(m, "git", "branch") // Only git branch commands
func Calls(m command.Machine, pattern ...string) []Call {
	var calls []Call

	if mm, ok := m.(*Machine); ok {
		mm.mu.Lock()
		defer mm.mu.Unlock()
		calls = append([]Call{}, mm.Calls...)
	} else if sh, ok := m.(command.Unsheller); ok {
		if mm, ok := sh.Unshell().(*Machine); ok {
			mm.mu.Lock()
			defer mm.mu.Unlock()
			calls = append([]Call{}, mm.Calls...)
		}
	}

	if calls == nil {
		return nil
	}

	if len(pattern) == 0 {
		return calls
	}

	var filtered []Call
	for _, call := range calls {
		if argsMatch(call.Args, pattern) {
			filtered = append(filtered, call)
		}
	}
	return filtered
}

// argsMatch checks if actual args match the pattern.
// Empty pattern matches all commands.
func argsMatch(actual, pattern []string) bool {
	if len(pattern) == 0 {
		return true
	}
	if len(actual) < len(pattern) {
		return false
	}
	for i, p := range pattern {
		if actual[i] != p {
			return false
		}
	}
	return true
}

// argsEqual checks if two argument patterns are equal.
func argsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// argsKey generates a unique key for an argument pattern.
func argsKey(args []string) string {
	if len(args) == 0 {
		return "<default>"
	}
	var buf bytes.Buffer
	for i, arg := range args {
		if i > 0 {
			buf.WriteByte('\x00')
		}
		buf.WriteString(arg)
	}
	return buf.String()
}
