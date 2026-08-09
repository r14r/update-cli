package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/r14r/update-cli/lib/projectsetup"
)

func TestJustfileDoesNotUseMakeStyleDoubleDollarEscapes(t *testing.T) {
	data, err := os.ReadFile("justfile")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "$$") {
		t.Fatal("justfile contains $$; just passes recipe text to Bash, where $$ expands to the shell PID")
	}
}

func TestSetupBootstrapPrefersLocalBinaryBeforeInstalledBinary(t *testing.T) {
	data, err := os.ReadFile("setup.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	localAt := strings.Index(text, `run_manifest_if_supported "${ROOT_DIR}/dist/update-cli"`)
	installedAt := strings.Index(text, `installed_cli="$(command -v update-cli`)
	if localAt < 0 || installedAt < 0 || localAt > installedAt {
		t.Fatal("setup.sh must prefer the checkout-local dist/update-cli before the globally installed update-cli")
	}
}

func TestSetupBootstrapUsesCompatibleLocalBinaryWithoutTouchingOldGlobalBinary(t *testing.T) {
	root := t.TempDir()
	setupData, err := os.ReadFile("setup.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "setup.sh"), setupData, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "setup.yaml"), []byte("version: 1\nsteps:\n  - name: noop\n    type: command\n    command: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	localMarker := filepath.Join(root, "local-used")
	localCLI := `#!/usr/bin/env bash
set -e
if [[ "${1:-}" == "--help" ]]; then
  echo 'update-cli --setup-manifest FILE'
  exit 0
fi
if [[ "${1:-}" == "--setup-manifest" ]]; then
  : > "` + localMarker + `"
  exit 0
fi
exit 2
`
	if err := os.WriteFile(filepath.Join(root, "dist", "update-cli"), []byte(localCLI), 0o755); err != nil {
		t.Fatal(err)
	}

	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	globalMarker := filepath.Join(root, "global-used")
	globalCLI := `#!/usr/bin/env bash
: > "` + globalMarker + `"
echo 'old update-cli'
exit 0
`
	if err := os.WriteFile(filepath.Join(binDir, "update-cli"), []byte(globalCLI), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", filepath.Join(root, "setup.sh"))
	cmd.Env = append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"), "NO_COLOR=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("setup.sh failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(localMarker); err != nil {
		t.Fatalf("local setup handler was not used: %v\n%s", err, out)
	}
	if _, err := os.Stat(globalMarker); !os.IsNotExist(err) {
		t.Fatalf("global update-cli should not have been touched when local handler is compatible; stat err=%v\n%s", err, out)
	}
	if strings.Contains(string(out), "Bootstrap über Go") {
		t.Fatalf("unexpected Go fallback when local handler is compatible:\n%s", out)
	}
}

func TestSetupBootstrapForwardsCompatibilityFlagsAndConfig(t *testing.T) {
	root := t.TempDir()
	setupData, err := os.ReadFile("setup.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "setup.sh"), setupData, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "other.yaml"), []byte("schemaVersion: 1\nsteps:\n  - id: noop\n    run: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "args.txt")
	localCLI := `#!/usr/bin/env bash
set -e
if [[ "${1:-}" == "--help" ]]; then
  echo 'update-cli --setup-manifest FILE'
  exit 0
fi
printf '%s\n' "$@" > "` + marker + `"
`
	if err := os.WriteFile(filepath.Join(root, "dist", "update-cli"), []byte(localCLI), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", filepath.Join(root, "setup.sh"), "--config", "other.yaml", "--details", "--no-wait", "--no-fullscreen")
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("setup.sh failed: %v\n%s", err, out)
	}
	b, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSpace(string(b)), "\n")
	wantManifest := filepath.Join(root, "other.yaml")
	// macOS exposes /var as a symlink to /private/var. setup.sh deliberately
	// canonicalizes the manifest path, so compare canonical paths rather than
	// raw temp-directory spellings.
	if canonical, err := filepath.EvalSymlinks(wantManifest); err == nil {
		wantManifest = canonical
	}
	want := []string{"--setup-manifest", wantManifest, "--details", "--no-wait"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("forwarded args mismatch:\n got %#v\nwant %#v", args, want)
	}
}

func TestGlobalSetupTemplateUsesCurrentDirectoryManifestAndNativeTUIRunner(t *testing.T) {
	projectCurrent := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectCurrent, "setup.yaml"), []byte("schemaVersion: 1\nsteps:\n  - id: noop\n    run: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	globalDir := t.TempDir()
	templateData, err := os.ReadFile("setup-template.sh")
	if err != nil {
		t.Fatal(err)
	}
	templatePath := filepath.Join(globalDir, "setup-template.sh")
	if err := os.WriteFile(templatePath, templateData, 0o755); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	argsFile := filepath.Join(binDir, "args.txt")
	envFile := filepath.Join(binDir, "tui.txt")
	fakeCLI := `#!/usr/bin/env bash
if [[ "${1:-}" == "--help" ]]; then
  echo 'update-cli --setup-manifest FILE --setup-list --setup-task NAME --setup-workflow NAME'
  exit 0
fi
printf '%s\n' "$@" > "` + argsFile + `"
printf '%s\n' "${UPDATE_CLI_TUI:-}" > "` + envFile + `"
`
	if err := os.WriteFile(filepath.Join(binDir, "update-cli"), []byte(fakeCLI), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", templatePath, "--details", "--no-wait")
	cmd.Dir = projectCurrent
	env := make([]string, 0, len(os.Environ())+3)
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, "UPDATE_CLI_TUI=") {
			continue
		}
		env = append(env, item)
	}
	cmd.Env = append(env, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"), "NO_COLOR=1", "UPDATE_CLI_TUI=auto")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("setup-template.sh failed: %v\n%s", err, out)
	}
	argsData, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(projectCurrent, "setup.yaml")
	if canonical, err := filepath.EvalSymlinks(manifest); err == nil {
		manifest = canonical
	}
	want := []string{"--setup-manifest", manifest, "--details", "--no-wait"}
	got := strings.Split(strings.TrimSpace(string(argsData)), "\n")
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("global template forwarded args mismatch:\n got %#v\nwant %#v", got, want)
	}
	tuiData, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(tuiData)) != "auto" {
		t.Fatalf("global template must enable native auto TUI, got %q", tuiData)
	}
}

