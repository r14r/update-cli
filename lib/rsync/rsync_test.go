package rsync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestReleaseExcludesEnv(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync is not installed")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "release")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "app.txt"), []byte("app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".env"), []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Release(context.Background(), source, destination, filepath.Join(root, "release.log")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, "app.txt")); err != nil {
		t.Fatalf("regular file was not synchronized: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, ".env")); !os.IsNotExist(err) {
		t.Fatalf(".env must not be copied into release, stat error: %v", err)
	}
}

func TestCurrentPreservesEnv(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync is not installed")
	}

	root := t.TempDir()
	source := filepath.Join(root, "release")
	destination := filepath.Join(root, "current")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "app.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".env"), []byte("archive-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, ".env"), []byte("local-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Current(context.Background(), source, destination, filepath.Join(root, "current.log"), false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(destination, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "local-secret\n" {
		t.Fatalf(".env was changed: %q", data)
	}
}

func TestCurrentDetectsSameSizeContentChange(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync is not installed")
	}

	root := t.TempDir()
	source := filepath.Join(root, "release")
	destination := filepath.Join(root, "current")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(source, "same-size.txt")
	destinationPath := filepath.Join(destination, "same-size.txt")
	if err := os.WriteFile(sourcePath, []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destinationPath, []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stamp := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(sourcePath, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(destinationPath, stamp, stamp); err != nil {
		t.Fatal(err)
	}

	result, err := Current(context.Background(), source, destination, filepath.Join(root, "current.log"), false)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(destinationPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "two\n" {
		t.Fatalf("content was not updated: %q", data)
	}
	if result.Changes == 0 {
		t.Fatal("expected itemized change")
	}
}
