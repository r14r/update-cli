package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/ui"
)

// captureCommandOutput transparently mirrors stdout/stderr and records the
// complete transcript on disk. Successful runs delete their temporary log.
func captureCommandOutput(run func() error) (runErr error, logPath string, logErr error) {
	directory := failureLogDirectory()
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return run(), "", err
	}
	logFile, err := os.CreateTemp(directory, "failure-*.log")
	if err != nil {
		return run(), "", err
	}
	path := logFile.Name()
	defer logFile.Close()

	originalOut, originalErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		_ = os.Remove(path)
		return run(), "", err
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		_ = outR.Close()
		_ = outW.Close()
		_ = os.Remove(path)
		return run(), "", err
	}

	// The output is piped for recording, but the original terminal remains
	// available on stdin. UI uses this marker for TTY and terminal dimensions.
	if terminalFile(originalOut) && terminalFile(os.Stdin) {
		old, existed := os.LookupEnv("UPDATE_CLI_CAPTURE_TTY")
		_ = os.Setenv("UPDATE_CLI_CAPTURE_TTY", "1")
		defer func() {
			if existed {
				_ = os.Setenv("UPDATE_CLI_CAPTURE_TTY", old)
			} else {
				_ = os.Unsetenv("UPDATE_CLI_CAPTURE_TTY")
			}
		}()
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	drain := func(reader, destination *os.File) {
		defer wg.Done()
		defer reader.Close()
		buf := make([]byte, 32*1024)
		for {
			n, readErr := reader.Read(buf)
			if n > 0 {
				_, _ = destination.Write(buf[:n])
				mu.Lock()
				_, _ = logFile.Write(buf[:n])
				mu.Unlock()
			}
			if readErr != nil {
				return
			}
		}
	}
	wg.Add(2)
	go drain(outR, originalOut)
	go drain(errR, originalErr)

	os.Stdout, os.Stderr = outW, errW
	func() {
		defer func() {
			if p := recover(); p != nil {
				runErr = fmt.Errorf("unexpected panic: %v", p)
			}
		}()
		runErr = run()
	}()
	os.Stdout, os.Stderr = originalOut, originalErr
	_ = outW.Close()
	_ = errW.Close()
	wg.Wait()

	if runErr == nil {
		_ = os.Remove(path)
		return nil, "", nil
	}
	if _, err := fmt.Fprintf(logFile, "\nERROR  %s\n", runErr); err != nil {
		return runErr, path, err
	}
	if err := logFile.Sync(); err != nil {
		return runErr, path, err
	}
	return runErr, path, nil
}

func failureLogDirectory() string {
	if override := strings.TrimSpace(os.Getenv("UPDATE_CLI_ERROR_LOG_DIR")); override != "" {
		return override
	}
	if root, err := config.ResolveRoot(""); err == nil {
		return filepath.Join(root, config.ConfigDirName, "logs")
	}
	directory, err := os.UserCacheDir()
	if err != nil {
		directory = os.TempDir()
	}
	return filepath.Join(directory, "update-cli", "logs")
}

func terminalFile(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// askCopyFailureLog offers an interactive modal after the error screen closes.
// In --noui mode the same question appears as a plain terminal prompt.
func askCopyFailureLog(path string) {
	if path == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "\nFailure log saved: %s\n", path)
	if !terminalFile(os.Stdin) {
		return
	}
	plain := false
	for _, arg := range os.Args[1:] {
		if arg == "--noui" || arg == "--no-ui" || arg == "-noui" {
			plain = true
			break
		}
	}
	console := ui.New(false)
	if !plain {
		if console.StartFullscreen("Update CLI — Fehlerprotokoll") {
			console.SetInfoTitle("Fehlerprotokoll")
			console.InfoRow("Logdatei", path)
		}
	}
	confirmed, err := console.Confirm("Copy Error Message to Clipboard?", false)
	if console.Fullscreen() {
		console.FinishFullscreen(false, false)
	}
	if err != nil || !confirmed {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot read error log:", err)
		return
	}
	// Terminal screen-control codes are useful for live rendering but must
	// not pollute issue reports or clipboard messages.
	data = stripTerminalSequences(data)
	if err := copyErrorLog(data); err != nil {
		fmt.Fprintln(os.Stderr, "Clipboard unavailable:", err)
		return
	}
	fmt.Fprintln(os.Stderr, "Error log copied to clipboard.")
}

var terminalEscapeSequence = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")

func stripTerminalSequences(data []byte) []byte {
	return terminalEscapeSequence.ReplaceAll(bytes.ReplaceAll(data, []byte("\r"), nil), nil)
}

func copyErrorLog(data []byte) error {
	var choices [][]string
	switch runtime.GOOS {
	case "darwin":
		choices = [][]string{{"pbcopy"}}
	case "windows":
		choices = [][]string{{"clip"}}
	default:
		choices = [][]string{
			{"wl-copy"},
			{"xclip", "-selection", "clipboard"},
			{"xsel", "--clipboard", "--input"},
		}
	}
	for _, args := range choices {
		executable, err := exec.LookPath(args[0])
		if err != nil {
			continue
		}
		cmd := exec.Command(executable, args[1:]...)
		cmd.Stdin = bytes.NewReader(data)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no usable clipboard command (pbcopy, wl-copy, xclip or xsel)")
}

func shouldRecordFailure(args []string) bool {
	if len(args) == 0 {
		return true
	}
	command := strings.TrimLeft(strings.ToLower(args[0]), "-")
	switch command {
	case "update", "setup", "bootstrap", "build", "install":
		return true
	default:
		return false
	}
}

