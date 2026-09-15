package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/effectiveconfig"
	"github.com/r14r/update-cli/lib/inventory"
	"github.com/r14r/update-cli/lib/projectdocker"
	"github.com/r14r/update-cli/lib/projectsetup"
	rsyncutil "github.com/r14r/update-cli/lib/rsync"
	"github.com/r14r/update-cli/lib/source"
	"github.com/r14r/update-cli/lib/tools"
	"github.com/r14r/update-cli/lib/updatecheck"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
)

type Level string

const (
	LevelOK      Level = "ok"
	LevelWarning Level = "warning"
	LevelError   Level = "error"
)

type Check struct {
	Name   string `json:"name"`
	Level  Level  `json:"level"`
	Detail string `json:"detail"`
}
type ManifestStatus struct {
	Location      string                               `json:"location"`
	Path          string                               `json:"path"`
	Exists        bool                                 `json:"exists"`
	SchemaVersion int                                  `json:"schemaVersion,omitempty"`
	CurrentSchema int                                  `json:"currentSchemaVersion"`
	Migration     *projectsetup.ProjectMigrationResult `json:"migration,omitempty"`
	Repair        *projectsetup.ProjectRepairResult    `json:"repair,omitempty"`
	Inspection    *projectsetup.ManifestInspection     `json:"inspection,omitempty"`
	Suggestions   []string                             `json:"suggestions,omitempty"`
}

type RuntimeConfigStatus struct {
	Path            string   `json:"path"`
	Exists          bool     `json:"exists"`
	SchemaVersion   int      `json:"schemaVersion,omitempty"`
	CurrentSchema   int      `json:"currentSchemaVersion"`
	MigrationNeeded bool     `json:"migrationNeeded,omitempty"`
	Suggestions     []string `json:"suggestions,omitempty"`
}

type Report struct {
	WorkingDirectory    string                               `json:"workingDirectory,omitempty"`
	Root                string                               `json:"root"`
	MigrationRequired   bool                                 `json:"migrationRequired"`
	MigrationReasons    []MigrationReason                    `json:"migrationReasons,omitempty"`
	Manifest            string                               `json:"manifest,omitempty"`
	SchemaVersion       int                                  `json:"schemaVersion,omitempty"`
	LatestSchemaVersion int                                  `json:"latestSchemaVersion,omitempty"`
	Migration           *projectsetup.ProjectMigrationResult `json:"migration,omitempty"`
	Manifests           []ManifestStatus                     `json:"manifests,omitempty"`
	RuntimeConfig       *RuntimeConfigStatus                 `json:"runtimeConfig,omitempty"`
	Config              *config.Config                       `json:"config,omitempty"`
	Checks              []Check                              `json:"checks"`
}

