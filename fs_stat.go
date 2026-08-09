package command

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"lesiw.io/fs"
	"lesiw.io/fs/path"
)

var _ fs.StatFS = (*cmdFS)(nil)

func (cfs *cmdFS) Stat(
	ctx context.Context, name string,
) (fs.FileInfo, error) {
	if err := cfs.init(ctx); err != nil {
		return nil, err
	}

	switch cfs.kind {
	case kindGNU:
		out, err := Read(ctx, cfs, "stat", "-c", "%f %s %Y %n", name)
		if err != nil {
			return nil, err
		}
		return parseGNUStat(out, name)
	case kindBSD:
		out, err := Read(ctx, cfs, "stat", "-f", "%p %z %m %N", name)
		if err != nil {
			return nil, err
		}
		return parseBSDStat(out, name)
	case kindWindows:
		out, err := psRead(ctx, cfs, psScript(
			`$f = Get-Item -Path "%s"`,
			`Write-Output "$($f.Mode) $($f.Length) `+
				`$($f.LastWriteTime.ToFileTime()) $($f.Name)"`,
		), name)
		if err != nil {
			return nil, err
		}
		return parseWindowsStat(out, name)
	default:
		return nil, errUnsupportedOS
	}
}

func parseGNUStat(out, name string) (fs.FileInfo, error) {
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) < 4 {
		return nil, fmt.Errorf("invalid stat output")
	}

	modeHex := fields[0]
	sizeStr := fields[1]
	mtimeStr := fields[2]

	modeInt, err := strconv.ParseUint(modeHex, 16, 32)
	if err != nil {
		return nil, err
	}
	mode := fs.Mode(modeInt & 0777)
	dir := (modeInt & 0x4000) != 0

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return nil, err
	}

	mtimeInt, err := strconv.ParseInt(mtimeStr, 10, 64)
	if err != nil {
		return nil, err
	}
	mtime := time.Unix(mtimeInt, 0)

	if dir {
		mode |= fs.ModeDir
	}

	return &fileInfo{
		name:  path.Base(name),
		size:  size,
		mode:  mode,
		mtime: mtime,
		dir:   dir,
	}, nil
}

func parseBSDStat(out, name string) (fs.FileInfo, error) {
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) < 4 {
		return nil, fmt.Errorf("invalid stat output")
	}

	modeOct := fields[0]
	sizeStr := fields[1]
	mtimeStr := fields[2]

	modeInt, err := strconv.ParseUint(modeOct, 8, 32)
	if err != nil {
		return nil, err
	}
	mode := fs.Mode(modeInt & 0777)
	dir := (modeInt & 040000) != 0

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return nil, err
	}

	mtimeInt, err := strconv.ParseInt(mtimeStr, 10, 64)
	if err != nil {
		return nil, err
	}
	mtime := time.Unix(mtimeInt, 0)

	if dir {
		mode |= fs.ModeDir
	}

	return &fileInfo{
		name:  path.Base(name),
		size:  size,
		mode:  mode,
		mtime: mtime,
		dir:   dir,
	}, nil
}

func parseWindowsStat(out, name string) (fs.FileInfo, error) {
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) < 4 {
		return nil, fmt.Errorf("invalid stat output")
	}

	modeStr := fields[0]
	sizeStr := fields[1]
	ftimeStr := fields[2]

	var mode fs.Mode
	var dir bool
	if strings.Contains(modeStr, "d") {
		dir = true
		mode = 0755 | fs.ModeDir
	} else {
		mode = 0644
	}

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return nil, err
	}

	ftimeInt, err := strconv.ParseInt(ftimeStr, 10, 64)
	if err != nil {
		return nil, err
	}
	const windowsEpochDiff = 116444736000000000
	unixNano := (ftimeInt - windowsEpochDiff) * 100
	mtime := time.Unix(0, unixNano)

	return &fileInfo{
		name:  path.Base(name),
		size:  size,
		mode:  mode,
		mtime: mtime,
		dir:   dir,
	}, nil
}
