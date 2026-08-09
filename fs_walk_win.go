package command

import (
	"bufio"
	"context"
	_ "embed"
	"fmt"
	"io"
	"iter"
	"strconv"
	"strings"
	"time"

	"lesiw.io/fs"
)

//go:embed walk.ps1
var psWalkScript string

func (cfs *cmdFS) walkWindows(
	ctx context.Context, root string, depth int,
) iter.Seq2[fs.DirEntry, error] {
	return func(yield func(fs.DirEntry, error) bool) {
		var (
			script   = strings.ReplaceAll(psWalkScript, "{PATH}", root)
			depthStr = "unlimited"
		)
		if depth > 0 {
			depthStr = strconv.Itoa(depth - 1)
		}
		script = strings.ReplaceAll(script, "{DEPTH}", depthStr)

		r := psReader(ctx, cfs, "%v", script)
		defer r.Close()

		for entry, err := range windowsWalkEntries(r) {
			if err != nil {
				yield(nil, err)
				return
			}
			// Skip root directory itself
			if entry.path == root {
				continue
			}
			if !yield(entry, nil) {
				return
			}
		}
	}
}

func windowsWalkEntries(r io.Reader) iter.Seq2[*dirEntry, error] {
	return func(yield func(*dirEntry, error) bool) {
		scanner := bufio.NewScanner(r)
		scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
			if atEOF && len(data) == 0 {
				return 0, nil, nil
			}
			if i := strings.IndexByte(string(data), '\x1E'); i >= 0 {
				return i + 1, data[0:i], nil
			}
			if atEOF {
				return len(data), data, nil
			}
			return 0, nil, nil
		})
		var record string

		for scanner.Scan() {
			if record = scanner.Text(); record == "" {
				continue
			}

			fields := strings.Split(record, "\x1F")
			if len(fields) != 4 {
				continue
			}

			name := fields[0]
			sizeOrType := fields[1]
			mtimeStr := fields[2]
			fullPath := fields[3]
			dir := sizeOrType == "DIR"
			var size int64

			if name == "." || name == ".." {
				continue
			}

			var err error
			if !dir {
				size, err = strconv.ParseInt(sizeOrType, 10, 64)
				if err != nil {
					yield(nil, fmt.Errorf("bad size: %w", err))
					return
				}
			}

			var mode fs.Mode
			if dir {
				mode = 0755 | fs.ModeDir
			} else {
				mode = 0644
			}

			mtime, err := time.Parse("2006-01-02T15:04:05Z", mtimeStr)
			if err != nil {
				yield(nil, fmt.Errorf("bad mtime: %w", err))
				return
			}

			info := &fileInfo{
				name:  name,
				size:  size,
				mode:  mode,
				mtime: mtime,
				dir:   dir,
			}

			entry := &dirEntry{
				name: name,
				dir:  dir,
				mode: fs.Mode(mode.Type()),
				info: info,
				path: fullPath,
			}

			if !yield(entry, nil) {
				return
			}
		}

		if err := scanner.Err(); err != nil {
			yield(nil, err)
			return
		}
	}
}
