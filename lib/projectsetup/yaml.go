package projectsetup

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Manifest struct {
	Version            int
	ProjectVersion     string
	ProjectName        string
	ProjectSlug        string
	ProjectDescription string
	ProjectType        string
	LegacySchema       bool
	Steps              []Step
	Variables          map[string]string
	Defaults           SetupDefaults
	Requirements       SetupRequirements
	Workflows          map[string]Workflow
	Tasks              map[string]Task
}

type SetupDefaults struct {
	Timeout  string
	FailFast bool
}

type SetupRequirements struct {
	Commands         []string
	OptionalCommands []string
}

type Workflow struct {
	Name        string
	Description string
	Tasks       []string
}

type Task struct {
	Name        string
	Description string
	Requires    []string
	Steps       []StepV2
}

type StepV2 struct {
	ID               string
	Name             string
	Operation        string
	Config           map[string]any
	WorkingDirectory string
	Environment      map[string]string
	Timeout          string
	Retries          int
	AllowFailure     bool
	When             *Condition
}

type Condition struct {
	Kind     string
	Value    any
	Children []*Condition
}

type Step struct {
	ID               string
	Name             string
	When             string
	Type             string
	Action           string
	Command          string
	WorkingDirectory string
	Source           string
	Destination      string
	Output           string
	Mode             string
	Requirements     string
	Args             []string
	Detach           bool
	ContinueOnError  bool
	legacyRun        bool
}

func ParseManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	if detectSetupSchemaVersion(data) == 2 {
		return parseManifestV2(path, data)
	}
	return parseManifestV1(path, data)
}

