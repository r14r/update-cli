package rsync

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ChangeKind string

const (
	ChangeCreated ChangeKind = "created"
	ChangeUpdated ChangeKind = "updated"
	ChangeDeleted ChangeKind = "deleted"
	ChangeOther   ChangeKind = "other"
)

type Change struct {
	Kind ChangeKind `json:"kind"`
	Path string     `json:"path"`
	Raw  string     `json:"raw,omitempty"`
}

type Result struct {
	Changes int      `json:"changes"`
	LogFile string   `json:"logFile,omitempty"`
	Items   []Change `json:"items,omitempty"`
}

func Require() error {
	if _, err := exec.LookPath("rsync"); err != nil {
		return fmt.Errorf("erforderliches Programm fehlt: rsync")
	}
	return nil
}

func Release(ctx context.Context, source, destination, logFile string) (Result, error) {
	return run(ctx, source, destination, logFile, false, []string{
		"--exclude=/.git/",
		"--exclude=/.venv/",
		"--exclude=/.env",
		"--exclude=/__MACOSX/",
		"--exclude=/.DS_Store",
	})
}

func Current(ctx context.Context, source, destination, logFile string, dryRun bool) (Result, error) {
	return run(ctx, source, destination, logFile, dryRun, []string{
		"--filter=protect /.git/",
		"--filter=protect /.venv/",
		"--filter=protect /.env",
		"--exclude=/.git/",
		"--exclude=/.venv/",
		"--exclude=/.env",
	})
}

// Snapshot creates a restorable current snapshot while excluding regenerated dependencies.
func Snapshot(ctx context.Context, source, destination, logFile string, dryRun bool) (Result, error) {
	return run(ctx, source, destination, logFile, dryRun, []string{
		"--exclude=/.git/",
		"--exclude=/.venv/",
		"--exclude=/node_modules/",
		"--exclude=/vendor/",
		"--exclude=/dist/",
		"--exclude=/build/",
		"--exclude=/__pycache__/",
	})
}

// Restore synchronizes a backup into current while preserving local environment state.
func Restore(ctx context.Context, source, destination, logFile string, dryRun bool) (Result, error) {
	return run(ctx, source, destination, logFile, dryRun, []string{
		"--filter=protect /.git/",
		"--filter=protect /.venv/",
		"--filter=protect /.env",
		"--exclude=/.git/",
		"--exclude=/.venv/",
		"--exclude=/.env",
		"--exclude=/.backup.json",
	})
}

func run(
	ctx context.Context,
	source string,
	destination string,
	logFile string,
	dryRun bool,
	extra []string,
) (Result, error) {
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return Result{}, fmt.Errorf("rsync-Ziel kann nicht erstellt werden: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		return Result{}, fmt.Errorf("Log-Ordner kann nicht erstellt werden: %w", err)
	}

	arguments := []string{"-a", "--delete", "--checksum", "--itemize-changes"}
	if dryRun {
		arguments = append(arguments, "--dry-run")
	}
	arguments = append(arguments, extra...)
	arguments = append(arguments, withTrailingSeparator(source), withTrailingSeparator(destination))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, "rsync", arguments...)
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return Result{}, fmt.Errorf("rsync fehlgeschlagen: %s", message)
	}

	if err := os.WriteFile(logFile, stdout.Bytes(), 0o644); err != nil {
		return Result{}, fmt.Errorf("rsync-Log kann nicht geschrieben werden: %w", err)
	}

	items := ParseChanges(stdout.String())
	return Result{Changes: len(items), LogFile: logFile, Items: items}, nil
}

func ParseChanges(value string) []Change {
	result := make([]Change, 0)
	scanner := bufio.NewScanner(strings.NewReader(value))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "*deleting") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "*deleting"))
			result = append(result, Change{Kind: ChangeDeleted, Path: path, Raw: line})
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			result = append(result, Change{Kind: ChangeOther, Path: line, Raw: line})
			continue
		}
		code := fields[0]
		path := strings.Join(fields[1:], " ")
		kind := ChangeUpdated
		if strings.Contains(code, "+++++++++") {
			kind = ChangeCreated
		} else if code == ".d..t......" || code == ".f..t......" {
			kind = ChangeOther
		}
		result = append(result, Change{Kind: kind, Path: path, Raw: line})
	}
	return result
}

func withTrailingSeparator(path string) string {
	if strings.HasSuffix(path, string(os.PathSeparator)) {
		return path
	}
	return path + string(os.PathSeparator)
}

// Describe returns the first line of rsync --version for diagnostic output.
func Describe() (string, error) {
	path, err := exec.LookPath("rsync")
	if err != nil {
		return "", fmt.Errorf("erforderliches Programm fehlt: rsync")
	}
	output, err := exec.Command(path, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("rsync-Version kann nicht ermittelt werden: %w", err)
	}
	line := strings.TrimSpace(strings.SplitN(string(output), "\n", 2)[0])
	if line == "" {
		line = path
	}
	return line, nil
}
