package command

import (
	"context"
	"io"

	"lesiw.io/fs"
	"lesiw.io/fs/path"
)

var _ fs.AppendFS = (*cmdFS)(nil)

func (cfs *cmdFS) Append(ctx context.Context, name string) (wc io.WriteCloser, err error) {
	if err = cfs.init(ctx); err != nil {
		return nil, err
	}
	if err = fs.MkdirAll(ctx, cfs, path.Dir(name)); err != nil {
		return nil, err
	}

	switch cfs.kind {
	case kindGNU, kindBSD:
		return NewWriter(ctx, cfs, "tee", "-a", name), nil
	case kindWindows:
		return psWriter(ctx, cfs, psScript(
			`$f = [System.IO.File]::Open('%s', 'Append')`,
			`[Console]::OpenStandardInput().CopyTo($f)`,
			`$f.Close()`,
		), name), nil
	case kindDOS:
		return dosWriter(ctx, cfs, "cmd", "/c", "more", ">>", name), nil
	default:
		return nil, errUnsupportedOS
	}
}
