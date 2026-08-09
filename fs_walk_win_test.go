package command

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"lesiw.io/fs"
)

func windowsWalkInput(records [][]string) string {
	var result []string
	for _, fields := range records {
		result = append(result, strings.Join(fields, "\x1F"))
	}
	return strings.Join(result, "\x1E") + "\x1E"
}

var windowsWalkEntriesTests = []struct {
	name  string
	input string
	want  []dirEntry
}{{
	name: "mixed files and directories",
	input: windowsWalkInput([][]string{
		{"file1.txt", "50", "2024-11-21T10:30:00Z",
			`C:\test\file1.txt`}, {"subdir", "DIR", "2025-01-15T08:45:00Z",
			`C:\test\subdir`}, {"file2.txt", "100", "2023-12-31T23:59:59Z",
			`C:\test\file2.txt`}}),
	want: []dirEntry{{
		name: "file1.txt",
		dir:  false,
		mode: 0,
		info: &fileInfo{
			name:  "file1.txt",
			size:  50,
			mode:  0644,
			mtime: time.Date(2024, 11, 21, 10, 30, 0, 0, time.UTC),
			dir:   false,
		},
		path: `C:\test\file1.txt`,
	}, {
		name: "subdir",
		dir:  true,
		mode: fs.ModeDir,
		info: &fileInfo{
			name:  "subdir",
			size:  0,
			mode:  0755 | fs.ModeDir,
			mtime: time.Date(2025, 1, 15, 8, 45, 0, 0, time.UTC),
			dir:   true,
		},
		path: `C:\test\subdir`,
	}, {
		name: "file2.txt",
		dir:  false,
		mode: 0,
		info: &fileInfo{
			name:  "file2.txt",
			size:  100,
			mode:  0644,
			mtime: time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
			dir:   false,
		},
		path: `C:\test\file2.txt`,
	}},
}, {
	name: "only files",
	input: windowsWalkInput([][]string{
		{"test.txt", "100", "2024-03-10T00:00:00Z",
			`C:\test\test.txt`}, {"data.json", "200", "2024-04-20T00:00:00Z",
			`C:\test\data.json`}}),
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
		path: `C:\test\test.txt`,
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
		path: `C:\test\data.json`,
	}},
}, {
	name:  "empty result",
	input: "",
	want:  nil,
}, {
	name: "filename with spaces",
	input: windowsWalkInput([][]string{
		{"file with spaces.txt", "50", "2024-05-05T00:00:00Z",
			`C:\test\file with spaces.txt`}}),
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
		path: `C:\test\file with spaces.txt`,
	}},
}, {
	name: "single-digit days",
	input: windowsWalkInput([][]string{
		{"epoch.txt", "100", "1970-01-01T00:00:00Z",
			`C:\test\epoch.txt`}, {"recent.txt", "200", "2009-02-02T15:04:00Z",
			`C:\test\recent.txt`}, {"testdir", "DIR", "2024-03-09T00:00:00Z",
			`C:\test\testdir`}}),
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
		path: `C:\test\epoch.txt`,
	}, {
		name: "recent.txt",
		dir:  false,
		mode: 0,
		info: &fileInfo{
			name:  "recent.txt",
			size:  200,
			mode:  0644,
			mtime: time.Date(2009, 2, 2, 15, 4, 0, 0, time.UTC),
			dir:   false,
		},
		path: `C:\test\recent.txt`,
	}, {
		name: "testdir",
		dir:  true,
		mode: fs.ModeDir,
		info: &fileInfo{
			name:  "testdir",
			size:  0,
			mode:  0755 | fs.ModeDir,
			mtime: time.Date(2024, 3, 9, 0, 0, 0, 0, time.UTC),
			dir:   true,
		},
		path: `C:\test\testdir`,
	}},
}}

func TestParseWindowsWalkEntries(t *testing.T) {
	for _, tt := range windowsWalkEntriesTests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				r   = strings.NewReader(tt.input)
				got []dirEntry
			)
			for entry, err := range windowsWalkEntries(r) {
				if err != nil {
					t.Fatalf("parser error = %v", err)
				}
				got = append(got, toDirEntry(entry))
			}

			opts := cmp.AllowUnexported(dirEntry{}, fileInfo{})
			if !cmp.Equal(tt.want, got, opts) {
				t.Errorf(
					"entries mismatch (-want +got):\n%s",
					cmp.Diff(tt.want, got, opts),
				)
			}
		})
	}
}
