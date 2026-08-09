package projectsetup

import (
	"context"
	"errors"
	"fmt"
	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/ui"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type Result struct {
	Manifest               string `json:"manifest,omitempty"`
	StepsExecuted          int    `json:"stepsExecuted"`
	StepsSkipped           int    `json:"stepsSkipped,omitempty"`
	LegacyScriptExecuted   bool   `json:"legacyScriptExecuted"`
	LegacyCommandsExecuted int    `json:"legacyCommandsExecuted"`
}

func FindManifest(dir string) (string, bool, error) {
	for _, name := range []string{"setup.yaml", "setup.yml"} {
		p := filepath.Join(dir, name)
		i, err := os.Stat(p)
		if err == nil {
			if i.IsDir() {
				return "", false, fmt.Errorf("%s ist ein Ordner", p)
			}
			return p, true, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", false, err
		}
	}
	return "", false, nil
}

func Detect(c config.Config) (string, bool, error) {
	if manifest, ok, err := FindManifest(c.CurrentDir); err != nil || ok {
		return manifest, ok, err
	}
	if i, err := os.Stat(filepath.Join(c.CurrentDir, "setup.sh")); err == nil && !i.IsDir() {
		return filepath.Join(c.CurrentDir, "setup.sh"), true, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", false, err
	}
	return "", len(c.LegacySetupCommands) > 0, nil
}

func Run(ctx context.Context, c config.Config, console *ui.Console) (Result, error) {
	return RunSelected(ctx, c, console, Selection{})
}

func RunSelected(ctx context.Context, c config.Config, console *ui.Console, selection Selection) (Result, error) {
	i, err := os.Stat(c.CurrentDir)
	if err != nil || !i.IsDir() {
		return Result{}, fmt.Errorf("Current-Ordner fehlt oder ist ungültig: %s", c.CurrentDir)
	}
	manifest, ok, err := FindManifest(c.CurrentDir)
	if err != nil {
		return Result{}, err
	}
	if ok {
		return runManifestSelected(ctx, c, console, manifest, selection)
	}
	return runLegacy(ctx, c, console)
}

func RunStandalone(ctx context.Context, path string, console *ui.Console) (Result, error) {
	return RunStandaloneSelected(ctx, path, console, Selection{})
}

func RunStandaloneSelected(ctx context.Context, path string, console *ui.Console, selection Selection) (Result, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return Result{}, err
	}
	manifest, err := ParseManifest(absolute)
	if err != nil {
		return Result{}, err
	}
	projectName := manifest.ProjectName
	if projectName == "" {
		projectName = filepath.Base(filepath.Dir(absolute))
	}
	cfg := config.Config{ProjectName: projectName, CurrentDir: filepath.Dir(absolute)}
	return runManifestSelected(ctx, cfg, console, absolute, selection)
}

func runManifest(ctx context.Context, c config.Config, console *ui.Console, path string) (Result, error) {
	return runManifestSelected(ctx, c, console, path, Selection{})
}