func (r *Report) Add(n string, l Level, d string) {
	r.Checks = append(r.Checks, Check{Name: n, Level: l, Detail: d})
}
func (r Report) ErrorCount() int {
	n := 0
	for _, c := range r.Checks {
		if c.Level == LevelError {
			n++
		}
	}
	return n
}
func (r Report) WarningCount() int {
	n := 0
	for _, c := range r.Checks {
		if c.Level == LevelWarning {
			n++
		}
	}
	return n
}
func RunProject(start string, migrate bool) Report {
	r := Report{LatestSchemaVersion: projectsetup.SchemaVersion}
	working, projectRoot, err := resolveDoctorContext(start)
	if err != nil {
		r.Add("Projektordner", LevelError, err.Error())
		return r
	}
	r.WorkingDirectory = working
	r.Root = projectRoot
	r.Add("Arbeitsordner", LevelOK, working)
	if working != projectRoot {
		r.Add("Projektwurzel", LevelOK, projectRoot+" (aus current/ abgeleitet)")
	} else {
		r.Add("Projektwurzel", LevelOK, projectRoot)
	}

	configStatus := checkRuntimeConfig(projectRoot, migrate, &r)
	r.RuntimeConfig = &configStatus

	locations := []struct {
		name string
		dir  string
	}{
		{name: "root", dir: projectRoot},
		{name: "current", dir: filepath.Join(projectRoot, "current")},
	}
	parsed := map[string]projectsetup.Manifest{}
	for _, location := range locations {
		status, manifest, ok := checkManifestLocation(location.name, location.dir, migrate, &r)
		r.Manifests = append(r.Manifests, status)
		if ok {
			parsed[location.name] = manifest
			if r.Manifest == "" || location.name == "root" {
				r.Manifest = status.Path
				r.SchemaVersion = status.SchemaVersion
				r.Migration = status.Migration
			}
		}
	}

	rootManifest, rootOK := parsed["root"]
	currentManifest, currentOK := parsed["current"]
	if rootOK && currentOK {
		if rootManifest.Version != currentManifest.Version {
			r.Add("Manifest-Abgleich", LevelWarning, fmt.Sprintf("root verwendet Schema %d, current Schema %d; beide auf Schema %d halten", rootManifest.Version, currentManifest.Version, projectsetup.SchemaVersion))
		}
		if !reflect.DeepEqual(rootManifest.Update, currentManifest.Update) {
			r.Add("Update-Konfiguration", LevelWarning, "./update-cli.yaml und ./current/update-cli.yaml enthalten unterschiedliche update-Einstellungen; für Runtime-Konfiguration hat ./update-cli.yaml Vorrang")
		}
	}

	migrationRequirement := InspectMigrationRequirement(projectRoot)
	r.MigrationRequired = migrationRequirement.Required
	r.MigrationReasons = append([]MigrationReason(nil), migrationRequirement.Reasons...)

	if configStatus.Exists {
		cfg, meta, effectiveErr := effectiveconfig.Load(projectRoot)
		if effectiveErr != nil {
			r.Add("Effektive Konfiguration", LevelError, effectiveErr.Error()+"; mit 'update-cli fix' Strukturfehler reparieren")
		} else {
			r.Config = &cfg
			detail := "global + lokal"
			if meta.ManifestPath != "" {
				detail += " + " + meta.ManifestPath
				if len(meta.Applied) > 0 {
					detail += "; YAML überschreibt: " + strings.Join(meta.Applied, ", ")
				}
			}
			r.Add("Konfigurations-Priorität", LevelOK, "global config.json < lokale .update-cli/config.json < update-cli.yaml < Kommandozeilen-Parameter")
			if fields := projectConfigValuesNotVersioned(projectRoot, meta.Applied); len(fields) > 0 {
				r.Add("Projektkonfiguration", LevelWarning, "Lokale config.json enthält noch projektabhängige Werte ohne entsprechenden YAML-Override: "+strings.Join(fields, ", ")+". Das ist kein Schemafehler; für reproduzierbare Projekte diese Werte in update-cli.yaml versionieren. Host-Security, source.defaultUser und CLI-Defaults bleiben in config.json.")
			}
			r.Add("Effektive Konfiguration", LevelOK, detail)
		}
	}

	if !rootOK && !currentOK {
		manifestExists := false
		for _, status := range r.Manifests {
			manifestExists = manifestExists || status.Exists
		}
		if manifestExists {
			r.Add("Manifest", LevelError, "kein vorhandenes Root-/current-Manifest ist gültig bzw. verwendbar")
		} else {
			r.Add("Manifest", LevelError, "weder ./update-cli.yaml noch ./current/update-cli.yaml gefunden")
		}
	}
	return r
}

