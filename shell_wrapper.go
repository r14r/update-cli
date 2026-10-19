package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func installShellWrapper(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: update-cli wrapper (shell selected interactively)")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	defaultShell := filepath.Base(os.Getenv("SHELL"))
	fmt.Printf("Shell wählen [bash/zsh/csh] (Standard %s): ", defaultShell)
	reader := bufio.NewReader(os.Stdin)
	selection, err := reader.ReadString('\n')
	if err != nil && len(selection) == 0 {
		return fmt.Errorf("Shell-Auswahl erforderlich: %w", err)
	}
	shell := strings.ToLower(strings.TrimSpace(selection))
	if shell == "" {
		shell = defaultShell
	}
	if shell == "tcsh" {
		shell = "csh"
	}

	var startup, filename, body string
	switch shell {
	case "bash":
		startup, filename, body = filepath.Join(home, ".bashrc"), "wrapper-bash.sh", wrapperBash
	case "zsh":
		startup, filename, body = filepath.Join(home, ".zshrc"), "wrapper-zsh.sh", wrapperZsh
	case "csh":
		startup, filename, body = filepath.Join(home, ".cshrc"), "wrapper-csh.csh", wrapperCsh
	default:
		return fmt.Errorf("nicht unterstützte Shell %q (bash, zsh, csh/tcsh)", shell)
	}
	installDir := filepath.Join(home, ".local", "share", "update-cli", "wrappers")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return err
	}
	installed := filepath.Join(installDir, filename)
	if err := os.WriteFile(installed, []byte(body), 0644); err != nil {
		return err
	}

	markerStart := "# >>> update-cli wrapper >>>"
	markerEnd := "# <<< update-cli wrapper <<<"
	var entry string
	if shell == "csh" {
		entry = fmt.Sprintf("%s\nalias u 'source \"%s\" \\!*'\n%s\n", markerStart, installed, markerEnd)
	} else {
		entry = fmt.Sprintf("%s\nsource \"%s\"\n%s\n", markerStart, installed, markerEnd)
	}
	existing, err := os.ReadFile(startup)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(existing), markerStart) {
		fmt.Printf("Wrapper bereits in %s installiert (Skript aktualisiert).\n", startup)
		return nil
	}
	mode := os.FileMode(0644)
	if info, statErr := os.Stat(startup); statErr == nil {
		mode = info.Mode().Perm()
		backup := fmt.Sprintf("%s.bak.%s", startup, time.Now().Format("20060102-150405.000000000"))
		if err := os.WriteFile(backup, existing, mode); err != nil {
			return err
		}
		fmt.Println("Backup:", backup)
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	content := string(existing)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "\n" + entry
	if err := os.WriteFile(startup, []byte(content), mode); err != nil {
		return err
	}
	fmt.Println("Wrapper installiert:", startup)
	fmt.Printf("Shell neu starten oder Konfiguration laden: source %s\n", startup)
	return nil
}
