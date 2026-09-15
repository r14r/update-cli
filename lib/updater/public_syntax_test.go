package updater

import "testing"

func TestValidatePublicSyntaxCanonicalGrammar(t *testing.T) {
	valid := [][]string{
		{"version"},
		{"help", "--details"},
		{"help", "--command", "setup", "--details"},
		{"update", "--archive", "release.zip", "--no-setup"},
		{"setup", "--list"},
		{"setup", "--run", "go-vet"},
		{"config", "--migrate"},
		{"schema", "--version"},
		{"releases", "--list"},
		{"rollback", "--version", "2.15.0", "--setup"},
		{"init", "--project", "demo", "--from-repository", "r14r/demo"},
		{"restore", "--snapshot", "latest"},
	}
	for _, args := range valid {
		if err := ValidatePublicSyntax(args); err != nil {
			t.Fatalf("ValidatePublicSyntax(%v): %v", args, err)
		}
	}
}

func TestValidatePublicSyntaxRejectsLegacyCommandFlagsAndBareSubcommands(t *testing.T) {
	invalid := [][]string{
		{"--version"}, {"--help"}, {"--setup"}, {"--update"},
		{"setup", "run", "go-vet"},
		{"schema", "version"},
		{"config", "migrate"},
		{"update", "release.zip"},
		{"help", "setup", "--details"},
	}
	for _, args := range invalid {
		if err := ValidatePublicSyntax(args); err == nil {
			t.Fatalf("ValidatePublicSyntax(%v) unexpectedly succeeded", args)
		}
	}
}
