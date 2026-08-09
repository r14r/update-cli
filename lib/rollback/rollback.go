package rollback

import (
	"context"
	"path/filepath"
	"time"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/inventory"
	rsyncutil "github.com/r14r/update-cli/lib/rsync"
	"github.com/r14r/update-cli/lib/updatecheck"
)

type Result struct {
	FromVersion string           `json:"fromVersion,omitempty"`
	ToVersion   string           `json:"toVersion"`
	ReleaseDir  string           `json:"releaseDir"`
	CurrentDir  string           `json:"currentDir"`
	Sync        rsyncutil.Result `json:"sync"`
}

func Resolve(c config.Config, requested string) (inventory.Release, error) {
	return inventory.FindRelease(c, requested)
}

func Apply(ctx context.Context, c config.Config, r inventory.Release) (Result, error) {
	from := ""
	if v, _, found, err := updatecheck.DetectInstalled(c.CurrentDir); err != nil {
		return Result{}, err
	} else if found {
		from = v.String()
	}
	log := filepath.Join(c.ConfigDir, "logs", "rollback-"+time.Now().Format("20060102-150405")+".log")
	syncResult, err := rsyncutil.Current(ctx, r.Path, c.CurrentDir, log, false, c.Preserve)
	if err != nil {
		return Result{}, err
	}
	return Result{FromVersion: from, ToVersion: r.Version, ReleaseDir: r.Path, CurrentDir: c.CurrentDir, Sync: syncResult}, nil
}