func runManifestSelected(ctx context.Context, c config.Config, console *ui.Console, path string, selection Selection) (Result, error) {
	m, err := ParseManifest(path)
	if err != nil {
		return Result{}, err
	}
	if m.Version == 2 {
		return runManifestV2(ctx, c.CurrentDir, m, console, path, selection)
	}
	if selection.Task != "" || selection.Workflow != "" {
		return Result{}, errors.New("--setup-task/--setup-workflow benötigen setup.yaml schemaVersion 2")
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
			console.InfoRow("Schema", fmt.Sprintf("%d", m.Version))
			console.InfoRow("Schritte", fmt.Sprintf("%d", len(m.Steps)))
		} else {
			// Nested setup (for example check -> update -> setup) must not replace
			// the update screen's fixed project information. Put setup metadata in
			// the scrollable content region instead.
			console.Append("")
			console.Append("Projekt-Setup")
			if m.ProjectName != "" {
				console.Append("  Projekt: " + m.ProjectName)
			}
			console.Append(fmt.Sprintf("  Manifest: %s | Schema: %d | Schritte: %d", path, m.Version, len(m.Steps)))
		}
	} else {
		console.Header("Projekt-Setup")
		if m.ProjectName != "" {
			console.Row("Projekt", m.ProjectName)
		}
		if m.ProjectDescription != "" {
			console.Row("Beschreibung", m.ProjectDescription)
		}
		if m.ProjectType != "" {
			console.Row("Typ", m.ProjectType)
		}
		console.Row("Manifest", path)
		console.Row("Schema", fmt.Sprintf("%d", m.Version))
		console.Row("Schritte", fmt.Sprintf("%d", len(m.Steps)))
	}

	r := Result{Manifest: path}
	for i, s := range m.Steps {
		label := s.Name
		if label == "" {
			label = fmt.Sprintf("%s/%s", s.Type, s.Action)
		}
		run, reason, err := shouldRunStep(c.CurrentDir, s)
		if err != nil {
			return r, fmt.Errorf("setup step %d (%s) Bedingung ungültig: %w", i+1, label, err)
		}
		if !run {
			r.StepsSkipped++
			console.SkipStep(i, len(m.Steps), label, reason)
			continue
		}
		err = console.Step(ctx, i, len(m.Steps), label, func() error {
			if console.Details() && s.Command != "" {
				console.Append("❯ " + s.Command)
			}
			return runStep(ctx, c.CurrentDir, s, console)
		})
		if err != nil {
			if s.ContinueOnError {
				console.Warn(fmt.Sprintf("Schritt fehlgeschlagen, wird fortgesetzt: %v", err))
				continue
			}
			return r, fmt.Errorf("setup step %d (%s) fehlgeschlagen: %w", i+1, label, err)
		}
		r.StepsExecuted++
	}
	return r, nil
}

func shouldRunStep(root string, s Step) (bool, string, error) {
	when := strings.TrimSpace(s.When)
	if when == "" || when == "always" {
		return true, "", nil
	}
	parts := strings.SplitN(when, ":", 2)
	kind := strings.ToLower(strings.TrimSpace(parts[0]))
	value := ""
	if len(parts) == 2 {
		value = strings.TrimSpace(parts[1])
	}
	switch kind {
	case "file", "not-file", "dir":
		if value == "" {
			return false, "", fmt.Errorf("%s benötigt einen relativen Pfad", kind)
		}
		p, err := containedSetupPath(root, value)
		if err != nil {
			return false, "", err
		}
		info, statErr := os.Stat(p)
		exists := statErr == nil
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return false, "", statErr
		}
		switch kind {
		case "file":
			ok := exists && !info.IsDir()
			return ok, "Datei fehlt: " + value, nil
		case "not-file":
			ok := !exists || info.IsDir()
			return ok, "Datei existiert: " + value, nil
		case "dir":
			ok := exists && info.IsDir()
			return ok, "Ordner fehlt: " + value, nil
		}
	case "command":
		if value == "" {
			return false, "", errors.New("command benötigt einen Namen")
		}
		_, err := exec.LookPath(value)
		return err == nil, "Kommando fehlt: " + value, nil
	case "env":
		if value == "" {
			return false, "", errors.New("env benötigt einen Variablennamen")
		}
		_, ok := os.LookupEnv(value)
		return ok, "Umgebungsvariable fehlt: " + value, nil
	case "compose":
		for _, name := range []string{"compose.yml", "compose.yaml", "docker-compose.yml", "docker-compose.yaml"} {
			if i, err := os.Stat(filepath.Join(root, name)); err == nil && !i.IsDir() {
				return true, "", nil
			}
		}
		return false, "keine Compose-Datei", nil
	case "os":
		if value == "" {
			return false, "", errors.New("os benötigt darwin oder linux")
		}
		return runtime.GOOS == strings.ToLower(value), "Betriebssystem ist " + runtime.GOOS, nil
	}
	return false, "", fmt.Errorf("unbekannte when-Bedingung %q", when)
}

func containedSetupPath(root, relative string) (string, error) {
	if filepath.IsAbs(relative) {
		return "", errors.New("Setup-Bedingungspfad muss relativ sein")
	}
	clean := filepath.Clean(relative)
	if clean == "." || clean == "" {
		return root, nil
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("Setup-Bedingungspfad darf das Projekt nicht verlassen")
	}
	return filepath.Join(root, clean), nil
}

