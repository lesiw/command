// Package ssh implements a command.Machine that executes commands over SSH.
//
// Unlike raw SSH execution, ssh.Machine handles environment variable passing
// by prefixing commands with the appropriate syntax for the remote operating
// system (VAR=value for Unix, set VAR=value& for Windows).
package ssh

import (
	"context"
	"sync"

	"lesiw.io/command"
	"lesiw.io/command/sub"
	"lesiw.io/fs"
)

// Machine creates a command.Machine that executes commands over SSH.
// The machine wraps the given machine m (typically sys.Machine()) and
// prefixes all commands with the SSH connection arguments.
//
// Environment variables from the context are automatically converted to
// inline command prefixes based on the detected remote operating system.
//
// Example:
//
//	m := ssh.Machine(sys.Machine(), "user@host")
//	ctx := command.WithEnv(ctx, map[string]string{"FOO": "bar"})
//	command.Read(ctx, m, "printenv", "FOO")
//	// Executes: ssh user@host FOO=bar printenv FOO
//
// Additional SSH options can be provided:
//
//	m := ssh.Machine(sys.Machine(), "-p", "2222", "user@host")
func Machine(m command.Machine, args ...string) command.Machine {
	return &machine{
		m:    m,
		args: args,
	}
}

var testHookOS func() string

type machine struct {
	m    command.Machine
	args []string
	once sync.Once
	os   string
	arch string
}

func (sm *machine) Command(
	ctx context.Context, args ...string,
) command.Buffer {
	sm.init(ctx)

	if dir := fs.WorkDir(ctx); dir != "" {
		// Use POSIX sh to change directory before executing.
		// The exec "$@" preserves argument boundaries correctly.
		cmdStr := "cd " + dir + " && exec \"$@\""
		args = append([]string{"sh", "-c", cmdStr, "sh"}, args...)
		ctx = fs.WithWorkDir(ctx, "")
	}

	env := command.Envs(ctx)
	if len(env) > 0 {
		args = prefixEnvVars(sm.os, env, args)
		ctx = command.WithoutEnv(ctx)
	}

	fullArgs := append(append([]string(nil), sm.args...), args...)
	return sm.m.Command(ctx, fullArgs...)
}

func (sm *machine) init(ctx context.Context) {
	sm.once.Do(func() {
		if h := testHookOS; h != nil {
			sm.os = h()
			return
		}
		probe := sub.Machine(sm.m, sm.args...)
		sm.os = command.OS(ctx, probe)
		sm.arch = command.Arch(ctx, probe)
	})
}

func (sm *machine) OS(ctx context.Context) string {
	sm.init(ctx)
	return sm.os
}

func (sm *machine) Arch(ctx context.Context) string {
	sm.init(ctx)
	return sm.arch
}

// prefixEnvVars prepends environment variable syntax to the command based
// on the detected OS.
func prefixEnvVars(os string, env map[string]string, args []string) []string {
	if os == "windows" {
		// Windows: Prepend "set VAR=value&" for each variable
		var prefix string
		for k, v := range env {
			prefix += "set " + k + "=" + v + "&"
		}
		// Concatenate first command arg with the prefix
		newArgs := make([]string, len(args))
		copy(newArgs, args)
		newArgs[0] = prefix + args[0]
		return newArgs
	}

	// Unix-like: VAR=value VAR2=value2 command args
	var prefixed []string
	for k, v := range env {
		prefixed = append(prefixed, k+"="+v)
	}
	return append(prefixed, args...)
}
