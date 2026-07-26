package projectstatus

import (
	"errors"
	"os"
	"path/filepath"

	"release-updater/lib/backup"
	"release-updater/lib/config"
	"release-updater/lib/history"
	"release-updater/lib/updatecheck"
)

type Result struct {
	ProjectName      string `json:"projectName"`
	SourceType       string `json:"sourceType"`
	SourceReference  string `json:"sourceReference,omitempty"`
	ConfigFile       string `json:"configFile"`
	CurrentDir       string `json:"currentDir"`
	ReleaseDir       string `json:"releaseDir"`
	InstalledVersion string `json:"installedVersion,omitempty"`
	InstalledSource  string `json:"installedSource,omitempty"`
	AvailableVersion string `json:"availableVersion,omitempty"`
	ArchivePath      string `json:"archivePath,omitempty"`
	UpdateAvailable  bool   `json:"updateAvailable"`
	State            string `json:"state"`
	SetupScript      bool   `json:"setupScript"`
	SetupCommands    int    `json:"setupCommands"`
	BackupCount      int    `json:"backupCount"`
	LatestBackup     string `json:"latestBackup,omitempty"`
	HistoryEntries   int    `json:"historyEntries"`
}

func Run(cfg config.Config) (Result, error) {
	result := Result{
		ProjectName:     cfg.ProjectName,
		SourceType:      cfg.SourceType,
		SourceReference: sourceReference(cfg),
		ConfigFile:      cfg.ConfigFile,
		CurrentDir:      cfg.CurrentDir,
		ReleaseDir:      cfg.ReleaseRoot,
		SetupCommands:   len(cfg.SetupCommands),
	}
	if info, err := os.Stat(filepath.Join(cfg.CurrentDir, "setup.sh")); err == nil && !info.IsDir() {
		result.SetupScript = true
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	}
	backups, err := backup.List(cfg)
	if err != nil {
		return Result{}, err
	}
	result.BackupCount = len(backups)
	if len(backups) > 0 {
		result.LatestBackup = backups[0].Name
	}
	historyEntries, err := history.List(cfg.HistoryFile, 0)
	if err != nil {
		return Result{}, err
	}
	result.HistoryEntries = len(historyEntries)

	installed, source, installedFound, err := updatecheck.DetectInstalled(cfg.CurrentDir)
	if err != nil {
		return Result{}, err
	}
	if installedFound {
		result.InstalledVersion = installed.String()
		result.InstalledSource = source
	}

	available, err := updatecheck.Run(cfg)
	if err != nil {
		if installedFound {
			result.State = "no-download"
		} else {
			result.State = "empty"
		}
		return result, nil
	}
	result.AvailableVersion = available.Available.String()
	result.ArchivePath = available.ArchivePath
	switch available.Status {
	case updatecheck.StatusNotInstalled:
		result.State = "not-installed"
		result.UpdateAvailable = true
	case updatecheck.StatusUpdateAvailable:
		result.State = "update-available"
		result.UpdateAvailable = true
	case updatecheck.StatusCurrent:
		result.State = "current"
	case updatecheck.StatusLocalNewer:
		result.State = "local-newer"
	}
	return result, nil
}

func sourceReference(cfg config.Config) string {
	switch cfg.SourceType {
	case "url":
		return cfg.SourceURL
	case "repository":
		return cfg.SourceRepository
	default:
		if cfg.SourceFolder != "" {
			return cfg.SourceFolder
		}
		return cfg.DownloadDir
	}
}
