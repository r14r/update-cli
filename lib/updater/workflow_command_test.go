package updater

import "testing"

func TestWorkflowPublicSyntax(t *testing.T) {
	valid := [][]string{
		{"workflow"},
		{"workflow", "--noui"},
		{"workflow", "--root", "/tmp/project"},
		{"workflow", "--debug", "--no-ui"},
	}
	for _, args := range valid {
		if err := ValidatePublicSyntax(args); err != nil {
			t.Errorf("%v: %v", args, err)
		}
	}
	if err := ValidatePublicSyntax([]string{"workflow", "unexpected"}); err == nil {
		t.Fatal("bare workflow positional argument should fail")
	}
}
