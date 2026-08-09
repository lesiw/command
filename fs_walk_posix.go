package command

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"iter"
	"strconv"
	"strings"
	"time"

	"lesiw.io/fs"
	"lesiw.io/fs/path"
)

func (cfs *cmdFS) walkPOSIX(ctx context.Context, root string, depth int) iter.Seq2[fs.DirEntry, error] {
	return func(yield func(fs.DirEntry, error) bool) {
		args := []string{"find", root}
		if depth > 0 {
			args = append(args, "-maxdepth", strconv.Itoa(depth))
		}

		args = append(args, "-exec", "ls", "-ld", "{}", "+")
		r := NewReader(ctx, cfs, args...)
		defer r.Close()

		for entry, err := range posixWalkEntries(r) {
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

func posixWalkEntries(r io.Reader) iter.Seq2[*dirEntry, error] {
	return func(yield func(*dirEntry, error) bool) {
		var (
			scanner = bufio.NewScanner(r)
			line    string
		)

		for scanner.Scan() {
			if line = strings.TrimSpace(scanner.Text()); line == "" {
				continue
			}

			fields := strings.Fields(line)
			if len(fields) < 9 {
				continue
			}

			permissions := fields[0]
			sizeStr := fields[4]
			timeStr := strings.Join(fields[5:8], " ")
			fullPath := strings.Join(fields[8:], " ")
			baseName := path.Base(fullPath)

			if baseName == "." || baseName == ".." {
				continue
			}

			dir := strings.HasPrefix(permissions, "d")
			var mode fs.Mode
			if dir {
				mode = 0755 | fs.ModeDir
			} else {
				mode = 0644
			}

			size, err := strconv.ParseInt(sizeStr, 10, 64)
			if err != nil {
				yield(nil, fmt.Errorf("bad size: %w", err))
				return
			}

			var mtime time.Time
			mtime, err = time.Parse("Jan 2 15:04", timeStr)
			if err != nil {
				mtime, err = time.Parse("Jan 2 2006", timeStr)
				if err != nil {
					yield(nil, fmt.Errorf("bad mtime: %w", err))
					return
				}
			} else {
				mtime = time.Date(
					time.Now().Year(), mtime.Month(), mtime.Day(),
					mtime.Hour(), mtime.Minute(), 0, 0, time.UTC,
				)
			}

			info := &fileInfo{
				name:  baseName,
				size:  size,
				mode:  mode,
				mtime: mtime,
				dir:   dir,
			}

			entry := &dirEntry{
				name: baseName,
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
