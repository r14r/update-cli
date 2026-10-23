package projectsetup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertManifestToLatest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-cli.yaml")
	legacy := `schemaVersion: 1
project:
  name: Demo
  type: go
steps:
  - id: modules
    name: Modules
    when: file:go.mod
    run: go mod download
  - id: test
    name: Tests
    run: go test ./...
`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := ConvertManifestToLatest(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed || res.PreviousSchema != 1 || res.CurrentSchema != 2 || res.BackupPath == "" {
		t.Fatalf("unexpected result: %#v", res)
	}
	m, err := ParseManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != 2 {
		t.Fatalf("version=%d", m.Version)
	}
	if _, ok := m.Workflows["setup"]; !ok {
		t.Fatal("setup workflow missing")
	}
	task := m.Tasks["setup"]
	if len(task.Steps) != 2 {
		t.Fatalf("steps=%d", len(task.Steps))
	}
	if task.Steps[0].When == nil || task.Steps[0].When.Kind != "fileExists" {
		t.Fatalf("condition not converted: %#v", task.Steps[0].When)
	}
}

func TestGenerateManifestDetectsProjectKinds(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"go", map[string]string{"go.mod": "module example.com/demo\n"}, []string{"go"}},
		{"python", map[string]string{"requirements.txt": "pytest\n"}, []string{"python"}},
		{"node", map[string]string{"package.json": `{"name":"demo-node","scripts":{"test":"echo ok","build":"echo build","lint":"echo lint"}}`, "package-lock.json": "{}"}, []string{"node"}},
		{"laravel", map[string]string{"artisan": "#!/usr/bin/env php\n", "composer.json": `{"require":{"laravel/framework":"^12"}}`}, []string{"laravel"}},
		{"docker", map[string]string{"compose.yaml": "services: {}\n"}, []string{"docker"}},
		{"mixed", map[string]string{"go.mod": "module example.com/mixed\n", "package.json": `{"name":"mixed","scripts":{"build":"echo ok"}}`, "compose.yaml": "services: {}\n"}, []string{"go", "node", "docker"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tc.files {
				p := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			res, err := GenerateManifest(dir, "", false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseManifest(res.Path); err != nil {
				t.Fatalf("generated manifest invalid: %v", err)
			}
			got := strings.Join(res.Technologies, ",")
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Fatalf("technologies=%q want %q", got, want)
				}
			}
		})
	}
}

func TestGenerateManifestRefusesOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-cli.yaml")
	if err := os.WriteFile(path, []byte("existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := GenerateManifest(dir, "", false); err == nil {
		t.Fatal("expected overwrite protection")
	}
}

func TestGenerateSetupScript(t *testing.T) {
	dir := t.TempDir()
	res, err := GenerateSetupScript(dir, "", false)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatal("setup.sh is not executable")
	}
	data, _ := os.ReadFile(res.Path)
	text := string(data)
	if !strings.Contains(text, "setup --manifest") || !strings.Contains(text, "go run") {
		t.Fatalf("unexpected script:\n%s", text)
	}
}

