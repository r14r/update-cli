package updater

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/projectsetup"
)

func TestFixCommandRepairsConfigAndManifestBeforeStrictLoad(t *testing.T) {
	root := t.TempDir()
	globalDir := t.TempDir()
	t.Setenv("UPDATE_CLI_GLOBAL_CONFIG_DIR", globalDir)
	if err := os.MkdirAll(filepath.Join(root, config.ConfigDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	localConfig := `{
  "schemaVersion": 8,
  "projectName": "demo",
  "defaultUser": "r1r",
  "source": {"type":"download","folder":"$HOME/Downloads","oldSourceValue":true},
  "releaseDir":"release",
  "currentDir":"current",
  "unknownConfigValue":"remove"
}`
	if err := os.WriteFile(filepath.Join(root, config.ConfigDirName, config.ConfigFileName), []byte(localConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, config.ConfigFileName), []byte(`{"defaultUser":"r1r","unknownGlobal":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `schemaVersion: 2
project:
  name: demo
  oldProjectField: remove
tasks:
  setup:
    steps:
      - shell: echo ok
        oldStepField: remove
`
	manifestPath := filepath.Join(root, "update-cli.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Run(context.Background(), "test", []string{"fix", "--root", root, "--no-color"}); err != nil {
		t.Fatalf("fix failed: %v", err)
	}
	cfg, err := config.Load(root, "")
	if err != nil {
		t.Fatalf("strict config load after fix failed: %v", err)
	}
	if cfg.Source.DefaultUser != "r1r" {
		t.Fatalf("default user=%q", cfg.Source.DefaultUser)
	}
	if _, err := projectsetup.ParseManifest(manifestPath); err != nil {
		t.Fatalf("strict manifest parse after fix failed: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(root, config.ConfigDirName, config.ConfigFileName))
	if strings.Contains(string(body), "unknownConfigValue") || strings.Contains(string(body), "oldSourceValue") {
		t.Fatalf("config not cleaned: %s", body)
	}
	manifestBody, _ := os.ReadFile(manifestPath)
	if strings.Contains(string(manifestBody), "oldProjectField") || strings.Contains(string(manifestBody), "oldStepField") {
		t.Fatalf("manifest not cleaned: %s", manifestBody)
	}
}

func TestFixCommandRepairsRootAndCurrentManifests(t *testing.T) {
	root := t.TempDir()
	t.Setenv("UPDATE_CLI_GLOBAL_CONFIG_DIR", t.TempDir())
	cfgDir := filepath.Join(root, config.ConfigDirName)
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	localConfig := `{
  "schemaVersion": 9,
  "projectName": "demo",
  "source": {"type":"download","folder":"$HOME/Downloads"},
  "releaseDir":"release",
  "currentDir":"current"
}`
	if err := os.WriteFile(filepath.Join(cfgDir, config.ConfigFileName), []byte(localConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	bad := `schemaVersion: 2
project:
  name: demo
  wrongField: remove
tasks:
  setup:
    steps:
      - shell: echo ok
        wrongStep: remove
workflows:
  setup:
    tasks: [setup]
`
	rootManifest := filepath.Join(root, config.ProjectFileName)
	currentManifest := filepath.Join(current, config.ProjectFileName)
	if err := os.WriteFile(rootManifest, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(currentManifest, []byte(strings.Replace(bad, "wrongField", "currentWrongField", 1)), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Run(context.Background(), "test", []string{"fix", "--root", root, "--no-color"}); err != nil {
		t.Fatalf("fix failed: %v", err)
	}
	for _, path := range []string{rootManifest, currentManifest} {
		if _, err := projectsetup.ParseManifest(path); err != nil {
			t.Fatalf("strict parse after fix failed for %s: %v", path, err)
		}
		body, _ := os.ReadFile(path)
		if strings.Contains(string(body), "wrongField") || strings.Contains(string(body), "currentWrongField") || strings.Contains(string(body), "wrongStep") {
			t.Fatalf("manifest not fully repaired: %s", body)
		}
	}
}
