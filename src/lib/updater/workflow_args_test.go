package updater

import (
	"slices"
	"testing"
)

func TestWorkflowStageArgsRunDoesNotPassWaitFlags(t *testing.T) {
	for _, tc := range []struct {
		name        string
		noUI, debug bool
	}{
		{"default", false, false},
		{"plain", true, false},
		{"debug", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := workflowStageArgs("run", "/tmp/project", tc.noUI, tc.debug)
			if slices.Contains(args, "--no-wait") || slices.Contains(args, "--wait") {
				t.Fatalf("run receives unsupported wait parameter: %v", args)
			}
			if _, err := parseOptions(args); err != nil {
				t.Fatalf("run workflow arguments must parse successfully: %v", err)
			}
		})
	}
}

func TestWorkflowStageUpdateDisablesSetupConfirmation(t *testing.T) {
	args := workflowStageArgs("update", "/tmp/project", false, false)
	if !slices.Contains(args, "--no-setup") {
		t.Fatalf("unattended workflow update must bypass setup confirmation: %v", args)
	}
	if _, err := parseOptions(args); err != nil {
		t.Fatalf("unattended workflow update flags invalid: %v", err)
	}
}

func TestWorkflowStageArgsRetainsNoWaitForSupportedStages(t *testing.T) {
	for _, stage := range []string{"update", "bootstrap", "install", "precommit", "test", "build"} {
		args := workflowStageArgs(stage, "/tmp/project", false, false)
		if !slices.Contains(args, "--no-wait") {
			t.Fatalf("%s missing no-wait: %v", stage, args)
		}
		if _, err := parseOptions(args); err != nil {
			t.Fatalf("%s workflow arguments must parse: %v", stage, err)
		}
	}
}
