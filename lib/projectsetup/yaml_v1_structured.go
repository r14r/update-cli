package projectsetup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// structuredLegacyV1 identifies the configuration-oriented schemaVersion: 1
// format that predates the simple `steps:` manifest. It stores project/build
// metadata and refers to built-in setup step IDs under setup.steps.
func structuredLegacyV1(data []byte) bool {
	root, err := parseSimpleYAML(data)
	if err != nil || root.kind != yamlMap {
		return false
	}
	if n := root.m["schemaVersion"]; n == nil {
		return false
	} else if v, err := nodeInt(n); err != nil || v != 1 {
		return false
	}
	if n := root.m["version"]; n != nil && n.kind == yamlMap {
		return true
	}
	for _, key := range []string{"build", "runtime", "go", "setup", "commands"} {
		if root.m[key] != nil {
			return true
		}
	}
	return false
}

func parseStructuredLegacyV1(path string, data []byte) (Manifest, error) {
	root, err := parseSimpleYAML(data)
	if err != nil {
		return Manifest{}, err
	}
	if root.kind != yamlMap {
		return Manifest{}, fmt.Errorf("update-cli.yaml: Top-Level muss eine Map sein")
	}

	allowedTop := map[string]bool{
		"schemaVersion": true,
		"project":       true,
		"version":       true,
		"build":         true,
		"runtime":       true,
		"go":            true,
		"setup":         true,
		"commands":      true,
	}
	for key, node := range root.m {
		if !allowedTop[key] {
			return Manifest{}, lineError(node, fmt.Sprintf("unbekanntes Top-Level-Feld %q", key))
		}
	}

	schema, err := nodeInt(root.m["schemaVersion"])
	if err != nil || schema != 1 {
		if err != nil {
			return Manifest{}, lineError(root.m["schemaVersion"], "schemaVersion muss 1 sein")
		}
		return Manifest{}, lineError(root.m["schemaVersion"], fmt.Sprintf("schemaVersion muss 1 sein; erhalten %d", schema))
	}

	m := Manifest{Version: 1, LegacySchema: true}
	if n := root.m["project"]; n != nil {
		if n.kind != yamlMap {
			return m, lineError(n, "project muss eine Map sein")
		}
		for key, value := range n.m {
			switch key {
			case "name":
				m.ProjectName, err = nodeString(value)
			case "slug":
				m.ProjectSlug, err = nodeString(value)
			case "description":
				m.ProjectDescription, err = nodeString(value)
			case "type":
				m.ProjectType, err = nodeString(value)
			default:
				return m, lineError(value, fmt.Sprintf("unbekanntes project-Feld %q", key))
			}
			if err != nil {
				return m, lineError(value, err.Error())
			}
		}
	}

	versionFile := "VERSION"
	versionRequired := false
	versionPattern := ""
	if n := root.m["version"]; n != nil {
		if n.kind != yamlMap {
			return m, lineError(n, "version muss in diesem schemaVersion-1-Format eine Map sein")
		}
		for key, value := range n.m {
			switch key {
			case "file":
				versionFile, err = nodeString(value)
			case "required":
				versionRequired, err = nodeBool(value)
			case "pattern":
				versionPattern, err = nodeString(value)
			default:
				return m, lineError(value, fmt.Sprintf("unbekanntes version-Feld %q", key))
			}
			if err != nil {
				return m, lineError(value, err.Error())
			}
		}
	}

	rootDir := filepath.Dir(path)
	if strings.TrimSpace(versionFile) == "" {
		versionFile = "VERSION"
	}
	versionPath := filepath.Join(rootDir, filepath.Clean(versionFile))
	if versionRequired {
		data, readErr := os.ReadFile(versionPath)
		if readErr != nil {
			return m, fmt.Errorf("update-cli.yaml: erforderliche Versionsdatei fehlt: %s", versionFile)
		}
		m.ProjectVersion = strings.TrimSpace(string(data))
		if versionPattern != "" {
			re, compileErr := regexp.Compile(versionPattern)
			if compileErr != nil {
				return m, fmt.Errorf("update-cli.yaml: ungültiges version.pattern: %w", compileErr)
			}
			if !re.MatchString(m.ProjectVersion) {
				return m, fmt.Errorf("update-cli.yaml: Version %q aus %s entspricht nicht pattern %q", m.ProjectVersion, versionFile, versionPattern)
			}
		}
	} else if data, readErr := os.ReadFile(versionPath); readErr == nil {
		m.ProjectVersion = strings.TrimSpace(string(data))
	}

	buildConfigFile := ""
	distDir := "bin"
	binaryName := ""
	if n := root.m["build"]; n != nil {
		if n.kind != yamlMap {
			return m, lineError(n, "build muss eine Map sein")
		}
		for key, value := range n.m {
			switch key {
			case "configFile":
				buildConfigFile, err = nodeString(value)
			case "distDir":
				distDir, err = nodeString(value)
			case "binaryName":
				binaryName, err = nodeString(value)
			default:
				return m, lineError(value, fmt.Sprintf("unbekanntes build-Feld %q", key))
			}
			if err != nil {
				return m, lineError(value, err.Error())
			}
		}
	}
	if binaryName == "" {
		binaryName = m.ProjectSlug
	}
	if binaryName == "" {
		binaryName = strings.ToLower(strings.ReplaceAll(m.ProjectName, " ", "-"))
	}

	requiredCommands := []string{}
	optionalCommands := []string{}
	if n := root.m["runtime"]; n != nil {
		if n.kind != yamlMap {
			return m, lineError(n, "runtime muss eine Map sein")
		}
		for key, value := range n.m {
			switch key {
			case "requiredCommands":
				requiredCommands, err = nodeStringList(value)
			case "optionalCommands":
				optionalCommands, err = nodeStringList(value)
			default:
				return m, lineError(value, fmt.Sprintf("unbekanntes runtime-Feld %q", key))
			}
			if err != nil {
				return m, lineError(value, err.Error())
			}
		}
	}
	for _, command := range requiredCommands {
		if _, lookErr := exec.LookPath(command); lookErr != nil {
			return m, fmt.Errorf("update-cli.yaml: erforderliches Kommando fehlt: %s", command)
		}
	}
	_ = optionalCommands // optional tools are informational in the legacy engine.

	goPackage := "./..."
	goBuildPackage := "."
	goLDFlags := ""
	if n := root.m["go"]; n != nil {
		if n.kind != yamlMap {
			return m, lineError(n, "go muss eine Map sein")
		}
		for key, value := range n.m {
			switch key {
			case "package":
				goPackage, err = nodeString(value)
			case "buildPackage":
				goBuildPackage, err = nodeString(value)
			case "ldflagsTemplate":
				goLDFlags, err = nodeString(value)
			default:
				return m, lineError(value, fmt.Sprintf("unbekanntes go-Feld %q", key))
			}
			if err != nil {
				return m, lineError(value, err.Error())
			}
		}
		if m.ProjectType == "" {
			m.ProjectType = "go"
		}
	}

	setupIDs := []string{}
	if n := root.m["setup"]; n != nil {
		if n.kind != yamlMap {
			return m, lineError(n, "setup muss eine Map sein")
		}
		for key, value := range n.m {
			switch key {
			case "steps":
				setupIDs, err = nodeStringList(value)
			default:
				return m, lineError(value, fmt.Sprintf("unbekanntes setup-Feld %q", key))
			}
			if err != nil {
				return m, lineError(value, err.Error())
			}
		}
	}
	if len(setupIDs) == 0 {
		return m, fmt.Errorf("update-cli.yaml: setup.steps enthält keine Schritte")
	}

	preCommands, justCommands, customCommands, postCommands := []string{}, []string{}, []string{}, []string{}
	if n := root.m["commands"]; n != nil {
		if n.kind != yamlMap {
			return m, lineError(n, "commands muss eine Map sein")
		}
		for key, value := range n.m {
			switch key {
			case "pre":
				preCommands, err = nodeStringList(value)
			case "just":
				justCommands, err = nodeStringList(value)
			case "custom":
				customCommands, err = nodeStringList(value)
			case "post":
				postCommands, err = nodeStringList(value)
			default:
				return m, lineError(value, fmt.Sprintf("unbekanntes commands-Feld %q", key))
			}
			if err != nil {
				return m, lineError(value, err.Error())
			}
		}
	}

	binaryPath := filepath.ToSlash(filepath.Join(distDir, binaryName))
	versionRead := fmt.Sprintf("VERSION_VALUE=\"$(tr -d '[:space:]' < %s)\"", shellSingleQuote(versionFile))
	buildCommand := structuredGoBuildCommand(versionFile, goBuildPackage, goLDFlags, binaryPath)

	for _, id := range setupIDs {
		step := Step{ID: id, When: "always", Type: "command", legacyRun: true}
		switch id {
		case "pre-commands":
			step.Name = "Vorbereitende Kommandos"
			step.Command = groupedLegacyCommands(preCommands)
		case "go-mod-download":
			step.Name = "Go-Module laden"
			step.When = "file:go.mod"
			step.Command = "go mod download"
		case "build-config-validate":
			step.Name = "Build-Konfiguration prüfen"
			if strings.TrimSpace(buildConfigFile) == "" {
				step.Command = "true"
			} else {
				step.When = "file:" + buildConfigFile
				step.Command = "go run ./cmd/buildconfig --validate"
			}
		case "go-vet":
			step.Name = "Go-Quellcode prüfen"
			step.When = "file:go.mod"
			step.Command = "go vet " + goPackage
		case "go-test":
			step.Name = "Go-Tests ausführen"
			step.When = "file:go.mod"
			step.Command = "go test " + goPackage
		case "go-build":
			step.Name = "Go-Binary bauen"
			step.When = "file:go.mod"
			step.Command = buildCommand
		case "binary-version-check":
			step.Name = "Binary-Version prüfen"
			step.When = "file:" + binaryPath
			step.Command = versionRead + "; OUTPUT=\"$(./" + binaryPath + " --version)\"; printf '%s\\n' \"$OUTPUT\"; printf '%s\\n' \"$OUTPUT\" | grep -Fq \"$VERSION_VALUE\""
		case "just-commands":
			step.Name = "Just-Kommandos ausführen"
			cmds := make([]string, 0, len(justCommands))
			for _, cmd := range justCommands {
				cmds = append(cmds, "just "+cmd)
			}
			step.Command = groupedLegacyCommands(cmds)
		case "custom-commands":
			step.Name = "Projektkommandos ausführen"
			step.Command = groupedLegacyCommands(customCommands)
		case "post-commands":
			step.Name = "Abschließende Kommandos"
			step.Command = groupedLegacyCommands(postCommands)
		case "docker-down":
			step.Name = "Docker-Container stoppen"
			step.When = "compose"
			step.Command = "docker compose down --remove-orphans"
		case "docker-up":
			step.Name = "Docker-Container starten"
			step.When = "compose"
			step.Command = "docker compose up -d --remove-orphans"
		case "composer-install":
			step.Name = "Composer-Abhängigkeiten installieren"
			step.When = "file:composer.json"
			step.Command = "composer install --no-interaction --prefer-dist --optimize-autoloader"
		case "python-venv":
			step.Name = "Python-Umgebung erstellen"
			step.Command = "python3 -m venv .venv"
		case "pip-install":
			step.Name = "Python-Abhängigkeiten installieren"
			step.When = "file:requirements.txt"
			step.Command = ".venv/bin/python -m pip install --upgrade pip && .venv/bin/python -m pip install -r requirements.txt"
		case "npm-install":
			step.Name = "Node-Abhängigkeiten installieren"
			step.When = "file:package.json"
			step.Command = "if [ -f package-lock.json ]; then npm ci --no-audit --no-fund; else npm install --no-audit --no-fund; fi"
		case "npm-build":
			step.Name = "Frontend bauen"
			step.When = "file:package.json"
			step.Command = "npm run build"
		default:
			return m, fmt.Errorf("update-cli.yaml: unbekannter Legacy-Setup-Schritt %q", id)
		}
		if strings.TrimSpace(step.Command) == "" {
			step.Command = "true"
		}
		m.Steps = append(m.Steps, step)
	}

	return m, nil
}

