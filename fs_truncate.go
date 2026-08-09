package command

import (
	"context"
	"strconv"

	"lesiw.io/fs"
)

var _ fs.TruncateFS = (*cmdFS)(nil)

func (cfs *cmdFS) Truncate(ctx context.Context, name string, size int64) (err error) {
	if err = cfs.init(ctx); err != nil {
		return
	}

	sz := strconv.FormatInt(size, 10)
	switch cfs.kind {
	case kindGNU:
		err = Do(ctx, cfs, "truncate", "-s", sz, name)
	case kindBSD:
		err = fs.ErrUnsupported
	case kindWindows, kindDOS:
		err = Do(ctx, cfs, "fsutil", "file", "seteof", name, sz)
	default:
		err = errUnsupportedOS
	}

	return
}
