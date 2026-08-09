package archive

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, body := range entries {
		e, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRejectsTraversal(t *testing.T) {
	p := makeZip(t, map[string]string{"../escape": "bad"})
	if err := Validate(context.Background(), p, Limits{MaxEntries: 100, MaxFileBytes: 1000, MaxUncompressedBytes: 10000, MaxCompressionRatio: 100}); err == nil {
		t.Fatal("expected traversal rejection")
	}
}
func TestRejectsUncompressedLimit(t *testing.T) {
	p := makeZip(t, map[string]string{"VERSION": "1.0.0", "big.txt": strings.Repeat("x", 2048)})
	_, err := Inspect(context.Background(), p, Limits{MaxEntries: 100, MaxFileBytes: 4096, MaxUncompressedBytes: 1000, MaxCompressionRatio: 1000})
	if err == nil || !strings.Contains(err.Error(), "maximale entpackte Größe") {
		t.Fatalf("expected size rejection, got %v", err)
	}
}
func TestValidateTreeRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	_, err := ValidateTree(context.Background(), root, Limits{MaxEntries: 100, MaxFileBytes: 1000, MaxUncompressedBytes: 10000, MaxCompressionRatio: 100})
	if err == nil || !strings.Contains(err.Error(), "symbolischer Link") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
}
