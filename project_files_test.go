package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/r14r/update-cli/lib/projectsetup"
	"github.com/r14r/update-cli/lib/ui"
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
	if err := os.WriteFile(filepath.Join(root, "update-cli.yaml"), []byte("version: 1\nsteps:\n  - name: noop\n    type: command\n    command: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	localMarker := filepath.Join(root, "local-used")
	localCLI := `#!/usr/bin/env bash
set -e
if [[ "${1:-}" == "help" ]]; then
  echo 'update-cli setup --manifest FILE'
  exit 0
fi
if [[ "${1:-}" == "setup" ]]; then
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
if [[ "${1:-}" == "help" ]]; then
  echo 'update-cli setup --manifest FILE'
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
	want := []string{"setup", "--manifest", wantManifest, "--details", "--no-wait"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("forwarded args mismatch:\n got %#v\nwant %#v", args, want)
	}
}

func TestGlobalSetupTemplateUsesCurrentDirectoryManifestAndNativeTUIRunner(t *testing.T) {
	projectCurrent := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectCurrent, "update-cli.yaml"), []byte("schemaVersion: 1\nsteps:\n  - id: noop\n    run: true\n"), 0o644); err != nil {
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
if [[ "${1:-}" == "help" ]]; then
  echo 'update-cli setup --manifest FILE --list --task NAME --workflow NAME'
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
	manifest := filepath.Join(projectCurrent, "update-cli.yaml")
	if canonical, err := filepath.EvalSymlinks(manifest); err == nil {
		manifest = canonical
	}
	want := []string{"setup", "--manifest", manifest, "--details", "--no-wait"}
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

func TestInstallInstallsGlobalSetupTemplate(t *testing.T) {
	setupData, err := os.ReadFile("update-cli.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(setupData), `setup-template.sh`) || !strings.Contains(string(setupData), `Globales Setup-TUI-Template installieren`) {
		t.Fatal("update-cli.yaml must install the global setup TUI template")
	}
	justData, err := os.ReadFile("justfile")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(justData), `install -m 0755 setup-template.sh "$config_path/setup-template.sh"`) {
		t.Fatal("just install must install setup-template.sh into the global config directory")
	}
}

func TestXCLISetupMigrationExampleParses(t *testing.T) {
	manifest, err := projectsetup.ParseManifest(filepath.Join("doc", "examples", "update-cli-x-cli.yaml"))
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
	if err := os.WriteFile(filepath.Join(root, "update-cli.yaml"), []byte("schemaVersion: 2\ntasks:\n  build:\n    steps:\n      - shell: true\nworkflows:\n  setup:\n    tasks: [build]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	argsFile := filepath.Join(bin, "args.txt")
	fake := "#!/usr/bin/env bash\nif [[ \"${1:-}\" == \"help\" ]]; then echo 'update-cli setup --manifest FILE --list --task NAME --workflow NAME'; exit 0; fi\nprintf '%s\\n' \"$@\" > \"" + argsFile + "\"\n"
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
	if !strings.Contains(text, "--task\nbuild") || !strings.Contains(text, "--no-ui") {
		t.Fatalf("selection flags were not forwarded:\n%s", text)
	}
}

func TestSetupTemplateRejectsSchemaV2CandidateThatCannotParseActualManifest(t *testing.T) {
	root := t.TempDir()
	template, err := os.ReadFile("setup-template.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "setup-template.sh"), template, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `schemaVersion: 2
project:
  name: Demo CLI
  slug: demo-cli
  type: go
workflows:
  setup:
    tasks: [build]
tasks:
  build:
    steps:
      - shell: true
`
	if err := os.WriteFile(filepath.Join(root, "update-cli.yaml"), []byte(manifest), 0o644); err != nil {
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

	incompatibleMarker := filepath.Join(root, "incompatible-used")
	incompatible := `#!/usr/bin/env bash
if [[ "${1:-}" == "help" ]]; then
  echo 'update-cli setup --manifest FILE --list --task NAME --workflow NAME'
  exit 0
fi
if [[ " $* " == *" --list "* ]]; then
  echo 'ERROR update-cli.yaml Zeile 5: unbekanntes project-Feld "slug"' >&2
  exit 1
fi
: > "` + incompatibleMarker + `"
exit 0
`
	incompatiblePath := filepath.Join(root, "dist", "update-cli-"+runtime.GOOS+"-"+runtime.GOARCH)
	if err := os.WriteFile(incompatiblePath, []byte(incompatible), 0o755); err != nil {
		t.Fatal(err)
	}

	compatibleMarker := filepath.Join(root, "compatible-used")
	compatible := `#!/usr/bin/env bash
if [[ "${1:-}" == "help" ]]; then
  echo 'update-cli setup --manifest FILE --list --task NAME --workflow NAME'
  exit 0
fi
if [[ " $* " == *" --list "* ]]; then
  exit 0
fi
: > "` + compatibleMarker + `"
exit 0
`
	if err := os.WriteFile(filepath.Join(root, "dist", "update-cli"), []byte(compatible), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", filepath.Join(root, "setup-template.sh"), "--no-ui")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("template failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(incompatibleMarker); !os.IsNotExist(err) {
		t.Fatalf("candidate that rejected actual manifest must not be selected; stat err=%v\n%s", err, out)
	}
	if _, err := os.Stat(compatibleMarker); err != nil {
		t.Fatalf("compatible fallback candidate was not used: %v\n%s", err, out)
	}
}

func TestProjectSetupManifestUsesSchemaV2(t *testing.T) {
	manifest, err := projectsetup.ParseManifest("update-cli.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != 2 {
		t.Fatalf("update-cli.yaml schema = %d, want 2", manifest.Version)
	}
	for _, name := range []string{"prepare", "check", "build", "verify", "deploy", "install", "clean"} {
		if _, ok := manifest.Tasks[name]; !ok {
			t.Fatalf("update-cli.yaml missing task %q", name)
		}
	}
	if _, ok := manifest.Workflows["setup"]; !ok {
		t.Fatal("update-cli.yaml missing setup workflow")
	}
	if _, ok := manifest.Workflows["ci"]; !ok {
		t.Fatal("update-cli.yaml missing ci workflow")
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
	if err := os.WriteFile(filepath.Join(root, "update-cli.yaml"), []byte("schemaVersion: 2\nworkflows:\n  setup:\n    tasks: [build]\ntasks:\n  build:\n    steps:\n      - shell: true\n"), 0o644); err != nil {
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
if [[ "${1:-}" == "help" ]]; then
  echo 'update-cli setup --manifest FILE --list --task NAME --workflow NAME'
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
if [[ "${1:-}" == "help" ]]; then
  echo 'update-cli setup --manifest FILE'
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
	if err := os.WriteFile(filepath.Join(root, "update-cli.yaml"), []byte("schemaVersion: 2\nworkflows:\n  setup:\n    tasks: [build]\ntasks:\n  build:\n    steps:\n      - shell: true\n"), 0o644); err != nil {
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
    _ "embed"
    "os"
    "strings"
)
//go:embed VERSION
var version string
func main() {
    if len(os.Args) > 1 && os.Args[1] == "help" {
        println("setup --manifest --list --task --workflow")
        return
    }
    _ = os.WriteFile("` + marker + `", []byte(strings.Join(os.Args[1:], "\n")+"\nversion="+strings.TrimSpace(version)), 0644)
}
`
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(mainSource), 0o644); err != nil {
		t.Fatal(err)
	}

	bin := t.TempDir()
	oldGlobal := `#!/usr/bin/env bash
if [[ "${1:-}" == "help" ]]; then echo 'update-cli setup --manifest FILE'; exit 0; fi
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
	if !strings.Contains(text, "--manifest") || !strings.Contains(text, "--no-ui") || !strings.Contains(text, "version=9.8.7") {
		t.Fatalf("unexpected source bootstrap args/version:\n%s", text)
	}
}

func TestInstallInstallsSetupScriptConversionPrompt(t *testing.T) {
	setupData, err := os.ReadFile("update-cli.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(setupData), `prompts/setup-script-to-yaml.txt`) || !strings.Contains(string(setupData), `{{ configDir }}/prompts/setup-script-to-yaml.txt`) {
		t.Fatal("update-cli.yaml must install the setup.sh AI conversion prompt")
	}
	justData, err := os.ReadFile("justfile")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(justData), `install -m 0644 prompts/setup-script-to-yaml.txt "$config_path/prompts/setup-script-to-yaml.txt"`) {
		t.Fatal("just install must install the setup.sh AI conversion prompt")
	}
	prompt, err := os.ReadFile(filepath.Join("prompts", "setup-script-to-yaml.txt"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(prompt)
	for _, marker := range []string{"{{SETUP_SCRIPT}}", "{{DRAFT_YAML}}", "Return ONLY YAML", "schemaVersion 2"} {
		if !strings.Contains(text, marker) {
			t.Fatalf("prompt missing %q", marker)
		}
	}
}

func TestEmbeddedAndInstalledSetupScriptPromptsStayInSync(t *testing.T) {
	embedded, err := os.ReadFile(filepath.Join("lib", "projectsetup", "setup-script-ai-prompt.txt"))
	if err != nil {
		t.Fatal(err)
	}
	installed, err := os.ReadFile(filepath.Join("prompts", "setup-script-to-yaml.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(embedded) != string(installed) {
		t.Fatal("embedded and installed setup-script conversion prompts diverged")
	}
}

func TestJustfileRecipeNamesAreUnique(t *testing.T) {
	data, err := os.ReadFile("justfile")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for lineNo, line := range strings.Split(string(data), "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' || strings.HasPrefix(strings.TrimSpace(line), "#") || strings.Contains(line, ":=") {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		head := strings.TrimSpace(line[:colon])
		if head == "" {
			continue
		}
		name := strings.Fields(head)[0]
		if previous, ok := seen[name]; ok {
			t.Fatalf("justfile recipe %q is duplicated on lines %d and %d", name, previous, lineNo+1)
		}
		seen[name] = lineNo + 1
	}
	if _, ok := seen["clear-releases"]; !ok {
		t.Fatal("justfile must provide clear-releases recipe")
	}
}

func TestProjectSettingsLiveInManifestAndHostPolicyStaysInJSON(t *testing.T) {
	manifest, err := projectsetup.ParseManifest("update-cli.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !manifest.Update.Configured || !manifest.Update.SourceConfigured {
		t.Fatalf("update-cli.yaml must provide project update settings: %#v", manifest.Update)
	}
	if manifest.Update.Mode != "update" || manifest.Update.Source.Type != "download" || manifest.Update.Source.Folder != "$HOME/Downloads" {
		t.Fatalf("unexpected manifest source config: %#v", manifest.Update)
	}
	if !manifest.Update.Sync.Configured || len(manifest.Update.Sync.Preserve) == 0 {
		t.Fatalf("project sync policy must be versioned in update-cli.yaml: %#v", manifest.Update.Sync)
	}
	b, err := os.ReadFile(filepath.Join("defaults", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Source struct {
			DefaultUser string `json:"defaultUser"`
		} `json:"source"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	if raw.Source.DefaultUser != "r1r" {
		t.Fatalf("source.defaultUser must remain host/user config: %#v", raw.Source)
	}
	manifestBody, err := os.ReadFile("update-cli.yaml")
	if err != nil {
		t.Fatal(err)
	}
	manifestText := string(manifestBody)
	if strings.Contains(manifestText, "defaultUser:") || strings.Contains(manifestText, "security:") {
		t.Fatal("host/user defaults and security policy must not be versioned in update-cli.yaml")
	}
}

