package doctor

import (
	"context"
	"errors"
	"fmt"
	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/inventory"
	"github.com/r14r/update-cli/lib/projectdocker"
	"github.com/r14r/update-cli/lib/projectsetup"
	rsyncutil "github.com/r14r/update-cli/lib/rsync"
	"github.com/r14r/update-cli/lib/source"
	"github.com/r14r/update-cli/lib/tools"
	"github.com/r14r/update-cli/lib/updatecheck"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func (r *Report) Add(n string, l Level, d string) {
	r.Checks = append(r.Checks, Check{Name: n, Level: l, Detail: d})
}
func (r Report) ErrorCount() int {
	n := 0
	for _, c := range r.Checks {
		if c.Level == LevelError {
			n++
		}
	}
	return n
}
func (r Report) WarningCount() int {
	n := 0
	for _, c := range r.Checks {
		if c.Level == LevelWarning {
			n++
		}
	}
	return n
}
func Run(ctx context.Context, root string, cfg config.Config) Report {
	r := Report{Root: root, Config: &cfg}
	if d, e := rsyncutil.Describe(); e != nil {
		r.Add("rsync", LevelError, e.Error())
	} else {
		r.Add("rsync", LevelOK, d)
	}
	lock := filepath.Join(root, ".release-update.lock")
	if _, e := os.Stat(lock); errors.Is(e, os.ErrNotExist) {
		r.Add("Update-Sperre", LevelOK, "keine aktive Sperre")
	} else if e != nil {
		r.Add("Update-Sperre", LevelError, e.Error())
	} else {
		stale, detail := tools.IsStaleLock(lock)
		if stale {
			r.Add("Update-Sperre", LevelWarning, "veraltete Sperre: "+detail)
		} else {
			r.Add("Update-Sperre", LevelError, "aktive/unklare Sperre: "+detail)
		}
	}
	if p, ok, e := projectsetup.Detect(cfg); e != nil {
		r.Add("Projekt-Setup", LevelError, e.Error())
	} else if !ok {
		r.Add("Projekt-Setup", LevelOK, "kein Setup konfiguriert")
	} else if projectsetup.IsManifest(p) {
		if _, e := projectsetup.ParseManifest(p); e != nil {
			r.Add("Projekt-Setup", LevelError, e.Error())
		} else {
			r.Add("Projekt-Setup", LevelOK, "gültiges Manifest: "+p)
		}
	} else {
		r.Add("Projekt-Setup", LevelWarning, "Legacy-Setup: "+p)
	}
	r.Add("Docker lifecycle", LevelOK, cfg.Docker.Lifecycle)
	if cfg.Docker.Lifecycle == "disabled" {
		r.Add("Docker Compose", LevelOK, "übersprungen; Docker-Lifecycle deaktiviert")
	} else {
		d, e := projectdocker.Detect(cfg.CurrentDir)
		if e != nil {
			if cfg.Docker.Lifecycle == "required" {
				r.Add("Docker Compose", LevelError, e.Error())
			} else {
				r.Add("Docker Compose", LevelWarning, e.Error())
			}
		} else if !d.Detected {
			r.Add("Docker Compose", LevelOK, "kein Compose-Projekt erkannt")
		} else if _, e := projectdocker.Running(ctx, cfg.CurrentDir); e != nil {
			if cfg.Docker.Lifecycle == "required" {
				r.Add("Docker Compose", LevelError, e.Error())
			} else {
				r.Add("Docker Compose", LevelWarning, e.Error())
			}
		} else {
			r.Add("Docker Compose", LevelOK, filepath.Base(d.ComposeFile)+" erkannt; Statusabfrage erfolgreich")
		}
	}
	local, e := inventory.ListLocal(cfg)
	if e != nil {
		r.Add("Lokales Inventar", LevelError, e.Error())
	} else {
		invalid := 0
		for _, x := range local.Releases {
			if !x.Validated {
				invalid++
			}
		}
		r.Add("Lokales Inventar", LevelOK, fmt.Sprintf("%d Releases, %d Backups, %d ungültige Releases", len(local.Releases), len(local.Backups), invalid))
	}
	m, e := source.Discover(ctx, source.Options{ProjectName: cfg.ProjectName, Mode: cfg.Mode, Source: cfg.Source, RepositoryCacheDir: cfg.RepositoryCacheDir, AllowHTTP: cfg.Security.AllowHTTP, MaxArchiveBytes: cfg.Security.MaxArchiveBytes})
	if e != nil {
		r.Add("Release-Quelle", LevelWarning, e.Error())
	} else {
		r.Add("Release-Quelle", LevelOK, fmt.Sprintf("%s Version %s", m.Type, m.VersionText))
	}
	if v, src, found, e := updatecheck.DetectInstalled(cfg.CurrentDir); e != nil {
		r.Add("Current", LevelError, e.Error())
	} else if !found {
		r.Add("Current", LevelWarning, "keine installierte Version erkannt")
	} else {
		r.Add("Current", LevelOK, fmt.Sprintf("Version %s (%s)", v.String(), src))
	}
	for name, p := range map[string]string{"Release-Ziel": cfg.ReleaseRoot, "Current-Ziel": cfg.CurrentDir, "Backup-Ziel": cfg.BackupRoot} {
		if _, e := tools.CanonicalInside(cfg.RootDir, p, true); e != nil {
			r.Add(name, LevelError, e.Error())
		} else {
			r.Add(name, LevelOK, p)
		}
	}
	if cfg.Source.Type == "repository" {
		if _, e := exec.LookPath("git"); e != nil {
			r.Add("git", LevelError, "git fehlt")
		}
	}
	if strings.TrimSpace(cfg.Healthcheck.Type) != "" && cfg.Healthcheck.Type != "none" {
		r.Add("Healthcheck", LevelOK, cfg.Healthcheck.Type)
	} else {
		r.Add("Healthcheck", LevelWarning, "kein projektbezogener Healthcheck konfiguriert")
	}
	return r
}
