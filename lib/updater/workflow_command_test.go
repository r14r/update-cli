package updater

import (
	"context"
	"strings"
	"testing"
)

func TestWorkflowPublicSyntax(t *testing.T) {
	valid := [][]string{
		{"workflow"},
		{"workflow", "--noui"},
		{"workflow", "--root", "/tmp/project"},
		{"workflow", "--root=/tmp/project"},
		{"workflow", "--no-wait"},
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

func TestWorkflowStageOrderInSource(t *testing.T) {
	stages := workflowStages()
	if strings.Join(stages, ",") != "update,bootstrap,install,precommit,test,build,run" {
		t.Fatal("unexpected workflow order")
	}
}

func TestWorkflowStagesAreNotShared(t *testing.T) {
	first := workflowStages()
	first[0] = "corrupted"
	second := workflowStages()
	if second[0] != "update" {
		t.Fatal("workflow stages must be independent")
	}
}

func TestWorkflowMissingRootArgument(t *testing.T) {
	for _, args := range [][]string{
		{"workflow", "--root"},
		{"workflow", "--root="},
		{"workflow", "--root", "--noui"},
	} {
		err := Run(context.Background(), "test", args)
		if err == nil {
			t.Errorf("%v: expected missing root error", args)
		}
	}
}
