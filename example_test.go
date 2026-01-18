package command_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"lesiw.io/command"
	"lesiw.io/command/mem"
	"lesiw.io/command/mock"
	"lesiw.io/fs"
)

func ExampleCopy() {
	ctx, m := context.Background(), mem.Machine()
	var buf bytes.Buffer
	_, err := command.Copy(
		&buf,
		strings.NewReader("hello world"),
		command.NewFilter(ctx, m, "tr", "a-z", "A-Z"),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(buf.String())
	// Output:
	// HELLO WORLD
}

func ExampleRead() {
	ctx, m := context.Background(), mem.Machine()
	out, err := command.Read(ctx, m, "echo", "hello world")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
	// Output:
	// hello world
}

func ExampleWithEnv() {
	ctx, m := context.Background(), mem.Machine()
	ctx = command.WithEnv(ctx, map[string]string{
		"HOME": "/home/mem",
	})
	fmt.Println("HOME:", command.Env(ctx, m, "HOME"))
	// Output:
	// HOME: /home/mem
}

func ExampleWithEnv_multiple() {
	m := mem.Machine()
	ctx1 := command.WithEnv(context.Background(), map[string]string{
		"HOME": "/",
		"TEST": "foobar",
	})
	ctx2 := command.WithEnv(ctx1, map[string]string{
		"HOME": "/home/example",
	})
	fmt.Println("ctx1(HOME):", command.Env(ctx1, m, "HOME"))
	fmt.Println("ctx1(TEST):", command.Env(ctx1, m, "TEST"))
	fmt.Println("ctx2(HOME):", command.Env(ctx2, m, "HOME"))
	fmt.Println("ctx2(TEST):", command.Env(ctx2, m, "TEST"))
	// Output:
	// ctx1(HOME): /
	// ctx1(TEST): foobar
	// ctx2(HOME): /home/example
	// ctx2(TEST): foobar
}

func ExampleShell() {
	ctx := context.Background()
	sh := command.Shell(mem.Machine(), "tr", "cat")

	var buf bytes.Buffer
	_, err := command.Copy(
		&buf,
		strings.NewReader("hello"),
		command.NewFilter(ctx, sh, "tr", "a-z", "A-Z"),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(buf.String())
	// Output:
	// HELLO
}

func ExampleHandle() {
	m, ctx := mem.Machine(), context.Background()
	uname := new(mock.Machine)
	uname.Return(strings.NewReader("fakeOS"), "uname")
	m = command.Handle(m, "uname", uname)

	str, err := command.Read(ctx, m, "uname")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(str)
	// Output:
	// fakeOS
}

func ExampleHandleFunc() {
	m, ctx := mem.Machine(), context.Background()
	m = command.HandleFunc(m, "uppercase",
		func(ctx context.Context, args ...string) command.Buffer {
			return strings.NewReader(strings.ToUpper(args[1]))
		},
	)
	out, err := command.Read(ctx, m, "uppercase", "hello")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
	// Output:
	// HELLO
}

func Example_trace() {
	command.Trace = os.Stdout // For capture only: consider os.Stderr instead.
	defer func() { command.Trace = io.Discard }()

	m, ctx := mem.Machine(), context.Background()
	ctx = command.WithEnv(ctx, map[string]string{"MY_VAR": "test"})

	out, err := command.Read(ctx, m, "echo", "hello")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
	out, err = command.Read(ctx, m, "echo", "world")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
	// Output:
	// MY_VAR=test echo hello
	// hello
	// MY_VAR=test echo world
	// world
}

func ExampleCreateBuffer() {
	ctx, m := context.Background(), mem.Machine()
	fsys := command.FS(m)

	_, err := io.Copy(
		fs.CreateBuffer(ctx, fsys, "message.txt"),
		command.NewReader(ctx, m, "echo", "Hello, World!"),
	)
	if err != nil {
		log.Fatal(err)
	}

	out, err := command.Read(ctx, m, "cat", "message.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
	// Output:
	// Hello, World!
}
