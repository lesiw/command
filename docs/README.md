# lesiw.io/command

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/command.svg)](https://pkg.go.dev/lesiw.io/command)
[![CI](https://github.com/lesiw/command/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/lesiw/command/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/lesiw/command)](../LICENSE)

Command buffers for Go: a command is an `io.Reader`, piping is
`io.Copy`, and anything that can run a command — the local system, a
container, a host across the network — is the same one-method
`Machine` interface.

```go
// Stream a database dump from a remote host into a local file.
db := ssh.Machine(sys.Machine(), "ssh", "postgres@db1.example.com")
_, err := io.Copy(
    fs.CreateBuffer(ctx, command.FS(sys.Machine()), "backup.sql"),
    command.NewReader(ctx, db, "pg_dumpall"),
)
```

[cmdbuf.io](https://cmdbuf.github.io) ·
[API reference](https://pkg.go.dev/lesiw.io/command) ·
[Field guide](https://cmdbuf.github.io/README.md)

## The model

1. A **Buffer** is a command's execution as an `io.Reader`. The
   command starts on the first `Read` and is finished at `io.EOF`.
   There is no `Start` and no `Wait`.
2. A **Machine** is anything that can run a command. It has one
   method: `Command(ctx context.Context, arg ...string) Buffer`.
3. **Machines wrap machines**, so a container on a remote host is
   two calls.
4. **Files are buffers too**, on any machine.
5. A **Shell** makes automation portable.

## Install

```sh
go get lesiw.io/command
```

Requires Go 1.24.7 or later.

## Quick start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "lesiw.io/command"
    "lesiw.io/command/sys"
)

func main() {
    ctx, m := context.Background(), sys.Machine()

    version, err := command.Read(ctx, m, "go", "version")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(version)
}
```

`command.Read` captures output like `$(...)`. Its siblings:
`command.Do` runs a command and discards output; `command.Exec`
attaches the command to the terminal.

> [!TIP]
> The in-memory machine runs in the Go Playground, so the basics can
> be tried without installing anything:
> [reading a command](https://go.dev/play/p/Bih75QdcZdw),
> [piping](https://go.dev/play/p/eU9TkBMOCls),
> [files](https://go.dev/play/p/0UjNOYQ_kN-).

## Machines

| Constructor | Runs commands |
|---|---|
| `sys.Machine()` | on the local system |
| `ssh.Machine(m, "ssh", "user@host")` | on a remote host |
| `ctr.Machine(m, "alpine:latest")` | in a container (Docker, Podman, or nerdctl) |
| `sub.Machine(m, "busybox")` | on `m`, with every command prefixed |
| `mem.Machine()` | in memory: echo, cat, tee, tr |
| `new(mock.Machine)` | nowhere: programmed responses for tests |

Machines take machines, so environments nest:

```go
// A container on a remote build host.
host := ssh.Machine(sys.Machine(), "ssh", "admin@build.example.com")
m := ctr.Machine(host, "golang:latest")
defer command.Shutdown(ctx, m)

sh := command.Shell(m, "go")
if err := sh.Exec(ctx, "go", "test", "./..."); err != nil {
    log.Fatal(err)
}
```

> [!IMPORTANT]
> `ssh.Machine`'s arguments are the complete SSH command line,
> including the SSH client itself. This is what lets any
> SSH-compatible client work — autossh, `sshpass -p pw ssh`, a jump
> host — with no dedicated API.

A machine can also be a single command.
`command.HandleFunc` routes one command name through a function —
this shim injects an environment variable into every `go` call:

```go
m := command.HandleFunc(sys.Machine(), "go",
    func(ctx context.Context, args ...string) command.Buffer {
        ctx = command.WithEnv(ctx, map[string]string{
            "GOFLAGS": "-trimpath",
        })
        return sys.Machine().Command(ctx, args...)
    })
```

## Piping

Two stages are `io.Copy`:

```go
// echo "hello, pipes" | tr a-z A-Z
_, err := io.Copy(
    command.NewWriter(ctx, m, "tr", "a-z", "A-Z"),
    command.NewReader(ctx, m, "echo", "hello, pipes"),
)
```

Three or more stages are `command.Copy`, with `command.NewFilter` for
the middle stages. When any stage fails, the returned error reports
every stage and its outcome, so the failing command is identifiable. Commands and files on different machines mix in
one pipeline:

```go
backup := ssh.Machine(m, "ssh", "backup@vault.example.com")
_, err := command.Copy(
    fs.CreateBuffer(ctx, command.FS(backup), "db.sql.gz"),
    command.NewReader(ctx, m, "pg_dumpall"),
    command.NewFilter(ctx, m, "gzip"),
)
```

## Files

`command.FS(m)` returns a [lesiw.io/fs](https://pkg.go.dev/lesiw.io/fs)
filesystem for any machine. On machines without native filesystem
access, operations are implemented with whatever commands the target
system has; calling code is identical either way, and automation
written this way is cross-platform by default — CI for these
libraries runs on Linux, macOS, Windows, FreeBSD, and Alpine.

```go
fsys := command.FS(m)
err := fs.WriteFile(ctx, fsys, "hello.txt", []byte("Hello!\n"))
data, err := fs.ReadFile(ctx, fsys, "hello.txt")
```

Copying between machines is `io.Copy` of lazy file buffers — the
same shape as scp, docker cp, and cp:

```go
local := command.Shell(sys.Machine())
remote := command.Shell(ssh.Machine(sys.Machine(), "ssh", "deploy@prod.example.com"))

_, err := io.Copy(
    remote.CreateBuffer(ctx, "/opt/app/server"),
    local.OpenBuffer(ctx, "bin/server"),
)
```

A trailing slash denotes a directory, which reads and writes as a tar
archive: `sh.Open(ctx, "dir/")` produces one, and
`sh.Create(ctx, "dir/")` accepts one.

## Shells

`command.Shell` wraps a machine with portable operations and an
explicit list of allowed external commands. Undeclared commands fail
with *command not found*, so a program's true dependencies sit in one
line at the top of the file:

```go
sh := command.Shell(sys.Machine(), "go")

if err := sh.Exec(ctx, "go", "vet", "./..."); err != nil {
    log.Fatal(err)
}

ver, err := sh.ReadFile(ctx, "VERSION")
if err != nil {
    ver = []byte("dev")
}

if err := sh.MkdirAll(ctx, "bin"); err != nil {
    log.Fatal(err)
}
err = sh.Exec(
    command.WithEnv(ctx, map[string]string{"CGO_ENABLED": "0"}),
    "go", "build", "-ldflags", "-X main.version="+string(ver),
    "-o", "bin/app", ".",
)
```

File and system operations (`ReadFile`, `MkdirAll`, `Temp`, `OS`,
`Arch`, and the rest) have no command to declare — they work on any
machine, including Windows. The
[field guide](https://cmdbuf.github.io/README.md) has a full table of shell
idioms and their equivalents.

## Testing

`mock.Machine` records calls and returns programmed responses;
unprogrammed commands succeed with empty output.

```go
func Deploy(ctx context.Context, sh *command.Sh) error {
    branch, err := sh.Read(ctx, "git", "branch", "--show-current")
    if err != nil {
        return fmt.Errorf("read branch: %w", err)
    }
    return sh.Exec(ctx, "git", "push", "origin", branch)
}

func TestDeploy(t *testing.T) {
    m := new(mock.Machine)
    m.Return(strings.NewReader("main\n"), "git", "branch", "--show-current")

    sh := command.Shell(m, "git")
    if err := Deploy(t.Context(), sh); err != nil {
        t.Fatal(err)
    }

    got := mock.Calls(sh, "git")
    want := []mock.Call{
        {Args: []string{"git", "branch", "--show-current"}},
        {Args: []string{"git", "push", "origin", "main"}},
    }
    if !cmp.Equal(want, got) {
        t.Errorf("git calls mismatch (-want +got):\n%s", cmp.Diff(want, got))
    }
}
```

The test runs with no git repository, no network, and no side
effects, and asserts the exact commands the automation would have
run.

## Why a library?

Configuration languages grow conditionals, loops, and modules until
they have become programming languages without a debugger, a
formatter, or a test framework. Shell is a real language, but its
sharp edges — quoting, word splitting, BSD/GNU drift, no modules, no
types — compound as automation grows. Go's compatibility promise,
module system, and testing culture hold up; what was missing was
shell's ergonomics for running and piping commands. Command buffers
supply that piece.

Command buffers are not a replacement for a five-line shell script.
They are for the automation that outgrew one — the build that needs a
real conditional, the deploy that spans three machines, the script
someone finally asked for a test on.

## Documentation

- [cmdbuf.io](https://cmdbuf.github.io) — the concepts, with worked examples.
- [Field guide](https://cmdbuf.github.io/README.md) — condensed reference:
  shell-to-Go translation table, common mistakes, API quick
  reference.
- [pkg.go.dev/lesiw.io/command](https://pkg.go.dev/lesiw.io/command)
  — full API documentation, including a cookbook of shell idioms.
- [pkg.go.dev/lesiw.io/fs](https://pkg.go.dev/lesiw.io/fs) — the
  filesystem abstraction.

## License

[BSD 3-Clause](../LICENSE)
