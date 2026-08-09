package projectsetup

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/r14r/update-cli/lib/ui"
)

type Selection struct {
	Workflow string
	Task     string
}

type Catalog struct {
	Project   string            `json:"project,omitempty"`
	Workflows []CatalogWorkflow `json:"workflows"`
	Tasks     []CatalogTask     `json:"tasks"`
}

type CatalogWorkflow struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tasks       []string `json:"tasks"`
}

type CatalogTask struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Requires    []string `json:"requires,omitempty"`
	Steps       int      `json:"steps"`
}

func CatalogForManifest(path string) (Catalog, error) {
	m, err := ParseManifest(path)
	if err != nil {
		return Catalog{}, err
	}
	c := Catalog{Project: m.ProjectName}
	if m.Version == 1 {
		c.Workflows = []CatalogWorkflow{{Name: "setup", Description: "Legacy setup workflow", Tasks: []string{"setup"}}}
		c.Tasks = []CatalogTask{{Name: "setup", Steps: len(m.Steps)}}
		return c, nil
	}
	names := make([]string, 0, len(m.Workflows))
	for name := range m.Workflows {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		w := m.Workflows[name]
		c.Workflows = append(c.Workflows, CatalogWorkflow{Name: name, Description: w.Description, Tasks: append([]string(nil), w.Tasks...)})
	}
	names = names[:0]
	for name := range m.Tasks {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t := m.Tasks[name]
		c.Tasks = append(c.Tasks, CatalogTask{Name: name, Description: t.Description, Requires: append([]string(nil), t.Requires...), Steps: len(t.Steps)})
	}
	return c, nil
}

func runManifestV2(ctx context.Context, root string, m Manifest, console *ui.Console, path string, selection Selection) (Result, error) {
	if err := validateV2Requirements(m, console); err != nil {
		return Result{Manifest: path}, err
	}
	vars := resolveVariables(root, m)
	taskNames, err := resolveTaskPlan(m, selection)
	if err != nil {
		return Result{Manifest: path}, err
	}
	total := 0
	for _, name := range taskNames {
		total += len(m.Tasks[name].Steps)
	}

	if console.Fullscreen() {
		if console.Title() == "Update CLI Setup" {
			console.SetInfoTitle("Projekt-Setup")
			if m.ProjectName != "" {
				console.InfoRow("Projekt", m.ProjectName)
			}
			if m.ProjectDescription != "" {
				console.InfoRow("Beschreibung", m.ProjectDescription)
			}
			if m.ProjectType != "" {
				console.InfoRow("Typ", m.ProjectType)
			}
			console.InfoRow("Manifest", path)
			console.InfoRow("Schema", "2")
			if selection.Task != "" {
				console.InfoRow("Task", selection.Task)
			} else {
				console.InfoRow("Workflow", selectedWorkflowName(m, selection))
			}
			console.InfoRow("Tasks", fmt.Sprintf("%d", len(taskNames)))
			console.InfoRow("Schritte", fmt.Sprintf("%d", total))
		} else {
			console.Append("")
			console.Append("Projekt-Setup")
			if m.ProjectName != "" {
				console.Append("  Projekt: " + m.ProjectName)
			}
			console.Append(fmt.Sprintf("  Schema: 2 | Tasks: %d | Schritte: %d", len(taskNames), total))
		}
	} else {
		console.Header("Projekt-Setup")
		if m.ProjectName != "" {
			console.Row("Projekt", m.ProjectName)
		}
		console.Row("Manifest", path)
		console.Row("Schema", "2")
		console.Row("Tasks", fmt.Sprintf("%d", len(taskNames)))
		console.Row("Schritte", fmt.Sprintf("%d", total))
	}

	result := Result{Manifest: path}
	index := 0
	for _, taskName := range taskNames {
		task := m.Tasks[taskName]
		if console.Details() {
			console.Append("Task: " + taskName)
		}
		for stepIndex := range task.Steps {
			original := task.Steps[stepIndex]
			step := expandStepV2(original, vars)
			label := step.Name
			run, reason, condErr := evaluateCondition(root, step.When)
			if condErr != nil {
				return result, fmt.Errorf("task %s step %d (%s) Bedingung ungültig: %w", taskName, stepIndex+1, label, condErr)
			}
			if !run {
				result.StepsSkipped++
				console.SkipStep(index, total, label, reason)
				index++
				continue
			}
			execErr := console.Step(ctx, index, total, label, func() error {
				return executeV2Step(ctx, root, step, m.Defaults, console)
			})
			if execErr != nil {
				if step.AllowFailure || !m.Defaults.FailFast {
					console.Warn(fmt.Sprintf("Schritt fehlgeschlagen, wird fortgesetzt: %v", execErr))
					index++
					continue
				}
				return result, fmt.Errorf("task %s step %d (%s) fehlgeschlagen: %w", taskName, stepIndex+1, label, execErr)
			}
			result.StepsExecuted++
			index++
		}
	}
	return result, nil
}