func runLegacy(ctx context.Context, c config.Config, console *ui.Console) (Result, error) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		return Result{}, errors.New("Legacy-Setup benötigt bash")
	}
	r := Result{}
	script := filepath.Join(c.CurrentDir, "setup.sh")
	hasScript := false
	if i, statErr := os.Stat(script); statErr == nil && !i.IsDir() {
		hasScript = true
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return r, statErr
	}

	total := len(c.LegacySetupCommands)
	if hasScript {
		total++
	}
	if total == 0 {
		console.Warn("Kein setup.yaml/setup.sh vorhanden")
		return r, nil
	}

	if console.Fullscreen() {
		if console.Title() == "Update CLI Setup" {
			console.SetInfoTitle("Legacy-Projekt-Setup")
			if c.ProjectName != "" {
				console.InfoRow("Projekt", c.ProjectName)
			}
			console.InfoRow("Current", c.CurrentDir)
			console.InfoRow("Schritte", fmt.Sprintf("%d", total))
		} else {
			console.Append("")
			console.Append("Legacy-Projekt-Setup")
			if c.ProjectName != "" {
				console.Append("  Projekt: " + c.ProjectName)
			}
			console.Append(fmt.Sprintf("  Current: %s | Schritte: %d", c.CurrentDir, total))
		}
	}

	stepIndex := 0
	if hasScript {
		console.Warn("Legacy setup.sh wird verwendet; Migration auf setup.yaml empfohlen")
		err := console.Step(ctx, stepIndex, total, "Legacy setup.sh ausführen", func() error {
			// A legacy setup.sh often has its own fullscreen/wait implementation.
			// When launched inside Update CLI that output is captured by the parent
			// TUI, so a child SETUP_WAIT=1 appears as a frozen screen waiting for
			// an invisible Enter prompt. Force the nested script into plain/no-wait
			// mode; the parent Update CLI owns terminal rendering and final waiting.
			return runCommandWithEnv(ctx, c.CurrentDir, bash, []string{"./setup.sh"}, console, map[string]string{
				"SETUP_WAIT":     "0",
				"SETUP_TUI_MODE": "plain",
				"SETUP_DETAILS":  "0",
			})
		})
		if err != nil {
			return r, err
		}
		r.LegacyScriptExecuted = true
		stepIndex++
	}
	for i, cmdText := range c.LegacySetupCommands {
		label := fmt.Sprintf("Legacy Setup-Kommando %d", i+1)
		err := console.Step(ctx, stepIndex, total, label, func() error {
			return runCommand(ctx, c.CurrentDir, bash, []string{"-c", cmdText}, console)
		})
		if err != nil {
			return r, err
		}
		r.LegacyCommandsExecuted++
		stepIndex++
	}
	return r, nil
}

func runStep(ctx context.Context, root string, s Step, console *ui.Console) error {
	work := root
	if s.WorkingDirectory != "" && s.WorkingDirectory != "." {
		if filepath.IsAbs(s.WorkingDirectory) {
			return errors.New("workingDirectory/cwd muss relativ sein")
		}
		clean := filepath.Clean(s.WorkingDirectory)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return errors.New("workingDirectory/cwd darf das Projekt nicht verlassen")
		}
		work = filepath.Join(root, clean)
	}
	switch s.Type {
	case "command", "shell":
		if s.Command == "" {
			return errors.New("command/run fehlt")
		}
		bash, err := exec.LookPath("bash")
		if err != nil {
			return err
		}
		return runCommand(ctx, work, bash, []string{"-c", s.Command}, console)
	case "go":
		return runGo(ctx, work, s, console)
	case "python":
		return runPython(ctx, work, s, console)
	case "node":
		return runNode(ctx, work, s, console)
	case "laravel":
		return runLaravel(ctx, work, s, console)
	case "docker-compose", "docker":
		return runDocker(ctx, work, s, console)
	case "copy":
		return copyStep(root, s, false)
	case "deploy":
		return copyStep(root, s, true)
	default:
		return fmt.Errorf("unbekannter setup handler %q", s.Type)
	}
}

