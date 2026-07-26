package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"release-updater/lib/archive"
	"release-updater/lib/backup"
	"release-updater/lib/config"
	"release-updater/lib/projectdocker"
	rsyncutil "release-updater/lib/rsync"
	"release-updater/lib/source"
	"release-updater/lib/templates"
	"release-updater/lib/tools"
	"release-updater/lib/updatecheck"
	versionutil "release-updater/lib/version"
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

type Report struct {
	Root   string         `json:"root"`
	Config *config.Config `json:"config,omitempty"`
	Checks []Check        `json:"checks"`
}

func (report *Report) Add(name string, level Level, detail string) {
	report.Checks = append(report.Checks, Check{Name: name, Level: level, Detail: detail})
}

func (report Report) ErrorCount() int {
	count := 0
	for _, check := range report.Checks {
		if check.Level == LevelError {
			count++
		}
	}
	return count
}

func (report Report) WarningCount() int {
	count := 0
	for _, check := range report.Checks {
		if check.Level == LevelWarning {
			count++
		}
	}
	return count
}

func Run(root, downloadOverride string) Report {
	return RunWithSource(root, "", downloadOverride, "", "")
}

func RunWithSource(root, sourceType, folder, sourceURL, repository string) Report {
	report := Report{Root: root}

	if info, err := os.Stat(root); err != nil {
		report.Add("Projektordner", LevelError, err.Error())
	} else if !info.IsDir() {
		report.Add("Projektordner", LevelError, "Pfad ist kein Ordner: "+root)
	} else {
		report.Add("Projektordner", LevelOK, root)
	}

	cfg, err := config.Load(root, "")
	if err == nil {
		cfg, err = config.WithSourceOverrides(cfg, sourceType, folder, sourceURL, repository)
	}
	if err != nil {
		report.Add("Konfiguration", LevelError, err.Error())
	} else {
		report.Config = &cfg
		report.Add("Konfiguration", LevelOK, cfg.ConfigFile)
	}

	if description, err := rsyncutil.Describe(); err != nil {
		report.Add("rsync", LevelError, err.Error())
	} else {
		report.Add("rsync", LevelOK, description)
	}

	lockPath := filepath.Join(root, ".release-update.lock")
	if info, err := os.Stat(lockPath); err == nil {
		report.Add("Update-Sperre", LevelError, fmt.Sprintf("Sperre vorhanden: %s (geändert %s)", lockPath, info.ModTime().Format("2006-01-02 15:04:05")))
	} else if errors.Is(err, os.ErrNotExist) {
		report.Add("Update-Sperre", LevelOK, "keine aktive Sperre")
	} else {
		report.Add("Update-Sperre", LevelError, err.Error())
	}

	if report.Config == nil {
		return report
	}
	cfg = *report.Config

	switch cfg.SourceType {
	case source.Download:
		checkDirectory(&report, "Quellordner", cfg.SourceFolder, true)
	case source.URL:
		report.Add("Release-Quelle", LevelOK, "URL: "+cfg.SourceURL)
	case source.Repository:
		if _, err := exec.LookPath("git"); err != nil {
			report.Add("git", LevelError, "erforderliches Programm fehlt: git")
		} else {
			report.Add("git", LevelOK, "Repository-Quelle: "+cfg.SourceRepository)
		}
	}
	checkWritable(&report, "Release-Ziel", cfg.ReleaseRoot)
	checkWritable(&report, "Current-Ziel", cfg.CurrentDir)
	checkWritable(&report, "Backup-Ziel", cfg.BackupRoot)
	checkTemplates(&report, cfg)
	checkBackups(&report, cfg)
	checkProjectSetup(&report, cfg)
	checkProjectDocker(&report, cfg)
	checkCurrent(&report, cfg)
	checkReleaseMetadata(&report, cfg)
	checkNewestSource(&report, cfg)
	checkUpdateStatus(&report, cfg)

	return report
}

func checkTemplates(report *Report, cfg config.Config) {
	file, err := templates.Load(cfg.TemplatesFile)
	if err != nil {
		report.Add("Templates", LevelError, err.Error())
		return
	}
	report.Add("Templates", LevelOK, fmt.Sprintf("%d gültige Templates in %s", len(file.Templates), cfg.TemplatesFile))

	if _, err := os.Stat(cfg.GlobalTemplatesFile); errors.Is(err, os.ErrNotExist) {
		report.Add("Globale Templates", LevelOK, "optionale Datei nicht vorhanden: "+cfg.GlobalTemplatesFile)
		return
	} else if err != nil {
		report.Add("Globale Templates", LevelError, err.Error())
		return
	}
	global, err := templates.Load(cfg.GlobalTemplatesFile)
	if err != nil {
		report.Add("Globale Templates", LevelError, err.Error())
		return
	}
	report.Add("Globale Templates", LevelOK, fmt.Sprintf("%d zusätzliche Templates in %s", len(global.Templates), cfg.GlobalTemplatesFile))
}

