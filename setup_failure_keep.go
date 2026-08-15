package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/history"
	"github.com/r14r/update-cli/lib/ui"
	"github.com/r14r/update-cli/lib/updater"
)

const setupFailureHistoryGrace = 2 * time.Second

func maybeKeepFailedSetupUpdate(ctx context.Context, buildVersion string, args []string, started time.Time, originalErr error) (bool, error) {
	if ctx.Err() != nil || hasCLIFlag(args, "--json") || automatedEnvironment() {
		return false, nil
	}

	cfg, entry, ok := recentFailedSetupUpdate(args, started)
	if !ok {
		return false, nil
	}

	console := ui.New(hasCLIFlag(args, "--no-color"))
	if !console.Interactive() {
		return false, nil
	}

	fullscreen := false
	if !hasCLIFlag(args, "--no-ui") {
		fullscreen = console.StartFullscreen("Update CLI — Setup fehlgeschlagen")
	}
	if fullscreen {
		console.SetInfoTitle("Projekt-Setup fehlgeschlagen")
		console.InfoRow("Projekt", entry.ProjectName)
		console.InfoRow("Version", emptyVersion(entry.FromVersion)+" → "+emptyVersion(entry.ToVersion))
		console.InfoRow("Current", cfg.CurrentDir)
		console.SetFooter("WARN Projekt-Setup fehlgeschlagen")
	} else {
		console.Header("Projekt-Setup fehlgeschlagen")
		console.Row("Projekt", entry.ProjectName)
		console.Row("Version", emptyVersion(entry.FromVersion)+" → "+emptyVersion(entry.ToVersion))
		console.Row("Current", cfg.CurrentDir)
	}
	if strings.TrimSpace(entry.Message) != "" {
		console.Warn(entry.Message)
	}

	keep, confirmErr := console.Confirm("Update trotz fehlgeschlagenem Setup behalten?", false)
	if fullscreen {
		console.FinishFullscreen(keep, false)
	}
	if confirmErr != nil {
		return true, fmt.Errorf("Bestätigung nach Setup-Fehler fehlgeschlagen: %w", confirmErr)
	}
	if !keep {
		return false, nil
	}

	retryArgs := retryUpdateWithoutSetupArgs(args, cfg.RootDir)
	fmt.Fprintln(os.Stdout, "INFO  Update wird ohne Projekt-Setup erneut installiert")
	if retryErr := updater.Run(ctx, buildVersion, retryArgs); retryErr != nil {
		return true, fmt.Errorf("%v; Update konnte anschließend nicht ohne Setup beibehalten werden: %w", originalErr, retryErr)
	}
	fmt.Fprintln(os.Stdout, "OK    Update wurde trotz fehlgeschlagenem Setup beibehalten")
	return true, nil
}

func automatedEnvironment() bool {
	return strings.TrimSpace(os.Getenv("CI")) != "" || strings.TrimSpace(os.Getenv("GITHUB_ACTIONS")) != ""
}

func recentFailedSetupUpdate(args []string, started time.Time) (config.Config, history.Entry, bool) {
	root, err := config.ResolveRoot(rootArgument(args))
	if err != nil {
		return config.Config{}, history.Entry{}, false
	}
	cfg, err := config.Load(root, "")
	if err != nil {
		return config.Config{}, history.Entry{}, false
	}
	entries, err := history.List(cfg.HistoryFile, 1)
	if err != nil || len(entries) == 0 {
		return config.Config{}, history.Entry{}, false
	}
	entry := entries[0]
	if entry.Timestamp.Before(started.Add(-setupFailureHistoryGrace)) {
		return config.Config{}, history.Entry{}, false
	}
	if entry.Action != "update" || entry.Phase != "setup" || entry.Status != "failed" {
		return config.Config{}, history.Entry{}, false
	}
	return cfg, entry, true
}

func retryUpdateWithoutSetupArgs(args []string, root string) []string {
	out := []string{"--update", "--no-setup"}
	for _, arg := range args {
		switch arg {
		case "--update", "--check", "--setup", "--no-setup", "--no-ask", "--json", "--plan", "--dry-run", "-n", "--details", "--backup":
			continue
		default:
			out = append(out, arg)
		}
	}
	if !hasRootArgument(args) {
		out = append(out, "--root", root)
	}
	return out
}

func rootArgument(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--root" || arg == "-r" {
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		}
		if strings.HasPrefix(arg, "--root=") {
			return strings.TrimPrefix(arg, "--root=")
		}
		if strings.HasPrefix(arg, "-r=") {
			return strings.TrimPrefix(arg, "-r=")
		}
	}
	return ""
}

func hasRootArgument(args []string) bool {
	for _, arg := range args {
		if arg == "--root" || arg == "-r" || strings.HasPrefix(arg, "--root=") || strings.HasPrefix(arg, "-r=") {
			return true
		}
	}
	return false
}

func hasCLIFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag || strings.HasPrefix(arg, flag+"=") {
			return true
		}
	}
	return false
}

func emptyVersion(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
