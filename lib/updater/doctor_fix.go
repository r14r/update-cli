package updater

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/doctor"
	"github.com/r14r/update-cli/lib/projectsetup"
	"github.com/r14r/update-cli/lib/ui"
)

type doctorFixPlan struct {
	Config         config.RepairResult
	ConfigChanged  bool
	ManifestDirs   []string
	PlannedChanges []string
}

func buildDoctorFixPlan(root string, report doctor.Report) (doctorFixPlan, error) {
	plan := doctorFixPlan{}
	runtimeNeedsRepair := report.RuntimeConfig != nil && report.RuntimeConfig.MigrationNeeded
	for _, check := range report.Checks {
		if check.Name == "Runtime config.json" && check.Level == doctor.LevelError {
			runtimeNeedsRepair = true
		}
	}
	if runtimeNeedsRepair && report.RuntimeConfig != nil && report.RuntimeConfig.Exists {
		preview, err := config.PreviewRepairProjectConfig(root)
		if err != nil {
			return plan, fmt.Errorf("config.json-Reparatur kann nicht sicher geplant werden: %w", err)
		}
		plan.Config = preview
		for _, item := range configRepairPlanLines(preview) {
			plan.PlannedChanges = append(plan.PlannedChanges, item)
		}
		plan.ConfigChanged = preview.Local.Changed || (preview.Global != nil && preview.Global.Changed)
	}

	seen := map[string]bool{}
	for _, manifest := range report.Manifests {
		if !manifest.Exists {
			continue
		}
		needsRepair := manifest.SchemaVersion > 0 && manifest.SchemaVersion != manifest.CurrentSchema
		if manifest.Inspection != nil && manifest.Inspection.Repairable {
			needsRepair = true
		}
		if !needsRepair {
			continue
		}
		dir := filepath.Dir(manifest.Path)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		plan.ManifestDirs = append(plan.ManifestDirs, dir)
		details := []string{}
		if manifest.SchemaVersion > 0 && manifest.SchemaVersion != manifest.CurrentSchema {
			details = append(details, fmt.Sprintf("Schema %d → %d", manifest.SchemaVersion, manifest.CurrentSchema))
		}
		if manifest.Inspection != nil {
			if len(manifest.Inspection.RemovedFields) > 0 {
				details = append(details, "entfernen: "+strings.Join(manifest.Inspection.RemovedFields, ", "))
			}
			if len(manifest.Inspection.Normalized) > 0 {
				details = append(details, "normalisieren: "+strings.Join(manifest.Inspection.Normalized, ", "))
			}
		}
		if len(details) == 0 {
			details = append(details, "auf aktuellen Manifest-Standard normalisieren")
		}
		plan.PlannedChanges = append(plan.PlannedChanges, manifest.Path+": "+strings.Join(details, "; "))
	}
	return plan, nil
}

func configRepairPlanLines(result config.RepairResult) []string {
	out := []string{}
	appendResult := func(item config.RepairFileResult) {
		if !item.Changed {
			return
		}
		details := []string{}
		if item.Created {
			details = append(details, "erstellen")
		}
		if len(item.MigratedFields) > 0 {
			details = append(details, "migrieren: "+strings.Join(item.MigratedFields, ", "))
		}
		if len(item.RemovedFields) > 0 {
			details = append(details, "entfernen: "+strings.Join(item.RemovedFields, ", "))
		}
		if len(item.Normalized) > 0 {
			details = append(details, "normalisieren: "+strings.Join(item.Normalized, ", "))
		}
		if len(details) == 0 {
			details = append(details, "auf aktuellen Standard normalisieren")
		}
		out = append(out, item.Path+": "+strings.Join(details, "; "))
	}
	appendResult(result.Local)
	if result.Global != nil {
		appendResult(*result.Global)
	}
	return out
}

func printDoctorFixPlan(c *ui.Console, plan doctorFixPlan) {
	c.Header("Doctor Fix — geplante Änderungen")
	if len(plan.PlannedChanges) == 0 {
		c.Success("Keine automatisch reparierbaren Fehler gefunden")
		return
	}
	for i, change := range plan.PlannedChanges {
		c.Row(fmt.Sprintf("[%02d]", i+1), change)
	}
	c.Info("Vor der Reparatur werden die üblichen timestamp-basierten Backups angelegt.")
}

func applyDoctorFix(root string, plan doctorFixPlan) error {
	if plan.ConfigChanged {
		if _, err := config.RepairProjectConfig(root); err != nil {
			return err
		}
	}
	for _, dir := range plan.ManifestDirs {
		if _, err := projectsetup.RepairProjectManifest(dir); err != nil {
			return fmt.Errorf("Manifest %s kann nicht repariert werden: %w", filepath.Join(dir, config.ProjectFileName), err)
		}
	}
	return nil
}

func confirmDoctorFix(c *ui.Console) (bool, error) {
	const prompt = "Geplante Reparaturen durchführen?"
	if c.Interactive() {
		return c.Confirm(prompt, false)
	}
	fmt.Fprint(os.Stdout, prompt+" [y/N] ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && len(line) == 0 {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes", "j", "ja":
		return true, nil
	default:
		return false, nil
	}
}
