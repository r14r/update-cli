package inventory

import (
	"os"
	"path/filepath"
	"testing"

	"release-updater/lib/config"
)

func TestListSortsDownloadsAndReleases(t *testing.T) {
	root := t.TempDir()
	downloads := filepath.Join(root, "downloads")
	releases := filepath.Join(root, "release")
	current := filepath.Join(root, "current")
	for _, path := range []string{downloads, releases, current} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"demo-v1.2.0.zip", "demo-v1.10.0.zip"} {
		if err := os.WriteFile(filepath.Join(downloads, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, version := range []string{"1.2.0", "1.10.0"} {
		dir := filepath.Join(releases, version)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".release-version"), []byte(version+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".release-project"), []byte("demo\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(current, ".release-version"), []byte("1.10.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := List(config.Config{ProjectName: "demo", DownloadDir: downloads, ReleaseRoot: releases, CurrentDir: current})
	if err != nil {
		t.Fatal(err)
	}
	if result.Downloads[0].VersionS != "1.10.0" || result.Releases[0].Version != "1.10.0" || !result.Releases[0].Active {
		t.Fatalf("unexpected result: %#v", result)
	}
}
