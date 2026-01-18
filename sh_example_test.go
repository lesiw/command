package command_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"lesiw.io/command"
	"lesiw.io/command/mem"
)

func ExampleSh_OS() {
	ctx, sh := context.Background(), command.Shell(mem.Machine())

	// OS detection is cached after first call
	os := sh.OS(ctx)
	fmt.Println(os)
	// Output:
	// linux
}

func ExampleSh_Arch() {
	ctx, sh := context.Background(), command.Shell(mem.Machine())

	// Architecture detection is cached after first call
	arch := sh.Arch(ctx)
	fmt.Println(arch)
	// Output:
	// amd64
}

func ExampleSh_Env() {
	ctx := context.Background()
	ctx = command.WithEnv(ctx, map[string]string{
		"MY_VAR": "test_value",
	})
	sh := command.Shell(mem.Machine())
	val := sh.Env(ctx, "MY_VAR")
	fmt.Println(val)
	// Output:
	// test_value
}

func ExampleSh_WriteFile() {
	ctx, sh := context.Background(), command.Shell(mem.Machine())
	err := sh.WriteFile(ctx, "message.txt", []byte("Hello from Sh!"))
	if err != nil {
		log.Fatal(err)
	}
	content, err := sh.ReadFile(ctx, "message.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(content))
	// Output:
	// Hello from Sh!
}

func ExampleSh_ReadFile() {
	ctx, sh := context.Background(), command.Shell(mem.Machine())
	if err := sh.WriteFile(ctx, "data.txt", []byte("content")); err != nil {
		log.Fatal(err)
	}
	data, err := sh.ReadFile(ctx, "data.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(data))
	// Output:
	// content
}

func ExampleSh_Create() {
	ctx, sh := context.Background(), command.Shell(mem.Machine())
	f, err := sh.Create(ctx, "new.txt")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := f.Write([]byte("created")); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
	content, err := sh.ReadFile(ctx, "new.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(content))
	// Output:
	// created
}

func ExampleSh_Open() {
	ctx, sh := context.Background(), command.Shell(mem.Machine())
	if err := sh.WriteFile(ctx, "file.txt", []byte("hello")); err != nil {
		log.Fatal(err)
	}
	f, err := sh.Open(ctx, "file.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(data))
	// Output:
	// hello
}

func ExampleSh_Append() {
	ctx, sh := context.Background(), command.Shell(mem.Machine())
	if err := sh.WriteFile(ctx, "log.txt", []byte("line1\n")); err != nil {
		log.Fatal(err)
	}
	f, err := sh.Append(ctx, "log.txt")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := f.Write([]byte("line2\n")); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
	content, err := sh.ReadFile(ctx, "log.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(content))
	// Output:
	// line1
	// line2
}

func ExampleSh_Mkdir() {
	ctx, sh := context.Background(), command.Shell(mem.Machine())
	if err := sh.Mkdir(ctx, "newdir"); err != nil {
		log.Fatal(err)
	}
	for entry, err := range sh.ReadDir(ctx, ".") {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(entry.Name())
	}
	// Output:
	// newdir
}

func ExampleSh_MkdirAll() {
	ctx, sh := context.Background(), command.Shell(mem.Machine())
	if err := sh.MkdirAll(ctx, "a/b/c"); err != nil {
		log.Fatal(err)
	}
	for entry, err := range sh.ReadDir(ctx, "a/b") {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(entry.Name())
	}
	// Output:
	// c
}

func ExampleSh_Remove() {
	ctx, sh := context.Background(), command.Shell(mem.Machine())
	if err := sh.WriteFile(ctx, "file.txt", []byte("content")); err != nil {
		log.Fatal(err)
	}
	if err := sh.Remove(ctx, "file.txt"); err != nil {
		log.Fatal(err)
	}
	var n int
	for _, err := range sh.ReadDir(ctx, ".") {
		if err != nil {
			log.Fatal(err)
		}
		n++
	}
	if n == 0 {
		fmt.Println("(empty)")
	}
	// Output:
	// (empty)
}

func ExampleSh_RemoveAll() {
	ctx, sh := context.Background(), command.Shell(mem.Machine())
	if err := sh.MkdirAll(ctx, "dir/subdir"); err != nil {
		log.Fatal(err)
	}
	err := sh.WriteFile(ctx, "dir/file.txt", []byte("content"))
	if err != nil {
		log.Fatal(err)
	}
	if err := sh.RemoveAll(ctx, "dir"); err != nil {
		log.Fatal(err)
	}
	var n int
	for _, err := range sh.ReadDir(ctx, ".") {
		if err != nil {
			log.Fatal(err)
		}
		n++
	}
	if n == 0 {
		fmt.Println("(empty)")
	}
	// Output:
	// (empty)
}

func ExampleSh_Rename() {
	ctx, sh := context.Background(), command.Shell(mem.Machine())
	if err := sh.WriteFile(ctx, "old.txt", []byte("data")); err != nil {
		log.Fatal(err)
	}
	if err := sh.Rename(ctx, "old.txt", "new.txt"); err != nil {
		log.Fatal(err)
	}
	content, err := sh.ReadFile(ctx, "new.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(content))
	// Output:
	// data
}

func ExampleSh_ReadDir() {
	ctx, sh := context.Background(), command.Shell(mem.Machine())
	if err := sh.WriteFile(ctx, "files/a.txt", []byte("")); err != nil {
		log.Fatal(err)
	}
	if err := sh.WriteFile(ctx, "files/b.txt", []byte("")); err != nil {
		log.Fatal(err)
	}
	for entry, err := range sh.ReadDir(ctx, "files") {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(entry.Name())
	}
	// Unordered output:
	// a.txt
	// b.txt
}

func ExampleSh_Walk() {
	ctx, sh := context.Background(), command.Shell(mem.Machine())
	if err := sh.WriteFile(ctx, "a/file1.txt", []byte("")); err != nil {
		log.Fatal(err)
	}
	if err := sh.WriteFile(ctx, "a/b/file2.txt", []byte("")); err != nil {
		log.Fatal(err)
	}
	for entry, err := range sh.Walk(ctx, "a", -1) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(entry.Name())
	}
	// Unordered output:
	// b
	// file1.txt
	// file2.txt
}

func ExampleSh_Glob() {
	ctx, sh := context.Background(), command.Shell(mem.Machine())
	if err := sh.WriteFile(ctx, "file1.txt", []byte("")); err != nil {
		log.Fatal(err)
	}
	if err := sh.WriteFile(ctx, "file2.txt", []byte("")); err != nil {
		log.Fatal(err)
	}
	if err := sh.WriteFile(ctx, "data.json", []byte("")); err != nil {
		log.Fatal(err)
	}
	matches, err := sh.Glob(ctx, "*.txt")
	if err != nil {
		log.Fatal(err)
	}
	for _, match := range matches {
		fmt.Println(match)
	}
	// Unordered output:
	// ./file1.txt
	// ./file2.txt
}

func ExampleSh_Stat() {
	ctx, sh := context.Background(), command.Shell(mem.Machine())
	if err := sh.WriteFile(ctx, "test.txt", []byte("hello")); err != nil {
		log.Fatal(err)
	}
	info, err := sh.Stat(ctx, "test.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(info.Name())
	// Output:
	// test.txt
}

func ExampleSh_FS() {
	ctx, sh := context.Background(), command.Shell(mem.Machine())
	err := sh.WriteFile(ctx, "message.txt", []byte("Hello from Sh!"))
	if err != nil {
		log.Fatal(err)
	}
	buf, err := sh.ReadFile(ctx, "message.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(buf))
	// Output:
	// Hello from Sh!
}

func ExampleSh_FS_readDir() {
	ctx, sh := context.Background(), command.Shell(mem.Machine())
	if err := sh.WriteFile(ctx, "logs/error.log", []byte("")); err != nil {
		log.Fatal(err)
	}
	if err := sh.WriteFile(ctx, "logs/access.log", []byte("")); err != nil {
		log.Fatal(err)
	}
	for entry, err := range sh.ReadDir(ctx, "logs") {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(entry.Name())
	}
	// Unordered output:
	// access.log
	// error.log
}

func ExampleSh_FS_workflow() {
	ctx, sh := context.Background(), command.Shell(mem.Machine()).
		Handle("tr", mem.Machine())
	err := sh.WriteFile(ctx, "input/data.txt", []byte("Hello World"))
	if err != nil {
		log.Fatal(err)
	}
	input, err := sh.ReadFile(ctx, "input/data.txt")
	if err != nil {
		log.Fatal(err)
	}
	var buf strings.Builder
	_, err = command.Copy(
		&buf,
		strings.NewReader(string(input)),
		sh.NewFilter(ctx, "tr", "A-Z", "a-z"),
	)
	if err != nil {
		log.Fatal(err)
	}
	err = sh.WriteFile(ctx, "output/result.txt", []byte(buf.String()))
	if err != nil {
		log.Fatal(err)
	}
	result, err := sh.ReadFile(ctx, "output/result.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(result))
	// Output:
	// hello world
}

func ExampleSh_Read() {
	ctx, sh := context.Background(), command.Shell(mem.Machine(), "echo")
	out, err := sh.Read(ctx, "echo", "hello world")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
	// Output:
	// hello world
}

func ExampleSh_Command() {
	ctx, sh := context.Background(), command.Shell(mem.Machine(), "tr")
	var buf strings.Builder
	_, err := command.Copy(
		&buf,
		strings.NewReader("hello world"),
		sh.NewFilter(ctx, "tr", "a-z", "A-Z"),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(buf.String())
	// Output:
	// HELLO WORLD
}

func ExampleSh_Unshell() {
	ctx := context.Background()
	sh := command.Shell(mem.Machine())
	sh = sh.Handle("tr", sh.Unshell())
	var buf strings.Builder
	_, err := command.Copy(
		&buf,
		strings.NewReader("hello"),
		sh.NewFilter(ctx, "tr", "a-z", "A-Z"),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(buf.String())
	// Output:
	// HELLO
}

func ExampleSh_CreateBuffer() {
	ctx := context.Background()
	sh := command.Shell(mem.Machine(), "echo")

	_, err := io.Copy(
		sh.CreateBuffer(ctx, "output.txt"),
		sh.NewReader(ctx, "echo", "Hello, World!"),
	)
	if err != nil {
		log.Fatal(err)
	}

	content, err := sh.ReadFile(ctx, "output.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(content))
	// Output:
	// Hello, World!
}
