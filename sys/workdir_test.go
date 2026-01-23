//go:build unix

package sys_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lesiw.io/command"
	"lesiw.io/command/sys"
	"lesiw.io/fs"
)

func TestWorkDirRelative(t *testing.T) {
	m, ctx := sys.Machine(), context.Background()

	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	ctx = fs.WithWorkDir(ctx, "subdir")
	out, err := command.Read(ctx, m, "pwd")
	if err != nil {
		t.Fatalf("Read(pwd): %v", err)
	}

	got := strings.TrimSpace(out)
	if !strings.HasSuffix(got, "subdir") {
		t.Errorf("pwd = %q, want suffix \"subdir\"", got)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("pwd = %q, want absolute path", got)
	}

	gotResolved, err := filepath.EvalSymlinks(got)
	if err != nil {
		gotResolved = got
	}
	expectedResolved, err := filepath.EvalSymlinks(subDir)
	if err != nil {
		expectedResolved = subDir
	}

	if gotResolved != expectedResolved {
		t.Errorf("pwd = %q, want %q", gotResolved, expectedResolved)
	}
}
