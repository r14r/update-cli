package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalInsideRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "current")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := CanonicalInside(root, link, true); err == nil {
		t.Fatal("expected symlink path rejection")
	}
}

func TestDirectorySwapRollbackRestoresPreviousTarget(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, "stage")
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(stage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "value.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "value.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	swap, err := SwapDirectory(stage, target)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(target, "value.txt"))
	if err != nil || string(data) != "new" {
		t.Fatalf("target was not activated: %q, %v", data, err)
	}
	if err := swap.Rollback(); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(target, "value.txt"))
	if err != nil || string(data) != "old" {
		t.Fatalf("previous target was not restored: %q, %v", data, err)
	}
}

func TestDirectorySwapCommitRemovesPreviousTarget(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, "stage")
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(stage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "value.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "value.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	swap, err := SwapDirectory(stage, target)
	if err != nil {
		t.Fatal(err)
	}
	previous := swap.Backup
	if err := swap.Commit(); err != nil {
		t.Fatal(err)
	}
	if previous != "" {
		if _, err := os.Stat(previous); !os.IsNotExist(err) {
			t.Fatalf("previous target still exists after commit: %s", previous)
		}
	}
	data, err := os.ReadFile(filepath.Join(target, "value.txt"))
	if err != nil || string(data) != "new" {
		t.Fatalf("committed target is wrong: %q, %v", data, err)
	}
}

func TestDirectorySwapDoesNotDeletePreexistingOldDirectory(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, "stage")
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(stage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := target + ".old-stale"
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	swap, err := SwapDirectory(stage, target)
	if err != nil {
		t.Fatal(err)
	}
	defer swap.Rollback()
	if _, err := os.Stat(filepath.Join(stale, "keep.txt")); err != nil {
		t.Fatalf("pre-existing recovery directory was touched: %v", err)
	}
	if swap.Backup == stale {
		t.Fatalf("swap reused pre-existing recovery directory %s", stale)
	}
}
