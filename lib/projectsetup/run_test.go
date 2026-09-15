package projectsetup

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/r14r/update-cli/lib/config"
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

func TestFindManifestPrefersUpdateCLIAndFallsBackToSetupYAML(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "setup.yaml")
	if err := os.WriteFile(legacy, []byte("schemaVersion: 2\nrun: echo legacy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok, err := FindManifest(root)
	if err != nil || !ok || got != legacy {
		t.Fatalf("legacy fallback path=%q ok=%v err=%v", got, ok, err)
	}
	current := filepath.Join(root, "update-cli.yaml")
	if err := os.WriteFile(current, []byte("schemaVersion: 2\nrun: echo current\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok, err = FindManifest(root)
	if err != nil || !ok || got != current {
		t.Fatalf("current path=%q ok=%v err=%v", got, ok, err)
	}
}

func TestLegacySetupYAMLWithoutSchemaInfersV2(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "setup.yaml")
	data := "project:\n  name: Demo\nrun:\n  command: echo hello\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != 2 || m.Run.Command != "echo hello" {
		t.Fatalf("unexpected manifest: %#v", m)
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

func TestDetectUsesJustfileAsSetupFallback(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "justfile"), []byte("build:\n    true\n\ninstall:\n    true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{CurrentDir: root}
	path, ok, err := Detect(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || filepath.Base(path) != "justfile" {
		t.Fatalf("path=%q ok=%v", path, ok)
	}
}

func TestSetupFallsBackToJustBuildAndInstall(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "justfile"), []byte("build:\n    true\n\ninstall:\n    true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	logFile := filepath.Join(root, "just.log")
	justPath := filepath.Join(bin, "just")
	script := "#!/bin/sh\nprintf '%s\\n' \"$1\" >> " + strconv.Quote(logFile) + "\n"
	if err := os.WriteFile(justPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	console := ui.New(true)
	console.SuppressFinalStatus(true)
	cfg := config.Config{ProjectName: "demo", CurrentDir: root}
	result, err := RunSelected(context.Background(), cfg, console, Selection{})
	if err != nil {
		t.Fatal(err)
	}
	if result.LegacyCommandsExecuted != 2 {
		t.Fatalf("fallback commands = %d", result.LegacyCommandsExecuted)
	}
	b, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "build\ninstall\n" {
		t.Fatalf("just calls = %q", b)
	}
}

func TestLegacySetupYAMLWithoutSchemaInfersV1(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "setup.yaml")
	data := "project:\n  name: Demo\nsteps:\n  - name: Build\n    run: echo build\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != 1 || len(m.Steps) != 1 || m.Steps[0].Command != "echo build" {
		t.Fatalf("unexpected legacy manifest: %#v", m)
	}
}
