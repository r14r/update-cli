package config

import "testing"

func TestNormalizedNoParameterWorkflow(t *testing.T) {
	got, err := normalizedNoParameter(NoParameterConfig{"workflow"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "workflow" {
		t.Fatalf("unexpected actions: %v", got)
	}
	if _, err := normalizedNoParameter(NoParameterConfig{"workflow", "update"}); err == nil {
		t.Fatal("workflow must be exclusive")
	}
}