func TestReleaseVersionMarkersStayInSync(t *testing.T) {
	versionData, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.TrimSpace(string(versionData))
	if version == "" {
		t.Fatal("VERSION must not be empty")
	}

	readmeData, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	readmeMarker := "Current release: **" + version + "**"
	if !strings.Contains(string(readmeData), readmeMarker) {
		t.Fatalf("README.md current release must match VERSION %q", version)
	}

	notesData, err := os.ReadFile("RELEASE_NOTES.md")
	if err != nil {
		t.Fatal(err)
	}
	notes := strings.TrimSpace(string(notesData))
	if !strings.HasPrefix(notes, "# "+version+"\n") && notes != "# "+version {
		t.Fatalf("RELEASE_NOTES.md must start with release %q", version)
	}
}

func TestSourcePackagingExcludesRuntimeStateAndRequiresCorePackages(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("scripts", "package-source.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, marker := range []string{
		"--exclude='/.update-cli/'",
		"lib/backup/backup.go",
		"defaults/config.json",
		"defaults/templates.json",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("source package script missing %q", marker)
		}
	}
	if _, err := os.Stat(filepath.Join("lib", "backup", "backup.go")); err != nil {
		t.Fatalf("required backup package missing from source tree: %v", err)
	}
}

func TestReadmeCLIDemoTapesAndCanonicalSetupInstall(t *testing.T) {
	for _, name := range []string{
		filepath.Join("docs", "tapes", "install.tape"),
		filepath.Join("docs", "tapes", "quickstart.tape"),
		filepath.Join("scripts", "render-tapes.sh"),
		filepath.Join("scripts", "tapes", "create-demo-release.sh"),
	} {
		if _, err := os.Stat(name); err != nil {
			t.Fatalf("required CLI demo artifact missing: %s: %v", name, err)
		}
	}

	manifest, err := projectsetup.ParseManifest("update-cli.yaml")
	if err != nil {
		t.Fatal(err)
	}
	setup, ok := manifest.Workflows["setup"]
	if !ok || len(setup.Tasks) != 1 || setup.Tasks[0] != "install" {
		t.Fatalf("setup workflow must use canonical install task: %#v", setup)
	}
	installTask, ok := manifest.Tasks["install"]
	if !ok {
		t.Fatal("install task missing")
	}
	body, err := os.ReadFile("update-cli.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "unset UPDATE_CLI_SETUP_RUNNING") || !strings.Contains(text, "exec just install") {
		t.Fatal("canonical setup install must sanitize the internal setup marker and exec just install")
	}
	_ = installTask

	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"## INSTALL", "## QUICKSTART", "docs/tapes/install.tape", "docs/tapes/quickstart.tape", "just tapes"} {
		if !strings.Contains(string(readme), marker) {
			t.Fatalf("README missing %q", marker)
		}
	}
}

