package updater

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/r14r/update-cli/src/lib/config"
	"github.com/r14r/update-cli/src/lib/projectsetup"
	"github.com/r14r/update-cli/src/lib/ui"
)

// runCandidateQualityGate runs an explicitly declared schemaVersion 2
// "preactivate" workflow in the isolated candidate, never active current/.
// Projects without this workflow keep the previous update behavior.
func runCandidateQualityGate(ctx context.Context, candidate config.Config, console *ui.Console, progress *updateProgress) error {
	manifestPath, exists, err := projectsetup.FindManifest(candidate.CurrentDir)
	if err != nil {
		return err
	}
	if !exists {
		progress.skip("Qualitätsprüfung des Release-Kandidaten", "kein Manifest vorhanden")
		return nil
	}
	catalog, err := projectsetup.CatalogForManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("Kandidaten-Manifest ungültig: %w", err)
	}
	enabled := false
	for _, workflow := range catalog.Workflows {
		if workflow.Name == "preactivate" {
			enabled = true
			break
		}
	}
	if !enabled {
		progress.skip("Qualitätsprüfung des Release-Kandidaten", "kein preactivate-Workflow definiert")
		return nil
	}
	if err := progress.run("Preactivation-Qualitätsprüfung im Staging", func() error {
		_, runErr := projectsetup.RunSelected(ctx, candidate, console, projectsetup.Selection{Workflow: "preactivate"})
		return runErr
	}); err != nil {
		return fmt.Errorf("preactivate-Workflow in %s fehlgeschlagen: %w", filepath.Base(candidate.CurrentDir), err)
	}
	return nil
}
