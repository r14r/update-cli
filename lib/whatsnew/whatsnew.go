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

var (
	versionPattern = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+$`)
	historyHeading = regexp.MustCompile(`(?m)^# ([0-9]+\.[0-9]+\.[0-9]+)$`)
)

// ForVersion returns the embedded Markdown describing the user-visible changes
// for one Update CLI release.
func ForVersion(version string) (string, error) {
	version = strings.TrimSpace(version)
	if !versionPattern.MatchString(version) {
		return "", fmt.Errorf("ungültige Version %q; erwartet X.Y.Z", version)
	}
	version = strings.TrimPrefix(version, "v")

	name := path.Join("releases", version+".md")
	if data, err := releaseFiles.ReadFile(name); err == nil {
		return strings.TrimSpace(string(data)) + "\n", nil
	}

	data, err := releaseFiles.ReadFile("releases/history.md")
	if err == nil {
		if section, ok := historySection(string(data), version); ok {
			return section, nil
		}
	}
	return "", fmt.Errorf("keine What's-New-Informationen für Version %s verfügbar", version)
}

// Versions returns all releases that have embedded What's-New documentation.
func Versions() ([]string, error) {
	entries, err := releaseFiles.ReadDir("releases")
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || entry.Name() == "history.md" {
			continue
		}
		seen[strings.TrimSuffix(entry.Name(), ".md")] = true
	}

	if data, err := releaseFiles.ReadFile("releases/history.md"); err == nil {
		for _, match := range historyHeading.FindAllStringSubmatch(string(data), -1) {
			seen[match[1]] = true
		}
	}

	versions := make([]string, 0, len(seen))
	for version := range seen {
		versions = append(versions, version)
	}
	sort.Strings(versions)
	return versions, nil
}

func historySection(markdown, version string) (string, bool) {
	header := "# " + version
	start := strings.Index(markdown, header+"\n")
	if start < 0 {
		if strings.TrimSpace(markdown) == header {
			return "# What's new in Update CLI " + version + "\n", true
		}
		return "", false
	}

	bodyStart := start + len(header) + 1
	rest := markdown[bodyStart:]
	end := len(rest)
	if next := historyHeading.FindStringIndex(rest); next != nil {
		end = next[0]
	}
	body := strings.TrimSpace(rest[:end])
	if body == "" {
		return "# What's new in Update CLI " + version + "\n", true
	}
	return "# What's new in Update CLI " + version + "\n\n" + body + "\n", true
}