func parseManifestV1(path string, data []byte) (Manifest, error) {
	if structuredLegacyV1(data) {
		return parseStructuredLegacyV1(path, data)
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	m := Manifest{}
	section := ""
	var current *Step
	seenVersion := false
	seenSchemaVersion := false

	for i := 0; i < len(lines); i++ {
		lineNo := i + 1
		raw := strings.TrimRight(lines[i], " \t\r")
		trim := strings.TrimSpace(raw)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		indent, indentErr := yamlIndent(raw)
		if indentErr != nil {
			return m, fmt.Errorf("setup.yaml Zeile %d: %w", lineNo, indentErr)
		}
		if indent == 0 {
			current = nil
			k, v, ok := splitKV(trim)
			if !ok {
				return m, fmt.Errorf("setup.yaml Zeile %d: erwartetes key: value", lineNo)
			}
			switch k {
			case "version", "schemaVersion":
				n, e := strconv.Atoi(unquote(v))
				if e != nil {
					return m, fmt.Errorf("setup.yaml Zeile %d: ungültige %s", lineNo, k)
				}
				if m.Version != 0 && m.Version != n {
					return m, fmt.Errorf("setup.yaml Zeile %d: version und schemaVersion widersprechen sich", lineNo)
				}
				m.Version = n
				if k == "version" {
					seenVersion = true
				} else {
					seenSchemaVersion = true
					m.LegacySchema = true
				}
				section = ""
			case "project":
				if v != "" {
					return m, fmt.Errorf("setup.yaml Zeile %d: project erwartet Unterfelder", lineNo)
				}
				section = "project"
			case "steps":
				if v != "" {
					return m, fmt.Errorf("setup.yaml Zeile %d: steps erwartet Liste", lineNo)
				}
				section = "steps"
			default:
				return m, fmt.Errorf("setup.yaml Zeile %d: unbekanntes Top-Level-Feld %q", lineNo, k)
			}
			continue
		}

		switch section {
		case "project":
			if indent < 2 {
				return m, fmt.Errorf("setup.yaml Zeile %d: project-Feld falsch eingerückt", lineNo)
			}
			k, v, ok := splitKV(trim)
			if !ok {
				return m, fmt.Errorf("setup.yaml Zeile %d: ungültiges project-Feld", lineNo)
			}
			switch k {
			case "name":
				m.ProjectName = unquote(v)
			case "slug":
				m.ProjectSlug = strings.TrimSpace(unquote(v))
				m.LegacySchema = true
			case "description":
				m.ProjectDescription = unquote(v)
				m.LegacySchema = true
			case "type":
				m.ProjectType = strings.TrimSpace(unquote(v))
				m.LegacySchema = true
			default:
				return m, fmt.Errorf("setup.yaml Zeile %d: unbekanntes project-Feld %q", lineNo, k)
			}
		case "steps":
			if strings.HasPrefix(trim, "- ") || trim == "-" {
				m.Steps = append(m.Steps, Step{})
				current = &m.Steps[len(m.Steps)-1]
				rest := strings.TrimSpace(strings.TrimPrefix(trim, "-"))
				if rest == "" {
					continue
				}
				k, v, ok := splitKV(rest)
				if !ok {
					return m, fmt.Errorf("setup.yaml Zeile %d: ungültiger Schritt", lineNo)
				}
				if isBlockScalar(v) {
					return m, fmt.Errorf("setup.yaml Zeile %d: Block-Inhalt muss als eingerücktes Schrittfeld geschrieben werden", lineNo)
				}
				legacy, assignErr := assignStep(current, k, v)
				if assignErr != nil {
					return m, fmt.Errorf("setup.yaml Zeile %d: %w", lineNo, assignErr)
				}
				m.LegacySchema = m.LegacySchema || legacy
				continue
			}
			if current == nil {
				return m, fmt.Errorf("setup.yaml Zeile %d: Schrittfeld ohne Listeneintrag", lineNo)
			}
			k, v, ok := splitKV(trim)
			if !ok {
				return m, fmt.Errorf("setup.yaml Zeile %d: ungültiges Schrittfeld; bei run: | muss der Befehlsblock eingerückt sein", lineNo)
			}
			if isBlockScalar(v) {
				if k != "run" && k != "command" {
					return m, fmt.Errorf("setup.yaml Zeile %d: Block-Skalar ist für %q nicht unterstützt", lineNo, k)
				}
				block, last, blockErr := readBlockScalar(lines, i, indent, k)
				if blockErr != nil {
					return m, blockErr
				}
				v = block
				i = last
			}
			legacy, assignErr := assignStep(current, k, v)
			if assignErr != nil {
				return m, fmt.Errorf("setup.yaml Zeile %d: %w", lineNo, assignErr)
			}
			m.LegacySchema = m.LegacySchema || legacy
		default:
			return m, fmt.Errorf("setup.yaml Zeile %d: eingerückter Inhalt ohne Abschnitt", lineNo)
		}
	}

	if !seenVersion && !seenSchemaVersion {
		return m, fmt.Errorf("setup.yaml version/schemaVersion fehlt")
	}
	if m.Version != 1 {
		return m, fmt.Errorf("setup.yaml version muss 1 sein; erhalten %d", m.Version)
	}
	if len(m.Steps) == 0 {
		return m, fmt.Errorf("setup.yaml enthält keine steps")
	}

	for i := range m.Steps {
		s := &m.Steps[i]
		if s.legacyRun {
			if strings.TrimSpace(s.Command) == "" {
				return m, fmt.Errorf("setup.yaml steps[%d].run fehlt", i)
			}
			if s.Type != "" && s.Type != "command" && s.Type != "shell" {
				return m, fmt.Errorf("setup.yaml steps[%d] mischt legacy run mit type=%q", i, s.Type)
			}
			s.Type = "command"
			if s.When == "" {
				s.When = "always"
			}
			if s.Name == "" {
				s.Name = s.ID
			}
			if s.Name == "" {
				s.Name = fmt.Sprintf("Schritt %d", i+1)
			}
			continue
		}
		if strings.TrimSpace(s.Type) == "" {
			return m, fmt.Errorf("setup.yaml steps[%d].type fehlt", i)
		}
		if s.When == "" {
			s.When = "always"
		}
		if s.Name == "" {
			s.Name = s.ID
		}
		if s.Name == "" {
			s.Name = strings.TrimSpace(s.Type + " " + s.Action)
		}
	}
	return m, nil
}

func yamlIndent(raw string) (int, error) {
	indent := 0
	for _, r := range raw {
		switch r {
		case ' ':
			indent++
		case '\t':
			return 0, fmt.Errorf("Tabs sind nicht erlaubt")
		default:
			return indent, nil
		}
	}
	return indent, nil
}

func isBlockScalar(value string) bool {
	switch strings.TrimSpace(value) {
	case "|", "|-", "|+", ">", ">-", ">+":
		return true
	default:
		return false
	}
}

func readBlockScalar(lines []string, headerIndex, headerIndent int, key string) (string, int, error) {
	start := headerIndex + 1
	last := headerIndex
	minIndent := -1
	for i := start; i < len(lines); i++ {
		raw := strings.TrimRight(lines[i], "\r")
		trim := strings.TrimSpace(raw)
		indent, err := yamlIndent(raw)
		if err != nil {
			return "", last, fmt.Errorf("setup.yaml Zeile %d: %w", i+1, err)
		}
		if trim != "" && indent <= headerIndent {
			break
		}
		last = i
		if trim != "" && (minIndent < 0 || indent < minIndent) {
			minIndent = indent
		}
	}
	if last == headerIndex || minIndent <= headerIndent {
		return "", headerIndex, fmt.Errorf("setup.yaml Zeile %d: %s: | benötigt einen stärker eingerückten Befehlsblock", headerIndex+1, key)
	}

	block := make([]string, 0, last-start+1)
	for i := start; i <= last; i++ {
		raw := strings.TrimRight(lines[i], "\r")
		if strings.TrimSpace(raw) == "" {
			block = append(block, "")
			continue
		}
		if len(raw) < minIndent {
			block = append(block, "")
			continue
		}
		block = append(block, raw[minIndent:])
	}
	return strings.TrimRight(strings.Join(block, "\n"), "\n"), last, nil
}

func splitKV(s string) (string, string, bool) {
	i := strings.Index(s, ":")
	if i <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:]), true
}

