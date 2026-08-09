package cleanup

import (
	"fmt"
	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/inventory"
	"github.com/r14r/update-cli/lib/tools"
	"github.com/r14r/update-cli/lib/updatecheck"
	versionutil "github.com/r14r/update-cli/lib/version"
	"os"
	"sort"
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

func Run(c config.Config, keepOverride int, plan bool) (Result, error) {
	kr, kb := c.KeepReleases, c.KeepBackups
	if keepOverride >= 0 {
		kr, kb = keepOverride, keepOverride
	}
	out := Result{KeepReleases: kr, KeepBackups: kb, Plan: plan}
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
			if r.Validated && v.Compare(installed) < 0 {
				protected[r.Version] = true
				break
			}
		}
	}
	kept := 0
	for _, r := range local.Releases {
		keep := protected[r.Version]
		if !keep && kept < kr {
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
	for i, b := range local.Backups {
		if i < kb {
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
	sort.Strings(out.RemovedRelease)
	sort.Strings(out.RemovedBackup)
	if !plan {
		cleanupEmpty(c.ReleaseRoot)
		cleanupEmpty(c.BackupRoot)
	}
	return out, nil
}
func cleanupEmpty(p string) {
	if e, err := os.ReadDir(p); err == nil && len(e) == 0 {
		_ = os.Remove(p)
	}
}
