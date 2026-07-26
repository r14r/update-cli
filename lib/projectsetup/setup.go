package projectsetup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"release-updater/lib/config"
	"release-updater/lib/ui"
)

type Result struct {
	ScriptExecuted   bool
	CommandsExecuted int
}

// Run executes current/setup.sh first when present, followed by every command
// from config.setup.commands. Every process runs with current as working directory.
func Run(ctx context.Context, cfg config.Config, console *ui.Console) (Result, error) {
	info, err := os.Stat(cfg.CurrentDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{}, fmt.Errorf("Current-Ordner fehlt: %s; zuerst ein Release installieren", cfg.CurrentDir)
		}
		return Result{}, fmt.Errorf("Current-Ordner kann nicht geprüft werden: %w", err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("Current-Pfad ist kein Ordner: %s", cfg.CurrentDir)
	}

	bash, err := exec.LookPath("bash")
	if err != nil {
		return Result{}, errors.New("Projekt-Setup benötigt bash")
	}

	setupScript := filepath.Join(cfg.CurrentDir, "setup.sh")
	scriptExists := false
	if scriptInfo, statErr := os.Stat(setupScript); statErr == nil {
		if scriptInfo.IsDir() {
			return Result{}, fmt.Errorf("setup.sh ist ein Ordner: %s", setupScript)
		}
		scriptExists = true
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return Result{}, fmt.Errorf("setup.sh kann nicht geprüft werden: %w", statErr)
	}

	total := len(cfg.SetupCommands)
	if scriptExists {
		total++
	}

	console.Header("Projekt-Setup")
	console.Row("Projekt", cfg.ProjectName)
	console.Row("Arbeitsordner", cfg.CurrentDir)
	console.Row("Schritte", fmt.Sprintf("%d", total))

	if total == 0 {
		console.Warn("Kein current/setup.sh und keine setup.commands konfiguriert")
		return Result{}, nil
	}

	result := Result{}
	completed := 0
	if scriptExists {
		completed++
		console.Info(fmt.Sprintf("[%d/%d] setup.sh", completed, total))
		if err := run(ctx, cfg.CurrentDir, bash, "./setup.sh"); err != nil {
			return result, fmt.Errorf("setup.sh fehlgeschlagen: %w", err)
		}
		result.ScriptExecuted = true
		console.Success("setup.sh abgeschlossen")
	}

	for index, commandText := range cfg.SetupCommands {
		completed++
		console.Info(fmt.Sprintf("[%d/%d] setup.commands[%d]: %s", completed, total, index, displayCommand(commandText)))
		if err := run(ctx, cfg.CurrentDir, bash, "-c", commandText); err != nil {
			return result, fmt.Errorf("setup.commands[%d] fehlgeschlagen (%s): %w", index, commandText, err)
		}
		result.CommandsExecuted++
		console.Success(fmt.Sprintf("setup.commands[%d] abgeschlossen", index))
	}

	console.Header("Setup abgeschlossen")
	console.Row("setup.sh", yesNo(result.ScriptExecuted))
	console.Row("Kommandos", fmt.Sprintf("%d", result.CommandsExecuted))
	return result, nil
}

func run(ctx context.Context, workDir, executable string, args ...string) error {
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = workDir
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return err
	}
	return nil
}

func displayCommand(value string) string {
	value = strings.ReplaceAll(value, "\r", `\r`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return value
}

func yesNo(value bool) string {
	if value {
		return "ausgeführt"
	}
	return "nicht vorhanden"
}
