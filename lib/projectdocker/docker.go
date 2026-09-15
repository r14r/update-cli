package projectdocker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	lookPath             = exec.LookPath
	commandContext       = exec.CommandContext
	composeProbeTimeout  = 15 * time.Second
	composeStatusTimeout = 15 * time.Second
	composeActionTimeout = 2 * time.Minute
	composeWaitDelay     = 250 * time.Millisecond
)

var ComposeFiles = []string{"compose.yml", "compose.yaml", "docker-compose.yml", "docker-compose.yaml"}

type Detection struct {
	Detected    bool   `json:"detected"`
	ComposeFile string `json:"composeFile,omitempty"`
}
type Result struct {
	Detection
	Changed bool   `json:"changed"`
	Command string `json:"command,omitempty"`
}

func Detect(current string) (Detection, error) {
	i, err := os.Stat(current)
	if errors.Is(err, os.ErrNotExist) {
		return Detection{}, nil
	}
	if err != nil {
		return Detection{}, err
	}
	if !i.IsDir() {
		return Detection{}, fmt.Errorf("Current-Pfad ist kein Ordner: %s", current)
	}
	for _, n := range ComposeFiles {
		p := filepath.Join(current, n)
		i, e := os.Stat(p)
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return Detection{}, e
		}
		if i.IsDir() {
			return Detection{}, fmt.Errorf("Compose-Pfad ist ein Ordner: %s", p)
		}
		return Detection{Detected: true, ComposeFile: p}, nil
	}
	return Detection{}, nil
}
func Running(ctx context.Context, current string) (bool, error) {
	d, err := Detect(current)
	if err != nil {
		return false, err
	}
	if !d.Detected {
		return false, nil
	}
	exe, prefix, err := composeCommand(ctx, current)
	if err != nil {
		return false, err
	}
	args := append([]string{}, prefix...)
	args = append(args, "-f", filepath.Base(d.ComposeFile), "ps", "-q")
	statusCtx, cancel := context.WithTimeout(ctx, composeStatusTimeout)
	defer cancel()
	cmd := commandContext(statusCtx, exe, args...)
	cmd.WaitDelay = composeWaitDelay
	cmd.Dir = current
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		return false, commandFailureWithContext("Docker Compose Status fehlgeschlagen", exe, args, current, runErr, statusCtx.Err(), stdout.String(), stderr.String())
	}
	return strings.TrimSpace(stdout.String()) != "", nil
}
func commandFailureWithContext(summary, exe string, args []string, cwd string, runErr, ctxErr error, stdout, stderr string) error {
	err := commandFailure(summary, exe, args, cwd, runErr, stdout, stderr)
	if errors.Is(ctxErr, context.DeadlineExceeded) {
		return fmt.Errorf("%w\nTimeout: Docker Compose wurde wegen Zeitüberschreitung abgebrochen", err)
	}
	if errors.Is(ctxErr, context.Canceled) {
		return fmt.Errorf("%w\nAbbruch: Docker Compose wurde durch den aufrufenden Kontext beendet", err)
	}
	return err
}

func commandFailure(summary, exe string, args []string, cwd string, runErr error, stdout, stderr string) error {
	command := strings.TrimSpace(filepath.Base(exe) + " " + strings.Join(args, " "))
	exitCode := -1
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		exitCode = exitErr.ExitCode()
	}
	lines := []string{
		summary,
		"Kommando: " + command,
		"Arbeitsverzeichnis: " + cwd,
	}
	if exitCode >= 0 {
		lines = append(lines, fmt.Sprintf("Exit-Code: %d", exitCode))
	} else {
		lines = append(lines, "Fehler: "+runErr.Error())
	}
	if text := strings.TrimSpace(stderr); text != "" {
		lines = append(lines, "stderr:", text)
	}
	if text := strings.TrimSpace(stdout); text != "" {
		lines = append(lines, "stdout:", text)
	}
	if strings.TrimSpace(stderr) == "" && strings.TrimSpace(stdout) == "" && exitCode >= 0 {
		lines = append(lines, "Fehler: "+runErr.Error())
	}
	return errors.New(strings.Join(lines, "\n"))
}

func Stop(ctx context.Context, current string) (Result, error)  { return invoke(ctx, current, "down") }
func Start(ctx context.Context, current string) (Result, error) { return invoke(ctx, current, "up") }
func invoke(ctx context.Context, current, action string) (Result, error) {
	d, err := Detect(current)
	if err != nil {
		return Result{}, err
	}
	r := Result{Detection: d}
	if !d.Detected {
		return r, nil
	}
	exe, prefix, err := composeCommand(ctx, current)
	if err != nil {
		return r, err
	}
	name := filepath.Base(d.ComposeFile)
	args := append([]string{}, prefix...)
	args = append(args, "-f", name)
	if action == "down" {
		args = append(args, "down", "--remove-orphans")
	} else {
		args = append(args, "up", "-d", "--remove-orphans")
	}
	r.Command = filepath.Base(exe) + " " + strings.Join(args, " ")
	actionCtx, cancel := context.WithTimeout(ctx, composeActionTimeout)
	defer cancel()
	cmd := commandContext(actionCtx, exe, args...)
	cmd.WaitDelay = composeWaitDelay
	cmd.Dir = current
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		return r, commandFailureWithContext(fmt.Sprintf("Docker Compose %s fehlgeschlagen", action), exe, args, current, runErr, actionCtx.Err(), stdout.String(), stderr.String())
	}
	r.Changed = true
	return r, nil
}
func composeCommand(ctx context.Context, dir string) (string, []string, error) {
	var dockerComposeErr error
	if d, err := lookPath("docker"); err == nil {
		probeCtx, cancel := context.WithTimeout(ctx, composeProbeTimeout)
		cmd := commandContext(probeCtx, d, "compose", "version")
		cmd.WaitDelay = composeWaitDelay
		cmd.Dir = dir
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		runErr := cmd.Run()
		ctxErr := probeCtx.Err()
		cancel()
		if runErr == nil {
			return d, []string{"compose"}, nil
		}
		dockerComposeErr = commandFailureWithContext("Docker Compose ist nicht verfügbar", d, []string{"compose", "version"}, dir, runErr, ctxErr, stdout.String(), stderr.String())
	}
	if d, err := lookPath("docker-compose"); err == nil {
		return d, nil, nil
	}
	if dockerComposeErr != nil {
		return "", nil, dockerComposeErr
	}
	return "", nil, errors.New("Docker-Compose-Projekt erkannt, aber weder docker noch docker-compose ist verfügbar")
}
