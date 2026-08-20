package projectsetup

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/r14r/update-cli/lib/ui"
)

func TestParseManifest(t *testing.T) {
	p := filepath.Join(t.TempDir(), "update-cli.yaml")
	data := `version: 1
project:
  name: demo
steps:
  - name: Test
    type: go
    action: test
  - type: deploy
    source: dist/demo
    destination: /usr/local/bin/demo
    mode: "0755"
`
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.ProjectName != "demo" || len(m.Steps) != 2 || m.Steps[1].Mode != "0755" {
		t.Fatalf("unexpected manifest: %#v", m)
	}
}
func TestParseManifestRejectsUnknownField(t *testing.T) {
	p := filepath.Join(t.TempDir(), "update-cli.yaml")
	_ = os.WriteFile(p, []byte("version: 1\nsteps:\n  - type: go\n    magic: yes\n"), 0o644)
	if _, err := ParseManifest(p); err == nil {
		t.Fatal("expected unknown field rejection")
	}
}

func TestRunStandaloneCommand(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "update-cli.yaml")
	data := "version: 1\nsteps:\n  - name: write\n    type: command\n    command: printf ok > result.txt\n"
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RunStandalone(context.Background(), p, ui.New(true)); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "result.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "ok" {
		t.Fatalf("unexpected result %q", b)
	}
}

func TestParseLegacy214Manifest(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "update-cli.yaml")
	data := `schemaVersion: 1

project:
  name: NVIDIA CLI
  type: go
  description: Build and deploy

steps:
  - id: prepare
    name: Prepare
    when: file:go.mod
    run: printf ok > result.txt
    cwd: .
    allowFailure: false
`
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if !m.LegacySchema || m.Version != 1 || m.ProjectName != "NVIDIA CLI" || m.ProjectType != "go" || m.ProjectDescription != "Build and deploy" {
		t.Fatalf("unexpected legacy manifest: %#v", m)
	}
	if len(m.Steps) != 1 || m.Steps[0].Type != "command" || m.Steps[0].Command != "printf ok > result.txt" || m.Steps[0].When != "file:go.mod" {
		t.Fatalf("unexpected translated step: %#v", m.Steps)
	}
}

func TestRunLegacy214ManifestConditionsAndAllowFailure(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "update-cli.yaml")
	data := `schemaVersion: 1
project:
  name: Friendly Display Name
steps:
  - id: skipped
    when: file:missing.file
    run: exit 99
  - id: allowed
    run: exit 7
    allowFailure: true
  - id: write
    when: os:` + runtime.GOOS + `
    run: printf ok > result.txt
`
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := RunStandalone(context.Background(), p, ui.New(true))
	if err != nil {
		t.Fatal(err)
	}
	if res.StepsSkipped != 1 || res.StepsExecuted != 1 {
		t.Fatalf("unexpected result: %#v", res)
	}
	b, err := os.ReadFile(filepath.Join(root, "result.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "ok" {
		t.Fatalf("unexpected result %q", b)
	}
}

func TestParseManifestBlockRun(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "update-cli.yaml")
	data := `schemaVersion: 1
project:
  name: block-test
  type: go
steps:
  - id: block
    name: Block command
    when: file\:go.mod
    run: |
      printf 'first\\n'
      printf 'second\\n'
`
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/block-test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Steps) != 1 {
		t.Fatalf("unexpected steps: %#v", m.Steps)
	}
	if m.Steps[0].When != "file:go.mod" {
		t.Fatalf("escaped colon was not normalized: %q", m.Steps[0].When)
	}
	want := "printf 'first\\\\n'\nprintf 'second\\\\n'"
	if m.Steps[0].Command != want {
		t.Fatalf("unexpected block command:\n got %q\nwant %q", m.Steps[0].Command, want)
	}
}

func TestParseManifestBlockRunRequiresIndentation(t *testing.T) {
	p := filepath.Join(t.TempDir(), "update-cli.yaml")
	data := `schemaVersion: 1
steps:
  - id: bad
    run: |
    echo not-indented
`
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseManifest(p)
	if err == nil || !strings.Contains(err.Error(), "stärker eingerückten Befehlsblock") {
		t.Fatalf("expected indentation diagnostic, got %v", err)
	}
}

func TestParseManifestAcceptsProjectSlug(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "update-cli.yaml")
	data := `schemaVersion: 1
project:
  name: Demo
  slug: demo-cli
steps:
  - id: test
    name: Test
    run: echo ok
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.ProjectSlug != "demo-cli" {
		t.Fatalf("slug = %q", m.ProjectSlug)
	}
}

func TestParseStructuredSchema1ManifestWithVersionMap(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "update-cli.yaml")
	manifest := `schemaVersion: 1