func checkBackups(report *Report, cfg config.Config) {
	items, err := backup.List(cfg)
	if err != nil {
		report.Add("Backups", LevelError, err.Error())
		return
	}
	invalid := 0
	for _, item := range items {
		if !item.Validated {
			invalid++
		}
	}
	if invalid > 0 {
		report.Add("Backups", LevelWarning, fmt.Sprintf("%d Backups, davon %d ohne gültige Metadaten", len(items), invalid))
		return
	}
	report.Add("Backups", LevelOK, fmt.Sprintf("%d validierte Backups; Retention=%d", len(items), cfg.KeepBackups))
}

func checkProjectSetup(report *Report, cfg config.Config) {
	setupScript := filepath.Join(cfg.CurrentDir, "setup.sh")
	scriptExists := false
	if info, err := os.Stat(setupScript); err == nil {
		if info.IsDir() {
			report.Add("Projekt-Setup", LevelError, "setup.sh ist ein Ordner: "+setupScript)
			return
		}
		scriptExists = true
	} else if !errors.Is(err, os.ErrNotExist) {
		report.Add("Projekt-Setup", LevelError, err.Error())
		return
	}

	if !scriptExists && len(cfg.SetupCommands) == 0 {
		report.Add("Projekt-Setup", LevelOK, "kein setup.sh und keine setup.commands konfiguriert")
		return
	}
	if _, err := exec.LookPath("bash"); err != nil {
		report.Add("Projekt-Setup", LevelError, "bash fehlt")
		return
	}
	if _, err := os.Stat(cfg.CurrentDir); errors.Is(err, os.ErrNotExist) {
		report.Add("Projekt-Setup", LevelWarning, "Setup konfiguriert, aber current fehlt")
		return
	}
	report.Add(
		"Projekt-Setup",
		LevelOK,
		fmt.Sprintf("setup.sh=%t, konfigurierte Kommandos=%d", scriptExists, len(cfg.SetupCommands)),
	)
}

func checkProjectDocker(report *Report, cfg config.Config) {
	detection, err := projectdocker.Detect(cfg.CurrentDir)
	if err != nil {
		report.Add("Docker Compose", LevelError, err.Error())
		return
	}
	if !detection.Detected {
		report.Add("Docker Compose", LevelOK, "kein Compose-Projekt in current erkannt")
		return
	}
	if docker, lookupErr := exec.LookPath("docker"); lookupErr == nil {
		command := exec.CommandContext(context.Background(), docker, "compose", "version")
		command.Dir = cfg.CurrentDir
		if output, runErr := command.CombinedOutput(); runErr == nil {
			detail := strings.TrimSpace(string(output))
			if detail == "" {
				detail = "docker compose verfügbar"
			}
			report.Add("Docker Compose", LevelOK, filepath.Base(detection.ComposeFile)+"; "+detail)
			return
		}
	}
	if _, lookupErr := exec.LookPath("docker-compose"); lookupErr == nil {
		report.Add("Docker Compose", LevelOK, filepath.Base(detection.ComposeFile)+"; docker-compose verfügbar")
		return
	}
	report.Add(
		"Docker Compose",
		LevelError,
		fmt.Sprintf("%s erkannt, aber docker compose/docker-compose fehlt; Updates würden sicher abgebrochen", detection.ComposeFile),
	)
}

func checkDirectory(report *Report, name, path string, required bool) {
	info, err := os.Stat(path)
	if err != nil {
		level := LevelWarning
		if required {
			level = LevelError
		}
		report.Add(name, level, err.Error())
		return
	}
	if !info.IsDir() {
		report.Add(name, LevelError, "Pfad ist kein Ordner: "+path)
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		report.Add(name, LevelError, "Ordner ist nicht lesbar: "+err.Error())
		return
	}
	report.Add(name, LevelOK, fmt.Sprintf("%s (%d Einträge)", path, len(entries)))
}

