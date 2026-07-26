package backup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"release-updater/lib/config"
	rsyncutil "release-updater/lib/rsync"
	"release-updater/lib/tools"
	"release-updater/lib/updatecheck"
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

func Create(ctx context.Context, cfg config.Config, dryRun bool) (Result, error) {
	info, err := os.Stat(cfg.CurrentDir)
	if errors.Is(err, os.ErrNotExist) {
		return Result{}, errors.New("current-Verzeichnis fehlt; es gibt nichts zu sichern")
	}
	if err != nil {
		return Result{}, fmt.Errorf("current-Verzeichnis kann nicht geprüft werden: %w", err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("currentDir ist kein Ordner: %s", cfg.CurrentDir)
	}
	version := "unknown"
	if installed, _, found, detectErr := updatecheck.DetectInstalled(cfg.CurrentDir); detectErr != nil {
		return Result{}, detectErr
	} else if found {
		version = installed.String()
	}
	created := time.Now()
	baseName := created.Format("20060102-150405") + "-v" + sanitize(version)
	name, err := availableName(cfg.BackupRoot, baseName)
	if err != nil {
		return Result{}, err
	}
	destination := filepath.Join(cfg.BackupRoot, name)
	if dryRun {
		destination = filepath.Join(os.TempDir(), ".update-cli-backup-plan", name)
		_ = tools.RemoveTree(destination)
	}
	logFile := filepath.Join(cfg.ConfigDir, "logs", "backup-"+created.Format("20060102-150405")+".log")
	result, err := rsyncutil.Snapshot(ctx, cfg.CurrentDir, destination, logFile, dryRun)
	if err != nil {
		return Result{}, err
	}
	item := Item{Name: name, Path: filepath.Join(cfg.BackupRoot, name), Version: version, CreatedAt: created, Validated: !dryRun, Project: cfg.ProjectName}
	if dryRun {
		return Result{Backup: item, Sync: result}, nil
	}
	metadata := Metadata{SchemaVersion: 1, ProjectName: cfg.ProjectName, Version: version, CreatedAt: created, SourceDir: cfg.CurrentDir}
	if err := writeMetadata(destination, metadata); err != nil {
		return Result{}, err
	}
	return Result{Backup: item, Sync: result}, nil
}

func Resolve(cfg config.Config, identifier string) (Item, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || strings.EqualFold(identifier, "latest") {
		items, err := List(cfg)
		if err != nil {
			return Item{}, err
		}
		if len(items) == 0 {
			return Item{}, errors.New("keine Backups vorhanden")
		}
		return items[0], nil
	}
	candidate := identifier
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(cfg.BackupRoot, candidate)
	}
	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return Item{}, err
	}
	if !within(cfg.BackupRoot, absolute) {
		return Item{}, fmt.Errorf("Backup liegt außerhalb von backup.directory: %s", absolute)
	}
	item, err := inspect(absolute, cfg.ProjectName)
	if err != nil {
		return Item{}, err
	}
	return item, nil
}

func List(cfg config.Config) ([]Item, error) {
	entries, err := os.ReadDir(cfg.BackupRoot)
	if errors.Is(err, os.ErrNotExist) {
		return []Item{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Backup-Ordner kann nicht gelesen werden: %w", err)
	}
	items := make([]Item, 0)
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(cfg.BackupRoot, entry.Name())
		item, inspectErr := inspect(path, cfg.ProjectName)
		if inspectErr != nil {
			info, infoErr := entry.Info()
			if infoErr != nil {
				return nil, infoErr
			}
			items = append(items, Item{Name: entry.Name(), Path: path, CreatedAt: info.ModTime(), Validated: false})
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items, nil
}

func Restore(ctx context.Context, cfg config.Config, item Item, dryRun bool) (rsyncutil.Result, error) {
	logFile := filepath.Join(cfg.ConfigDir, "logs", "restore-"+time.Now().Format("20060102-150405")+".log")
	return rsyncutil.Restore(ctx, item.Path, cfg.CurrentDir, logFile, dryRun)
}

func inspect(path, projectName string) (Item, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Item{}, fmt.Errorf("Backup wurde nicht gefunden: %w", err)
	}
	if !info.IsDir() {
		return Item{}, fmt.Errorf("Backup ist kein Ordner: %s", path)
	}
	data, err := os.ReadFile(filepath.Join(path, MetadataFile))
	if err != nil {
		return Item{}, fmt.Errorf("Backup-Metadaten fehlen in %s: %w", path, err)
	}
	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return Item{}, fmt.Errorf("Backup-Metadaten sind ungültig: %w", err)
	}
	if metadata.SchemaVersion != 1 {
		return Item{}, fmt.Errorf("nicht unterstützte Backup-Schemaversion: %d", metadata.SchemaVersion)
	}
	if metadata.ProjectName != projectName {
		return Item{}, fmt.Errorf("Backup gehört zu Projekt %q, erwartet wird %q", metadata.ProjectName, projectName)
	}
	return Item{Name: filepath.Base(path), Path: path, Version: metadata.Version, CreatedAt: metadata.CreatedAt, Validated: true, Project: metadata.ProjectName}, nil
}

func availableName(root, base string) (string, error) {
	for index := 0; index < 1000; index++ {
		name := base
		if index > 0 {
			name = fmt.Sprintf("%s-%02d", base, index)
		}
		_, err := os.Stat(filepath.Join(root, name))
		if errors.Is(err, os.ErrNotExist) {
			return name, nil
		}
		if err != nil {
			return "", fmt.Errorf("Backup-Ziel kann nicht geprüft werden: %w", err)
		}
	}
	return "", errors.New("kein eindeutiger Backup-Name verfügbar")
}

func writeMetadata(path string, metadata Metadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(path, MetadataFile), data, 0o644); err != nil {
		return fmt.Errorf("Backup-Metadaten können nicht geschrieben werden: %w", err)
	}
	return nil
}

func within(root, path string) bool {
	rootAbs, _ := filepath.Abs(root)
	pathAbs, _ := filepath.Abs(path)
	relative, err := filepath.Rel(rootAbs, pathAbs)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func sanitize(value string) string {
	value = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, value)
	if value == "" {
		return "unknown"
	}
	return value
}
