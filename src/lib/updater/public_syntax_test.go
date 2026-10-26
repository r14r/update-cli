package updater

import "testing"

func TestValidatePublicSyntaxCanonicalGrammar(t *testing.T) {
	valid := [][]string{
		{"version"},
		{"whatsnew"},
		{"whatsnew", "--version", "2.16.16"},
		{"help", "--details"},
		{"help", "--command", "setup", "--details"},
		{"update", "--archive", "release.zip", "--no-setup"},
		{"setup", "--list"},
		{"setup", "--run", "go-vet"},
		{"config", "--migrate"},
		{"migrate"},
		{"recover"},
		{"recover", "--json"},
		{"schema", "--version"},
		{"releases", "--list"},
		{"init", "--project", "demo", "--from-repository", "r14r/demo"},
		{"init", "--repository", "https://github.com/r14r/demo.git"},
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
		{"rollback", "--version", "3.4.38"},
		{"restore", "--snapshot", "latest"},
		{"help", "setup", "--details"},
	}
	for _, args := range invalid {
		if err := ValidatePublicSyntax(args); err == nil {
			t.Fatalf("ValidatePublicSyntax(%v) unexpectedly succeeded", args)
		}
	}
}
