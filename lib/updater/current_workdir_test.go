package updater

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureSafeUpdateWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	nested := filepath.Join(current, "backend")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	for _, tc := range []struct {
		name      string
		cwd       string
		wantError bool
	}{
		{"root", root, false},
		{"current", current, true},
		{"nested current", nested, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.Chdir(tc.cwd); err != nil {
				t.Fatal(err)
			}
			err := ensureSafeUpdateWorkingDirectory(current)
			if tc.wantError {
				if err == nil || !strings.Contains(err.Error(), "atomaren Update") {
					t.Fatalf("expected actionable current directory error, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("root working directory should be allowed: %v", err)
			}
		})
	}
}
