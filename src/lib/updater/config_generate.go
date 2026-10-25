package updater

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"
)

// runConfigGenerate prints a runtime configuration draft. It never writes files.
func runConfigGenerate(args []string) error {
	var repository string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--repo" || args[i] == "--repository": 
			if i+1 >= len(args) {
				return fmt.Errorf("%s benötigt eine Repository-URL", args[i])
			}
			i++
			repository = args[i]
		case strings.HasPrefix(args[i], "--repo="):
			repository = strings.TrimPrefix(args[i], "--repo=")
		case strings.HasPrefix(args[i], "--repository="):
			repository = strings.TrimPrefix(args[i], "--repository=")
		default:
			return fmt.Errorf("config --generate: unbekannter Parameter %q", args[i])
		}
	}
	if strings.TrimSpace(repository) == "" {
		return fmt.Errorf("config --generate benötigt --repo URL")
	}
	repository = strings.TrimSpace(repository)
	name := ""
	if strings.Contains(repository, ": //") {
		parsed, err := url.Parse(repository)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
			return fmt.Errorf("ungültige Repository-URL %q", repository)
		}
		name = path.Base(strings.TrimSuffix(parsed.Path, "/"))
	} else {
		name = path.Base(strings.TrimSuffix(repository, "/"))
	}
	name = strings.TrimSuffix(name, ".git")
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("Projektname kann aus %q nicht abgeleitet werden", repository)
	}
	draft := map[string]any{
		"schemaVersion":	10,
		"projectName":	name,
		"mode":	"pull",
		"source":	map[string]any{"type": "repository", "repository": repository},
		"releaseDir":	"release",
		"currentDir":	"current",
		"no parameter":	[]string{"update"},
		"backup":	map[string]any{"directory": "backup","keep": 3},
		"retention":	map[string]any{"releases": 5},
		"sync":	map[string]any{"preserve": []string{".git/", ".gitignore", ".venv/", ".env", ".env.*", "data/", "storage/", "uploads/", "media/", "logs/", "var/"}},
		"security":	map[string]any{"allowHttp": false,"maxArchiveBytes": 2147483648,"maxUncompressedBytes": 8589934592,"maxFileBytes": 2147483648,"maxEntries": 100000,"maxCompressionRatio": 200},
		"docker":	map[string]any{"lifecycle": "auto"},
		"healthcheck":	map[string]any{"type": "none"},
	}
	data, err := json.MarshalIndent(draft, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
