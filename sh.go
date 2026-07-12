package command

//go:generate go run ./internal/generate/sh -p lesiw.io/fs -t FS -f
//go:generate go run ./internal/generate/sh -p lesiw.io/command -t Machine

import (
	"context"
	"fmt"
	"sync"

	"lesiw.io/fs"
	"lesiw.io/zeros"
)

// Sh is a Machine that routes commands to different machines based on
// command name, similar to how a system shell routes commands via $PATH.
//
// Unlike a raw Machine, a Sh requires explicit registration of all
// commands. Unregistered commands return "command not found" errors.
// This creates a controlled environment with explicit command provenance.
//
// A Sh wraps a "core" machine (accessible via the Unwrapper interface)
// which is used for operations like OS detection and filesystem probing, but
// not for direct command execution unless explicitly routed.
//
// Sh provides convenient methods for common operations that delegate to
// package-level functions. This provides an ergonomic API while maintaining
// flexibility for code that needs to work with the Machine interface.
//
// Example:
//
//	sh := command.Shell(sys.Machine())
//	sh = sh.Handle("jq", jqMachine)
//	sh = sh.Handle("go", goMachine)
//
//	// Ergonomic method calls
//	out, err := sh.Read(ctx, "jq", ".foo")  // ✓
//	data, err := sh.ReadFile(ctx, "config.yaml")
//
//	// Unregistered commands fail
//	sh.Read(ctx, "cat", "file") // ✗ command not found
type Sh struct {
	routes   zeros.Map[string, Machine]
	m        Machine // The underlying machine (core or self)
	fallback bool    // Enable fallback to inner machine
	once     sync.Once

	// Cached values (populated on first use)
	os   string
	arch string
	fsys zeros.OnceValue[fs.FS]
}

// Shell creates a new shell that wraps the given core machine.
// Commands must be explicitly registered via Handle to be accessible.
// The core machine is accessible via the Unwrapper interface for operations
// like OS detection and filesystem probing.
//
// Optional commands can be provided as varargs. Each specified command
// will be routed to the underlying machine via sh.Handle(cmd, sh.Unshell()).
// This is equivalent to manually calling:
//
//	sh := command.Shell(core)
//	for _, cmd := range commands {
//	    sh = sh.Handle(cmd, sh.Unshell())
//	}
//
// Example:
//
//	// Whitelist specific commands
//	sh := command.Shell(sys.Machine(), "go", "git", "make")
//	sh.Read(ctx, "go", "version")  // ✓ Works
//	sh.Read(ctx, "cat", "file")    // ✗ command not found
//
// This follows the exec.Command naming pattern where the constructor has
// the longer name and returns a pointer to the shorter type name.
func Shell(core Machine, commands ...string) *Sh {
	sh := &Sh{
		m: core,
	}
	for _, cmd := range commands {
		sh = sh.Handle(cmd, sh.Unshell())
	}
	return sh
}

// init detects and caches OS and Arch on first use.
func (sh *Sh) init(ctx context.Context) {
	sh.once.Do(func() {
		sh.os = OS(ctx, sh.m)
		sh.arch = Arch(ctx, sh.m)
	})
}

// OS returns the operating system type for this shell.
// The value is detected only once on first use and cached for performance.
// Returns normalized GOOS values: "linux", "darwin", "freebsd", "openbsd",
// "netbsd", "dragonfly", "windows", or "unknown".
func (sh *Sh) OS(ctx context.Context) string {
	sh.init(ctx)
	return sh.os
}

// Arch returns the architecture for this shell.
// The value is detected only once on first use and cached for performance.
// Returns normalized GOARCH values: "amd64", "arm64", "386", "arm", or
// "unknown".
func (sh *Sh) Arch(ctx context.Context) string {
	sh.init(ctx)
	return sh.arch
}

// FS returns the filesystem for this shell.
// The filesystem is created only once on first use and cached for performance.
func (sh *Sh) FS() fs.FS {
	return sh.fsys.Do(func() fs.FS { return FS(sh.m) })
}

// Env returns the value of the environment variable named by key.
// It probes the inner machine (sh.m) to retrieve the value, piercing through
// Shell layers just like OS() and Arch() do.
//
// This is a convenience method that calls [Env].
func (sh *Sh) Env(ctx context.Context, key string) string {
	return Env(ctx, sh.m, key)
}

// Shutdown shuts down the underlying machine if it is a [ShutdownMachine].
func (sh *Sh) Shutdown(ctx context.Context) error {
	return Shutdown(ctx, sh.m)
}

