package updatecheck

import (
	"context"
	"errors"
	"fmt"
	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/source"
	versionutil "github.com/r14r/update-cli/lib/version"
	"os"
	"path/filepath"
	"strings"
)

type Status string

const (
	StatusNotInstalled    Status = "not-installed"
	StatusUpdateAvailable Status = "update-available"
	StatusCurrent         Status = "current"
	StatusLocalNewer      Status = "local-newer"
)

type Result struct {
	ProjectName      string              `json:"projectName"`
	Installed        versionutil.Version `json:"-"`
	InstalledVersion string              `json:"installed,omitempty"`
	InstalledFound   bool                `json:"installedFound"`
	InstalledSource  string              `json:"installedSource,omitempty"`
	InstalledCommit  string              `json:"installedCommit,omitempty"`
	Available        versionutil.Version `json:"-"`
	AvailableVersion string              `json:"available,omitempty"`
	AvailableCommit  string              `json:"availableCommit,omitempty"`
	SourceType       string              `json:"sourceType"`
	SourceReference  string              `json:"sourceReference,omitempty"`
	Status           Status              `json:"status"`
	SourceError      string              `json:"sourceError,omitempty"`
}

func Run(ctx context.Context, c config.Config) (Result, error) {
	installed, src, found, err := DetectInstalled(c.CurrentDir)
	if err != nil {
		return Result{}, err
	}
	r := Result{ProjectName: c.ProjectName, Installed: installed, InstalledFound: found, InstalledSource: src, InstalledCommit: DetectInstalledCommit(c.CurrentDir)}
	if found {
		r.InstalledVersion = installed.String()
	}
	m, err := source.Discover(ctx, source.Options{ProjectName: c.ProjectName, Mode: c.Mode, Source: c.Source, RepositoryCacheDir: c.RepositoryCacheDir, AllowHTTP: c.Security.AllowHTTP, MaxArchiveBytes: c.Security.MaxArchiveBytes})
	if err != nil {
		r.SourceError = err.Error()
		return r, nil
	}
	r.Available = m.Version
	r.AvailableVersion = m.Version.String()
	r.SourceType = m.Type
	r.SourceReference = m.Reference
	r.AvailableCommit = m.Commit
	if !found {
		r.Status = StatusNotInstalled
		return r, nil
	}
	switch versionutil.CompareForProject(c.ProjectName, installed, m.Version) {
	case -1:
		r.Status = StatusUpdateAvailable
	case 0:
		if c.Mode == config.ModePull && m.Commit != "" && r.InstalledCommit != m.Commit {
			r.Status = StatusUpdateAvailable
		} else {
			r.Status = StatusCurrent
		}
	default:
		r.Status = StatusLocalNewer
	}
	return r, nil
}
func DetectInstalledCommit(current string) string {
	b, err := os.ReadFile(filepath.Join(current, ".release-commit"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func DetectInstalled(current string) (versionutil.Version, string, bool, error) {
	for _, p := range []string{filepath.Join(current, ".release-version"), filepath.Join(current, "VERSION")} {
		b, err := os.ReadFile(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return versionutil.Version{}, "", false, fmt.Errorf("installierte Version kann nicht gelesen werden (%s): %w", p, err)
		}
		v, err := versionutil.Parse(strings.TrimSpace(string(b)))
		if err != nil {
			return v, "", false, fmt.Errorf("ungültige installierte Version in %s: %w", p, err)
		}
		return v, p, true, nil
	}
	return versionutil.Version{}, "", false, nil
}
