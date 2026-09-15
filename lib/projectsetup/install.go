package projectsetup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/r14r/update-cli/lib/ui"
)

// RunJustInstall executes the canonical `just install` recipe for a project.
// For managed Update CLI projects, current/ is preferred when it contains a
// justfile. Source projects without current/ can run directly from the project root.
func RunJustInstall(ctx context.Context, root string, console *ui.Console) error {
	work, err := justInstallDirectory(root)
	if err != nil {
		return err
	}
	just, err := exec.LookPath("just")
	if err != nil {
		return errors.New("Kommando 'just' ist nicht installiert")
	}

	console.Header("Projekt installieren")
	console.Row("Arbeitsordner", work)
	console.Row("Kommando", "just install")

	cmd := exec.CommandContext(ctx, just, "install")
	cmd.Dir = work
	cmd.Env = append(os.Environ(), "UPDATE_CLI_INSTALL_RUNNING=1")
	cmd.Stdin = os.Stdin
	stdout, stderr := console.ProcessWriters()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("just install fehlgeschlagen: %w", err)
	}
	return nil
}

func justInstallDirectory(root string) (string, error) {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("Projektordner fehlt oder ist ungültig: %s", root)
	}
	candidates := []string{filepath.Join(root, "current"), root}
	for _, candidate := range candidates {
		for _, name := range []string{"justfile", "Justfile"} {
			path := filepath.Join(candidate, name)
			if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
				return candidate, nil
			} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
				return "", statErr
			}
		}
	}
	return "", fmt.Errorf("kein justfile/Justfile in %s oder %s gefunden", filepath.Join(root, "current"), root)
}
