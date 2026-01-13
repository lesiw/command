// Package ctr implements a command.Machine that executes commands
// in containers using docker, podman, nerdctl, or lima nerdctl.
package ctr

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lesiw.io/command"
	"lesiw.io/command/sub"
	"lesiw.io/zeros"
)

var (
	clis = [...][]string{
		{"docker"},
		{"podman"},
		{"nerdctl"},
		{"lima", "nerdctl"},
	}

	errShutdown = errors.New("machine shut down")
)

type ctlMachine struct {
	host command.Machine
	once zeros.OnceValues[command.Machine, error]
}

// Ctl returns a Machine for the controller CLI (docker, podman, etc.)
// found on the given Machine.
//
//	ctl := ctr.Ctl(sys.Machine())
//	ctl.Command(ctx, "run", "-ti", "alpine", "sh")
func Ctl(m command.Machine) command.Machine { return &ctlMachine{host: m} }

func (m *ctlMachine) init(ctx context.Context) (command.Machine, error) {
	return m.once.Do(func() (command.Machine, error) { return m.doInit(ctx) })
}

func (m *ctlMachine) doInit(ctx context.Context) (command.Machine, error) {
	var ctrcli []string
	for _, cli := range clis {
		args := append([]string{}, cli...)
		args = append(args, "--version")
		if !command.NotFound(command.Do(ctx, m.host, args...)) {
			ctrcli = cli
			break
		}
	}
	if len(ctrcli) == 0 {
		return nil, fmt.Errorf("no container CLI found: %v", clis)
	}
	return sub.Machine(m.host, ctrcli...), nil
}

func (m *ctlMachine) Command(
	ctx context.Context, arg ...string,
) command.Buffer {
	ctl, err := m.init(ctx)
	if err != nil {
		return command.Fail(err)
	}
	return ctl.Command(ctx, arg...)
}

// Machine instantiates a command.Machine that runs commands in a container.
//
// If name begins with / or ., it is treated as a path to a Containerfile
// and will be built. Otherwise it is treated as an image name.
//
// Additional args are passed to the container run command.
//
// The container does not start until the first command is executed,
// so it is safe to declare a package variable as a ctr.Machine()
// without incurring side effects at package initialization time.
//
//	m := ctr.Machine(sys.Machine(), "alpine")
//	m := ctr.Machine(sys.Machine(), "./Containerfile", "-v", "/data:/data")
func Machine(
	m command.Machine, name string, args ...string,
) command.Machine {
	return &machine{host: m, name: name, args: args}
}

type machine struct {
	sync.RWMutex
	command.Machine
	host command.Machine
	name string
	args []string
	once zeros.OnceValue[error]
	hash string
	done bool
}

func (m *machine) init(ctx context.Context) error {
	m.Lock()
	defer m.Unlock()

	if m.done {
		return errShutdown
	}

	return m.once.Do(func() error { return m.doInit(ctx) })
}

func (m *machine) doInit(ctx context.Context) error {
	m.Machine = Ctl(m.host)

	// Build container if path provided.
	name := m.name
	if len(name) > 0 && (name[0] == '/' || name[0] == '.') {
		var err error
		if name, err = buildContainer(ctx, m.Machine, name); err != nil {
			return fmt.Errorf("failed to build container: %w", err)
		}
	}

	// Start container.
	cmd := []string{"container", "run", "--rm", "-d", "-i"}
	cmd = append(cmd, m.args...)
	cmd = append(cmd, name, "cat")
	out, err := command.Read(ctx, m.Machine, cmd...)
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	m.hash = strings.TrimSpace(string(out))
	return nil
}

func (m *machine) Command(ctx context.Context, arg ...string) command.Buffer {
	if err := m.init(ctx); err != nil {
		return command.Fail(err)
	}

	m.RLock()
	defer m.RUnlock()

	if m.done {
		return command.Fail(errShutdown)
	}

	return newCmd(m, ctx, arg...)
}

var _ command.ShutdownMachine = (*machine)(nil)

func (m *machine) Shutdown(ctx context.Context) error {
	m.Lock()
	defer m.Unlock()

	m.done = true

	if m.Machine == nil {
		return nil
	}

	// Derive context with timeout to prevent blocking indefinitely.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return command.Do(ctx, m.Machine, "container", "rm", "-f", m.hash)
}

func buildContainer(
	ctx context.Context, m command.Machine, rpath string,
) (image string, err error) {
	var path string
	if path, err = filepath.Abs(rpath); err != nil {
		err = fmt.Errorf("bad Containerfile path %q: %w", rpath, err)
		return
	}
	imagehash := sha1.New()
	imagehash.Write([]byte(path))
	image = fmt.Sprintf("%x", imagehash.Sum(nil))
	out, insperr := command.Read(ctx, m,
		"image", "inspect",
		"--format", "{{.Created}}",
		image,
	)
	mtime, err := getMtime(path)
	if err != nil {
		err = fmt.Errorf("bad Containerfile %q: %w", path, err)
		return
	}
	if insperr == nil {
		var ctime time.Time
		outStr := strings.TrimSpace(string(out))
		ctime, err = time.Parse(time.RFC3339, outStr)
		if err != nil {
			err = fmt.Errorf(
				"failed to parse container timestamp %q: %s",
				outStr, err)
			return
		}
		if ctime.Unix() > mtime {
			return // Container is newer than Containerfile.
		}
	}
	err = command.Exec(ctx, m,
		"image", "build",
		"--file", path,
		"--no-cache",
		"--tag", image,
		filepath.Dir(path),
	)
	if err != nil {
		err = fmt.Errorf("failed to build %q: %w", path, err)
	}
	return
}

func getMtime(path string) (mtime int64, err error) {
	var info fs.FileInfo
	info, err = os.Lstat(path)
	if err != nil {
		return
	}
	mtime = info.ModTime().Unix()
	return
}
