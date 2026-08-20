package projectstatus

import (
	"context"
	"errors"
	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/history"
	"github.com/r14r/update-cli/lib/inventory"
	"github.com/r14r/update-cli/lib/projectsetup"
	"github.com/r14r/update-cli/lib/updatecheck"
	"os"
)

type Result struct {
	ProjectName      string `json:"projectName"`
	Mode             string `json:"mode"`
	SourceType       string `json:"sourceType"`
	SourceReference  string `json:"sourceReference,omitempty"`
	SourceError      string `json:"sourceError,omitempty"`
	ConfigFile       string `json:"configFile"`
	CurrentDir       string `json:"currentDir"`
	ReleaseDir       string `json:"releaseDir"`
	InstalledVersion string `json:"installedVersion,omitempty"`
	AvailableVersion string `json:"availableVersion,omitempty"`
	UpdateAvailable  bool   `json:"updateAvailable"`
	State            string `json:"state"`
	SetupAvailable   bool   `json:"setupAvailable"`
	SetupPath        string `json:"setupPath,omitempty"`
	BackupCount      int    `json:"backupCount"`
	LatestBackup     string `json:"latestBackup,omitempty"`
	HistoryEntries   int    `json:"historyEntries"`
	LockState        string `json:"lockState,omitempty"`
	DockerLifecycle  string `json:"dockerLifecycle"`
}

func Run(ctx context.Context, c config.Config) (Result, error) {
	r := Result{ProjectName: c.ProjectName, Mode: c.Mode, SourceType: c.Source.Type, ConfigFile: c.ConfigFile, CurrentDir: c.CurrentDir, ReleaseDir: c.ReleaseRoot, DockerLifecycle: c.Docker.Lifecycle}
	if c.Source.Type == "url" {
		r.SourceReference = c.Source.URL
	} else if c.Source.Type == "repository" {
		r.SourceReference = c.Source.Repository
	} else {
		r.SourceReference = c.Source.Folder
	}
	if p, ok, e := projectsetup.Detect(c); e != nil {
		return r, e
	} else {
		r.SetupAvailable = ok
		r.SetupPath = p
	}
	local, e := inventory.ListLocal(c)
	if e != nil {
		return r, e
	}
	r.BackupCount = len(local.Backups)
	if len(local.Backups) > 0 {
		r.LatestBackup = local.Backups[0].Name
	}
	h, e := history.List(c.HistoryFile, 0)
	if e != nil {
		return r, e
	}
	r.HistoryEntries = len(h)
	check, e := updatecheck.Run(ctx, c)
	if e != nil {
		return r, e
	}
	r.InstalledVersion = check.InstalledVersion
	r.AvailableVersion = check.AvailableVersion
	r.SourceError = check.SourceError
	if check.SourceError != "" {
		if r.InstalledVersion != "" {
			r.State = "source-error"
		} else {
			r.State = "empty"
		}
		return r, nil
	}
	switch check.Status {
	case updatecheck.StatusNotInstalled:
		r.State = "not-installed"
		r.UpdateAvailable = true
	case updatecheck.StatusUpdateAvailable:
		r.State = "update-available"
		r.UpdateAvailable = true
	case updatecheck.StatusCurrent:
		r.State = "current"
	case updatecheck.StatusLocalNewer:
		r.State = "local-newer"
	}
	return r, nil
}
func CurrentExists(c config.Config) bool { i, e := os.Stat(c.CurrentDir); return e == nil && i.IsDir() }
func IsMissing(err error) bool           { return errors.Is(err, os.ErrNotExist) }