func runGo(ctx context.Context, work string, s Step, console *ui.Console) error {
	args := []string{}
	switch s.Action {
	case "mod-download":
		args = []string{"mod", "download"}
	case "vet":
		args = []string{"vet", "./..."}
	case "test":
		args = []string{"test"}
		args = append(args, s.Args...)
		args = append(args, "./...")
	case "generate":
		args = []string{"generate", "./..."}
	case "build":
		args = []string{"build"}
		args = append(args, s.Args...)
		if s.Output != "" {
			args = append(args, "-o", s.Output)
		}
		args = append(args, ".")
	default:
		return fmt.Errorf("unbekannte Go-Aktion %q", s.Action)
	}
	return runNamed(ctx, work, "go", args, console)
}

func runPython(ctx context.Context, work string, s Step, console *ui.Console) error {
	python := "python3"
	if _, err := os.Stat(filepath.Join(work, ".venv", "bin", "python")); err == nil {
		python = filepath.Join(work, ".venv", "bin", "python")
	}
	switch s.Action {
	case "venv":
		return runNamed(ctx, work, "python3", []string{"-m", "venv", ".venv"}, console)
	case "install":
		req := s.Requirements
		if req == "" {
			req = "requirements.txt"
		}
		return runNamed(ctx, work, python, []string{"-m", "pip", "install", "-r", req}, console)
	case "test":
		return runNamed(ctx, work, python, append([]string{"-m", "pytest"}, s.Args...), console)
	default:
		return fmt.Errorf("unbekannte Python-Aktion %q", s.Action)
	}
}

func runNode(ctx context.Context, work string, s Step, console *ui.Console) error {
	switch s.Action {
	case "install":
		args := []string{"install"}
		if _, err := os.Stat(filepath.Join(work, "package-lock.json")); err == nil {
			args = []string{"ci"}
		}
		return runNamed(ctx, work, "npm", args, console)
	case "build":
		return runNamed(ctx, work, "npm", append([]string{"run", "build", "--"}, s.Args...), console)
	case "test":
		return runNamed(ctx, work, "npm", append([]string{"test", "--"}, s.Args...), console)
	default:
		return fmt.Errorf("unbekannte Node-Aktion %q", s.Action)
	}
}

func runLaravel(ctx context.Context, work string, s Step, console *ui.Console) error {
	switch s.Action {
	case "install":
		return runNamed(ctx, work, "composer", []string{"install", "--no-interaction", "--prefer-dist"}, console)
	case "migrate":
		return runNamed(ctx, work, "php", []string{"artisan", "migrate", "--force"}, console)
	case "cache":
		for _, a := range [][]string{{"artisan", "config:cache"}, {"artisan", "route:cache"}, {"artisan", "view:cache"}} {
			if err := runNamed(ctx, work, "php", a, console); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unbekannte Laravel-Aktion %q", s.Action)
	}
}

func runDocker(ctx context.Context, work string, s Step, console *ui.Console) error {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return errors.New("docker fehlt")
	}
	base := []string{"compose"}
	switch s.Action {
	case "build":
		base = append(base, "build")
	case "pull":
		base = append(base, "pull")
	case "up":
		base = append(base, "up", "-d", "--remove-orphans")
	case "down":
		base = append(base, "down", "--remove-orphans")
	case "restart":
		base = append(base, "restart")
	default:
		return fmt.Errorf("unbekannte Docker-Aktion %q", s.Action)
	}
	base = append(base, s.Args...)
	return runCommand(ctx, work, docker, base, console)
}

func copyStep(root string, s Step, allowAbsoluteDest bool) error {
	if s.Source == "" || s.Destination == "" {
		return errors.New("source und destination sind erforderlich")
	}
	src := s.Source
	if !filepath.IsAbs(src) {
		src = filepath.Join(root, src)
	}
	dst := s.Destination
	if !filepath.IsAbs(dst) {
		dst = filepath.Join(root, dst)
	} else if !allowAbsoluteDest {
		return errors.New("copy destination muss relativ sein")
	}
	if err := copyPath(src, dst); err != nil {
		return err
	}
	if s.Mode != "" {
		n, err := strconv.ParseUint(s.Mode, 8, 32)
		if err != nil {
			return fmt.Errorf("ungültiger mode %q", s.Mode)
		}
		if err := os.Chmod(dst, os.FileMode(n)); err != nil {
			return err
		}
	}
	return nil
}

