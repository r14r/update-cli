package templates

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	FileName      = "templates.json"
	SchemaVersion = 1

	DockerDownCommand = "if command -v docker >/dev/null 2>&1 && { [ -f compose.yml ] || [ -f compose.yaml ] || [ -f docker-compose.yml ] || [ -f docker-compose.yaml ]; }; then docker compose down --remove-orphans; fi"
)

type SetupConfig struct {
	Commands []string `json:"commands"`
}

// Template describes a partial updater configuration. Only the populated
// sections are applied to config.json.
type Template struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	NoParameter []string     `json:"no parameter,omitempty"`
	Setup       *SetupConfig `json:"setup,omitempty"`
}

type File struct {
	SchemaVersion int        `json:"schemaVersion"`
	Templates     []Template `json:"templates"`
}

var builtins = []Template{
	{
		Name:        "Laravel",
		Description: "Stoppt Docker Compose, installiert Composer- und Node-Abhängigkeiten, baut Assets und führt Migrationen aus.",
		Setup: &SetupConfig{Commands: []string{
			DockerDownCommand,
			"composer install --no-interaction --prefer-dist --optimize-autoloader",
			"if [ -f package-lock.json ]; then npm ci --no-audit --no-fund; elif [ -f package.json ]; then npm install --no-audit --no-fund; fi",
			"if [ -f package.json ]; then npm run build; fi",
			"php artisan migrate --force",
		}},
	},
	{
		Name:        "Django",
		Description: "Stoppt Docker Compose, erstellt eine virtuelle Umgebung, installiert Python-Abhängigkeiten und führt Migrationen aus.",
		Setup: &SetupConfig{Commands: []string{
			DockerDownCommand,
			"python3 -m venv .venv",
			".venv/bin/python -m pip install --upgrade pip",
			"if [ -f requirements.txt ]; then .venv/bin/python -m pip install -r requirements.txt; fi",
			".venv/bin/python manage.py migrate",
		}},
	},
	{
		Name:        "FastAPI",
		Description: "Stoppt Docker Compose, erstellt eine virtuelle Umgebung und installiert requirements.txt oder das lokale pyproject-Paket.",
		Setup: &SetupConfig{Commands: []string{
			DockerDownCommand,
			"python3 -m venv .venv",
			".venv/bin/python -m pip install --upgrade pip",
			"if [ -f requirements.txt ]; then .venv/bin/python -m pip install -r requirements.txt; elif [ -f pyproject.toml ]; then .venv/bin/python -m pip install -e .; fi",
		}},
	},
	{
		Name:        "Vue",
		Description: "Stoppt Docker Compose, installiert Node-Abhängigkeiten und erstellt den Produktions-Build.",
		Setup: &SetupConfig{Commands: []string{
			DockerDownCommand,
			"if [ -f package-lock.json ]; then npm ci --no-audit --no-fund; else npm install --no-audit --no-fund; fi",
			"npm run build",
		}},
	},
	{
		Name:        "Go",
		Description: "Stoppt Docker Compose, lädt Go-Module, führt Tests aus und prüft den Build.",
		Setup: &SetupConfig{Commands: []string{
			DockerDownCommand,
			"go mod download",
			"go test ./...",
			"go build ./...",
		}},
	},
	{
		Name:        "update-and-setup",
		Description: "Führt bei einem parameterlosen Aufruf zuerst update und anschließend setup aus.",
		NoParameter: []string{"update", "setup"},
	},
}

// BuiltinFile returns a defensive copy of all templates compiled into the binary.
func BuiltinFile() File {
	result := File{SchemaVersion: SchemaVersion, Templates: make([]Template, 0, len(builtins))}
	for _, template := range builtins {
		result.Templates = append(result.Templates, clone(template))
	}
	return result
}

// Catalog returns the templates compiled into the binary plus optional global
// templates. Global definitions with the same name replace the compiled
// definition, allowing distributors to customize their catalog.
func Catalog(globalPath string) (File, error) {
	result := BuiltinFile()
	globalPath = strings.TrimSpace(globalPath)
	if globalPath == "" {
		return result, nil
	}
	if _, err := os.Stat(globalPath); errors.Is(err, os.ErrNotExist) {
		return result, nil
	} else if err != nil {
		return File{}, fmt.Errorf("globale Template-Datei kann nicht geprüft werden: %w", err)
	}
	additional, err := Load(globalPath)
	if err != nil {
		return File{}, fmt.Errorf("globale Template-Datei ist ungültig: %w", err)
	}
	indexes := make(map[string]int, len(result.Templates))
	for index, template := range result.Templates {
		indexes[strings.ToLower(strings.TrimSpace(template.Name))] = index
	}
	for _, template := range additional.Templates {
		key := strings.ToLower(strings.TrimSpace(template.Name))
		if index, exists := indexes[key]; exists {
			result.Templates[index] = clone(template)
			continue
		}
		result.Templates = append(result.Templates, clone(template))
		indexes[key] = len(result.Templates) - 1
	}
	if err := Validate(result); err != nil {
		return File{}, err
	}
	return result, nil
}

