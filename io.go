package command

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"lesiw.io/prefix"
)

var (
	Trace   = io.Discard
	ShTrace = prefix.NewWriter("+ ", stderr)

	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

// Copy copies the output of each filter into the input of the next filter.
//
// Copy uses io.Copy internally, which automatically optimizes for
// io.ReaderFrom and io.WriterTo implementations. When using NewWriter(),
// its io.ReaderFrom implementation will automatically close stdin after
// copying.
//
// The fil stages must be both readable and writable (io.ReadWriter). Use
// NewFilter() to wrap Buffer instances for use in pipelines.
func Copy(
	dst io.Writer, src io.Reader, fil ...io.ReadWriter,
) (written int64, err error) {
	var (
		g errgroup.Group
		r io.Reader
		w io.Writer

		count = make(chan int64)
		total = make(chan int64)
	)

	results := &copyError{results: make([]copyResult, len(fil)+1)}

	go func() {
		var written int64
		for n := range count {
			written += n
		}
		total <- written
	}()

	for i := -1; i < len(fil); i++ {
		if i < 0 {
			r = src
		} else {
			r = fil[i]
		}
		if i == len(fil)-1 {
			w = dst
		} else {
			w = fil[i+1]
		}
		i := i
		w := w
		r := r
		g.Go(func() (err error) {
			defer func() {
				// Close the writer after copying completes.
				// This is critical for pipelines using io.Pipe() or similar
				// constructs, where the next stage's reader won't get EOF
				// until the writer closes.
				var closeErr error
				if c, ok := w.(io.Closer); ok {
					closeErr = c.Close()
				}
				results.set(i+1, copyResult{
					cmd: cmdString(r),
					err: errors.Join(err, closeErr),
				})
			}()
			// io.Copy automatically uses ReaderFrom/WriterTo optimizations.
			// When w implements io.ReaderFrom (like NewWriter), it will
			// auto-close stdin after the copy completes.
			n, err := io.Copy(w, r)
			if err == nil {
				count <- n
			}
			return err
		})
	}
	err = g.Wait()
	close(count)

	// If any stage errored, return combined error with all results.
	if err != nil {
		err = results
	}

	return <-total, err
}

type copyResult struct {
	cmd string
	err error
}

type copyError struct {
	sync.Mutex
	results []copyResult
}

func (e *copyError) set(offset int, result copyResult) {
	e.Lock()
	defer e.Unlock()
	e.results[offset] = result
}

func (e *copyError) Error() string {
	e.Lock()
	defer e.Unlock()

	var parts []string
	for _, result := range e.results {
		part := result.cmd
		if result.err != nil {
			errStr := result.err.Error()
			errStr = strings.ReplaceAll(errStr, "\n", "\n\t")
			part += "\n\t" + errStr
		} else {
			part += "\n\t<success>"
		}
		parts = append(parts, part)
	}

	return strings.Join(parts, "\n\n")
}

func (e *copyError) Unwrap() []error {
	e.Lock()
	defer e.Unlock()

	var errs []error
	for _, result := range e.results {
		if result.err != nil {
			errs = append(errs, result.err)
		}
	}
	return errs
}

func cmdString(v any) string {
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	return fmt.Sprintf("<%T>", v)
}

// ReadAll reads r through filters f, returning the data it read as a string
// with trailing newlines stripped.
func ReadAll(r io.Reader, f ...io.ReadWriter) (string, error) {
	var buf strings.Builder
	if _, err := Copy(&buf, r, f...); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\r\n"), nil
}
