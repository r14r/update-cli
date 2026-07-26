package cleanup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"release-updater/lib/backup"
	"release-updater/lib/config"
)

func TestCleanupProtectsActiveAndPreviousRelease(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{
		ProjectName:  "demo",
		DownloadDir:  filepath.Join(root, "downloads"),
		CurrentDir:   filepath.Join(root, "current"),
		ReleaseRoot:  filepath.Join(root, "release"),
		BackupRoot:   filepath.Join(root, "backup"),
		KeepReleases: 0,
		KeepBackups:  1,
	}
	if err := os.MkdirAll(cfg.DownloadDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.CurrentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.CurrentDir, ".release-version"), []byte("3.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"1.0.0", "2.0.0", "3.0.0"} {
		path := filepath.Join(cfg.ReleaseRoot, version)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, ".release-version"), []byte(version+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, ".release-project"), []byte("demo\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for index, name := range []string{"backup-old", "backup-new"} {
		path := filepath.Join(cfg.BackupRoot, name)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		metadata := backup.Metadata{SchemaVersion: 1, ProjectName: "demo", Version: "3.0.0", CreatedAt: time.Date(2026, 7, 20+index, 10, 0, 0, 0, time.UTC)}
		data, _ := json.Marshal(metadata)
		if err := os.WriteFile(filepath.Join(path, backup.MetadataFile), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := Run(cfg, -1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RemovedRelease) != 1 || filepath.Base(result.RemovedRelease[0]) != "1.0.0" {
		t.Fatalf("unexpected removed releases: %#v", result.RemovedRelease)
	}
	if _, err := os.Stat(filepath.Join(cfg.ReleaseRoot, "2.0.0")); err != nil {
		t.Fatalf("previous release removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ReleaseRoot, "3.0.0")); err != nil {
		t.Fatalf("active release removed: %v", err)
	}
	if len(result.RemovedBackup) != 1 || filepath.Base(result.RemovedBackup[0]) != "backup-old" {
		t.Fatalf("unexpected removed backups: %#v", result.RemovedBackup)
	}
}
