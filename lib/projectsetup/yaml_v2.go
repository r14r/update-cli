package projectsetup

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type simpleYAMLKind int

const (
	yamlScalar simpleYAMLKind = iota
	yamlMap
	yamlList
)

type simpleYAMLNode struct {
	kind   simpleYAMLKind
	scalar string
	m      map[string]*simpleYAMLNode
	list   []*simpleYAMLNode
	line   int
}

type yamlLogicalLine struct {
	raw    string
	trim   string
	indent int
	line   int
}

func detectSetupSchemaVersion(data []byte) int {
	// schemaVersion is authoritative in the current manifest format. A plain
	// top-level version field may describe the project/application version, so
	// it must never win merely because it appears earlier in the file.
	legacyVersion := 0
	for _, raw := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if len(raw) > 0 && (raw[0] == ' ' || raw[0] == '\t') {
			continue
		}
		trim := strings.TrimSpace(raw)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		k, v, ok := splitKV(trim)
		if !ok {
			continue
		}
		switch k {
		case "schemaVersion":
			n, _ := strconv.Atoi(unquote(v))
			return n
		case "version":
			// Keep the historical `version: 1` schema alias as a fallback only.
			// Non-integer values are application versions, not schema selectors.
			if n, err := strconv.Atoi(unquote(v)); err == nil {
				legacyVersion = n
			}
		}
	}
	return legacyVersion
}