func groupedLegacyCommands(commands []string) string {
	filtered := make([]string, 0, len(commands))
	for _, command := range commands {
		if strings.TrimSpace(command) != "" {
			filtered = append(filtered, command)
		}
	}
	if len(filtered) == 0 {
		return "true"
	}
	return strings.Join(filtered, "\n")
}

func structuredGoBuildCommand(versionFile, buildPackage, ldflagsTemplate, binaryPath string) string {
	if strings.TrimSpace(buildPackage) == "" {
		buildPackage = "."
	}
	ldflags := strings.TrimSpace(ldflagsTemplate)
	ldflags = strings.ReplaceAll(ldflags, "{{VERSION}}", "${VERSION_VALUE}")
	ldflags = strings.ReplaceAll(ldflags, "{{COMMIT}}", "${COMMIT_VALUE}")
	ldflags = strings.ReplaceAll(ldflags, "{{BUILD_DATE}}", "${BUILD_DATE_VALUE}")
	ldflags = strings.ReplaceAll(ldflags, "\\", "\\\\")
	ldflags = strings.ReplaceAll(ldflags, "\"", "\\\"")

	parts := []string{
		fmt.Sprintf("VERSION_VALUE=\"$(tr -d '[:space:]' < %s)\"", shellSingleQuote(versionFile)),
		"COMMIT_VALUE=\"$(git rev-parse --short HEAD 2>/dev/null || printf local)\"",
		"BUILD_DATE_VALUE=\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"",
		"mkdir -p " + shellSingleQuote(filepath.Dir(binaryPath)),
	}
	build := "go build -trimpath"
	if ldflags != "" {
		build += " -ldflags \"" + ldflags + "\""
	}
	build += " -o " + shellSingleQuote(binaryPath) + " " + shellSingleQuote(buildPackage)
	parts = append(parts, build, "chmod 0755 "+shellSingleQuote(binaryPath))
	return strings.Join(parts, "\n")
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
