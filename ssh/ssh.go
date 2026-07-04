// Package ssh implements a command.Machine that executes commands over SSH.
//
// Unlike raw SSH execution, ssh.Machine preserves argument boundaries
// and passes environment variables from the context, using the
// mechanism appropriate to the remote operating system: a quoting
// shell wrapper with VAR=value prefixes on Unix, and a base64-encoded
// PowerShell script on Windows.
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
	if sm.os == "windows" {
		return sm.windowsCommand(ctx, args...)
	}

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
		ctx = fs.WithoutWorkDir(ctx)
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
		args = prefixEnvVars(env, args)
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

func prefixEnvVars(env map[string]string, args []string) []string {
	var prefixed []string
	for k, v := range env {
		prefixed = append(prefixed, k+"="+sh.Quote(v))
	}
	return append(prefixed, args...)
}
