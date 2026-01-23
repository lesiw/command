// Package sys implements a command.Machine that executes commands
// on the local system using os/exec.
package sys

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"lesiw.io/command"
	"lesiw.io/command/internal/sh"
	"lesiw.io/fs"
	"lesiw.io/fs/osfs"
	"lesiw.io/fs/path"
)

var (
	Stdin  io.Reader = os.Stdin
	Stdout io.Writer = os.Stdout
	Stderr io.Writer = os.Stderr

	// useOSFS controls whether sys.Machine exposes osfs optimization.
	// This is used in testing to ensure both code paths,
	// osfs and command.FS, work correctly.
	useOSFS = true
)

// Machine returns a command.Machine that executes commands
// on the local system.
func Machine() command.Machine { return machine{} }

type machine struct{}

var _ command.Machine = (*machine)(nil)

func (machine) Command(ctx context.Context, arg ...string) command.Buffer {
	return newCmd(ctx, arg...)
}

var _ command.FSMachine = (*machine)(nil)

func (machine) FS() fs.FS {
	if useOSFS {
		// FS implements command.FSMachine to optimize filesystem operations.
		// This returns the native OS filesystem instead of running commands.
		return osfs.New()
	}
	return nil
}

type cmd struct {
	ctx context.Context
	cmd *exec.Cmd
	env map[string]string

	cmdwait chan error

	start func() error
	wait  func() error

	reader io.ReadCloser
	writer io.WriteCloser
	logger io.Writer
	logbuf bytes.Buffer

	closers []io.Closer
}

func (c *cmd) Attach() error {
	c.cmd.Stdin = Stdin
	c.cmd.Stdout = Stdout
	c.cmd.Stderr = Stderr
	return nil
}

// cmdError wraps os/exec errors into command.Error.
// If err is an ExitError, uses its exit code.
// Otherwise, wraps the error with code 0 (e.g., for command not found).
func cmdError(err error) error {
	if err == nil {
		return nil
	}

	cmdErr := &command.Error{Err: err}

	// If it's an ExitError, extract the exit code
	if ee := new(exec.ExitError); errors.As(err, &ee) {
		cmdErr.Code = ee.ExitCode()
	}

	return cmdErr
}

func newCmd(ctx context.Context, args ...string) command.Buffer {
	if len(args) == 0 {
		return command.Fail(fmt.Errorf("no command given"))
	}

	c := new(cmd)
	c.ctx = ctx
	c.cmd = exec.CommandContext(ctx, args[0], args[1:]...)
	c.env = command.Envs(ctx)

	dir := fs.WorkDir(ctx)
	if dir != "" && !path.IsAbs(dir) {
		var err error
		dir, err = filepath.Localize(dir)
		if err != nil {
			return command.Fail(
				fmt.Errorf("workdir localization failed: %w", err),
			)
		}
	}
	c.cmd.Dir = dir
	c.cmd.Env = os.Environ()
	for k, v := range c.env {
		c.cmd.Env = append(c.cmd.Env, k+"="+v)
	}

	c.start = sync.OnceValue(c.startFunc)
	c.wait = sync.OnceValue(c.waitFunc)
	c.cmdwait = make(chan error, 1)

	return c
}

func (c *cmd) startFunc() error {
	if c.cmd.Stdin == nil {
		w, err := c.cmd.StdinPipe()
		if err != nil {
			return fmt.Errorf("failed to pipe stdin: %w", err)
		}
		c.writer = w
	}
	if c.cmd.Stdout == nil {
		r, w := io.Pipe()
		c.reader = r
		c.cmd.Stdout = w
		c.closers = append(c.closers, w)
	}
	if c.cmd.Stderr == nil {
		if c.logger == nil {
			c.cmd.Stderr = &c.logbuf
		} else {
			c.cmd.Stderr = c.logger
		}
	}
	if err := c.cmd.Start(); err != nil {
		for _, cl := range c.closers {
			_ = cl.Close() // Best effort.
		}
		return cmdError(err)
	}
	go func() {
		err := c.cmd.Wait()

		// Workaround for pipe cleanup race conditions on some systems.
		// On certain systems (e.g., Windows with PowerShell), Wait() can
		// return before the OS finishes closing pipe handles internally.
		// This causes hangs when closing our end of the pipe after ~30-45
		// sequential command spawns. A 100μs delay allows the OS to complete
		// cleanup. The overhead is negligible (~0.0001s per command).
		time.Sleep(100 * time.Microsecond)

		for _, cl := range c.closers {
			err = errors.Join(err, cl.Close())
		}
		c.cmdwait <- err
	}()
	return nil
}

func (c *cmd) Write(bytes []byte) (int, error) {
	if err := c.start(); err != nil {
		return 0, err
	}
	if c.writer == nil {
		return 0, nil
	}
	n, err := c.writer.Write(bytes)
	if err != nil {
		return n, fmt.Errorf("failed write: %w", err)
	}
	return n, nil
}

func (c *cmd) Close() error {
	if err := c.start(); err != nil {
		return err
	}
	if c.writer == nil {
		return nil
	}
	if err := c.writer.Close(); err != nil {
		if !errors.Is(err, os.ErrClosed) {
			return fmt.Errorf("failed close: %w", err)
		}
	}
	return nil
}

func (c *cmd) Read(bytes []byte) (int, error) {
	if err := c.start(); err != nil {
		return 0, err
	}
	ch := make(chan struct {
		n   int
		err error
	})
	var n int
	var err error
	if c.reader == nil {
		err = io.EOF
		goto skipread
	}

	go func() {
		n, err := c.reader.Read(bytes)
		ch <- struct {
			n   int
			err error
		}{n, err}
	}()
	select {
	case <-c.ctx.Done():
		n = 0
		err = io.EOF
	case ret := <-ch:
		n = ret.n
		err = ret.err
	}

skipread:
	if err != nil {
		if err1 := c.wait(); err1 != nil {
			err = err1
		}
	}
	return n, err
}

func (c *cmd) Log(w io.Writer) {
	c.logger = w
}

func (c *cmd) waitFunc() error {
	err := <-c.cmdwait
	if err != nil {
		cmdErr := cmdError(err)
		// Add log buffer if available
		if ce, ok := cmdErr.(*command.Error); ok && c.logger == nil {
			ce.Log = c.logbuf.Bytes()
		}
		return cmdErr
	}
	return nil
}

func (c *cmd) String() string {
	return sh.String(c.env, c.cmd.Args...).String()
}
