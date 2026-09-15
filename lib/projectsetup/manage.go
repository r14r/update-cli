package projectsetup

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ConvertResult struct {
	Path           string `json:"path"`
	BackupPath     string `json:"backupPath,omitempty"`
	PreviousSchema int    `json:"previousSchema"`
	CurrentSchema  int    `json:"currentSchema"`
	Changed        bool   `json:"changed"`
}

type ProjectMigrationResult struct {
	SourcePath     string `json:"sourcePath"`
	Path           string `json:"path"`
	BackupPath     string `json:"backupPath,omitempty"`
	PreviousSchema int    `json:"previousSchema"`
	CurrentSchema  int    `json:"currentSchema"`
	Changed        bool   `json:"changed"`
	Canonicalized  bool   `json:"canonicalized,omitempty"`
}

// MigrateProjectManifest upgrades the project manifest in root to the latest
// supported schema. update-cli.yaml is canonical; a legacy setup.yaml is kept
// untouched and migrated into a new update-cli.yaml.
func MigrateProjectManifest(root string) (ProjectMigrationResult, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ProjectMigrationResult{}, err
	}
	if info, err := os.Stat(absRoot); err != nil {
		return ProjectMigrationResult{}, err
	} else if !info.IsDir() {
		return ProjectMigrationResult{}, fmt.Errorf("Projektpfad ist kein Ordner: %s", absRoot)
	}

	canonical := filepath.Join(absRoot, "update-cli.yaml")
	if info, err := os.Stat(canonical); err == nil {
		if info.IsDir() {
			return ProjectMigrationResult{}, fmt.Errorf("update-cli.yaml ist ein Ordner: %s", canonical)
		}
		// Schema 2 had a short-lived 2.11.x/2.12.0-2.12.1 layout using
		// update.setup.keepRsyncOnError. Treat that as a structural migration
		// even though the numeric schemaVersion did not change.
		if manifest, parseErr := ParseManifest(canonical); parseErr == nil && manifest.Version == SchemaVersion && manifest.Update.Setup.Configured {
			repair, repairErr := RepairProjectManifest(absRoot)
			if repairErr != nil {
				return ProjectMigrationResult{}, repairErr
			}
			return ProjectMigrationResult{
				SourcePath: canonical, Path: repair.Path, BackupPath: repair.BackupPath,
				PreviousSchema: repair.PreviousSchema, CurrentSchema: repair.CurrentSchema, Changed: repair.Changed,
			}, nil
		}
		res, err := ConvertManifestToLatest(canonical, false)
		if err != nil {
			return ProjectMigrationResult{}, err
		}
		return ProjectMigrationResult{
			SourcePath: canonical, Path: res.Path, BackupPath: res.BackupPath,
			PreviousSchema: res.PreviousSchema, CurrentSchema: res.CurrentSchema, Changed: res.Changed,
		}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return ProjectMigrationResult{}, err
	}

	legacy := filepath.Join(absRoot, "setup.yaml")
	info, err := os.Stat(legacy)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ProjectMigrationResult{}, fmt.Errorf("kein update-cli.yaml oder setup.yaml in %s", absRoot)
		}
		return ProjectMigrationResult{}, err
	}
	if info.IsDir() {
		return ProjectMigrationResult{}, fmt.Errorf("setup.yaml ist ein Ordner: %s", legacy)
	}

	text, previous, err := PreviewConvertManifest(legacy)
	if err != nil {
		return ProjectMigrationResult{}, err
	}
	// Validate the generated canonical form before creating the new file.
	tmp, err := os.CreateTemp(absRoot, ".doctor-migrate-*.yaml")
	if err != nil {
		return ProjectMigrationResult{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(text); err != nil {
		_ = tmp.Close()
		return ProjectMigrationResult{}, err
	}
	if err := tmp.Close(); err != nil {
		return ProjectMigrationResult{}, err
	}
	if _, err := ParseManifest(tmpName); err != nil {
		return ProjectMigrationResult{}, fmt.Errorf("migriertes update-cli.yaml ist ungültig: %w", err)
	}
	if err := atomicWrite(canonical, []byte(text), 0o644); err != nil {
		return ProjectMigrationResult{}, err
	}
	return ProjectMigrationResult{
		SourcePath: legacy, Path: canonical, PreviousSchema: previous,
		CurrentSchema: SchemaVersion, Changed: true, Canonicalized: true,
	}, nil
}

type GenerateResult struct {
	Path         string   `json:"path"`
	Technologies []string `json:"technologies"`
	Overwritten  bool     `json:"overwritten"`
}

type ScriptResult struct {
	Path        string `json:"path"`
	Overwritten bool   `json:"overwritten"`
}

func ConvertManifestToLatest(path string, force bool) (ConvertResult, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ConvertResult{}, err
	}
	m, err := ParseManifest(abs)
	if err != nil {
		return ConvertResult{}, err
	}
	res := ConvertResult{Path: abs, PreviousSchema: m.Version, CurrentSchema: 2}
	if m.Version == 2 {
		return res, nil
	}
	if m.Version != 1 {
		return res, fmt.Errorf("update-cli.yaml Schema %d kann nicht automatisch konvertiert werden", m.Version)
	}
	out := renderConvertedV1(m)
	// Parse before replacing the original so conversion can never destroy a valid manifest.
	tmp, err := os.CreateTemp(filepath.Dir(abs), ".setup-convert-*.yaml")
	if err != nil {
		return res, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(out); err != nil {
		_ = tmp.Close()
		return res, err
	}
	if err := tmp.Close(); err != nil {
		return res, err
	}
	if _, err := ParseManifest(tmpName); err != nil {
		return res, fmt.Errorf("konvertiertes update-cli.yaml ist ungültig: %w", err)
	}

	backup := abs + ".schema1-" + time.Now().Format("20060102-150405") + ".bak"
	if !force {
		if _, err := os.Stat(backup); err == nil {
			return res, fmt.Errorf("Backup existiert bereits: %s; --force verwenden", backup)
		}
	}
	original, err := os.ReadFile(abs)
	if err != nil {
		return res, err
	}
	if err := os.WriteFile(backup, original, 0o600); err != nil {
		return res, err
	}
	if err := atomicWrite(abs, []byte(out), 0o644); err != nil {
		return res, err
	}
	res.BackupPath = backup
	res.Changed = true
	return res, nil
}

func GenerateManifest(root, path string, force bool) (GenerateResult, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return GenerateResult{}, err
	}
	if path == "" {
		path = filepath.Join(absRoot, "update-cli.yaml")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(absRoot, path)
	}
	existed := false
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return GenerateResult{}, fmt.Errorf("Ziel ist ein Ordner: %s", path)
		}
		existed = true
		if !force {
			return GenerateResult{}, fmt.Errorf("update-cli.yaml existiert bereits: %s; --force verwenden", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return GenerateResult{}, err
	}

	d, err := detectProject(absRoot)
	if err != nil {
		return GenerateResult{}, err
	}
	text := renderDetectedManifest(d)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return GenerateResult{}, err
	}
	if err := atomicWrite(path, []byte(text), 0o644); err != nil {
		return GenerateResult{}, err
	}
	if _, err := ParseManifest(path); err != nil {
		return GenerateResult{}, fmt.Errorf("erzeugtes update-cli.yaml ist ungültig: %w", err)
	}
	return GenerateResult{Path: path, Technologies: d.Technologies, Overwritten: existed}, nil
}

