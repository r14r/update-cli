package cleanup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/r14r/update-cli/lib/config"
)

func writeRelease(t *testing.T, root, project, version string) {
	t.Helper()
	dir := filepath.Join(root, version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".release-version"), []byte(version+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".release-project"), []byte(project+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunReleasesLeavesBackupsUntouchedAndKeepsRollbackRelease(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	releases := filepath.Join(root, "release")
	backups := filepath.Join(root, "backup")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "VERSION"), []byte("0.8.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"0.8.0", "3.3.4", "3.3.3"} {
		writeRelease(t, releases, "update-cli", v)
	}
	backupDir := filepath.Join(backups, "keep-me")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{ProjectName: "update-cli", CurrentDir: current, ReleaseRoot: releases, BackupRoot: backups, KeepBackups: 3}
	res, err := RunReleases(cfg, -1, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.ReleaseOnly {
		t.Fatal("release-only cleanup not reported")
	}
	if len(res.RemovedRelease) != 1 || filepath.Base(res.RemovedRelease[0]) != "3.3.3" {
		t.Fatalf("removed releases = %#v", res.RemovedRelease)
	}
	for _, v := range []string{"0.8.0", "3.3.4"} {
		if _, err := os.Stat(filepath.Join(releases, v)); err != nil {
			t.Fatalf("protected release %s missing: %v", v, err)
		}
	}
	if _, err := os.Stat(backupDir); err != nil {
		t.Fatalf("backup was touched by --clean semantics: %v", err)
	}
}
