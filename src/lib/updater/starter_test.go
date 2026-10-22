package updater

import (
	"strings"
	"testing"
)

func TestStarterManifest(t *testing.T) {
	text := StarterManifest()
	for _, part := range []string{
		"schemaVersion: 2", "workflows:", "tasks:",
		"shell: just bootstrap", "shell: just setup",
		"shell: just precommit", "shell: just test",
		"shell: just build", "shell: just ci",
		"shell: just clean", "command: just run",
	} {
		if !strings.Contains(text, part) {
			t.Errorf("starter manifest missing %q", part)
		}
	}
	if err := ValidatePublicSyntax([]string{"generate-config"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePublicSyntax([]string{"generate-config", "--root", "/tmp"}); err == nil {
		t.Fatal("generate-config should reject parameters")
	}
}
