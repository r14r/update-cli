package updater

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r14r/update-cli/src/lib/config"
)

// The full updater must never activate a release rejected by preactivate.
func TestUpdateFailedCandidateQualityPreservesCurrent(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := config.Init(root, config.InitOptions{ProjectName: "candidate-demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	old := releaseZip(t, downloads, "candidate-demo", "1.0.0", map[string]string{"app.txt": "old"})
	if err := Run(context.Background(), "1.9.0", []string{"--update", old, "--root", root, "--no-setup", "--noui"}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.CurrentDir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.CurrentDir, "data", "persist.txt"), []byte("important"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := "schemaVersion: 2\nproject:\n  name: candidate-demo\n  slug: candidate-demo\n  type: generic\nworkflows:\n  preactivate:\n    tasks: [test]\ntasks:\n  test:\n    steps:\n      - id: fail\n        shell: 'exit 17'\n"
	next := releaseZip(t, downloads, "candidate-demo", "2.0.0", map[string]string{
		"app.txt": "new", "update-cli.yaml": manifest,
	})
	err = Run(context.Background(), "1.9.0", []string{"--update", next, "--root", root, "--no-setup", "--noui"})
	if err == nil || !strings.Contains(err.Error(), "preactivate") {
		t.Fatalf("expected preactivation failure, got %v", err)
	}
	for file, want := range map[string]string{"VERSION": "1.0.0\n", "app.txt": "old", "data/persist.txt": "important"} {
		got, readErr := os.ReadFile(filepath.Join(cfg.CurrentDir, file))
		if readErr != nil || string(got) != want {
			t.Fatalf("live %s after failure: %q, %v", file, got, readErr)
		}
	}
}