func selectedWorkflowName(m Manifest, selection Selection) string {
	if selection.Workflow != "" {
		return selection.Workflow
	}
	if _, ok := m.Workflows["setup"]; ok {
		return "setup"
	}
	if len(m.Workflows) == 1 {
		for name := range m.Workflows {
			return name
		}
	}
	return "setup"
}

func resolveTaskPlan(m Manifest, selection Selection) ([]string, error) {
	roots := []string{}
	if selection.Task != "" {
		if _, ok := m.Tasks[selection.Task]; !ok {
			return nil, fmt.Errorf("unbekannter setup task %q", selection.Task)
		}
		roots = []string{selection.Task}
	} else {
		name := selectedWorkflowName(m, selection)
		w, ok := m.Workflows[name]
		if !ok {
			if name == "setup" {
				if _, taskOK := m.Tasks["setup"]; taskOK {
					roots = []string{"setup"}
				} else {
					return nil, errors.New("setup.yaml schemaVersion 2 benötigt workflow 'setup' oder task 'setup'")
				}
			} else {
				return nil, fmt.Errorf("unbekannter setup workflow %q", name)
			}
		} else {
			roots = append(roots, w.Tasks...)
		}
	}
	visited := map[string]bool{}
	active := map[string]bool{}
	out := []string{}
	var visit func(string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		if active[name] {
			return fmt.Errorf("zyklische task-Abhängigkeit bei %q", name)
		}
		task, ok := m.Tasks[name]
		if !ok {
			return fmt.Errorf("unbekannter task %q", name)
		}
		active[name] = true
		for _, dep := range task.Requires {
			if err := visit(dep); err != nil {
				return err
			}
		}
		delete(active, name)
		visited[name] = true
		out = append(out, name)
		return nil
	}
	for _, name := range roots {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func validateV2Requirements(m Manifest, console *ui.Console) error {
	for _, name := range m.Requirements.Commands {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("erforderliches Programm fehlt: %s", name)
		}
	}
	for _, name := range m.Requirements.OptionalCommands {
		if _, err := exec.LookPath(name); err != nil && console.Details() {
			console.Warn("Optionales Programm fehlt: " + name)
		}
	}
	return nil
}

func resolveVariables(root string, m Manifest) map[string]string {
	vars := map[string]string{
		"project.root": root,
		"project.name": m.ProjectName,
		"project.type": m.ProjectType,
		"os":           runtime.GOOS,
		"arch":         runtime.GOARCH,
	}
	for k, v := range m.Variables {
		vars[k] = v
	}
	for pass := 0; pass < 8; pass++ {
		changed := false
		for k, v := range vars {
			expanded := expandTemplate(v, vars)
			if expanded != v {
				vars[k] = expanded
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return vars
}

func expandTemplate(value string, vars map[string]string) string {
	for {
		start := strings.Index(value, "{{")
		if start < 0 {
			break
		}
		endRel := strings.Index(value[start+2:], "}}")
		if endRel < 0 {
			break
		}
		end := start + 2 + endRel
		key := strings.TrimSpace(value[start+2 : end])
		lookup := key
		fallback := ""
		if parts := strings.SplitN(key, "|", 2); len(parts) == 2 {
			lookup = strings.TrimSpace(parts[0])
			fallback = strings.TrimSpace(parts[1])
		}
		replacement := ""
		if strings.HasPrefix(lookup, "env.") {
			replacement = os.Getenv(strings.TrimPrefix(lookup, "env."))
		} else {
			replacement = vars[lookup]
		}
		if replacement == "" {
			replacement = fallback
		}
		value = value[:start] + replacement + value[end+2:]
	}
	return value
}

func expandStepV2(s StepV2, vars map[string]string) StepV2 {
	s.ID = expandTemplate(s.ID, vars)
	s.Name = expandTemplate(s.Name, vars)
	s.WorkingDirectory = expandTemplate(s.WorkingDirectory, vars)
	s.Timeout = expandTemplate(s.Timeout, vars)
	env := map[string]string{}
	for k, v := range s.Environment {
		env[k] = expandTemplate(v, vars)
	}
	s.Environment = env
	s.Config = expandAnyMap(s.Config, vars)
	s.When = expandCondition(s.When, vars)
	return s
}
func expandAnyMap(in map[string]any, vars map[string]string) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = expandAny(v, vars)
	}
	return out
}
func expandAny(v any, vars map[string]string) any {
	switch x := v.(type) {
	case string:
		return expandTemplate(x, vars)
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = expandAny(x[i], vars)
		}
		return out
	case map[string]any:
		return expandAnyMap(x, vars)
	default:
		return v
	}
}
func expandCondition(c *Condition, vars map[string]string) *Condition {
	if c == nil {
		return nil
	}
	out := &Condition{Kind: c.Kind, Value: expandAny(c.Value, vars)}
	for _, ch := range c.Children {
		out.Children = append(out.Children, expandCondition(ch, vars))
	}
	return out
}

func evaluateCondition(root string, c *Condition) (bool, string, error) {
	if c == nil {
		return true, "", nil
	}
	switch c.Kind {
	case "all":
		for _, ch := range c.Children {
			ok, reason, err := evaluateCondition(root, ch)
			if err != nil {
				return false, "", err
			}
			if !ok {
				return false, reason, nil
			}
		}
		return true, "", nil
	case "any":
		reasons := []string{}
		for _, ch := range c.Children {
			ok, reason, err := evaluateCondition(root, ch)
			if err != nil {
				return false, "", err
			}
			if ok {
				return true, "", nil
			}
			if reason != "" {
				reasons = append(reasons, reason)
			}
		}
		return false, strings.Join(reasons, "; "), nil
	case "not":
		if len(c.Children) != 1 {
			return false, "", errors.New("not benötigt genau eine Bedingung")
		}
		ok, _, err := evaluateCondition(root, c.Children[0])
		if err != nil {
			return false, "", err
		}
		if ok {
			return false, "negierte Bedingung ist erfüllt", nil
		}
		return true, "", nil
	}
	stringValue := func() string {
		if c.Value == nil {
			return ""
		}
		return fmt.Sprint(c.Value)
	}
	pathCheck := func(wantDir bool, negate bool) (bool, string, error) {
		rel := stringValue()
		p, err := containedSetupPath(root, rel)
		if err != nil {
			return false, "", err
		}
		info, statErr := os.Stat(p)
		exists := statErr == nil
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return false, "", statErr
		}
		ok := exists && info.IsDir() == wantDir
		if negate {
			ok = !ok
		}
		if ok {
			return true, "", nil
		}
		return false, "Bedingung nicht erfüllt: " + c.Kind + "=" + rel, nil
	}
	switch c.Kind {
	case "fileExists", "file":
		return pathCheck(false, false)
	case "fileNotExists", "not-file":
		return pathCheck(false, true)
	case "directoryExists", "dir":
		return pathCheck(true, false)
	case "commandExists", "command":
		_, err := exec.LookPath(stringValue())
		return err == nil, "Kommando fehlt: " + stringValue(), nil
	case "envSet", "env":
		_, ok := os.LookupEnv(stringValue())
		return ok, "Umgebungsvariable fehlt: " + stringValue(), nil
	case "os":
		return valueMatches(c.Value, runtime.GOOS), "Betriebssystem ist " + runtime.GOOS, nil
	case "arch":
		return valueMatches(c.Value, runtime.GOARCH), "Architektur ist " + runtime.GOARCH, nil
	case "compose":
		for _, name := range []string{"compose.yml", "compose.yaml", "docker-compose.yml", "docker-compose.yaml"} {
			if info, err := os.Stat(filepath.Join(root, name)); err == nil && !info.IsDir() {
				return true, "", nil
			}
		}
		return false, "keine Compose-Datei", nil
	default:
		return false, "", fmt.Errorf("unbekannte when-Bedingung %q", c.Kind)
	}
}
func valueMatches(v any, actual string) bool {
	switch x := v.(type) {
	case string:
		return strings.EqualFold(x, actual)
	case []any:
		for _, item := range x {
			if strings.EqualFold(fmt.Sprint(item), actual) {
				return true
			}
		}
	}
	return false
}

