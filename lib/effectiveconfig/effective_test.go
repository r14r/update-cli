package effectiveconfig

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/r14r/update-cli/lib/config"
)

func initConfig(t *testing.T, root string) config.Config {
	t.Helper()
	downloads := filepath.Join(root, "downloads")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.Set(root, []string{
		"sync.preserve=.env,config-only/",
		"docker.lifecycle=auto",
		"setup.keepRsyncOnError=false",
	}); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestLoadManifestOverridesProjectDependentConfig(t *testing.T) {
	root := t.TempDir()
	initConfig(t, root)
	manifest := `schemaVersion: 2
project:
  name: Demo
  slug: yaml-demo
update:
  sync:
    preserve:
      - .env
      - data/
    keepOnSetupError: true
  docker:
    lifecycle: disabled
  retention:
    releases: 9
run:
  command: echo ok
`
	if err := os.WriteFile(filepath.Join(root, "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, meta, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProjectName != "yaml-demo" {
		t.Fatalf("project name = %q", cfg.ProjectName)
	}
	if !reflect.DeepEqual(cfg.Preserve, []string{".env", "data/", ".gitignore"}) {
		t.Fatalf("preserve = %#v", cfg.Preserve)
	}
	if cfg.Docker.Lifecycle != "disabled" || !cfg.KeepRsyncOnSetupError || cfg.KeepReleases != 9 {
		t.Fatalf("manifest overrides missing: %#v", cfg)
	}
	if meta.ManifestPath != filepath.Join(root, "update-cli.yaml") || len(meta.Applied) == 0 {
		t.Fatalf("metadata = %#v", meta)
	}
}

func TestLoadRootManifestHasPriorityOverCurrentManifest(t *testing.T) {
	root := t.TempDir()
	initConfig(t, root)
	if err := os.MkdirAll(filepath.Join(root, "current"), 0o755); err != nil {
		t.Fatal(err)
	}
	rootManifest := `schemaVersion: 2
update:
  docker:
    lifecycle: disabled
run:
  command: echo root
`
	currentManifest := `schemaVersion: 2
update:
  docker:
    lifecycle: required
run:
  command: echo current
`
	if err := os.WriteFile(filepath.Join(root, "update-cli.yaml"), []byte(rootManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "current", "update-cli.yaml"), []byte(currentManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, meta, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Docker.Lifecycle != "disabled" {
		t.Fatalf("docker lifecycle = %q", cfg.Docker.Lifecycle)
	}
	if meta.ManifestPath != filepath.Join(root, "update-cli.yaml") {
		t.Fatalf("manifest = %q", meta.ManifestPath)
	}
}

func TestLoadFallsBackToCurrentManifest(t *testing.T) {
	root := t.TempDir()
	initConfig(t, root)
	if err := os.MkdirAll(filepath.Join(root, "current"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `schemaVersion: 2
update:
  sync:
    preserve: [.env, uploads/]
run:
  command: echo current
`
	if err := os.WriteFile(filepath.Join(root, "current", "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, meta, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.Preserve, []string{".env", "uploads/", ".gitignore"}) {
		t.Fatalf("preserve = %#v", cfg.Preserve)
	}
	if meta.ManifestPath != filepath.Join(root, "current", "update-cli.yaml") {
		t.Fatalf("manifest = %q", meta.ManifestPath)
	}
}
