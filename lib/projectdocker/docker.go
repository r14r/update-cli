package projectdocker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = current
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("Docker Compose Status fehlgeschlagen: %w", err)
	}
	return strings.TrimSpace(string(out)) != "", nil
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
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = current
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return r, fmt.Errorf("Docker Compose %s fehlgeschlagen (%s): %w: %s", action, r.Command, runErr, detail)
		}
		return r, fmt.Errorf("Docker Compose %s fehlgeschlagen (%s): %w", action, r.Command, runErr)
	}
	r.Changed = true
	return r, nil
}
func composeCommand(ctx context.Context, dir string) (string, []string, error) {
	if d, err := exec.LookPath("docker"); err == nil {
		cmd := exec.CommandContext(ctx, d, "compose", "version")
		cmd.Dir = dir
		if cmd.Run() == nil {
			return d, []string{"compose"}, nil
		}
	}
	if d, err := exec.LookPath("docker-compose"); err == nil {
		return d, nil, nil
	}
	return "", nil, errors.New("Docker-Compose-Projekt erkannt, aber docker compose/docker-compose fehlt")
}
