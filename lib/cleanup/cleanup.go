package cleanup

import (
	"fmt"
	"os"
	"sort"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/inventory"
	"github.com/r14r/update-cli/lib/tools"
	"github.com/r14r/update-cli/lib/updatecheck"
	versionutil "github.com/r14r/update-cli/lib/version"
)

type Result struct {
	KeepReleases   int      `json:"keepReleases"`
	KeepBackups    int      `json:"keepBackups"`
	ReleaseOnly    bool     `json:"releaseOnly,omitempty"`
	RemovedRelease []string `json:"removedReleases"`
	RemovedBackup  []string `json:"removedBackups"`
	KeptRelease    []string `json:"keptReleases"`
	KeptBackup     []string `json:"keptBackups"`
	Plan           bool     `json:"plan"`
}

// Run applies the configured cleanup policy to both releases and backups.
func Run(c config.Config, keepOverride int, plan bool) (Result, error) {
	kr, kb := c.KeepReleases, c.KeepBackups
	if keepOverride >= 0 {
		kr, kb = keepOverride, keepOverride
	}
	return run(c, kr, kb, plan, true)
}

// RunReleases cleans only the release directory. Backups are deliberately left
// untouched. By default --clean keeps no extra releases beyond the protected
// installed and rollback-safe previous release. --keep N can retain N
// additional releases.
func RunReleases(c config.Config, keepOverride int, plan bool) (Result, error) {
	kr := 0
	if keepOverride >= 0 {
		kr = keepOverride
	}
	return run(c, kr, c.KeepBackups, plan, false)
}

func run(c config.Config, keepReleases, keepBackups int, plan, cleanBackups bool) (Result, error) {
	out := Result{KeepReleases: keepReleases, KeepBackups: keepBackups, ReleaseOnly: !cleanBackups, Plan: plan}
	local, err := inventory.ListLocal(c)
	if err != nil {
		return out, err
	}

	protected := map[string]bool{}
	if installed, _, found, e := updatecheck.DetectInstalled(c.CurrentDir); e != nil {
		return out, e
	} else if found {
		protected[installed.String()] = true
		for _, r := range local.Releases {
			v, _ := versionutil.Parse(r.Version)
			if r.Validated && versionutil.CompareForProject(c.ProjectName, v, installed) < 0 {
				protected[r.Version] = true
				break
			}
		}
	} else if !cleanBackups {
		// Release-only cleanup should never erase the entire local release set
		// merely because current/ is missing. Keep the newest validated release
		// as a safe recovery/install candidate.
		for _, r := range local.Releases {
			if r.Validated {
				protected[r.Version] = true
				break
			}
		}
	}

	kept := 0
	for _, r := range local.Releases {
		keep := protected[r.Version]
		if !keep && kept < keepReleases {
			keep = true
			kept++
		}
		if keep {
			out.KeptRelease = append(out.KeptRelease, r.Path)
		} else {
			out.RemovedRelease = append(out.RemovedRelease, r.Path)
			if !plan {
				if err := tools.RemoveTree(r.Path); err != nil {
					return out, fmt.Errorf("Release kann nicht entfernt werden: %w", err)
				}
			}
		}
	}

	if cleanBackups {
		for i, b := range local.Backups {
			if i < keepBackups {
				out.KeptBackup = append(out.KeptBackup, b.Path)
			} else {
				out.RemovedBackup = append(out.RemovedBackup, b.Path)
				if !plan {
					if err := tools.RemoveTree(b.Path); err != nil {
						return out, err
					}
				}
			}
		}
	}

	sort.Strings(out.RemovedRelease)
	sort.Strings(out.RemovedBackup)
	if !plan {
		cleanupEmpty(c.ReleaseRoot)
		if cleanBackups {
			cleanupEmpty(c.BackupRoot)
		}
	}
	return out, nil
}

func cleanupEmpty(p string) {
	if e, err := os.ReadDir(p); err == nil && len(e) == 0 {
		_ = os.Remove(p)
	}
}
