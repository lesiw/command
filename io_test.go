package command

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/google/go-cmp/cmp"
)

func TestCopyErrorCollection(t *testing.T) {
	var (
		errRead       = errors.New("read failed")
		errProcessing = errors.New("processing failed")

		src  = &readStringer{iotest.ErrReader(errRead), "source reader"}
		mid1 = namedRW("transform 1")
		mid2 = struct {
			io.Reader
			io.Writer
		}{iotest.ErrReader(errProcessing), io.Discard}
		dst = io.Discard
	)
	_, err := Copy(dst, src, mid1, mid2)
	if err == nil {
		t.Fatal("Copy() error = nil, want error")
	}
	want := strings.TrimSpace(`
source reader
	read failed

transform 1
	<success>

<struct { io.Reader; io.Writer }>
	processing failed
`)
	if got := err.Error(); !cmp.Equal(got, want) {
		t.Errorf("Error() mismatch (-want +got):\n%s", cmp.Diff(want, got))
	}
	if !errors.Is(err, errRead) {
		t.Error("error chain missing errRead")
	}
	if !errors.Is(err, errProcessing) {
		t.Error("error chain missing errProcessing")
	}
}

func TestCopySuccessNoError(t *testing.T) {
	var buf bytes.Buffer
	src := strings.NewReader("data")
	pr, pw := io.Pipe()
	mid := struct {
		io.Reader
		io.Writer
		io.Closer
	}{pr, pw, pw}

	n, err := Copy(&buf, src, mid)
	if err != nil {
		t.Errorf("Copy() error = %v, want nil", err)
	}
	if got, want := n, int64(8); got != want {
		t.Errorf("Copy() = %d bytes, want %d", got, want)
	}
	if got, want := buf.String(), "data"; got != want {
		t.Errorf("buf = %q, want %q", got, want)
	}
}

func TestReadAll(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{{
		name:  "basic",
		input: "hello world",
		want:  "hello world",
	}, {
		name:  "trailing newline",
		input: "hello world\n",
		want:  "hello world",
	}, {
		name:  "trailing newlines",
		input: "hello world\n\n",
		want:  "hello world",
	}, {
		name:  "trailing crlf",
		input: "hello world\r\n\r\n",
		want:  "hello world",
	}, {
		name:  "preserves trailing spaces",
		input: "hello world  ",
		want:  "hello world  ",
	}, {
		name:  "preserves trailing tabs",
		input: "hello world\t\t",
		want:  "hello world\t\t",
	}, {
		name:  "empty",
		input: "",
		want:  "",
	}, {
		name:  "only newlines",
		input: "\n\n",
		want:  "",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadAll(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ReadAll() = %q, want %q", got, tt.want)
			}
		})
	}
}
