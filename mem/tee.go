package mem

import (
	"context"
	"io"
	"sync"

	"lesiw.io/command"
	"lesiw.io/command/internal/sh"
	"lesiw.io/fs"
)

type teeReader struct {
	io.Reader
	once  sync.Once
	files []io.Closer
}

func (t *teeReader) Read(p []byte) (n int, err error) {
	n, err = t.Reader.Read(p)
	if err == io.EOF {
		t.once.Do(func() {
			for _, f := range t.files {
				_ = f.Close()
			}
		})
	}
	return n, err
}

func teeCommand(ctx context.Context, m *machine, args ...string) command.Buffer {
	// Open all output files
	var (
		writers = make([]io.Writer, 0, len(args)-1)
		closers = make([]io.Closer, 0, len(args)-1)
	)
	for _, path := range args[1:] {
		fw, err := fs.Create(ctx, m.FS(), path)
		if err != nil {
			// Close any already-opened files
			for _, c := range closers {
				_ = c.Close()
			}
			return command.Fail(&command.Error{Code: 1, Err: err})
		}
		writers = append(writers, fw)
		closers = append(closers, fw)
	}

	pr, pw := io.Pipe()

	return struct {
		*teeReader
		*io.PipeWriter
		sh.Stringer
	}{
		teeReader: &teeReader{
			Reader: io.TeeReader(pr, io.MultiWriter(writers...)),
			files:  closers,
		},
		PipeWriter: pw,
		Stringer:   sh.String(command.Envs(ctx), args...),
	}
}
