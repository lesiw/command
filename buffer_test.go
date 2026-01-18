package command

import "io"

// readStringer wraps an io.Reader and provides a custom String() method.
type readStringer struct {
	io.Reader
	string
}

func (r *readStringer) String() string { return r.string }

// namedRW is a string that implements io.ReadWriter and fmt.Stringer.
type namedRW string

func (s namedRW) String() string            { return string(s) }
func (namedRW) Read(p []byte) (int, error)  { return 0, io.EOF }
func (namedRW) Write(p []byte) (int, error) { return len(p), nil }

// closeTracker tracks whether Close() has been called.
type closeTracker bool

func (*closeTracker) Read(p []byte) (int, error)  { return 0, io.EOF }
func (*closeTracker) Write(p []byte) (int, error) { return len(p), nil }
func (c *closeTracker) Close() error {
	*c = true
	return nil
}
