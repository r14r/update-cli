package projectsetup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/r14r/update-cli/lib/ui"
)

func TestRunApplicationInDirectory(t *testing.T) {
	root := t.TempDir()
	manifest := `schemaVersion: 2
project:
  name: demo
run:
  command: printf '%s' "$RUN_VALUE" > result.txt
  cwd: .
  env:
    RUN_VALUE: works
`
	if err := os.WriteFile(filepath.Join(root, "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	console := ui.New(true)
	console.SuppressFinalStatus(true)
	if err := RunApplicationInDirectory(context.Background(), root, console); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "result.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "works" {
		t.Fatalf("got %q, want works", got)
	}
}

func TestRunApplicationRequiresCommand(t *testing.T) {
	root := t.TempDir()
	manifest := `schemaVersion: 2
project:
  name: demo
workflows:
  setup:
    tasks: [setup]
tasks:
  setup:
    steps:
      - name: noop
        shell: "true"
`
	if err := os.WriteFile(filepath.Join(root, "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	console := ui.New(true)
	console.SuppressFinalStatus(true)
	if err := RunApplicationInDirectory(context.Background(), root, console); err == nil {
		t.Fatal("expected missing run.command error")
	}
}

func TestFindManifestUsesUpdateCLIFilename(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "setup.yaml"), []byte("schemaVersion: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := FindManifest(root); err != nil || ok {
		t.Fatalf("legacy setup.yaml must not be discovered: ok=%v err=%v", ok, err)
	}
	path := filepath.Join(root, "update-cli.yaml")
	if err := os.WriteFile(path, []byte("schemaVersion: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok, err := FindManifest(root)
	if err != nil || !ok || got != path {
		t.Fatalf("got path=%q ok=%v err=%v", got, ok, err)
	}
}

func TestParseRunScalar(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "update-cli.yaml")
	if err := os.WriteFile(path, []byte("schemaVersion: 2\nrun: echo hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Run.Command != "echo hello" {
		t.Fatalf("run command = %q", m.Run.Command)
	}
}

func TestRunApplicationStructuredSteps(t *testing.T) {
	root := t.TempDir()
	manifest := `schemaVersion: 2
project:
  name: demo
run:
  description: Start Streamlit app
  env:
    RUN_VALUE: works
  steps:
    - name: Start app
      command:
        exec: sh
        args:
          - -c
          - printf '%s' "$RUN_VALUE" > result.txt
`
	if err := os.WriteFile(filepath.Join(root, "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	console := ui.New(true)
	console.SuppressFinalStatus(true)
	if err := RunApplicationInDirectory(context.Background(), root, console); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "result.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "works" {
		t.Fatalf("got %q, want works", got)
	}
}

func TestParseRunStructuredStreamlitCommand(t *testing.T) {
	root := t.TempDir()
	manifest := `schemaVersion: 2
run:
  description: Start Streamlit app
  steps:
    - name: Start Streamlit
      command:
        exec: .venv/bin/streamlit
        args:
          - run
          - app/app.py
`
	path := filepath.Join(root, "update-cli.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Run.Description != "Start Streamlit app" || len(m.Run.Steps) != 1 {
		t.Fatalf("run = %#v", m.Run)
	}
	step := m.Run.Steps[0]
	if step.Operation != "command" || step.Config["exec"] != ".venv/bin/streamlit" {
		t.Fatalf("step = %#v", step)
	}
}

func TestRunRejectsCommandAndStepsTogether(t *testing.T) {
	root := t.TempDir()
	manifest := `schemaVersion: 2
run:
  command: echo short
  steps:
    - command:
        exec: echo
        args: [structured]
`
	path := filepath.Join(root, "update-cli.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseManifest(path); err == nil {
		t.Fatal("expected command/steps conflict")
	}
}
