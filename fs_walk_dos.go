package command

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"iter"
	"regexp"
	"strconv"
	"strings"
	"time"

	"lesiw.io/fs"
)

type dosQueueItem struct {
	path  string
	level int
}

func (cfs *cmdFS) walkDOS(
	ctx context.Context, root string, depth int,
) iter.Seq2[fs.DirEntry, error] {
	return func(yield func(fs.DirEntry, error) bool) {
		queue := []dosQueueItem{{path: root, level: 1}}
		for len(queue) > 0 {
			item := queue[0]
			queue = queue[1:]

			if depth > 0 && item.level > depth {
				continue
			}

			dirs, ok := cfs.walkDOSDir(ctx, root, item, yield)
			if !ok {
				return
			}
			queue = append(queue, dirs...)
		}
	}
}

func (cfs *cmdFS) walkDOSDir(
	ctx context.Context,
	root string,
	item dosQueueItem,
	yield func(fs.DirEntry, error) bool,
) (dirs []dosQueueItem, ok bool) {
	_, err := Read(
		ctx, cfs, "cmd", "/c", "dir", "/ad", item.path,
	)
	if err != nil {
		// Not a directory - stat it and yield as a file.
		// But skip if this IS the root (we don't yield root itself).
		if item.path == root {
			return nil, true
		}

		info, statErr := fs.Stat(ctx, cfs, item.path)
		if statErr != nil {
			yield(nil, statErr)
			return nil, false
		}

		entry := &dirEntry{
			name: info.Name(),
			dir:  info.IsDir(),
			mode: info.Mode().Type(),
			info: &fileInfo{
				name:  info.Name(),
				size:  info.Size(),
				mode:  info.Mode(),
				mtime: info.ModTime(),
				dir:   info.IsDir(),
			},
			path: item.path,
		}
		if !yield(entry, nil) {
			return nil, false
		}
		return nil, true
	}

	r := NewReader(ctx, cfs, "cmd", "/c", "dir", "/a", item.path)
	defer r.Close()

	basePath := item.path
	if !strings.HasSuffix(basePath, "\\") &&
		!strings.HasSuffix(basePath, "/") {
		basePath += "\\"
	}

	for entry, err := range dosDirEntries(r, basePath) {
		if !yield(entry, err) {
			return nil, false
		}
		if err != nil {
			return nil, false
		}

		if entry.IsDir() {
			// Reconstruct full path for queueing
			fullPath := basePath + entry.Name()
			dirs = append(dirs, dosQueueItem{
				path:  fullPath,
				level: item.level + 1,
			})
		}
	}

	return dirs, true
}

var dosEntryPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)

func dosDirEntries(r io.Reader, basePath string) iter.Seq2[*dirEntry, error] {
	return func(yield func(*dirEntry, error) bool) {
		var (
			scanner = bufio.NewScanner(r)
			line    string
		)

		for scanner.Scan() {
			if line = scanner.Text(); !dosEntryPattern.MatchString(line) {
				continue
			}

			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}

			mtimeStr := strings.Join(fields[0:3], " ")
			sizeStr := fields[3]
			filename := strings.Join(fields[4:], " ")
			var dir bool
			var size int64

			if sizeStr == "<DIR>" {
				dir = true
			} else if strings.HasPrefix(sizeStr, "<") {
				continue
			} else {
				var err error
				size, err = strconv.ParseInt(
					strings.ReplaceAll(sizeStr, ",", ""), 10, 64,
				)
				if err != nil {
					yield(nil, fmt.Errorf("bad size: %w", err))
					return
				}
			}

			if filename == "." || filename == ".." {
				continue
			}
			if strings.HasPrefix(filename, "[") {
				continue
			}

			mtime, err := time.Parse("2006-01-02 03:04 PM", mtimeStr)
			if err != nil {
				yield(nil, fmt.Errorf("bad mtime: %w", err))
				return
			}

			var mode fs.Mode
			if dir {
				mode = 0755 | fs.ModeDir
			} else {
				mode = 0644
			}

			var fullPath string
			if basePath != "" {
				fullPath = basePath + filename
			}

			entry := &dirEntry{
				name: filename,
				dir:  dir,
				mode: fs.Mode(mode.Type()),
				info: &fileInfo{
					name:  filename,
					size:  size,
					mode:  mode,
					mtime: mtime,
					dir:   dir,
				},
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
