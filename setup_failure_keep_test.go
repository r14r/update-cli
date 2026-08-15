package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/history"
)

func TestRetryUpdateWithoutSetupArgs(t *testing.T) {
	got := retryUpdateWithoutSetupArgs([]string{
		"--update", "/tmp/demo-v2.0.0.zip", "--root", "/srv/demo", "--setup", "--backup", "--no-wait",
	}, "/ignored")
	want := []string{
		"--update", "--no-setup", "/tmp/demo-v2.0.0.zip", "--root", "/srv/demo", "--no-wait",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("retry args mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestRetryUpdateWithoutSetupArgsFromCheck(t *testing.T) {
	got := retryUpdateWithoutSetupArgs([]string{"--check", "--no-wait"}, "/srv/demo")
	want := []string{"--update", "--no-setup", "--no-wait", "--root", "/srv/demo"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("retry args mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestAutomatedEnvironment(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	if automatedEnvironment() {
		t.Fatal("empty automation markers must be interactive")
	}
	t.Setenv("CI", "true")
	if !automatedEnvironment() {
		t.Fatal("CI=true must be detected as automated")
	}
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "true")
	if !automatedEnvironment() {
		t.Fatal("GITHUB_ACTIONS=true must be detected as automated")
	}
}

func TestRecentFailedSetupUpdate(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if err := history.Append(cfg.HistoryFile, history.Entry{
		Timestamp:   started.Add(time.Second),
		Action:      "update",
		Phase:       "setup",
		ProjectName: "demo",
		FromVersion: "1.0.0",
		ToVersion:   "2.0.0",
		Status:      "failed",
		Message:     "setup failed",
	}); err != nil {
		t.Fatal(err)
	}

	loaded, entry, ok := recentFailedSetupUpdate([]string{"--root", root}, started)
	if !ok {
		t.Fatal("expected recent setup failure to be detected")
	}
	if loaded.RootDir != cfg.RootDir {
		t.Fatalf("root mismatch: got %q want %q", loaded.RootDir, cfg.RootDir)
	}
	if entry.Phase != "setup" || entry.Status != "failed" || entry.ToVersion != "2.0.0" {
		t.Fatalf("unexpected history entry: %#v", entry)
	}
}

func TestRecentFailedSetupUpdateIgnoresStaleHistory(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if err := history.Append(cfg.HistoryFile, history.Entry{
		Timestamp:   started.Add(-10 * time.Second),
		Action:      "update",
		Phase:       "setup",
		ProjectName: "demo",
		Status:      "failed",
		Message:     "old setup failure",
	}); err != nil {
		t.Fatal(err)
	}

	if _, _, ok := recentFailedSetupUpdate([]string{"--root", root}, started); ok {
		t.Fatal("stale setup failure must not trigger keep-update prompt")
	}
}
