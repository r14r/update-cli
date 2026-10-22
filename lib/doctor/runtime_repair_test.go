package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/r14r/update-cli/lib/config"
)

func TestDoctorMigrateRepairsUnsupportedRuntimeFields(t *testing.T) {
	root := t.TempDir()
	if _, err := config.Init(root, config.InitOptions{
		ProjectName: "demo",
		SourceType:  "download",
		Folder:      t.TempDir(),
	}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, config.ConfigDirName, config.ConfigFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Strict upgrade rejects the invalid field type; doctor --migrate
	// falls back to the backup-backed repair engine.
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	raw["no parameter"] = map[string]any{"invalid": true}
	b, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	report := &Report{}
	status := checkRuntimeConfig(root, true, report)
	if status.MigrationNeeded {
		t.Fatal("runtime configuration still marked as needing migration")
	}
	if _, err := config.Load(root, ""); err != nil {
		t.Fatalf("runtime config remains invalid after doctor migration: %v", err)
	}
}
