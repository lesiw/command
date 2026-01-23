//go:build windows

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
	out, err := command.Read(ctx, m, "powershell", "-NoProfile",
		"-Command", "Get-Location")
	if err != nil {
		t.Fatalf("Read(Get-Location): %v", err)
	}

	got := strings.TrimSpace(out)
	if !strings.HasSuffix(got, "subdir") {
		t.Errorf("Get-Location = %q, want suffix \"subdir\"", got)
	}
}

func TestWorkDirUnixStyle(t *testing.T) {
	m, ctx := sys.Machine(), context.Background()

	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	nestedDir := filepath.Join(subDir, "nested")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
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

	ctx = fs.WithWorkDir(ctx, "subdir/nested")
	out, err := command.Read(ctx, m, "powershell", "-NoProfile",
		"-Command", "Get-Location")
	if err != nil {
		t.Fatalf("Read(Get-Location): %v", err)
	}

	got := strings.TrimSpace(out)
	if !strings.HasSuffix(got, "nested") {
		t.Errorf("Get-Location = %q, want suffix \"nested\"", got)
	}
	if !strings.Contains(got, "\\") {
		t.Errorf("Get-Location = %q, want backslashes", got)
	}
}
