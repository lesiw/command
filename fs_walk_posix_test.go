package command

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"lesiw.io/fs"
)

var posixReadDirTests = []struct {
	name  string
	input string
	want  []dirEntry
}{{
	name: "mixed files and directories",
	input: strings.TrimSpace(`
drwxr-xr-x  5 user  group  160 Nov 21 10:00 .
drwxr-xr-x 10 user  group  320 Nov 21 09:00 ..
-rw-r--r--  1 user  group    3 Nov 21 2024 file1.txt
-rw-r--r--  1 user  group    3 Dec 31 2023 file2.txt
drwxr-xr-x  2 user  group   64 Jan 15 2025 subdir
`),
	want: []dirEntry{{
		name: "file1.txt",
		dir:  false,
		mode: 0,
		info: &fileInfo{
			name:  "file1.txt",
			size:  3,
			mode:  0644,
			mtime: time.Date(2024, 11, 21, 0, 0, 0, 0, time.UTC),
			dir:   false,
		},
		path: "file1.txt",
	}, {
		name: "file2.txt",
		dir:  false,
		mode: 0,
		info: &fileInfo{
			name:  "file2.txt",
			size:  3,
			mode:  0644,
			mtime: time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
			dir:   false,
		},
		path: "file2.txt",
	}, {
		name: "subdir",
		dir:  true,
		mode: fs.ModeDir,
		info: &fileInfo{
			name:  "subdir",
			size:  64,
			mode:  0755 | fs.ModeDir,
			mtime: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			dir:   true,
		},
		path: "subdir",
	}},
}, {
	name: "only files",
	input: strings.TrimSpace(`
-rw-r--r--  1 user  group  100 Mar 10 2024 test.txt
-rw-r--r--  1 user  group  200 Apr 20 2024 data.json
`),
	want: []dirEntry{{
		name: "test.txt",
		dir:  false,
		mode: 0,
		info: &fileInfo{
			name:  "test.txt",
			size:  100,
			mode:  0644,
			mtime: time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC),
			dir:   false,
		},
		path: "test.txt",
	}, {
		name: "data.json",
		dir:  false,
		mode: 0,
		info: &fileInfo{
			name:  "data.json",
			size:  200,
			mode:  0644,
			mtime: time.Date(2024, 4, 20, 0, 0, 0, 0, time.UTC),
			dir:   false,
		},
		path: "data.json",
	}},
}, {
	name:  "empty directory",
	input: ``,
	want:  nil,
}, {
	name: "filename with spaces",
	input: strings.TrimSpace(`
-rw-r--r--  1 user  group  50 May 05 2024 file with spaces.txt
`),
	want: []dirEntry{{
		name: "file with spaces.txt",
		dir:  false,
		mode: 0,
		info: &fileInfo{
			name:  "file with spaces.txt",
			size:  50,
			mode:  0644,
			mtime: time.Date(2024, 5, 5, 0, 0, 0, 0, time.UTC),
			dir:   false,
		},
		path: "file with spaces.txt",
	}},
}, {
	name: "single-digit days",
	input: strings.TrimSpace(`
-rw-r--r--  1 user  group  100 Jan 1 1970 epoch.txt
-rw-r--r--  1 user  group  200 Feb 2 2009 recent.txt
drwxr-xr-x  2 user  group   64 Mar 9 2024 testdir
`),
	want: []dirEntry{{
		name: "epoch.txt",
		dir:  false,
		mode: 0,
		info: &fileInfo{
			name:  "epoch.txt",
			size:  100,
			mode:  0644,
			mtime: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
			dir:   false,
		},
		path: "epoch.txt",
	}, {
		name: "recent.txt",
		dir:  false,
		mode: 0,
		info: &fileInfo{
			name:  "recent.txt",
			size:  200,
			mode:  0644,
			mtime: time.Date(2009, 2, 2, 0, 0, 0, 0, time.UTC),
			dir:   false,
		},
		path: "recent.txt",
	}, {
		name: "testdir",
		dir:  true,
		mode: fs.ModeDir,
		info: &fileInfo{
			name:  "testdir",
			size:  64,
			mode:  0755 | fs.ModeDir,
			mtime: time.Date(2024, 3, 9, 0, 0, 0, 0, time.UTC),
			dir:   true,
		},
		path: "testdir",
	}},
}}

func TestParsePOSIXReadDir(t *testing.T) {
	opts := cmp.AllowUnexported(dirEntry{}, fileInfo{})
	for _, tt := range posixReadDirTests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				r   = strings.NewReader(tt.input)
				got []dirEntry
			)
			for entry, err := range posixWalkEntries(r) {
				if err != nil {
					t.Fatalf("posixWalkEntries() error = %v", err)
				}
				got = append(got, toDirEntry(entry))
			}
			if !cmp.Equal(tt.want, got, opts) {
				t.Errorf(
					"entries mismatch (-want +got):\n%s",
					cmp.Diff(tt.want, got, opts),
				)
			}
		})
	}
}
