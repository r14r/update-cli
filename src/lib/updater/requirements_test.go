package updater

import (
	"strings"
	"testing"
)

func TestProjectRequirements(t *testing.T) {
	output := ProjectRequirements()
	for _, expected := range []string{
		"schemaVersion: 2",
		"justfile",
		"bootstrap",
		"setup",
		"test",
		"build",
		"install",
		"run",
		"ci",
		"SOURCE.md",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("missing requirement %q", expected)
		}
	}
	if err := ValidatePublicSyntax([]string{"show-requirements"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePublicSyntax([]string{"show-requirements", "unexpected"}); err == nil {
		t.Fatal("unexpected argument accepted")
	}
}