func copyPath(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("Symlink wird beim Setup nicht kopiert: %s", src)
	}
	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyPath(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("nur reguläre Dateien können kopiert werden: %s", src)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, cp := io.Copy(out, in)
	closeErr := out.Close()
	if cp != nil {
		return cp
	}
	return closeErr
}

func runNamed(ctx context.Context, work, name string, args []string, console *ui.Console) error {
	exe, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("erforderliches Programm fehlt: %s", name)
	}
	return runCommand(ctx, work, exe, args, console)
}

func runCommand(ctx context.Context, work, exe string, args []string, console *ui.Console) error {
	return runCommandWithEnv(ctx, work, exe, args, console, nil)
}

func runCommandWithEnv(ctx context.Context, work, exe string, args []string, console *ui.Console, extra map[string]string) error {
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = work
	env := map[string]string{"UPDATE_CLI_SETUP_RUNNING": "1"}
	for key, value := range extra {
		env[key] = value
	}
	cmd.Env = mergedEnvironment(os.Environ(), env)
	cmd.Stdin = os.Stdin

	// In fullscreen mode child-process output is streamed into the scrollable
	// content region. Keep a bounded copy in parallel so failures can still
	// repeat the most relevant tail even after earlier output has scrolled out.
	var captured *tailWriter
	stdout, stderr := console.ProcessWriters()
	if console.Fullscreen() {
		captured = &tailWriter{max: 64 * 1024}
		cmd.Stdout = io.MultiWriter(stdout, captured)
		cmd.Stderr = io.MultiWriter(stderr, captured)
	} else {
		cmd.Stdout = stdout
		cmd.Stderr = stderr
	}

	err := cmd.Run()
	for _, writer := range []io.Writer{stdout, stderr} {
		if flusher, ok := writer.(interface{ Flush() }); ok {
			flusher.Flush()
		}
	}
	if err != nil && captured != nil {
		console.Append("")
		console.Append("Fehlgeschlagener Befehl: " + displayCommand(exe, args))
		lines := failureTail(captured.String(), 30)
		if len(lines) == 0 {
			console.Append("Fehler: " + err.Error())
		} else {
			console.Append("Fehlerausgabe (letzte Zeilen):")
			for _, line := range lines {
				console.Append("  " + line)
			}
		}
	}
	return err
}

func mergedEnvironment(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}
	out := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, replace := overrides[key]; replace {
				continue
			}
		}
		out = append(out, entry)
	}
	for key, value := range overrides {
		out = append(out, key+"="+value)
	}
	return out
}

type tailWriter struct {
	data []byte
	max  int
}

func (w *tailWriter) Write(p []byte) (int, error) {
	n := len(p)
	if w.max <= 0 {
		return n, nil
	}
	if len(p) >= w.max {
		w.data = append(w.data[:0], p[len(p)-w.max:]...)
		return n, nil
	}
	if overflow := len(w.data) + len(p) - w.max; overflow > 0 {
		copy(w.data, w.data[overflow:])
		w.data = w.data[:len(w.data)-overflow]
	}
	w.data = append(w.data, p...)
	return n, nil
}

func (w *tailWriter) String() string { return string(w.data) }

func failureTail(output string, maxLines int) []string {
	output = strings.ReplaceAll(output, "\r\n", "\n")
	output = strings.ReplaceAll(output, "\r", "\n")
	raw := strings.Split(output, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines
}

func displayCommand(exe string, args []string) string {
	parts := append([]string{filepath.Base(exe)}, args...)
	for i, part := range parts {
		if strings.ContainsAny(part, " \t\n\"'") {
			parts[i] = strconv.Quote(part)
		}
	}
	return strings.Join(parts, " ")
}

func Available(c config.Config) (bool, error) { _, ok, err := Detect(c); return ok, err }
func ManifestPath(c config.Config) string {
	for _, n := range []string{"setup.yaml", "setup.yml"} {
		p := filepath.Join(c.CurrentDir, n)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
func IsManifest(path string) bool {
	b := strings.ToLower(filepath.Base(path))
	return b == "setup.yaml" || b == "setup.yml"
}
