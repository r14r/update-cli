package projectdocker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ComposeFiles contains the standard Docker Compose filenames checked in the
// current project root. The first matching file is used.
var ComposeFiles = []string{
	"compose.yml",
	"compose.yaml",
	"docker-compose.yml",
	"docker-compose.yaml",
}

// Detection describes whether current is a Docker Compose project.
type Detection struct {
	Detected    bool   `json:"detected"`
	ComposeFile string `json:"composeFile,omitempty"`
}

// Result describes a completed Docker shutdown operation.
type Result struct {
	Detection
	Stopped bool   `json:"stopped"`
	Command string `json:"command,omitempty"`
}

// Detect checks only the current project root for a standard Compose file.
// A missing current directory is valid during an initial installation.
func Detect(currentDir string) (Detection, error) {
	info, err := os.Stat(currentDir)
	if errors.Is(err, os.ErrNotExist) {
		return Detection{}, nil
	}
	if err != nil {
		return Detection{}, fmt.Errorf("Current-Ordner kann für Docker-Prüfung nicht gelesen werden: %w", err)
	}
	if !info.IsDir() {
		return Detection{}, fmt.Errorf("Current-Pfad ist kein Ordner: %s", currentDir)
	}

	for _, name := range ComposeFiles {
		path := filepath.Join(currentDir, name)
		entry, statErr := os.Stat(path)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return Detection{}, fmt.Errorf("Docker-Compose-Datei kann nicht geprüft werden: %w", statErr)
		}
		if entry.IsDir() {
			return Detection{}, fmt.Errorf("Docker-Compose-Pfad ist ein Ordner: %s", path)
		}
		return Detection{Detected: true, ComposeFile: path}, nil
	}
	return Detection{}, nil
}

// Stop detects a Docker Compose project and stops it before update-cli changes
// release or current. If Compose is detected, a missing or failing Docker
// command aborts the update rather than modifying a running project.
func Stop(ctx context.Context, currentDir string) (Result, error) {
	detection, err := Detect(currentDir)
	if err != nil {
		return Result{}, err
	}
	result := Result{Detection: detection}
	if !detection.Detected {
		return result, nil
	}

	composeName := filepath.Base(detection.ComposeFile)
	if docker, lookupErr := exec.LookPath("docker"); lookupErr == nil && supportsComposePlugin(ctx, currentDir, docker) {
		args := []string{"compose", "-f", composeName, "down", "--remove-orphans"}
		result.Command = "docker " + strings.Join(args, " ")
		if err := run(ctx, currentDir, docker, args...); err != nil {
			return result, fmt.Errorf("Docker-Container konnten nicht gestoppt werden (%s): %w", result.Command, err)
		}
		result.Stopped = true
		return result, nil
	}

	if legacy, lookupErr := exec.LookPath("docker-compose"); lookupErr == nil {
		args := []string{"-f", composeName, "down", "--remove-orphans"}
		result.Command = "docker-compose " + strings.Join(args, " ")
		if err := run(ctx, currentDir, legacy, args...); err != nil {
			return result, fmt.Errorf("Docker-Container konnten nicht gestoppt werden (%s): %w", result.Command, err)
		}
		result.Stopped = true
		return result, nil
	}

	return result, fmt.Errorf(
		"Docker-Compose-Projekt erkannt (%s), aber weder docker noch docker-compose ist verfügbar; Update wird nicht gestartet",
		detection.ComposeFile,
	)
}

func supportsComposePlugin(ctx context.Context, workDir, docker string) bool {
	command := exec.CommandContext(ctx, docker, "compose", "version")
	command.Dir = workDir
	command.Stdout = nil
	command.Stderr = nil
	return command.Run() == nil
}

func run(ctx context.Context, workDir, executable string, args ...string) error {
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = workDir
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}
