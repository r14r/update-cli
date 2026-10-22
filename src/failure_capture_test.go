package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestCaptureCommandOutputPersistsFailureTranscript(t *testing.T) {
	t.Setenv("UPDATE_CLI_ERROR_LOG_DIR", t.TempDir())
	runErr, logPath, logErr := captureCommandOutput(func() error {
		fmt.Fprintln(os.Stdout, "stdout marker")
		fmt.Fprintln(os.Stderr, "stderr marker")
		return errors.New("simulated failure")
	})
	if logErr != nil {
		t.Fatal(logErr)
	}
	if runErr == nil || logPath == "" {
		t.Fatalf("expected saved failure transcript, got error=%v path=%q", runErr, logPath)
	}
	body, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"stdout marker", "stderr marker", "simulated failure"} {
		if !strings.Contains(string(body), value) {
			t.Errorf("failure log is missing %q", value)
		}
	}
}

func TestCaptureCommandOutputSuccessRemovesTranscript(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("UPDATE_CLI_ERROR_LOG_DIR", dir)
	runErr, logPath, logErr := captureCommandOutput(func() error {
		fmt.Fprintln(os.Stdout, "successful run")
		return nil
	})
	if runErr != nil || logErr != nil || logPath != "" {
		t.Fatalf("successful run: error=%v log=%v path=%q", runErr, logErr, logPath)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("successful execution left %d log files", len(entries))
	}
}

func TestStripTerminalSequences(t *testing.T) {
	got := stripTerminalSequences([]byte("\x1b[31mERROR\x1b[0m\r\nproblem\n"))
	if string(got) != "ERROR\nproblem\n" {
		t.Fatalf("unexpected clean log: %q", got)
	}
}

func TestShouldRecordFailure(t *testing.T) {
	for _, command := range []string{"update", "setup", "bootstrap", "build", "install", "--setup"} {
		if !shouldRecordFailure([]string{command, "--noui"}) {
			t.Errorf("should capture %s", command)
		}
	}
	if !shouldRecordFailure(nil) {
		t.Fatal("no-argument lifecycle should be captured")
	}
	for _, command := range []string{"version", "help", "whatsnew"} {
		if shouldRecordFailure([]string{command}) {
			t.Errorf("should not capture %s", command)
		}
	}
}
