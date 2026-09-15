package projectsetup

import "github.com/r14r/update-cli/lib/config"

// ConfigOverrides converts project-versioned update-cli.yaml settings into the
// runtime override representation consumed by lib/config. Host security and
// user-level defaults are intentionally excluded.
func ConfigOverrides(m Manifest) config.ProjectOverrides {
	var out config.ProjectOverrides
	if m.ProjectSlug != "" {
		out.ProjectName = m.ProjectSlug
	}
	if m.Update.Mode != "" {
		v := m.Update.Mode
		out.Mode = &v
	}
	if m.Update.SourceConfigured {
		out.Source = &config.SourceConfig{
			Type:       m.Update.Source.Type,
			Folder:     m.Update.Source.Folder,
			URL:        m.Update.Source.URL,
			Repository: m.Update.Source.Repository,
			Ref:        m.Update.Source.Ref,
			Commit:     m.Update.Source.Commit,
			Version:    m.Update.Source.Version,
			SHA256:     m.Update.Source.SHA256,
		}
	}
	if m.Update.ReleaseDir != "" {
		v := m.Update.ReleaseDir
		out.ReleaseDir = &v
	}
	if m.Update.CurrentDir != "" {
		v := m.Update.CurrentDir
		out.CurrentDir = &v
	}
	if m.Update.Backup.Configured {
		if m.Update.Backup.Directory != "" {
			v := m.Update.Backup.Directory
			out.BackupDirectory = &v
		}
		out.BackupKeep = m.Update.Backup.Keep
	}
	if m.Update.Retention.Configured {
		out.RetentionReleases = m.Update.Retention.Releases
	}
	if m.Update.Sync.Configured {
		v := append([]string(nil), m.Update.Sync.Preserve...)
		out.Preserve = &v
		if m.Update.Sync.KeepOnSetupError != nil {
			out.KeepRsyncOnError = m.Update.Sync.KeepOnSetupError
		}
	}
	// Transitional compatibility: update.setup.keepRsyncOnError was emitted by
	// 2.11.x/2.12.0-2.12.1. Canonical update.sync.keepOnSetupError wins.
	if out.KeepRsyncOnError == nil && m.Update.Setup.Configured {
		out.KeepRsyncOnError = m.Update.Setup.KeepRsyncOnError
	}
	if m.Update.Docker.Configured && m.Update.Docker.Lifecycle != "" {
		v := m.Update.Docker.Lifecycle
		out.DockerLifecycle = &v
	}
	if m.Update.Healthcheck.Configured {
		out.HealthcheckType = m.Update.Healthcheck.Type
		out.HealthcheckURL = m.Update.Healthcheck.URL
		out.HealthcheckCommand = m.Update.Healthcheck.Command
		out.HealthcheckTimeout = m.Update.Healthcheck.TimeoutSeconds
	}
	return out
}
