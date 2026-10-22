package updater

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/r14r/update-cli/lib/projectsetup"
)

// showWorkflowView lists the configured stages without executing them.
func showWorkflowView(root string, pipeline, details bool) error {
	stages := workflowStages()
	if pipeline {
		stages = deploymentStages()
	}
	var manifest *projectsetup.Manifest
	if details {
		for _, path := range []string{filepath.Join(root, "current", "update-cli.yaml"), filepath.Join(root, "update-cli.yaml")} {
			if _, err := os.Stat(path); err == nil {
				m, err := projectsetup.ParseManifest(path)
				if err != nil {
					return fmt.Errorf("Workflow-Manifest %s: %w", path, err)
				}
				manifest = &m
				break
			}
		}
	}
	for i, stage := range stages {
		fmt.Printf("[%d/%d] %s\n", i+1, len(stages), strings.ToUpper(stage))
		if !details {
			continue
		}
		fmt.Printf("       update-cli %s\n", stage)
		if stage == "healthcheck" {
			fmt.Println("       Konfigurierter Runtime-Healthcheck")
			continue
		}
		if stage == "install" {
			fmt.Println("       just install (native Installationsphase)")
			continue
		}
		if stage == "update" {
			fmt.Println("       Repository/Archiv abrufen und Release aktivieren")
			continue
		}
		if manifest == nil {
			fmt.Println("       Projektmanifest nicht gefunden; keine Task-Kommandos verfügbar")
			continue
		}
		if stage == "run" {
			if cmd := strings.TrimSpace(manifest.Run.Command); cmd != "" {
				fmt.Printf("       %s\n", cmd)
			}
			continue
		}
		w, ok := manifest.Workflows[stage]
		if !ok {
			fmt.Println("       Kein entsprechender Workflow im Manifest definiert")
			continue
		}
		for _, task := range w.Tasks {
			showWorkflowTask(manifest, task, map[string]bool{})
		}
	}
	return nil
}

func showWorkflowTask(m *projectsetup.Manifest, name string, seen map[string]bool) {
	if seen[name] {
		return
	}
	seen[name] = true
	t, ok := m.Tasks[name]
	if !ok {
		fmt.Printf("       Task %s ist nicht definiert\n", name)
		return
	}
	for _, req := range t.Requires {
		showWorkflowTask(m, req, seen)
	}
	for _, step := range t.Steps {
		op := step.Operation
		if val, ok := step.Config["value"]; ok {
			fmt.Printf("       %s: %v\n", op, val)
		} else {
			fmt.Printf("       %s: %v\n", op, step.Config)
		}
	}
}
