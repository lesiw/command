package ctr

import (
	"context"
	"io"

	"golang.org/x/term"

	"lesiw.io/command"
	"lesiw.io/fs"
	"lesiw.io/fs/path"
)

type cmd struct {
	command.Buffer
	m   *machine
	ctx context.Context
	arg []string
}

func newCmd(m *machine, ctx context.Context, args ...string) command.Buffer {
	c := &cmd{
		m:   m,
		ctx: ctx,
		arg: args,
	}
	c.setCmd(false)
	return c
}

func (c *cmd) Attach() error {
	c.setCmd(true)
	return command.Attach(c.Buffer)
}

func (c *cmd) Close() error {
	if closer, ok := c.Buffer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (c *cmd) Write(p []byte) (int, error) {
	if wb, ok := c.Buffer.(command.WriteBuffer); ok {
		return wb.Write(p)
	}
	return 0, command.ErrReadOnly
}

func (c *cmd) Log(w io.Writer) { command.Log(c.Buffer, w) }
func (c *cmd) String() string  { return command.String(c.Buffer) }

func (c *cmd) setCmd(attach bool) {
	cmdArgs := []string{"container", "exec"}
	if attach {
		if term.IsTerminal(0) {
			cmdArgs = append(cmdArgs, "-i")
			if term.IsTerminal(1) {
				cmdArgs = append(cmdArgs, "-t")
			}
		}
	} else {
		// Unattached commands should not probe stdin/stdout.
		cmdArgs = append(cmdArgs, "-i")
	}
	if dir := fs.WorkDir(c.ctx); dir != "" && path.IsAbs(dir) {
		cmdArgs = append(cmdArgs, "-w", dir)
	}
	for k, v := range command.Envs(c.ctx) {
		cmdArgs = append(cmdArgs, "-e", k+"="+v)
	}
	cmdArgs = append(cmdArgs, c.m.name)
	cmdArgs = append(cmdArgs, c.arg...)
	c.Buffer = c.m.Machine.Command(command.WithoutEnv(c.ctx), cmdArgs...)
}
