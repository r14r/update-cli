package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"github.com/r14r/update-cli/lib/buildconfig"
	"github.com/r14r/update-cli/lib/ui"
	"github.com/r14r/update-cli/lib/updater"
	"os"
	"bufio"
	"path/filepath"
	"time"
	"os/signal"
	"strings"
	"syscall"
)

//go:embed scripts/wrapper-bash.sh
var wrapperBash string

//go:embed scripts/wrapper-zsh.sh
var wrapperZsh string

//go:embed scripts/wrapper-csh.csh
var wrapperCsh string

//go:embed VERSION
var embeddedVersion string

//go:embed build-config.json
var embeddedBuildConfig []byte

func main() {
	if len(os.Args) > 1 && os.Args[1] == "wrapper" {
		if err := installShellWrapper(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR ", err)
			os.Exit(1)
		}
		return
	}

	c, err := buildconfig.Parse(embeddedBuildConfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR", ui.DisplayText(err.Error()))
		os.Exit(1)
	}
	if err := buildconfig.Set(c); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR", ui.DisplayText(err.Error()))
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := updater.ValidatePublicSyntax(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "\nERROR  %s\n", ui.DisplayText(err.Error()))
		os.Exit(2)
	}
	run := func() error { return updater.Run(ctx, resolvedVersion(), os.Args[1:]) }
	var runErr error
	if shouldRecordFailure(os.Args[1:]) {
		var logPath string
		var captureErr error
		runErr, logPath, captureErr = captureCommandOutput(run)
		if captureErr != nil {
			fmt.Fprintln(os.Stderr, "WARN  Error log could not be saved:", captureErr)
		}
		if runErr != nil {
			askCopyFailureLog(logPath)
		}
	} else {
		runErr = run()
	}
	if err := runErr; err != nil {
		var e *updater.ExitError
		if errors.As(err, &e) {
			if e.Err != nil {
				fmt.Fprintf(os.Stderr, "\nERROR  %s\n", ui.DisplayText(e.Err.Error()))
			}
			os.Exit(e.Code)
		}
		fmt.Fprintf(os.Stderr, "\nERROR  %s\n", ui.DisplayText(err.Error()))
		os.Exit(1)
	}
}
func resolvedVersion() string {
	// VERSION is the single canonical source for the update-cli release version.
	if v := strings.TrimSpace(embeddedVersion); v != "" && !strings.Contains(v, "{{") {
		return v
	}
	return "dev"
}

func installShellWrapper(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("Usage: update-cli wrapper (shell selected interactively)")
	}
	home, err := os.UserHomeDir()
	if err != nil { return err }
	defaultShell := filepath.Base(os.Getenv("SHELL"))
	fmt.Printf("Shell wählen [bash/zsh/csh] (Standard %s): ", defaultShell)
	reader := bufio.NewReader(os.Stdin)
	selection, err := reader.ReadString('\n')
	if err != nil && len(selection) == 0 { return fmt.Errorf("Shell-Auswahl erforderlich: %w", err) }
	shell := strings.ToLower(strings.TrimSpace(selection))
	if shell == "" { shell = defaultShell }
	if shell == "tcsh" { shell = "csh" }
	var startup, filename, body string
	switch shell {
	case "bash": startup, filename, body = filepath.Join(home, ".bashrc"), "wrapper-bash.sh", wrapperBash
	case "zsh": startup, filename, body = filepath.Join(home, ".zshrc"), "wrapper-zsh.sh", wrapperZsh
	case "csh": startup, filename, body = filepath.Join(home, ".cshrc"), "wrapper-csh.csh", wrapperCsh
	default: return fmt.Errorf("Nicht unterstützte Shell %q (bash, zsh, csh/tcsh)", shell)
	}
	installDir := filepath.Join(home, ".local", "share", "update-cli", "wrappers")
	if err := os.MkdirAll(installDir, 0755); err != nil { return err }
	installed := filepath.Join(installDir, filename)
	if err := os.WriteFile(installed, []byte(body), 0644); err != nil { return err }
	markerStart := "# >>> update-cli wrapper >>>"
	markerEnd := "# <<< update-cli wrapper <<<"
	var entry string
	if shell == "csh" {
		entry = fmt.Sprintf("%s\nalias u 'source \"%s\" \\!*'\n%s\n", markerStart, installed, markerEnd)
	} else {
		entry = fmt.Sprintf("%s\nsource \"%s\"\n%s\n", markerStart, installed, markerEnd)
	}
	existing, err := os.ReadFile(startup)
	if err != nil && !os.IsNotExist(err) { return err }
	if strings.Contains(string(existing), markerStart) {
		fmt.Printf("Wrapper bereits in %s installiert (Skript aktualisiert).\n", startup)
		return nil
	}
	if len(existing) > 0 {
		backup := fmt.Sprintf("%s.bak.%s", startup, time.Now().Format("20060102-150405.000000000"))
		info, statErr := os.Stat(startup)
		if statErr != nil { return statErr }
		if err := os.WriteFile(backup, existing, info.Mode().Perm()); err != nil { return err }
		fmt.Println("Backup:", backup)
	}
	content := string(existing)
	if content != "" && !strings.HasSuffix(content, "\n") { content += "\n" }
	content += "\n" + entry
	mode := os.FileMode(0644)
	if info, statErr := os.Stat(startup); statErr == nil { mode = info.Mode().Perm() }
	if err := os.WriteFile(startup, []byte(content), mode); err != nil { return err }
	fmt.Println("Wrapper installiert:", startup)
	fmt.Printf("Shell neu starten oder Konfiguration laden: source %s\n", startup)
	return nil
}
