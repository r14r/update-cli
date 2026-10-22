package projectsetup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/ui"
)

func TestMigrationRunsOncePerVersionAndMarkerPersistsOutsideCurrent(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `#!/usr/bin/env bash
set -eu
count_file="$UPDATE_CLI_PROJECT_ROOT/migration-count.txt"
count=0
[ ! -f "$count_file" ] || count="$(cat "$count_file")"
printf '%s\n' "$((count + 1))" > "$count_file"
printf '%s\n' "$UPDATE_CLI_VERSION" > migration-version.txt
`
	if err := os.WriteFile(filepath.Join(current, MigrationScriptName), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{RootDir: root, ConfigDir: filepath.Join(root, config.ConfigDirName), ProjectName: "demo", CurrentDir: current}

	if _, err := Run(context.Background(), cfg, ui.New(true)); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), cfg, ui.New(true)); err != nil {
		t.Fatal(err)
	}
	count, err := os.ReadFile(filepath.Join(root, "migration-count.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(count)) != "1" {
		t.Fatalf("migration count=%q want 1", count)
	}
	marker := filepath.Join(root, config.ConfigDirName, ".migration.done.1.2.3")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("persistent marker missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(current, ".migration.done.1.2.3")); !os.IsNotExist(err) {
		t.Fatalf("marker must not live in replaceable current/: %v", err)
	}

	if err := os.WriteFile(filepath.Join(current, "VERSION"), []byte("1.2.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), cfg, ui.New(true)); err != nil {
		t.Fatal(err)
	}
	count, err = os.ReadFile(filepath.Join(root, "migration-count.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(count)) != "2" {
		t.Fatalf("migration count=%q want 2 after new version", count)
	}
	if _, err := os.Stat(filepath.Join(root, config.ConfigDirName, ".migration.done.1.2.4")); err != nil {
		t.Fatalf("second version marker missing: %v", err)
	}
}

func TestMigrationFailurePreventsSetupAndDoesNotWriteDoneMarker(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "VERSION"), []byte("2.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, MigrationScriptName), []byte("#!/usr/bin/env bash\nexit 17\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `schemaVersion: 1
steps:
  - name: setup must not run
    type: command
    command: printf setup > setup-ran.txt
`
	if err := os.WriteFile(filepath.Join(current, "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{RootDir: root, ConfigDir: filepath.Join(root, config.ConfigDirName), ProjectName: "demo", CurrentDir: current}

	_, err := Run(context.Background(), cfg, ui.New(true))
	if err == nil || !strings.Contains(err.Error(), "Migration für Version 2.0.0 fehlgeschlagen") {
		t.Fatalf("expected migration failure, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(current, "setup-ran.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("setup ran despite migration error: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, config.ConfigDirName, ".migration.done.2.0.0")); !os.IsNotExist(statErr) {
		t.Fatalf("done marker written after failed migration: %v", statErr)
	}
}

func TestMigrationRequiresValidVersionWhenScriptExists(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, MigrationScriptName), []byte("#!/usr/bin/env bash\nexit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{RootDir: root, CurrentDir: root}
	if _, err := EnsureMigration(context.Background(), cfg, ui.New(true)); err == nil || !strings.Contains(err.Error(), "VERSION-Datei") {
		t.Fatalf("expected missing VERSION error, got %v", err)
	}
}