func TestPreviewGeneratedManifestFromSetupScriptLegacyTemplate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `#!/usr/bin/env bash
PROJECT_NAME="Demo CLI"
PROJECT_DESCRIPTION="Demo bauen"
DIST_DIR="bin"
BINARY_NAME="demo"
GO_PACKAGE="./..."
GO_BUILD_PACKAGE="./cmd/demo"
SETUP_STEPS=(
  "pre-commands"
  "go-mod-download"
  "go-vet"
  "go-test"
  "go-build"
  "binary-version-check"
  "post-commands"
)
PRE_COMMANDS=("gofmt -w cmd internal")
POST_COMMANDS=("./bin/demo doctor")
`
	path := filepath.Join(dir, "setup.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	text, tech, analysis, err := PreviewGeneratedManifestFromSetupScript(dir, path)
	if err != nil {
		t.Fatal(err)
	}
	if !analysis.Legacy || analysis.Steps != 7 {
		t.Fatalf("analysis=%#v", analysis)
	}
	if !strings.Contains(strings.Join(tech, ","), "go") {
		t.Fatalf("tech=%v", tech)
	}
	if !strings.Contains(text, "schemaVersion: 2") || !strings.Contains(text, "Go-Module laden") || !strings.Contains(text, "./bin/demo doctor") {
		t.Fatalf("unexpected manifest:\n%s", text)
	}
	tmp := filepath.Join(dir, "generated.yaml")
	if err := os.WriteFile(tmp, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseManifest(tmp); err != nil {
		t.Fatalf("generated manifest invalid: %v\n%s", err, text)
	}
}

func TestPreviewGeneratedManifestFromSimpleSetupScript(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `#!/usr/bin/env bash
set -e
go mod download
gofmt -w .
go vet ./...
go test ./...
go build -o demo .
./demo --version
`
	path := filepath.Join(dir, "setup.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	text, _, analysis, err := PreviewGeneratedManifestFromSetupScript(dir, path)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Steps < 5 || analysis.Legacy {
		t.Fatalf("analysis=%#v", analysis)
	}
	if !strings.Contains(text, "Go-Tests ausführen") || !strings.Contains(text, "go build -o demo .") {
		t.Fatalf("unexpected manifest:\n%s", text)
	}
}

func TestMigrateProjectManifestCanonicalizesSetupYAML(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "setup.yaml")
	data := `schemaVersion: 1
project:
  name: Demo
steps:
  - id: test
    run: echo ok
`
	if err := os.WriteFile(legacy, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := MigrateProjectManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed || !res.Canonicalized || res.PreviousSchema != 1 || res.CurrentSchema != SchemaVersion {
		t.Fatalf("unexpected result: %#v", res)
	}
	if filepath.Base(res.Path) != "update-cli.yaml" {
		t.Fatalf("unexpected target: %s", res.Path)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("legacy source should remain untouched: %v", err)
	}
	m, err := ParseManifest(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != SchemaVersion {
		t.Fatalf("version=%d", m.Version)
	}
}

func TestRepairProjectManifestRemovesUnknownAndNormalizesValues(t *testing.T) {
	root := t.TempDir()
	manifest := `schemaVersion: 2
unknownTop: true
project:
  name: demo
  obsolete: value
defaults:
  failFast: definitely-not-bool
  obsolete: true
tasks:
  setup:
    obsolete: value
    steps:
      - name: Build
        unknownStep: true
        shell: echo ok
workflows:
  setup:
    tasks: [setup, missing]
`
	path := filepath.Join(root, "update-cli.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := RepairProjectManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Fatal("expected manifest repair")
	}
	if result.BackupPath == "" {
		t.Fatal("expected manifest backup")
	}
	if _, err := ParseManifest(path); err != nil {
		t.Fatalf("repaired manifest invalid: %v", err)
	}
	body, _ := os.ReadFile(path)
	for _, bad := range []string{"unknownTop", "obsolete:", "unknownStep", "missing"} {
		if strings.Contains(string(body), bad) {
			t.Fatalf("manifest still contains %q:\n%s", bad, body)
		}
	}
}

func TestRepairProjectManifestMigratesTransitionalUpdateSetupPolicy(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "update-cli.yaml")
	manifest := `schemaVersion: 2
update:
  sync:
    preserve: [.env]
  setup:
    keepRsyncOnError: true
run:
  command: echo ok
`
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := RepairProjectManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Fatal("expected transitional policy migration")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if strings.Contains(text, "\n  setup:") || strings.Contains(text, "keepRsyncOnError") {
		t.Fatalf("transitional setup policy still present:\n%s", text)
	}
	m, err := ParseManifest(path)
	if err != nil {
		t.Fatalf("migrated manifest invalid: %v", err)
	}
	if m.Update.Sync.KeepOnSetupError == nil || !*m.Update.Sync.KeepOnSetupError {
		t.Fatalf("canonical sync policy missing after migration:\n%s", text)
	}
}

func TestInspectManifestDoesNotTreatFormattingOnlyDifferenceAsMigration(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "update-cli.yaml")
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
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	inspection := InspectManifest(path)
	if !inspection.Valid {
		t.Fatalf("current manifest must be valid: %#v", inspection)
	}
	if inspection.Repairable {
		t.Fatalf("formatting-only canonical renderer difference must not require migration: %#v", inspection)
	}
	if len(inspection.RemovedFields) != 0 || len(inspection.Normalized) != 0 {
		t.Fatalf("unexpected structural diagnostics: %#v", inspection)
	}
}

func TestInspectManifestReportsAllRepairableProblemsWithoutChangingFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "update-cli.yaml")
	original := `schemaVersion: 2
project:
  name: demo
  magic: remove-me
defaults:
  failFast: maybe
  obsolete: yes
update:
  mode: invalid
  sync:
    preserve: [.env]
    unknownSync: true
tasks:
  build:
    strangeTaskField: yes
    steps:
      - shell: echo ok
        alienStepField: yes
workflows:
  setup:
    tasks: [build]
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	inspection := InspectManifest(path)
	if inspection.Valid {
		t.Fatal("invalid manifest unexpectedly reported valid")
	}
	if !inspection.Repairable {
		t.Fatalf("manifest should be repairable: %#v", inspection)
	}
	joined := strings.Join(inspection.RemovedFields, "|")
	for _, want := range []string{"project.magic", "defaults.obsolete", "update.sync.unknownSync", "tasks.build.strangeTaskField"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing diagnostic %q in %#v", want, inspection.RemovedFields)
		}
	}
	normalized := strings.Join(inspection.Normalized, "|")
	if !strings.Contains(normalized, "defaults.failFast") || !strings.Contains(normalized, "update.mode") {
		t.Fatalf("missing normalization diagnostics: %#v", inspection.Normalized)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != original {
		t.Fatal("inspection modified original manifest")
	}
}

func TestParseManifestForUseIncludesRepairCommand(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "update-cli.yaml")
	manifest := `schemaVersion: 2
project:
  name: demo
  wrongField: value
tasks:
  build:
    steps:
      - shell: echo ok
workflows:
  setup:
    tasks: [build]
`
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseManifestForUse(path)
	if err == nil {
		t.Fatal("expected parser error")
	}
	text := err.Error()
	if !strings.Contains(text, "project.wrongField") || !strings.Contains(text, "update-cli fix") {
		t.Fatalf("repair diagnostics missing: %s", text)
	}
}
