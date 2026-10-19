package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallShellWrapperBackupsAndIdempotence(t *testing.T) {
	for _, tc := range []struct {
		shell string
		rc    string
	}{
		{"bash", ".bashrc"},
		{"zsh", ".zshrc"},
		{"csh", ".cshrc"},
	} {
		t.Run(tc.shell, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("SHELL", "/bin/"+tc.shell)
			rc := filepath.Join(home, tc.rc)
			if err := os.WriteFile(rc, []byte("# existing settings\n"), 0600); err != nil {
				t.Fatal(err)
			}
			savedStdin := os.Stdin
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Stdin = savedStdin; _ = r.Close(); _ = w.Close() })
			os.Stdin = r
			if _, err := w.WriteString("\n\n"); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if err := installShellWrapper(nil); err != nil {
					t.Fatalf("attempt %d: %v", i, err)
				}
			}
			content, err := os.ReadFile(rc)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(content), "# >>> update-cli wrapper >>>") != 1 {
				t.Fatalf("duplicate wrapper installation: %s", content)
			}
			if !strings.Contains(string(content), "# existing settings") {
				t.Fatalf("original settings lost: %s", content)
			}
			backups, err := filepath.Glob(rc + ".bak.*")
			if err != nil || len(backups) != 1 {
				t.Fatalf("expected one backup, got %v: %v", backups, err)
			}
			original, err := os.ReadFile(backups[0])
			if err != nil || string(original) != "# existing settings\n" {
				t.Fatalf("backup differs: %q %v", original, err)
			}
		})
	}
}
