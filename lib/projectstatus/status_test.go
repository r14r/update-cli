package projectstatus

import (
	"os"
	"path/filepath"
	"testing"

	"release-updater/lib/config"
)

func TestRunDetectsUpdate(t *testing.T) {
	root := t.TempDir()
	downloads := filepath.Join(root, "downloads")
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(downloads, "demo-v2.0.0.zip"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, ".release-version"), []byte("1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(config.Config{ProjectName: "demo", DownloadDir: downloads, CurrentDir: current})
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "update-available" || result.AvailableVersion != "2.0.0" {
		t.Fatalf("unexpected result: %#v", result)
	}
}
