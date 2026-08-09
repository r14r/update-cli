package projectsetup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r14r/update-cli/lib/ui"
)

func writeV2Manifest(t *testing.T, root string) string {
	t.Helper()
	data := `schemaVersion: 2
project:
  name: demo
  type: test
  description: Declarative setup test

defaults:
  timeout: 30s
  failFast: true

variables:
  outputDir: build
  marker: "{{ outputDir }}/marker.txt"

requirements:
  commands:
    - sh
  optionalCommands:
    - definitely-not-installed-update-cli-test

workflows:
  setup:
    description: Full setup
    tasks:
      - build
      - verify
  ci:
    tasks: [check, verify]

tasks:
  prepare:
    steps:
      - id: mkdir
        name: Prepare output
        mkdir: "{{ outputDir }}"
  check:
    requires:
      - prepare
    steps:
      - name: Conditional check
        shell: |
          printf checked > "{{ outputDir }}/checked.txt"
        when:
          all:
            - fileExists: input.txt
            - not:
                fileExists: missing.txt
  build:
    requires: [check]
    steps:
      - name: Write marker
        write:
          path: "{{ marker }}"
          content: built
  verify:
    requires:
      - build
    steps:
      - name: Verify marker
        assert:
          fileExists: "{{ marker }}"
      - name: Verify directory
        assert:
          directoryExists: "{{ outputDir }}"
`
	path := filepath.Join(root, "setup.yaml")
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "input.txt"), []byte("input"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseManifestV2(t *testing.T) {
	root := t.TempDir()
	path := writeV2Manifest(t, root)
	m, err := ParseManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != 2 || m.ProjectName != "demo" || len(m.Tasks) != 4 || len(m.Workflows) != 2 {
		t.Fatalf("unexpected manifest: %#v", m)
	}
	if got := m.Variables["marker"]; got != "{{ outputDir }}/marker.txt" {
		t.Fatalf("unexpected variable %q", got)
	}
	if got := m.Tasks["build"].Requires; len(got) != 1 || got[0] != "check" {
		t.Fatalf("unexpected requires %#v", got)
	}
}

func TestRunManifestV2DefaultWorkflowAndDependencies(t *testing.T) {
	root := t.TempDir()
	path := writeV2Manifest(t, root)
	result, err := RunStandalone(context.Background(), path, ui.New(true))
	if err != nil {
		t.Fatal(err)
	}
	if result.StepsExecuted != 5 {
		t.Fatalf("unexpected executed steps: %#v", result)
	}
	for _, rel := range []string{"build/checked.txt", "build/marker.txt"} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
}

func TestRunManifestV2SelectedTask(t *testing.T) {
	root := t.TempDir()
	path := writeV2Manifest(t, root)
	result, err := RunStandaloneSelected(context.Background(), path, ui.New(true), Selection{Task: "check"})
	if err != nil {
		t.Fatal(err)
	}
	if result.StepsExecuted != 2 {
		t.Fatalf("expected prepare+check, got %#v", result)
	}
	if _, err := os.Stat(filepath.Join(root, "build/marker.txt")); !os.IsNotExist(err) {
		t.Fatalf("build task unexpectedly executed: %v", err)
	}
}

func TestManifestV2Catalog(t *testing.T) {
	root := t.TempDir()
	path := writeV2Manifest(t, root)
	c, err := CatalogForManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Workflows) != 2 || len(c.Tasks) != 4 {
		t.Fatalf("unexpected catalog %#v", c)
	}
}

func TestManifestV2DetectsDependencyCycle(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "setup.yaml")
	data := `schemaVersion: 2
workflows:
  setup:
    tasks: [a]
tasks:
  a:
    requires: [b]
    steps:
      - shell: echo a
  b:
    requires: [a]
    steps:
      - shell: echo b
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := RunStandalone(context.Background(), path, ui.New(true))
	if err == nil || !strings.Contains(err.Error(), "zyklische") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestManifestV2StructuredCommandAndEnvironment(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "setup.yaml")
	data := `schemaVersion: 2
workflows:
  setup:
    tasks: [run]
tasks:
  run:
    steps:
      - name: structured
        command:
          exec: sh
          args: [-c, "printf %s $DEMO_VALUE > result.txt"]
        env:
          DEMO_VALUE: works
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RunStandalone(context.Background(), path, ui.New(true)); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "result.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "works" {
		t.Fatalf("unexpected result %q", got)
	}
}
