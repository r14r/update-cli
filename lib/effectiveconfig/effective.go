package effectiveconfig

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/projectsetup"
)

type Metadata struct {
	ManifestPath string   `json:"manifestPath,omitempty"`
	Applied      []string `json:"applied,omitempty"`
}

// Load merges global config.json, local config.json and project-versioned
// update-cli.yaml settings. Priority is global config < local config <
// update-cli.yaml. Command-line overrides are intentionally applied by callers
// after this function.
func Load(root string) (config.Config, Metadata, error) {
	cfg, _, err := config.LoadResilient(root, "")
	if err != nil {
		return config.Config{}, Metadata{}, err
	}
	manifest, found, err := preferredManifest(cfg)
	if err != nil {
		return config.Config{}, Metadata{}, err
	}
	if !found {
		return cfg, Metadata{}, nil
	}
	parsed, err := projectsetup.ParseManifestForUse(manifest)
	if err != nil {
		return config.Config{}, Metadata{ManifestPath: manifest}, err
	}
	overrides := projectsetup.ConfigOverrides(parsed)
	cfg, err = config.ApplyProjectOverrides(cfg, overrides)
	if err != nil {
		return config.Config{}, Metadata{ManifestPath: manifest}, err
	}
	applied := describeOverrides(parsed)
	cfg.ProjectManifestFile = manifest
	cfg.ManifestOverrides = append([]string(nil), applied...)
	return cfg, Metadata{ManifestPath: manifest, Applied: applied}, nil
}

func preferredManifest(cfg config.Config) (string, bool, error) {
	rootManifest := filepath.Join(cfg.RootDir, config.ProjectFileName)
	if info, err := os.Stat(rootManifest); err == nil && !info.IsDir() {
		return rootManifest, true, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", false, err
	}
	currentManifest := filepath.Join(cfg.CurrentDir, config.ProjectFileName)
	if info, err := os.Stat(currentManifest); err == nil && !info.IsDir() {
		return currentManifest, true, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", false, err
	}
	return "", false, nil
}

func describeOverrides(m projectsetup.Manifest) []string {
	var out []string
	if m.ProjectSlug != "" {
		out = append(out, "project.slug")
	}
	if m.Update.Mode != "" {
		out = append(out, "update.mode")
	}
	if m.Update.SourceConfigured {
		out = append(out, "update.source")
	}
	if m.Update.ReleaseDir != "" {
		out = append(out, "update.releaseDir")
	}
	if m.Update.CurrentDir != "" {
		out = append(out, "update.currentDir")
	}
	if m.Update.Backup.Configured {
		out = append(out, "update.backup")
	}
	if m.Update.Retention.Configured {
		out = append(out, "update.retention")
	}
	if m.Update.Sync.Configured {
		out = append(out, "update.sync")
	}
	if m.Update.Setup.Configured {
		out = append(out, "update.setup")
	}
	if m.Update.Docker.Configured {
		out = append(out, "update.docker")
	}
	if m.Update.Healthcheck.Configured {
		out = append(out, "update.healthcheck")
	}
	return out
}
