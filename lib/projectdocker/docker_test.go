package projectdocker

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDetectMissingCurrentIsNotDockerProject(t *testing.T) {
	result, err := Detect(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Detected {
		t.Fatalf("unexpected detection: %#v", result)
	}
}

func TestDetectComposeFile(t *testing.T) {
	root := t.TempDir()
	compose := filepath.Join(root, "compose.yaml")
	if err := os.WriteFile(compose, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Detected || result.ComposeFile != compose {
		t.Fatalf("unexpected detection: %#v", result)
	}
}

func TestStopUsesDockerComposeDown(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper is POSIX-specific")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "docker-compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	logFile := filepath.Join(t.TempDir(), "docker.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$PWD|$*\" > \"$DOCKER_TEST_LOG\"\n"
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("DOCKER_TEST_LOG", logFile)

	result, err := Stop(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Detected || !result.Stopped {
		t.Fatalf("unexpected result: %#v", result)
	}
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	logged := strings.TrimSpace(string(data))
	want := root + "|compose -f docker-compose.yml down --remove-orphans"
	if logged != want {
		t.Fatalf("unexpected docker invocation:\n got %q\nwant %q", logged, want)
	}
}

func TestStopFailsWhenComposeExistsButDockerIsUnavailable(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	_, err := Stop(context.Background(), root)
	if err == nil || !strings.Contains(err.Error(), "Update wird nicht gestartet") {
		t.Fatalf("expected safe abort, got %v", err)
	}
}
