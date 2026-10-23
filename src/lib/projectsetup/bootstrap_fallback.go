package projectsetup

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/r14r/update-cli/src/lib/ui"
)

// runBootstrapJustFallback supports legacy manifests when the new update
// lifecycle requests bootstrap but no "bootstrap" workflow was declared.
// The project must explicitly contain a Justfile/justfile.
func runBootstrapJustFallback(ctx context.Context, workdir, manifest string, console *ui.Console) (Result, error) {
	just, err := exec.LookPath("just")
	if err != nil {
		return Result{}, fmt.Errorf("Bootstrap-Workflow fehlt in %s und 'just' ist nicht installiert: %w", manifest, err)
	}
	console.Warn(fmt.Sprintf("Workflow 'bootstrap' fehlt in %s; verwende 'just bootstrap' als Kompatibilitäts-Fallback", manifest))
	cmd := exec.CommandContext(ctx, just, "bootstrap")
	cmd.Dir = workdir
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	stdout, stderr := console.ProcessWriters()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return Result{}, fmt.Errorf("just bootstrap fehlgeschlagen: %w", err)
	}
	return Result{Manifest: manifest, StepsExecuted: 1}, nil
}
