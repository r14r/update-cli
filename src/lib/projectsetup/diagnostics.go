package projectsetup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ManifestInspection is a non-mutating structural analysis of an update-cli
// manifest. It is intentionally separate from ParseManifest: normal execution
// remains strict, while doctor/fix can explain all safely repairable problems
// instead of stopping after the first unknown field.
type ManifestInspection struct {
	Path           string   `json:"path"`
	SchemaVersion  int      `json:"schemaVersion,omitempty"`
	CurrentSchema  int      `json:"currentSchemaVersion"`
	Valid          bool     `json:"valid"`
	Repairable     bool     `json:"repairable"`
	Error          string   `json:"error,omitempty"`
	RemovedFields  []string `json:"removedFields,omitempty"`
	Normalized     []string `json:"normalizedValues,omitempty"`
	SuggestedFixes []string `json:"suggestedFixes,omitempty"`
}

// InspectManifest reads and analyzes a manifest without changing it. The
// existing repair implementation is run only against a temporary copy, which
// keeps diagnostics and actual repair behavior aligned.
func InspectManifest(path string) ManifestInspection {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ManifestInspection{Path: path, CurrentSchema: SchemaVersion, Error: err.Error()}
	}
	result := ManifestInspection{Path: abs, CurrentSchema: SchemaVersion}
	data, err := os.ReadFile(abs)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.SchemaVersion = detectSetupSchemaVersion(normalizeLegacySetupYAML(abs, data))
	if manifest, parseErr := ParseManifest(abs); parseErr == nil {
		result.Valid = true
		result.SchemaVersion = manifest.Version
	} else {
		result.Error = parseErr.Error()
	}

	tmpRoot, err := os.MkdirTemp("", "update-cli-manifest-inspect-*")
	if err != nil {
		return result
	}
	defer os.RemoveAll(tmpRoot)
	name := filepath.Base(abs)
	if !strings.EqualFold(name, "setup.yaml") {
		name = "update-cli.yaml"
	}
	copyPath := filepath.Join(tmpRoot, name)
	if err := os.WriteFile(copyPath, data, 0o644); err != nil {
		return result
	}
	repair, repairErr := RepairProjectManifest(tmpRoot)
	if repairErr == nil {
		result.RemovedFields = append([]string(nil), repair.RemovedFields...)
		result.Normalized = append([]string(nil), repair.Normalized...)
		if repair.CurrentSchema > 0 {
			result.CurrentSchema = repair.CurrentSchema
		}
		// RepairProjectManifest renders a canonical YAML representation. A
		// byte-level formatting difference alone is not a migration and must not
		// trigger the UI's "Migration required" badge. Only structural/schema
		// changes, legacy-file canonicalization, or a manifest that does not pass
		// the strict parser count as a repair requirement.
		result.Repairable = !result.Valid ||
			(result.SchemaVersion > 0 && result.SchemaVersion != result.CurrentSchema) ||
			repair.Canonicalized || len(result.RemovedFields) > 0 || len(result.Normalized) > 0
		if result.Repairable {
			result.SuggestedFixes = []string{"update-cli fix", "update-cli doctor --migrate"}
		}
		return result
	}
	if result.Error == "" {
		result.Error = repairErr.Error()
	}
	result.SuggestedFixes = []string{"update-cli doctor"}
	return result
}

// ParseManifestForUse keeps execution strict but enriches parser errors with a
// complete repair preview. This prevents a single typo from yielding only an
// opaque first-field error and gives the user an immediate repair path.
func ParseManifestForUse(path string) (Manifest, error) {
	manifest, err := ParseManifest(path)
	if err == nil {
		return manifest, nil
	}
	inspection := InspectManifest(path)
	var lines []string
	lines = append(lines, err.Error())
	if len(inspection.RemovedFields) > 0 {
		lines = append(lines, "Unbekannte/entfernbare Felder: "+strings.Join(inspection.RemovedFields, ", "))
	}
	if len(inspection.Normalized) > 0 {
		lines = append(lines, "Reparierbare Werte/Strukturen: "+strings.Join(inspection.Normalized, ", "))
	}
	if inspection.Repairable {
		lines = append(lines, "Reparatur: update-cli fix (alternativ: update-cli doctor --migrate)")
	} else {
		lines = append(lines, "Prüfung: update-cli doctor; automatische Reparatur ist für diesen Fehler nicht sicher")
	}
	return Manifest{}, errors.New(strings.Join(uniqueNonEmpty(lines), "\n"))
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func formatInspectionDetail(i ManifestInspection) string {
	parts := []string{}
	if len(i.RemovedFields) > 0 {
		parts = append(parts, "unbekannte Felder: "+strings.Join(i.RemovedFields, ", "))
	}
	if len(i.Normalized) > 0 {
		parts = append(parts, "normalisierbar: "+strings.Join(i.Normalized, ", "))
	}
	if i.Error != "" {
		parts = append(parts, "Parser: "+i.Error)
	}
	if len(parts) == 0 {
		return "keine strukturellen Probleme erkannt"
	}
	return fmt.Sprintf("%s", strings.Join(parts, "; "))
}