// WriteDefaults creates templates.json from the templates compiled into the
// binary. Existing files are preserved unless force is true.
func WriteDefaults(path string, force bool) (bool, error) {
	return WriteConfiguredDefaults(path, "", force)
}

// WriteConfiguredDefaults creates templates.json from the compiled templates
// and an optional global catalog.
func WriteConfiguredDefaults(path, globalPath string, force bool) (bool, error) {
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return false, fmt.Errorf("Template-Datei ist ein Ordner: %s", path)
		}
		if !force {
			return false, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("Template-Datei kann nicht geprüft werden: %w", err)
	}
	catalog, err := Catalog(globalPath)
	if err != nil {
		return false, err
	}
	if err := Write(path, catalog); err != nil {
		return false, err
	}
	return true, nil
}

// Load reads and validates a templates.json file.
func Load(path string) (File, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return File{}, fmt.Errorf("Template-Datei fehlt: %s\nErstellen mit update-cli --upgrade oder update-cli --init PROJECTNAME", path)
	}
	if err != nil {
		return File{}, fmt.Errorf("Template-Datei kann nicht gelesen werden: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var result File
	if err := decoder.Decode(&result); err != nil {
		return File{}, fmt.Errorf("templates.json enthält ungültiges JSON: %w", err)
	}
	if err := ensureEOF(decoder); err != nil {
		return File{}, err
	}
	if err := Validate(result); err != nil {
		return File{}, fmt.Errorf("ungültige Template-Datei %s: %w", path, err)
	}
	return result, nil
}

// Write validates and atomically writes templates.json.
func Write(path string, value File) error {
	if err := Validate(value); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("Templates können nicht serialisiert werden: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("Template-Ordner kann nicht erstellt werden: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".templates-*.json")
	if err != nil {
		return fmt.Errorf("temporäre Template-Datei kann nicht erstellt werden: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return fmt.Errorf("Dateirechte der Template-Datei können nicht gesetzt werden: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("Template-Datei kann nicht geschrieben werden: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("Template-Datei kann nicht synchronisiert werden: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("Template-Datei kann nicht geschlossen werden: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("Template-Datei kann nicht aktiviert werden: %w", err)
	}
	return nil
}

func Validate(value File) error {
	if value.SchemaVersion != SchemaVersion {
		return fmt.Errorf("nicht unterstützte schemaVersion %d; erwartet wird %d", value.SchemaVersion, SchemaVersion)
	}
	if len(value.Templates) == 0 {
		return errors.New("templates muss mindestens ein Template enthalten")
	}
	seen := make(map[string]bool, len(value.Templates))
	for index, template := range value.Templates {
		name := strings.TrimSpace(template.Name)
		if name == "" {
			return fmt.Errorf("templates[%d].name fehlt oder ist leer", index)
		}
		key := strings.ToLower(name)
		if seen[key] {
			return fmt.Errorf("Template %q ist mehrfach definiert", name)
		}
		seen[key] = true
		hasAction := false
		if len(template.NoParameter) > 0 {
			hasAction = true
			if _, err := normalizeActions(template.NoParameter); err != nil {
				return fmt.Errorf("Template %q: %w", name, err)
			}
		}
		if template.Setup != nil {
			hasAction = true
			for commandIndex, command := range template.Setup.Commands {
				if strings.TrimSpace(command) == "" {
					return fmt.Errorf("Template %q: setup.commands[%d] ist leer", name, commandIndex)
				}
				if strings.ContainsRune(command, '\x00') {
					return fmt.Errorf("Template %q: setup.commands[%d] enthält ein Nullzeichen", name, commandIndex)
				}
			}
		}
		if !hasAction {
			return fmt.Errorf("Template %q enthält weder \"no parameter\" noch setup.commands", name)
		}
	}
	return nil
}

// Lookup resolves a built-in template case-insensitively. It is kept for API
// compatibility; project operations should use LookupFile.
func Lookup(name string) (Template, error) {
	return lookup(BuiltinFile(), name)
}

// LookupCatalog resolves a template from the compiled catalog plus optional
// global additions.
func LookupCatalog(globalPath, name string) (Template, error) {
	file, err := Catalog(globalPath)
	if err != nil {
		return Template{}, err
	}
	return lookup(file, name)
}

// LookupFile resolves a project template from templates.json.
func LookupFile(path, name string) (Template, error) {
	file, err := Load(path)
	if err != nil {
		return Template{}, err
	}
	return lookup(file, name)
}

func lookup(file File, name string) (Template, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	for _, template := range file.Templates {
		if strings.ToLower(strings.TrimSpace(template.Name)) == key {
			return clone(template), nil
		}
	}
	return Template{}, fmt.Errorf("unbekanntes Template %q; verfügbar: %s", name, strings.Join(NamesFromFile(file), ", "))
}

// Names returns the names of templates compiled into the binary.
func Names() []string { return NamesFromFile(BuiltinFile()) }

func NamesFromFile(file File) []string {
	result := make([]string, 0, len(file.Templates))
	for _, template := range file.Templates {
		result = append(result, template.Name)
	}
	return result
}

// Sorted returns a defensive copy ordered case-insensitively by name.
func Sorted(file File) []Template {
	result := make([]Template, 0, len(file.Templates))
	for _, template := range file.Templates {
		result = append(result, clone(template))
	}
	sort.SliceStable(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result
}

func clone(value Template) Template {
	result := value
	result.NoParameter = append([]string(nil), value.NoParameter...)
	if value.Setup != nil {
		result.Setup = &SetupConfig{Commands: append([]string(nil), value.Setup.Commands...)}
	}
	return result
}

func normalizeActions(actions []string) ([]string, error) {
	seen := map[string]bool{}
	hasHelp, hasUpdate, hasSetup := false, false, false
	for index, raw := range actions {
		action := strings.ToLower(strings.TrimSpace(raw))
		if action == "" {
			return nil, fmt.Errorf(`"no parameter"[%d] ist leer`, index)
		}
		if seen[action] {
			return nil, fmt.Errorf(`"no parameter" enthält %q mehrfach`, action)
		}
		seen[action] = true
		switch action {
		case "help":
			hasHelp = true
		case "update":
			hasUpdate = true
		case "setup":
			hasSetup = true
		default:
			return nil, fmt.Errorf(`"no parameter" unterstützt nur "help", "update" und "setup"; erhalten: %q`, action)
		}
	}
	if hasHelp && len(seen) > 1 {
		return nil, errors.New(`"help" kann nicht mit weiteren Aktionen kombiniert werden`)
	}
	if hasHelp {
		return []string{"help"}, nil
	}
	result := make([]string, 0, 2)
	if hasUpdate {
		result = append(result, "update")
	}
	if hasSetup {
		result = append(result, "setup")
	}
	return result, nil
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("templates.json enthält ungültige Zusatzdaten: %w", err)
	}
	return errors.New("templates.json enthält mehrere JSON-Werte")
}

// migrateBuiltinDockerDown upgrades an unchanged legacy built-in template by
// prepending the Docker Compose shutdown command. Customized templates remain
// untouched.
func migrateBuiltinDockerDown(existing *Template, current Template) bool {
	if existing == nil || existing.Setup == nil || current.Setup == nil || len(current.Setup.Commands) == 0 {
		return false
	}
	if current.Setup.Commands[0] != DockerDownCommand || len(existing.Setup.Commands)+1 != len(current.Setup.Commands) {
		return false
	}
	if existing.Setup.Commands[0] == DockerDownCommand {
		return false
	}
	for index, command := range existing.Setup.Commands {
		if command != current.Setup.Commands[index+1] {
			return false
		}
	}
	existing.Setup.Commands = append([]string{DockerDownCommand}, existing.Setup.Commands...)
	if strings.TrimSpace(existing.Description) == "" {
		existing.Description = current.Description
	}
	return true
}

// EnsureBuiltins creates templates.json when missing and appends any built-in
// templates that are not yet present. Unchanged legacy built-ins are migrated
// safely; customized and custom definitions are preserved.
func EnsureBuiltins(path string) (created bool, updated bool, err error) {
	return EnsureCatalog(path, "")
}

// EnsureCatalog creates the local catalog when missing and appends templates
// from the compiled/global catalog that are not already defined locally.
// Existing local templates are always preserved.
func EnsureCatalog(path, globalPath string) (created bool, updated bool, err error) {
	if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
		created, err = WriteConfiguredDefaults(path, globalPath, false)
		return created, false, err
	} else if statErr != nil {
		return false, false, fmt.Errorf("Template-Datei kann nicht geprüft werden: %w", statErr)
	}

	file, err := Load(path)
	if err != nil {
		return false, false, err
	}
	catalog, err := Catalog(globalPath)
	if err != nil {
		return false, false, err
	}
	seen := make(map[string]int, len(file.Templates))
	for index, template := range file.Templates {
		seen[strings.ToLower(strings.TrimSpace(template.Name))] = index
	}
	builtinByName := make(map[string]Template, len(builtins))
	for _, template := range builtins {
		builtinByName[strings.ToLower(template.Name)] = template
	}
	for _, template := range catalog.Templates {
		key := strings.ToLower(strings.TrimSpace(template.Name))
		if index, exists := seen[key]; exists {
			if builtin, ok := builtinByName[key]; ok && migrateBuiltinDockerDown(&file.Templates[index], builtin) {
				updated = true
			}
			continue
		}
		file.Templates = append(file.Templates, clone(template))
		seen[key] = len(file.Templates) - 1
		updated = true
	}
	if !updated {
		return false, false, nil
	}
	if err := Write(path, file); err != nil {
		return false, false, err
	}
	return false, true, nil
}
