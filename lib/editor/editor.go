package editor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Open starts an interactive editor for path and waits until the editor exits.
// VISUAL has priority over EDITOR. If neither is configured, common editors are
// detected in a deterministic order.
func Open(ctx context.Context, path string) (string, error) {
	if value := strings.TrimSpace(os.Getenv("VISUAL")); value != "" {
		return value, runConfigured(ctx, value, path)
	}
	if value := strings.TrimSpace(os.Getenv("EDITOR")); value != "" {
		return value, runConfigured(ctx, value, path)
	}

	candidates := []struct {
		name string
		args []string
	}{
		{name: "code", args: []string{"--wait", path}},
		{name: "cursor", args: []string{"--wait", path}},
		{name: "nano", args: []string{path}},
		{name: "vim", args: []string{path}},
		{name: "vi", args: []string{path}},
	}
	for _, candidate := range candidates {
		binary, err := exec.LookPath(candidate.name)
		if err != nil {
			continue
		}
		command := exec.CommandContext(ctx, binary, candidate.args...)
		command.Stdin = os.Stdin
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if err := command.Run(); err != nil {
			return candidate.name, fmt.Errorf("Editor %s wurde mit Fehler beendet: %w", candidate.name, err)
		}
		return candidate.name, nil
	}

	return "", fmt.Errorf("kein Editor gefunden; VISUAL oder EDITOR setzen, z. B. EDITOR='code --wait'")
}

func runConfigured(ctx context.Context, configured, path string) error {
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		shell = "/bin/sh"
	}
	if _, err := os.Stat(shell); err != nil {
		shell = "/bin/sh"
	}

	// The path is passed as positional parameter instead of interpolated into the
	// command string, so spaces and shell metacharacters in paths remain safe.
	command := exec.CommandContext(ctx, shell, "-lc", `exec `+configured+` "$1"`, "updater-editor", path)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("konfigurierter Editor %q wurde mit Fehler beendet: %w", configured, err)
	}
	return nil
}
