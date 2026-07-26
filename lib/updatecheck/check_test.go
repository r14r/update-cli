package updatecheck

import (
	"os"
	"path/filepath"
	"testing"

	"release-updater/lib/config"
)

func TestRunDetectsAvailableUpdate(t *testing.T) {
	root := t.TempDir()
	downloads := filepath.Join(root, "downloads")
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(downloads, "demo-v1.10.0.zip"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, ".release-version"), []byte("1.2.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(config.Config{
		ProjectName: "demo",
		DownloadDir: downloads,
		CurrentDir:  current,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusUpdateAvailable || result.Available.String() != "1.10.0" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRunDetectsMissingInstallation(t *testing.T) {
	root := t.TempDir()
	downloads := filepath.Join(root, "downloads")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(downloads, "demo-v1.0.0.zip"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(config.Config{
		ProjectName: "demo",
		DownloadDir: downloads,
		CurrentDir:  filepath.Join(root, "current"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusNotInstalled || result.InstalledFound {
		t.Fatalf("unexpected result: %#v", result)
	}
}