func parseManifestV2(path string, data []byte) (Manifest, error) {
	root, err := parseSimpleYAML(data)
	if err != nil {
		return Manifest{}, err
	}
	if root.kind != yamlMap {
		return Manifest{}, fmt.Errorf("update-cli.yaml: Top-Level muss eine Map sein")
	}
	allowedTop := map[string]bool{
		"schemaVersion": true, "version": true, "project": true, "defaults": true, "variables": true,
		"requirements": true, "workflows": true, "tasks": true, "run": true, "update": true,
	}
	for key, node := range root.m {
		if !allowedTop[key] {
			return Manifest{}, fmt.Errorf("update-cli.yaml Zeile %d: unbekanntes Top-Level-Feld %q", node.line, key)
		}
	}
	version, err := nodeInt(root.m["schemaVersion"])
	if err != nil || version != 2 {
		if err != nil {
			return Manifest{}, fmt.Errorf("update-cli.yaml schemaVersion ungültig: %w", err)
		}
		return Manifest{}, fmt.Errorf("update-cli.yaml schemaVersion muss 2 sein; erhalten %d", version)
	}
	m := Manifest{
		Version:      2,
		Variables:    map[string]string{},
		Workflows:    map[string]Workflow{},
		Tasks:        map[string]Task{},
		Defaults:     SetupDefaults{FailFast: true},
		LegacySchema: false,
	}
	if n := root.m["version"]; n != nil {
		m.ProjectVersion, err = nodeString(n)
		if err != nil {
			return m, lineError(n, "version muss ein skalarer Wert sein")
		}
	}
	if n := root.m["project"]; n != nil {
		if n.kind != yamlMap {
			return m, lineError(n, "project muss eine Map sein")
		}
		for k, v := range n.m {
			switch k {
			case "name":
				m.ProjectName, err = nodeString(v)
			case "slug":
				m.ProjectSlug, err = nodeString(v)
			case "description":
				m.ProjectDescription, err = nodeString(v)
			case "type":
				m.ProjectType, err = nodeString(v)
			default:
				return m, lineError(v, fmt.Sprintf("unbekanntes project-Feld %q", k))
			}
			if err != nil {
				return m, lineError(v, err.Error())
			}
		}
	}
	if n := root.m["defaults"]; n != nil {
		if n.kind != yamlMap {
			return m, lineError(n, "defaults muss eine Map sein")
		}
		for k, v := range n.m {
			switch k {
			case "timeout":
				m.Defaults.Timeout, err = nodeString(v)
			case "failFast":
				m.Defaults.FailFast, err = nodeBool(v)
			default:
				return m, lineError(v, fmt.Sprintf("unbekanntes defaults-Feld %q", k))
			}
			if err != nil {
				return m, lineError(v, err.Error())
			}
		}
	}
	if n := root.m["variables"]; n != nil {
		if n.kind != yamlMap {
			return m, lineError(n, "variables muss eine Map sein")
		}
		for k, v := range n.m {
			value, e := nodeString(v)
			if e != nil {
				return m, lineError(v, "Variable muss ein skalarer Wert sein")
			}
			m.Variables[k] = value
		}
	}
	if n := root.m["update"]; n != nil {
		if n.kind != yamlMap {
			return m, lineError(n, "update muss eine Map sein")
		}
		m.Update.Configured = true
		for k, v := range n.m {
			switch k {
			case "mode":
				m.Update.Mode, err = nodeString(v)
				m.Update.Mode = strings.ToLower(strings.TrimSpace(m.Update.Mode))
			case "source":
				if v.kind != yamlMap {
					return m, lineError(v, "update.source muss eine Map sein")
				}
				for sk, sv := range v.m {
					var value string
					value, err = nodeString(sv)
					if err != nil {
						return m, lineError(sv, fmt.Sprintf("update.source.%s muss skalar sein", sk))
					}
					value = strings.TrimSpace(value)
					switch sk {
					case "type":
						m.Update.Source.Type = strings.ToLower(value)
					case "folder":
						m.Update.Source.Folder = value
					case "url":
						m.Update.Source.URL = value
					case "repository":
						m.Update.Source.Repository = value
					case "ref":
						m.Update.Source.Ref = value
					case "commit":
						m.Update.Source.Commit = value
					case "version":
						m.Update.Source.Version = value
					case "sha256":
						m.Update.Source.SHA256 = value
					default:
						return m, lineError(sv, fmt.Sprintf("unbekanntes update.source-Feld %q", sk))
					}
				}
			default:
				return m, lineError(v, fmt.Sprintf("unbekanntes update-Feld %q", k))
			}
			if err != nil {
				return m, lineError(v, err.Error())
			}
		}
		if m.Update.Source.Type == "" {
			return m, lineError(n, "update.source.type fehlt")
		}
		if m.Update.Mode == "" {
			if m.Update.Source.Type == "repository" {
				m.Update.Mode = "pull"
			} else {
				m.Update.Mode = "update"
			}
		}
		switch m.Update.Mode {
		case "update":
			if m.Update.Source.Type != "download" && m.Update.Source.Type != "url" {
				return m, lineError(n, "update.mode update benötigt update.source.type download oder url")
			}
		case "pull":
			if m.Update.Source.Type != "repository" {
				return m, lineError(n, "update.mode pull benötigt update.source.type repository")
			}
		default:
			return m, lineError(n, "update.mode unterstützt nur update oder pull")
		}
		switch m.Update.Source.Type {
		case "download":
			if m.Update.Source.Folder == "" {
				return m, lineError(n, "update.source.folder fehlt für download")
			}
		case "url":
			if m.Update.Source.URL == "" {
				return m, lineError(n, "update.source.url fehlt für url")
			}
		case "repository":
			if m.Update.Source.Repository == "" {
				return m, lineError(n, "update.source.repository fehlt für repository")
			}
		default:
			return m, lineError(n, "update.source.type unterstützt nur download, url oder repository")
		}
	}
	if n := root.m["run"]; n != nil {
		m.Run.Environment = map[string]string{}
		switch n.kind {
		case yamlScalar:
			m.Run.Command, err = nodeString(n)
			if err != nil {
				return m, lineError(n, "run muss ein Kommando oder eine Map sein")
			}
		case yamlMap:
			for k, v := range n.m {
				switch k {
				case "description":
					m.Run.Description, err = nodeString(v)
				case "command":
					m.Run.Command, err = nodeString(v)
				case "cwd", "workingDirectory":
					m.Run.WorkingDirectory, err = nodeString(v)
				case "env":
					if v.kind != yamlMap {
						return m, lineError(v, "run.env muss eine Map sein")
					}
					for ek, ev := range v.m {
						value, e := nodeString(ev)
						if e != nil {
							return m, lineError(ev, "run.env-Wert muss skalar sein")
						}
						m.Run.Environment[ek] = value
					}
				case "steps":
					if v.kind != yamlList {
						return m, lineError(v, "run.steps muss eine Liste sein")
					}
					for i, item := range v.list {
						step, e := parseV2Step(item)
						if e != nil {
							return m, fmt.Errorf("update-cli.yaml run.steps[%d]: %w", i, e)
						}
						m.Run.Steps = append(m.Run.Steps, step)
					}
				default:
					return m, lineError(v, fmt.Sprintf("unbekanntes run-Feld %q", k))
				}
				if err != nil {
					return m, lineError(v, err.Error())
				}
			}
		default:
			return m, lineError(n, "run muss ein Kommando oder eine Map sein")
		}
		if strings.TrimSpace(m.Run.Command) != "" && len(m.Run.Steps) > 0 {
			return m, lineError(n, "run.command und run.steps schließen sich aus")
		}
		if strings.TrimSpace(m.Run.Command) == "" && len(m.Run.Steps) == 0 {
			return m, lineError(n, "run benötigt command oder steps")
		}
	}
	if n := root.m["requirements"]; n != nil {
		if n.kind != yamlMap {
			return m, lineError(n, "requirements muss eine Map sein")
		}
		for k, v := range n.m {
			switch k {
			case "commands":
				m.Requirements.Commands, err = nodeStringList(v)
			case "optionalCommands":
				m.Requirements.OptionalCommands, err = nodeStringList(v)
			default:
				return m, lineError(v, fmt.Sprintf("unbekanntes requirements-Feld %q", k))
			}
			if err != nil {
				return m, lineError(v, err.Error())
			}
		}
	}
	workflowsNode := root.m["workflows"]
	if workflowsNode != nil {
		if workflowsNode.kind != yamlMap {
			return m, lineError(workflowsNode, "workflows muss eine Map sein")
		}
		for name, n := range workflowsNode.m {
			if n.kind != yamlMap {
				return m, lineError(n, fmt.Sprintf("workflow %q muss eine Map sein", name))
			}
			w := Workflow{Name: name}
			for k, v := range n.m {
				switch k {
				case "description":
					w.Description, err = nodeString(v)
				case "tasks":
					w.Tasks, err = nodeStringList(v)
				default:
					return m, lineError(v, fmt.Sprintf("unbekanntes workflow-Feld %q", k))
				}
				if err != nil {
					return m, lineError(v, err.Error())
				}
			}
			if len(w.Tasks) == 0 {
				return m, lineError(n, fmt.Sprintf("workflow %q enthält keine tasks", name))
			}
			m.Workflows[name] = w
		}
	}
	tasksNode := root.m["tasks"]
	if tasksNode != nil {
		if tasksNode.kind != yamlMap {
			return m, lineError(tasksNode, "tasks muss eine Map sein")
		}
		for name, n := range tasksNode.m {
			task, e := parseV2Task(name, n)
			if e != nil {
				return m, e
			}
			m.Tasks[name] = task
		}
	}
	if len(m.Tasks) == 0 && strings.TrimSpace(m.Run.Command) == "" && len(m.Run.Steps) == 0 {
		return m, fmt.Errorf("update-cli.yaml benötigt mindestens tasks oder run")
	}
	for name, w := range m.Workflows {
		for _, task := range w.Tasks {
			if _, ok := m.Tasks[task]; !ok {
				return m, fmt.Errorf("update-cli.yaml workflow %q verweist auf unbekannten task %q", name, task)
			}
		}
	}
	for name, task := range m.Tasks {
		for _, dep := range task.Requires {
			if _, ok := m.Tasks[dep]; !ok {
				return m, fmt.Errorf("update-cli.yaml task %q requires unbekannten task %q", name, dep)
			}
		}
	}
	return m, nil
}

