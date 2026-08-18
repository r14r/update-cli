package templates

import "testing"

func TestDefaultTemplatesUseCheckAsNoParameterAction(t *testing.T) {
	defaults := Defaults()
	if len(defaults.Templates) == 0 {
		t.Fatal("expected default templates")
	}
	for _, template := range defaults.Templates {
		if len(template.NoParameter) != 1 || template.NoParameter[0] != "check" {
			t.Fatalf("template %q noParameter = %#v, want [check]", template.Name, template.NoParameter)
		}
	}
}
