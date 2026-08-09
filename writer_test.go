package command

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

// closeOnCleanup registers a cleanup that closes c and reports errors.
// Multiple Close() calls are safe (Close returns nil if already closed).
func closeOnCleanup(t *testing.T, c io.Closer) {
	t.Helper()
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Errorf("cleanup Close() failed: %v", err)
		}
	})
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

func TestWriterWrite(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewWriter(t.Context(), MachineFunc(func(
		context.Context, ...string,
	) Buffer {
		return struct {
			io.Reader
			io.Writer
			io.Closer
		}{strings.NewReader(""), buf, nopCloser{}}
	}))
	closeOnCleanup(t, w)
	if n, err := w.Write([]byte("hello world")); err != nil {
		t.Fatalf("Write() error = %v", err)
	} else if n != 11 {
		t.Errorf("Write() = %d bytes, want 11", n)
	}
	if got := buf.String(); got != "hello world" {
		t.Errorf("buffer = %q, want %q", got, "hello world")
	}
}

func TestWriterNoOpIfUnused(t *testing.T) {
	var (
		cmdCtx context.Context
		w      = NewWriter(t.Context(), MachineFunc(func(
			ctx context.Context, _ ...string,
		) Buffer {
			cmdCtx = ctx
			return struct {
				io.Reader
				io.Writer
				io.Closer
			}{strings.NewReader(""), io.Discard, nopCloser{}}
		}))
	)
	closeOnCleanup(t, w)
	if err := w.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if cmdCtx != nil {
		select {
		case <-cmdCtx.Done():
			t.Error("context was canceled even though writer was unused")
		default:
		}
	}
}

func TestWriterClosesUnderlyingWriter(t *testing.T) {
	var (
		closed closeTracker
		w      = NewWriter(t.Context(), MachineFunc(func(
			context.Context, ...string,
		) Buffer {
			return &closed
		}))
	)
	closeOnCleanup(t, w)
	if _, err := w.Write([]byte("data")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if !bool(closed) {
		t.Error("Close() did not close underlying writer")
	}
}

func TestWriterReadError(t *testing.T) {
	w := NewWriter(t.Context(), MachineFunc(func(
		context.Context, ...string,
	) Buffer {
		return struct {
			io.Reader
			io.Writer
			io.Closer
		}{
			Reader: Fail(io.ErrUnexpectedEOF),
			Writer: io.Discard,
			Closer: new(closeTracker),
		}
	}))
	closeOnCleanup(t, w)
	if _, err := w.Write([]byte("data")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := w.Close(); err != io.ErrUnexpectedEOF {
		t.Errorf("Close() error = %v, want io.ErrUnexpectedEOF", err)
	}
}

func TestWriterWriteAfterClose(t *testing.T) {
	w := NewWriter(t.Context(), MachineFunc(func(
		context.Context, ...string,
	) Buffer {
		return struct {
			io.Reader
			io.Writer
			io.Closer
		}{strings.NewReader(""), new(bytes.Buffer), nopCloser{}}
	}))
	closeOnCleanup(t, w)
	if _, err := w.Write([]byte("data")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if _, err := w.Write([]byte("more")); err != ErrClosed {
		t.Errorf("Write() after Close() error = %v, want ErrClosed", err)
	}
}

func TestWriterWriteAfterCloseUnused(t *testing.T) {
	w := NewWriter(t.Context(), MachineFunc(func(
		context.Context, ...string,
	) Buffer {
		return struct {
			io.Reader
			io.Writer
			io.Closer
		}{strings.NewReader(""), io.Discard, nopCloser{}}
	}))
	closeOnCleanup(t, w)
	if err := w.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if n, err := w.Write([]byte("data")); err != ErrClosed {
		t.Errorf("Write() after unused Close() = %v, want ErrClosed", err)
	} else if n != 0 {
		t.Errorf("Write() = %d bytes, want 0", n)
	}
}

func TestWriterMultipleClose(t *testing.T) {
	w := NewWriter(t.Context(), MachineFunc(func(
		context.Context, ...string,
	) Buffer {
		return struct {
			io.Reader
			io.Writer
			io.Closer
		}{strings.NewReader(""), new(bytes.Buffer), nopCloser{}}
	}))
	closeOnCleanup(t, w)
	if _, err := w.Write([]byte("data")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("First Close() error = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

//ignore:errcheck
func TestWriterConcurrentWriteClose(t *testing.T) {
	// Test concurrent Write() and Close() calls to detect race conditions.
	for range 1000 {
		w := NewWriter(t.Context(), MachineFunc(func(
			context.Context, ...string,
		) Buffer {
			return struct {
				io.Reader
				io.Writer
				io.Closer
			}{strings.NewReader(""), new(bytes.Buffer), nopCloser{}}
		}))
		closeOnCleanup(t, w)

		go w.Write([]byte("data"))
		go w.Close()
	}
}
