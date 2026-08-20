package inventory

import (
	"context"
	"errors"
	"fmt"
	"github.com/r14r/update-cli/lib/backup"
	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/source"
	"github.com/r14r/update-cli/lib/updatecheck"
	versionutil "github.com/r14r/update-cli/lib/version"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Release struct {
	Path      string    `json:"path"`
	Version   string    `json:"version"`
	Active    bool      `json:"active"`
	Modified  time.Time `json:"modifiedAt"`
	Validated bool      `json:"validated"`
}
type LocalResult struct {
	ProjectName string        `json:"projectName"`
	Releases    []Release     `json:"releases"`
	Backups     []backup.Item `json:"backups"`
}
type Result struct {
	LocalResult
	SourceType  string                    `json:"sourceType"`
	Downloads   []versionutil.ArchiveInfo `json:"downloads,omitempty"`
	Remote      *source.Metadata          `json:"remote,omitempty"`
	SourceError string                    `json:"sourceError,omitempty"`
}

func ListLocal(c config.Config) (LocalResult, error) {
	installed, _, found, err := updatecheck.DetectInstalled(c.CurrentDir)
	if err != nil {
		return LocalResult{}, err
	}
	rels, err := listReleases(c, installed, found)
	if err != nil {
		return LocalResult{}, err
	}
	backs, err := backup.List(c)
	if err != nil {
		return LocalResult{}, err
	}
	return LocalResult{ProjectName: c.ProjectName, Releases: rels, Backups: backs}, nil
}
func List(ctx context.Context, c config.Config) (Result, error) {
	local, err := ListLocal(c)
	if err != nil {
		return Result{}, err
	}
	r := Result{LocalResult: local, SourceType: c.Source.Type}
	if c.Source.Type == source.Download {
		a, err := versionutil.ListArchives(c.Source.Folder, c.ProjectName)
		if err != nil {
			r.SourceError = err.Error()
		} else {
			r.Downloads = a
		}
		return r, nil
	}
	m, err := source.Discover(ctx, source.Options{ProjectName: c.ProjectName, Mode: c.Mode, Source: c.Source, RepositoryCacheDir: c.RepositoryCacheDir, AllowHTTP: c.Security.AllowHTTP, MaxArchiveBytes: c.Security.MaxArchiveBytes})
	if err != nil {
		r.SourceError = err.Error()
		return r, nil
	}
	r.Remote = &m
	return r, nil
}
func listReleases(c config.Config, installed versionutil.Version, found bool) ([]Release, error) {
	entries, err := os.ReadDir(c.ReleaseRoot)
	if errors.Is(err, os.ErrNotExist) {
		return []Release{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Release{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		v, err := versionutil.Parse(e.Name())
		if err != nil {
			continue
		}
		info, _ := e.Info()
		p := filepath.Join(c.ReleaseRoot, e.Name())
		valid := marker(filepath.Join(p, ".release-version"), v.String()) && marker(filepath.Join(p, ".release-project"), c.ProjectName)
		out = append(out, Release{Path: p, Version: v.String(), Active: found && versionutil.CompareForProject(c.ProjectName, v, installed) == 0, Modified: info.ModTime(), Validated: valid})
	}
	sort.Slice(out, func(i, j int) bool {
		a, _ := versionutil.Parse(out[i].Version)
		b, _ := versionutil.Parse(out[j].Version)
		return versionutil.CompareForProject(c.ProjectName, a, b) > 0
	})
	return out, nil
}
func marker(p, want string) bool {
	b, err := os.ReadFile(p)
	return err == nil && strings.TrimSpace(string(b)) == want
}
func FindRelease(c config.Config, requested string) (Release, error) {
	local, err := ListLocal(c)
	if err != nil {
		return Release{}, err
	}
	if requested != "" {
		v, err := versionutil.Parse(strings.TrimPrefix(requested, "v"))
		if err != nil {
			return Release{}, err
		}
		for _, r := range local.Releases {
			if r.Version == v.String() {
				if !r.Validated {
					return Release{}, fmt.Errorf("Release %s ist nicht validiert", r.Version)
				}
				return r, nil
			}
		}
		return Release{}, fmt.Errorf("Release %s wurde nicht gefunden", v.String())
	}
	installed, _, found, err := updatecheck.DetectInstalled(c.CurrentDir)
	if err != nil {
		return Release{}, err
	}
	if !found {
		return Release{}, errors.New("keine installierte Version erkannt")
	}
	for _, r := range local.Releases {
		v, _ := versionutil.Parse(r.Version)
		if r.Validated && versionutil.CompareForProject(c.ProjectName, v, installed) < 0 {
			return r, nil
		}
	}
	return Release{}, errors.New("kein vorheriges validiertes Release vorhanden")
}
