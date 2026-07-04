// Package ssh implements a command.Machine that executes commands over SSH.
//
// Unlike raw SSH execution, ssh.Machine handles environment variable passing
// by prefixing commands with the appropriate syntax for the remote operating
// system (VAR=value for Unix, set VAR=value& for Windows).
package ssh

import (
	"context"
	"strings"
	"sync"

	"lesiw.io/command"
	"lesiw.io/command/internal/sh"
	"lesiw.io/command/sub"
	"lesiw.io/fs"
)

// Machine creates a command.Machine that executes commands over SSH.
// The machine wraps the given machine m (typically sys.Machine()) and
// prefixes all commands with args: the complete SSH command line,
// including the SSH client itself. Any SSH-compatible client works,
// such as autossh or ssh wrapped in sshpass.
//
// Environment variables from the context are automatically converted to
// inline command prefixes based on the detected remote operating system.
//
// Example:
//
//	m := ssh.Machine(sys.Machine(), "ssh", "user@host")
//	ctx := command.WithEnv(ctx, map[string]string{"FOO": "bar"})
//	command.Read(ctx, m, "printenv", "FOO")
//	// Effectively: ssh user@host FOO=bar printenv FOO
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

	// SSH concatenates remote arguments into one string and hands it to
	// the login shell, so metacharacters in args would be interpreted by
	// the remote shell. Build a single quoted command of the form
	//     sh -c 'exec "$@"' sh 'arg1' 'arg2' ...
	// where exec "$@" preserves argument boundaries and each arg is
	// individually quoted so the remote shell passes it through literally.
	inner := `exec "$@"`
	dir := fs.WorkDir(ctx)
	if dir != "" {
		inner = "cd " + sh.Quote(dir) + " && " + inner
		ctx = fs.WithWorkDir(ctx, "")
	}
	var remote strings.Builder
	remote.WriteString("sh -c " + sh.Quote(inner) + " sh")
	for _, arg := range args {
		remote.WriteByte(' ')
		remote.WriteString(sh.Quote(arg))
	}
	args = []string{remote.String()}

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
