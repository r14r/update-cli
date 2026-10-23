package editor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func Open(ctx context.Context, path string) (string, error) {
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v, runConfigured(ctx, v, path)
		}
	}
	for _, c := range []struct {
		name string
		args []string
	}{{"code", []string{"--wait", path}}, {"cursor", []string{"--wait", path}}, {"nano", []string{path}}, {"vim", []string{path}}, {"vi", []string{path}}} {
		if exe, err := exec.LookPath(c.name); err == nil {
			cmd := exec.CommandContext(ctx, exe, c.args...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return c.name, cmd.Run()
		}
	}
	return "", fmt.Errorf("kein Editor gefunden; VISUAL oder EDITOR setzen")
}
func runConfigured(ctx context.Context, c, path string) error {
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		shell = "/bin/sh"
	}
	cmd := exec.CommandContext(ctx, shell, "-lc", `exec `+c+` "$1"`, "update-cli-editor", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
