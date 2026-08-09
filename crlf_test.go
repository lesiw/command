package command

import (
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

func TestCRLFReaderWithCRLF(t *testing.T) {
	r := crlfReader(io.NopCloser(strings.NewReader("line1\r\nline2\r\n")))
	if got, err := io.ReadAll(r); err != nil {
		t.Errorf("ReadAll() error = %v", err)
	} else if want := "line1\nline2\n"; string(got) != want {
		t.Errorf("ReadAll() = %q, want %q", got, want)
	}
}

func TestCRLFReaderWithCR(t *testing.T) {
	r := crlfReader(io.NopCloser(strings.NewReader("line1\rline2\r")))
	if got, err := io.ReadAll(r); err != nil {
		t.Errorf("ReadAll() error = %v", err)
	} else if want := "line1\nline2\n"; string(got) != want {
		t.Errorf("ReadAll() = %q, want %q", got, want)
	}
}

func TestCRLFReaderWithLF(t *testing.T) {
	r := crlfReader(io.NopCloser(strings.NewReader("line1\nline2\n")))
	if got, err := io.ReadAll(r); err != nil {
		t.Errorf("ReadAll() error = %v", err)
	} else if want := "line1\nline2\n"; string(got) != want {
		t.Errorf("ReadAll() = %q, want %q", got, want)
	}
}

func TestCRLFReaderWithMixed(t *testing.T) {
	r := crlfReader(io.NopCloser(strings.NewReader("unix\nwin\r\nmac\rend")))
	if got, err := io.ReadAll(r); err != nil {
		t.Errorf("ReadAll() error = %v", err)
	} else if want := "unix\nwin\nmac\nend"; string(got) != want {
		t.Errorf("ReadAll() = %q, want %q", got, want)
	}
}

func TestCRLFReaderWithSplitCRLF(t *testing.T) {
	var (
		r       = iotest.OneByteReader(strings.NewReader("test\r\ndata"))
		wrapped = crlfReader(io.NopCloser(r))
	)
	if got, err := io.ReadAll(wrapped); err != nil {
		t.Errorf("ReadAll() error = %v", err)
	} else if want := "test\ndata"; string(got) != want {
		t.Errorf("ReadAll() = %q, want %q", got, want)
	}
}

func TestCRLFReaderClose(t *testing.T) {
	var (
		closed  closeTracker
		wrapped = crlfReader(&closed)
	)
	if err := wrapped.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if !closed {
		t.Error("Close() did not call underlying Close()")
	}
}
