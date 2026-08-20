package updater

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/history"
)

func releaseZip(t *testing.T, dir, project, version string, files map[string]string) string {
	t.Helper()
	p := filepath.Join(dir, project+"-v"+version+".zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	all := map[string]string{"VERSION": version}
	for k, v := range files {
		all[k] = v
	}
	for name, body := range all {
		e, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFailedSetupRestoresPreviousCurrentAndPersistentData(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	v1 := releaseZip(t, downloads, "demo", "1.0.0", map[string]string{"app.txt": "old"})
	if err := Run(context.Background(), "3.0.0", []string{"--update", v1, "--root", root, "--no-setup"}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.CurrentDir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.CurrentDir, "data", "db.txt"), []byte("user-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.CurrentDir, ".env"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	v2 := releaseZip(t, downloads, "demo", "2.0.0", map[string]string{"app.txt": "new", "data/db.txt": "release-data", "update-cli.yaml": "version: 1\nsteps:\n  - name: fail\n    type: command\n    command: exit 17\n"})
	err = Run(context.Background(), "3.0.0", []string{"--update", v2, "--root", root, "--setup"})
	if err == nil {
		t.Fatal("expected setup failure")
	}
	b, _ := os.ReadFile(filepath.Join(cfg.CurrentDir, "app.txt"))
	if string(b) != "old" {
		t.Fatalf("application not rolled back: %q", b)
	}
	b, _ = os.ReadFile(filepath.Join(cfg.CurrentDir, "data", "db.txt"))
	if string(b) != "user-data" {
		t.Fatalf("persistent data not restored: %q", b)
	}
	b, _ = os.ReadFile(filepath.Join(cfg.CurrentDir, ".env"))
	if string(b) != "secret" {
		t.Fatalf("env not restored: %q", b)
	}
	if _, statErr := os.Stat(filepath.Join(cfg.ReleaseRoot, "2.0.0")); !os.IsNotExist(statErr) {
		t.Fatalf("failed release was activated: %v", statErr)
	}
	entries, err := history.List(cfg.HistoryFile, 10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if e.Action == "update" && e.ToVersion == "2.0.0" && e.Status == "failed" && e.Phase == "setup" {
			found = true
		}
	}
	if !found {
		t.Fatalf("setup failure not recorded: %#v", entries)
	}
}

func TestVerifyArchiveReportsActualPath(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	p := releaseZip(t, downloads, "demo", "1.0.0", map[string]string{"app.txt": "x"})
	res, err := verifyArchive(context.Background(), cfg, p)
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(p)
	if res.ArchivePath != abs {
		t.Fatalf("archive path mismatch: got %q want %q", res.ArchivePath, abs)
	}
	if !strings.HasSuffix(res.ArchivePath, "demo-v1.0.0.zip") {
		t.Fatal(res.ArchivePath)
	}
}

func TestRunNoParameterDoesNotMaskInvalidExistingConfig(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	dir := filepath.Join(root, config.ConfigDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{
  "schemaVersion": 6,
  "projectName": "demo",
  "source": {"type": "download", "folder": "` + downloads + `"},
  "releaseDir": "release",
  "currentDir": "current",
  "no parameter": ["bogus"]
}`
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()

	err = runNoParameter(context.Background(), "3.0.3")
	if err == nil || !strings.Contains(err.Error(), "no parameter") {
		t.Fatalf("expected invalid config error, got %v", err)
	}
}

func TestNoParameterCheckSetupRunsCheckWithoutModeCollision(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	dir := filepath.Join(root, config.ConfigDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"schemaVersion":6,"projectName":"demo","source":{"type":"download","folder":"` + downloads + `"},"releaseDir":"release","currentDir":"current","no parameter":["check","setup"]}`
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()

	if err := runNoParameter(context.Background(), "3.0.7"); err != nil {
		t.Fatalf("no-parameter check+setup failed: %v", err)
	}
}

func TestUpgradeAcceptsNoParameterCheckSetup(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	dir := filepath.Join(root, config.ConfigDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"schemaVersion":6,"projectName":"demo","source":{"type":"download","folder":"` + downloads + `"},"releaseDir":"release","currentDir":"current","no parameter":["check","setup"]}`
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), "3.0.7", []string{"--upgrade", "--root", root}); err != nil {
		t.Fatalf("--upgrade rejected historical check+setup config: %v", err)
	}
}

func TestUpdateAcceptsLegacy214SetupManifest(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	v1 := releaseZip(t, downloads, "demo", "1.0.0", map[string]string{"app.txt": "old"})
	if err := Run(context.Background(), "3.0.4", []string{"--update", v1, "--root", root, "--no-setup"}); err != nil {
		t.Fatal(err)
	}
	legacy := `schemaVersion: 1
project:
  name: Demo Friendly Name
  description: Legacy 2.14 setup
steps:
  - id: setup
    name: Legacy setup works
    when: file:app.txt
    run: printf legacy-ok > setup-result.txt
`
	v2 := releaseZip(t, downloads, "demo", "1.1.0", map[string]string{"app.txt": "new", "update-cli.yaml": legacy})
	if err := Run(context.Background(), "3.0.4", []string{"--update", v2, "--root", root, "--setup"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(cfg.CurrentDir, "setup-result.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "legacy-ok" {
		t.Fatalf("unexpected legacy setup result %q", b)
	}
}

func TestSetupFromCurrentDirectoryUsesLocalManifestWithoutProjectConfig(t *testing.T) {
	current := t.TempDir()
	manifest := `schemaVersion: 1
project:
  name: Standalone Current
  type: go
steps:
  - id: marker
    name: Write setup marker
    run: printf current-setup-ok > setup-result.txt
`
	if err := os.WriteFile(filepath.Join(current, "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(current); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()

	if err := Run(context.Background(), "3.0.8", []string{"--setup", "--no-wait"}); err != nil {
		t.Fatalf("--setup from current directory failed: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(current, "setup-result.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "current-setup-ok" {
		t.Fatalf("unexpected standalone setup result %q", b)
	}
}

func TestSetupV2TaskFromCurrentDirectory(t *testing.T) {
	current := t.TempDir()
	manifest := `schemaVersion: 2
workflows:
  setup:
    tasks: [all]
tasks:
  prepare:
    steps:
      - write:
          path: prepare.txt
          content: prepared
  build:
    requires: [prepare]
    steps:
      - write:
          path: build.txt
          content: built
  all:
    requires: [build]
    steps:
      - write:
          path: all.txt
          content: all
`
	if err := os.WriteFile(filepath.Join(current, "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(current); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()

	if err := Run(context.Background(), "3.1.0", []string{"--setup-task", "build", "--no-ui"}); err != nil {
		t.Fatalf("--setup-task failed: %v", err)
	}
	for _, name := range []string{"prepare.txt", "build.txt"} {
		if _, err := os.Stat(filepath.Join(current, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(current, "all.txt")); !os.IsNotExist(err) {
		t.Fatalf("all task unexpectedly ran: %v", err)
	}
}

func TestCreateYAMLTargetsConfiguredCurrentDirectory(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.CurrentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.CurrentDir, "go.mod"), []byte("module example.com/demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), "3.2.0", []string{"--create-yaml", "--root", root}); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(cfg.CurrentDir, "update-cli.yaml")
	if _, err := os.Stat(manifest); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(manifest)
	if !strings.Contains(string(data), "schemaVersion: 2") || !strings.Contains(string(data), "type: go") {
		t.Fatalf("unexpected generated manifest:\n%s", data)
	}
}

func TestConvertYAMLTargetsConfiguredCurrentDirectory(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.CurrentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := "schemaVersion: 1\nsteps:\n  - id: test\n    name: Test\n    run: echo ok\n"
	manifest := filepath.Join(cfg.CurrentDir, "update-cli.yaml")
	if err := os.WriteFile(manifest, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), "3.2.0", []string{"--convert-yaml", "--root", root}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(manifest)
	if !strings.Contains(string(data), "schemaVersion: 2") || !strings.Contains(string(data), "workflows:") {
		t.Fatalf("unexpected converted manifest:\n%s", data)
	}
}

func TestCreateSetupScriptAlias(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()
	if err := Run(context.Background(), "3.2.0", []string{"-create-setup-script"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "setup.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatal("generated setup.sh is not executable")
	}
}

func TestCreateYAMLFromSetupScriptTargetsConfiguredCurrentDirectory(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.CurrentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.CurrentDir, "go.mod"), []byte("module example.com/demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nset -e\ngo mod download\ngo vet ./...\ngo test ./...\n"
	if err := os.WriteFile(filepath.Join(cfg.CurrentDir, "setup.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), "3.3.0", []string{"--create-yaml", "--from", "setup-script", "--root", root}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(cfg.CurrentDir, "update-cli.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "schemaVersion: 2") || !strings.Contains(text, "Go-Tests ausführen") || !strings.Contains(text, "In setup.sh erkannte") {
		t.Fatalf("unexpected generated manifest:\n%s", text)
	}
}

func TestCreateYAMLFromSetupScriptWithAIRefinement(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "setup.sh"), []byte("#!/bin/sh\ngo test ./...\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	refined := `schemaVersion: 2
project:
  name: Demo AI
  type: go
  description: AI refined
defaults:
  failFast: true
workflows:
  setup:
    tasks: [test]
tasks:
  test:
    steps:
      - id: test
        name: Tests
        go:
          action: test
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": refined}}}})
	}))
	defer server.Close()
	t.Setenv("UPDATE_CLI_AI_PROVIDER", "openai-compatible")
	t.Setenv("UPDATE_CLI_AI_BASE_URL", server.URL)
	t.Setenv("UPDATE_CLI_AI_MODEL", "test-model")
	t.Setenv("UPDATE_CLI_CONFIG_PATH", t.TempDir())
	if err := Run(context.Background(), "3.3.0", []string{"--create-yaml", "--from", "setup-script", "--with-ai", "--root", dir}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "update-cli.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "AI refined") || !strings.Contains(string(data), "schemaVersion: 2") {
		t.Fatalf("AI result not written:\n%s", data)
	}
}

func TestSuccessfulUpdatePrintsInstalledVersionAsFinalConsoleLine(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	if _, err := config.Init(root, config.InitOptions{ProjectName: "nvidia-cli", SourceType: "download", Folder: downloads}); err != nil {
		t.Fatal(err)
	}
	archive := releaseZip(t, downloads, "nvidia-cli", "1.2.4", map[string]string{"app.txt": "new"})

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	defer func() { os.Stdout = oldStdout }()

	if err := Run(context.Background(), "0.8.13", []string{"--update", archive, "--root", root, "--no-ui", "--no-setup"}); err != nil {
		_ = writer.Close()
		t.Fatal(err)
	}
	_ = writer.Close()
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		t.Fatal("no console output")
	}
	got := strings.TrimSpace(lines[len(lines)-1])
	want := "Update CLI Version 0.8.13 | nvidia-cli | Aktualisiert auf Version: v1.2.4"
	if got != want {
		t.Fatalf("last console line = %q, want %q\nfull output:\n%s", got, want, output)
	}
}

func TestUpdateAlreadyInstalledIsSuccessfulNoOp(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	if _, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads}); err != nil {
		t.Fatal(err)
	}
	archive := releaseZip(t, downloads, "demo", "1.0.3", map[string]string{"app.txt": "same"})
	if err := Run(context.Background(), "0.8.18", []string{"--update", archive, "--root", root, "--no-ui", "--no-setup"}); err != nil {
		t.Fatalf("initial update: %v", err)
	}

	oldStdout, oldStderr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = oldStdout, oldStderr }()

	runErr := Run(context.Background(), "0.8.18", []string{"--update", archive, "--root", root, "--no-ui", "--no-setup"})
	_ = outW.Close()
	_ = errW.Close()
	out, _ := io.ReadAll(outR)
	errOut, _ := io.ReadAll(errR)
	if runErr != nil {
		t.Fatalf("same-version update must succeed: %v\nstdout:\n%s\nstderr:\n%s", runErr, out, errOut)
	}
	if len(errOut) != 0 {
		t.Fatalf("same-version update wrote stderr:\n%s", errOut)
	}
	text := string(out)
	if !strings.Contains(text, "Version 1.0.3 ist bereits installiert") {
		t.Fatalf("missing already-installed notice:\n%s", text)
	}
	if strings.Contains(text, "FAIL") || strings.Contains(text, "Zur erneuten Installation") {
		t.Fatalf("same-version update still looks like an error:\n%s", text)
	}
	wantFinal := "Update CLI Version 0.8.18 | demo | Installierte Version: v1.0.3"
	if !strings.Contains(text, wantFinal) {
		t.Fatalf("missing final installed-version line %q:\n%s", wantFinal, text)
	}
}
