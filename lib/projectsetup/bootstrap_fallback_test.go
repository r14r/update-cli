package projectsetup

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/ui"
)

// Regression for 'u': Update CLI 3.x invokes bootstrap after a no-argument
// update, but a previously installed schemaVersion-2 manifest may lack the
// new bootstrap workflow while still having the project's justfile.
func TestMissingBootstrapWorkflowFallsBackToJustRecipe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture only works on Unix hosts")
	}
	workdir := t.TempDir()
	manifest := filepath.Join(workdir, "update-cli.yaml")
	content := "schemaVersion: 2\nworkflows:\n  setup:\n    tasks: [prepare]\ntasks:\n  prepare:\n    steps:\n      - shell: echo setup\n"
	if err := os.WriteFile(manifest, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workdir, "justfile"), []byte("bootstrap:\n\t@echo bootstrap\n"), 0600); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	just := filepath.Join(bin, "just")
	if err := os.WriteFile(just, []byte("#!/bin/sh\nprintf '%s\\n' \"$PWD:$1\" > \"$BOOTSTRAP_RESULT\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(workdir, "result.txt")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BOOTSTRAP_RESULT", resultPath)
	console := ui.New(true)
	console.SetDirect(true)
	got, err := runManifestSelected(context.Background(), config.Config{CurrentDir: workdir}, console, manifest, Selection{Workflow: "bootstrap"})
	if err != nil {
		t.Fatal(err)
	}
	if got.StepsExecuted != 1 {
		t.Fatalf("expected one bootstrap step, got %+v", got)
	}
	result, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(result)), ":bootstrap") || !strings.HasPrefix(string(result), workdir+":") {
		t.Fatalf("unexpected fallback command: %s", result)
	}
}

func TestMissingNonBootstrapWorkflowStillFails(t *testing.T) {
	workdir := t.TempDir()
	manifest := filepath.Join(workdir, "update-cli.yaml")
	content := "schemaVersion: 2\nworkflows:\n  setup:\n    tasks: [prepare]\ntasks:\n  prepare:\n    steps:\n      - shell: echo setup\n"
	if err := os.WriteFile(manifest, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workdir, "justfile"), []byte("bootstrap:\n\t@echo bootstrap\n"), 0600); err != nil {
		t.Fatal(err)
	}
	console := ui.New(true)
	console.SetDirect(true)
	_, err := runManifestSelected(context.Background(), config.Config{CurrentDir: workdir}, console, manifest, Selection{Workflow: "deployment"})
	if err == nil || !strings.Contains(err.Error(), "unbekannter setup workflow") {
		t.Fatalf("unknown workflow must remain an error, got %v", err)
	}
}

func TestMissingBootstrapWithoutJustfileStillFails(t *testing.T) {
	workdir := t.TempDir()
	manifest := filepath.Join(workdir, "update-cli.yaml")
	content := "schemaVersion: 2\nworkflows:\n  setup:\n    tasks: [prepare]\ntasks:\n  prepare:\n    steps:\n      - shell: echo setup\n"
	if err := os.WriteFile(manifest, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	console := ui.New(true)
	console.SetDirect(true)
	_, err := runManifestSelected(context.Background(), config.Config{CurrentDir: workdir}, console, manifest, Selection{Workflow: "bootstrap"})
	if err == nil || !strings.Contains(err.Error(), "unbekannter setup workflow") {
		t.Fatalf("missing workflow without Justfile must fail, got %v", err)
	}
}
