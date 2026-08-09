package command

import (
	"context"
	"iter"

	"lesiw.io/fs"
)

var _ fs.WalkFS = (*cmdFS)(nil)

func (cfs *cmdFS) Walk(ctx context.Context, root string, depth int) iter.Seq2[fs.DirEntry, error] {
	if err := cfs.init(ctx); err != nil {
		return func(yield func(fs.DirEntry, error) bool) {
			yield(nil, err)
		}
	}

	switch cfs.kind {
	case kindGNU, kindBSD:
		return cfs.walkPOSIX(ctx, root, depth)
	case kindWindows:
		return cfs.walkWindows(ctx, root, depth)
	case kindDOS:
		return cfs.walkDOS(ctx, root, depth)
	default:
		return func(yield func(fs.DirEntry, error) bool) {
			yield(nil, errUnsupportedOS)
		}
	}
}
