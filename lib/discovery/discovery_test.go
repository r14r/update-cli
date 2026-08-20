package discovery

import (
	"encoding/json"
	"testing"
)

func TestContractBasics(t *testing.T) {
	b, err := Marshal("0.8.15")
	if err != nil {
		t.Fatal(err)
	}
	var cli CLI
	if err := json.Unmarshal(b, &cli); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if cli.SchemaVersion != 1 {
		t.Fatalf("schemaVersion=%d", cli.SchemaVersion)
	}
	if cli.Name != "update-cli" {
		t.Fatalf("name=%q", cli.Name)
	}
	if cli.Executable != "update-cli" {
		t.Fatalf("executable=%q", cli.Executable)
	}
	if cli.Version != "0.8.15" {
		t.Fatalf("version=%q", cli.Version)
	}
	if len(cli.Commands) == 0 {
		t.Fatal("commands is empty")
	}
	for _, name := range []string{"update", "check", "status", "run", "setup", "rollback", "doctor"} {
		if findCommand(cli.Commands, []string{name}) == nil {
			t.Fatalf("missing command %q", name)
		}
	}
}

func TestAllOptionsHaveFlagsAndEnumsHaveValues(t *testing.T) {
	cli := Build("test")
	walkCommands(t, cli.Commands, func(path string, c Command) {
		for _, o := range c.Options {
			if len(o.Flags) == 0 {
				t.Fatalf("%s option %q has no flags", path, o.Name)
			}
			if o.Required {
				t.Fatalf("%s option %q must explicitly be optional", path, o.Name)
			}
			if o.Type == "enum" && len(o.Choices) == 0 && o.Values == nil {
				t.Fatalf("%s option %q enum has no choices/values", path, o.Name)
			}
		}
		for _, a := range c.Arguments {
			if a.Type == "enum" && len(a.Choices) == 0 && a.Values == nil {
				t.Fatalf("%s argument %q enum has no choices/values", path, a.Name)
			}
		}
	})
}

func TestDynamicSourcesReferenceStructuredCommands(t *testing.T) {
	cli := Build("test")
	walkCommands(t, cli.Commands, func(path string, c Command) {
		params := append(append([]Parameter{}, c.Arguments...), c.Options...)
		for _, p := range params {
			if p.Values == nil {
				continue
			}
			if p.Values.Type != "command" {
				t.Fatalf("%s %s values type=%q", path, p.Name, p.Values.Type)
			}
			args := p.Values.Args
			if len(args) < 2 || args[len(args)-1] != "--json" {
				t.Fatalf("%s %s dynamic source is not JSON command: %v", path, p.Name, args)
			}
			commandPath := []string{}
			for _, a := range args[:len(args)-1] {
				if len(a) > 0 && a[0] != '-' {
					commandPath = append(commandPath, a)
				}
			}
			ref := findCommand(cli.Commands, commandPath)
			if ref == nil {
				t.Fatalf("%s %s references missing command %v", path, p.Name, commandPath)
			}
			if !hasJSONOption(*ref) {
				t.Fatalf("%s %s references command without --json: %v", path, p.Name, commandPath)
			}
		}
	})
}

func findCommand(commands []Command, path []string) *Command {
	if len(path) == 0 {
		return nil
	}
	for i := range commands {
		if commands[i].Name != path[0] {
			continue
		}
		if len(path) == 1 {
			return &commands[i]
		}
		return findCommand(commands[i].Commands, path[1:])
	}
	return nil
}
func hasJSONOption(c Command) bool {
	for _, o := range c.Options {
		for _, f := range o.Flags {
			if f == "--json" {
				return true
			}
		}
	}
	return false
}
func walkCommands(t *testing.T, commands []Command, fn func(string, Command)) {
	t.Helper()
	var walk func([]Command, string)
	walk = func(items []Command, prefix string) {
		for _, c := range items {
			path := c.Name
			if prefix != "" {
				path = prefix + " " + c.Name
			}
			fn(path, c)
			walk(c.Commands, path)
		}
	}
	walk(commands, "")
}