func parseV2Task(name string, n *simpleYAMLNode) (Task, error) {
	if n.kind != yamlMap {
		return Task{}, lineError(n, fmt.Sprintf("task %q muss eine Map sein", name))
	}
	t := Task{Name: name}
	for k, v := range n.m {
		switch k {
		case "description":
			s, err := nodeString(v)
			if err != nil {
				return t, lineError(v, err.Error())
			}
			t.Description = s
		case "requires":
			list, err := nodeStringList(v)
			if err != nil {
				return t, lineError(v, err.Error())
			}
			t.Requires = list
		case "steps":
			if v.kind != yamlList {
				return t, lineError(v, "steps muss eine Liste sein")
			}
			for i, item := range v.list {
				step, err := parseV2Step(item)
				if err != nil {
					return t, fmt.Errorf("update-cli.yaml task %q steps[%d]: %w", name, i, err)
				}
				t.Steps = append(t.Steps, step)
			}
		default:
			return t, lineError(v, fmt.Sprintf("unbekanntes task-Feld %q", k))
		}
	}
	if len(t.Steps) == 0 && len(t.Requires) == 0 {
		return t, lineError(n, fmt.Sprintf("task %q enthält weder requires noch steps", name))
	}
	return t, nil
}

var v2OperationKeys = map[string]bool{
	"command": true, "shell": true, "mkdir": true, "copy": true, "move": true,
	"remove": true, "chmod": true, "symlink": true, "touch": true, "write": true,
	"assert": true, "pythonVenv": true, "pip": true, "npm": true, "pnpm": true,
	"yarn": true, "composer": true, "artisan": true, "dockerCompose": true, "go": true,
	"httpCheck": true, "download": true, "extract": true, "deploy": true,
}

