package archive

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractAndResolveWrapper(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "release.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("wrapper/VERSION")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("1.2.3\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if err := Validate(archivePath); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	if err := Extract(archivePath, destination); err != nil {
		t.Fatal(err)
	}
	root, err := ResolveContentRoot(destination)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(root) != "wrapper" {
		t.Fatalf("unexpected root: %s", root)
	}
	if err := ValidateVersionFile(root, "1.2.3"); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsTraversal(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "unsafe.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("../escape.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("unsafe")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if err := Validate(archivePath); err == nil {
		t.Fatal("expected traversal archive to be rejected")
	}
}
