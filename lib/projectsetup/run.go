package projectsetup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/ui"
)

// RunApplication executes the application run definition declared in update-cli.yaml.
// The run definition always runs from the active project release (current/) unless a
// contained relative cwd is configured in the manifest or on an individual step.
func RunApplication(ctx context.Context, c config.Config, console *ui.Console) error {
	return RunApplicationInDirectory(ctx, c.CurrentDir, console)
}

// RunApplicationInDirectory is the standalone variant used when no Update CLI
// project configuration exists yet and update-cli.yaml is in the current root.
func RunApplicationInDirectory(ctx context.Context, root string, console *ui.Console) error {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("Projektordner fehlt oder ist ungültig: %s", root)
	}
	manifestPath, ok, err := FindManifest(root)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("kein update-cli.yaml in %s", root)
	}
	manifest, err := ParseManifest(manifestPath)
	if err != nil {
		return err
	}
	if manifest.Version != 2 {
		return errors.New("--run benötigt update-cli.yaml schemaVersion 2")
	}
	if strings.TrimSpace(manifest.Run.Command) == "" && len(manifest.Run.Steps) == 0 {
		return errors.New("update-cli.yaml enthält keine run-Konfiguration")
	}

	vars := resolveVariables(root, manifest)
	console.Header("Anwendung starten")
	console.Row("Manifest", manifestPath)
	if manifest.Run.Description != "" {
		console.Row("Beschreibung", expandTemplate(manifest.Run.Description, vars))
	}

	if strings.TrimSpace(manifest.Run.Command) != "" {
		return runApplicationCommand(ctx, root, manifest, vars, console)
	}
	return runApplicationSteps(ctx, root, manifest, vars, console)
}

func runApplicationCommand(ctx context.Context, root string, manifest Manifest, vars map[string]string, console *ui.Console) error {
	command := expandTemplate(manifest.Run.Command, vars)
	cwd := expandTemplate(manifest.Run.WorkingDirectory, vars)
	work, err := setupWorkDir(root, cwd)
	if err != nil {
		return fmt.Errorf("run.cwd ungültig: %w", err)
	}
	env := make(map[string]string, len(manifest.Run.Environment)+1)
	for key, value := range manifest.Run.Environment {
		env[key] = expandTemplate(value, vars)
	}
	env["UPDATE_CLI_RUN_RUNNING"] = "1"

	shell, err := exec.LookPath("bash")
	if err != nil {
		shell, err = exec.LookPath("sh")
		if err != nil {
			return errors.New("--run benötigt bash oder sh")
		}
	}

	console.Row("Arbeitsordner", work)
	console.Row("Kommando", command)

	cmd := exec.CommandContext(ctx, shell, "-c", command)
	cmd.Dir = work
	cmd.Env = mergedEnvironment(os.Environ(), env)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("Anwendung beendet mit Exit-Code %d: %w", exitErr.ExitCode(), err)
		}
		return err
	}
	return nil
}

func runApplicationSteps(ctx context.Context, root string, manifest Manifest, vars map[string]string, console *ui.Console) error {
	total := len(manifest.Run.Steps)
	console.Row("Schritte", fmt.Sprintf("%d", total))
	for i, original := range manifest.Run.Steps {
		step := original
		if step.WorkingDirectory == "" {
			step.WorkingDirectory = manifest.Run.WorkingDirectory
		}
		step.Environment = mergeStringMap(manifest.Run.Environment, step.Environment)
		step.Environment["UPDATE_CLI_RUN_RUNNING"] = "1"
		step = expandStepV2(step, vars)

		run, reason, err := evaluateCondition(root, step.When)
		if err != nil {
			return fmt.Errorf("run step %d (%s) Bedingung ungültig: %w", i+1, step.Name, err)
		}
		if !run {
			console.SkipStep(i, total, step.Name, reason)
			continue
		}

		err = console.Step(ctx, i, total, step.Name, func() error {
			return executeV2Step(ctx, root, step, manifest.Defaults, console)
		})
		if err != nil {
			if step.AllowFailure || !manifest.Defaults.FailFast {
				console.Warn(fmt.Sprintf("Run-Schritt fehlgeschlagen, wird fortgesetzt: %v", err))
				continue
			}
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return fmt.Errorf("Anwendung beendet mit Exit-Code %d: %w", exitErr.ExitCode(), err)
			}
			return fmt.Errorf("run step %d (%s) fehlgeschlagen: %w", i+1, step.Name, err)
		}
	}
	return nil
}

func mergeStringMap(base, override map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(override))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range override {
		out[key] = value
	}
	return out
}