func parseV2Step(n *simpleYAMLNode) (StepV2, error) {
	if n.kind != yamlMap {
		return StepV2{}, lineError(n, "Schritt muss eine Map sein")
	}
	s := StepV2{Config: map[string]any{}, Environment: map[string]string{}}
	operationCount := 0
	for k, v := range n.m {
		if v2OperationKeys[k] {
			operationCount++
			s.Operation = k
			s.Config = yamlNodeToMap(v)
			if v.kind == yamlScalar {
				s.Config = map[string]any{"value": v.scalar}
			}
			continue
		}
		switch k {
		case "id":
			value, err := nodeString(v)
			if err != nil {
				return s, lineError(v, err.Error())
			}
			s.ID = value
		case "name":
			value, err := nodeString(v)
			if err != nil {
				return s, lineError(v, err.Error())
			}
			s.Name = value
		case "cwd":
			value, err := nodeString(v)
			if err != nil {
				return s, lineError(v, err.Error())
			}
			s.WorkingDirectory = value
		case "env":
			if v.kind != yamlMap {
				return s, lineError(v, "env muss eine Map sein")
			}
			for ek, ev := range v.m {
				value, err := nodeString(ev)
				if err != nil {
					return s, lineError(ev, err.Error())
				}
				s.Environment[ek] = value
			}
		case "timeout":
			value, err := nodeString(v)
			if err != nil {
				return s, lineError(v, err.Error())
			}
			s.Timeout = value
		case "retries":
			value, err := nodeInt(v)
			if err != nil {
				return s, lineError(v, err.Error())
			}
			if value < 0 {
				return s, lineError(v, "retries darf nicht negativ sein")
			}
			s.Retries = value
		case "allowFailure", "continueOnError":
			value, err := nodeBool(v)
			if err != nil {
				return s, lineError(v, err.Error())
			}
			s.AllowFailure = value
		case "when":
			cond, err := parseCondition(v)
			if err != nil {
				return s, err
			}
			s.When = cond
		default:
			return s, lineError(v, fmt.Sprintf("unbekanntes Schrittfeld %q", k))
		}
	}
	if operationCount != 1 {
		return s, lineError(n, fmt.Sprintf("Schritt benötigt genau eine Operation; gefunden %d", operationCount))
	}
	if s.Name == "" {
		s.Name = s.ID
	}
	if s.Name == "" {
		s.Name = s.Operation
	}
	return s, nil
}

