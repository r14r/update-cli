package updater

import (
	"fmt"
	"os"
	"path/filepath"
)

// validateActivationCandidate ensures the candidate to be activated is a
// standalone directory with the expected release marker. The gate runs after
// staged setup and before any current/ activation.
func validateActivationCandidate(stageDir, currentDir, version string) error {
	stage, err := filepath.Abs(stageDir)
	if err != nil {
		return fmt.Errorf("Staging-Pfad ungültig: %w", err)
	}
	current, err := filepath.Abs(currentDir)
	if err != nil {
		return fmt.Errorf("Current-Pfad ungültig: %w", err)
	}
	stage = filepath.Clean(stage)
	current = filepath.Clean(current)
	if stage == current {
		return fmt.Errorf("Staging darf nicht mit current identisch sein: %s", stage)
	}
	rel, err := filepath.Rel(current, stage)
	if err != nil {
		return fmt.Errorf("Staging-Pfad kann nicht relativ zu current geprüft werden: %w", err)
	}
	if rel != ".." && !filepath.IsAbs(rel) && rel != "." && !startsOutside(rel) {
		return fmt.Errorf("Staging darf nicht innerhalb von current liegen: %s", stage)
	}
	info, err := os.Lstat(stage)
	if err != nil {
		return fmt.Errorf("Staging-Verzeichnis fehlt: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("Staging muss ein echtes Verzeichnis sein: %s", stage)
	}
	if err := verifyMarker(stage, version); err != nil {
		return fmt.Errorf("Staging-Version nicht valide: %w", err)
	}
	return nil
}

func startsOutside(rel string) bool {
	return rel == ".." || (len(rel) > 3 && rel[:3] == ".."+string(os.PathSeparator))
}
