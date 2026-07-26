package updatecheck

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"release-updater/lib/config"
	"release-updater/lib/source"
	versionutil "release-updater/lib/version"
)

type Status string

const (
	StatusNotInstalled    Status = "not-installed"
	StatusUpdateAvailable Status = "update-available"
	StatusCurrent         Status = "current"
	StatusLocalNewer      Status = "local-newer"
)

type Result struct {
	ProjectName     string
	Installed       versionutil.Version
	InstalledFound  bool
	InstalledSource string
	Available       versionutil.Version
	ArchivePath     string
	SourceType      string
	Status          Status
}

func Run(cfg config.Config) (Result, error) {
	folder := cfg.SourceFolder
	if folder == "" {
		folder = cfg.DownloadDir
	}
	artifact, err := source.Discover(context.Background(), source.Options{
		Type:        cfg.SourceType,
		ProjectName: cfg.ProjectName,
		Folder:      folder,
		URL:         cfg.SourceURL,
		Repository:  cfg.SourceRepository,
	})
	if err != nil {
		return Result{}, err
	}

	installed, installedSource, found, err := DetectInstalled(cfg.CurrentDir)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		ProjectName:     cfg.ProjectName,
		Installed:       installed,
		InstalledFound:  found,
		InstalledSource: installedSource,
		Available:       artifact.Version,
		ArchivePath:     artifact.Reference,
		SourceType:      artifact.Type,
	}
	if artifact.Type == source.Download {
		result.ArchivePath = artifact.ArchivePath
	}
	if !found {
		result.Status = StatusNotInstalled
		return result, nil
	}

	switch installed.Compare(artifact.Version) {
	case -1:
		result.Status = StatusUpdateAvailable
	case 0:
		result.Status = StatusCurrent
	default:
		result.Status = StatusLocalNewer
	}
	return result, nil
}

func DetectInstalled(currentDir string) (versionutil.Version, string, bool, error) {
	candidates := []string{
		filepath.Join(currentDir, ".release-version"),
		filepath.Join(currentDir, "VERSION"),
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return versionutil.Version{}, "", false, fmt.Errorf("installierte Version kann nicht gelesen werden (%s): %w", path, err)
		}
		value := strings.TrimSpace(string(data))
		parsed, err := versionutil.Parse(value)
		if err != nil {
			return versionutil.Version{}, "", false, fmt.Errorf("ungültige installierte Version in %s: %w", path, err)
		}
		return parsed, path, true, nil
	}
	return versionutil.Version{}, "", false, nil
}
