package backup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/r14r/update-cli/lib/config"
)

func TestResolveRejectsSymlinkBackup(t *testing.T) {
	root := t.TempDir()
	backupRoot := filepath.Join(root, "backup")
	outside := t.TempDir()
	if err := os.MkdirAll(backupRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	m := Metadata{SchemaVersion: 1, ProjectName: "demo", Version: "1.0.0", CreatedAt: time.Now(), SourceDir: "/tmp/demo"}
	b, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(outside, MetadataFile), b, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(backupRoot, "escape")); err != nil {
		t.Fatal(err)
	}
	_, err := Resolve(config.Config{BackupRoot: backupRoot, ProjectName: "demo"}, "escape")
	if err == nil || !strings.Contains(err.Error(), "unsicher") {
		t.Fatalf("expected unsafe symlink rejection, got %v", err)
	}
}

func TestResolveLatestValidatedBackup(t *testing.T) {
	root := t.TempDir()
	backupRoot := filepath.Join(root, "backup")
	itemDir := filepath.Join(backupRoot, "20260817-v1.0.0")
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := Metadata{SchemaVersion: 1, ProjectName: "demo", Version: "1.0.0", CreatedAt: time.Now(), SourceDir: "/tmp/demo"}
	b, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(itemDir, MetadataFile), b, 0o600); err != nil {
		t.Fatal(err)
	}
	item, err := Resolve(config.Config{BackupRoot: backupRoot, ProjectName: "demo"}, "latest")
	if err != nil {
		t.Fatal(err)
	}
	if item.Version != "1.0.0" || !item.Validated {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestResolveLatestSkipsNewerInvalidBackup(t *testing.T) {
	root := t.TempDir()
	backupRoot := filepath.Join(root, "backup")
	validDir := filepath.Join(backupRoot, "valid")
	invalidDir := filepath.Join(backupRoot, "invalid")
	if err := os.MkdirAll(validDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	m := Metadata{SchemaVersion: 1, ProjectName: "demo", Version: "1.0.0", CreatedAt: old, SourceDir: "/tmp/demo"}
	b, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(validDir, MetadataFile), b, 0o600); err != nil {
		t.Fatal(err)
	}
	// Make the invalid directory look newer than the validated backup.
	if err := os.Chtimes(invalidDir, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	item, err := Resolve(config.Config{BackupRoot: backupRoot, ProjectName: "demo"}, "latest")
	if err != nil {
		t.Fatal(err)
	}
	if item.Name != "valid" || !item.Validated {
		t.Fatalf("expected validated backup, got %+v", item)
	}
}