func GenerateSetupScript(root, path string, force bool) (ScriptResult, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ScriptResult{}, err
	}
	if path == "" {
		path = filepath.Join(absRoot, "setup.sh")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(absRoot, path)
	}
	existed := false
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return ScriptResult{}, fmt.Errorf("Ziel ist ein Ordner: %s", path)
		}
		existed = true
		if !force {
			return ScriptResult{}, fmt.Errorf("setup.sh existiert bereits: %s; --force verwenden", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return ScriptResult{}, err
	}
	if err := atomicWrite(path, []byte(generatedSetupScript), 0o755); err != nil {
		return ScriptResult{}, err
	}
	if err := os.Chmod(path, 0o755); err != nil {
		return ScriptResult{}, err
	}
	return ScriptResult{Path: path, Overwritten: existed}, nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".update-cli-write-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err := f.Chmod(mode); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func renderConvertedV1(m Manifest) string {
	var b strings.Builder
	b.WriteString("schemaVersion: 2\n\nproject:\n")
	if m.ProjectName != "" {
		fmt.Fprintf(&b, "  name: %s\n", yamlQuote(m.ProjectName))
	}
	if m.ProjectSlug != "" {
		fmt.Fprintf(&b, "  slug: %s\n", yamlQuote(m.ProjectSlug))
	}
	if m.ProjectType != "" {
		fmt.Fprintf(&b, "  type: %s\n", yamlQuote(m.ProjectType))
	}
	if m.ProjectDescription != "" {
		fmt.Fprintf(&b, "  description: %s\n", yamlQuote(m.ProjectDescription))
	}
	b.WriteString("\ndefaults:\n  failFast: true\n\nworkflows:\n  setup:\n    description: Aus schemaVersion 1 konvertierter Setup-Workflow\n    tasks:\n      - setup\n\ntasks:\n  setup:\n    description: Konvertierte Setup-Schritte\n    steps:\n")
	for i, s := range m.Steps {
		id := s.ID
		if id == "" {
			id = fmt.Sprintf("step-%02d", i+1)
		}
		name := s.Name
		if name == "" {
			name = id
		}
		fmt.Fprintf(&b, "      - id: %s\n        name: %s\n", yamlQuote(id), yamlQuote(name))
		if s.WorkingDirectory != "" && s.WorkingDirectory != "." {
			fmt.Fprintf(&b, "        cwd: %s\n", yamlQuote(s.WorkingDirectory))
		}
		if s.ContinueOnError {
			b.WriteString("        allowFailure: true\n")
		}
		renderV1Operation(&b, s, "        ")
		renderLegacyWhen(&b, s.When, "        ")
	}
	return b.String()
}

func renderV1Operation(b *strings.Builder, s Step, indent string) {
	switch s.Type {
	case "go":
		b.WriteString(indent + "go:\n")
		fmt.Fprintf(b, "%s  action: %s\n", indent, yamlQuote(s.Action))
		if s.Output != "" {
			fmt.Fprintf(b, "%s  output: %s\n", indent, yamlQuote(s.Output))
		}
		if len(s.Args) > 0 {
			fmt.Fprintf(b, "%s  args: %s\n", indent, yamlInlineList(s.Args))
		}
	case "python":
		switch s.Action {
		case "venv":
			b.WriteString(indent + "pythonVenv:\n" + indent + "  path: .venv\n")
		case "install":
			b.WriteString(indent + "pip:\n")
			req := s.Requirements
			if req == "" {
				req = "requirements.txt"
			}
			fmt.Fprintf(b, "%s  requirements: %s\n", indent, yamlQuote(req))
		default:
			args := append([]string{"-m", "pytest"}, s.Args...)
			b.WriteString(indent + "command:\n" + indent + "  exec: python3\n")
			fmt.Fprintf(b, "%s  args: %s\n", indent, yamlInlineList(args))
		}
	case "node":
		b.WriteString(indent + "npm:\n")
		fmt.Fprintf(b, "%s  action: %s\n", indent, yamlQuote(s.Action))
		if len(s.Args) > 0 {
			fmt.Fprintf(b, "%s  args: %s\n", indent, yamlInlineList(s.Args))
		}
	case "laravel":
		if s.Action == "install" {
			b.WriteString(indent + "composer:\n" + indent + "  action: install\n")
		} else {
			b.WriteString(indent + "artisan:\n")
			command := s.Action
			if command == "migrate" {
				command = "migrate"
			}
			fmt.Fprintf(b, "%s  command: %s\n", indent, yamlQuote(command))
		}
	case "docker", "docker-compose":
		b.WriteString(indent + "dockerCompose:\n")
		fmt.Fprintf(b, "%s  action: %s\n", indent, yamlQuote(s.Action))
		if s.Detach {
			b.WriteString(indent + "  detach: true\n")
		}
		if len(s.Args) > 0 {
			fmt.Fprintf(b, "%s  args: %s\n", indent, yamlInlineList(s.Args))
		}
	case "copy", "deploy":
		op := s.Type
		b.WriteString(indent + op + ":\n")
		fmt.Fprintf(b, "%s  source: %s\n%s  target: %s\n", indent, yamlQuote(s.Source), indent, yamlQuote(s.Destination))
		if s.Mode != "" {
			fmt.Fprintf(b, "%s  mode: %s\n", indent, yamlQuote(s.Mode))
		}
	default:
		script := s.Command
		if script == "" {
			script = "# TODO: migrate legacy step type=" + s.Type + " action=" + s.Action
		}
		b.WriteString(indent + "shell: |\n")
		writeBlock(b, script, indent+"  ")
	}
}

func renderLegacyWhen(b *strings.Builder, when, indent string) {
	when = strings.TrimSpace(strings.ReplaceAll(when, "\\:", ":"))
	if when == "" || when == "always" {
		return
	}
	parts := strings.SplitN(when, ":", 2)
	kind, value := parts[0], ""
	if len(parts) == 2 {
		value = parts[1]
	}
	key := map[string]string{"file": "fileExists", "not-file": "fileNotExists", "dir": "directoryExists", "command": "commandExists", "env": "envSet", "os": "os", "compose": "compose"}[kind]
	if key == "" {
		key = kind
	}
	b.WriteString(indent + "when:\n")
	if key == "compose" {
		b.WriteString(indent + "  compose: true\n")
	} else {
		fmt.Fprintf(b, "%s  %s: %s\n", indent, key, yamlQuote(value))
	}
}

