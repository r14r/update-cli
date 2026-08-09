package updater

import "testing"

func TestParseOptionsRestoresTUICompatibilityFlags(t *testing.T) {
	o, err := parseOptions([]string{"--check", "--no-ask", "--no-wait"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.check || !o.noAsk || !o.noWait {
		t.Fatalf("unexpected options: %#v", o)
	}

	o, err = parseOptions([]string{"--setup-manifest", "setup.yaml", "--details", "--wait"})
	if err != nil {
		t.Fatal(err)
	}
	if o.setupManifest != "setup.yaml" || !o.details || !o.wait {
		t.Fatalf("unexpected setup options: %#v", o)
	}
}

func TestParseOptionsRejectsNoAskOutsideCheck(t *testing.T) {
	if _, err := parseOptions([]string{"--update", "--no-ask"}); err == nil {
		t.Fatal("expected --no-ask validation error")
	}
}

func TestParseOptionsNoUISkipsTUI(t *testing.T) {
	o, err := parseOptions([]string{"--setup", "--no-ui"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.setup || !o.noUI {
		t.Fatalf("unexpected options: %#v", o)
	}

	o, err = parseOptions([]string{"--check", "---no-ui"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.check || !o.noUI {
		t.Fatalf("triple-dash compatibility was not normalized: %#v", o)
	}
}

func TestParseOptionsSetupV2Selectors(t *testing.T) {
	o, err := parseOptions([]string{"--setup-list"})
	if err != nil || !o.setupList {
		t.Fatalf("unexpected setup-list: %#v err=%v", o, err)
	}
	o, err = parseOptions([]string{"--setup-task", "build", "--no-ui"})
	if err != nil || o.setupTask != "build" || !o.noUI {
		t.Fatalf("unexpected setup-task: %#v err=%v", o, err)
	}
	o, err = parseOptions([]string{"--setup-workflow", "ci"})
	if err != nil || o.setupWorkflow != "ci" {
		t.Fatalf("unexpected setup-workflow: %#v err=%v", o, err)
	}
	o, err = parseOptions([]string{"--setup-manifest", "setup.yaml", "--setup-task", "build"})
	if err != nil || o.setupManifest != "setup.yaml" || o.setupTask != "build" {
		t.Fatalf("unexpected setup-manifest task selection: %#v err=%v", o, err)
	}
	if _, err := parseOptions([]string{"--setup-task", "build", "--setup-workflow", "ci"}); err == nil {
		t.Fatal("expected task/workflow conflict")
	}
}

func TestSetupManagementOptions(t *testing.T) {
	for _, args := range [][]string{{"--convert-yaml"}, {"--create-yaml"}, {"--create-setup-script"}, {"-create-setup-script"}} {
		o, err := parseOptions(args)
		if err != nil {
			t.Fatalf("parseOptions(%v): %v", args, err)
		}
		if !(o.convertYAML || o.createYAML || o.createSetupScript) {
			t.Fatalf("management mode not set for %v: %#v", args, o)
		}
	}
	if _, err := parseOptions([]string{"--create-yaml", "--create-setup-script"}); err == nil {
		t.Fatal("expected mutually exclusive management flags")
	}
	if _, err := parseOptions([]string{"--create-yaml", "--dry-run"}); err != nil {
		t.Fatalf("dry-run should be allowed: %v", err)
	}
	if _, err := parseOptions([]string{"--create-yaml", "--force"}); err != nil {
		t.Fatalf("force should be allowed: %v", err)
	}
}
