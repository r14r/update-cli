package updater

import (
	"strings"
	"testing"

	"github.com/r14r/update-cli/src/lib/discovery"
)

// All public commands must appear in the human help and JSON discovery.
func TestEveryPublicCommandIsDocumentedAndDiscoverable(t *testing.T) {
	found := make(map[string]bool)
	for _, cmd := range discovery.Build("test").Commands {
		if found[cmd.Name] {
			t.Errorf("duplicate discovery command %q", cmd.Name)
		}
		found[cmd.Name] = true
	}
	for name := range publicCommands {
		if _, ok := commandHelpDetails[name]; !ok {
			t.Errorf("public command %q missing from detailed help", name)
		}
		if !found[name] {
			t.Errorf("public command %q missing from JSON discovery", name)
		}
	}
	for name := range found {
		if _, ok := publicCommands[name]; !ok {
			t.Errorf("JSON discovery advertises unknown command %q", name)
		}
	}
}

func TestDetailedHelpShowsRegisteredParameters(t *testing.T) {
	out, err := captureStdout(func() error {
		printCommandHelp("3.4.41", "update", true)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, param := range []string{"--no-setup", "--root", "--archive", "--allow-downgrade"} {
		if !strings.Contains(out, param) {
			t.Errorf("detailed update help misses registered parameter %q", param)
		}
	}
}
