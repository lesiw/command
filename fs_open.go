package command

import (
	"context"
	"io"

	"lesiw.io/fs"
)

var _ fs.FS = (*cmdFS)(nil)

func (cfs *cmdFS) Open(ctx context.Context, name string) (rc io.ReadCloser, err error) {
	if err = cfs.init(ctx); err != nil {
		return nil, err
	}

	switch cfs.kind {
	case kindGNU, kindBSD:
		return NewReader(ctx, cfs, "cat", name), nil
	case kindWindows:
		return psReader(ctx, cfs, psScript(
			`$f = [System.IO.File]::OpenRead('%s')`,
			`$f.CopyTo([Console]::OpenStandardOutput())`,
			`$f.Close()`,
		), name), nil
	case kindDOS:
		return dosReader(ctx, cfs, "cmd", "/c", "type", name), nil
	default:
		return nil, errUnsupportedOS
	}
}
