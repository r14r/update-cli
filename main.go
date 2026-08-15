package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/r14r/update-cli/lib/buildconfig"
	"github.com/r14r/update-cli/lib/updater"
)

//go:embed VERSION
var embeddedVersion string

//go:embed build-config.json
var embeddedBuildConfig []byte
var version = "dev"

func main() {
	c, err := buildconfig.Parse(embeddedBuildConfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR", err)
		os.Exit(1)
	}
	if err := buildconfig.Set(c); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	args := os.Args[1:]
	started := time.Now()
	if err := updater.Run(ctx, resolvedVersion(), args); err != nil {
		if handled, followErr := maybeKeepFailedSetupUpdate(ctx, resolvedVersion(), args, started, err); handled {
			if followErr == nil {
				return
			}
			err = followErr
		}
		var e *updater.ExitError
		if errors.As(err, &e) {
			if e.Err != nil {
				fmt.Fprintf(os.Stderr, "\nERROR  %v\n", e.Err)
			}
			os.Exit(e.Code)
		}
		fmt.Fprintf(os.Stderr, "\nERROR  %v\n", err)
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