project:
  name: x-cli
  slug: x-cli
  description: x-cli prüfen, testen und bauen

version:
  file: VERSION
  required: true
  pattern: '^[0-9]+\.[0-9]+\.[0-9]+$'

build:
  configFile: ''
  distDir: bin
  binaryName: x-cli

runtime:
  requiredCommands:
    - go
  optionalCommands:
    - just

go:
  package: ./...
  buildPackage: ./cmd/x-cli
  ldflagsTemplate: >-
    -s -w
    -X github.com/r14r/x-cli/internal/version.Version={{VERSION}}
    -X github.com/r14r/x-cli/internal/version.Commit={{COMMIT}}
    -X github.com/r14r/x-cli/internal/version.Date={{BUILD_DATE}}

setup:
  steps:
    - pre-commands
    - go-mod-download
    - go-vet
    - go-test
    - custom-commands
    - go-build
    - binary-version-check
    - post-commands

commands:
  pre:
    - gofmt -w cmd internal
  just: []
  custom: []
  post:
    - ./bin/x-cli doctor
    - >-
      if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
        install -m 0755 ./bin/x-cli /usr/local/bin/x-cli;
      else
        sudo mkdir -p /usr/local/bin && sudo install -m 0755 ./bin/x-cli /usr/local/bin/x-cli;
      fi
    - /usr/local/bin/x-cli --version
    - /usr/local/bin/x-cli doctor
`
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != 1 || !m.LegacySchema || m.ProjectSlug != "x-cli" || m.ProjectVersion != "1.2.3" {
		t.Fatalf("unexpected manifest metadata: %#v", m)
	}
	if len(m.Steps) != 8 {
		t.Fatalf("steps = %d, want 8: %#v", len(m.Steps), m.Steps)
	}
	if m.Steps[0].ID != "pre-commands" || !strings.Contains(m.Steps[0].Command, "gofmt -w cmd internal") {
		t.Fatalf("unexpected pre step: %#v", m.Steps[0])
	}
	build := m.Steps[5].Command
	for _, marker := range []string{"./cmd/x-cli", "bin/x-cli", "${VERSION_VALUE}", "${COMMIT_VALUE}", "${BUILD_DATE_VALUE}"} {
		if !strings.Contains(build, marker) {
			t.Fatalf("build command missing %q:\n%s", marker, build)
		}
	}
	post := m.Steps[7].Command
	if !strings.Contains(post, "install -m 0755 ./bin/x-cli /usr/local/bin/x-cli") || !strings.Contains(post, "/usr/local/bin/x-cli doctor") {
		t.Fatalf("unexpected post command:\n%s", post)
	}
}

func TestRunStructuredSchema1CommandGroups(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "update-cli.yaml")
	manifest := `schemaVersion: 1
project:
  name: demo
  slug: demo
version:
  file: VERSION
  required: true
  pattern: '^[0-9]+\.[0-9]+\.[0-9]+$'
build:
  distDir: bin
  binaryName: demo
runtime:
  requiredCommands: []
  optionalCommands: []
setup:
  steps:
    - pre-commands
    - custom-commands
    - post-commands
commands:
  pre:
    - printf pre > pre.txt
  custom:
    - printf custom > custom.txt
  post:
    - printf post > post.txt
`
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := RunStandalone(context.Background(), path, ui.New(true))
	if err != nil {
		t.Fatal(err)
	}
	if res.StepsExecuted != 3 {
		t.Fatalf("steps executed = %d, want 3", res.StepsExecuted)
	}
	for _, name := range []string{"pre.txt", "custom.txt", "post.txt"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("%s not created: %v", name, err)
		}
	}
}

func TestParseManifestV2UpdateRepositorySource(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "update-cli.yaml")
	manifest := `schemaVersion: 2
update:
  mode: pull
  source:
    type: repository
    repository: https://github.com/r14r/update-cli.git
    ref: main
    commit: abc123
    version: 1.5.0
    sha256: deadbeef
run:
  command: echo ok
`
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Update.Configured || m.Update.Mode != "pull" || m.Update.Source.Type != "repository" {
		t.Fatalf("update = %#v", m.Update)
	}
	if m.Update.Source.Repository != "https://github.com/r14r/update-cli.git" || m.Update.Source.Ref != "main" {
		t.Fatalf("source = %#v", m.Update.Source)
	}
}

func TestParseManifestV2UpdateSourceValidation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "update-cli.yaml")
	manifest := `schemaVersion: 2
update:
  mode: update
  source:
    type: repository
    repository: https://example.invalid/demo.git
run:
  command: echo ok
`
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseManifest(path); err == nil {
		t.Fatal("expected incompatible update mode/source error")
	}
}
