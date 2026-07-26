package cleanup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"release-updater/lib/backup"
	"release-updater/lib/config"
	"release-updater/lib/inventory"
	"release-updater/lib/tools"
	"release-updater/lib/updatecheck"
	versionutil "release-updater/lib/version"
)

type Result struct {
	KeepReleases   int      `json:"keepReleases"`
	KeepBackups    int      `json:"keepBackups"`
	RemovedRelease []string `json:"removedReleases"`
	RemovedBackup  []string `json:"removedBackups"`
	KeptRelease    []string `json:"keptReleases"`
	KeptBackup     []string `json:"keptBackups"`
	Plan           bool     `json:"plan"`
}

func Run(cfg config.Config, keepOverride int, plan bool) (Result, error) {
	keepReleases := cfg.KeepReleases
	keepBackups := cfg.KeepBackups
	if keepOverride >= 0 {
		keepReleases = keepOverride
		keepBackups = keepOverride
	}
	result := Result{KeepReleases: keepReleases, KeepBackups: keepBackups, Plan: plan}

	inv, err := inventory.List(cfg)
	if err != nil {
		return Result{}, err
	}
	protected := map[string]bool{}
	installed, _, found, err := updatecheck.DetectInstalled(cfg.CurrentDir)
	if err != nil {
		return Result{}, err
	}
	if found {
		protected[installed.String()] = true
		previous := ""
		for _, release := range inv.Releases {
			parsed, _ := versionutil.Parse(release.Version)
			if release.Validated && parsed.Compare(installed) < 0 {
				previous = release.Version
				break
			}
		}
		if previous != "" {
			protected[previous] = true
		}
	}

	keptRegular := 0
	for _, release := range inv.Releases {
		keep := protected[release.Version]
		if !keep && keptRegular < keepReleases {
			keep = true
			keptRegular++
		}
		if keep {
			result.KeptRelease = append(result.KeptRelease, release.Path)
			continue
		}
		result.RemovedRelease = append(result.RemovedRelease, release.Path)
		if !plan {
			if err := tools.RemoveTree(release.Path); err != nil {
				return Result{}, fmt.Errorf("Release kann nicht entfernt werden (%s): %w", release.Path, err)
			}
		}
	}

	backups, err := backup.List(cfg)
	if err != nil {
		return Result{}, err
	}
	for index, item := range backups {
		if index < keepBackups {
			result.KeptBackup = append(result.KeptBackup, item.Path)
			continue
		}
		result.RemovedBackup = append(result.RemovedBackup, item.Path)
		if !plan {
			if err := tools.RemoveTree(item.Path); err != nil {
				return Result{}, fmt.Errorf("Backup kann nicht entfernt werden (%s): %w", item.Path, err)
			}
		}
	}

	sort.Strings(result.RemovedRelease)
	sort.Strings(result.RemovedBackup)
	cleanupEmpty(cfg.ReleaseRoot)
	cleanupEmpty(cfg.BackupRoot)
	return result, nil
}

func cleanupEmpty(path string) {
	entries, err := os.ReadDir(path)
	if err == nil && len(entries) == 0 {
		_ = os.Remove(filepath.Clean(path))
	}
}
