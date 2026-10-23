package updater

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r14r/update-cli/src/lib/config"
)

func TestInPlaceActivationKeepsCurrentDirectoryAndSynchronizesContents(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	v1 := releaseZip(t, downloads, "demo", "1.0.0", map[string]string{
		"app.txt":      "before",
		"obsolete.txt": "remove",
	})
	if err := Run(context.Background(), "3.2.0", []string{"update", v1, "--root", root, "--no-setup", "--no-ui"}); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(cfg.CurrentDir)
	if err != nil {
		t.Fatal(err)
	}
	// .env is a mandatory deployment preserve, not present in the new release.
	if err := os.WriteFile(filepath.Join(cfg.CurrentDir, ".env"), []byte("secret-local-value"), 0600); err != nil {
		t.Fatal(err)
	}
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cfg.CurrentDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldCwd) }()
	v2 := releaseZip(t, downloads, "demo", "1.0.1", map[string]string{
		"app.txt": "after",
		"new.txt": "added",
	})
	if err := Run(context.Background(), "3.2.0", []string{"update", v2, "--no-setup", "--no-ui"}); err != nil {
		t.Fatalf("updating from inside current should succeed: %v", err)
	}
	after, err := os.Stat(cfg.CurrentDir)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("in-place activation replaced the current directory inode")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory is no longer valid: %v", err)
	}
	cwdInfo, err := os.Stat(cwd)
	if err != nil || !os.SameFile(after, cwdInfo) {
		t.Fatalf("working directory changed identity: %q (%v)", cwd, err)
	}
	for _, tc := range []struct{ name, expected string }{
		{"app.txt", "after"},
		{"new.txt", "added"},
		{".env", "secret-local-value"},
	} {
		b, err := os.ReadFile(filepath.Join(cfg.CurrentDir, tc.name))
		if err != nil || string(b) != tc.expected {
			t.Fatalf("%s = %q (%v), want %q", tc.name, b, err, tc.expected)
		}
	}
	if _, err := os.Stat(filepath.Join(cfg.CurrentDir, "obsolete.txt")); !os.IsNotExist(err) {
		t.Fatalf("obsolete file survived rsync --delete: %v", err)
	}
}

func TestAtomicActivationStillRejectsWorkingDirectoryWithinCurrent(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	v1 := releaseZip(t, downloads, "demo", "1.0.0", map[string]string{"app.txt": "before"})
	if err := Run(context.Background(), "3.2.0", []string{"update", v1, "--root", root, "--no-setup", "--no-ui"}); err != nil {
		t.Fatal(err)
	}
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cfg.CurrentDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldCwd) }()
	v2 := releaseZip(t, downloads, "demo", "1.0.1", map[string]string{"app.txt": "after"})
	err = Run(context.Background(), "3.2.0", []string{"update", v2, "--activation", "atomic", "--no-setup", "--no-ui"})
	if err == nil || !strings.Contains(err.Error(), "atomaren Update") {
		t.Fatalf("expected atomic activation to reject current cwd; got %v", err)
	}
}
