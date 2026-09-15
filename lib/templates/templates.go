package templates

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const SchemaVersion = 1

type Template struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	NoParameter []string `json:"noParameter,omitempty"`
	Preserve    []string `json:"preserve,omitempty"`
}
type File struct {
	SchemaVersion int        `json:"schemaVersion"`
	Templates     []Template `json:"templates"`
}

func Defaults() File {
	return File{SchemaVersion: 1, Templates: []Template{{Name: "Go", Description: "Go project with setup prompt", NoParameter: []string{"check"}}, {Name: "FastAPI", Description: "Python/FastAPI project", NoParameter: []string{"check"}, Preserve: []string{".git/", ".gitignore", ".venv/", ".env", ".env.*", "data/", "storage/", "uploads/", "media/", "logs/", "var/"}}, {Name: "Laravel", Description: "Laravel project", NoParameter: []string{"check"}, Preserve: []string{".git/", ".gitignore", ".env", ".env.*", "storage/", "uploads/", "media/", "logs/"}}, {Name: "Docker", Description: "Docker Compose project", NoParameter: []string{"check"}, Preserve: []string{".git/", ".gitignore", ".env", ".env.*", "data/", "storage/", "uploads/", "media/", "logs/", "var/"}}}}
}
func Ensure(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return Save(path, Defaults())
}
func Load(path string) (File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var f File
	if err := dec.Decode(&f); err != nil {
		return f, err
	}
	if f.SchemaVersion != 1 {
		return f, fmt.Errorf("templates schemaVersion muss 1 sein")
	}
	seen := map[string]bool{}
	for _, t := range f.Templates {
		if strings.TrimSpace(t.Name) == "" {
			return f, errors.New("Template-Name fehlt")
		}
		if seen[strings.ToLower(t.Name)] {
			return f, fmt.Errorf("Template %q mehrfach", t.Name)
		}
		seen[strings.ToLower(t.Name)] = true
	}
	return f, nil
}
func Save(path string, f File) error {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
func Lookup(path, name string) (Template, error) {
	f, err := Load(path)
	if err != nil {
		return Template{}, err
	}
	for _, t := range f.Templates {
		if strings.EqualFold(t.Name, name) {
			return t, nil
		}
	}
	return Template{}, fmt.Errorf("Template %q nicht gefunden", name)
}
func Sorted(f File) []Template {
	out := append([]Template(nil), f.Templates...)
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}
func Apply(configPath, templatesPath, name string) error {
	t, err := Lookup(templatesPath, name)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	if len(t.NoParameter) > 0 {
		m["no parameter"] = t.NoParameter
	}
	if len(t.Preserve) > 0 {
		m["sync"] = map[string]any{"preserve": t.Preserve}
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, append(out, '\n'), 0o644)
}

// LoadMerged loads installation-wide templates first and overlays project-local
// templates by name. If the global file does not exist, the compiled-in
// defaults are used. A missing local file is treated as an empty override.
func LoadMerged(globalPath, localPath string) (File, error) {
	base := Defaults()
	if strings.TrimSpace(globalPath) != "" {
		if _, err := os.Stat(globalPath); err == nil {
			loaded, err := Load(globalPath)
			if err != nil {
				return File{}, fmt.Errorf("globale templates.json %s: %w", globalPath, err)
			}
			base = loaded
		} else if !errors.Is(err, os.ErrNotExist) {
			return File{}, err
		}
	}
	if strings.TrimSpace(localPath) == "" {
		return base, nil
	}
	if _, err := os.Stat(localPath); errors.Is(err, os.ErrNotExist) {
		return base, nil
	} else if err != nil {
		return File{}, err
	}
	local, err := Load(localPath)
	if err != nil {
		return File{}, fmt.Errorf("lokale templates.json %s: %w", localPath, err)
	}
	return Merge(base, local), nil
}

// Merge overlays templates from local on global by case-insensitive name.
// Project-local definitions replace complete global template definitions with
// the same name; new local templates are appended.
func Merge(global, local File) File {
	out := File{SchemaVersion: SchemaVersion, Templates: append([]Template(nil), global.Templates...)}
	index := make(map[string]int, len(out.Templates))
	for i, template := range out.Templates {
		index[strings.ToLower(strings.TrimSpace(template.Name))] = i
	}
	for _, template := range local.Templates {
		key := strings.ToLower(strings.TrimSpace(template.Name))
		if i, ok := index[key]; ok {
			out.Templates[i] = template
			continue
		}
		index[key] = len(out.Templates)
		out.Templates = append(out.Templates, template)
	}
	return out
}

func LookupMerged(globalPath, localPath, name string) (Template, error) {
	f, err := LoadMerged(globalPath, localPath)
	if err != nil {
		return Template{}, err
	}
	for _, template := range f.Templates {
		if strings.EqualFold(template.Name, name) {
			return template, nil
		}
	}
	return Template{}, fmt.Errorf("Template %q nicht gefunden", name)
}

// EnsureLocal creates an empty project-local override file. Global templates
// remain the baseline and are therefore not copied into every project.
func EnsureLocal(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return Save(path, File{SchemaVersion: SchemaVersion, Templates: []Template{}})
}
