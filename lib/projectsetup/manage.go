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