func parseCondition(n *simpleYAMLNode) (*Condition, error) {
	if n == nil {
		return nil, nil
	}
	if n.kind == yamlScalar {
		v := strings.TrimSpace(n.scalar)
		if v == "" || v == "always" {
			return nil, nil
		}
		parts := strings.SplitN(strings.ReplaceAll(v, "\\:", ":"), ":", 2)
		if len(parts) != 2 {
			return nil, lineError(n, "when muss eine strukturierte Bedingung oder kind:value sein")
		}
		return &Condition{Kind: parts[0], Value: parts[1]}, nil
	}
	if n.kind != yamlMap || len(n.m) != 1 {
		return nil, lineError(n, "when muss genau eine Bedingung enthalten")
	}
	for k, v := range n.m {
		switch k {
		case "all", "any":
			if v.kind != yamlList {
				return nil, lineError(v, k+" muss eine Liste sein")
			}
			c := &Condition{Kind: k}
			for _, item := range v.list {
				child, err := parseCondition(item)
				if err != nil {
					return nil, err
				}
				c.Children = append(c.Children, child)
			}
			return c, nil
		case "not":
			child, err := parseCondition(v)
			if err != nil {
				return nil, err
			}
			return &Condition{Kind: "not", Children: []*Condition{child}}, nil
		default:
			return &Condition{Kind: k, Value: yamlNodeToAny(v)}, nil
		}
	}
	return nil, lineError(n, "ungültige when-Bedingung")
}

func parseSimpleYAML(data []byte) (*simpleYAMLNode, error) {
	rawLines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	lines := make([]yamlLogicalLine, 0, len(rawLines))
	for i := 0; i < len(rawLines); i++ {
		raw := strings.TrimRight(rawLines[i], " \t\r")
		trim := strings.TrimSpace(raw)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		indent, err := yamlIndent(raw)
		if err != nil {
			return nil, fmt.Errorf("update-cli.yaml Zeile %d: %w", i+1, err)
		}
		lines = append(lines, yamlLogicalLine{raw: raw, trim: trim, indent: indent, line: i + 1})
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("update-cli.yaml ist leer")
	}
	idx := 0
	node, err := parseSimpleBlock(lines, &idx, lines[0].indent)
	if err != nil {
		return nil, err
	}
	if idx != len(lines) {
		return nil, fmt.Errorf("update-cli.yaml Zeile %d: Inhalt konnte nicht zugeordnet werden", lines[idx].line)
	}
	return node, nil
}

func parseSimpleBlock(lines []yamlLogicalLine, idx *int, indent int) (*simpleYAMLNode, error) {
	if *idx >= len(lines) {
		return &simpleYAMLNode{kind: yamlMap, m: map[string]*simpleYAMLNode{}}, nil
	}
	if lines[*idx].indent < indent {
		return nil, fmt.Errorf("update-cli.yaml Zeile %d: unerwartete Einrückung", lines[*idx].line)
	}
	if strings.HasPrefix(lines[*idx].trim, "-") {
		return parseSimpleList(lines, idx, indent)
	}
	return parseSimpleMap(lines, idx, indent)
}

