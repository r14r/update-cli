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
		{"workflow", "--pipeline", "--plan"},
		{"workflow", "--view", "--details"},
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

func TestParseWorkflowArguments(t *testing.T) {
	cases := []struct {
		args  []string
		root  string
		debug bool
		noUI  bool
	}{
		{args: nil},
		{args: []string{"--root", "/tmp/project"}, root: "/tmp/project"},
		{args: []string{"--root=/tmp/project", "--no-ui"}, root: "/tmp/project", noUI: true},
		{args: []string{"--debug", "--noui", "--no-wait"}, debug: true, noUI: true},
	}
	for _, tc := range cases {
		root, debug, noUI, _, _, _, err := parseWorkflowArguments(tc.args)
		if err != nil || root != tc.root || debug != tc.debug || noUI != tc.noUI {
			t.Errorf("%v: got root=%q debug=%v noUI=%v err=%v", tc.args, root, debug, noUI, err)
		}
	}
	for _, args := range [][]string{
		{"--root"}, {"--root="}, {"--root", "--noui"}, {"--invalid"},
	} {
		if _, _, _, _, _, _, err := parseWorkflowArguments(args); err == nil {
			t.Errorf("%v: expected argument error", args)
		}
	}
}

func TestDeploymentStageOrder(t *testing.T) {
	got := strings.Join(deploymentStages(), ",")
	want := "update,bootstrap,precommit,test,build,install,run,healthcheck"
	if got != want {
		t.Fatalf("deployment pipeline = %q, want %q", got, want)
	}
}

func TestParseDeploymentOptions(t *testing.T) {
	_, _, _, pipeline, plan, _, err := parseWorkflowArguments([]string{"--pipeline", "--plan"})
	if err != nil || !pipeline || !plan {
		t.Fatalf("pipeline=%v plan=%v err=%v", pipeline, plan, err)
	}
}

func TestDeploymentStagesAreIndependent(t *testing.T) {
	first := deploymentStages()
	first[0] = "unexpected"
	if deploymentStages()[0] != "update" {
		t.Fatal("deploymentStages returned shared storage")
	}
}

func TestWorkflowViewOptions(t *testing.T) {
	_, _, _, pipeline, view, details, err := parseWorkflowArguments([]string{"--view", "--details"})
	if err != nil || pipeline || !view || !details {
		t.Fatalf("unexpected view options: pipeline=%v view=%v details=%v err=%v", pipeline, view, details, err)
	}
}
