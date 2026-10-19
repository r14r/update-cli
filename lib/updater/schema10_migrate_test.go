package updater

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/doctor"
)

func TestMigrateCommandUpgradesSchemaNineAndDoctorReportsState(t *testing.T) {
	root := t.TempDir()
	if _, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, config.ConfigDirName, config.ConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	doc["schemaVersion"] = 9
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	before := doctor.RunProject(root, false)
	if before.RuntimeConfig == nil || !before.RuntimeConfig.MigrationNeeded || before.RuntimeConfig.CurrentSchema != 10 {
		t.Fatalf("doctor did not report schema migration: %#v", before.RuntimeConfig)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(saved) != string(b) {
		t.Fatal("doctor unexpectedly mutated config")
	}
	if err := Run(context.Background(), "3.3.0", []string{"migrate", "--root", root, "--noui"}); err != nil {
		t.Fatal(err)
	}
	migrated, err := config.Check(root)
	if err != nil {
		t.Fatal(err)
	}
	if migrated.SchemaVersion != 10 || migrated.MigrationNeeded {
		t.Fatalf("migration incomplete: %#v", migrated)
	}
	backups, err := filepath.Glob(path + ".backup-v9-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) == 0 {
		t.Fatal("migration did not preserve pre-migration backup")
	}
	if err := Run(context.Background(), "3.3.0", []string{"migrate", "--root", root, "--noui"}); err != nil {
		t.Fatalf("idempotent migration failed: %v", err)
	}
}
