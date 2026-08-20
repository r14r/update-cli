package updater

import (
	"fmt"
	"strings"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/projectsetup"
)

// withManifestSourceDefaults overlays the optional update source declared in
// current/update-cli.yaml onto the machine-local updater configuration.
// Explicit command-line source options are applied afterwards and therefore
// retain the highest precedence.
func withManifestSourceDefaults(c config.Config) (config.Config, error) {
	manifestPath := projectsetup.ManifestPath(c)
	if manifestPath == "" {
		return c, nil
	}
	manifest, err := projectsetup.ParseManifest(manifestPath)
	if err != nil {
		return c, err
	}
	if manifest.Version != 2 || !manifest.Update.Configured {
		return c, nil
	}

	u := manifest.Update
	c, err = config.WithSourceOverrides(c, u.Mode, u.Source.Type, u.Source.Folder, u.Source.URL, u.Source.Repository)
	if err != nil {
		return c, fmt.Errorf("update-cli.yaml update-Quelle ungültig: %w", err)
	}
	if v := strings.TrimSpace(u.Source.Ref); v != "" {
		c.Source.Ref = v
	}
	if v := strings.TrimSpace(u.Source.Commit); v != "" {
		c.Source.Commit = v
	}
	if v := strings.TrimSpace(u.Source.Version); v != "" {
		c.Source.Version = v
	}
	if v := strings.TrimSpace(u.Source.SHA256); v != "" {
		c.Source.SHA256 = v
	}
	return c, nil
}
