package projectsetup

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"release-updater/lib/config"
	"release-updater/lib/ui"
)

func TestRunExecutesScriptBeforeConfiguredCommands(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	setup := "#!/usr/bin/env bash\nprintf 'script\\n' >> order.log\n"
	if err := os.WriteFile(filepath.Join(current, "setup.sh"), []byte(setup), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		ProjectName:   "demo",
		CurrentDir:    current,
		SetupCommands: []string{"printf 'first\\n' >> order.log", "printf 'second\\n' >> order.log"},
	}
	result, err := Run(context.Background(), cfg, ui.New(true))
	if err != nil {
		t.Fatal(err)
	}
	if !result.ScriptExecuted || result.CommandsExecuted != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
	data, err := os.ReadFile(filepath.Join(current, "order.log"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(data)); got != "script\nfirst\nsecond" {
		t.Fatalf("unexpected execution order: %q", got)
	}
}

func TestRunRequiresCurrentDirectory(t *testing.T) {
	_, err := Run(context.Background(), config.Config{CurrentDir: filepath.Join(t.TempDir(), "missing")}, ui.New(true))
	if err == nil || !strings.Contains(err.Error(), "Current-Ordner fehlt") {
		t.Fatalf("expected missing current error, got %v", err)
	}
}