func TestDeployInstallsGlobalSetupTemplate(t *testing.T) {
	setupData, err := os.ReadFile("setup.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(setupData), `setup-template.sh`) || !strings.Contains(string(setupData), `Globales Setup-TUI-Template installieren`) {
		t.Fatal("setup.yaml must install the global setup TUI template")
	}
	justData, err := os.ReadFile("justfile")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(justData), `install -m 0755 setup-template.sh "$config_path/setup-template.sh"`) {
		t.Fatal("just deploy must install setup-template.sh into the global config directory")
	}
}

func TestXCLISetupMigrationExampleParses(t *testing.T) {
	manifest, err := projectsetup.ParseManifest(filepath.Join("doc", "examples", "setup-x-cli.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ProjectName != "x-cli" || manifest.ProjectType != "go" {
		t.Fatalf("unexpected x-cli manifest metadata: %#v", manifest)
	}
	if got, want := len(manifest.Steps), 7; got != want {
		t.Fatalf("x-cli manifest steps = %d, want %d", got, want)
	}
	build := manifest.Steps[4]
	if build.ID != "go-build" || !strings.Contains(build.Command, `MODULE="$(go list -m)"`) || !strings.Contains(build.Command, "${MODULE}/internal/version.Version") {
		t.Fatalf("x-cli build step must derive linker package from go.mod: %#v", build)
	}
	if strings.Contains(build.Command, "github.com/ralphg/") {
		t.Fatalf("x-cli build step still hard-codes a GitHub module path: %q", build.Command)
	}
}

func TestSetupTemplateForwardsSchemaV2SelectionFlags(t *testing.T) {
	root := t.TempDir()
	template, err := os.ReadFile("setup-template.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "setup-template.sh"), template, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "setup.yaml"), []byte("schemaVersion: 2\ntasks:\n  build:\n    steps:\n      - shell: true\nworkflows:\n  setup:\n    tasks: [build]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	argsFile := filepath.Join(bin, "args.txt")
	fake := "#!/usr/bin/env bash\nif [[ \"${1:-}\" == \"--help\" ]]; then echo 'update-cli --setup-manifest FILE --setup-list --setup-task NAME --setup-workflow NAME'; exit 0; fi\nprintf '%s\\n' \"$@\" > \"" + argsFile + "\"\n"
	if err := os.WriteFile(filepath.Join(bin, "update-cli"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", filepath.Join(root, "setup-template.sh"), "--task", "build", "--no-ui")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("template failed: %v\n%s", err, out)
	}
	got, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, "--setup-task\nbuild") || !strings.Contains(text, "--no-ui") {
		t.Fatalf("selection flags were not forwarded:\n%s", text)
	}
}

func TestProjectSetupManifestUsesSchemaV2(t *testing.T) {
	manifest, err := projectsetup.ParseManifest("setup.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != 2 {
		t.Fatalf("setup.yaml schema = %d, want 2", manifest.Version)
	}
	for _, name := range []string{"prepare", "check", "build", "verify", "deploy", "clean"} {
		if _, ok := manifest.Tasks[name]; !ok {
			t.Fatalf("setup.yaml missing task %q", name)
		}
	}
	if _, ok := manifest.Workflows["setup"]; !ok {
		t.Fatal("setup.yaml missing setup workflow")
	}
	if _, ok := manifest.Workflows["ci"]; !ok {
		t.Fatal("setup.yaml missing ci workflow")
	}
}

func TestGlobalSetupTemplatePrefersSchemaV2CapableLocalPlatformBinary(t *testing.T) {
	root := t.TempDir()
	template, err := os.ReadFile("setup-template.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "setup-template.sh"), template, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "setup.yaml"), []byte("schemaVersion: 2\nworkflows:\n  setup:\n    tasks: [build]\ntasks:\n  build:\n    steps:\n      - shell: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("template platform candidate is only defined for darwin/linux")
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("template platform candidate is only defined for amd64/arm64")
	}
	if err := os.MkdirAll(filepath.Join(root, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	localMarker := filepath.Join(root, "local-used")
	localCLI := `#!/usr/bin/env bash
if [[ "${1:-}" == "--help" ]]; then
  echo 'update-cli --setup-manifest FILE --setup-list --setup-task NAME --setup-workflow NAME'
  exit 0
fi
: > "` + localMarker + `"
exit 0
`
	localPath := filepath.Join(root, "dist", "update-cli-"+runtime.GOOS+"-"+runtime.GOARCH)
	if err := os.WriteFile(localPath, []byte(localCLI), 0o755); err != nil {
		t.Fatal(err)
	}

	bin := t.TempDir()
	globalMarker := filepath.Join(root, "global-used")
	oldGlobal := `#!/usr/bin/env bash
if [[ "${1:-}" == "--help" ]]; then
  echo 'update-cli --setup-manifest FILE'
  exit 0
fi
: > "` + globalMarker + `"
exit 0
`
	if err := os.WriteFile(filepath.Join(bin, "update-cli"), []byte(oldGlobal), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", filepath.Join(root, "setup-template.sh"), "--no-ui")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "NO_COLOR=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("schema-v2 template failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(localMarker); err != nil {
		t.Fatalf("schema-v2 local platform binary was not used: %v\n%s", err, out)
	}
	if _, err := os.Stat(globalMarker); !os.IsNotExist(err) {
		t.Fatalf("old global binary must not execute for schema 2; stat err=%v\n%s", err, out)
	}
}

func TestGlobalSetupTemplateBootstrapsSchemaV2FromGoSource(t *testing.T) {
	root := t.TempDir()
	template, err := os.ReadFile("setup-template.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "setup-template.sh"), template, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "setup.yaml"), []byte("schemaVersion: 2\nworkflows:\n  setup:\n    tasks: [build]\ntasks:\n  build:\n    steps:\n      - shell: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("9.8.7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/bootstrap\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "go-run-used")
	mainSource := `package main
import (
    "os"
    "strings"
)
var version = "dev"
func main() {
    if len(os.Args) > 1 && os.Args[1] == "--help" {
        println("--setup-manifest --setup-list --setup-task --setup-workflow")
        return
    }
    _ = os.WriteFile("` + marker + `", []byte(strings.Join(os.Args[1:], "\n")+"\nversion="+version), 0644)
}
`
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(mainSource), 0o644); err != nil {
		t.Fatal(err)
	}

	bin := t.TempDir()
	oldGlobal := `#!/usr/bin/env bash
if [[ "${1:-}" == "--help" ]]; then echo 'update-cli --setup-manifest FILE'; exit 0; fi
exit 99
`
	if err := os.WriteFile(filepath.Join(bin, "update-cli"), []byte(oldGlobal), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", filepath.Join(root, "setup-template.sh"), "--no-ui")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "NO_COLOR=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("schema-v2 source bootstrap failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("go source bootstrap was not used: %v\n%s", err, out)
	}
	text := string(data)
	if !strings.Contains(text, "--setup-manifest") || !strings.Contains(text, "--no-ui") || !strings.Contains(text, "version=9.8.7") {
		t.Fatalf("unexpected source bootstrap args/version:\n%s", text)
	}
}
