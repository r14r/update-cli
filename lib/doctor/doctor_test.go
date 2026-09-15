package doctor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/projectsetup"
)

func dockerCheck(r Report) (Check, bool) {
	for _, c := range r.Checks {
		if c.Name == "Docker Compose" {
			return c, true
		}
	}
	return Check{}, false
}

func TestDoctorDockerLifecycleDisabledSkipsDocker(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{RootDir: root, CurrentDir: current, ReleaseRoot: filepath.Join(root, "release"), BackupRoot: filepath.Join(root, "backup"), Docker: config.DockerConfig{Lifecycle: "disabled"}}
	r := Run(context.Background(), root, cfg)
	c, ok := dockerCheck(r)
	if !ok || c.Level != LevelOK || c.Detail != "übersprungen; Docker-Lifecycle deaktiviert" {
		t.Fatalf("unexpected Docker check: %#v", c)
	}
}

func TestDoctorDockerLifecycleAutoUnavailableIsWarning(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	cfg := config.Config{RootDir: root, CurrentDir: current, ReleaseRoot: filepath.Join(root, "release"), BackupRoot: filepath.Join(root, "backup"), Docker: config.DockerConfig{Lifecycle: "auto"}}
	c, ok := dockerCheck(Run(context.Background(), root, cfg))
	if !ok || c.Level != LevelWarning {
		t.Fatalf("auto unavailable should warn: %#v", c)
	}
}

func TestDoctorDockerLifecycleRequiredUnavailableIsError(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	cfg := config.Config{RootDir: root, CurrentDir: current, ReleaseRoot: filepath.Join(root, "release"), BackupRoot: filepath.Join(root, "backup"), Docker: config.DockerConfig{Lifecycle: "required"}}
	c, ok := dockerCheck(Run(context.Background(), root, cfg))
	if !ok || c.Level != LevelError {
		t.Fatalf("required unavailable should error: %#v", c)
	}
}

func TestRunProjectValidatesManifestWithoutRuntimeConfig(t *testing.T) {
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
	r := RunProject(root, false)
	if r.ErrorCount() != 0 {
		t.Fatalf("unexpected errors: %#v", r.Checks)
	}
	if r.SchemaVersion != 2 || r.LatestSchemaVersion != 2 {
		t.Fatalf("unexpected schema versions: %#v", r)
	}
	if r.Config != nil {
		t.Fatalf("project doctor must not require runtime config: %#v", r.Config)
	}
}

