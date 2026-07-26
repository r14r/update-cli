package rollback

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"release-updater/lib/config"
	"release-updater/lib/inventory"
	rsyncutil "release-updater/lib/rsync"
	"release-updater/lib/tools"
	"release-updater/lib/updatecheck"
	versionutil "release-updater/lib/version"
)

type Result struct {
	FromVersion string           `json:"fromVersion,omitempty"`
	ToVersion   string           `json:"toVersion"`
	ReleaseDir  string           `json:"releaseDir"`
	CurrentDir  string           `json:"currentDir"`
	Sync        rsyncutil.Result `json:"sync"`
}

func Resolve(cfg config.Config, requested string) (inventory.Release, error) {
	result, err := inventory.List(cfg)
	if err != nil {
		return inventory.Release{}, err
	}
	requested = strings.TrimSpace(requested)
	if requested != "" {
		parsed, err := versionutil.Parse(strings.TrimPrefix(requested, "v"))
		if err != nil {
			return inventory.Release{}, fmt.Errorf("Rollback-Version ist ungültig: %w", err)
		}
		for _, release := range result.Releases {
			if release.Version == parsed.String() {
				if !release.Validated {
					return inventory.Release{}, fmt.Errorf("Release %s ist nicht validiert", release.Version)
				}
				return release, nil
			}
		}
		return inventory.Release{}, fmt.Errorf("Release %s wurde nicht gefunden", parsed.String())
	}

	installed, _, found, err := updatecheck.DetectInstalled(cfg.CurrentDir)
	if err != nil {
		return inventory.Release{}, err
	}
	if !found {
		return inventory.Release{}, errors.New("keine installierte Version erkannt; Rollback-Version explizit angeben")
	}
	for _, release := range result.Releases {
		parsed, _ := versionutil.Parse(release.Version)
		if release.Validated && parsed.Compare(installed) < 0 {
			return release, nil
		}
	}
	return inventory.Release{}, errors.New("kein vorheriges validiertes Release vorhanden")
}

func Apply(ctx context.Context, cfg config.Config, release inventory.Release, dryRun bool) (Result, error) {
	from := ""
	if installed, _, found, err := updatecheck.DetectInstalled(cfg.CurrentDir); err != nil {
		return Result{}, err
	} else if found {
		from = installed.String()
	}
	if info, err := os.Stat(release.Path); err != nil || !info.IsDir() {
		if err == nil {
			err = errors.New("kein Ordner")
		}
		return Result{}, fmt.Errorf("Rollback-Release ist nicht verfügbar: %w", err)
	}
	logFile := filepath.Join(cfg.ConfigDir, "logs", "rollback-"+time.Now().Format("20060102-150405")+".log")
	syncResult, err := rsyncutil.Current(ctx, release.Path, cfg.CurrentDir, logFile, dryRun)
	if err != nil {
		return Result{}, err
	}
	if !dryRun {
		if err := tools.WriteMarker(cfg.ReleaseRoot, ".last-version", release.Version); err != nil {
			return Result{}, err
		}
		if err := tools.WriteMarker(cfg.ReleaseRoot, ".last-archive", "rollback:"+release.Version); err != nil {
			return Result{}, err
		}
	}
	return Result{FromVersion: from, ToVersion: release.Version, ReleaseDir: release.Path, CurrentDir: cfg.CurrentDir, Sync: syncResult}, nil
}
