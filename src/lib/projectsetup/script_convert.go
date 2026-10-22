package projectsetup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type ScriptAnalysis struct {
	Path        string   `json:"path"`
	Steps       int      `json:"steps"`
	Legacy      bool     `json:"legacyTemplate"`
	Project     string   `json:"project,omitempty"`
	ProjectType string   `json:"projectType,omitempty"`
	Commands    []string `json:"commands,omitempty"`
}

type scriptStep struct {
	ID        string
	Name      string
	Operation string
	Body      string
}

func PreviewGeneratedManifestFromSetupScript(root, scriptPath string) (string, []string, ScriptAnalysis, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", nil, ScriptAnalysis{}, err
	}
	if scriptPath == "" {
		scriptPath = filepath.Join(absRoot, "setup.sh")
	}
	if !filepath.IsAbs(scriptPath) {
		scriptPath = filepath.Join(absRoot, scriptPath)
	}
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, ScriptAnalysis{}, fmt.Errorf("setup.sh fehlt: %s", scriptPath)
		}
		return "", nil, ScriptAnalysis{}, err
	}
	detected, err := detectProject(absRoot)
	if err != nil {
		return "", nil, ScriptAnalysis{}, err
	}
	text := string(data)
	assignments := parseShellAssignments(text)
	steps, legacy := analyzeSetupScript(text, assignments)
	if len(steps) == 0 {
		steps = []scriptStep{{
			ID:        "legacy-setup-script",
			Name:      "Nicht automatisch zerlegbares setup.sh ausführen",
			Operation: "shell",
			Body:      "SETUP_WAIT=0 SETUP_TUI_MODE=plain bash ./setup.sh",
		}}
	}
	projectName := firstNonBlank(assignments["PROJECT_NAME"], assignments["PROJECT_SLUG"], detected.Name)
	projectType := detected.Type
	if v := strings.TrimSpace(assignments["PROJECT_TYPE"]); v != "" {
		projectType = v
	}
	out := renderScriptManifest(projectName, projectType, detected, steps)
	analysis := ScriptAnalysis{Path: scriptPath, Steps: len(steps), Legacy: legacy, Project: projectName, ProjectType: projectType}
	for _, s := range steps {
		analysis.Commands = append(analysis.Commands, s.Body)
	}
	return out, detected.Technologies, analysis, nil
}

