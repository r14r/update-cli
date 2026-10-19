package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/r14r/update-cli/lib/buildconfig"
	"github.com/r14r/update-cli/lib/ui"
	"github.com/r14r/update-cli/lib/updater"
)

//go:embed VERSION
var embeddedVersion string

//go:embed build-config.json
var embeddedBuildConfig []byte

func main() {
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
			var unsafe *updater.UnsafeWorkingDirectoryError
			if !errors.As(runErr, &unsafe) {
				askCopyFailureLog(logPath)
			}
		}
	} else {
		runErr = run()
	}
	if err := runErr; err != nil {
		var unsafe *updater.UnsafeWorkingDirectoryError
		if errors.As(err, &unsafe) {
			printUnsafeUpdateError(unsafe)
			os.Exit(1)
		}
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

func printUnsafeUpdateError(e *updater.UnsafeWorkingDirectoryError) {
	fmt.Fprintln(os.Stderr, "Update fehlgeschlagen: "+e.Error())
	fmt.Fprintln(os.Stderr)
	if terminalFile(os.Stderr) {
		fmt.Fprintln(os.Stderr, "\x1b[42;97mTODO\x1b[K\x1b[0m")
	} else {
		fmt.Fprintln(os.Stderr, "TODO")
	}
	fmt.Fprintf(os.Stderr, "In der Shell zuerst 'cd %s' ausführen\n", filepath.Clean(e.ProjectRoot))
	fmt.Fprintln(os.Stderr, "dann 'update-cli update'")
	fmt.Fprintln(os.Stderr, "und anschließend 'cd current'")
}
