package command

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"lesiw.io/fs"
)

var dosReadDirTests = []struct {
	name  string
	input string
	want  []dirEntry
}{{
	name: "mixed files and directories",
	input: strings.TrimSpace(`
 Volume in drive C has no label.
 Volume Serial Number is 1234-5678

 Directory of C:\test

2025-11-22  06:11 PM    <DIR>          .
2025-11-22  06:11 PM    <DIR>          ..
2021-12-31  09:44 AM                54 .bash_history
2025-11-22  06:11 PM    <DIR>          subdir
2025-11-22  06:11 PM               100 test.txt
               2 File(s)            154 bytes
               3 Dir(s)  1,234,567,890 bytes free
`),
	want: []dirEntry{{
		name: ".bash_history",
		dir:  false,
		mode: 0,
		info: &fileInfo{
			name:  ".bash_history",
			size:  54,
			mode:  0644,
			mtime: time.Date(2021, 12, 31, 9, 44, 0, 0, time.UTC),
			dir:   false,
		},
	}, {
		name: "subdir",
		dir:  true,
		mode: fs.ModeDir,
		info: &fileInfo{
			name:  "subdir",
			size:  0,
			mode:  0755 | fs.ModeDir,
			mtime: time.Date(2025, 11, 22, 18, 11, 0, 0, time.UTC),
			dir:   true,
		},
	}, {
		name: "test.txt",
		dir:  false,
		mode: 0,
		info: &fileInfo{
			name:  "test.txt",
			size:  100,
			mode:  0644,
			mtime: time.Date(2025, 11, 22, 18, 11, 0, 0, time.UTC),
			dir:   false,
		},
	}},
}, {
	name: "only files",
	input: strings.TrimSpace(`
 Directory of C:\test

2025-11-22  06:11 PM               100 file1.txt
2025-11-22  06:11 PM               200 file2.txt
               2 File(s)            300 bytes
`),
	want: []dirEntry{{
		name: "file1.txt",
		dir:  false,
		mode: 0,
		info: &fileInfo{
			name:  "file1.txt",
			size:  100,
			mode:  0644,
			mtime: time.Date(2025, 11, 22, 18, 11, 0, 0, time.UTC),
			dir:   false,
		},
	}, {
		name: "file2.txt",
		dir:  false,
		mode: 0,
		info: &fileInfo{
			name:  "file2.txt",
			size:  200,
			mode:  0644,
			mtime: time.Date(2025, 11, 22, 18, 11, 0, 0, time.UTC),
			dir:   false,
		},
	}},
}, {
	name: "with size commas",
	input: strings.TrimSpace(`
 Directory of C:\test

2025-11-22  06:11 PM             1,234 largefile.bin
               1 File(s)          1,234 bytes
`),
	want: []dirEntry{{
		name: "largefile.bin",
		dir:  false,
		mode: 0,
		info: &fileInfo{
			name:  "largefile.bin",
			size:  1234,
			mode:  0644,
			mtime: time.Date(2025, 11, 22, 18, 11, 0, 0, time.UTC),
			dir:   false,
		},
	}},
}, {
	name: "filename with spaces",
	input: strings.TrimSpace(`
 Directory of C:\test

2025-11-22  06:11 PM               150 file with spaces.txt
               1 File(s)            150 bytes
`),
	want: []dirEntry{{
		name: "file with spaces.txt",
		dir:  false,
		mode: 0,
		info: &fileInfo{
			name:  "file with spaces.txt",
			size:  150,
			mode:  0644,
			mtime: time.Date(2025, 11, 22, 18, 11, 0, 0, time.UTC),
			dir:   false,
		},
	}},
}, {
	name: "empty directory",
	input: strings.TrimSpace(`
 Volume in drive C has no label.
 Directory of C:\test

2025-11-22  06:11 PM    <DIR>          .
2025-11-22  06:11 PM    <DIR>          ..
               0 File(s)              0 bytes
`),
	want: nil,
}, {
	name: "single-digit days",
	input: strings.TrimSpace(`
 Directory of C:\test

2009-02-02  03:04 PM               100 feb.txt
1970-01-01  12:00 AM               200 epoch.txt
               2 File(s)            300 bytes
`),
	want: []dirEntry{{
		name: "feb.txt",
		dir:  false,
		mode: 0,
		info: &fileInfo{
			name:  "feb.txt",
			size:  100,
			mode:  0644,
			mtime: time.Date(2009, 2, 2, 15, 4, 0, 0, time.UTC),
			dir:   false,
		},
	}, {
		name: "epoch.txt",
		dir:  false,
		mode: 0,
		info: &fileInfo{
			name:  "epoch.txt",
			size:  200,
			mode:  0644,
			mtime: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
			dir:   false,
		},
	}},
}}

func TestParseDOSReadDir(t *testing.T) {
	for _, tt := range dosReadDirTests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				r   = strings.NewReader(tt.input)
				got []dirEntry
			)
			for entry, err := range dosDirEntries(r, "") {
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
