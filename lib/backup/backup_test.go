package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"release-updater/lib/config"
)

func TestCreateListAndResolveLatest(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, ".release-version"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "file.txt"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{ProjectName: "demo", CurrentDir: current, BackupRoot: filepath.Join(root, "backup"), ConfigDir: filepath.Join(root, ".updater-cli")}
	result, err := Create(context.Background(), cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Backup.Version != "1.2.3" || !result.Backup.Validated {
		t.Fatalf("unexpected backup: %#v", result.Backup)
	}
	item, err := Resolve(cfg, "latest")
	if err != nil {
		t.Fatal(err)
	}
	if item.Name != result.Backup.Name {
		t.Fatalf("resolved %q, want %q", item.Name, result.Backup.Name)
	}
	if _, err := os.Stat(filepath.Join(item.Path, MetadataFile)); err != nil {
		t.Fatal(err)
	}
}
