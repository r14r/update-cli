package updater

import (
	"strings"
	"testing"
)

func TestBashUpdateAlias(t *testing.T) {
	script := BashUpdateAlias()
	for _, part := range []string{"u() (", "unalias u", "command update-cli update \"$@\"", "cd -- \"$root\"", ".update-cli/config.json"} {
		if !strings.Contains(script, part) { t.Fatalf("missing %q", part) }
	}
	if strings.Contains(script, "uupdate()") { t.Fatal("legacy name uupdate must not be defined") }
	if err := ValidatePublicSyntax([]string{"show-alias"}); err != nil { t.Fatal(err) }
	if err := ValidatePublicSyntax([]string{"show-alias", "--root", "/tmp"}); err == nil {
		t.Fatal("show-alias should reject extra arguments")
	}
}
