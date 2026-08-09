package command

import (
	"context"
	"io"

	"lesiw.io/fs"
	"lesiw.io/fs/path"
)

var _ fs.CreateFS = (*cmdFS)(nil)

func (cfs *cmdFS) Create(ctx context.Context, name string) (wc io.WriteCloser, err error) {
	if err = cfs.init(ctx); err != nil {
		return nil, err
	}
	if err = fs.MkdirAll(ctx, cfs, path.Dir(name)); err != nil {
		return nil, err
	}

	switch cfs.kind {
	case kindGNU, kindBSD:
		wc = NewWriter(ctx, cfs, "tee", name)
	case kindWindows:
		wc = psWriter(ctx, cfs, psScript(
			`$f = [System.IO.File]::Create('%s')`,
			`[Console]::OpenStandardInput().CopyTo($f)`,
			`$f.Close()`,
		), name)
	case kindDOS:
		wc = dosWriter(ctx, cfs, "cmd", "/c", "more", ">", name)
	default:
		return nil, errUnsupportedOS
	}

	// Perform a no-op write to force file creation or truncation
	// even if the stream is immediately Closed.
	if _, err = wc.Write(nil); err != nil {
		return nil, err
	}

	return wc, nil
}
