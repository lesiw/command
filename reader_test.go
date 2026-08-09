package command

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestReaderRead(t *testing.T) {
	r := NewReader(t.Context(), MachineFunc(func(
		context.Context, ...string,
	) Buffer {
		return strings.NewReader("hello world")
	}))
	closeOnCleanup(t, r)
	if out, err := io.ReadAll(r); err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	} else if got, want := string(out), "hello world"; got != want {
		t.Errorf("ReadAll() = %q, want %q", got, want)
	}
}

func TestReaderCancelOnClose(t *testing.T) {
	var (
		ctxs = make(chan context.Context, 1)
		r    = NewReader(t.Context(), MachineFunc(func(
			ctx context.Context, _ ...string,
		) Buffer {
			ctxs <- ctx
			return strings.NewReader("data")
		}))
	)
	closeOnCleanup(t, r)
	if _, err := r.Read(make([]byte, 1)); err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	ctx := <-ctxs
	select {
	case <-ctx.Done():
	default:
		t.Error("Close() did not cancel context")
	}
}

func TestReaderNoOpIfUnused(t *testing.T) {
	var (
		ctxs = make(chan context.Context, 1)
		r    = NewReader(t.Context(), MachineFunc(func(
			ctx context.Context, _ ...string,
		) Buffer {
			ctxs <- ctx
			return strings.NewReader("data")
		}))
	)
	closeOnCleanup(t, r)
	if err := r.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	select {
	case ctx := <-ctxs:
		select {
		case <-ctx.Done():
			t.Error("context was canceled even though reader was unused")
		default:
		}
	default:
	}
}

func TestReaderClosesUnderlyingReader(t *testing.T) {
	var (
		closed closeTracker
		r      = NewReader(t.Context(), MachineFunc(func(
			context.Context, ...string,
		) Buffer {
			return &closed
		}))
	)
	closeOnCleanup(t, r)
	if _, err := r.Read(make([]byte, 1)); err != nil && err != io.EOF {
		t.Fatalf("Read() error = %v", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if !bool(closed) {
		t.Error("Close() did not close underlying reader")
	}
}

func TestReaderNoCloseIfUnused(t *testing.T) {
	var (
		closed closeTracker
		r      = NewReader(t.Context(), MachineFunc(func(
			context.Context, ...string,
		) Buffer {
			return &closed
		}))
	)
	closeOnCleanup(t, r)
	if err := r.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if bool(closed) {
		t.Error("Close() should not close reader if never read from")
	}
}

func TestReaderReadAfterClose(t *testing.T) {
	r := NewReader(t.Context(), MachineFunc(func(
		context.Context, ...string,
	) Buffer {
		return strings.NewReader("data")
	}))
	closeOnCleanup(t, r)
	if _, err := r.Read(make([]byte, 1)); err != nil && err != io.EOF {
		t.Fatalf("Read() error = %v", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if _, err := r.Read(make([]byte, 1)); err != ErrClosed {
		t.Errorf("Read() after Close() error = %v, want ErrClosed", err)
	}
}

func TestReaderMultipleClose(t *testing.T) {
	r := NewReader(t.Context(), MachineFunc(func(
		context.Context, ...string,
	) Buffer {
		return strings.NewReader("data")
	}))
	closeOnCleanup(t, r)
	if _, err := r.Read(make([]byte, 1)); err != nil && err != io.EOF {
		t.Fatalf("Read() error = %v", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("First Close() error = %v", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

//ignore:errcheck
func TestReaderConcurrentReadClose(t *testing.T) {
	// Test concurrent Read() and Close() calls to detect race conditions.
	for range 1000 {
		r := NewReader(t.Context(), MachineFunc(func(
			context.Context, ...string,
		) Buffer {
			return strings.NewReader("data")
		}))
		closeOnCleanup(t, r)

		go r.Read(make([]byte, 1))
		go r.Close()
	}
}
