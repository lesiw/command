package command

import (
	"context"
	"fmt"

	"lesiw.io/fs"
)

var _ fs.ChownFS = (*cmdFS)(nil)

func (cfs *cmdFS) Chown(ctx context.Context, name string, uid, gid int) (err error) {
	if max(uid, gid) < 0 {
		return nil
	}
	if err = cfs.init(ctx); err != nil {
		return
	}

	switch cfs.kind {
	case kindGNU, kindBSD:
		var owner string
		if uid == -1 {
			owner = fmt.Sprintf(":%d", gid)
		} else if gid == -1 {
			owner = fmt.Sprintf("%d", uid)
		} else {
			owner = fmt.Sprintf("%d:%d", uid, gid)
		}
		err = Do(ctx, cfs, "chown", owner, name)
	case kindWindows, kindDOS:
		err = fs.ErrUnsupported
	default:
		err = errUnsupportedOS
	}

	return
}