func parseSimpleMap(lines []yamlLogicalLine, idx *int, indent int) (*simpleYAMLNode, error) {
	out := &simpleYAMLNode{kind: yamlMap, m: map[string]*simpleYAMLNode{}, line: lines[*idx].line}
	for *idx < len(lines) {
		l := lines[*idx]
		if l.indent < indent {
			break
		}
		if l.indent > indent {
			return nil, fmt.Errorf("update-cli.yaml Zeile %d: unerwartete Einrückung", l.line)
		}
		if strings.HasPrefix(l.trim, "-") {
			break
		}
		k, v, ok := splitKV(l.trim)
		if !ok {
			return nil, fmt.Errorf("update-cli.yaml Zeile %d: erwartetes key: value", l.line)
		}
		if _, exists := out.m[k]; exists {
			return nil, fmt.Errorf("update-cli.yaml Zeile %d: doppeltes Feld %q", l.line, k)
		}
		(*idx)++
		if isBlockScalar(v) {
			content, err := collectLogicalBlockScalar(lines, idx, indent, l.line)
			if err != nil {
				return nil, err
			}
			out.m[k] = &simpleYAMLNode{kind: yamlScalar, scalar: content, line: l.line}
			continue
		}
		if strings.TrimSpace(v) != "" {
			out.m[k] = scalarOrInlineNode(v, l.line)
			continue
		}
		if *idx >= len(lines) || lines[*idx].indent <= indent {
			out.m[k] = &simpleYAMLNode{kind: yamlMap, m: map[string]*simpleYAMLNode{}, line: l.line}
			continue
		}
		childIndent := lines[*idx].indent
		child, err := parseSimpleBlock(lines, idx, childIndent)
		if err != nil {
			return nil, err
		}
		out.m[k] = child
	}
	return out, nil
}

func parseSimpleList(lines []yamlLogicalLine, idx *int, indent int) (*simpleYAMLNode, error) {
	out := &simpleYAMLNode{kind: yamlList, line: lines[*idx].line}
	for *idx < len(lines) {
		l := lines[*idx]
		if l.indent < indent {
			break
		}
		if l.indent != indent || !strings.HasPrefix(l.trim, "-") {
			break
		}
		rest := strings.TrimSpace(strings.TrimPrefix(l.trim, "-"))
		(*idx)++
		if isBlockScalar(rest) {
			content, err := collectLogicalBlockScalar(lines, idx, indent, l.line)
			if err != nil {
				return nil, err
			}
			out.list = append(out.list, &simpleYAMLNode{kind: yamlScalar, scalar: content, line: l.line})
			continue
		}
		if rest == "" {
			if *idx >= len(lines) || lines[*idx].indent <= indent {
				return nil, fmt.Errorf("update-cli.yaml Zeile %d: leerer Listeneintrag", l.line)
			}
			child, err := parseSimpleBlock(lines, idx, lines[*idx].indent)
			if err != nil {
				return nil, err
			}
			out.list = append(out.list, child)
			continue
		}
		if k, v, ok := splitKV(rest); ok {
			item := &simpleYAMLNode{kind: yamlMap, m: map[string]*simpleYAMLNode{}, line: l.line}
			if isBlockScalar(v) {
				content, err := collectLogicalBlockScalar(lines, idx, indent, l.line)
				if err != nil {
					return nil, err
				}
				item.m[k] = &simpleYAMLNode{kind: yamlScalar, scalar: content, line: l.line}
			} else if strings.TrimSpace(v) != "" {
				item.m[k] = scalarOrInlineNode(v, l.line)
			} else if *idx < len(lines) && lines[*idx].indent > indent {
				child, err := parseSimpleBlock(lines, idx, lines[*idx].indent)
				if err != nil {
					return nil, err
				}
				item.m[k] = child
			} else {
				item.m[k] = &simpleYAMLNode{kind: yamlMap, m: map[string]*simpleYAMLNode{}, line: l.line}
			}
			if *idx < len(lines) && lines[*idx].indent > indent {
				extraIndent := lines[*idx].indent
				extra, err := parseSimpleMap(lines, idx, extraIndent)
				if err != nil {
					return nil, err
				}
				for ek, ev := range extra.m {
					if _, exists := item.m[ek]; exists {
						return nil, fmt.Errorf("update-cli.yaml Zeile %d: doppeltes Feld %q", ev.line, ek)
					}
					item.m[ek] = ev
				}
			}
			out.list = append(out.list, item)
			continue
		}
		out.list = append(out.list, scalarOrInlineNode(rest, l.line))
	}
	return out, nil
}