func executeV2Step(ctx context.Context, root string, s StepV2, defaults SetupDefaults, console *ui.Console) error {
	work, err := setupWorkDir(root, s.WorkingDirectory)
	if err != nil {
		return err
	}
	timeout := s.Timeout
	if timeout == "" {
		timeout = defaults.Timeout
	}
	if timeout != "" {
		d, err := time.ParseDuration(timeout)
		if err != nil {
			return fmt.Errorf("ungültiges timeout %q", timeout)
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}
	attempts := s.Retries + 1
	var last error
	for attempt := 1; attempt <= attempts; attempt++ {
		last = executeV2Operation(ctx, root, work, s, console)
		if last == nil {
			return nil
		}
		if attempt < attempts {
			console.Append(fmt.Sprintf("Wiederholung %d/%d: %v", attempt, attempts-1, last))
		}
	}
	return last
}

func setupWorkDir(root, cwd string) (string, error) {
	if cwd == "" || cwd == "." {
		return root, nil
	}
	return containedSetupPath(root, cwd)
}

func executeV2Operation(ctx context.Context, root, work string, s StepV2, console *ui.Console) error {
	cfg := s.Config
	switch s.Operation {
	case "command":
		exe := cfgString(cfg, "exec", cfgString(cfg, "value", ""))
		if exe == "" {
			return errors.New("command.exec fehlt")
		}
		args := cfgStrings(cfg, "args")
		return runV2Command(ctx, work, exe, args, s.Environment, console)
	case "shell":
		script := cfgString(cfg, "value", cfgString(cfg, "run", ""))
		if script == "" {
			return errors.New("shell-Inhalt fehlt")
		}
		bash, err := exec.LookPath("bash")
		if err != nil {
			return err
		}
		return runV2Command(ctx, work, bash, []string{"-c", script}, s.Environment, console)
	case "mkdir":
		return opMkdir(root, cfg)
	case "copy", "deploy":
		return opCopy(root, cfg, s.Operation == "deploy")
	case "move":
		return opMove(root, cfg)
	case "remove":
		return opRemove(root, cfg)
	case "chmod":
		return opChmod(root, cfg)
	case "symlink":
		return opSymlink(root, cfg)
	case "touch":
		return opTouch(root, cfg)
	case "write":
		return opWrite(root, cfg)
	case "assert":
		return opAssert(ctx, root, cfg)
	case "pythonVenv":
		python := cfgString(cfg, "python", "python3")
		path := cfgString(cfg, "path", ".venv")
		return runV2Command(ctx, work, python, []string{"-m", "venv", path}, s.Environment, console)
	case "pip":
		return opPip(ctx, work, cfg, s.Environment, console)
	case "npm", "pnpm", "yarn":
		return opPackageManager(ctx, work, s.Operation, cfg, s.Environment, console)
	case "composer":
		return opComposer(ctx, work, cfg, s.Environment, console)
	case "artisan":
		return opArtisan(ctx, work, cfg, s.Environment, console)
	case "dockerCompose":
		return opDockerCompose(ctx, work, cfg, s.Environment, console)
	case "go":
		return opGo(ctx, work, cfg, s.Environment, console)
	case "httpCheck":
		return opHTTPCheck(ctx, cfg)
	case "download":
		return opDownload(ctx, root, cfg)
	case "extract":
		return opExtract(root, cfg)
	default:
		return fmt.Errorf("unbekannte setup operation %q", s.Operation)
	}
}

func runV2Command(ctx context.Context, work, exe string, args []string, env map[string]string, console *ui.Console) error {
	resolved, err := exec.LookPath(exe)
	if err != nil {
		if filepath.IsAbs(exe) || strings.Contains(exe, string(filepath.Separator)) {
			resolved = exe
		} else {
			return fmt.Errorf("erforderliches Programm fehlt: %s", exe)
		}
	}
	if console.Details() || console.Direct() {
		console.Append("❯ " + displayCommand(resolved, args))
	}
	return runCommandWithEnv(ctx, work, resolved, args, console, env)
}

func cfgString(m map[string]any, key, def string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return def
	}
	return fmt.Sprint(v)
}
func cfgBool(m map[string]any, key string, def bool) bool {
	v, ok := m[key]
	if !ok {
		return def
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		b, e := strconv.ParseBool(x)
		if e == nil {
			return b
		}
	}
	return def
}
func cfgInt(m map[string]any, key string, def int) int {
	v, ok := m[key]
	if !ok {
		return def
	}
	switch x := v.(type) {
	case int:
		return x
	case string:
		i, e := strconv.Atoi(x)
		if e == nil {
			return i
		}
	}
	return def
}
func cfgStrings(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch x := v.(type) {
	case []any:
		out := []string{}
		for _, item := range x {
			out = append(out, fmt.Sprint(item))
		}
		return out
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	}
	return nil
}
func cfgPathList(m map[string]any) []string {
	if paths := cfgStrings(m, "paths"); len(paths) > 0 {
		return paths
	}
	if p := cfgString(m, "path", cfgString(m, "value", "")); p != "" {
		return []string{p}
	}
	return nil
}