func checkWritable(report *Report, name, target string) {
	candidate := target
	for {
		info, err := os.Stat(candidate)
		if err == nil {
			if !info.IsDir() {
				report.Add(name, LevelError, "Pfad ist kein Ordner: "+candidate)
				return
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			report.Add(name, LevelError, err.Error())
			return
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			report.Add(name, LevelError, "kein existierender Elternordner gefunden")
			return
		}
		candidate = parent
	}

	temporary, err := os.CreateTemp(candidate, ".updater-doctor-*")
	if err != nil {
		report.Add(name, LevelError, fmt.Sprintf("nicht beschreibbar über %s: %v", candidate, err))
		return
	}
	path := temporary.Name()
	closeErr := temporary.Close()
	removeErr := os.Remove(path)
	if closeErr != nil {
		report.Add(name, LevelError, closeErr.Error())
		return
	}
	if removeErr != nil {
		report.Add(name, LevelWarning, "Testdatei konnte nicht entfernt werden: "+removeErr.Error())
		return
	}
	report.Add(name, LevelOK, "beschreibbar: "+target)
}

func checkCurrent(report *Report, cfg config.Config) {
	info, err := os.Stat(cfg.CurrentDir)
	if errors.Is(err, os.ErrNotExist) {
		report.Add("Current-Installation", LevelWarning, "noch nicht vorhanden: "+cfg.CurrentDir)
		return
	}
	if err != nil {
		report.Add("Current-Installation", LevelError, err.Error())
		return
	}
	if !info.IsDir() {
		report.Add("Current-Installation", LevelError, "Pfad ist kein Ordner: "+cfg.CurrentDir)
		return
	}

	installed, source, found, err := updatecheck.DetectInstalled(cfg.CurrentDir)
	if err != nil {
		report.Add("Current-Installation", LevelError, err.Error())
		return
	}
	if !found {
		report.Add("Current-Installation", LevelWarning, "kein .release-version- oder VERSION-Marker gefunden")
		return
	}

	releaseVersionPath := filepath.Join(cfg.CurrentDir, ".release-version")
	versionFilePath := filepath.Join(cfg.CurrentDir, "VERSION")
	if releaseData, releaseErr := os.ReadFile(releaseVersionPath); releaseErr == nil {
		if versionData, versionErr := os.ReadFile(versionFilePath); versionErr == nil {
			releaseVersion, parseReleaseErr := versionutil.Parse(strings.TrimSpace(string(releaseData)))
			fileVersion, parseFileErr := versionutil.Parse(strings.TrimSpace(string(versionData)))
			if parseReleaseErr != nil || parseFileErr != nil {
				report.Add("Current-Installation", LevelError, "Versionsmarker können nicht konsistent ausgewertet werden")
				return
			}
			if releaseVersion.Compare(fileVersion) != 0 {
				report.Add("Current-Installation", LevelError, fmt.Sprintf(".release-version %s und VERSION %s stimmen nicht überein", releaseVersion.String(), fileVersion.String()))
				return
			}
		}
	}

	projectMarker := filepath.Join(cfg.CurrentDir, ".release-project")
	if data, markerErr := os.ReadFile(projectMarker); markerErr == nil {
		actual := strings.TrimSpace(string(data))
		if actual != cfg.ProjectName {
			report.Add("Current-Installation", LevelError, fmt.Sprintf("Projektmarker %q passt nicht zu %q", actual, cfg.ProjectName))
			return
		}
	} else if !errors.Is(markerErr, os.ErrNotExist) {
		report.Add("Current-Installation", LevelError, markerErr.Error())
		return
	}

	report.Add("Current-Installation", LevelOK, fmt.Sprintf("Version %s (%s)", installed.String(), source))
}

func checkReleaseMetadata(report *Report, cfg config.Config) {
	info, err := os.Stat(cfg.ReleaseRoot)
	if errors.Is(err, os.ErrNotExist) {
		report.Add("Release-Metadaten", LevelWarning, "Release-Ordner noch nicht vorhanden")
		return
	}
	if err != nil {
		report.Add("Release-Metadaten", LevelError, err.Error())
		return
	}
	if !info.IsDir() {
		report.Add("Release-Metadaten", LevelError, "releaseDir ist kein Ordner")
		return
	}

	projectPath := filepath.Join(cfg.ReleaseRoot, ".project-name")
	versionPath := filepath.Join(cfg.ReleaseRoot, ".last-version")
	projectData, projectErr := os.ReadFile(projectPath)
	versionData, versionErr := os.ReadFile(versionPath)
	if errors.Is(projectErr, os.ErrNotExist) && errors.Is(versionErr, os.ErrNotExist) {
		report.Add("Release-Metadaten", LevelWarning, "noch keine Installationsmarker vorhanden")
		return
	}
	if projectErr != nil {
		report.Add("Release-Metadaten", LevelError, projectErr.Error())
		return
	}
	if versionErr != nil {
		report.Add("Release-Metadaten", LevelError, versionErr.Error())
		return
	}
	projectName := strings.TrimSpace(string(projectData))
	if projectName != cfg.ProjectName {
		report.Add("Release-Metadaten", LevelError, fmt.Sprintf("Projektmarker %q passt nicht zu %q", projectName, cfg.ProjectName))
		return
	}
	parsed, err := versionutil.Parse(strings.TrimSpace(string(versionData)))
	if err != nil {
		report.Add("Release-Metadaten", LevelError, err.Error())
		return
	}
	releaseDir := filepath.Join(cfg.ReleaseRoot, parsed.String())
	if _, err := os.Stat(releaseDir); err != nil {
		report.Add("Release-Metadaten", LevelError, fmt.Sprintf("markierter Release-Ordner fehlt: %s", releaseDir))
		return
	}
	report.Add("Release-Metadaten", LevelOK, fmt.Sprintf("letzte Version %s", parsed.String()))
}

func checkNewestSource(report *Report, cfg config.Config) {
	temporary, err := os.MkdirTemp("", "updater-doctor-source-*")
	if err != nil {
		report.Add("Release-Quelle", LevelError, err.Error())
		return
	}
	defer tools.RemoveTree(temporary)
	folder := cfg.SourceFolder
	if folder == "" {
		folder = cfg.DownloadDir
	}
	artifact, err := source.Resolve(context.Background(), source.Options{
		Type: cfg.SourceType, ProjectName: cfg.ProjectName, Folder: folder,
		URL: cfg.SourceURL, Repository: cfg.SourceRepository, WorkDir: temporary,
		ReleaseRoot: cfg.ReleaseRoot, Simulation: true,
	})
	if err != nil {
		report.Add("Release-Quelle", LevelWarning, err.Error())
		return
	}
	if artifact.Type == source.Repository {
		if err := archive.ValidateVersionFile(artifact.ContentDir, artifact.Version.String()); err != nil {
			report.Add("Release-Quelle", LevelError, err.Error())
			return
		}
		report.Add("Release-Quelle", LevelOK, fmt.Sprintf("Repository %s (Version %s)", artifact.Reference, artifact.Version.String()))
		return
	}
	if err := archive.Validate(artifact.ArchivePath); err != nil {
		report.Add("Release-Quelle", LevelError, fmt.Sprintf("%s: %v", filepath.Base(artifact.ArchivePath), err))
		return
	}
	extractDir := filepath.Join(temporary, "extract")
	if err := archive.Extract(artifact.ArchivePath, extractDir); err != nil {
		report.Add("Release-Quelle", LevelError, err.Error())
		return
	}
	contentRoot, err := archive.ResolveContentRoot(extractDir)
	if err != nil {
		report.Add("Release-Quelle", LevelError, err.Error())
		return
	}
	if err := archive.ValidateVersionFile(contentRoot, artifact.Version.String()); err != nil {
		report.Add("Release-Quelle", LevelError, err.Error())
		return
	}
	report.Add("Release-Quelle", LevelOK, fmt.Sprintf("%s (Version %s, ZIP und VERSION gültig)", artifact.Reference, artifact.Version.String()))
}

func checkUpdateStatus(report *Report, cfg config.Config) {
	result, err := updatecheck.Run(cfg)
	if err != nil {
		return
	}
	switch result.Status {
	case updatecheck.StatusCurrent:
		report.Add("Update-Status", LevelOK, "installierte Version ist aktuell: "+result.Available.String())
	case updatecheck.StatusUpdateAvailable:
		report.Add("Update-Status", LevelWarning, fmt.Sprintf("Update verfügbar: %s → %s", result.Installed.String(), result.Available.String()))
	case updatecheck.StatusNotInstalled:
		report.Add("Update-Status", LevelWarning, "keine Installation erkannt; verfügbar ist "+result.Available.String())
	case updatecheck.StatusLocalNewer:
		report.Add("Update-Status", LevelWarning, fmt.Sprintf("lokale Version %s ist neuer als Quelle %s", result.Installed.String(), result.Available.String()))
	}
}
