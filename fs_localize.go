package command

import (
	"context"
	"strings"

	"lesiw.io/fs"
)

var _ fs.LocalizeFS = (*cmdFS)(nil)

func (cfs *cmdFS) Localize(ctx context.Context, name string) (result string, err error) {
	if err = cfs.init(ctx); err != nil {
		return "", err
	}
	return localize(ctx, cfs.kind, name)
}

func localize(_ context.Context, kind cfsKind, name string) (string, error) {
	switch kind {
	case kindGNU, kindBSD:
		// Unix paths are already in the correct format.
		return name, nil
	case kindWindows, kindDOS:
		// Early exit: path already looks localized (contains backslash).
		if strings.Contains(name, "\\") {
			return name, nil
		}
		// Convert forward slashes to backslashes.
		return strings.ReplaceAll(name, "/", "\\"), nil
	default:
		return "", errUnsupportedOS
	}
}