func safeProjectPath(root, p string, allowAbsolute bool) (string, error) {
	if filepath.IsAbs(p) {
		if allowAbsolute {
			return filepath.Clean(p), nil
		}
		return "", errors.New("absoluter Pfad ist hier nicht erlaubt")
	}
	return containedSetupPath(root, p)
}
func parseMode(value string, def os.FileMode) (os.FileMode, error) {
	if value == "" {
		return def, nil
	}
	n, err := strconv.ParseUint(value, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("ungültiger mode %q", value)
	}
	return os.FileMode(n), nil
}

func opMkdir(root string, cfg map[string]any) error {
	mode, err := parseMode(cfgString(cfg, "mode", ""), 0o755)
	if err != nil {
		return err
	}
	paths := cfgPathList(cfg)
	if len(paths) == 0 {
		return errors.New("mkdir benötigt path oder paths")
	}
	for _, p := range paths {
		abs, e := safeProjectPath(root, p, false)
		if e != nil {
			return e
		}
		if e = os.MkdirAll(abs, mode); e != nil {
			return e
		}
	}
	return nil
}
func opCopy(root string, cfg map[string]any, allowAbs bool) error {
	src := cfgString(cfg, "source", "")
	dst := cfgString(cfg, "target", cfgString(cfg, "destination", ""))
	if src == "" || dst == "" {
		return errors.New("copy benötigt source und target")
	}
	srcAbs, e := safeProjectPath(root, src, false)
	if e != nil {
		return e
	}
	dstAbs, e := safeProjectPath(root, dst, allowAbs)
	if e != nil {
		return e
	}
	if !cfgBool(cfg, "overwrite", true) {
		if _, e = os.Stat(dstAbs); e == nil {
			return nil
		}
	}
	if e = copyPath(srcAbs, dstAbs); e != nil {
		return e
	}
	if mode := cfgString(cfg, "mode", ""); mode != "" {
		m, e := parseMode(mode, 0)
		if e != nil {
			return e
		}
		return os.Chmod(dstAbs, m)
	}
	return nil
}
func opMove(root string, cfg map[string]any) error {
	src := cfgString(cfg, "source", "")
	dst := cfgString(cfg, "target", cfgString(cfg, "destination", ""))
	if src == "" || dst == "" {
		return errors.New("move benötigt source und target")
	}
	a, e := safeProjectPath(root, src, false)
	if e != nil {
		return e
	}
	b, e := safeProjectPath(root, dst, false)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(b), 0o755); e != nil {
		return e
	}
	return os.Rename(a, b)
}
func opRemove(root string, cfg map[string]any) error {
	paths := cfgPathList(cfg)
	if len(paths) == 0 {
		return errors.New("remove benötigt path oder paths")
	}
	recursive := cfgBool(cfg, "recursive", false)
	cleanRoot := filepath.Clean(root)
	for _, p := range paths {
		abs, e := safeProjectPath(root, p, false)
		if e != nil {
			return e
		}
		if filepath.Clean(abs) == cleanRoot {
			return errors.New("remove darf nicht den Projektwurzelordner löschen")
		}
		if recursive {
			e = os.RemoveAll(abs)
		} else {
			e = os.Remove(abs)
		}
		if e != nil && !errors.Is(e, os.ErrNotExist) {
			return e
		}
	}
	return nil
}
func opChmod(root string, cfg map[string]any) error {
	p := cfgString(cfg, "path", "")
	mode := cfgString(cfg, "mode", "")
	if p == "" || mode == "" {
		return errors.New("chmod benötigt path und mode")
	}
	abs, e := safeProjectPath(root, p, false)
	if e != nil {
		return e
	}
	m, e := parseMode(mode, 0)
	if e != nil {
		return e
	}
	return os.Chmod(abs, m)
}
func opSymlink(root string, cfg map[string]any) error {
	src := cfgString(cfg, "source", "")
	target := cfgString(cfg, "target", "")
	if src == "" || target == "" {
		return errors.New("symlink benötigt source und target")
	}
	targetAbs, e := safeProjectPath(root, target, false)
	if e != nil {
		return e
	}
	if cfgBool(cfg, "replace", false) {
		_ = os.Remove(targetAbs)
	}
	if e = os.MkdirAll(filepath.Dir(targetAbs), 0o755); e != nil {
		return e
	}
	return os.Symlink(src, targetAbs)
}
func opTouch(root string, cfg map[string]any) error {
	paths := cfgPathList(cfg)
	if len(paths) == 0 {
		return errors.New("touch benötigt path oder paths")
	}
	now := time.Now()
	for _, p := range paths {
		abs, e := safeProjectPath(root, p, false)
		if e != nil {
			return e
		}
		f, e := os.OpenFile(abs, os.O_CREATE|os.O_WRONLY, 0o644)
		if e != nil {
			return e
		}
		_ = f.Close()
		if e = os.Chtimes(abs, now, now); e != nil {
			return e
		}
	}
	return nil
}
func opWrite(root string, cfg map[string]any) error {
	p := cfgString(cfg, "path", "")
	content := cfgString(cfg, "content", cfgString(cfg, "value", ""))
	if p == "" {
		return errors.New("write benötigt path")
	}
	abs, e := safeProjectPath(root, p, false)
	if e != nil {
		return e
	}
	if !cfgBool(cfg, "overwrite", true) {
		if _, e = os.Stat(abs); e == nil {
			return nil
		}
	}
	mode, e := parseMode(cfgString(cfg, "mode", ""), 0o644)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(abs), 0o755); e != nil {
		return e
	}
	return os.WriteFile(abs, []byte(content), mode)
}

