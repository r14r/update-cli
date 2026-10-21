package updater

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/ui"
)

// parseWorkflowArguments isolates CLI parsing from the lifecycle executor.
func parseWorkflowArguments(args []string) (root string, debug, noUI, pipeline, plan bool, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if value, ok := strings.CutPrefix(arg, "--root="); ok {
			if strings.TrimSpace(value) == "" {
				return "", false, false, false, false, fmt.Errorf("--root benötigt einen Pfad")
			}
			root = value
			continue
		}
		switch arg {
		case "--root", "-r":
			if i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" || strings.HasPrefix(args[i+1], "-") {
				return "", false, false, false, false, fmt.Errorf("--root benötigt einen gültigen Pfad")
			}
			i++
			root = args[i]
		case "--pipeline":
			pipeline = true
		case "--plan":
			plan = true
		case "--no-wait":
			// No prompts are shown between workflow phases.
		case "--debug":
			debug = true
		case "--noui", "--no-ui":
			noUI = true
		default:
			return "", false, false, false, false, fmt.Errorf("workflow unterstützt --root, --debug, --noui, --no-wait, --plan und --pipeline")
		}
	}
	return root, debug, noUI, pipeline, plan, nil
}

// workflowStages returns a fresh slice so callers cannot mutate the sequence.
func workflowStages() []string {
	return []string{"update", "bootstrap", "install", "precommit", "test", "build", "run"}
}

// deploymentStages performs quality checks before installation and verifies health after startup.
// Note: update activation remains governed by the existing transaction layer.
func deploymentStages() []string {
	return []string{"update", "bootstrap", "precommit", "test", "build", "install", "run", "healthcheck"}
}

func runWorkflowHealthcheck(ctx context.Context, root string) error {
	cfg, err := config.Load(root, "")
	if err != nil {
		return fmt.Errorf("Healthcheck-Konfiguration: %w", err)
	}
	if cfg.Healthcheck.Type == "" || cfg.Healthcheck.Type == "none" {
		return fmt.Errorf("Deployment-Workflow benötigt einen konfigurierten Healthcheck")
	}
	return runHealthcheck(ctx, cfg)
}

func runDefaultWorkflow(ctx context.Context, buildVersion, root string, debug, noUI bool) error {
	return executeWorkflow(ctx, buildVersion, root, debug, noUI, false)
}

func executeWorkflow(ctx context.Context, buildVersion, root string, debug, noUI, pipeline bool) (retErr error) {
	if pipeline {
		cfg, err := config.Load(root, "")
		if err != nil {
			return fmt.Errorf("Deployment-Konfiguration: %w", err)
		}
		if cfg.Healthcheck.Type == "" || cfg.Healthcheck.Type == "none" {
			return fmt.Errorf("Deployment-Pipeline benötigt einen konfigurierten Healthcheck (healthcheck.type)")
		}
	}
	stages := workflowStages()
	if pipeline {
		stages = deploymentStages()
	}
	console := ui.New(false)
	shared := !noUI && !debug && console.StartFullscreen(fmt.Sprintf("Update CLI Version %s — Workflow", buildVersion))
	if shared {
		ctx = context.WithValue(ctx, lifecycleUIContextKey{}, console)
		defer func() {
			if retErr != nil && !console.ErrorShown() {
				console.ErrorNotice("Workflow fehlgeschlagen", retErr.Error())
			}
			console.FinishFullscreen(retErr == nil, false)
		}()
	}
	for index, stage := range stages {
		title := fmt.Sprintf("Update CLI Version %s — Workflow [%d/%d] %s", buildVersion, index+1, len(stages), strings.ToUpper(stage))
		if shared {
			// Reuse one alternate screen. Changing the title resets the
			// previous phase's content while keeping the current step visible.
			console.StartFullscreen(title)
			console.SetFooter("RUN  " + stage)
		} else {
			fmt.Printf("\n==> Workflow [%d/%d]: %s\n", index+1, len(stages), stage)
		}
		args := []string{stage, "--root", root, "--no-wait"}
		if noUI || debug {
			args = append(args, "--noui")
		}
		if debug {
			args = append(args, "--debug")
		}
		var err error
		if stage == "healthcheck" {
			err = runWorkflowHealthcheck(ctx, root)
		} else {
			err = Run(ctx, buildVersion, args)
		}
		if err != nil {
			var already *VersionAlreadyInstalledError
			if stage == "update" && errors.As(err, &already) {
				if shared {
					console.Info("Bereits installiert: " + already.Error())
				} else {
					fmt.Printf("SKIP  %s; lokales Release bereits vorhanden\n", already.Error())
				}
				continue
			}
			return fmt.Errorf("Workflow-Phase %s fehlgeschlagen: %w", stage, err)
		}
	}
	return nil
}