func projectConfigValuesNotVersioned(root string, applied []string) []string {
	path := filepath.Join(root, config.ConfigDirName, config.ConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	covered := map[string]bool{}
	for _, value := range applied {
		covered[value] = true
	}
	out := []string{}
	add := func(jsonField, yamlField string) {
		if _, ok := raw[jsonField]; ok && !covered[yamlField] {
			out = append(out, jsonField+" → "+yamlField)
		}
	}
	add("projectName", "project.slug")
	add("mode", "update.mode")
	add("releaseDir", "update.releaseDir")
	add("currentDir", "update.currentDir")
	add("backup", "update.backup")
	add("retention", "update.retention")
	add("sync", "update.sync")
	add("docker", "update.docker")

	if value, ok := raw["source"]; ok && !covered["update.source"] {
		var source map[string]json.RawMessage
		if json.Unmarshal(value, &source) == nil {
			for _, key := range []string{"type", "folder", "url", "repository", "ref", "commit", "version", "sha256"} {
				if _, exists := source[key]; exists {
					out = append(out, "source."+key+" → update.source")
					break
				}
			}
		}
	}
	if value, ok := raw["setup"]; ok && !covered["update.sync"] && !covered["update.setup"] {
		var setup map[string]json.RawMessage
		if json.Unmarshal(value, &setup) == nil {
			if _, exists := setup["keepRsyncOnError"]; exists {
				out = append(out, "setup.keepRsyncOnError → update.sync.keepOnSetupError")
			}
		}
	}
	if value, ok := raw["healthcheck"]; ok && !covered["update.healthcheck"] {
		var health map[string]json.RawMessage
		if json.Unmarshal(value, &health) == nil && len(health) > 0 {
			out = append(out, "healthcheck → update.healthcheck")
		}
	}
	return out
}

func resolveDoctorContext(start string) (string, string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("kein Ordner: %s", abs)
	}
	root := abs
	if filepath.Base(abs) == "current" {
		parent := filepath.Dir(abs)
		if hasDoctorRootMarker(parent) {
			root = parent
		}
	}
	return abs, root, nil
}