func opAssert(ctx context.Context, root string, cfg map[string]any) error {
	for k, v := range cfg {
		switch k {
		case "fileExists":
			ok, _, e := evaluateCondition(root, &Condition{Kind: k, Value: v})
			if e != nil {
				return e
			}
			if !ok {
				return fmt.Errorf("assert fehlgeschlagen: Datei fehlt: %v", v)
			}
		case "directoryExists":
			ok, _, e := evaluateCondition(root, &Condition{Kind: k, Value: v})
			if e != nil {
				return e
			}
			if !ok {
				return fmt.Errorf("assert fehlgeschlagen: Ordner fehlt: %v", v)
			}
		case "executable":
			p, e := safeProjectPath(root, fmt.Sprint(v), false)
			if e != nil {
				return e
			}
			info, e := os.Stat(p)
			if e != nil || info.Mode()&0o111 == 0 {
				return fmt.Errorf("assert fehlgeschlagen: nicht ausführbar: %v", v)
			}
		case "commandExists":
			if _, e := exec.LookPath(fmt.Sprint(v)); e != nil {
				return fmt.Errorf("assert fehlgeschlagen: Kommando fehlt: %v", v)
			}
		case "envSet":
			if _, ok := os.LookupEnv(fmt.Sprint(v)); !ok {
				return fmt.Errorf("assert fehlgeschlagen: Umgebungsvariable fehlt: %v", v)
			}
		case "portAvailable":
			addr := fmt.Sprintf("127.0.0.1:%v", v)
			ln, e := net.Listen("tcp", addr)
			if e != nil {
				return fmt.Errorf("assert fehlgeschlagen: Port nicht verfügbar: %v", v)
			}
			_ = ln.Close()
		case "http":
			m, ok := v.(map[string]any)
			if !ok {
				return errors.New("assert.http muss eine Map sein")
			}
			if e := opHTTPCheck(ctx, m); e != nil {
				return e
			}
		default:
			return fmt.Errorf("unbekannte assert-Prüfung %q", k)
		}
	}
	return nil
}

