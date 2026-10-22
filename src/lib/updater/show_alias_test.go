package updater

import (
	"strings"
	"testing"
)

func TestBashUpdateAlias(t *testing.T) {
	script := BashUpdateAlias()
	for _, part := range []string{"u() (", "unalias u", "command update-cli\n  else", "command update-cli \"$@\"", "cd -- \"$root\"", ".update-cli/config.json"} {
		if !strings.Contains(script, part) {
			t.Fatalf("missing %q", part)
		}
	}
	if strings.Contains(script, "command update-cli update") {
		t.Fatal("bare u must delegate to configured no-parameter action, not force update")
	}
	if strings.Contains(script, "uupdate()") {
		t.Fatal("legacy name uupdate must not be defined")
	}
	if err := ValidatePublicSyntax([]string{"generate-alias"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePublicSyntax([]string{"generate-alias", "--root", "/tmp"}); err == nil {
		t.Fatal("generate-alias should reject extra arguments")
	}
}