func assignStep(s *Step, k, v string) (bool, error) {
	v = unquote(v)
	legacy := false
	switch k {
	case "id":
		s.ID = v
		legacy = true
	case "name":
		s.Name = v
	case "when":
		// Be tolerant of manifests copied through Markdown where ':' may have
		// been escaped as '\:'. Native YAML does not require that escape.
		s.When = strings.ReplaceAll(v, "\\:", ":")
		legacy = true
	case "run":
		s.Command = v
		s.legacyRun = true
		legacy = true
	case "cwd":
		s.WorkingDirectory = v
		legacy = true
	case "allowFailure":
		b, e := strconv.ParseBool(v)
		if e != nil {
			return legacy, fmt.Errorf("allowFailure muss true/false sein")
		}
		s.ContinueOnError = b
		legacy = true
	case "type":
		s.Type = strings.ToLower(v)
	case "action":
		s.Action = strings.ToLower(v)
	case "command":
		s.Command = v
	case "workingDirectory":
		s.WorkingDirectory = v
	case "source":
		s.Source = v
	case "destination":
		s.Destination = v
	case "output":
		s.Output = v
	case "mode":
		s.Mode = v
	case "requirements":
		s.Requirements = v
	case "detach":
		b, e := strconv.ParseBool(v)
		if e != nil {
			return legacy, fmt.Errorf("detach muss true/false sein")
		}
		s.Detach = b
	case "continueOnError":
		b, e := strconv.ParseBool(v)
		if e != nil {
			return legacy, fmt.Errorf("continueOnError muss true/false sein")
		}
		s.ContinueOnError = b
	case "args":
		var a []string
		if err := json.Unmarshal([]byte(v), &a); err != nil {
			return legacy, fmt.Errorf("args muss eine JSON-kompatible Inline-Liste sein, z. B. [\"-trimpath\"]")
		}
		s.Args = a
	default:
		return legacy, fmt.Errorf("unbekanntes Schrittfeld %q", k)
	}
	return legacy, nil
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}
