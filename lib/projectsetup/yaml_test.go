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
	p := filepath.Join(t.TempDir(), "setup.yaml")
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
	p := filepath.Join(t.TempDir(), "setup.yaml")
	_ = os.WriteFile(p, []byte("version: 1\nsteps:\n  - type: go\n    magic: yes\n"), 0o644)
	if _, err := ParseManifest(p); err == nil {
		t.Fatal("expected unknown field rejection")
	}
}

func TestRunStandaloneCommand(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "setup.yaml")
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
	p := filepath.Join(root, "setup.yaml")
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
	p := filepath.Join(root, "setup.yaml")
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
	p := filepath.Join(root, "setup.yaml")
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
	p := filepath.Join(t.TempDir(), "setup.yaml")
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
