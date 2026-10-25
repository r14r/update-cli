package updater

import "testing"

func TestNoArgumentNoUISelection(t *testing.T) {
	tests := []struct {
		name string
		args []string
		wantNoUI bool
	}{
		{name: "explicit noui", args: []string{"--noui"}, wantNoUI: true},
		{name: "explicit no-ui", args: []string{"--no-ui"}, wantNoUI: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := parseOptions(tc.args)
			if err != nil {
				t.Fatalf("parseOptions(%v): %v", tc.args, err)
			}
			if !opts.noParameterInvocation || opts.noUI != tc.wantNoUI {
				t.Fatalf("unexpected no-argument UI options: %+v", opts)
			}
		})
	}
}

func TestWorkflowDefaultUISelection(t *testing.T) {
	_, _, noUI, _, _, _, err := parseWorkflowArguments(nil)
	if err != nil || noUI {
		t.Fatalf("workflow should default to UI: noUI=%v err=%v", noUI, err)
	}
	_, _, noUI, _, _, _, err = parseWorkflowArguments([]string{"--noui"})
	if err != nil || !noUI {
		t.Fatalf("workflow --noui should disable UI: noUI=%v err=%v", noUI, err)
	}
}