type detectedProject struct {
	Name, Type                        string
	Technologies                      []string
	Go, Python, Node, Laravel, Docker bool
	RequirementsFile                  string
	NodeManager                       string
	NodeScripts                       map[string]bool
	HasEnvExample, HasEnv             bool
}

func detectProject(root string) (detectedProject, error) {
	d := detectedProject{Name: filepath.Base(root), NodeScripts: map[string]bool{}}
	exists := func(name string) bool {
		info, err := os.Stat(filepath.Join(root, name))
		return err == nil && !info.IsDir()
	}
	d.Go = exists("go.mod")
	d.Python = exists("pyproject.toml") || exists("requirements.txt") || exists("setup.py") || exists("Pipfile")
	d.RequirementsFile = "requirements.txt"
	if !exists(d.RequirementsFile) {
		d.RequirementsFile = ""
	}
	d.Node = exists("package.json")
	d.Laravel = exists("artisan") && exists("composer.json")
	if !d.Laravel && exists("composer.json") {
		if raw, err := os.ReadFile(filepath.Join(root, "composer.json")); err == nil && strings.Contains(string(raw), "laravel/framework") {
			d.Laravel = true
		}
	}
	d.Docker = exists("compose.yml") || exists("compose.yaml") || exists("docker-compose.yml") || exists("docker-compose.yaml")
	d.HasEnvExample, d.HasEnv = exists(".env.example"), exists(".env")
	if d.Node {
		d.NodeManager = "npm"
		if exists("pnpm-lock.yaml") {
			d.NodeManager = "pnpm"
		} else if exists("yarn.lock") {
			d.NodeManager = "yarn"
		}
		raw, err := os.ReadFile(filepath.Join(root, "package.json"))
		if err == nil {
			var pkg struct {
				Name    string            `json:"name"`
				Scripts map[string]string `json:"scripts"`
			}
			if json.Unmarshal(raw, &pkg) == nil {
				if strings.TrimSpace(pkg.Name) != "" {
					d.Name = pkg.Name
				}
				for k := range pkg.Scripts {
					d.NodeScripts[k] = true
				}
			}
		}
	}
	types := []string{}
	if d.Go {
		types = append(types, "go")
	}
	if d.Python {
		types = append(types, "python")
	}
	if d.Node {
		types = append(types, "node")
	}
	if d.Laravel {
		types = append(types, "laravel")
	}
	if d.Docker {
		types = append(types, "docker")
	}
	if len(types) == 0 {
		types = append(types, "generic")
	}
	d.Technologies = types
	d.Type = strings.Join(types, "+")
	return d, nil
}

func renderDetectedManifest(d detectedProject) string {
	var b strings.Builder
	b.WriteString("schemaVersion: 2\n\nproject:\n")
	fmt.Fprintf(&b, "  name: %s\n  type: %s\n  description: Automatisch erkannte Projekt-Automation; vor dem ersten produktiven Einsatz prüfen\n", yamlQuote(d.Name), yamlQuote(d.Type))
	b.WriteString("\ndefaults:\n  timeout: 15m\n  failFast: true\n\n")
	req := []string{}
	if d.Go {
		req = append(req, "go")
	}
	if d.Python {
		req = append(req, "python3")
	}
	if d.Node {
		req = append(req, d.NodeManager)
	}
	if d.Laravel {
		req = append(req, "php", "composer")
	}
	if d.Docker {
		req = append(req, "docker")
	}
	if len(req) > 0 {
		sort.Strings(req)
		req = uniqueStrings(req)
		b.WriteString("requirements:\n  commands:\n")
		for _, x := range req {
			fmt.Fprintf(&b, "    - %s\n", yamlQuote(x))
		}
		b.WriteString("\n")
	}
	tasks := []string{"prepare", "check", "test", "build", "verify"}
	if d.Docker {
		tasks = append(tasks, "containers")
	}
	b.WriteString("workflows:\n  setup:\n    description: Erkanntes Standard-Setup\n    tasks:\n")
	for _, t := range tasks {
		fmt.Fprintf(&b, "      - %s\n", t)
	}
	b.WriteString("  ci:\n    description: Prüfen, testen und bauen\n    tasks:\n      - prepare\n      - check\n      - test\n      - build\n      - verify\n\ntasks:\n")

	b.WriteString("  prepare:\n    description: Abhängigkeiten und Umgebung vorbereiten\n    steps:\n")
	count := 0
	if d.HasEnvExample {
		count++
		b.WriteString("      - id: env-file\n        name: .env aus Vorlage vorbereiten\n        copy:\n          source: .env.example\n          target: .env\n          overwrite: false\n        when:\n          fileNotExists: .env\n")
	}
	if d.Go {
		count++
		b.WriteString("      - id: go-mod-download\n        name: Go-Module laden\n        go:\n          action: mod-download\n        when:\n          fileExists: go.mod\n")
	}
	if d.Python {
		count++
		b.WriteString("      - id: python-venv\n        name: Python-Umgebung erstellen\n        pythonVenv:\n          path: .venv\n          python: python3\n        when:\n          not:\n            directoryExists: .venv\n")
		if d.RequirementsFile != "" {
			count++
			fmt.Fprintf(&b, "      - id: python-dependencies\n        name: Python-Abhängigkeiten installieren\n        pip:\n          python: .venv/bin/python\n          requirements: %s\n        when:\n          fileExists: %s\n", yamlQuote(d.RequirementsFile), yamlQuote(d.RequirementsFile))
		}
	}
	if d.Laravel {
		count++
		b.WriteString("      - id: composer-install\n        name: Composer-Abhängigkeiten installieren\n        composer:\n          action: install\n        when:\n          fileExists: composer.json\n")
	}
	if d.Node {
		count++
		fmt.Fprintf(&b, "      - id: node-install\n        name: Node-Abhängigkeiten installieren\n        %s:\n          action: install\n        when:\n          fileExists: package.json\n", d.NodeManager)
	}
	if count == 0 {
		b.WriteString("      - id: prepare\n        name: Vorbereitung\n        shell: |\n          printf 'Keine automatisch erkannten Vorbereitungsschritte.\\n'\n")
	}

	b.WriteString("\n  check:\n    description: Statische Prüfungen\n    steps:\n")
	count = 0
	if d.Go {
		count++
		b.WriteString("      - id: go-vet\n        name: Go-Quellcode prüfen\n        go:\n          action: vet\n")
	}
	if d.Python {
		count++
		b.WriteString("      - id: python-compile\n        name: Python-Quellcode kompilieren\n        command:\n          exec: .venv/bin/python\n          args: [-m, compileall, -q, .]\n        when:\n          directoryExists: .venv\n")
	}
	if d.Node && d.NodeScripts["lint"] {
		count++
		fmt.Fprintf(&b, "      - id: node-lint\n        name: Node-Lint ausführen\n        command:\n          exec: %s\n          args: [run, lint]\n", d.NodeManager)
	}
	if d.Laravel {
		count++
		b.WriteString("      - id: laravel-about\n        name: Laravel-Konfiguration prüfen\n        artisan:\n          command: about\n")
	}
	if count == 0 {
		b.WriteString("      - id: check\n        name: Projekt prüfen\n        shell: |\n          printf 'Keine automatisch erkannten statischen Prüfungen.\\n'\n")
	}

	b.WriteString("\n  test:\n    description: Tests ausführen\n    steps:\n")
	count = 0
	if d.Go {
		count++
		b.WriteString("      - id: go-test\n        name: Go-Tests ausführen\n        go:\n          action: test\n")
	}
	if d.Laravel {
		count++
		b.WriteString("      - id: laravel-test\n        name: Laravel-Tests ausführen\n        artisan:\n          command: test\n")
	}
	if d.Python {
		count++
		b.WriteString("      - id: python-test\n        name: Python-Tests ausführen\n        command:\n          exec: .venv/bin/python\n          args: [-m, pytest]\n        when:\n          commandExists: pytest\n        allowFailure: true\n")
	}
	if d.Node && d.NodeScripts["test"] {
		count++
		fmt.Fprintf(&b, "      - id: node-test\n        name: Node-Tests ausführen\n        %s:\n          action: test\n", d.NodeManager)
	}
	if count == 0 {
		b.WriteString("      - id: tests\n        name: Keine Tests erkannt\n        shell: |\n          printf 'Keine automatisch erkannten Tests.\\n'\n")
	}

	b.WriteString("\n  build:\n    description: Projekt bauen\n    steps:\n")
	count = 0
	if d.Go {
		count++
		b.WriteString("      - id: go-build\n        name: Go-Projekt bauen\n        go:\n          action: build\n          package: ./...\n")
	}
	if d.Node && d.NodeScripts["build"] {
		count++
		fmt.Fprintf(&b, "      - id: node-build\n        name: Node-Projekt bauen\n        %s:\n          action: build\n", d.NodeManager)
	}
	if d.Docker {
		count++
		b.WriteString("      - id: docker-build\n        name: Container bauen\n        dockerCompose:\n          action: build\n        when:\n          compose: true\n")
	}
	if count == 0 {
		b.WriteString("      - id: build\n        name: Kein Build erkannt\n        shell: |\n          printf 'Kein automatisch erkannter Build-Schritt.\\n'\n")
	}

	b.WriteString("\n  verify:\n    description: Ergebnis prüfen\n    steps:\n      - id: project-files\n        name: Projektstruktur prüfen\n        assert:\n          directoryExists: .\n")
	if d.Docker {
		b.WriteString("\n  containers:\n    description: Docker-Compose-Dienste starten\n    steps:\n      - id: docker-up\n        name: Container starten\n        dockerCompose:\n          action: up\n          detach: true\n          removeOrphans: true\n        when:\n          compose: true\n")
	}
	b.WriteString("\n  clean:\n    description: Generierte Artefakte bereinigen\n    steps:\n      - id: clean\n        name: Projektspezifische Bereinigung ergänzen\n        shell: |\n          printf 'TODO: projektspezifische Clean-Schritte ergänzen.\\n'\n")
	return b.String()
}

