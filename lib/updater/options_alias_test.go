package updater

import (
	"reflect"
	"testing"
)

func TestCommandAliasesMatchLegacyFlags(t *testing.T) {
	cases := []struct {
		name    string
		command []string
		legacy  []string
	}{
		{"check", []string{"check"}, []string{"--check"}},
		{"update", []string{"update", "release.zip", "--setup"}, []string{"--update", "release.zip", "--setup"}},
		{"run", []string{"run"}, []string{"--run"}},
		{"backup", []string{"backup"}, []string{"--backup"}},
		{"rollback", []string{"rollback", "1.2.3", "--setup"}, []string{"--rollback", "1.2.3", "--setup"}},
		{"restore", []string{"restore", "latest"}, []string{"--restore", "latest"}},
		{"status json", []string{"status", "--json"}, []string{"--status", "--json"}},
		{"list json", []string{"list", "--json"}, []string{"--list", "--json"}},
		{"verify", []string{"verify", "release.zip"}, []string{"--verify", "release.zip"}},
		{"doctor", []string{"doctor"}, []string{"--doctor"}},
		{"cleanup", []string{"cleanup", "--keep", "3"}, []string{"--cleanup", "--keep", "3"}},
		{"history", []string{"history", "--limit", "5"}, []string{"--history", "--limit", "5"}},
		{"init", []string{"init", "demo"}, []string{"--init", "demo"}},
		{"upgrade", []string{"upgrade"}, []string{"--upgrade"}},
		{"unlock", []string{"unlock"}, []string{"--unlock"}},
		{"setup", []string{"setup"}, []string{"--setup"}},
		{"setup list", []string{"setup", "list", "--json"}, []string{"--setup-list", "--json"}},
		{"setup task", []string{"setup", "task", "build", "--details"}, []string{"--setup-task", "build", "--details"}},
		{"setup workflow", []string{"setup", "workflow", "ci"}, []string{"--setup-workflow", "ci"}},
		{"setup manifest", []string{"setup", "manifest", "update-cli.yaml"}, []string{"--setup-manifest", "update-cli.yaml"}},
		{"convert yaml", []string{"convert-yaml", "--dry-run"}, []string{"--convert-yaml", "--dry-run"}},
		{"create yaml", []string{"create-yaml", "--from", "project", "--dry-run"}, []string{"--create-yaml", "--from", "project", "--dry-run"}},
		{"create setup script", []string{"create-setup-script", "--dry-run"}, []string{"--create-setup-script", "--dry-run"}},
		{"config list", []string{"config", "list"}, []string{"--config", "--list"}},
		{"config edit", []string{"config", "edit"}, []string{"--config", "--edit"}},
		{"config use-template", []string{"config", "use-template", "go"}, []string{"--config", "--use-template", "go"}},
		{"templates list", []string{"templates", "list", "--details"}, []string{"--templates", "--list", "--details"}},
		{"templates edit", []string{"templates", "edit"}, []string{"--templates", "--edit"}},
		{"templates use", []string{"templates", "use", "go"}, []string{"--templates", "--use", "go"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseOptions(tc.command)
			if err != nil {
				t.Fatalf("command parse: %v", err)
			}
			want, err := parseOptions(tc.legacy)
			if err != nil {
				t.Fatalf("legacy parse: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("options differ\ncommand: %#v\nlegacy:  %#v", got, want)
			}
		})
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
