package whatsnew

import (
	"embed"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

//go:embed releases/*.md
var releaseFiles embed.FS

var versionPattern = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+$`)

// ForVersion returns the embedded Markdown describing the user-visible changes
// for one Update CLI release.
func ForVersion(version string) (string, error) {
	version = strings.TrimSpace(version)
	if !versionPattern.MatchString(version) {
		return "", fmt.Errorf("ungültige Version %q; erwartet X.Y.Z", version)
	}
	version = strings.TrimPrefix(version, "v")
	name := path.Join("releases", version+".md")
	data, err := releaseFiles.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("keine What's-New-Informationen für Version %s verfügbar", version)
	}
	return strings.TrimSpace(string(data)) + "\n", nil
}

// Versions returns all releases that have embedded What's-New documentation.
func Versions() ([]string, error) {
	entries, err := releaseFiles.ReadDir("releases")
	if err != nil {
		return nil, err
	}
	versions := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		versions = append(versions, strings.TrimSuffix(entry.Name(), ".md"))
	}
	sort.Strings(versions)
	return versions, nil
}
