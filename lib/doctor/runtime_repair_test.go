package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r14r/update-cli/lib/config"
)

func TestDoctorMigrateRepairsUnsupportedRuntimeFields(t *testing.T) {
	root := t.TempDir()
	if _, err := config.Init(root, config.InitOptions{
		ProjectName: "demo",
		SourceType: "download",
		Folder: t.TempDir(),
	}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, config.ConfigDirName, config.ConfigFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Deliberately malformed known field. Strict upgrade must not silently
	// ignore it; the repair engine should normalize it and keep a backup.
	b = []byte(strings.Replace(string(b), "\"no parameter\": [", "\"no parameter\": { \"invalid\": true }, \"legacy-unused\": [", 1))
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
