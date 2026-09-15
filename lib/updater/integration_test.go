package updater

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r14r/update-cli/lib/buildconfig"
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

func TestRunNoParameterRepairsInvalidExistingConfigBeforeExecution(t *testing.T) {
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

	err = runNoParameter(context.Background(), "3.0.3", false)
	if err != nil {
		t.Fatalf("expected automatic config repair, got %v", err)
	}
	loaded, loadErr := config.Load(root, "")
	if loadErr != nil {
		t.Fatalf("repaired config cannot be loaded: %v", loadErr)
	}
	if loaded.NoParameterActions[0] != "check" {
		t.Fatalf("no parameter not normalized: %#v", loaded.NoParameterActions)
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

	if err := runNoParameter(context.Background(), "3.0.7", false); err != nil {
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

func TestInitBootstrapsNewestDownloadReleaseIntoCurrentDirectory(t *testing.T) {
	originalBuildConfig := buildconfig.Current()
	downloads := t.TempDir()
	globalConfig := t.TempDir()
	if err := buildconfig.Set(buildconfig.Config{
		SchemaVersion:         1,
		DefaultDownloadFolder: downloads,
		DefaultDeploymentPath: t.TempDir(),
		DefaultConfigPath:     globalConfig,
	}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = buildconfig.Set(originalBuildConfig) }()

	releaseZip(t, downloads, "demo", "1.2.2", map[string]string{"app.txt": "old"})
	releaseZip(t, downloads, "demo", "1.2.3", map[string]string{"app.txt": "new"})
	// Files that do not match either supported project[-v]<semver>.zip convention must
	// never be considered for a bootstrap.
	if err := os.WriteFile(filepath.Join(downloads, "demo-v9.9.zip"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}

	parent := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(parent); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	if err := Run(context.Background(), "2.2.0", []string{"--init", "demo", "--no-ui"}); err != nil {
		t.Fatal(err)
	}

	root := parent
	cfg, err := config.Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Source.Type != "download" || cfg.Source.Folder != downloads {
		t.Fatalf("unexpected source: %#v", cfg.Source)
	}
	body, err := os.ReadFile(filepath.Join(cfg.CurrentDir, "app.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "new" {
		t.Fatalf("newest release was not installed: %q", body)
	}
	versionBody, err := os.ReadFile(filepath.Join(cfg.CurrentDir, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(versionBody)) != "1.2.3" {
		t.Fatalf("installed VERSION = %q", versionBody)
	}
	rootVersion, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(rootVersion)) != "1.2.3" {
		t.Fatalf("root VERSION = %q; expected current version 1.2.3", rootVersion)
	}
}

func TestInitBootstrapsRepositoryIntoCurrentDirectory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	originalBuildConfig := buildconfig.Current()
	if err := buildconfig.Set(buildconfig.Config{
		SchemaVersion:         1,
		DefaultDownloadFolder: t.TempDir(),
		DefaultDeploymentPath: t.TempDir(),
		DefaultConfigPath:     t.TempDir(),
	}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = buildconfig.Set(originalBuildConfig) }()

	repository := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repository}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-b", "main")
	git("config", "user.email", "test@example.invalid")
	git("config", "user.name", "Update CLI Test")
	if err := os.WriteFile(filepath.Join(repository, "VERSION"), []byte("1.4.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "app.txt"), []byte("from repository"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "initial")

	parent := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(parent); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	if err := Run(context.Background(), "2.6.0", []string{"--init", "demo", "--from-repository", "--repository", repository, "--no-ui"}); err != nil {
		t.Fatal(err)
	}

	root := parent
	cfg, err := config.Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != config.ModePull || cfg.Source.Type != "repository" || cfg.Source.Repository != repository {
		t.Fatalf("unexpected repository config: mode=%s source=%#v", cfg.Mode, cfg.Source)
	}
	body, err := os.ReadFile(filepath.Join(cfg.CurrentDir, "app.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "from repository" {
		t.Fatalf("repository content was not installed: %q", body)
	}
}

func TestFailedSetupKeepsSyncedCurrentWhenConfigured(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	v1 := releaseZip(t, downloads, "demo", "1.0.0", map[string]string{"app.txt": "old"})
	if err := Run(context.Background(), "2.8.0", []string{"--update", v1, "--root", root, "--no-setup"}); err != nil {
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
	if _, err := config.Set(root, []string{"setup.keepRsyncOnError=true"}); err != nil {
		t.Fatal(err)
	}
	cfg, err = config.Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.KeepRsyncOnSetupError {
		t.Fatal("keep rsync policy not enabled")
	}
	v2 := releaseZip(t, downloads, "demo", "2.0.0", map[string]string{
		"app.txt":         "new",
		"data/db.txt":     "release-data",
		"update-cli.yaml": "version: 1\nsteps:\n  - name: fail\n    type: command\n    command: exit 17\n",
	})
	err = Run(context.Background(), "2.8.0", []string{"--update", v2, "--root", root, "--setup", "--no-ui"})
	if err == nil {
		t.Fatal("expected setup failure")
	}
	b, readErr := os.ReadFile(filepath.Join(cfg.CurrentDir, "app.txt"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(b) != "new" {
		t.Fatalf("synced application should be kept: %q", b)
	}
	b, readErr = os.ReadFile(filepath.Join(cfg.CurrentDir, "data", "db.txt"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(b) != "user-data" {
		t.Fatalf("persistent data should remain preserved: %q", b)
	}
	b, readErr = os.ReadFile(filepath.Join(cfg.CurrentDir, ".env"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(b) != "secret" {
		t.Fatalf("env should remain preserved: %q", b)
	}
	if _, statErr := os.Stat(filepath.Join(cfg.ReleaseRoot, "2.0.0", "app.txt")); statErr != nil {
		t.Fatalf("failed setup release should remain available: %v", statErr)
	}
	state, stateErr := os.ReadFile(filepath.Join(cfg.ReleaseRoot, ".last-state.json"))
	if stateErr != nil {
		t.Fatal(stateErr)
	}
	if !strings.Contains(string(state), `"version": "2.0.0"`) {
		t.Fatalf("release state not updated to kept version: %s", state)
	}
	entries, listErr := history.List(cfg.HistoryFile, 10)
	if listErr != nil {
		t.Fatal(listErr)
	}
	found := false
	for _, e := range entries {
		if e.Action == "update" && e.ToVersion == "2.0.0" && e.Status == "failed" && e.Phase == "setup" {
			found = true
		}
	}
	if !found {
		t.Fatalf("kept setup failure not recorded: %#v", entries)
	}
}

func TestDeclinedInteractiveSetupSkipsDockerRestartAndHealthcheck(t *testing.T) {
	progress := newUpdateProgress(context.Background(), nil, false, 2)
	tx := &transaction{servicesWereRunning: true}
	cfg := config.Config{Healthcheck: config.HealthcheckConfig{Type: "command", Command: "exit 99"}}
	recoveryCalled := false

	err := runPostSetupActivation(context.Background(), cfg, tx, progress, true, func(err error) error {
		recoveryCalled = true
		return err
	})
	if err != nil {
		t.Fatalf("declined setup must not fail post-setup activation: %v", err)
	}
	if recoveryCalled {
		t.Fatal("declined setup unexpectedly entered recovery")
	}
	if progress.current != 2 {
		t.Fatalf("expected both activation steps to be skipped, progress=%d", progress.current)
	}
}

func TestUpdateRunsMigrationBeforeSetupAndStandaloneSetupDoesNotRepeatIt(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	migration := `#!/usr/bin/env bash
set -eu
count_file="$UPDATE_CLI_PROJECT_ROOT/migration-count.txt"
count=0
[ ! -f "$count_file" ] || count="$(cat "$count_file")"
printf '%s\n' "$((count + 1))" > "$count_file"
printf migrated > migration-ready.txt
`
	manifest := `schemaVersion: 1
steps:
  - name: Verify migration ran first
    type: command
    command: test -f migration-ready.txt && printf setup > setup-ran.txt
`
	archive := releaseZip(t, downloads, "demo", "4.5.6", map[string]string{
		"app.txt":         "new",
		"migrate.sh":      migration,
		"update-cli.yaml": manifest,
	})
	if err := Run(context.Background(), "2.12.0", []string{"update", archive, "--root", root, "--setup", "--no-ui"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.CurrentDir, "setup-ran.txt")); err != nil {
		t.Fatalf("setup did not observe completed migration: %v", err)
	}
	marker := filepath.Join(root, config.ConfigDirName, ".migration.done.4.5.6")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("migration marker missing: %v", err)
	}
	count, err := os.ReadFile(filepath.Join(root, "migration-count.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(count)) != "1" {
		t.Fatalf("migration count=%q want 1", count)
	}

	if err := Run(context.Background(), "2.12.0", []string{"setup", "--root", root, "--no-ui"}); err != nil {
		t.Fatal(err)
	}
	count, err = os.ReadFile(filepath.Join(root, "migration-count.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(count)) != "1" {
		t.Fatalf("standalone setup repeated migration: count=%q", count)
	}
}

func TestNoSetupDefersMigrationUntilStandaloneSetup(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	migration := `#!/usr/bin/env bash
set -eu
printf migrated > migration-ready.txt
`
	manifest := `schemaVersion: 1
steps:
  - name: Setup
    type: command
    command: test -f migration-ready.txt
`
	archive := releaseZip(t, downloads, "demo", "5.0.0", map[string]string{
		"migrate.sh":      migration,
		"update-cli.yaml": manifest,
	})
	if err := Run(context.Background(), "2.12.0", []string{"update", archive, "--root", root, "--no-setup", "--no-ui"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.CurrentDir, "migration-ready.txt")); !os.IsNotExist(err) {
		t.Fatalf("migration ran despite --no-setup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, config.ConfigDirName, ".migration.done.5.0.0")); !os.IsNotExist(err) {
		t.Fatalf("marker created despite --no-setup: %v", err)
	}
	if err := Run(context.Background(), "2.12.0", []string{"setup", "--root", root, "--no-ui"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.CurrentDir, "migration-ready.txt")); err != nil {
		t.Fatalf("pending migration did not run during setup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.RootDir, config.ConfigDirName, ".migration.done.5.0.0")); err != nil {
		t.Fatalf("marker missing after deferred migration: %v", err)
	}
}

func TestFailedMigrationRollsBackUpdateAndRemainsPending(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	v1 := releaseZip(t, downloads, "demo", "1.0.0", map[string]string{"app.txt": "old"})
	if err := Run(context.Background(), "2.12.0", []string{"update", v1, "--root", root, "--no-setup", "--no-ui"}); err != nil {
		t.Fatal(err)
	}
	v2 := releaseZip(t, downloads, "demo", "2.0.0", map[string]string{
		"app.txt":         "new",
		"migrate.sh":      "#!/usr/bin/env bash\nexit 23\n",
		"update-cli.yaml": "schemaVersion: 1\nsteps:\n  - name: Setup\n    type: command\n    command: printf setup > setup-ran.txt\n",
	})
	err = Run(context.Background(), "2.12.0", []string{"update", v2, "--root", root, "--setup", "--no-ui"})
	if err == nil || !strings.Contains(err.Error(), "Migration für Version 2.0.0 fehlgeschlagen") {
		t.Fatalf("expected migration failure, got %v", err)
	}
	body, readErr := os.ReadFile(filepath.Join(cfg.CurrentDir, "app.txt"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(body) != "old" {
		t.Fatalf("current was not rolled back after migration failure: %q", body)
	}
	if _, statErr := os.Stat(filepath.Join(root, config.ConfigDirName, ".migration.done.2.0.0")); !os.IsNotExist(statErr) {
		t.Fatalf("failed migration marker exists: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(cfg.CurrentDir, "setup-ran.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("setup ran after failed migration: %v", statErr)
	}
}

func TestNoParameterUpdateNoSetupSkipsSetupPromptAndSetupExecution(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	cfg, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.Set(root, []string{"no-parameter=update,no-setup"}); err != nil {
		t.Fatal(err)
	}
	archive := releaseZip(t, downloads, "demo", "1.0.0", map[string]string{
		"VERSION":         "1.0.0\n",
		"app.txt":         "new\n",
		"update-cli.yaml": "schemaVersion: 2\nproject:\n  name: demo\ntasks:\n  setup:\n    steps:\n      - name: must-not-run\n        shell: 'exit 97'\nworkflows:\n  setup:\n    tasks: [setup]\n",
	})
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()

	if err := runNoParameter(context.Background(), "2.13.1", false); err != nil {
		t.Fatalf("no-parameter update+no-setup failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.CurrentDir, "app.txt")); err != nil {
		t.Fatalf("updated current missing: %v", err)
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatal(err)
	}
}

func TestNoParamAliasUpdateInstallsWithoutCheckConfirmation(t *testing.T) {
	root := t.TempDir()
	downloads := t.TempDir()
	if _, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: downloads}); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, config.ConfigDirName, config.ConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw, "no parameter")
	raw["no-param"] = "update"
	data, err = json.MarshalIndent(raw, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	releaseZip(t, downloads, "demo", "1.0.0", map[string]string{"VERSION": "1.0.0\n", "app.txt": "installed\n"})

	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()

	if err := runNoParameter(context.Background(), "2.14.3", false); err != nil {
		t.Fatalf("no-param update failed: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "current", "app.txt"))
	if err != nil {
		t.Fatalf("update was not installed directly: %v", err)
	}
	if string(body) != "installed\n" {
		t.Fatalf("installed content = %q", body)
	}
}
