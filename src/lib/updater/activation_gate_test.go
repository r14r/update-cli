package updater

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActivationCandidateGate(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	stage := filepath.Join(root, "current.next-test")
	if err := os.MkdirAll(stage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "VERSION"), []byte("3.5.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateActivationCandidate(stage, current, "3.5.0"); err != nil {
		t.Fatalf("valid isolated candidate rejected: %v", err)
	}
	tests := []struct {
		name, stage, version string
	}{
		{"same as current", current, "3.5.0"},
		{"wrong version", stage, "3.4.42"},
		{"missing stage", filepath.Join(root, "absent"), "3.5.0"},
		{"nested in current", filepath.Join(current, "candidate"), "3.5.0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateActivationCandidate(tc.stage, current, tc.version); err == nil {
				t.Fatal("expected activation gate failure")
			}
		})
	}
}

func TestActivationGateRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, "stage")
	real := filepath.Join(root, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "VERSION"), []byte("3.5.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, stage); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	err := validateActivationCandidate(stage, filepath.Join(root, "current"), "3.5.0")
	if err == nil || !strings.Contains(err.Error(), "echtes Verzeichnis") {
		t.Fatalf("symlink candidate unexpectedly accepted: %v", err)
	}
}
