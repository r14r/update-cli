package doctor

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/projectsetup"
)

// MigrationReason describes one concrete condition that makes the fullscreen
// UI show the "Migration required" badge.
type MigrationReason struct {
	Scope  string `json:"scope"`
	Path   string `json:"path,omitempty"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

// MigrationRequirement is the single source of truth for the fullscreen
// migration badge and doctor diagnostics.
type MigrationRequirement struct {
	Required bool              `json:"required"`
	Reasons  []MigrationReason `json:"reasons,omitempty"`
}

// InspectMigrationRequirement performs the same non-mutating checks that drive
// the fullscreen migration badge. Doctor uses this result verbatim so the UI
// and diagnostics cannot disagree about whether a migration is pending.
func InspectMigrationRequirement(root string) MigrationRequirement {
	result := MigrationRequirement{}

	if check, err := config.Check(root); err == nil && check.MigrationNeeded {
		result.Reasons = append(result.Reasons, MigrationReason{
			Scope:  "runtime-config",
			Path:   check.ConfigFile,
			Kind:   "schema",
			Detail: fmt.Sprintf("config.json verwendet Schema %d; aktuell ist Schema %d", check.SchemaVersion, check.CurrentSchema),
		})
	}

	currentDir := filepath.Join(root, "current")
	if cfg, err := config.Load(root, ""); err == nil && strings.TrimSpace(cfg.CurrentDir) != "" {
		currentDir = cfg.CurrentDir
	}

	locations := []struct {
		scope string
		dir   string
	}{
		{scope: "manifest-root", dir: root},
		{scope: "manifest-current", dir: currentDir},
	}
	seen := map[string]bool{}
	for _, location := range locations {
		dir := filepath.Clean(location.dir)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		manifest, ok, err := projectsetup.FindManifest(dir)
		if err != nil || !ok {
			continue
		}
		inspection := projectsetup.InspectManifest(manifest)
		appendManifestMigrationReasons(&result, location.scope, manifest, inspection)
	}

	result.Required = len(result.Reasons) > 0
	return result
}

func appendManifestMigrationReasons(result *MigrationRequirement, scope, path string, inspection projectsetup.ManifestInspection) {
	parts := []string{}
	kind := "manifest"
	if inspection.SchemaVersion > 0 && inspection.SchemaVersion != inspection.CurrentSchema {
		kind = "schema"
		parts = append(parts, fmt.Sprintf("Schema %d → %d", inspection.SchemaVersion, inspection.CurrentSchema))
	}
	if inspection.Repairable {
		if len(inspection.RemovedFields) > 0 {
			parts = append(parts, "unbekannte/entfernbare Felder: "+strings.Join(inspection.RemovedFields, ", "))
		}
		if len(inspection.Normalized) > 0 {
			parts = append(parts, "reparierbare Werte/Strukturen: "+strings.Join(inspection.Normalized, ", "))
		}
		if !inspection.Valid && len(inspection.RemovedFields) == 0 && len(inspection.Normalized) == 0 && strings.TrimSpace(inspection.Error) != "" {
			parts = append(parts, inspection.Error)
		}
		if len(parts) == 0 {
			parts = append(parts, "Manifest benötigt eine deterministische Legacy-/Strukturmigration")
		}
	}
	if len(parts) == 0 {
		return
	}
	result.Reasons = append(result.Reasons, MigrationReason{
		Scope:  scope,
		Path:   path,
		Kind:   kind,
		Detail: strings.Join(parts, "; "),
	})
}