func opPip(ctx context.Context, work string, cfg map[string]any, env map[string]string, console *ui.Console) error {
	python := cfgString(cfg, "python", filepath.Join(".venv", "bin", "python"))
	if _, e := os.Stat(filepath.Join(work, python)); e != nil && python == filepath.Join(".venv", "bin", "python") {
		python = "python3"
	}
	args := []string{"-m", "pip", "install"}
	if req := cfgString(cfg, "requirements", ""); req != "" {
		args = append(args, "-r", req)
	}
	args = append(args, cfgStrings(cfg, "args")...)
	return runV2Command(ctx, work, python, args, env, console)
}
func opPackageManager(ctx context.Context, work, manager string, cfg map[string]any, env map[string]string, console *ui.Console) error {
	action := cfgString(cfg, "action", "install")
	args := []string{}
	switch manager {
	case "npm":
		switch action {
		case "install":
			if _, e := os.Stat(filepath.Join(work, "package-lock.json")); e == nil {
				args = []string{"ci"}
			} else {
				args = []string{"install"}
			}
		case "build":
			args = []string{"run", "build"}
		case "test":
			args = []string{"test"}
		default:
			args = []string{action}
		}
	case "pnpm", "yarn":
		args = []string{action}
	}
	args = append(args, cfgStrings(cfg, "args")...)
	return runV2Command(ctx, work, manager, args, env, console)
}
func opComposer(ctx context.Context, work string, cfg map[string]any, env map[string]string, console *ui.Console) error {
	action := cfgString(cfg, "action", "install")
	args := []string{action}
	if action == "install" {
		args = append(args, "--no-interaction", "--prefer-dist")
	}
	if cfgBool(cfg, "production", false) {
		args = append(args, "--no-dev", "--optimize-autoloader")
	}
	args = append(args, cfgStrings(cfg, "args")...)
	return runV2Command(ctx, work, "composer", args, env, console)
}
func opArtisan(ctx context.Context, work string, cfg map[string]any, env map[string]string, console *ui.Console) error {
	command := cfgString(cfg, "command", cfgString(cfg, "value", ""))
	if command == "" {
		return errors.New("artisan.command fehlt")
	}
	args := []string{"artisan", command}
	args = append(args, cfgStrings(cfg, "args")...)
	return runV2Command(ctx, work, "php", args, env, console)
}
func opDockerCompose(ctx context.Context, work string, cfg map[string]any, env map[string]string, console *ui.Console) error {
	action := cfgString(cfg, "action", "")
	if action == "" {
		return errors.New("dockerCompose.action fehlt")
	}
	args := []string{"compose"}
	if f := cfgString(cfg, "file", ""); f != "" {
		args = append(args, "-f", f)
	}
	for _, p := range cfgStrings(cfg, "profiles") {
		args = append(args, "--profile", p)
	}
	args = append(args, action)
	if action == "up" && cfgBool(cfg, "detach", true) {
		args = append(args, "-d")
	}
	if (action == "up" || action == "down") && cfgBool(cfg, "removeOrphans", true) {
		args = append(args, "--remove-orphans")
	}
	args = append(args, cfgStrings(cfg, "args")...)
	return runV2Command(ctx, work, "docker", args, env, console)
}
func opGo(ctx context.Context, work string, cfg map[string]any, env map[string]string, console *ui.Console) error {
	action := cfgString(cfg, "action", "")
	args := []string{}
	switch action {
	case "mod-download":
		args = []string{"mod", "download"}
	case "vet":
		args = []string{"vet", "./..."}
	case "test":
		args = []string{"test"}
		args = append(args, cfgStrings(cfg, "args")...)
		args = append(args, "./...")
	case "generate":
		args = []string{"generate", "./..."}
	case "build":
		args = []string{"build"}
		args = append(args, cfgStrings(cfg, "args")...)
		if out := cfgString(cfg, "output", ""); out != "" {
			args = append(args, "-o", out)
		}
		pkg := cfgString(cfg, "package", ".")
		args = append(args, pkg)
	default:
		return fmt.Errorf("unbekannte Go-Aktion %q", action)
	}
	return runV2Command(ctx, work, "go", args, env, console)
}
func opHTTPCheck(ctx context.Context, cfg map[string]any) error {
	url := cfgString(cfg, "url", cfgString(cfg, "value", ""))
	if url == "" {
		return errors.New("httpCheck.url fehlt")
	}
	status := cfgInt(cfg, "status", 200)
	timeout := 10 * time.Second
	if raw := cfgString(cfg, "timeout", ""); raw != "" {
		d, e := time.ParseDuration(raw)
		if e != nil {
			return e
		}
		timeout = d
	}
	client := &http.Client{Timeout: timeout}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if e != nil {
		return e
	}
	resp, e := client.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode != status {
		return fmt.Errorf("HTTP %s: erwartet %d, erhalten %d", url, status, resp.StatusCode)
	}
	return nil
}
func opDownload(ctx context.Context, root string, cfg map[string]any) error {
	url := cfgString(cfg, "url", "")
	dst := cfgString(cfg, "destination", cfgString(cfg, "target", ""))
	if url == "" || dst == "" {
		return errors.New("download benötigt url und destination")
	}
	abs, e := safeProjectPath(root, dst, false)
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if e != nil {
		return e
	}
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Download HTTP %d", resp.StatusCode)
	}
	if e = os.MkdirAll(filepath.Dir(abs), 0o755); e != nil {
		return e
	}
	tmp := abs + ".tmp"
	_ = os.Remove(tmp)
	f, e := os.Create(tmp)
	if e != nil {
		return e
	}
	keepTemp := false
	defer func() {
		if !keepTemp {
			_ = os.Remove(tmp)
		}
	}()
	h := sha256.New()
	_, cp := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, 512*1024*1024+1))
	closeErr := f.Close()
	if cp != nil {
		return cp
	}
	if closeErr != nil {
		return closeErr
	}
	if info, e := os.Stat(tmp); e == nil && info.Size() > 512*1024*1024 {
		return errors.New("Download überschreitet 512 MiB")
	}
	if expected := strings.ToLower(cfgString(cfg, "sha256", "")); expected != "" && hex.EncodeToString(h.Sum(nil)) != expected {
		return errors.New("Download SHA-256 stimmt nicht überein")
	}
	if err := os.Rename(tmp, abs); err != nil {
		return err
	}
	keepTemp = true
	return nil
}
func opExtract(root string, cfg map[string]any) error {
	src := cfgString(cfg, "archive", cfgString(cfg, "source", ""))
	dst := cfgString(cfg, "destination", cfgString(cfg, "target", ""))
	if src == "" || dst == "" {
		return errors.New("extract benötigt archive/source und destination")
	}
	a, e := safeProjectPath(root, src, false)
	if e != nil {
		return e
	}
	d, e := safeProjectPath(root, dst, false)
	if e != nil {
		return e
	}
	r, e := zip.OpenReader(a)
	if e != nil {
		return e
	}
	defer r.Close()
	for _, f := range r.File {
		name := filepath.Clean(f.Name)
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsicherer ZIP-Pfad %q", f.Name)
		}
		target := filepath.Join(d, name)
		if f.FileInfo().IsDir() {
			if e = os.MkdirAll(target, 0o755); e != nil {
				return e
			}
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("ZIP-Symlink nicht erlaubt: %s", f.Name)
		}
		if e = os.MkdirAll(filepath.Dir(target), 0o755); e != nil {
			return e
		}
		in, e := f.Open()
		if e != nil {
			return e
		}
		out, e := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode().Perm())
		if e != nil {
			_ = in.Close()
			return e
		}
		written, cp := io.Copy(out, io.LimitReader(in, 256*1024*1024+1))
		_ = in.Close()
		closeErr := out.Close()
		if cp != nil {
			return cp
		}
		if closeErr != nil {
			return closeErr
		}
		if written > 256*1024*1024 {
			return fmt.Errorf("ZIP-Eintrag überschreitet 256 MiB: %s", f.Name)
		}
	}
	return nil
}
