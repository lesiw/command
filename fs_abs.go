package command

import (
	"context"

	"lesiw.io/fs"
	"lesiw.io/fs/path"
)

var _ fs.AbsFS = (*cmdFS)(nil)

func (cfs *cmdFS) Abs(ctx context.Context, name string) (string, error) {
	if err := cfs.init(ctx); err != nil {
		return "", err
	}
	return abs(ctx, cfs.Machine, cfs.kind, name)
}

func abs(
	ctx context.Context, m Machine, kind cfsKind, name string,
) (string, error) {
	dir := path.IsDir(name) // Remember if input is a directory.

	// Join with WorkDir if provided and path is relative.
	if workDir := fs.WorkDir(ctx); workDir != "" && !path.IsAbs(name) {
		name = path.Join(workDir, name)
		ctx = fs.WithWorkDir(ctx, "")
	}

	// Localize the path (lexical operation).
	name, err := localize(ctx, kind, name)
	if err != nil {
		return "", err
	}

	// Then, make it absolute (filesystem operation).
	switch kind {
	case kindGNU, kindBSD:
		real, err := Read(ctx, m, "realpath", name)
		if err != nil {
			return name, nil
		} else if real == "" && dir {
			return "./", nil
		} else if real == "" {
			return ".", nil
		} else if dir {
			return real + "/", nil
		} else {
			return real, nil
		}
	case kindWindows:
		real, err := psRead(ctx, m,
			"(Resolve-Path -Path '%s' -ErrorAction Stop).Path", name)
		if err != nil {
			return name, nil
		} else if real == "" && dir {
			return `.\`, nil
		} else if real == "" {
			return ".", nil
		} else {
			return real, nil
		}
	case kindDOS: // No realpath equivalent, best effort.
		return name, nil
	default:
		return "", errUnsupportedOS
	}
}