func collectLogicalBlockScalar(lines []yamlLogicalLine, idx *int, headerIndent, headerLine int) (string, error) {
	if *idx >= len(lines) || lines[*idx].indent <= headerIndent {
		return "", fmt.Errorf("update-cli.yaml Zeile %d: | benötigt einen stärker eingerückten Befehlsblock", headerLine)
	}
	minIndent := lines[*idx].indent
	parts := []string{}
	for *idx < len(lines) && lines[*idx].indent > headerIndent {
		l := lines[*idx]
		text := l.raw
		if len(text) >= minIndent {
			text = text[minIndent:]
		}
		parts = append(parts, text)
		(*idx)++
	}
	return strings.Join(parts, "\n"), nil
}

func scalarOrInlineNode(v string, line int) *simpleYAMLNode {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
		var values []any
		normalized := v
		if err := json.Unmarshal([]byte(normalized), &values); err == nil {
			n := &simpleYAMLNode{kind: yamlList, line: line}
			for _, value := range values {
				n.list = append(n.list, &simpleYAMLNode{kind: yamlScalar, scalar: fmt.Sprint(value), line: line})
			}
			return n
		}
		inner := strings.TrimSpace(v[1 : len(v)-1])
		n := &simpleYAMLNode{kind: yamlList, line: line}
		if inner == "" {
			return n
		}
		for _, item := range strings.Split(inner, ",") {
			n.list = append(n.list, &simpleYAMLNode{kind: yamlScalar, scalar: unquote(strings.TrimSpace(item)), line: line})
		}
		return n
	}
	return &simpleYAMLNode{kind: yamlScalar, scalar: unquote(v), line: line}
}

func nodeString(n *simpleYAMLNode) (string, error) {
	if n == nil || n.kind != yamlScalar {
		return "", fmt.Errorf("skalarer Wert erwartet")
	}
	return n.scalar, nil
}
func nodeInt(n *simpleYAMLNode) (int, error) {
	s, err := nodeString(n)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(s)
}
func nodeBool(n *simpleYAMLNode) (bool, error) {
	s, err := nodeString(n)
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(strings.ToLower(s))
}
func nodeStringList(n *simpleYAMLNode) ([]string, error) {
	if n == nil {
		return nil, nil
	}
	if n.kind == yamlScalar {
		return []string{n.scalar}, nil
	}
	if n.kind != yamlList {
		return nil, fmt.Errorf("Liste erwartet")
	}
	out := make([]string, 0, len(n.list))
	for _, item := range n.list {
		s, err := nodeString(item)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}
func lineError(n *simpleYAMLNode, message string) error {
	if n == nil {
		return fmt.Errorf("update-cli.yaml: %s", message)
	}
	return fmt.Errorf("update-cli.yaml Zeile %d: %s", n.line, message)
}

func yamlNodeToAny(n *simpleYAMLNode) any {
	if n == nil {
		return nil
	}
	switch n.kind {
	case yamlScalar:
		if b, err := strconv.ParseBool(strings.ToLower(n.scalar)); err == nil {
			return b
		}
		if i, err := strconv.Atoi(n.scalar); err == nil {
			return i
		}
		return n.scalar
	case yamlList:
		out := make([]any, 0, len(n.list))
		for _, item := range n.list {
			out = append(out, yamlNodeToAny(item))
		}
		return out
	case yamlMap:
		out := map[string]any{}
		for k, v := range n.m {
			out[k] = yamlNodeToAny(v)
		}
		return out
	default:
		return nil
	}
}
func yamlNodeToMap(n *simpleYAMLNode) map[string]any {
	if n == nil {
		return map[string]any{}
	}
	if n.kind != yamlMap {
		return map[string]any{"value": yamlNodeToAny(n)}
	}
	out := map[string]any{}
	for k, v := range n.m {
		out[k] = yamlNodeToAny(v)
	}
	return out
}
