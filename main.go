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

	"release-updater/lib/buildconfig"
	"release-updater/lib/updater"
)

//go:embed VERSION
var embeddedVersion string

//go:embed build-config.json
var embeddedBuildConfig []byte

// version may be overridden at build time with -ldflags "-X main.version=X.Y.Z".
var version = "dev"

func main() {
	buildDefaults, err := buildconfig.Parse(embeddedBuildConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nFEHLER  Ungültige eingebettete Build-Konfiguration: %v\n", err)
		os.Exit(1)
	}
	if err := buildconfig.Set(buildDefaults); err != nil {
		fmt.Fprintf(os.Stderr, "\nFEHLER  Build-Konfiguration kann nicht aktiviert werden: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := updater.Run(ctx, resolvedVersion(), os.Args[1:]); err != nil {
		var exitErr *updater.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.Err != nil {
				fmt.Fprintf(os.Stderr, "\nFEHLER  %v\n", exitErr.Err)
			}
			os.Exit(exitErr.Code)
		}

		fmt.Fprintf(os.Stderr, "\nFEHLER  %v\n", err)
		os.Exit(1)
	}
}

func resolvedVersion() string {
	candidate := strings.TrimSpace(version)
	if candidate != "" && candidate != "dev" && !strings.Contains(candidate, "{{") {
		return candidate
	}
	candidate = strings.TrimSpace(embeddedVersion)
	if candidate == "" {
		return "dev"
	}
	return candidate
}
