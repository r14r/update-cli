package updater

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// validateGitArguments permits only the documented git subcommands.
// Commit messages are positional arguments, unlike other update-cli commands.
func validateGitArguments(args []string) error {
	if len(args) == 0 {
		return errors.New("git-Unterbefehl fehlt: status, add, commit <message> oder push")
	}
	switch args[0] {
	case "status", "add", "push":
		if len(args) != 1 {
			return fmt.Errorf("git %s akzeptiert keine weiteren Argumente", args[0])
		}
	case "commit":
		if len(args) < 2 || strings.TrimSpace(strings.Join(args[1:], " ")) == "" {
			return errors.New("git commit benötigt eine Commit-Nachricht: update-cli git commit <message>")
		}
		for _, v := range args[1:] {
			if strings.HasPrefix(v, "-") {
				return errors.New("git commit erwartet eine Nachricht, keine Git-Optionen")
			}
		}
	default:
		return fmt.Errorf("unbekannter git-Unterbefehl %q; verfügbar: status, add, commit <message>, push", args[0])
	}
	return nil
}

// RunGit runs Git in the repository containing the current working directory.
// This also works from a project's current/ directory when it is a Git checkout.
func RunGit(ctx context.Context, args []string) error {
	if err := validateGitArguments(args); err != nil {
		return &ExitError{Code: 2, Err: err}
	}
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("git ist nicht installiert oder nicht im PATH")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	probe := exec.CommandContext(ctx, "git", "-C", cwd, "rev-parse", "--show-toplevel")
	root, err := probe.Output()
	if err != nil {
		return fmt.Errorf("kein Git-Repository im aktuellen Verzeichnis: %s", cwd)
	}
	directory := strings.TrimSpace(string(root))
	var gitArgs []string
	switch args[0] {
	case "status":
		gitArgs = []string{"status"}
	case "add":
		gitArgs = []string{"add", "-A"}
	case "commit":
		gitArgs = []string{"commit", "-m", strings.Join(args[1:], " ")}
	case "push":
		gitArgs = []string{"push"}
	}
	cmd := exec.CommandContext(ctx, "git", gitArgs...)
	cmd.Dir = directory
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var failed *exec.ExitError
		if errors.As(err, &failed) {
			return &ExitError{Code: failed.ExitCode(), Err: fmt.Errorf("git %s fehlgeschlagen", args[0])}
		}
		return fmt.Errorf("git %s: %w", args[0], err)
	}
	return nil
}