func TestRunProjectReportsOutdatedSchema(t *testing.T) {
	root := t.TempDir()
	manifest := `schemaVersion: 1
project:
  name: Demo
steps:
  - id: test
    run: echo ok
`
	if err := os.WriteFile(filepath.Join(root, "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	r := RunProject(root, false)
	if r.ErrorCount() != 0 || r.SchemaVersion != 1 || r.WarningCount() == 0 {
		t.Fatalf("expected outdated-schema warning, got %#v", r)
	}
}

func TestRunProjectMigrateUpdatesManifestAndCreatesBackup(t *testing.T) {
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
	r := RunProject(root, true)
	if r.ErrorCount() != 0 || r.Migration == nil || !r.Migration.Changed {
		t.Fatalf("unexpected migration result: %#v", r)
	}
	if r.SchemaVersion != 2 || r.Migration.BackupPath == "" {
		t.Fatalf("migration metadata incomplete: %#v", r.Migration)
	}
	if _, err := os.Stat(r.Migration.BackupPath); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
}

func TestRunProjectMigrateCanonicalizesLegacySetupYAML(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "setup.yaml")
	manifest := `schemaVersion: 1
project:
  name: Demo
steps:
  - id: test
    run: echo ok
`
	if err := os.WriteFile(legacy, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	r := RunProject(root, true)
	if r.ErrorCount() != 0 || r.Migration == nil || !r.Migration.Canonicalized {
		t.Fatalf("unexpected canonicalization: %#v", r)
	}
	if filepath.Base(r.Manifest) != "update-cli.yaml" || r.SchemaVersion != 2 {
		t.Fatalf("canonical manifest not selected: %#v", r)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("legacy setup.yaml must be preserved: %v", err)
	}
}

func TestRunProjectMissingManifestIsError(t *testing.T) {
	r := RunProject(t.TempDir(), false)
	if r.ErrorCount() == 0 {
		t.Fatalf("expected missing-manifest error: %#v", r)
	}
}

func TestRunProjectFromCurrentUsesParentRuntimeConfig(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	downloads := filepath.Join(root, "downloads")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads}); err != nil {
		t.Fatal(err)
	}
	manifest := `schemaVersion: 2
project:
  name: Demo
update:
  sync:
    preserve: [.env, data/]
run:
  command: echo ok
`
	if err := os.WriteFile(filepath.Join(current, "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	r := RunProject(current, false)
	if r.ErrorCount() != 0 {
		t.Fatalf("unexpected errors: %#v", r.Checks)
	}
	if r.Root != root || r.WorkingDirectory != current {
		t.Fatalf("context root=%q working=%q", r.Root, r.WorkingDirectory)
	}
	if r.RuntimeConfig == nil || r.RuntimeConfig.Path != filepath.Join(root, ".update-cli", "config.json") {
		t.Fatalf("runtime config = %#v", r.RuntimeConfig)
	}
	if r.Manifest != filepath.Join(current, "update-cli.yaml") {
		t.Fatalf("manifest = %q", r.Manifest)
	}
}

func TestRunProjectChecksRootAndCurrentManifests(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	rootManifest := `schemaVersion: 2
project:
  name: Demo
update:
  docker:
    lifecycle: disabled
run:
  command: echo root
`
	currentManifest := `schemaVersion: 2
project:
  name: Demo
update:
  docker:
    lifecycle: required
run:
  command: echo current
`
	if err := os.WriteFile(filepath.Join(root, "update-cli.yaml"), []byte(rootManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "update-cli.yaml"), []byte(currentManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	r := RunProject(root, false)
	if r.ErrorCount() != 0 {
		t.Fatalf("unexpected errors: %#v", r.Checks)
	}
	if len(r.Manifests) != 2 || !r.Manifests[0].Exists || !r.Manifests[1].Exists {
		t.Fatalf("manifest statuses = %#v", r.Manifests)
	}
	foundDifference := false
	for _, check := range r.Checks {
		if check.Name == "Update-Konfiguration" && check.Level == LevelWarning {
			foundDifference = true
		}
	}
	if !foundDifference {
		t.Fatalf("expected root/current difference warning: %#v", r.Checks)
	}
}

func TestDoctorReportsAndMigratesRepairableManifestProblems(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, config.ConfigDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `{
  "schemaVersion": 9,
  "projectName": "demo",
  "source": {"type":"download","folder":"$HOME/Downloads"},
  "releaseDir":"release",
  "currentDir":"current"
}`
	if err := os.WriteFile(filepath.Join(root, config.ConfigDirName, config.ConfigFileName), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `schemaVersion: 2
project:
  name: demo
  unknownProjectField: remove
defaults:
  failFast: maybe
  unknownDefault: remove
tasks:
  setup:
    steps:
      - shell: echo ok
        unknownStep: remove
workflows:
  setup:
    tasks: [setup]
`
	path := filepath.Join(root, config.ProjectFileName)
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	report := RunProject(root, false)
	if report.ErrorCount() == 0 {
		t.Fatalf("doctor must report strict manifest error: %#v", report.Checks)
	}
	joined := ""
	for _, check := range report.Checks {
		joined += check.Detail + "\n"
	}
	if !strings.Contains(joined, "unknownProjectField") || !strings.Contains(joined, "automatisch reparierbar") {
		t.Fatalf("repair diagnostics missing: %s", joined)
	}

	report = RunProject(root, true)
	if report.ErrorCount() != 0 {
		t.Fatalf("doctor --migrate should repair known structural problems: %#v", report.Checks)
	}
	if _, err := projectsetup.ParseManifest(path); err != nil {
		t.Fatalf("manifest remains invalid after doctor --migrate: %v", err)
	}
	body, _ := os.ReadFile(path)
	if strings.Contains(string(body), "unknownProjectField") || strings.Contains(string(body), "unknownDefault") || strings.Contains(string(body), "unknownStep") {
		t.Fatalf("manifest was not cleaned: %s", body)
	}
}

func TestInspectMigrationRequirementExplainsCurrentManifestRepair(t *testing.T) {
	root := t.TempDir()
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
	manifest := `schemaVersion: 2
project:
  name: Demo
  unknownProjectField: remove-me
workflows:
  setup:
    tasks: [setup]
tasks:
  setup:
    steps:
      - shell: echo ok
`
	path := filepath.Join(current, config.ProjectFileName)
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	status := InspectMigrationRequirement(root)
	if !status.Required {
		t.Fatalf("current manifest repair must require migration: %#v", status)
	}
	found := false
	for _, reason := range status.Reasons {
		if reason.Scope == "manifest-current" && reason.Path == path && strings.Contains(reason.Detail, "unknownProjectField") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing current-manifest reason: %#v", status.Reasons)
	}

	report := RunProject(root, false)
	if !report.MigrationRequired || len(report.MigrationReasons) == 0 {
		t.Fatalf("doctor report must expose the same migration state: %#v", report)
	}
}

func TestInspectMigrationRequirementReportsNoMigrationForCurrentState(t *testing.T) {
	root := t.TempDir()
	downloads := filepath.Join(root, "downloads")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads}); err != nil {
		t.Fatal(err)
	}
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
	if err := os.WriteFile(filepath.Join(root, config.ProjectFileName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	status := InspectMigrationRequirement(root)
	if status.Required || len(status.Reasons) != 0 {
		t.Fatalf("current config and manifest must not require migration: %#v", status)
	}
	report := RunProject(root, false)
	if report.MigrationRequired || len(report.MigrationReasons) != 0 {
		t.Fatalf("doctor report must say no migration is required: %#v", report)
	}
}

func TestDoctorDoesNotWarnAboutMissingRootManifestWhenCurrentExists(t *testing.T) {
	root := t.TempDir()
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
	manifest := `schemaVersion: 2
project:
  name: Demo
  slug: demo
update:
  mode: update
  source:
    type: download
    folder: ./downloads
  releaseDir: release
  currentDir: current
  backup:
    directory: backup
    keep: 3
  retention:
    releases: 5
  sync:
    preserve: [.env, data/]
  docker:
    lifecycle: auto
workflows:
  setup:
    tasks: [setup]
tasks:
  setup:
    steps:
      - shell: echo ok
`
	if err := os.WriteFile(filepath.Join(current, config.ProjectFileName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	report := RunProject(root, false)
	for _, check := range report.Checks {
		if check.Name == "Manifest root" && check.Level == LevelWarning {
			t.Fatalf("missing root manifest must not warn when current manifest exists: %#v", check)
		}
	}
	if report.ErrorCount() != 0 {
		t.Fatalf("current-only managed project should be valid: %#v", report.Checks)
	}
}
