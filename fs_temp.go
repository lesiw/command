package command

import (
	"cmp"
	"context"

	"lesiw.io/fs"
	"lesiw.io/fs/path"
)

var _ fs.TempFS = (*cmdFS)(nil)

func (cfs *cmdFS) Temp(ctx context.Context, prefix string) (string, error) {
	return cfs.temp(ctx, prefix, false)
}

var _ fs.TempDirFS = (*cmdFS)(nil)

func (cfs *cmdFS) TempDir(ctx context.Context, prefix string) (string, error) {
	return cfs.temp(ctx, prefix, true)
}

func (cfs *cmdFS) temp(ctx context.Context, prefix string, dir bool) (out string, err error) {
	if err := cfs.init(ctx); err != nil {
		return "", err
	}

	switch cfs.kind {
	case kindGNU:
		if dir {
			out, err = Read(ctx, cfs,
				"mktemp", "-d", "-t", cmp.Or(prefix, "tmp")+".XXXXXX",
			)
		} else {
			out, err = Read(
				ctx, cfs, "mktemp", "-t", cmp.Or(prefix, "tmp")+".XXXXXX",
			)
		}
	case kindBSD:
		if dir {
			out, err = Read(ctx, cfs,
				"mktemp", "-d", "-t", cmp.Or(prefix, "tmp"),
			)
		} else {
			out, err = Read(ctx, cfs, "mktemp", "-t", cmp.Or(prefix, "tmp"))
		}
	case kindWindows:
		if dir {
			out, err = psRead(ctx, cfs,
				psScript(
					"$p=[System.IO.Path]::Combine("+
						"[System.IO.Path]::GetTempPath(),'%s-'+"+
						"[System.IO.Path]::GetRandomFileName())",
					"New-Item -ItemType Directory -Path $p | "+
						"Select-Object -ExpandProperty FullName",
				),
				cmp.Or(prefix, "tmp"),
			)
		} else {
			out, err = psRead(ctx, cfs,
				"New-TemporaryFile | "+
					"Select-Object -ExpandProperty FullName",
			)
		}
	default:
		return "", errUnsupportedOS
	}

	if err != nil || out == "" {
		return "", fs.ErrUnsupported
	}

	// Make the path absolute if it isn't already.
	if !path.IsAbs(out) {
		return cfs.Abs(ctx, out)
	}
	return out, nil
}
