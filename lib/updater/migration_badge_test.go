package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectMigrationRequiredForOutdatedRuntimeConfig(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".update-cli")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"schemaVersion":6,"projectName":"demo","mode":"update","source":{"type":"download","folder":"$HOME/Downloads"},"releaseDir":"release","currentDir":"current","backup":{"directory":"backup","keep":3},"retention":{"releases":5},"sync":{"preserve":[".gitignore"]},"security":{"allowHttp":false,"maxArchiveBytes":2147483648,"maxUncompressedBytes":8589934592,"maxFileBytes":2147483648,"maxEntries":100000,"maxCompressionRatio":200},"docker":{"lifecycle":"auto"},"healthcheck":{}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if !projectMigrationRequired(root) {
		t.Fatal("outdated runtime config must require migration")
	}
}

func TestProjectMigrationRequiredForLegacyManifestSchema(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "update-cli.yaml"), []byte("version: 1\nproject: demo\nsetup:\n  - name: test\n    command: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !projectMigrationRequired(root) {
		t.Fatal("legacy manifest schema must require migration")
	}
}

func TestProjectMigrationRequiredFalseForCurrentConfigAndValidManifest(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, ".update-cli")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `{"schemaVersion":9,"projectName":"demo","mode":"update","source":{"type":"download","folder":"$HOME/Downloads"},"releaseDir":"release","currentDir":"current","backup":{"directory":"backup","keep":3},"retention":{"releases":5},"sync":{"preserve":[".env"]},"security":{"allowHttp":false,"maxArchiveBytes":2147483648,"maxUncompressedBytes":8589934592,"maxFileBytes":2147483648,"maxEntries":100000,"maxCompressionRatio":200},"docker":{"lifecycle":"auto"},"healthcheck":{}}`
	if err := os.WriteFile(filepath.Join(stateDir, "config.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := `schemaVersion: 2
project:
  name: demo
update:
  mode: update
  source:
    type: download
    folder: $HOME/Downloads
  sync:
    preserve:
      - .env
tasks:
  setup:
    steps:
      - shell: echo ok
workflows:
  setup:
    tasks:
      - setup
`
	if err := os.WriteFile(filepath.Join(root, "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if projectMigrationRequired(root) {
		t.Fatal("current config plus valid current-schema manifest must not require migration")
	}
}
