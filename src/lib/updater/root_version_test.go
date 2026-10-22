package updater

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r14r/update-cli/lib/config"
)

func TestWriteLegacyRootMarkersSynchronizesRootVersion(t *testing.T) {
	root := t.TempDir()
	release := filepath.Join(root, "release")
	if err := os.MkdirAll(release, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{RootDir: root, ReleaseRoot: release, ProjectName: "demo"}
	if err := writeLegacyRootMarkers(cfg, "2.12.1", "test"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(body)) != "2.12.1" {
		t.Fatalf("VERSION = %q", body)
	}
}

func TestWriteProjectVersionRejectsInvalidVersion(t *testing.T) {
	cfg := config.Config{RootDir: t.TempDir()}
	if err := writeProjectVersion(cfg, "latest"); err == nil {
		t.Fatal("expected invalid semantic version to fail")
	}
}

func TestSyncProjectVersionUsesCurrentVersionAsAuthority(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, ".release-version"), []byte("2.2.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "VERSION"), []byte("2.14.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{RootDir: root, CurrentDir: current}
	if err := syncProjectVersionFromCurrent(cfg); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(body)) != "2.14.1" {
		t.Fatalf("root VERSION = %q; expected current/VERSION 2.14.1", body)
	}
}

func TestSyncProjectVersionMigratesLegacyReleaseVersionOnlyWhenVersionMissing(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, ".release-version"), []byte("2.13.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{RootDir: root, CurrentDir: current}
	if err := syncProjectVersionFromCurrent(cfg); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(root, "VERSION"), filepath.Join(current, "VERSION")} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(body)) != "2.13.3" {
			t.Fatalf("%s = %q; expected migrated legacy version", path, body)
		}
	}
}