func TestProjectSetupWorkflowDelegatesToJustInstallWithoutSetupMarker(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based update-cli setup manifest")
	}
	root := t.TempDir()
	manifest, err := os.ReadFile("update-cli.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "update-cli.yaml"), manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("2.13.2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeGo := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(fakeGo), 0o755); err != nil {
		t.Fatal(err)
	}
	result := filepath.Join(root, "just-result")
	fakeJust := "#!/bin/sh\n" +
		"if [ -n \"${UPDATE_CLI_SETUP_RUNNING:-}\" ]; then echo marker-leaked > \"" + result + "\"; exit 88; fi\n" +
		"printf '%s\\n' \"$*\" > \"" + result + "\"\n"
	if err := os.WriteFile(filepath.Join(bin, "just"), []byte(fakeJust), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err = projectsetup.RunStandaloneSelected(context.Background(), filepath.Join(root, "update-cli.yaml"), ui.New(true), projectsetup.Selection{Workflow: "setup"})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(body)) != "install" {
		t.Fatalf("setup workflow did not delegate exactly to just install: %q", body)
	}
}

func TestVersionIsSingleReleaseSource(t *testing.T) {
	versionData, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(versionData))
	if got == "" || !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(got) {
		t.Fatalf("VERSION=%q is not a canonical semantic version", got)
	}
	for _, obsolete := range []string{"RELEASE_VERSION", filepath.Join("scripts", "sync-release-version.sh")} {
		if _, err := os.Stat(obsolete); !os.IsNotExist(err) {
			t.Fatalf("obsolete parallel version source must not exist: %s", obsolete)
		}
	}
	script, err := os.ReadFile(filepath.Join("scripts", "validate-version.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), "$ROOT/VERSION") || strings.Contains(string(script), "RELEASE_VERSION") {
		t.Fatal("version validator must use VERSION as its only source")
	}
}
