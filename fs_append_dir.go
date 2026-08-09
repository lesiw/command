package command

import (
	"context"
	"io"

	"lesiw.io/fs"
)

var _ fs.AppendDirFS = (*cmdFS)(nil)

func (cfs *cmdFS) AppendDir(ctx context.Context, dir string) (io.WriteCloser, error) {
	if err := cfs.init(ctx); err != nil {
		return nil, err
	}
	if !cfs.hasTar {
		return nil, fs.ErrUnsupported
	}
	if err := fs.MkdirAll(ctx, cfs, dir); err != nil {
		return nil, err
	}

	return NewWriter(ctx, cfs, "tar", "-xf-", "-C", dir), nil
}
