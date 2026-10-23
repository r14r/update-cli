package projectsetup

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/r14r/update-cli/src/lib/ui"
)

// RunJustTest executes the canonical just test recipe before installation.
func RunJustTest(ctx context.Context, root string, console *ui.Console) error {
	work, err := justInstallDirectory(root)
	if err != nil {
		return err
	}
	just, err := exec.LookPath("just")
	if err != nil {
		return fmt.Errorf("Kommando 'just' ist nicht installiert: %w", err)
	}
	if console.Fullscreen() {
		console.SetInfoTitle("Projekt testen")
		console.InfoRow("Arbeitsordner", work)
		console.InfoRow("Kommando", "just test")
	} else {
		console.Header("Projekt testen")
		console.Row("Arbeitsordner", work)
		console.Row("Kommando", "just test")
	}
	return console.Step(ctx, 0, 1, "just test ausführen", func() error {
		cmd := exec.CommandContext(ctx, just, "test")
		cmd.Dir = work
		cmd.Stdin = os.Stdin
		stdout, stderr := console.ProcessWriters()
		cmd.Stdout, cmd.Stderr = stdout, stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("just test fehlgeschlagen: %w", err)
		}
		return nil
	})
}
