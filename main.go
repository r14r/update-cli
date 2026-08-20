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
	"os/signal"
	"strings"
	"syscall"
)

//go:embed VERSION
var embeddedVersion string

//go:embed build-config.json
var embeddedBuildConfig []byte
var version = "dev"

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
	if err := updater.Run(ctx, resolvedVersion(), os.Args[1:]); err != nil {
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
	v := strings.TrimSpace(version)
	if v != "" && v != "dev" && !strings.Contains(v, "{{") {
		return v
	}
	v = strings.TrimSpace(embeddedVersion)
	if v == "" {
		return "dev"
	}
	return v
}
