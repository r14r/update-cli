package inventory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"release-updater/lib/backup"
	"release-updater/lib/config"
	"release-updater/lib/source"
	"release-updater/lib/updatecheck"
	versionutil "release-updater/lib/version"
)

type Release struct {
	Path      string    `json:"path"`
	Version   string    `json:"version"`
	Active    bool      `json:"active"`
	Modified  time.Time `json:"modifiedAt"`
	Validated bool      `json:"validated"`
}

type Result struct {
	ProjectName string                    `json:"projectName"`
	SourceType  string                    `json:"sourceType"`
	Downloads   []versionutil.ArchiveInfo `json:"downloads"`
	Releases    []Release                 `json:"releases"`
	Backups     []backup.Item             `json:"backups"`
}

func List(cfg config.Config) (Result, error) {
	folder := cfg.SourceFolder
	if folder == "" {
		folder = cfg.DownloadDir
	}
	archives := []versionutil.ArchiveInfo{}
	if cfg.SourceType == "" || cfg.SourceType == source.Download {
		var err error
		archives, err = versionutil.ListArchives(folder, cfg.ProjectName)
		if err != nil {
			return Result{}, err
		}
	} else {
		artifact, err := source.Discover(context.Background(), source.Options{
			Type: cfg.SourceType, ProjectName: cfg.ProjectName, Folder: folder,
			URL: cfg.SourceURL, Repository: cfg.SourceRepository,
		})
		if err != nil {
			return Result{}, err
		}
		archives = append(archives, versionutil.ArchiveInfo{
			Path: artifact.Reference, Name: artifact.Type, Version: artifact.Version, VersionS: artifact.Version.String(),
		})
	}
	installed, _, found, err := updatecheck.DetectInstalled(cfg.CurrentDir)
	if err != nil {
		return Result{}, err
	}
	releases, err := listReleases(cfg, installed, found)
	if err != nil {
		return Result{}, err
	}
	backups, err := backup.List(cfg)
	if err != nil {
		return Result{}, err
	}
	return Result{ProjectName: cfg.ProjectName, SourceType: cfg.SourceType, Downloads: archives, Releases: releases, Backups: backups}, nil
}

func listReleases(cfg config.Config, installed versionutil.Version, installedFound bool) ([]Release, error) {
	entries, err := os.ReadDir(cfg.ReleaseRoot)
	if errors.Is(err, os.ErrNotExist) {
		return []Release{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Release-Ordner kann nicht gelesen werden: %w", err)
	}

	result := make([]Release, 0)
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		parsed, parseErr := versionutil.Parse(entry.Name())
		if parseErr != nil {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil, infoErr
		}
		path := filepath.Join(cfg.ReleaseRoot, entry.Name())
		validated := markerEquals(filepath.Join(path, ".release-version"), parsed.String()) &&
			markerEquals(filepath.Join(path, ".release-project"), cfg.ProjectName)
		result = append(result, Release{
			Path:      path,
			Version:   parsed.String(),
			Active:    installedFound && parsed.Compare(installed) == 0,
			Modified:  info.ModTime(),
			Validated: validated,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		left, _ := versionutil.Parse(result[i].Version)
		right, _ := versionutil.Parse(result[j].Version)
		return left.Compare(right) > 0
	})
	return result, nil
}

func markerEquals(path, expected string) bool {
	data, err := os.ReadFile(path)
	return err == nil && strings.TrimSpace(string(data)) == expected
}
