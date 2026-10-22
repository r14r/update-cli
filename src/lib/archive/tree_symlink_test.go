package archive

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidateTreeInternalSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions differ on Windows")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".set_env.sh"), []byte("export TEST=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(".set_env.sh", filepath.Join(root, ".envrc")); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateTree(context.Background(), root, Limits{}); err != nil {
		t.Fatalf("internal relative symlink should be accepted: %v", err)
	}
}

func TestValidateTreeRejectsUnsafeSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions differ on Windows")
	}
	for _, tc := range []struct {
		name   string
		target string
	}{
		{"absolute", "/tmp"},
		{"outside", "../"},
		{"dangling", "missing.sh"},
		{"loop", "link"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Symlink(tc.target, filepath.Join(root, "link")); err != nil {
				t.Fatal(err)
			}
			_, err := ValidateTree(context.Background(), root, Limits{})
			if err == nil {
				t.Fatal("unsafe symlink accepted")
			}
			if !strings.Contains(err.Error(), "symbolischer Link") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
