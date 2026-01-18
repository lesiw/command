// Package command provides command buffers.
//
// A [Buffer] represents a command's execution and its output.
// Buffers begin executing on the first Read and complete at [io.EOF].
//
// Buffers may implement [WriteBuffer] to accept input, like stdin.
// They may also implement [LogBuffer] to log diagnostics, like stderr.
// The standard [Buffer] is an [io.Reader] over stdout.
//
// Buffers are created by a [Machine].
// [lesiw.io/command/sys] is a [Machine] that creates command buffers
// on the local system.
//
// Other Machines provided by this package:
//   - [lesiw.io/command/mem] - in-memory Machine for examples
//   - [lesiw.io/command/ctr] - executes commands in containers
//   - [lesiw.io/command/ssh] - executes commands over SSH
//   - [lesiw.io/command/sub] - prefixes commands with fixed arguments
//   - [lesiw.io/command/mock] - mock Machine for testing
//
// Use [NewReader] and [NewWriter] to construct Buffers.
// [NewReader] is an [io.ReadCloser], where Close() stops the command early.
// [NewWriter] is an [io.WriteCloser], where Close() closes the input stream
// and waits for the command to finish.
//
// Buffers are usable with standard [io].
//
//	// Example only: use OS(ctx, m) and Arch(ctx, m).
//	uname, err := io.ReadAll(m.Command(ctx, "uname", "-a"))
//
// They can be piped with [io.Copy].
//
//	// Example only: use fs.WriteFile(ctx, FS(m)).
//	io.Copy(
//	    command.NewWriter(ctx, m, "tee", "hello.txt"),
//	    command.NewReader(ctx, m, "echo", "Hello world!"),
//	)
//
// [Copy] is a generalization of [io.Copy],
// allowing three or more buffers to be piped together.
// Commands in the middle of the [Copy] must be [io.ReadWriter].
// [NewFilter] returns an [io.ReadWriteCloser] for piping.
//
// Helpers are available for common buffer operations.
// [Do] creates and executes a [Buffer], discarding its output to [io.Discard].
// [Read] creates and executes a [Buffer], then returns its output as a string.
// Trailing whitespace is removed, like command substitution in a shell.
// [Exec] creates and executes a [Buffer],
// streaming output to the terminal rather than capturing it.
// When possible, the underlying command's standard streams are attached
// directly to the controlling terminal, letting it run interactively.
//
// Environment variables are part of the [context.Context].
// They can be set using [WithEnv] and inspected using [Env].
//
//	m := mem.Machine()
//	ctx := command.WithEnv(context.Background(), map[string]string{
//	    "CGO_ENABLED": "0",
//	})
//	command.Exec(ctx, m, "go", "build", ".")
//
// # Files
//
// [FS] provides a [lesiw.io/fs.FS] that can be accessed
// using [lesiw.io/fs] top-level functions.
//
//	fsys := command.FS(m)
//	fs.WriteFile(ctx, fsys, []byte("Hello world!"), "hello.txt")
//
// If the underlying [Machine] is a [FSMachine],
// [FS] will return the [lesiw.io/fs.FS] presented by the Machine.
// Otherwise, it will return a [lesiw.io/fs.FS] that uses commands
// to provide filesystem access.
//
// The default [FS] probes the remote system to determine
// which commands to use for filesystem operations.
// For example, on a Unix-like system, [fs.Remove] might use rm,
// whereas on a Windows system, it might use Remove-Item or del.
//
// For composing file operations with [io] primitives, use
// [fs.OpenBuffer], [fs.CreateBuffer], and [fs.AppendBuffer].
// These return lazy-executing [io.ReadCloser] and [io.WriteCloser]
// that defer opening files until first Read or Write.
//
//	io.Copy(
//	    fs.CreateBuffer(ctx, fsys, "output.txt"),
//	    fs.OpenBuffer(ctx, fsys, "input.txt"),
//	)
//
// # Shells
//
// A [Machine] is a broadly applicable concept.
// A simple function can be adapted into a Machine via [MachineFunc].
//
// [Shell] provides a useful abstraction over a [Machine]
// for Machines that run commands and store state in a filesystem:
// that is to say, a typical computing environment.
//
// A Shell's methods mirror the top level functions of this package
// and of [lesiw.io/fs].
//
// Commands must be explicitly registered on Shells.
// This encourages users to use commands only when necessary
// and to rely on portable abstractions when possible.
// For instance, reading a file via [fs.ReadFile]
// instead of registering "cat".
//
//	goMachine := command.Shell(sys.Machine(), "go")
//	goMachine.Exec(ctx, "go", "run", ".")
//
// It is encouraged to register all external commands at
// the Shell's construction.
// If commands must be registered later,
// they can be done by registering that command with [Sh.Unshell],
// which returns the underlying [Machine].
//
//	sh := command.Shell(sys.Machine())
//	sh = sh.Handle("go", sh.Unshell())
//
// Here is an example of typical Shell usage.
// Note that external commands are kept to a minimum
// and portable operations are preferred where possible -
// for example, using ReadFile over cat.
//
//	ctx, sh := context.Background(), command.Shell(sys.Machine(), "go")
//
//	if err := sh.Exec(ctx, "go", "mod", "tidy"); err != nil {
//	    log.Fatalf("go mod tidy failed: %v", err)
//	}
//
//	if err := sh.Exec(ctx, "go", "test", "./..."); err != nil {
//	    log.Fatalf("tests failed: %v", err)
//	}
//
//	ver, err := sh.ReadFile(ctx, "VERSION")
//	if err != nil {
//	    ver = []byte("dev")
//	}
//
//	if err := sh.MkdirAll(ctx, "bin"); err != nil {
//	    log.Fatalf("failed to create bin directory: %v", err)
//	}
//	err = sh.Exec(
//	    command.WithEnv(ctx, map[string]string{
//	        "CGO_ENABLED": "0",
//	    }),
//	    "go", "build",
//		"-ldflags", fmt.Sprintf(
//			"-X main.version=%s", strings.TrimSpace(string(ver)),
//		),
//		"-o", "bin/app", ".",
//	)
//	if err != nil {
//	    log.Fatalf("build failed: %v", err)
//	}
//
//	info, err := sh.Stat(ctx, "bin/app")
//	if err != nil {
//	    log.Fatalf("binary not found: %v", err)
//	}
//
//	fmt.Printf("Built %s (%d bytes)\n", info.Name(), info.Size())
//
// # Cookbook
//
// Some common operations in shellcode expressed as Go with command buffers.
//
// Creating an executable file.
//
//	sh.WriteFile(
//		fs.WithFileMode(ctx, 0755),
//		"hello.sh",
//		[]byte(`#!/bin/sh
//	echo "Hello world!"`),
//	)
//
// Field-splitting (parsing whitespace-separated fields).
//
//	f, err := sh.Open("access.log")
//	if err != nil {
//		log.Fatal(err)
//	}
//	scn := bufio.NewScanner(f)
//	for scn.Scan() {
//		fields := strings.Fields(scn.Text())
//		if len(fields) > 0 {
//			fmt.Println("IP:", fields[0])
//		}
//	}
//
// Copying a file or directory.
//
//	dst, err := remoteSh.Create(ctx, "foo") // Or "foo/" for directory.
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer dst.Close()
//	src, err := localSh.Open(ctx, "foo") // Or "foo/" for directory.
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer src.Close()
//	if _, err := io.Copy(dst, src); err != nil {
//		log.Fatal(err)
//	}
//
// Command substitution (capturing command output).
//
//	version, err := command.Read(ctx, sh, "git", "describe", "--tags")
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("Building version %s\n", version)
//
// Appending to a file.
//
//	f, err := sh.Append(ctx, "app.log")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer f.Close()
//	fmt.Fprintln(f, "Log entry")
//
// Creating a temporary file.
//
//	f, err := sh.Temp(ctx, "data") // Or "data/" for directory.
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer sh.RemoveAll(ctx, f.Path())
//	defer f.Close()
//	fmt.Fprintf(f, "input data")
//
//	if err := sh.Exec(ctx, "process", f.Path()); err != nil {
//		log.Fatal(err)
//	}
//
// Checking if a file exists.
//
//	if _, err := sh.Stat(ctx, "config.yaml"); err != nil {
//		log.Fatal("config.yaml not found")
//	}
//
// Searching a file for a substring.
//
//	f, err := sh.Open(ctx, "app.log")
//	if err != nil {
//		log.Fatal(err)
//	}
//	scn := bufio.NewScanner(f)
//	for scn.Scan() {
//		if strings.Contains(scn.Text(), "ERROR") {
//			fmt.Println(scn.Text())
//		}
//	}
//
// Searching a file with a regular expression.
//
//	re := regexp.MustCompile(`\bTODO\b`)
//	f, err := sh.Open(ctx, "main.go")
//	if err != nil {
//		log.Fatal(err)
//	}
//	scn := bufio.NewScanner(f)
//	for scn.Scan() {
//		if re.MatchString(scn.Text()) {
//			fmt.Println(scn.Text())
//		}
//	}
//
// # Testing
//
// For tests requiring simple machines, use [MachineFunc].
// For more complex scenarios, use [lesiw.io/command/mock].
//
// Responses queue in a [lesiw.io/command/mock.Machine].
// When deciding which response to return,
// more specific commands take precedent over less specific ones.
//
//	m := new(mock.Machine)
//	m.Return(strings.NewReader("hello\n"), "echo")
//	m.Return(strings.NewReader(""), "exit")
//	m.Return(command.Fail(&command.Error{Code: 1}), "exit", "1")
//
//	out, err := command.Read(ctx, m, "echo")
//	if err != nil {
//		t.Fatal(err)
//	}
//	if out != "hello" {
//		t.Errorf("got %q, want %q", out, "hello")
//	}
//
//	if err := command.Do(ctx, m, "exit"); err == nil {
//		t.Error("expected error from exit command")
//	}
//
// Use [lesiw.io/command/mock.Calls] to retrieve calls
// when working with a Shelled [lesiw.io/command/mock.Machine].
//
//	m := new(mock.Machine)
//	m.Return(strings.NewReader("main\n"), "git", "branch", "--show-current")
//	m.Return(strings.NewReader(""), "git", "push", "origin", "main")
//
//	sh := command.Shell(m, "git")
//	branch, err := sh.Read(ctx, "git", "branch", "--show-current")
//	if err != nil {
//		t.Fatal(err)
//	}
//	if branch != "main" {
//		t.Errorf("got %q, want %q", branch, "main")
//	}
//
//	if err := sh.Exec(ctx, "git", "push", "origin", "main"); err != nil {
//		t.Fatal(err)
//	}
//
//	calls := mock.Calls(sh, "git")
//	if len(calls) != 2 {
//		t.Errorf("got %d git calls, want 2", len(calls))
//	}
//
// [github.com/google/go-cmp/cmp] is useful for comparing calls.
//
//	m := new(mock.Machine)
//	m.Return(strings.NewReader("v1.0.0\n"), "git", "describe", "--tags")
//	m.Return(strings.NewReader(""), "git", "push", "origin", "v1.0.0")
//
//	sh := command.Shell(m, "git")
//	sh.Read(ctx, "git", "describe", "--tags")
//	sh.Exec(ctx, "git", "push", "origin", "v1.0.0")
//
//	got := mock.Calls(sh, "git")
//	want := []mock.Call{
//		{Args: []string{"git", "describe", "--tags"}},
//		{Args: []string{"git", "push", "origin", "v1.0.0"}},
//	}
//	if !cmp.Equal(want, got) {
//		t.Errorf("git calls mismatch (-want +got):\n%s", cmp.Diff(want, got))
//	}
//
// # Tracing
//
// [Trace] can optionally be set to any [io.Writer], including [os.Stderr].
// Commands are traced when buffers are created via [Exec], [Read], or [Do],
// before any I/O operations begin.
// [lesiw.io/command/sys] provides output that mimics set +x.
package command