func yamlQuote(s string) string {
	if s == "" {
		return `""`
	}
	safe := true
	for _, r := range s {
		if !(r == '-' || r == '_' || r == '.' || r == '/' || r == '+' || r == ':' || r == ' ' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			safe = false
			break
		}
	}
	lower := strings.ToLower(strings.TrimSpace(s))
	if safe && s == strings.TrimSpace(s) && lower != "true" && lower != "false" && lower != "null" && !strings.Contains(s, "#") {
		return s
	}
	return strconv.Quote(s)
}
func yamlInlineList(values []string) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = yamlQuote(v)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
func writeBlock(b *strings.Builder, value, indent string) {
	for _, line := range strings.Split(value, "\n") {
		b.WriteString(indent + line + "\n")
	}
}
func uniqueStrings(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func PreviewConvertManifest(path string) (string, int, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", 0, err
	}
	m, err := ParseManifest(abs)
	if err != nil {
		return "", 0, err
	}
	if m.Version == 2 {
		data, err := os.ReadFile(abs)
		return string(data), 2, err
	}
	if m.Version != 1 {
		return "", m.Version, fmt.Errorf("update-cli.yaml Schema %d kann nicht automatisch konvertiert werden", m.Version)
	}
	return renderConvertedV1(m), m.Version, nil
}

func PreviewGeneratedManifest(root string) (string, []string, error) {
	d, err := detectProject(root)
	if err != nil {
		return "", nil, err
	}
	return renderDetectedManifest(d), append([]string(nil), d.Technologies...), nil
}

func SetupScriptTemplate() string { return generatedSetupScript }

// ProjectRepairResult describes structural cleanup performed by `update-cli fix`.
type ProjectRepairResult struct {
	Path           string   `json:"path"`
	BackupPath     string   `json:"backupPath,omitempty"`
	PreviousSchema int      `json:"previousSchema"`
	CurrentSchema  int      `json:"currentSchema"`
	Changed        bool     `json:"changed"`
	Canonicalized  bool     `json:"canonicalized,omitempty"`
	RemovedFields  []string `json:"removedFields,omitempty"`
	Normalized     []string `json:"normalizedValues,omitempty"`
}

