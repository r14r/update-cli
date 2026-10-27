package updater

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/r14r/update-cli/src/lib/config"
	"github.com/r14r/update-cli/src/lib/ui"
)

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
