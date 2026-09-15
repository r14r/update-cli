package updater

import (
	"reflect"
	"strings"
	"testing"
)

func TestCanonicalCommandFormsParse(t *testing.T) {
	cases := [][]string{
		{"check"},
		{"update", "--plan", "--json"},
		{"doctor", "--migrate"},
		{"schema", "--view"},
		{"schema", "--save", "schema.json"},
		{"schema", "--version"},
		{"releases", "--list", "--json"},
		{"config", "--check"},
		{"config", "--migrate"},
		{"config", "--set", "no-parameter=update,no-setup"},
		{"update", "--archive", "release.zip", "--setup"},
		{"run"},
		{"backup"},
		{"rollback", "--version", "1.2.3", "--setup"},
		{"restore", "--snapshot", "latest"},
		{"status", "--json"},
		{"verify", "--archive", "release.zip"},
		{"cleanup", "--keep", "3"},
		{"history", "--limit", "5"},
		{"init", "--project", "demo"},
		{"upgrade"},
		{"unlock"},
		{"howto"},
		{"version"},
		{"setup"},
		{"setup", "--list", "--json"},
		{"setup", "--task", "build", "--details"},
		{"setup", "--workflow", "ci"},
		{"setup", "--manifest", "update-cli.yaml"},
		{"convert-yaml", "--dry-run"},
		{"create-yaml", "--from", "project", "--dry-run"},
		{"create-setup-script", "--dry-run"},
		{"templates", "--list", "--details"},
		{"templates", "--edit"},
		{"templates", "--use", "go"},
	}
	for _, args := range cases {
		if _, err := parseOptions(args); err != nil {
			t.Fatalf("parseOptions(%v): %v", args, err)
		}
	}
}

func TestHelpCommandAliasMatchesFlag(t *testing.T) {
	got, err := parseOptions([]string{"help", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	want, err := parseOptions([]string{"--help", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("options differ: %#v != %#v", got, want)
	}
}

func TestModeOptionParsing(t *testing.T) {
	o, err := parseOptions([]string{"update", "--mode", "pull", "--repository", "https://example.invalid/demo.git"})
	if err != nil {
		t.Fatal(err)
	}
	if o.mode != "pull" || o.repository == "" {
		t.Fatalf("unexpected options: %#v", o)
	}
	if _, err := parseOptions([]string{"update", "--mode", "clone"}); err == nil {
		t.Fatal("invalid mode unexpectedly accepted")
	}
}

func TestInitFromRepositoryShortcut(t *testing.T) {
	cases := []string{
		"https://github.com/r14r/git-cli",
		"r14r/ollama-cli",
		"ollama-cli",
	}
	for _, repository := range cases {
		o, err := parseOptions([]string{"--init", "demo", "--from-repository", repository})
		if err != nil {
			t.Fatalf("repository %q: %v", repository, err)
		}
		if !o.init || strings.TrimSpace(o.fromRepository) == "" || o.projectName != "demo" {
			t.Fatalf("unexpected init options for %q: %#v", repository, o)
		}
		if o.mode != "pull" || o.sourceType != "repository" || o.fromRepository != repository {
			t.Fatalf("repository shortcut not normalized: %#v", o)
		}
	}
}

func TestInitFromRepositoryLegacySyntaxRemainsAccepted(t *testing.T) {
	o, err := parseOptions([]string{"--init", "demo", "--from-repository", "--repository", "https://github.com/r14r/demo"})
	if err != nil {
		t.Fatal(err)
	}
	if o.fromRepository != "https://github.com/r14r/demo" || o.repository != "" || o.mode != "pull" {
		t.Fatalf("legacy shortcut not normalized: %#v", o)
	}
}

func TestInitFromRepositoryRequiresRepository(t *testing.T) {
	if _, err := parseOptions([]string{"--init", "demo", "--from-repository"}); err == nil {
		t.Fatal("missing repository unexpectedly accepted")
	}
	if _, err := parseOptions([]string{"--from-repository", "r14r/demo"}); err == nil {
		t.Fatal("--from-repository without --init unexpectedly accepted")
	}
}

func TestSchemaRequiresExactlyOneAction(t *testing.T) {
	if _, err := parseOptions([]string{"schema"}); err == nil {
		t.Fatal("schema without action unexpectedly accepted")
	}
	if _, err := parseOptions([]string{"schema", "--view", "--save", "schema.json"}); err == nil {
		t.Fatal("schema view + save unexpectedly accepted")
	}
	if _, err := parseOptions([]string{"schema", "--view", "--version"}); err == nil {
		t.Fatal("schema view + version unexpectedly accepted")
	}
	o, err := parseOptions([]string{"schema", "--version"})
	if err != nil {
		t.Fatalf("schema --version: %v", err)
	}
	if !o.schema || !o.schemaVersion || o.showVersion {
		t.Fatalf("schema --version not normalized as schema action: %#v", o)
	}
	if _, err := parseOptions([]string{"--view"}); err == nil {
		t.Fatal("--view without schema unexpectedly accepted")
	}
}

func TestDebugIsGlobalAndSupportsNoParameterInvocation(t *testing.T) {
	o, err := parseOptions([]string{"--debug"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.debug || !o.noParameterInvocation {
		t.Fatalf("debug-only invocation not normalized: %#v", o)
	}

	for _, args := range [][]string{
		{"config", "--list", "--debug"},
		{"--debug", "config", "--list"},
		{"--init", "demo", "--debug"},
	} {
		o, err := parseOptions(args)
		if err != nil {
			t.Fatalf("parseOptions(%#v): %v", args, err)
		}
		if !o.debug {
			t.Fatalf("debug not set for %#v: %#v", args, o)
		}
	}
}

func TestSetupListAndRunAreStandaloneSubcommands(t *testing.T) {
	for _, args := range [][]string{
		{"setup", "--list"},
		{"setup", "list"},
	} {
		o, err := parseOptions(args)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !o.setupList || o.setup {
			t.Fatalf("%v not normalized to setup list: %#v", args, o)
		}
	}
	for _, args := range [][]string{
		{"setup", "--run", "go-vet"},
		{"setup", "run", "go-vet"},
		{"--setup", "--run", "go-vet"},
	} {
		o, err := parseOptions(args)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if o.setupStep != "go-vet" || o.setup {
			t.Fatalf("%v not normalized to setup run: %#v", args, o)
		}
	}
	for _, args := range [][]string{
		{"update", "--setup", "--list"},
		{"--update", "--setup", "--list"},
	} {
		_, err := parseOptions(args)
		if err == nil || !strings.Contains(err.Error(), "eigenständige Befehle") {
			t.Fatalf("%v: expected explicit standalone setup error, got %v", args, err)
		}
	}
}

func TestCommandHelpDetailsParsing(t *testing.T) {
	for _, args := range [][]string{
		{"help", "setup", "--details"},
		{"setup", "--help", "--details"},
	} {
		o, err := parseOptions(args)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !o.showHelp || !o.details || o.helpTopic != "setup" {
			t.Fatalf("%v: unexpected help options %#v", args, o)
		}
	}
}