// RepairProjectManifest migrates and sanitizes update-cli.yaml to the current
// supported schema. It removes unknown fields from supported sections and
// normalizes invalid optional values where a deterministic safe default exists.
func RepairProjectManifest(root string) (ProjectRepairResult, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ProjectRepairResult{}, err
	}
	canonical := filepath.Join(absRoot, "update-cli.yaml")
	legacy := filepath.Join(absRoot, "setup.yaml")
	path := canonical
	if _, err := os.Stat(canonical); errors.Is(err, os.ErrNotExist) {
		if _, legacyErr := os.Stat(legacy); legacyErr == nil {
			migration, migrateErr := MigrateProjectManifest(absRoot)
			if migrateErr != nil {
				return ProjectRepairResult{}, migrateErr
			}
			return ProjectRepairResult{Path: migration.Path, BackupPath: migration.BackupPath, PreviousSchema: migration.PreviousSchema, CurrentSchema: migration.CurrentSchema, Changed: migration.Changed, Canonicalized: migration.Canonicalized}, nil
		}
		return ProjectRepairResult{}, fmt.Errorf("update-cli.yaml fehlt in %s", absRoot)
	} else if err != nil {
		return ProjectRepairResult{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ProjectRepairResult{}, err
	}
	previous := detectSetupSchemaVersion(data)
	if previous == 1 {
		migration, migrateErr := MigrateProjectManifest(absRoot)
		if migrateErr == nil {
			return ProjectRepairResult{Path: migration.Path, BackupPath: migration.BackupPath, PreviousSchema: migration.PreviousSchema, CurrentSchema: migration.CurrentSchema, Changed: migration.Changed, Canonicalized: migration.Canonicalized}, nil
		}
		// Fall through to structural repair only when the legacy parser cannot
		// consume a transitional manifest. The simple YAML parser can still
		// safely remove unsupported metadata before a second conversion attempt.
	}
	rootNode, err := parseSimpleYAML(data)
	if err != nil {
		return ProjectRepairResult{}, fmt.Errorf("update-cli.yaml Syntax kann nicht automatisch repariert werden: %w", err)
	}
	if rootNode.kind != yamlMap {
		return ProjectRepairResult{}, errors.New("update-cli.yaml Top-Level muss eine Map sein")
	}
	removed := []string{}
	normalized := []string{}
	if previous != SchemaVersion {
		normalized = append(normalized, fmt.Sprintf("schemaVersion %d → %d", previous, SchemaVersion))
	}
	rootNode.m["schemaVersion"] = &simpleYAMLNode{kind: yamlScalar, scalar: strconv.Itoa(SchemaVersion), line: 0}
	delete(rootNode.m, "version") // project version is re-added below only when it is not the legacy schema selector

	allowedTop := map[string]bool{"schemaVersion": true, "project": true, "defaults": true, "variables": true, "requirements": true, "workflows": true, "tasks": true, "run": true, "update": true}
	for key := range rootNode.m {
		if !allowedTop[key] {
			delete(rootNode.m, key)
			removed = append(removed, key)
		}
	}

	sanitizeScalarMapSection(rootNode, "project", map[string]bool{"name": true, "slug": true, "description": true, "type": true}, &removed, &normalized)
	if defaults := rootNode.m["defaults"]; defaults != nil {
		if defaults.kind != yamlMap {
			delete(rootNode.m, "defaults")
			normalized = append(normalized, "defaults entfernt (keine Map)")
		} else {
			for key, value := range defaults.m {
				switch key {
				case "timeout":
					if value.kind != yamlScalar {
						delete(defaults.m, key)
						normalized = append(normalized, "defaults.timeout entfernt")
					}
				case "failFast":
					if _, err := nodeBool(value); err != nil {
						defaults.m[key] = &simpleYAMLNode{kind: yamlScalar, scalar: "true", line: value.line}
						normalized = append(normalized, "defaults.failFast → true")
					}
				default:
					delete(defaults.m, key)
					removed = append(removed, "defaults."+key)
				}
			}
		}
	}
	if variables := rootNode.m["variables"]; variables != nil {
		if variables.kind != yamlMap {
			delete(rootNode.m, "variables")
			normalized = append(normalized, "variables entfernt (keine Map)")
		} else {
			for key, value := range variables.m {
				if value.kind != yamlScalar {
					delete(variables.m, key)
					normalized = append(normalized, "variables."+key+" entfernt (kein skalarer Wert)")
				}
			}
		}
	}
	if requirements := rootNode.m["requirements"]; requirements != nil {
		if requirements.kind != yamlMap {
			delete(rootNode.m, "requirements")
			normalized = append(normalized, "requirements entfernt (keine Map)")
		} else {
			for key, value := range requirements.m {
				if key != "commands" && key != "optionalCommands" {
					delete(requirements.m, key)
					removed = append(removed, "requirements."+key)
					continue
				}
				requirements.m[key] = sanitizeStringListNode(value, &normalized, "requirements."+key)
			}
		}
	}
	sanitizeUpdateSection(rootNode, &removed, &normalized)
	sanitizeRunSection(rootNode, &removed, &normalized)
	taskNames := sanitizeTasksSection(rootNode, &removed, &normalized)
	sanitizeWorkflowsSection(rootNode, taskNames, &removed, &normalized)

	if tasks := rootNode.m["tasks"]; tasks != nil && tasks.kind == yamlMap {
		for name, task := range tasks.m {
			if task.kind != yamlMap {
				continue
			}
			if req := task.m["requires"]; req != nil {
				filtered := filterStringListNode(req, func(value string) bool { return taskNames[value] })
				if len(filtered.list) == 0 {
					delete(task.m, "requires")
				} else {
					task.m["requires"] = filtered
				}
			}
			if task.m["requires"] == nil && task.m["steps"] == nil {
				delete(tasks.m, name)
				delete(taskNames, name)
				normalized = append(normalized, "leeren task "+name+" entfernt")
			}
		}
	}

	if rootNode.m["tasks"] == nil && rootNode.m["run"] == nil && rootNode.m["update"] == nil {
		return ProjectRepairResult{}, errors.New("update-cli.yaml enthält nach Reparatur weder tasks, run noch update; automatische Reparatur wäre nicht sicher")
	}
	out := renderSimpleYAML(rootNode)
	// Validate the exact generated file before touching the original.
	tmp, err := os.CreateTemp(absRoot, ".fix-manifest-*.yaml")
	if err != nil {
		return ProjectRepairResult{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(out); err != nil {
		tmp.Close()
		return ProjectRepairResult{}, err
	}
	if err := tmp.Close(); err != nil {
		return ProjectRepairResult{}, err
	}
	if _, err := ParseManifest(tmpName); err != nil {
		return ProjectRepairResult{}, fmt.Errorf("repariertes update-cli.yaml ist weiterhin ungültig: %w", err)
	}

	changed := strings.TrimSpace(string(data)) != strings.TrimSpace(out)
	res := ProjectRepairResult{Path: canonical, PreviousSchema: previous, CurrentSchema: SchemaVersion, Changed: changed, RemovedFields: uniqueSortedStrings(removed), Normalized: uniqueSortedStrings(normalized)}
	if !changed {
		return res, nil
	}
	backup, err := backupManifestForFix(canonical)
	if err != nil {
		return res, err
	}
	res.BackupPath = backup
	if err := atomicWrite(canonical, []byte(out), 0o644); err != nil {
		return res, err
	}
	return res, nil
}

func sanitizeScalarMapSection(root *simpleYAMLNode, section string, allowed map[string]bool, removed, normalized *[]string) {
	n := root.m[section]
	if n == nil {
		return
	}
	if n.kind != yamlMap {
		delete(root.m, section)
		*normalized = append(*normalized, section+" entfernt (keine Map)")
		return
	}
	for key, value := range n.m {
		if !allowed[key] {
			delete(n.m, key)
			*removed = append(*removed, section+"."+key)
			continue
		}
		if value.kind != yamlScalar {
			delete(n.m, key)
			*normalized = append(*normalized, section+"."+key+" entfernt (kein skalarer Wert)")
		}
	}
}

func sanitizeUpdateSection(root *simpleYAMLNode, removed, normalized *[]string) {
	n := root.m["update"]
	if n == nil {
		return
	}
	if n.kind != yamlMap {
		delete(root.m, "update")
		*normalized = append(*normalized, "update entfernt (keine Map)")
		return
	}
	allowed := map[string]bool{"mode": true, "source": true, "releaseDir": true, "currentDir": true, "backup": true, "retention": true, "sync": true, "setup": true, "docker": true, "healthcheck": true}
	for key, value := range n.m {
		if !allowed[key] {
			delete(n.m, key)
			*removed = append(*removed, "update."+key)
			continue
		}
		if key == "releaseDir" || key == "currentDir" {
			if value.kind != yamlScalar || strings.TrimSpace(value.scalar) == "" {
				delete(n.m, key)
				*normalized = append(*normalized, "update."+key+" entfernt")
			}
		}
	}

	sourceType := ""
	if source := n.m["source"]; source != nil {
		if source.kind != yamlMap {
			delete(n.m, "source")
			*normalized = append(*normalized, "update.source entfernt (keine Map)")
		} else {
			allowedSource := map[string]bool{"type": true, "folder": true, "url": true, "repository": true, "ref": true, "commit": true, "version": true, "sha256": true}
			for key, value := range source.m {
				if !allowedSource[key] {
					delete(source.m, key)
					*removed = append(*removed, "update.source."+key)
					continue
				}
				if value.kind != yamlScalar {
					delete(source.m, key)
					*normalized = append(*normalized, "update.source."+key+" entfernt")
				}
			}
			if v := source.m["type"]; v != nil && v.kind == yamlScalar {
				sourceType = strings.ToLower(strings.TrimSpace(v.scalar))
			}
			if sourceType != "download" && sourceType != "url" && sourceType != "repository" {
				switch {
				case source.m["repository"] != nil:
					sourceType = "repository"
				case source.m["url"] != nil:
					sourceType = "url"
				case source.m["folder"] != nil:
					sourceType = "download"
				default:
					sourceType = ""
				}
				if sourceType == "" {
					delete(source.m, "type")
				} else {
					source.m["type"] = &simpleYAMLNode{kind: yamlScalar, scalar: sourceType, line: source.line}
					*normalized = append(*normalized, "update.source.type normalisiert")
				}
			}
			if len(source.m) == 0 {
				delete(n.m, "source")
			}
		}
	}

	if mode := n.m["mode"]; mode != nil {
		if mode.kind != yamlScalar {
			delete(n.m, "mode")
			*normalized = append(*normalized, "update.mode entfernt")
		} else {
			value := strings.ToLower(strings.TrimSpace(mode.scalar))
			if value != "update" && value != "pull" {
				if sourceType == "repository" {
					value = "pull"
				} else if sourceType == "download" || sourceType == "url" {
					value = "update"
				} else {
					delete(n.m, "mode")
					value = ""
				}
				if value != "" {
					n.m["mode"] = &simpleYAMLNode{kind: yamlScalar, scalar: value, line: mode.line}
				}
				*normalized = append(*normalized, "update.mode normalisiert")
			}
		}
	}

	if backup := n.m["backup"]; backup != nil {
		if backup.kind != yamlMap {
			delete(n.m, "backup")
			*normalized = append(*normalized, "update.backup entfernt (keine Map)")
		} else {
			for key, value := range backup.m {
				switch key {
				case "directory":
					if value.kind != yamlScalar || strings.TrimSpace(value.scalar) == "" {
						delete(backup.m, key)
						*normalized = append(*normalized, "update.backup.directory entfernt")
					}
				case "keep":
					v, err := nodeInt(value)
					if err != nil || v < 0 {
						delete(backup.m, key)
						*normalized = append(*normalized, "update.backup.keep entfernt")
					}
				default:
					delete(backup.m, key)
					*removed = append(*removed, "update.backup."+key)
				}
			}
			if len(backup.m) == 0 {
				delete(n.m, "backup")
			}
		}
	}
	if retention := n.m["retention"]; retention != nil {
		if retention.kind != yamlMap {
			delete(n.m, "retention")
			*normalized = append(*normalized, "update.retention entfernt (keine Map)")
		} else {
			for key, value := range retention.m {
				if key != "releases" {
					delete(retention.m, key)
					*removed = append(*removed, "update.retention."+key)
					continue
				}
				v, err := nodeInt(value)
				if err != nil || v < 0 {
					delete(retention.m, key)
					*normalized = append(*normalized, "update.retention.releases entfernt")
				}
			}
			if len(retention.m) == 0 {
				delete(n.m, "retention")
			}
		}
	}
	if sync := n.m["sync"]; sync != nil {
		if sync.kind != yamlMap {
			delete(n.m, "sync")
			*normalized = append(*normalized, "update.sync entfernt (keine Map)")
		} else {
			for key, value := range sync.m {
				switch key {
				case "preserve":
					sync.m[key] = sanitizeStringListNode(value, normalized, "update.sync.preserve")
				case "keepOnSetupError":
					if _, err := nodeBool(value); err != nil {
						delete(sync.m, key)
						*normalized = append(*normalized, "update.sync.keepOnSetupError entfernt")
					}
				default:
					delete(sync.m, key)
					*removed = append(*removed, "update.sync."+key)
				}
			}
		}
	}
	// 2.11.x/2.12.0-2.12.1 briefly emitted update.setup.keepRsyncOnError.
	// Move it into the bootstrap-compatible canonical sync section.
	if setup := n.m["setup"]; setup != nil {
		if setup.kind == yamlMap {
			if value := setup.m["keepRsyncOnError"]; value != nil {
				if _, err := nodeBool(value); err == nil {
					sync := n.m["sync"]
					if sync == nil || sync.kind != yamlMap {
						sync = &simpleYAMLNode{kind: yamlMap, m: map[string]*simpleYAMLNode{}, line: setup.line}
						n.m["sync"] = sync
					}
					if sync.m["keepOnSetupError"] == nil {
						sync.m["keepOnSetupError"] = value
						*normalized = append(*normalized, "update.setup.keepRsyncOnError → update.sync.keepOnSetupError")
					}
				} else {
					*normalized = append(*normalized, "update.setup.keepRsyncOnError entfernt")
				}
			}
			for key := range setup.m {
				if key != "keepRsyncOnError" {
					*removed = append(*removed, "update.setup."+key)
				}
			}
		} else {
			*normalized = append(*normalized, "update.setup entfernt (keine Map)")
		}
		delete(n.m, "setup")
	}
	if docker := n.m["docker"]; docker != nil {
		if docker.kind != yamlMap {
			delete(n.m, "docker")
			*normalized = append(*normalized, "update.docker entfernt (keine Map)")
		} else {
			for key, value := range docker.m {
				if key != "lifecycle" {
					delete(docker.m, key)
					*removed = append(*removed, "update.docker."+key)
					continue
				}
				lifecycle := strings.ToLower(strings.TrimSpace(value.scalar))
				if value.kind != yamlScalar || (lifecycle != "auto" && lifecycle != "disabled" && lifecycle != "required") {
					docker.m[key] = &simpleYAMLNode{kind: yamlScalar, scalar: "auto", line: value.line}
					*normalized = append(*normalized, "update.docker.lifecycle → auto")
				}
			}
		}
	}
	if health := n.m["healthcheck"]; health != nil {
		if health.kind != yamlMap {
			delete(n.m, "healthcheck")
			*normalized = append(*normalized, "update.healthcheck entfernt (keine Map)")
		} else {
			for key, value := range health.m {
				switch key {
				case "type":
					v := strings.ToLower(strings.TrimSpace(value.scalar))
					if value.kind != yamlScalar || (v != "none" && v != "http" && v != "command") {
						delete(health.m, key)
						*normalized = append(*normalized, "update.healthcheck.type entfernt")
					}
				case "url", "command":
					if value.kind != yamlScalar {
						delete(health.m, key)
						*normalized = append(*normalized, "update.healthcheck."+key+" entfernt")
					}
				case "timeoutSeconds":
					v, err := nodeInt(value)
					if err != nil || v < 0 {
						delete(health.m, key)
						*normalized = append(*normalized, "update.healthcheck.timeoutSeconds entfernt")
					}
				default:
					delete(health.m, key)
					*removed = append(*removed, "update.healthcheck."+key)
				}
			}
			if len(health.m) == 0 {
				delete(n.m, "healthcheck")
			}
		}
	}
	if len(n.m) == 0 {
		delete(root.m, "update")
	}
}

func sanitizeRunSection(root *simpleYAMLNode, removed, normalized *[]string) {
	n := root.m["run"]
	if n == nil {
		return
	}
	if n.kind == yamlScalar {
		if strings.TrimSpace(n.scalar) == "" {
			delete(root.m, "run")
			*normalized = append(*normalized, "leeres run entfernt")
		}
		return
	}
	if n.kind != yamlMap {
		delete(root.m, "run")
		*normalized = append(*normalized, "run entfernt (ungültige Struktur)")
		return
	}
	allowed := map[string]bool{"description": true, "command": true, "cwd": true, "workingDirectory": true, "env": true, "steps": true}
	for key, value := range n.m {
		if !allowed[key] {
			delete(n.m, key)
			*removed = append(*removed, "run."+key)
			continue
		}
		switch key {
		case "description", "command", "cwd", "workingDirectory":
			if value.kind != yamlScalar {
				delete(n.m, key)
				*normalized = append(*normalized, "run."+key+" entfernt")
			}
		case "env":
			sanitizeEnvNode(n, key, value, normalized)
		case "steps":
			n.m[key] = sanitizeStepsNode(value, removed, normalized, "run.steps")
		}
	}
	if n.m["command"] != nil && n.m["steps"] != nil {
		delete(n.m, "steps")
		*normalized = append(*normalized, "run.steps entfernt, da run.command gesetzt ist")
	}
	if n.m["command"] == nil && n.m["steps"] == nil {
		delete(root.m, "run")
		*normalized = append(*normalized, "run ohne command/steps entfernt")
	}
}

func sanitizeTasksSection(root *simpleYAMLNode, removed, normalized *[]string) map[string]bool {
	n := root.m["tasks"]
	names := map[string]bool{}
	if n == nil {
		return names
	}
	if n.kind != yamlMap {
		delete(root.m, "tasks")
		*normalized = append(*normalized, "tasks entfernt (keine Map)")
		return names
	}
	for name, task := range n.m {
		if task.kind != yamlMap {
			delete(n.m, name)
			*normalized = append(*normalized, "task "+name+" entfernt (keine Map)")
			continue
		}
		for key, value := range task.m {
			switch key {
			case "description":
				if value.kind != yamlScalar {
					delete(task.m, key)
					*normalized = append(*normalized, "task "+name+" description entfernt")
				}
			case "requires":
				task.m[key] = sanitizeStringListNode(value, normalized, "tasks."+name+".requires")
			case "steps":
				clean := sanitizeStepsNode(value, removed, normalized, "tasks."+name+".steps")
				if clean == nil || len(clean.list) == 0 {
					delete(task.m, key)
				} else {
					task.m[key] = clean
				}
			default:
				delete(task.m, key)
				*removed = append(*removed, "tasks."+name+"."+key)
			}
		}
		if task.m["steps"] == nil && task.m["requires"] == nil {
			delete(n.m, name)
			*normalized = append(*normalized, "leeren task "+name+" entfernt")
			continue
		}
		names[name] = true
	}
	if len(n.m) == 0 {
		delete(root.m, "tasks")
	}
	return names
}

func sanitizeWorkflowsSection(root *simpleYAMLNode, taskNames map[string]bool, removed, normalized *[]string) {
	n := root.m["workflows"]
	if n == nil {
		return
	}
	if n.kind != yamlMap {
		delete(root.m, "workflows")
		*normalized = append(*normalized, "workflows entfernt (keine Map)")
		return
	}
	for name, w := range n.m {
		if w.kind != yamlMap {
			delete(n.m, name)
			*normalized = append(*normalized, "workflow "+name+" entfernt")
			continue
		}
		for key, value := range w.m {
			switch key {
			case "description":
				if value.kind != yamlScalar {
					delete(w.m, key)
				}
			case "tasks":
				w.m[key] = filterStringListNode(value, func(v string) bool { return taskNames[v] })
			default:
				delete(w.m, key)
				*removed = append(*removed, "workflows."+name+"."+key)
			}
		}
		if tasks := w.m["tasks"]; tasks == nil || tasks.kind != yamlList || len(tasks.list) == 0 {
			delete(n.m, name)
			*normalized = append(*normalized, "workflow "+name+" ohne gültige tasks entfernt")
		}
	}
	if len(n.m) == 0 {
		delete(root.m, "workflows")
	}
}

func sanitizeStepsNode(n *simpleYAMLNode, removed, normalized *[]string, prefix string) *simpleYAMLNode {
	if n == nil || n.kind != yamlList {
		*normalized = append(*normalized, prefix+" entfernt (keine Liste)")
		return nil
	}
	out := &simpleYAMLNode{kind: yamlList, line: n.line}
	for index, item := range n.list {
		if item.kind != yamlMap {
			*normalized = append(*normalized, fmt.Sprintf("%s[%d] entfernt", prefix, index))
			continue
		}
		allowedMeta := map[string]bool{"id": true, "name": true, "cwd": true, "env": true, "timeout": true, "retries": true, "allowFailure": true, "continueOnError": true, "when": true}
		operations := []string{}
		for key, value := range item.m {
			if v2OperationKeys[key] {
				operations = append(operations, key)
				continue
			}
			if !allowedMeta[key] {
				delete(item.m, key)
				*removed = append(*removed, fmt.Sprintf("%s[%d].%s", prefix, index, key))
				continue
			}
			switch key {
			case "id", "name", "cwd", "timeout":
				if value.kind != yamlScalar {
					delete(item.m, key)
					*normalized = append(*normalized, fmt.Sprintf("%s[%d].%s entfernt", prefix, index, key))
				}
			case "env":
				sanitizeEnvNode(item, key, value, normalized)
			case "retries":
				if v, err := nodeInt(value); err != nil || v < 0 {
					delete(item.m, key)
					*normalized = append(*normalized, fmt.Sprintf("%s[%d].retries entfernt", prefix, index))
				}
			case "allowFailure", "continueOnError":
				if _, err := nodeBool(value); err != nil {
					delete(item.m, key)
					*normalized = append(*normalized, fmt.Sprintf("%s[%d].%s entfernt", prefix, index, key))
				}
			}
		}
		if len(operations) == 0 {
			*normalized = append(*normalized, fmt.Sprintf("%s[%d] ohne Operation entfernt", prefix, index))
			continue
		}
		if len(operations) > 1 {
			sort.Slice(operations, func(i, j int) bool { return item.m[operations[i]].line < item.m[operations[j]].line })
			keep := operations[0]
			for _, op := range operations[1:] {
				delete(item.m, op)
				*removed = append(*removed, fmt.Sprintf("%s[%d].%s (zweite Operation; %s bleibt)", prefix, index, op, keep))
			}
		}
		out.list = append(out.list, item)
	}
	return out
}

func sanitizeEnvNode(parent *simpleYAMLNode, key string, value *simpleYAMLNode, normalized *[]string) {
	if value.kind != yamlMap {
		delete(parent.m, key)
		*normalized = append(*normalized, key+" entfernt (keine Map)")
		return
	}
	for envKey, envValue := range value.m {
		if envValue.kind != yamlScalar {
			delete(value.m, envKey)
			*normalized = append(*normalized, key+"."+envKey+" entfernt")
		}
	}
}
func sanitizeStringListNode(n *simpleYAMLNode, normalized *[]string, name string) *simpleYAMLNode {
	out := filterStringListNode(n, func(string) bool { return true })
	if out == nil {
		*normalized = append(*normalized, name+" → []")
		return &simpleYAMLNode{kind: yamlList, line: lineOf(n)}
	}
	return out
}
func filterStringListNode(n *simpleYAMLNode, keep func(string) bool) *simpleYAMLNode {
	out := &simpleYAMLNode{kind: yamlList, line: lineOf(n)}
	if n == nil {
		return out
	}
	if n.kind == yamlScalar {
		if s := strings.TrimSpace(n.scalar); s != "" && keep(s) {
			out.list = append(out.list, &simpleYAMLNode{kind: yamlScalar, scalar: s, line: n.line})
		}
		return out
	}
	if n.kind != yamlList {
		return out
	}
	seen := map[string]bool{}
	for _, item := range n.list {
		if item.kind != yamlScalar {
			continue
		}
		value := strings.TrimSpace(item.scalar)
		if value == "" || seen[value] || !keep(value) {
			continue
		}
		seen[value] = true
		out.list = append(out.list, &simpleYAMLNode{kind: yamlScalar, scalar: value, line: item.line})
	}
	return out
}
func lineOf(n *simpleYAMLNode) int {
	if n == nil {
		return 0
	}
	return n.line
}

func renderSimpleYAML(root *simpleYAMLNode) string {
	var b strings.Builder
	renderYAMLNode(&b, root, 0, false)
	if !strings.HasSuffix(b.String(), "\n") {
		b.WriteByte('\n')
	}
	return b.String()
}
func renderYAMLNode(b *strings.Builder, n *simpleYAMLNode, indent int, listItem bool) {
	if n == nil {
		return
	}
	pad := strings.Repeat(" ", indent)
	switch n.kind {
	case yamlMap:
		keys := make([]string, 0, len(n.m))
		for key := range n.m {
			keys = append(keys, key)
		}
		sort.SliceStable(keys, func(i, j int) bool {
			if keys[i] == "schemaVersion" {
				return true
			}
			if keys[j] == "schemaVersion" {
				return false
			}
			li, lj := n.m[keys[i]].line, n.m[keys[j]].line
			if li == lj {
				return keys[i] < keys[j]
			}
			return li < lj
		})
		for _, key := range keys {
			child := n.m[key]
			b.WriteString(pad)
			b.WriteString(key)
			b.WriteString(":")
			if child.kind == yamlScalar {
				renderYAMLScalar(b, child, indent)
			} else {
				b.WriteByte('\n')
				renderYAMLNode(b, child, indent+2, false)
			}
		}
	case yamlList:
		for _, child := range n.list {
			b.WriteString(pad)
			b.WriteString("-")
			if child.kind == yamlScalar {
				renderYAMLScalar(b, child, indent)
			} else if child.kind == yamlMap {
				keys := make([]string, 0, len(child.m))
				for key := range child.m {
					keys = append(keys, key)
				}
				sort.SliceStable(keys, func(i, j int) bool { return child.m[keys[i]].line < child.m[keys[j]].line })
				if len(keys) == 0 {
					b.WriteString(" {}\n")
					continue
				}
				first := keys[0]
				firstNode := child.m[first]
				b.WriteByte(' ')
				b.WriteString(first)
				b.WriteString(":")
				if firstNode.kind == yamlScalar {
					renderYAMLScalar(b, firstNode, indent+2)
				} else {
					b.WriteByte('\n')
					renderYAMLNode(b, firstNode, indent+4, false)
				}
				for _, key := range keys[1:] {
					node := child.m[key]
					b.WriteString(strings.Repeat(" ", indent+2))
					b.WriteString(key)
					b.WriteString(":")
					if node.kind == yamlScalar {
						renderYAMLScalar(b, node, indent+2)
					} else {
						b.WriteByte('\n')
						renderYAMLNode(b, node, indent+4, false)
					}
				}
			} else {
				b.WriteByte('\n')
				renderYAMLNode(b, child, indent+2, true)
			}
		}
	case yamlScalar:
		if listItem {
			b.WriteString(pad)
			b.WriteString("-")
		}
		renderYAMLScalar(b, n, indent)
	}
}
func renderYAMLScalar(b *strings.Builder, n *simpleYAMLNode, indent int) {
	if strings.Contains(n.scalar, "\n") {
		b.WriteString(" |\n")
		for _, line := range strings.Split(n.scalar, "\n") {
			b.WriteString(strings.Repeat(" ", indent+2))
			b.WriteString(line)
			b.WriteByte('\n')
		}
		return
	}
	b.WriteByte(' ')
	b.WriteString(strconv.Quote(n.scalar))
	b.WriteByte('\n')
}
func backupManifestForFix(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	base := path + ".backup-fix-" + time.Now().Format("20060102-150405")
	for i := 0; ; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i+1)
		}
		f, err := os.OpenFile(candidate, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if _, err := f.Write(data); err != nil {
			f.Close()
			os.Remove(candidate)
			return "", err
		}
		if err := f.Close(); err != nil {
			return "", err
		}
		return candidate, nil
	}
}
func uniqueSortedStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
