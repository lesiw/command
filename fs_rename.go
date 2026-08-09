package command

import (
	"context"

	"lesiw.io/fs"
)

var _ fs.RenameFS = (*cmdFS)(nil)

func (cfs *cmdFS) Rename(ctx context.Context, oldname, newname string) (err error) {
	if err = cfs.init(ctx); err != nil {
		return
	}

	switch cfs.kind {
	case kindGNU, kindBSD:
		err = Do(ctx, cfs, "mv", oldname, newname)
	case kindWindows, kindDOS:
		err = Do(ctx, cfs, "cmd", "/c", "move", "/Y", oldname, newname)
	default:
		err = errUnsupportedOS
	}

	return
}
