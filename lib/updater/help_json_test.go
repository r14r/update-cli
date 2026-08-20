package updater

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/discovery"
)

func TestHelpJSONFlagAndCommandAreJSONOnly(t *testing.T) {
	for _, args := range [][]string{{"--help", "--json"}, {"help", "--json"}} {
		out, err := captureStdout(func() error { return Run(context.Background(), "9.9.9", args) })
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		trimmed := strings.TrimSpace(out)
		if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
			t.Fatalf("%v not JSON-only: %q", args, out)
		}
		if strings.Contains(out, "\x1b[") {
			t.Fatalf("%v contains ANSI", args)
		}
		var cli discovery.CLI
		if err := json.Unmarshal([]byte(out), &cli); err != nil {
			t.Fatalf("%v invalid JSON: %v\n%s", args, err, out)
		}
		if cli.SchemaVersion != 1 || cli.Name != "update-cli" || cli.Executable != "update-cli" {
			t.Fatalf("unexpected contract: %#v", cli)
		}
	}
}

func TestNormalHelpStaysHumanReadable(t *testing.T) {
	out, err := captureStdout(func() error { return Run(context.Background(), "9.9.9", []string{"--help"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "Update CLI 9.9.9\n\nUsage:\n") {
		t.Fatalf("unexpected help output: %q", out)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatal("normal help became JSON")
	}
}

func captureStdout(fn func() error) (string, error) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	done := make(chan []byte, 1)
	go func() { b, _ := io.ReadAll(r); done <- b }()
	runErr := fn()
	_ = w.Close()
	os.Stdout = old
	b := <-done
	_ = r.Close()
	return string(bytes.Clone(b)), runErr
}

func TestSetupListCommandJSONIsStructured(t *testing.T) {
	root := t.TempDir()
	manifest := `schemaVersion: 2
project:
  name: demo
workflows:
  setup:
    tasks: [build]
tasks:
  build:
    steps:
      - shell: "echo ok"
`
	if err := os.WriteFile(root+"/update-cli.yaml", []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureStdout(func() error {
		return Run(context.Background(), "9.9.9", []string{"setup", "list", "--json", "--root", root})
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid setup list JSON: %v\n%s", err, out)
	}
	if got["project"] != "demo" {
		t.Fatalf("project=%v", got["project"])
	}
	if _, ok := got["tasks"].([]any); !ok {
		t.Fatalf("tasks missing: %#v", got)
	}
	if _, ok := got["workflows"].([]any); !ok {
		t.Fatalf("workflows missing: %#v", got)
	}
}

func TestSetupListCommandJSONWithInitializedProject(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(root+"/current", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root+"/downloads", 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Init(root, config.InitOptions{ProjectName: "demo", SourceType: "download", Folder: root + "/downloads"}); err != nil {
		t.Fatal(err)
	}
	manifest := `schemaVersion: 2
project:
  name: demo
workflows:
  setup:
    tasks:
      - build
tasks:
  build:
    steps:
      - shell: "echo ok"
`
	if err := os.WriteFile(root+"/current/update-cli.yaml", []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureStdout(func() error {
		return Run(context.Background(), "9.9.9", []string{"setup", "list", "--json", "--root", root})
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid setup list JSON: %v\n%s", err, out)
	}
	if got["project"] != "demo" {
		t.Fatalf("project=%v", got["project"])
	}
}

func TestHelpJSONAdvertisesUpdateAndPullModes(t *testing.T) {
	cli := discovery.Build("1.1.0")
	for _, commandName := range []string{"check", "update", "init"} {
		foundCommand := false
		foundMode := false
		for _, command := range cli.Commands {
			if command.Name != commandName {
				continue
			}
			foundCommand = true
			for _, option := range command.Options {
				if option.Name != "mode" {
					continue
				}
				foundMode = true
				values := map[string]bool{}
				for _, choice := range option.Choices {
					values[choice.Value] = true
				}
				if !values["update"] || !values["pull"] {
					t.Fatalf("%s mode choices = %#v", commandName, option.Choices)
				}
			}
		}
		if !foundCommand || !foundMode {
			t.Fatalf("mode option missing for %s", commandName)
		}
	}
}
