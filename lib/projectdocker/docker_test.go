package projectdocker

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunningAndStart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "compose.yaml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "docker.log")
	script := `#!/bin/sh
if [ "$1 $2" = "compose version" ]; then exit 0; fi
printf '%s\n' "$*" >> "$DOCKER_TEST_LOG"
case "$*" in *"ps -q"*) printf 'container-id\n';; esac
`
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("DOCKER_TEST_LOG", log)
	running, err := Running(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !running {
		t.Fatal("expected running compose stack")
	}
	if _, err := Start(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if !strings.Contains(text, "compose -f compose.yaml ps -q") || !strings.Contains(text, "compose -f compose.yaml up -d --remove-orphans") {
		t.Fatalf("unexpected docker commands: %q", text)
	}
}