func hasDoctorRootMarker(root string) bool {
	for _, path := range []string{
		filepath.Join(root, config.ConfigDirName, config.ConfigFileName),
		filepath.Join(root, config.ProjectFileName),
		filepath.Join(root, "setup.yaml"),
	} {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func checkRuntimeConfig(root string, migrate bool, r *Report) RuntimeConfigStatus {
	canonical := filepath.Join(root, config.ConfigDirName, config.ConfigFileName)
	path := canonical
	if _, err := os.Stat(canonical); errors.Is(err, os.ErrNotExist) {
		r.Add("Runtime config.json", LevelWarning, "nicht vorhanden: "+canonical+"; für reine Source-Projekte optional, für verwaltete Updates mit 'update-cli init <project>' erzeugen")
		return RuntimeConfigStatus{Path: canonical, CurrentSchema: config.SchemaVersion}
	} else if err != nil {
		r.Add("Runtime config.json", LevelError, err.Error())
		return RuntimeConfigStatus{Path: canonical, CurrentSchema: config.SchemaVersion}
	}
	status := RuntimeConfigStatus{Path: path, Exists: true, CurrentSchema: config.SchemaVersion}
	if migrate {
		res, err := config.Upgrade(root)
		if err != nil {
			status.Suggestions = append(status.Suggestions, "Migration nicht automatisch möglich; 'update-cli fix' verwenden: "+err.Error())
			r.Add("Runtime config.json", LevelError, err.Error()+"; 'update-cli fix' kann bekannte Legacy-/Fehlfelder normalisieren")
			return status
		}
		status.Path = res.ConfigFile
		status.SchemaVersion = res.CurrentSchema
		status.MigrationNeeded = false
		if res.Changed {
			detail := fmt.Sprintf("Schema %d → %d", res.PreviousSchema, res.CurrentSchema)
			if res.BackupFile != "" {
				detail += "; Backup: " + res.BackupFile
			}
			r.Add("Runtime config.json", LevelOK, detail)
		} else {
			r.Add("Runtime config.json", LevelOK, fmt.Sprintf("Schema %d ist aktuell", res.CurrentSchema))
		}
		return status
	}
	res, err := config.Check(root)
	if err != nil {
		status.Suggestions = append(status.Suggestions, "'update-cli fix' ausführen")
		r.Add("Runtime config.json", LevelError, err.Error()+"; mit 'update-cli fix' bekannte Feld-/Strukturfehler reparieren")
		return status
	}
	status.Path = res.ConfigFile
	status.SchemaVersion = res.SchemaVersion
	status.CurrentSchema = res.CurrentSchema
	status.MigrationNeeded = res.MigrationNeeded
	if res.MigrationNeeded {
		status.Suggestions = append(status.Suggestions, "'update-cli doctor --migrate' oder 'update-cli upgrade' ausführen")
		r.Add("Runtime config.json", LevelWarning, fmt.Sprintf("Schema %d ist veraltet; aktuell %d", res.SchemaVersion, res.CurrentSchema))
	} else {
		r.Add("Runtime config.json", LevelOK, fmt.Sprintf("Schema %d ist aktuell: %s", res.SchemaVersion, res.ConfigFile))
	}
	return status
}

func checkManifestLocation(location, dir string, migrate bool, r *Report) (ManifestStatus, projectsetup.Manifest, bool) {
	canonical := filepath.Join(dir, config.ProjectFileName)
	legacy := filepath.Join(dir, "setup.yaml")
	status := ManifestStatus{Location: location, Path: canonical, CurrentSchema: projectsetup.SchemaVersion}
	path := canonical
	if _, err := os.Stat(canonical); errors.Is(err, os.ErrNotExist) {
		if _, legacyErr := os.Stat(legacy); legacyErr == nil {
			path = legacy
			status.Path = legacy
			status.Suggestions = append(status.Suggestions, "Legacy setup.yaml auf update-cli.yaml migrieren")
		} else if errors.Is(legacyErr, os.ErrNotExist) {
			return status, projectsetup.Manifest{}, false
		} else {
			r.Add("Manifest "+location, LevelError, legacyErr.Error())
			return status, projectsetup.Manifest{}, false
		}
	} else if err != nil {
		r.Add("Manifest "+location, LevelError, err.Error())
		return status, projectsetup.Manifest{}, false
	}
	status.Exists = true

	if migrate {
		migration, err := projectsetup.MigrateProjectManifest(dir)
		if err == nil {
			status.Migration = &migration
			status.Path = migration.Path
			path = migration.Path
			if migration.Changed {
				detail := fmt.Sprintf("Schema %d → %d", migration.PreviousSchema, migration.CurrentSchema)
				if migration.Canonicalized {
					detail += "; update-cli.yaml erzeugt"
				}
				if migration.BackupPath != "" {
					detail += "; Backup: " + migration.BackupPath
				}
				r.Add("Manifest "+location, LevelOK, detail)
			}
		} else {
			// Numeric schema migration can fail before it reaches structural cleanup.
			// doctor --migrate therefore falls back to the same deterministic repair
			// engine used by `update-cli fix`.
			repair, repairErr := projectsetup.RepairProjectManifest(dir)
			if repairErr != nil {
				inspection := projectsetup.InspectManifest(path)
				status.Inspection = &inspection
				status.Suggestions = append(status.Suggestions, "'update-cli fix' für strukturelle Reparatur verwenden")
				r.Add("Manifest "+location, LevelError, err.Error()+"; automatische Strukturreparatur ebenfalls fehlgeschlagen: "+repairErr.Error())
				addManifestInspectionChecks(location, inspection, r)
				return status, projectsetup.Manifest{}, false
			}
			status.Repair = &repair
			status.Path = repair.Path
			path = repair.Path
			if repair.Changed {
				detail := "Struktur auf aktuellen Standard repariert"
				if repair.BackupPath != "" {
					detail += "; Backup: " + repair.BackupPath
				}
				r.Add("Manifest "+location, LevelOK, detail)
			}
		}
	}

	manifest, err := projectsetup.ParseManifest(path)
	inspection := projectsetup.InspectManifest(path)
	status.Inspection = &inspection
	if err != nil {
		status.Suggestions = append(status.Suggestions, "'update-cli fix' ausführen")
		r.Add("Manifest "+location, LevelError, err.Error())
		addManifestInspectionChecks(location, inspection, r)
		return status, projectsetup.Manifest{}, false
	}

	// A manifest can parse successfully while still carrying tolerated transition
	// metadata. Surface and optionally normalize that too, so doctor never hides
	// repairable drift merely because execution could continue.
	if inspection.Repairable {
		if migrate && (status.Repair == nil || !status.Repair.Changed) {
			repair, repairErr := projectsetup.RepairProjectManifest(dir)
			if repairErr != nil {
				r.Add("Manifest "+location, LevelError, "reparierbare Struktur erkannt, Reparatur fehlgeschlagen: "+repairErr.Error())
				return status, manifest, true
			}
			status.Repair = &repair
			status.Path = repair.Path
			path = repair.Path
			if repair.Changed {
				r.Add("Manifest "+location, LevelOK, "Tolerierte Legacy-/Fehlfelder auf kanonische Struktur normalisiert")
			}
			manifest, err = projectsetup.ParseManifest(path)
			if err != nil {
				r.Add("Manifest "+location, LevelError, "Manifest ist nach Reparatur ungültig: "+err.Error())
				return status, projectsetup.Manifest{}, false
			}
			inspection = projectsetup.InspectManifest(path)
			status.Inspection = &inspection
		} else {
			status.Suggestions = append(status.Suggestions, "'update-cli fix' oder 'update-cli doctor --migrate' ausführen")
			addManifestInspectionChecks(location, inspection, r)
		}
	}

	status.SchemaVersion = manifest.Version
	if manifest.Version < projectsetup.SchemaVersion {
		status.Suggestions = append(status.Suggestions, "'update-cli doctor --migrate' ausführen")
		r.Add("Manifest "+location, LevelWarning, fmt.Sprintf("%s verwendet Schema %d; aktuell %d", path, manifest.Version, projectsetup.SchemaVersion))
	} else if manifest.Version > projectsetup.SchemaVersion {
		r.Add("Manifest "+location, LevelError, fmt.Sprintf("Schema %d ist neuer als unterstützt %d", manifest.Version, projectsetup.SchemaVersion))
		return status, manifest, true
	} else if !migrate || (status.Migration == nil || !status.Migration.Changed) && (status.Repair == nil || !status.Repair.Changed) {
		r.Add("Manifest "+location, LevelOK, fmt.Sprintf("Schema %d ist aktuell: %s", manifest.Version, path))
	}
	if strings.TrimSpace(manifest.ProjectName) == "" && strings.TrimSpace(manifest.ProjectSlug) == "" {
		status.Suggestions = append(status.Suggestions, "project.name bzw. project.slug ergänzen")
		r.Add("Projekt "+location, LevelWarning, "project.name/project.slug ist nicht gesetzt")
	}
	for _, name := range manifest.Requirements.Commands {
		if _, err := exec.LookPath(name); err != nil {
			r.Add("Requirement "+location+"/"+name, LevelWarning, "Kommando nicht im PATH gefunden")
		}
	}
	return status, manifest, true
}

func addManifestInspectionChecks(location string, inspection projectsetup.ManifestInspection, r *Report) {
	for _, field := range inspection.RemovedFields {
		r.Add("Manifest-Feld "+location, LevelWarning, "unbekannt/entfernbar: "+field)
	}
	for _, value := range inspection.Normalized {
		r.Add("Manifest-Struktur "+location, LevelWarning, "normalisierbar: "+value)
	}
	if inspection.Repairable {
		r.Add("Manifest-Reparatur "+location, LevelWarning, "automatisch reparierbar mit 'update-cli fix' oder 'update-cli doctor --migrate'")
	}
}

func Run(ctx context.Context, root string, cfg config.Config) Report {
	r := Report{Root: root, Config: &cfg}
	if d, e := rsyncutil.Describe(); e != nil {
		r.Add("rsync", LevelError, e.Error())
	} else {
		r.Add("rsync", LevelOK, d)
	}
	lock := filepath.Join(root, ".release-update.lock")
	if _, e := os.Stat(lock); errors.Is(e, os.ErrNotExist) {
		r.Add("Update-Sperre", LevelOK, "keine aktive Sperre")
	} else if e != nil {
		r.Add("Update-Sperre", LevelError, e.Error())
	} else {
		stale, detail := tools.IsStaleLock(lock)
		if stale {
			r.Add("Update-Sperre", LevelWarning, "veraltete Sperre: "+detail)
		} else {
			r.Add("Update-Sperre", LevelError, "aktive/unklare Sperre: "+detail)
		}
	}
	if p, ok, e := projectsetup.Detect(cfg); e != nil {
		r.Add("Projekt-Setup", LevelError, e.Error())
	} else if !ok {
		r.Add("Projekt-Setup", LevelOK, "kein Setup konfiguriert")
	} else if projectsetup.IsManifest(p) {
		if _, e := projectsetup.ParseManifest(p); e != nil {
			r.Add("Projekt-Setup", LevelError, e.Error())
		} else {
			r.Add("Projekt-Setup", LevelOK, "gültiges Manifest: "+p)
		}
	} else {
		r.Add("Projekt-Setup", LevelWarning, "Legacy-Setup: "+p)
	}
	r.Add("Docker lifecycle", LevelOK, cfg.Docker.Lifecycle)
	if cfg.Docker.Lifecycle == "disabled" {
		r.Add("Docker Compose", LevelOK, "übersprungen; Docker-Lifecycle deaktiviert")
	} else {
		d, e := projectdocker.Detect(cfg.CurrentDir)
		if e != nil {
			if cfg.Docker.Lifecycle == "required" {
				r.Add("Docker Compose", LevelError, e.Error())
			} else {
				r.Add("Docker Compose", LevelWarning, e.Error())
			}
		} else if !d.Detected {
			r.Add("Docker Compose", LevelOK, "kein Compose-Projekt erkannt")
		} else if _, e := projectdocker.Running(ctx, cfg.CurrentDir); e != nil {
			if cfg.Docker.Lifecycle == "required" {
				r.Add("Docker Compose", LevelError, e.Error())
			} else {
				r.Add("Docker Compose", LevelWarning, e.Error())
			}
		} else {
			r.Add("Docker Compose", LevelOK, filepath.Base(d.ComposeFile)+" erkannt; Statusabfrage erfolgreich")
		}
	}
	local, e := inventory.ListLocal(cfg)
	if e != nil {
		r.Add("Lokales Inventar", LevelError, e.Error())
	} else {
		invalid := 0
		for _, x := range local.Releases {
			if !x.Validated {
				invalid++
			}
		}
		r.Add("Lokales Inventar", LevelOK, fmt.Sprintf("%d Releases, %d Backups, %d ungültige Releases", len(local.Releases), len(local.Backups), invalid))
	}
	m, e := source.Discover(ctx, source.Options{ProjectName: cfg.ProjectName, Mode: cfg.Mode, Source: cfg.Source, RepositoryCacheDir: cfg.RepositoryCacheDir, AllowHTTP: cfg.Security.AllowHTTP, MaxArchiveBytes: cfg.Security.MaxArchiveBytes})
	if e != nil {
		r.Add("Release-Quelle", LevelWarning, e.Error())
	} else {
		r.Add("Release-Quelle", LevelOK, fmt.Sprintf("%s Version %s", m.Type, m.VersionText))
	}
	if v, src, found, e := updatecheck.DetectInstalled(cfg.CurrentDir); e != nil {
		r.Add("Current", LevelError, e.Error())
	} else if !found {
		r.Add("Current", LevelWarning, "keine installierte Version erkannt")
	} else {
		r.Add("Current", LevelOK, fmt.Sprintf("Version %s (%s)", v.String(), src))
	}
	for name, p := range map[string]string{"Release-Ziel": cfg.ReleaseRoot, "Current-Ziel": cfg.CurrentDir, "Backup-Ziel": cfg.BackupRoot} {
		if _, e := tools.CanonicalInside(cfg.RootDir, p, true); e != nil {
			r.Add(name, LevelError, e.Error())
		} else {
			r.Add(name, LevelOK, p)
		}
	}
	if cfg.Source.Type == "repository" {
		if _, e := exec.LookPath("git"); e != nil {
			r.Add("git", LevelError, "git fehlt")
		}
	}
	if strings.TrimSpace(cfg.Healthcheck.Type) != "" && cfg.Healthcheck.Type != "none" {
		r.Add("Healthcheck", LevelOK, cfg.Healthcheck.Type)
	} else {
		r.Add("Healthcheck", LevelWarning, "kein projektbezogener Healthcheck konfiguriert")
	}
	return r
}
