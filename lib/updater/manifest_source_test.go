package updater

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/r14r/update-cli/lib/config"
)

func TestManifestSourceDefaultsOverrideLocalConfig(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `schemaVersion: 2
update:
  mode: pull
  source:
    type: repository
    repository: https://github.com/r14r/update-cli.git
    ref: main
run:
  command: echo ok
`
	if err := os.WriteFile(filepath.Join(current, "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		RootDir: root, CurrentDir: current, ProjectName: "demo",
		Mode:       config.ModeUpdate,
		Source:     config.SourceConfig{Type: "download", Folder: filepath.Join(root, "downloads")},
		SourceType: "download", SourceFolder: filepath.Join(root, "downloads"), DownloadDir: filepath.Join(root, "downloads"),
	}
	got, err := withManifestSourceDefaults(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != config.ModePull || got.Source.Type != "repository" || got.Source.Repository != "https://github.com/r14r/update-cli.git" || got.Source.Ref != "main" {
		t.Fatalf("got %#v", got)
	}
}

func TestCLIOverridesManifestSourceDefaults(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `schemaVersion: 2
update:
  mode: pull
  source:
    type: repository
    repository: https://github.com/r14r/update-cli.git
    ref: main
run:
  command: echo ok
`
	if err := os.WriteFile(filepath.Join(current, "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{RootDir: root, CurrentDir: current, ProjectName: "demo", Mode: config.ModeUpdate, Source: config.SourceConfig{Type: "download", Folder: filepath.Join(root, "downloads")}}
	got, err := withManifestSourceDefaults(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, err = config.WithSourceOverrides(got, "pull", "repository", "", "", "https://github.com/acme/override.git")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source.Repository != "https://github.com/acme/override.git" {
		t.Fatalf("CLI repository did not win: %#v", got.Source)
	}
}
