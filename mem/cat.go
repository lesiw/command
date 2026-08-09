package mem

import (
	"bytes"
	"context"
	"io"

	"lesiw.io/command"
	"lesiw.io/command/internal/sh"
	"lesiw.io/fs"
)

type nopClose struct{}

func (nopClose) Close() error { return nil }

func catCommand(ctx context.Context, m *machine, args ...string) command.Buffer {
	// No args: read from stdin.
	if len(args) == 1 {
		return struct {
			*bytes.Buffer
			nopClose
			sh.Stringer
		}{
			Buffer:   new(bytes.Buffer),
			Stringer: sh.String(command.Envs(ctx), args...),
		}
	}

	// Args: open and concatenate files.
	readers := make([]io.Reader, 0, len(args)-1)
	for _, path := range args[1:] {
		fr, err := fs.Open(ctx, m.FS(), path)
		if err != nil {
			return command.Fail(&command.Error{Code: 1, Err: err})
		}
		readers = append(readers, fr)
	}

	return struct {
		io.Reader
		sh.Stringer
	}{
		Reader:   io.MultiReader(readers...),
		Stringer: sh.String(command.Envs(ctx), args...),
	}
}
