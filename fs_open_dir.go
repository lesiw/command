package command

import (
	"context"
	"io"

	"lesiw.io/fs"
)

var _ fs.DirFS = (*cmdFS)(nil)

func (cfs *cmdFS) OpenDir(ctx context.Context, dir string) (io.ReadCloser, error) {
	if err := cfs.init(ctx); err != nil {
		return nil, err
	}
	if !cfs.hasTar {
		return nil, fs.ErrUnsupported
	}

	return NewReader(ctx, cfs, "tar", "-cf-", "-C", dir, "."), nil
}