// Handle registers a machine to handle the specified command.
// Returns the shell for method chaining.
func (sh *Sh) Handle(command string, machine Machine) *Sh {
	sh.routes.Set(command, machine)
	return sh
}

// HandleFunc registers a function to handle the specified command.
// This is a convenience wrapper around Handle for function handlers.
// Returns the shell for method chaining.
func (sh *Sh) HandleFunc(
	command string,
	fn func(context.Context, ...string) Buffer,
) *Sh {
	return sh.Handle(command, MachineFunc(fn))
}

// Command implements the Machine interface. It routes the command to the
// registered machine based on the command name (args[0]).
// If no machine is registered, behavior depends on the fallback flag:
// with fallback, falls back to inner machine; without fallback, returns error.
func (sh *Sh) Command(ctx context.Context, args ...string) Buffer {
	sh.init(ctx)
	return sh.command(ctx, sh.fallback, args...)
}

// Unshell implements the Unsheller interface. It returns the machine
// one layer down (the inner/core machine that was wrapped by Shell).
//
// This allows selective command whitelisting by explicitly routing
// specific commands to the underlying machine:
//
//	sh := command.Shell(sys.Machine())
//	sh = sh.Handle("go", sh.Unshell())  // Whitelist "go" command
//
// IMPORTANT: This breaks Shell's portability guarantees. The returned
// machine will execute commands on the underlying machine even if they
// aren't explicitly registered in the Shell.
func (sh *Sh) Unshell() Machine {
	return sh.m
}

// command is the shared routing logic used by both Command() and fallback.
// If fallback is true, unregistered commands fall back to the inner machine.
// If fallback is false, unregistered commands return NotFoundError.
func (sh *Sh) command(
	ctx context.Context, fallback bool, args ...string,
) Buffer {
	if len(args) == 0 {
		return Fail(fmt.Errorf("no command specified"))
	}

	cmdName := args[0]

	// Try registered route
	if machine, ok := sh.routes.CheckGet(cmdName); ok {
		return machine.Command(ctx, args...)
	}

	// No route found
	if fallback {
		return sh.m.Command(ctx, args...)
	}

	return Fail(&Error{
		Err: fmt.Errorf("command not found: %s", cmdName),
	})
}

// MachineFunc is an adapter to allow ordinary functions to be used as
// Machines. This is similar to http.HandlerFunc.
type MachineFunc func(context.Context, ...string) Buffer

// Command implements the Machine interface.
func (f MachineFunc) Command(ctx context.Context, args ...string) Buffer {
	return f(ctx, args...)
}

// Handle registers a handler for a specific command name on the given machine.
// If m is already a Sh, the handler is added to it.
// Otherwise, a new Sh is created with m as the core machine, with fallback
// enabled to preserve visibility of the underlying machine's commands.
//
// This allows users to work with Machine as their primary interface while
// building up a shell:
//
//	m := sys.Machine()
//	m = command.Handle(m, "jq", jqMachine)
//	m = command.Handle(m, "go", goMachine)
//	// m is now routing with fallback to sys.Machine() commands
//
// The handled command takes precedence, but unhandled commands still work:
//
//	m := sys.Machine()
//	m = command.Handle(m, "echo", customEchoMachine)
//	m.Command(ctx, "echo", "hello")  // Uses customEchoMachine
//	m.Command(ctx, "cat", "file")    // Falls back to sys.Machine()
//
// For more ergonomic usage, consider using Sh methods directly:
//
//	sh := command.Shell(sys.Machine())
//	sh.Handle("jq", jqMachine).Handle("go", goMachine)
func Handle(m Machine, command string, handler Machine) Machine {
	if sh, ok := m.(*Sh); ok {
		sh.Handle(command, handler)
		return sh
	}

	sh := Shell(m)
	sh.fallback = true // Enable fallback for non-Sh machines
	sh.Handle(command, handler)
	return sh
}

// HandleFunc is a convenience wrapper around Handle for function handlers.
// This matches the pattern of http.HandleFunc.
//
//	m := sys.Machine()
//	m = command.HandleFunc(m, "echo",
//	    func(ctx context.Context, args ...string) Buffer {
//	        // Custom echo implementation
//	        return customMachine.Command(ctx, args...)
//	    })
//
// For more ergonomic usage, consider using Sh methods directly:
//
//	sh := command.Shell(sys.Machine())
//	sh.HandleFunc("echo",
//	    func(ctx context.Context, args ...string) Buffer {
//	        return customMachine.Command(ctx, args...)
//	    })
func HandleFunc(
	m Machine,
	command string,
	fn func(context.Context, ...string) Buffer,
) Machine {
	return Handle(m, command, MachineFunc(fn))
}
