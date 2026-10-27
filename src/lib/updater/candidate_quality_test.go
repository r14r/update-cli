package updater

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/r14r/update-cli/src/lib/config"
	"github.com/r14r/update-cli/src/lib/ui"
)

func TestCandidateQualityGateFailurePreservesLiveCurrent(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	stage := filepath.Join(root, "current.next-test")
	for _, dir := range []string{current, stage} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(current, "VERSION"), []byte("1.8.0\\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "data.txt"), []byte("persistent"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := "schemaVersion: 2\\nproject:\\n  name: Demo\\n  slug: demo\\n  type: generic\\nworkflows:\\n  preactivate:\\n    tasks: [test]\\ntasks:\\n  test:\\n    steps:\\n      - id: fail\\n        shell: 'exit 17'\\n"
	if err := os.WriteFile(filepath.Join(stage, "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{RootDir: root, ConfigDir: filepath.Join(root, ".update-cli"), CurrentDir: stage}
	progress := newUpdateProgress(context.Background(), ui.New(true), false, 1)
	err := runCandidateQualityGate(context.Background(), cfg, ui.New(true), progress)
	if err == nil {
		t.Fatal("failing candidate quality workflow must prevent activation")
	}
	got, readErr := os.ReadFile(filepath.Join(current, "data.txt"))
	if readErr != nil || string(got) != "persistent" {
		t.Fatalf("live current modified by candidate quality check: %q, %v", got, readErr)
	}
}

func TestCandidateQualityGateWithoutManifestIsOptional(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, "current.next-test")
	if err := os.MkdirAll(stage, 0o755); err != nil {
		t.Fatal(err)
	}
	progress := newUpdateProgress(context.Background(), ui.New(true), false, 1)
	if err := runCandidateQualityGate(context.Background(), config.Config{CurrentDir: stage}, ui.New(true), progress); err != nil {
		t.Fatalf("candidate without manifest should be compatible: %v", err)
	}
	if progress.current != 1 {
		t.Fatalf("missing progress step: got %d", progress.current)
	}
}

func TestCandidateQualityGateWithoutPreactivateWorkflowIsOptional(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, "current.next-test")
	if err := os.MkdirAll(stage, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "schemaVersion: 2\nproject:\n  name: Demo\n  slug: demo\n  type: generic\nworkflows:\n  test:\n    tasks: [test]\ntasks:\n  test:\n    steps:\n      - id: ok\n        shell: 'true'\n"
	if err := os.WriteFile(filepath.Join(stage, "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	progress := newUpdateProgress(context.Background(), ui.New(true), false, 1)
	if err := runCandidateQualityGate(context.Background(), config.Config{CurrentDir: stage}, ui.New(true), progress); err != nil {
		t.Fatalf("undeclared quality workflow should not be run: %v", err)
	}
	if progress.current != 1 {
		t.Fatalf("missing progress step: got %d", progress.current)
	}
}


// A successful build must leave its output inside the candidate so the
// subsequent directory activation can publish it without rebuilding in current.
func TestCandidateQualityGateBuildsInsideStage(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, "current.next-test")
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(stage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "sentinel.txt"), []byte("untouched"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := "schemaVersion: 2\nproject:\n  name: Demo\n  slug: demo\n  type: generic\nworkflows:\n  preactivate:\n    tasks: [build]\ntasks:\n  build:\n    steps:\n      - id: compile\n        shell: 'printf staged > compiled.txt'\n"
	if err := os.WriteFile(filepath.Join(stage, "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{RootDir: root, ConfigDir: filepath.Join(root, ".update-cli"), CurrentDir: stage}
	progress := newUpdateProgress(context.Background(), ui.New(true), false, 1)
	if err := runCandidateQualityGate(context.Background(), cfg, ui.New(true), progress); err != nil {
		t.Fatalf("candidate build failed: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(stage, "compiled.txt"))
	if err != nil || string(b) != "staged" {
		t.Fatalf("build output missing from staged candidate: %q, %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(current, "compiled.txt")); !os.IsNotExist(err) {
		t.Fatalf("candidate build leaked into live current: %v", err)
	}
}
