package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/r14r/update-cli/lib/config"
)

func dockerCheck(r Report) (Check, bool) {
	for _, c := range r.Checks {
		if c.Name == "Docker Compose" {
			return c, true
		}
	}
	return Check{}, false
}

func TestDoctorDockerLifecycleDisabledSkipsDocker(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{RootDir: root, CurrentDir: current, ReleaseRoot: filepath.Join(root, "release"), BackupRoot: filepath.Join(root, "backup"), Docker: config.DockerConfig{Lifecycle: "disabled"}}
	r := Run(context.Background(), root, cfg)
	c, ok := dockerCheck(r)
	if !ok || c.Level != LevelOK || c.Detail != "übersprungen; Docker-Lifecycle deaktiviert" {
		t.Fatalf("unexpected Docker check: %#v", c)
	}
}

func TestDoctorDockerLifecycleAutoUnavailableIsWarning(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	cfg := config.Config{RootDir: root, CurrentDir: current, ReleaseRoot: filepath.Join(root, "release"), BackupRoot: filepath.Join(root, "backup"), Docker: config.DockerConfig{Lifecycle: "auto"}}
	c, ok := dockerCheck(Run(context.Background(), root, cfg))
	if !ok || c.Level != LevelWarning {
		t.Fatalf("auto unavailable should warn: %#v", c)
	}
}

func TestDoctorDockerLifecycleRequiredUnavailableIsError(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	cfg := config.Config{RootDir: root, CurrentDir: current, ReleaseRoot: filepath.Join(root, "release"), BackupRoot: filepath.Join(root, "backup"), Docker: config.DockerConfig{Lifecycle: "required"}}
	c, ok := dockerCheck(Run(context.Background(), root, cfg))
	if !ok || c.Level != LevelError {
		t.Fatalf("required unavailable should error: %#v", c)
	}
}
