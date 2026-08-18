package projectdocker

import (
	"context"
	"os"
	"os/exec"
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

func TestRunningFailureIncludesCommandDirectoryExitAndOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "compose.yaml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := `#!/bin/sh
if [ "$1 $2" = "compose version" ]; then exit 0; fi
if echo "$*" | grep -q 'ps -q'; then
  echo 'compose stdout detail'
  echo 'daemon unavailable' >&2
  exit 17
fi
exit 0
`
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, err := Running(context.Background(), root)
	if err == nil {
		t.Fatal("expected compose status failure")
	}
	text := err.Error()
	for _, want := range []string{
		"Docker Compose Status fehlgeschlagen",
		"Kommando: docker compose -f compose.yaml ps -q",
		"Arbeitsverzeichnis: " + root,
		"Exit-Code: 17",
		"stderr:",
		"daemon unavailable",
		"stdout:",
		"compose stdout detail",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("detailed compose error missing %q:\n%s", want, text)
		}
	}
}

func TestComposeCommandReportsComposeVersionFailureDetails(t *testing.T) {
	oldLookPath, oldCommand := lookPath, commandContext
	defer func() { lookPath, commandContext = oldLookPath, oldCommand }()
	dir := t.TempDir()
	docker := filepath.Join(dir, "docker")
	if err := os.WriteFile(docker, []byte("#!/bin/sh\necho 'compose plugin broken' >&2\nexit 42\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	lookPath = func(name string) (string, error) {
		if name == "docker" {
			return docker, nil
		}
		return "", exec.ErrNotFound
	}
	_, _, err := composeCommand(context.Background(), dir)
	if err == nil {
		t.Fatal("expected compose command error")
	}
	text := err.Error()
	for _, want := range []string{"Docker Compose ist nicht verfügbar", "docker compose version", "Exit-Code: 42", "compose plugin broken"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
}
