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

func TestDoctorCommandDoesNotRequireRuntimeConfig(t *testing.T) {
	root := t.TempDir()
	manifest := `schemaVersion: 2
project:
  name: Demo
workflows:
  setup:
    tasks: [setup]
tasks:
  setup:
    steps:
      - shell: echo ok
`
	if err := os.WriteFile(filepath.Join(root, "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), "test", []string{"doctor", "--root", root, "--no-color"}); err != nil {
		t.Fatalf("doctor failed without runtime config: %v", err)
	}
}

func TestDoctorCommandMigrateUpdatesLegacyManifest(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "update-cli.yaml")
	manifest := `schemaVersion: 1
project:
  name: Demo
steps:
  - id: test
    run: echo ok
`
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), "test", []string{"doctor", "--migrate", "--root", root, "--no-color"}); err != nil {
		t.Fatalf("doctor migrate failed: %v", err)
	}
	m, err := projectsetup.ParseManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != projectsetup.SchemaVersion {
		t.Fatalf("schema=%d want=%d", m.Version, projectsetup.SchemaVersion)
	}
}

func TestDoctorCommandFromCurrentUsesParentProject(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	downloads := filepath.Join(root, "downloads")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads}); err != nil {
		t.Fatal(err)
	}
	manifest := `schemaVersion: 2
project:
  name: Demo
run:
  command: echo ok
`
	if err := os.WriteFile(filepath.Join(current, "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(current); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	if err := Run(context.Background(), "test", []string{"doctor", "--no-color"}); err != nil {
		t.Fatalf("doctor from current failed: %v", err)
	}
}

func TestDoctorFixPreviewsAndRepairsAfterConfirmation(t *testing.T) {
	root := t.TempDir()
	t.Setenv("UPDATE_CLI_GLOBAL_CONFIG_DIR", filepath.Join(t.TempDir(), "global"))
	downloads := filepath.Join(root, "downloads")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads}); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(current, config.ProjectFileName)
	manifest := `schemaVersion: 2
project:
  name: Demo
  wrongField: remove-me
workflows:
  setup:
    tasks: [setup]
tasks:
  setup:
    steps:
      - id: ok
        shell: echo ok
`
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString("y\n"); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		_ = r.Close()
	}()

	if err := Run(context.Background(), "test", []string{"doctor", "--fix", "--root", root, "--no-color", "--no-ui"}); err != nil {
		t.Fatalf("doctor --fix failed: %v", err)
	}
	inspection := projectsetup.InspectManifest(path)
	if inspection.Repairable || !inspection.Valid {
		t.Fatalf("manifest remains repairable/invalid after doctor --fix: %#v", inspection)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "wrongField") {
		t.Fatalf("doctor --fix did not remove wrongField:\n%s", data)
	}
}
