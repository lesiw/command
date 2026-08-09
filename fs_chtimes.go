package command

import (
	"context"
	"errors"
	"time"

	"lesiw.io/fs"
)

var _ fs.ChtimesFS = (*cmdFS)(nil)

func (cfs *cmdFS) Chtimes(ctx context.Context, name string, atime, mtime time.Time) (err error) {
	if atime.IsZero() && mtime.IsZero() {
		return nil
	}
	if err = cfs.init(ctx); err != nil {
		return
	}

	switch cfs.kind {
	case kindGNU, kindBSD:
		if !mtime.IsZero() {
			err = errors.Join(err, Do(ctx, cfs,
				"touch", "-t", mtime.Format("200601021504.05"), name,
			))
		}
		if !atime.IsZero() {
			err = errors.Join(err, Do(ctx, cfs,
				"touch", "-a", "-t", atime.Format("200601021504.05"), name,
			))
		}
	case kindWindows:
		if !atime.IsZero() && !mtime.IsZero() {
			err = psDo(ctx, cfs, psScript(
				`$f = Get-Item "%s"`,
				`$f.LastAccessTime = [DateTime]::Parse("%s")`,
				`$f.LastWriteTime = [DateTime]::Parse("%s")`,
			), name, atime.Format(time.RFC3339), mtime.Format(time.RFC3339))
		} else if !atime.IsZero() {
			err = psDo(ctx, cfs, psScript(
				`$f = Get-Item "%s"`,
				`$f.LastAccessTime = [DateTime]::Parse("%s")`,
			), name, atime.Format(time.RFC3339))
		} else {
			err = psDo(ctx, cfs, psScript(
				`$f = Get-Item "%s"`,
				`$f.LastWriteTime = [DateTime]::Parse("%s")`,
			), name, mtime.Format(time.RFC3339))
		}
	case kindDOS:
		err = fs.ErrUnsupported
	default:
		err = errUnsupportedOS
	}

	return
}
