package command

import (
	"context"
	"io"
)

// NewFilter creates a bidirectional command filter with full
// Read/Write/Close access.
//
// The returned io.ReadWriteCloser provides direct access to the command's
// stdin (Write), stdout (Read), and stdin close signal (Close).
//
// If the underlying command does not support writing (is read-only), Write()
// will return an error. Close() closes stdin if supported, otherwise it is
// a no-op.
//
// NewFilter is primarily useful with command.Copy for pipeline composition.
// For most use cases, prefer NewReader (read-only with cancellation) or
// NewWriter (write-only with completion wait).
func NewFilter(
	ctx context.Context, m Machine, args ...string,
) io.ReadWriteCloser {
	buf := m.Command(ctx, args...)
	return &filter{buf: buf}
}

// Deprecated: Use NewFilter instead.
var NewStream = NewFilter

type filter struct {
	buf Buffer
}

func (f *filter) Read(p []byte) (int, error) {
	return f.buf.Read(p)
}

func (f *filter) Write(p []byte) (int, error) {
	if wb, ok := f.buf.(WriteBuffer); ok {
		return wb.Write(p)
	}
	return 0, ErrReadOnly
}

func (f *filter) Close() error {
	if wb, ok := f.buf.(WriteBuffer); ok {
		return wb.Close()
	}
	// Read-only commands - Close is a no-op
	return nil
}

// ReadFrom implements io.ReaderFrom for optimized copying that auto-closes
// stdin when the source reaches EOF.
// This allows io.Copy to automatically close stdin in pipeline stages.
func (f *filter) ReadFrom(src io.Reader) (n int64, err error) {
	wb, ok := f.buf.(WriteBuffer)
	if !ok {
		return 0, ErrReadOnly
	}

	// Copy from source to command stdin
	n, err = io.Copy(wb, src)

	// Auto-close stdin after copy completes
	closeErr := wb.Close()
	if err == nil {
		err = closeErr
	}

	return n, err
}