func GenerateManifestFromSetupScript(root, scriptPath, path string, force bool, refinedText string) (GenerateResult, ScriptAnalysis, error) {
	text, technologies, analysis, err := PreviewGeneratedManifestFromSetupScript(root, scriptPath)
	if err != nil {
		return GenerateResult{}, analysis, err
	}
	if strings.TrimSpace(refinedText) != "" {
		text = refinedText
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return GenerateResult{}, analysis, err
	}
	if path == "" {
		path = filepath.Join(absRoot, "update-cli.yaml")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(absRoot, path)
	}
	existed := false
	if info, statErr := os.Stat(path); statErr == nil {
		if info.IsDir() {
			return GenerateResult{}, analysis, fmt.Errorf("Ziel ist ein Ordner: %s", path)
		}
		existed = true
		if !force {
			return GenerateResult{}, analysis, fmt.Errorf("update-cli.yaml existiert bereits: %s; --force verwenden", path)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return GenerateResult{}, analysis, statErr
	}
	if err := validateManifestText(filepath.Dir(path), text); err != nil {
		return GenerateResult{}, analysis, err
	}
	if err := atomicWrite(path, []byte(text), 0o644); err != nil {
		return GenerateResult{}, analysis, err
	}
	return GenerateResult{Path: path, Technologies: technologies, Overwritten: existed}, analysis, nil
}

func validateManifestText(dir, text string) error {
	tmp, err := os.CreateTemp(dir, ".setup-generated-*.yaml")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.WriteString(text); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	m, err := ParseManifest(name)
	if err != nil {
		return fmt.Errorf("erzeugtes update-cli.yaml ist ungültig: %w", err)
	}
	if m.Version != 2 {
		return fmt.Errorf("erzeugtes update-cli.yaml muss schemaVersion 2 verwenden")
	}
	return nil
}

func analyzeSetupScript(script string, vars map[string]string) ([]scriptStep, bool) {
	ids := parseShellArray(script, "SETUP_STEPS")
	if len(ids) > 0 {
		return analyzeLegacySetupSteps(ids, script, vars), true
	}
	commands := extractCommonShellCommands(script)
	steps := make([]scriptStep, 0, len(commands))
	for i, cmd := range commands {
		steps = append(steps, classifyScriptCommand(fmt.Sprintf("step-%02d", i+1), cmd))
	}
	return steps, false
}

func analyzeLegacySetupSteps(ids []string, script string, vars map[string]string) []scriptStep {
	pre := parseShellArray(script, "PRE_COMMANDS")
	custom := parseShellArray(script, "CUSTOM_COMMANDS")
	post := parseShellArray(script, "POST_COMMANDS")
	steps := []scriptStep{}
	appendCommands := func(prefix, title string, commands []string) {
		for i, cmd := range commands {
			s := classifyScriptCommand(fmt.Sprintf("%s-%02d", prefix, i+1), cmd)
			s.Name = title
			if len(commands) > 1 {
				s.Name = fmt.Sprintf("%s %d", title, i+1)
			}
			steps = append(steps, s)
		}
	}
	for _, rawID := range ids {
		id := strings.Trim(rawID, " \t\r\n\"'")
		switch id {
		case "pre-commands":
			appendCommands("pre", "Vorbereitendes Kommando", pre)
		case "custom-commands":
			appendCommands("custom", "Projektkommando", custom)
		case "post-commands":
			appendCommands("post", "Abschließendes Kommando", post)
		case "go-mod-download":
			steps = append(steps, scriptStep{ID: id, Name: "Go-Module laden", Operation: "go", Body: "mod-download"})
		case "go-vet":
			pkg := firstNonBlank(vars["GO_PACKAGE"], "./...")
			if pkg == "./..." {
				steps = append(steps, scriptStep{ID: id, Name: "Go-Quellcode prüfen", Operation: "go", Body: "vet"})
			} else {
				steps = append(steps, scriptStep{ID: id, Name: "Go-Quellcode prüfen", Operation: "shell", Body: "go vet " + pkg})
			}
		case "go-test":
			pkg := firstNonBlank(vars["GO_PACKAGE"], "./...")
			if pkg == "./..." {
				steps = append(steps, scriptStep{ID: id, Name: "Go-Tests ausführen", Operation: "go", Body: "test"})
			} else {
				steps = append(steps, scriptStep{ID: id, Name: "Go-Tests ausführen", Operation: "shell", Body: "go test " + pkg})
			}
		case "go-build":
			output := filepath.ToSlash(filepath.Join(firstNonBlank(vars["DIST_DIR"], "bin"), firstNonBlank(vars["BINARY_NAME"], "app")))
			pkg := firstNonBlank(vars["GO_BUILD_PACKAGE"], ".")
			steps = append(steps, scriptStep{ID: id, Name: "Go-Binary bauen", Operation: "go-build", Body: output + "\n" + pkg})
		case "binary-version-check":
			binary := filepath.ToSlash(filepath.Join(firstNonBlank(vars["DIST_DIR"], "bin"), firstNonBlank(vars["BINARY_NAME"], "app")))
			steps = append(steps, scriptStep{ID: id, Name: "Binary-Version prüfen", Operation: "shell", Body: "./" + strings.TrimPrefix(binary, "./") + " --version"})
		case "docker-down":
			steps = append(steps, scriptStep{ID: id, Name: "Docker-Container stoppen", Operation: "docker", Body: "down"})
		case "docker-up":
			steps = append(steps, scriptStep{ID: id, Name: "Docker-Container starten", Operation: "docker", Body: "up"})
		case "npm-install":
			steps = append(steps, scriptStep{ID: id, Name: "Node-Abhängigkeiten installieren", Operation: "npm", Body: "install"})
		case "npm-build":
			steps = append(steps, scriptStep{ID: id, Name: "Frontend bauen", Operation: "npm", Body: "build"})
		case "composer-install":
			steps = append(steps, scriptStep{ID: id, Name: "Composer-Abhängigkeiten installieren", Operation: "composer", Body: "install"})
		case "python-venv":
			steps = append(steps, scriptStep{ID: id, Name: "Python-Umgebung erstellen", Operation: "python-venv", Body: firstNonBlank(vars["PYTHON_VENV_DIR"], ".venv")})
		case "pip-install":
			steps = append(steps, scriptStep{ID: id, Name: "Python-Abhängigkeiten installieren", Operation: "pip", Body: firstNonBlank(vars["PYTHON_REQUIREMENTS_FILE"], "requirements.txt")})
		case "just-commands":
			appendCommands("just", "Just-Kommando", prefixCommands("just ", parseShellArray(script, "JUST_COMMANDS")))
		default:
			// Unknown built-in steps are retained as a conservative shell marker instead of invented behavior.
			steps = append(steps, scriptStep{ID: sanitizeID(id), Name: humanizeStepID(id), Operation: "shell", Body: "# TODO: aus setup.sh prüfen: " + id})
		}
	}
	return steps
}

func classifyScriptCommand(id, cmd string) scriptStep {
	cmd = strings.TrimSpace(cmd)
	name := commandTitle(cmd)
	s := scriptStep{ID: sanitizeID(id), Name: name, Operation: "shell", Body: cmd}
	switch cmd {
	case "go mod download":
		s.Operation, s.Body = "go", "mod-download"
	case "go vet ./...":
		s.Operation, s.Body = "go", "vet"
	case "go test ./...":
		s.Operation, s.Body = "go", "test"
	case "docker compose down", "docker compose down --remove-orphans":
		s.Operation, s.Body = "docker", "down"
	case "docker compose up -d", "docker compose up -d --remove-orphans":
		s.Operation, s.Body = "docker", "up"
	case "composer install":
		s.Operation, s.Body = "composer", "install"
	}
	return s
}

func renderScriptManifest(projectName, projectType string, d detectedProject, steps []scriptStep) string {
	var b strings.Builder
	b.WriteString("schemaVersion: 2\n\nproject:\n")
	fmt.Fprintf(&b, "  name: %s\n", yamlQuote(projectName))
	fmt.Fprintf(&b, "  type: %s\n", yamlQuote(projectType))
	b.WriteString("  description: Aus setup.sh deterministisch konvertierte Projekt-Automation; vor produktivem Einsatz prüfen\n")
	b.WriteString("\ndefaults:\n  timeout: 15m\n  failFast: true\n\n")
	req := requirementsForDetected(d)
	if len(req) > 0 {
		b.WriteString("requirements:\n  commands:\n")
		for _, r := range req {
			fmt.Fprintf(&b, "    - %s\n", yamlQuote(r))
		}
		b.WriteString("\n")
	}
	b.WriteString("workflows:\n  setup:\n    description: Aus setup.sh konvertierter Setup-Workflow\n    tasks:\n      - setup\n\ntasks:\n  setup:\n    description: In setup.sh erkannte Setup-Schritte in Originalreihenfolge\n    steps:\n")
	for i, s := range steps {
		id := s.ID
		if id == "" {
			id = fmt.Sprintf("step-%02d", i+1)
		}
		fmt.Fprintf(&b, "      - id: %s\n        name: %s\n", yamlQuote(id), yamlQuote(s.Name))
		switch s.Operation {
		case "go":
			b.WriteString("        go:\n")
			fmt.Fprintf(&b, "          action: %s\n", yamlQuote(s.Body))
		case "go-build":
			parts := strings.SplitN(s.Body, "\n", 2)
			out, pkg := parts[0], "."
			if len(parts) == 2 {
				pkg = parts[1]
			}
			b.WriteString("        go:\n          action: build\n")
			fmt.Fprintf(&b, "          output: %s\n          package: %s\n", yamlQuote(out), yamlQuote(pkg))
		case "docker":
			b.WriteString("        dockerCompose:\n")
			fmt.Fprintf(&b, "          action: %s\n", yamlQuote(s.Body))
			if s.Body == "up" {
				b.WriteString("          detach: true\n          removeOrphans: true\n")
			}
		case "npm":
			b.WriteString("        npm:\n")
			fmt.Fprintf(&b, "          action: %s\n", yamlQuote(s.Body))
		case "composer":
			b.WriteString("        composer:\n")
			fmt.Fprintf(&b, "          action: %s\n", yamlQuote(s.Body))
		case "python-venv":
			b.WriteString("        pythonVenv:\n")
			fmt.Fprintf(&b, "          path: %s\n          python: python3\n", yamlQuote(s.Body))
		case "pip":
			b.WriteString("        pip:\n          python: .venv/bin/python\n")
			fmt.Fprintf(&b, "          requirements: %s\n", yamlQuote(s.Body))
		default:
			b.WriteString("        shell: |\n")
			writeBlock(&b, s.Body, "          ")
		}
	}
	return b.String()
}

func requirementsForDetected(d detectedProject) []string {
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
	sort.Strings(req)
	return uniqueStrings(req)
}

func parseShellAssignments(script string) map[string]string {
	out := map[string]string{}
	re := regexp.MustCompile(`(?m)^\s*([A-Z][A-Z0-9_]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\n#]+))\s*$`)
	for _, m := range re.FindAllStringSubmatch(script, -1) {
		value := m[2]
		if value == "" {
			value = m[3]
		}
		if value == "" {
			value = strings.TrimSpace(m[4])
		}
		out[m[1]] = value
	}
	return out
}

func parseShellArray(script, name string) []string {
	re := regexp.MustCompile(`(?ms)^\s*` + regexp.QuoteMeta(name) + `\s*=\s*\((.*?)\)`)
	m := re.FindStringSubmatch(script)
	if len(m) != 2 {
		return nil
	}
	return shellArrayWords(m[1])
}

func shellArrayWords(body string) []string {
	re := regexp.MustCompile(`(?s)"((?:\\.|[^"])*)"|'((?:\\.|[^'])*)'|([^\s#]+)`)
	out := []string{}
	for _, m := range re.FindAllStringSubmatch(body, -1) {
		v := m[1]
		if v == "" {
			v = m[2]
		}
		if v == "" {
			v = m[3]
		}
		v = strings.TrimSpace(strings.ReplaceAll(v, `\"`, `"`))
		if v != "" && !strings.HasPrefix(v, "#") {
			out = append(out, v)
		}
	}
	return out
}

func extractCommonShellCommands(script string) []string {
	physical := strings.Split(strings.ReplaceAll(script, "\r\n", "\n"), "\n")
	logical := []string{}
	current := ""
	for _, raw := range physical {
		line := strings.TrimSpace(raw)
		if current != "" {
			current += " " + strings.TrimSpace(strings.TrimSuffix(line, `\`))
		} else {
			current = strings.TrimSpace(strings.TrimSuffix(line, `\`))
		}
		if strings.HasSuffix(line, `\`) {
			continue
		}
		if current != "" {
			logical = append(logical, current)
		}
		current = ""
	}
	prefixes := []string{"go ", "gofmt ", "npm ", "pnpm ", "yarn ", "composer ", "php artisan ", "docker compose ", "docker-compose ", "python ", "python3 ", "pip ", "pip3 ", "mkdir ", "install ", "cp ", "chmod ", "just ", "make ", "./"}
	out := []string{}
	seen := map[string]bool{}
	for _, line := range logical {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.Contains(line, "command -v ") || strings.Contains(line, ">> \"${CURRENT_STEP_OUTPUT_FILE}") {
			continue
		}
		for _, p := range prefixes {
			if strings.HasPrefix(line, p) {
				if !seen[line] {
					seen[line] = true
					out = append(out, line)
				}
				break
			}
		}
	}
	return out
}

func prefixCommands(prefix string, values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, prefix+v)
	}
	return out
}
func firstNonBlank(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
func sanitizeID(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	re := regexp.MustCompile(`[^a-z0-9._-]+`)
	v = strings.Trim(re.ReplaceAllString(v, "-"), "-")
	if v == "" {
		return "step"
	}
	return v
}
func humanizeStepID(v string) string {
	v = strings.ReplaceAll(strings.ReplaceAll(v, "-", " "), "_", " ")
	if v == "" {
		return "Setup-Schritt"
	}
	return strings.ToUpper(v[:1]) + v[1:]
}
func commandTitle(cmd string) string {
	lower := strings.ToLower(cmd)
	switch {
	case strings.Contains(lower, "go mod "):
		return "Go-Module vorbereiten"
	case strings.HasPrefix(lower, "gofmt "):
		return "Go-Quellcode formatieren"
	case strings.Contains(lower, "go vet "):
		return "Go-Quellcode prüfen"
	case strings.Contains(lower, "go test "):
		return "Go-Tests ausführen"
	case strings.Contains(lower, "go build "):
		return "Go-Projekt bauen"
	case strings.Contains(lower, "npm ") || strings.Contains(lower, "pnpm ") || strings.Contains(lower, "yarn "):
		return "Node-Schritt ausführen"
	case strings.Contains(lower, "docker "):
		return "Docker-Schritt ausführen"
	case strings.Contains(lower, "composer "):
		return "Composer-Schritt ausführen"
	case strings.Contains(lower, "install ") || strings.HasPrefix(lower, "cp "):
		return "Dateien deployen"
	default:
		return "Setup-Kommando ausführen"
	}
}
