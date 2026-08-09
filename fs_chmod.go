package command

import (
	"context"
	"fmt"

	"lesiw.io/fs"
)

var _ fs.ChmodFS = (*cmdFS)(nil)

func (cfs *cmdFS) Chmod(ctx context.Context, name string, mode fs.Mode) (err error) {
	if err = cfs.init(ctx); err != nil {
		return
	}

	switch cfs.kind {
	case kindGNU, kindBSD:
		err = Do(ctx, cfs, "chmod", fmt.Sprintf("%04o", mode&0777), name)
	case kindWindows, kindDOS:
		err = fs.ErrUnsupported
	default:
		err = errUnsupportedOS
	}

	return
}
