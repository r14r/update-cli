package backup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/r14r/update-cli/lib/config"
	rsyncutil "github.com/r14r/update-cli/lib/rsync"
	"github.com/r14r/update-cli/lib/updatecheck"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const MetadataFile = ".backup.json"

type Metadata struct {
	SchemaVersion int       `json:"schemaVersion"`
	ProjectName   string    `json:"projectName"`
	Version       string    `json:"version,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	SourceDir     string    `json:"sourceDir"`
}
type Item struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Version   string    `json:"version,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	Validated bool      `json:"validated"`
	Project   string    `json:"projectName,omitempty"`
}
type Result struct {
	Backup Item             `json:"backup"`
	Sync   rsyncutil.Result `json:"sync"`
}

func Create(ctx context.Context, c config.Config, dry bool) (Result, error) {
	i, err := os.Stat(c.CurrentDir)
	if errors.Is(err, os.ErrNotExist) {
		return Result{}, errors.New("current-Verzeichnis fehlt; es gibt nichts zu sichern")
	}
	if err != nil || !i.IsDir() {
		return Result{}, fmt.Errorf("current-Verzeichnis ist nicht verfügbar")
	}
	ver := "unknown"
	if v, _, found, e := updatecheck.DetectInstalled(c.CurrentDir); e != nil {
		return Result{}, e
	} else if found {
		ver = v.String()
	}
	created := time.Now()
	base := created.Format("20060102-150405") + "-v" + sanitize(ver)
	name, err := availableName(c.BackupRoot, base)
	if err != nil {
		return Result{}, err
	}
	dest := filepath.Join(c.BackupRoot, name)
	log := filepath.Join(c.ConfigDir, "logs", "backup-"+created.Format("20060102-150405")+".log")
	r, err := rsyncutil.Snapshot(ctx, c.CurrentDir, dest, log, dry)
	if err != nil {
		return Result{}, err
	}
	item := Item{Name: name, Path: dest, Version: ver, CreatedAt: created, Validated: !dry, Project: c.ProjectName}
	if dry {
		return Result{Backup: item, Sync: r}, nil
	}
	m := Metadata{SchemaVersion: 1, ProjectName: c.ProjectName, Version: ver, CreatedAt: created, SourceDir: c.CurrentDir}
	b, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(dest, MetadataFile), append(b, '\n'), 0o600); err != nil {
		return Result{}, err
	}
	return Result{Backup: item, Sync: r}, nil
}
func Resolve(c config.Config, id string) (Item, error) {
	id = strings.TrimSpace(id)
	if id == "" || strings.EqualFold(id, "latest") {
		items, err := List(c)
		if err != nil {
			return Item{}, err
		}
		if len(items) == 0 {
			return Item{}, errors.New("keine Backups vorhanden")
		}
		return items[0], nil
	}
	p := id
	if !filepath.IsAbs(p) {
		p = filepath.Join(c.BackupRoot, p)
	}
	a, err := filepath.Abs(p)
	if err != nil {
		return Item{}, err
	}
	if !within(c.BackupRoot, a) {
		return Item{}, fmt.Errorf("Backup liegt außerhalb von backup.directory: %s", a)
	}
	return inspect(a, c.ProjectName)
}
func List(c config.Config) ([]Item, error) {
	entries, err := os.ReadDir(c.BackupRoot)
	if errors.Is(err, os.ErrNotExist) {
		return []Item{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Item{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p := filepath.Join(c.BackupRoot, e.Name())
		it, err := inspect(p, c.ProjectName)
		if err != nil {
			info, _ := e.Info()
			out = append(out, Item{Name: e.Name(), Path: p, CreatedAt: info.ModTime(), Validated: false})
			continue
		}
		out = append(out, it)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func Restore(ctx context.Context, c config.Config, item Item, dry bool) (rsyncutil.Result, error) {
	log := filepath.Join(c.ConfigDir, "logs", "restore-"+time.Now().Format("20060102-150405")+".log")
	return rsyncutil.Restore(ctx, item.Path, c.CurrentDir, log, dry, c.Preserve)
}
func inspect(p, project string) (Item, error) {
	i, err := os.Stat(p)
	if err != nil || !i.IsDir() {
		return Item{}, fmt.Errorf("Backup wurde nicht gefunden: %s", p)
	}
	b, err := os.ReadFile(filepath.Join(p, MetadataFile))
	if err != nil {
		return Item{}, err
	}
	var m Metadata
	if err := json.Unmarshal(b, &m); err != nil {
		return Item{}, err
	}
	if m.SchemaVersion != 1 || m.ProjectName != project {
		return Item{}, errors.New("Backup-Metadaten passen nicht zum Projekt")
	}
	return Item{Name: filepath.Base(p), Path: p, Version: m.Version, CreatedAt: m.CreatedAt, Validated: true, Project: m.ProjectName}, nil
}
func availableName(root, base string) (string, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	for i := 0; i < 1000; i++ {
		n := base
		if i > 0 {
			n = fmt.Sprintf("%s-%02d", base, i)
		}
		if _, err := os.Stat(filepath.Join(root, n)); errors.Is(err, os.ErrNotExist) {
			return n, nil
		}
	}
	return "", errors.New("kein eindeutiger Backup-Name verfügbar")
}
func within(root, p string) bool {
	ra, _ := filepath.Abs(root)
	pa, _ := filepath.Abs(p)
	r, e := filepath.Rel(ra, pa)
	return e == nil && r != ".." && !strings.HasPrefix(r, ".."+string(os.PathSeparator))
}
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune(".-_", r) {
			return r
		}
		return '_'
	}, s)
}
