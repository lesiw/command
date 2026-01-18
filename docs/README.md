# lesiw.io/command

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/command.svg)](https://pkg.go.dev/lesiw.io/command)

Package command provides command buffers for Go.

## `command.Buffer`

A `command.Buffer` is an `io.Reader` that executes a command.

```go
import (
    "context"
    "io"
    "log"

    "lesiw.io/command"
    "lesiw.io/command/sys"
)

ctx, m := context.Background(), sys.Machine()

// Reading a command executes it
data, err := io.ReadAll(command.NewReader(ctx, m, "generate-data"))
if err != nil {
    log.Fatal(err)
}
```

Commands start on first Read and complete at EOF.
No explicit `Start()` or `Wait()`.
Because commands are `io.Reader`,
they compose naturally with other `io` primitives.

```go
_, err := io.Copy(
    command.NewWriter(ctx, m, "process"),
    command.NewReader(ctx, m, "generate-data"),
)
if err != nil {
    log.Fatal(err)
}
```

Or pipe multiple stages together:

```go
err := command.Copy(
    command.NewWriter(ctx, m, "transform"),
    command.NewReader(ctx, m, "generate"),
    command.NewFilter(ctx, m, "filter"),
)
if err != nil {
    log.Fatal(err)
}
```

## `command.Machine`

A `command.Machine` is anything that can execute commands:

```go
type Machine interface {
    Command(ctx context.Context, arg ...string) Buffer
}
```

Same code, different execution context.
This package provides:

- **`sys.Machine()`** - local system
- **`ctr.Machine()`** - container (Docker/Podman/nerdctl)
- **`ssh.Machine()`** - remote host over SSH
- **`mem.Machine()`** - in-memory for examples
- **`mock.Machine`** - programmable mock for tests

Here's what that looks like:

```go
import (
    "lesiw.io/command"
    "lesiw.io/command/ctr"
    "lesiw.io/command/sys"
)

ctx := context.Background()

// Local or container - same code
var m command.Machine = sys.Machine()
if *useContainer {
    m = ctr.Machine(sys.Machine(), "golang:latest")
    defer command.Shutdown(ctx, m)
}

sh := command.Shell(m, "go")
if err := sh.Exec(ctx, "go", "build", "."); err != nil {
    log.Fatal(err)
}
```

Deploy to a remote host:

```go
import "lesiw.io/command/ssh"

ctx, sh := context.Background(), command.Shell(
    ssh.Machine(sys.Machine(), "deploy@prod.example.com"),
    "systemctl",
)

// Copy binary to remote
localSh := command.Shell(sys.Machine())
_, err := io.Copy(
    sh.CreateBuffer(ctx, "/opt/app/server"),
    localSh.OpenBuffer(ctx, "./bin/server"),
)
if err != nil {
    log.Fatal(err)
}

// Restart service
if err := sh.Exec(ctx, "systemctl", "restart", "app"); err != nil {
    log.Fatal(err)
}
```

Command machines compose.
Copy from one to another as easily as copying files:

```go
ctx, localSh := context.Background(), command.Shell(sys.Machine())
remoteSh := command.Shell(ssh.Machine(sys.Machine(), "host.example.com"))

// Stream from local to remote
_, err := io.Copy(
    remoteSh.CreateBuffer(ctx, "data.tar.gz"),
    localSh.OpenBuffer(ctx, "data.tar.gz"),
)
if err != nil {
    log.Fatal(err)
}
```

## Why?

CI configs, Makefiles, YAML, shell.
We write automation every day,
but we don't treat it like programming.
And yet, as builds grow complex,
we eventually need what code gives us:
linters, formatters, modules, tests.

Learning yet another configuration language is frustrating
when you could solve the same problems with if statements.
Shell gets you further, it's mostly portable, it's code,
but it's also famously full of sharp edges.
No modules makes code sharing hard.
No types leaves you open to a whole class of timewasting bugs.
Subtle differences between BSD and GNU tools create incompatibilities.
Ever written a script on Linux that breaks on Mac?
Sometimes, not even POSIX can save you.

Some tools let you run automation locally or remotely,
but the code looks completely different depending on where it runs.
Others are polyglot (you pick your language), but
then code you write in one language can't easily move to another project
using a different one.

The solution?
**Use a real programming language.**
Write automation once that works everywhere.
Treat your builds as programs, because they are.

Go is the standout choice for an automation language.
The `go1compat` promise means your automation keeps working,
just like the trusty shell scripts you return to years later.
Teams already pick up quirky automation tools out of necessity.
Go has [25 keywords](https://go.dev/tour/),
takes an afternoon to learn,
and was designed to be a productive language from the ground up.

**Go's ecosystem:**
- Modules and minimum version selection (no dependency hell)
- `go1compat` - automation code that keeps working
- Formatter (`gofmt`), linter, test framework included
- Type checking prevents whole classes of bugs

**Concurrency without colored functions:**

Go's goroutines mean concurrent code looks like sequential code.
Write utilities that work for 1 host or 1000:

```go
var wg sync.WaitGroup
errs := make(chan error)

go func() {
    for err := range errs {
        log.Print(err)
    }
}()

for _, host := range hosts {
    wg.Add(1)
    go func(h string) {
        defer wg.Done()
        m := ssh.Machine(sys.Machine(), h)
        err := command.Do(ctx, m, "systemctl", "restart", "app")
        if err != nil {
            errs <- fmt.Errorf("%s: %w", h, err)
        }
    }(host)
}
wg.Wait()
close(errs)
```

The missing piece is ergonomics around command execution.
Piping commands in shell is trivial:

```bash
generate-data | process
```

In Go, it's machinery:

```go
cmd1 := exec.Command("generate-data")
cmd2 := exec.Command("process")
stdout, _ := cmd1.StdoutPipe()
cmd2.Stdin = stdout
cmd1.Start()
cmd2.Start()
cmd1.Wait()
cmd2.Wait()
```

Command buffers fix that.
Local code looks like remote code looks like testable code.

## `command.FS`

Filesystem operations use the same patterns.

```go
ctx, sh := context.Background(), command.Shell(m)

// Copy a file - looks just like copying between commands.
_, err := io.Copy(
    sh.CreateBuffer(ctx, "output.txt"),
    sh.OpenBuffer(ctx, "input.txt"),
)
if err != nil {
    log.Fatal(err)
}
```

The [`lesiw.io/fs`](https://pkg.go.dev/lesiw.io/fs) package extends Go's
`fs.FS` with context-aware operations,
perfect for long-running remote filesystem operations.

## Real-World Example: Installing Go

The [example Go installer](../internal/example/go/main.go) demonstrates the
abstraction. It:

- Probes the target system for OS/architecture.
- Downloads the appropriate Go toolchain on the host.
- Streams it into a minimal container via the filesystem.
- Installs and runs Go programs.

The same code works locally or in a container.
No package manager, no curl, no wget required: everything
streams through the filesystem from the host.

This is automation that doesn't assume anything about the target environment.

## `go test`

Use a `mock.Machine` to program responses:

```go
ctx, m := context.Background(), new(mock.Machine)
m.Return(strings.NewReader("v1.0.0\n"), "git", "describe", "--tags")
m.Return(strings.NewReader(""), "git", "push", "origin", "v1.0.0")

sh := command.Shell(m, "git")
version, err := sh.Read(ctx, "git", "describe", "--tags")
if err != nil {
    log.Fatal(err)
}

if err := sh.Exec(ctx, "git", "push", "origin", version); err != nil {
    log.Fatal(err)
}

// Verify what was called.
calls := mock.Calls(sh, "git")
```

Or write your own command machine in a few lines: `command.MachineFunc`
adapts any function.

## Declarative Shell, Imperative Filling

Automation code is imperative: you write the exact steps to execute.
But `command.Shell` adds a declarative layer:
you declare which commands you need upfront.

```go
sh := command.Shell(sys.Machine(), "go", "git", "docker")
```

This prevents accidentally relying on commands you haven't declared.
When moving automation from your local machine to a container or VM,
the list of required commands is self-documenting.
If a command isn't available,
you'll know immediately rather than discovering it halfway through execution.

The rest is pure imperative control: no YAML schemas, no DSLs.
Just write Go.

```go
// Create a container.
ctx := context.Background()
m := ctr.Machine(sys.Machine(), "ubuntu:latest")
defer command.Shutdown(ctx, m)
sh := command.Shell(m, "apt-get")

// Install packages.
if err := sh.Exec(ctx, "apt-get", "update"); err != nil {
    log.Fatal(err)
}
err := sh.Exec(ctx, "apt-get", "install", "-y", "build-essential")
if err != nil {
    log.Fatal(err)
}

// Copy in your application.
localSh := command.Shell(sys.Machine())
_, err := io.Copy(
    sh.CreateBuffer(ctx, "/app/server"),
    localSh.OpenBuffer(ctx, "./bin/server"),
)
if err != nil {
    log.Fatal(err)
}

// Commit the container.
if err := command.Exec(ctx, sys.Machine(),
    "docker", "commit", containerID, "myapp:latest",
); err != nil {
    log.Fatal(err)
}
```

## Getting Started

```bash
go get lesiw.io/command
```

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
    ctx, sh := context.Background(), command.Shell(sys.Machine(), "go")

    if err := sh.Exec(ctx, "go", "version"); err != nil {
        log.Fatal(err)
    }

    version, err := sh.Read(ctx, "go", "version")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Go version:", version)
}
```

**Documentation:** [pkg.go.dev/lesiw.io/command](https://pkg.go.dev/lesiw.io/command)

## License

BSD 3-Clause
