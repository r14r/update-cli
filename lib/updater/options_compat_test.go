package updater

import (
	"strings"
	"testing"
)

func TestParseOptionsRestoresTUICompatibilityFlags(t *testing.T) {
	o, err := parseOptions([]string{"--check", "--no-ask", "--no-wait"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.check || !o.noAsk || !o.noWait {
		t.Fatalf("unexpected options: %#v", o)
	}

	o, err = parseOptions([]string{"--setup-manifest", "setup.yaml", "--details", "--wait"})
	if err != nil {
		t.Fatal(err)
	}
	if o.setupManifest != "setup.yaml" || !o.details || !o.wait {
		t.Fatalf("unexpected setup options: %#v", o)
	}
}

func TestParseOptionsRejectsNoAskOutsideCheck(t *testing.T) {
	if _, err := parseOptions([]string{"--update", "--no-ask"}); err == nil {
		t.Fatal("expected --no-ask validation error")
	}
}

func TestParseOptionsNoUISkipsTUI(t *testing.T) {
	o, err := parseOptions([]string{"--setup", "--no-ui"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.setup || !o.noUI {
		t.Fatalf("unexpected options: %#v", o)
	}

	o, err = parseOptions([]string{"--check", "---no-ui"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.check || !o.noUI {
		t.Fatalf("triple-dash compatibility was not normalized: %#v", o)
	}

	o, err = parseOptions([]string{"--check", "--noui"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.check || !o.noUI {
		t.Fatalf("--noui compatibility alias was not normalized: %#v", o)
	}
}

func TestParseOptionsSetupV2Selectors(t *testing.T) {
	o, err := parseOptions([]string{"--setup-list"})
	if err != nil || !o.setupList {
		t.Fatalf("unexpected setup-list: %#v err=%v", o, err)
	}
	o, err = parseOptions([]string{"--setup-task", "build", "--no-ui"})
	if err != nil || o.setupTask != "build" || !o.noUI {
		t.Fatalf("unexpected setup-task: %#v err=%v", o, err)
	}
	o, err = parseOptions([]string{"--setup-workflow", "ci"})
	if err != nil || o.setupWorkflow != "ci" {
		t.Fatalf("unexpected setup-workflow: %#v err=%v", o, err)
	}
	o, err = parseOptions([]string{"--setup-manifest", "setup.yaml", "--setup-task", "build"})
	if err != nil || o.setupManifest != "setup.yaml" || o.setupTask != "build" {
		t.Fatalf("unexpected setup-manifest task selection: %#v err=%v", o, err)
	}
	if _, err := parseOptions([]string{"--setup-task", "build", "--setup-workflow", "ci"}); err == nil {
		t.Fatal("expected task/workflow conflict")
	}
}

func TestSetupManagementOptions(t *testing.T) {
	for _, args := range [][]string{{"--convert-yaml"}, {"--create-yaml"}, {"--create-setup-script"}, {"-create-setup-script"}} {
		o, err := parseOptions(args)
		if err != nil {
			t.Fatalf("parseOptions(%v): %v", args, err)
		}
		if !(o.convertYAML || o.createYAML || o.createSetupScript) {
			t.Fatalf("management mode not set for %v: %#v", args, o)
		}
	}
	if _, err := parseOptions([]string{"--create-yaml", "--create-setup-script"}); err == nil {
		t.Fatal("expected mutually exclusive management flags")
	}
	if _, err := parseOptions([]string{"--create-yaml", "--dry-run"}); err != nil {
		t.Fatalf("dry-run should be allowed: %v", err)
	}
	if _, err := parseOptions([]string{"--create-yaml", "--force"}); err != nil {
		t.Fatalf("force should be allowed: %v", err)
	}
}

func TestCreateYAMLFromModes(t *testing.T) {
	for _, args := range [][]string{
		{"--create-yaml"},
		{"--create-yaml", "--from", "project"},
		{"--create-yaml", "--from", "setup-script"},
		{"--create-yaml", "--from", "setup-script", "--with-ai"},
	} {
		if _, err := parseOptions(args); err != nil {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
	if _, err := parseOptions([]string{"--create-yaml", "--from", "project", "--with-ai"}); err == nil {
		t.Fatal("expected --with-ai source validation error")
	}
	if _, err := parseOptions([]string{"--create-yaml", "--from", "unknown"}); err == nil {
		t.Fatal("expected invalid create-yaml source")
	}
	if _, err := parseOptions([]string{"--with-ai"}); err == nil {
		t.Fatal("expected standalone --with-ai error")
	}
}

func TestCleanOption(t *testing.T) {
	o, err := parseOptions([]string{"--clean"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.clean || o.cleanup {
		t.Fatalf("unexpected clean options: %#v", o)
	}
	if _, err := parseOptions([]string{"--clean", "--keep", "2", "--plan"}); err != nil {
		t.Fatalf("--clean --keep --plan should be valid: %v", err)
	}
	if _, err := parseOptions([]string{"--clean", "--cleanup"}); err == nil {
		t.Fatal("--clean and --cleanup must be mutually exclusive")
	}
}

func TestUnknownOptionSuggestsClosestFlag(t *testing.T) {
	_, err := parseOptions([]string{"--vesion"})
	if err == nil {
		t.Fatal("expected unknown option error")
	}
	text := err.Error()
	if !strings.Contains(text, `--vesion`) || !strings.Contains(text, `--version`) {
		t.Fatalf("unexpected suggestion: %q", text)
	}

	_, err = parseOptions([]string{"--updat"})
	if err == nil || !strings.Contains(err.Error(), `--update`) {
		t.Fatalf("expected --update suggestion, got %v", err)
	}
}

func TestUnknownOptionWithoutCloseMatchDoesNotGuess(t *testing.T) {
	_, err := parseOptions([]string{"--banana"})
	if err == nil {
		t.Fatal("expected unknown option error")
	}
	if strings.Contains(err.Error(), "meinten Sie") {
		t.Fatalf("unexpected low-confidence suggestion: %q", err)
	}
}

func TestConfigCommandSetSyntax(t *testing.T) {
	o, err := parseOptions([]string{"config", "--set", "no-parameter=check,setup"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.config || len(o.configSet) != 1 || o.configSet[0] != "no-parameter=check,setup" {
		t.Fatalf("unexpected config command options: %#v", o)
	}

	o, err = parseOptions([]string{"--config", "--set", "backup.keep=7", "--set", "retention.releases=8"})
	if err != nil {
		t.Fatal(err)
	}
	if len(o.configSet) != 2 {
		t.Fatalf("expected two --set assignments, got %#v", o.configSet)
	}
}

func TestConfigSetRejectsConflictingConfigModes(t *testing.T) {
	for _, args := range [][]string{
		{"config", "--set", "project-name=demo", "--list"},
		{"config", "--set", "project-name=demo", "--edit"},
		{"config", "--set", "project-name=demo", "--use-template", "go"},
	} {
		if _, err := parseOptions(args); err == nil {
			t.Fatalf("expected conflict for %v", args)
		}
	}
}
