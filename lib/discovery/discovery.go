package discovery

import "encoding/json"

type CLI struct {
	SchemaVersion int          `json:"schemaVersion"`
	Name          string       `json:"name"`
	Version       string       `json:"version"`
	Description   string       `json:"description"`
	Executable    string       `json:"executable"`
	Capabilities  Capabilities `json:"capabilities"`
	GlobalOptions []Parameter  `json:"globalOptions"`
	Commands      []Command    `json:"commands"`
}

type Capabilities struct {
	StructuredOutput []string `json:"structuredOutput"`
	StreamingOutput  []string `json:"streamingOutput"`
}

type Command struct {
	Name        string      `json:"name"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Arguments   []Parameter `json:"arguments"`
	Options     []Parameter `json:"options"`
	Commands    []Command   `json:"commands"`
}

type Parameter struct {
	Name        string       `json:"name"`
	Flags       []string     `json:"flags,omitempty"`
	Description string       `json:"description"`
	Type        string       `json:"type"`
	Required    bool         `json:"required"`
	Position    int          `json:"position,omitempty"`
	Minimum     *float64     `json:"minimum,omitempty"`
	Choices     []Choice     `json:"choices,omitempty"`
	Values      *ValueSource `json:"values,omitempty"`
	Repeatable  bool         `json:"repeatable,omitempty"`
	ValueHint   string       `json:"valueHint,omitempty"`
}

type Choice struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}

type ValueSource struct {
	Type          string   `json:"type"`
	Args          []string `json:"args"`
	ItemsField    string   `json:"itemsField"`
	ValueField    string   `json:"valueField"`
	LabelTemplate string   `json:"labelTemplate"`
}

func Build(version string) CLI {
	root := opt("root", []string{"--root", "-r"}, "Project root directory", "directory")
	jsonOpt := opt("json", []string{"--json"}, "Write structured JSON output", "boolean")
	noColor := opt("no-color", []string{"--no-color"}, "Disable ANSI colors", "boolean")
	noUI := opt("no-ui", []string{"--no-ui", "--noui"}, "Disable fullscreen TUI and stream output directly", "boolean")
	wait := opt("wait", []string{"--wait"}, "Wait before leaving interactive output", "boolean")
	noWait := opt("no-wait", []string{"--no-wait"}, "Do not wait before leaving interactive output", "boolean")
	details := opt("details", []string{"--details"}, "Show detailed output; with help, include command purpose, options and examples", "boolean")

	modeOpt := enumOpt("mode", []string{"--mode"}, "Update mode override", []Choice{{Value: "update", Label: "ZIP release update"}, {Value: "pull", Label: "Git repository pull"}})
	sourceOpts := []Parameter{
		modeOpt,
		opt("downloads", []string{"--downloads", "-d"}, "Downloads/source directory override", "directory"),
		enumOpt("from", []string{"--from"}, "Release source type override", []Choice{{Value: "download"}, {Value: "url"}, {Value: "repository"}}),
		opt("folder", []string{"--folder"}, "Release source folder override", "directory"),
		opt("url", []string{"--url"}, "Release source URL override", "url"),
		opt("repository", []string{"--repository"}, "Release repository override", "string"),
	}

	rollbackArg := arg("version", "Release version to restore", "enum", false, 1)
	rollbackArg.Values = &ValueSource{Type: "command", Args: []string{"releases", "--list", "--json"}, ItemsField: "releases", ValueField: "version", LabelTemplate: "{{version}}"}
	restoreArg := arg("backup", "Backup to restore", "enum", true, 1)
	restoreArg.Choices = []Choice{{Value: "latest", Label: "latest (most recent backup)"}}
	restoreArg.Values = &ValueSource{Type: "command", Args: []string{"releases", "--list", "--json"}, ItemsField: "backups", ValueField: "name", LabelTemplate: "{{name}} ({{version}})"}

	setupTaskArg := arg("name", "Setup task name", "enum", true, 1)
	setupTaskArg.Values = &ValueSource{Type: "command", Args: []string{"setup", "--list", "--json"}, ItemsField: "tasks", ValueField: "name", LabelTemplate: "{{name}}"}
	setupWorkflowArg := arg("name", "Setup workflow name", "enum", true, 1)
	setupWorkflowArg.Values = &ValueSource{Type: "command", Args: []string{"setup", "--list", "--json"}, ItemsField: "workflows", ValueField: "name", LabelTemplate: "{{name}}"}
	setupStepArg := arg("stepId", "Setup step id", "enum", true, 1)
	setupStepArg.Values = &ValueSource{Type: "command", Args: []string{"setup", "--list", "--json"}, ItemsField: "steps", ValueField: "id", LabelTemplate: "{{id}} — {{task}} / {{name}}"}

	commands := []Command{
		cmd("check", "Check", "Check for an available project update", nil, join([]Parameter{root, jsonOpt, opt("no-ask", []string{"--no-ask"}, "Do not ask to install an available update", "boolean"), wait, noWait, noUI, noColor}, sourceOpts...)),
		cmd("update", "Update", "Install a new project release", nil, join([]Parameter{root, opt("archive", []string{"--archive", "-a"}, "Release ZIP archive", "file"), opt("dry-run", []string{"--dry-run", "-n"}, "Preview update without applying it", "boolean"), opt("plan", []string{"--plan"}, "Create an update plan without applying changes", "boolean"), opt("allow-downgrade", []string{"--allow-downgrade"}, "Allow installing an older project version", "boolean"), jsonOpt, opt("backup", []string{"--backup"}, "Create a backup before updating", "boolean"), opt("setup", []string{"--setup"}, "Run project setup after update without asking", "boolean"), opt("no-setup", []string{"--no-setup"}, "Do not run project setup after update", "boolean"), opt("force", []string{"--force", "-f"}, "Force update where supported", "boolean"), wait, noWait, noUI, noColor}, sourceOpts...)),
		cmd("backup", "Backup", "Create a project backup", nil, []Parameter{root, jsonOpt, noColor}),
		cmd("rollback", "Rollback", "Restore a previous validated release", nil, []Parameter{root, enumParam("version", []string{"--version"}, rollbackArg), opt("setup", []string{"--setup"}, "Run project setup after rollback", "boolean"), jsonOpt, wait, noWait, noUI, noColor}),
		cmd("restore", "Restore", "Restore a project backup", nil, []Parameter{root, enumParam("snapshot", []string{"--snapshot"}, restoreArg), jsonOpt, noColor}),
		cmd("status", "Status", "Show project and release status", nil, join([]Parameter{root, jsonOpt, noColor}, sourceOpts...)),
		Command{Name: "releases", Title: "Releases", Description: "Inspect releases and backups", Options: join([]Parameter{root, opt("list", []string{"--list"}, "List releases and backups", "boolean"), jsonOpt, noColor}, sourceOpts...), Commands: []Command{}},
		cmd("verify", "Verify", "Verify a release ZIP archive", nil, join([]Parameter{root, opt("archive", []string{"--archive", "-a"}, "Release ZIP archive", "file"), jsonOpt, noColor}, sourceOpts...)),
		cmd("doctor", "Doctor", "Validate runtime config and project manifests, explain the UI migration badge, and optionally migrate or interactively fix them", nil, []Parameter{root, opt("migrate", []string{"--migrate"}, "Migrate safely repairable runtime/manifest state", "boolean"), opt("fix", []string{"--fix"}, "Preview deterministic repairs, ask y/n, then apply them", "boolean"), jsonOpt, noColor}),
		cmd("fix", "Fix project", "Repair and migrate config.json and update-cli.yaml to the current standard", nil, []Parameter{root, jsonOpt, noColor}),
		cmd("run", "Run", "Run the application command from update-cli.yaml", nil, []Parameter{root, noColor}),
		cmd("install", "Install", "Run the project just install recipe", nil, []Parameter{root, noColor}),
		cmd("schema", "Schema", "View, save, or report the canonical update-cli.yaml JSON Schema version", nil, []Parameter{opt("view", []string{"--view"}, "Write the JSON Schema to stdout", "boolean"), opt("save", []string{"--save"}, "Save the JSON Schema to a file", "file"), opt("version", []string{"--version"}, "Print the manifest schema version", "boolean")}),
		cmd("clean", "Clean releases", "Remove obsolete release directory entries only", nil, []Parameter{root, intOpt("keep", []string{"--keep"}, "Number of releases to retain", 0), opt("plan", []string{"--plan"}, "Show what would be removed", "boolean"), jsonOpt, noColor}),
		cmd("cleanup", "Cleanup", "Apply configured release and backup retention", nil, []Parameter{root, intOpt("keep", []string{"--keep"}, "Number of releases/backups to retain", 0), opt("plan", []string{"--plan"}, "Show what would be removed", "boolean"), jsonOpt, noColor}),
		cmd("history", "History", "Show update history", nil, []Parameter{root, intOpt("limit", []string{"--limit"}, "Maximum history entries", 1), jsonOpt, noColor}),
		cmd("init", "Initialize", "Initialize Update CLI configuration for a project", nil, []Parameter{root, opt("project", []string{"--project"}, "Project name", "string"), modeOpt, enumOpt("from", []string{"--from"}, "Initial source type", []Choice{{Value: "download"}, {Value: "url"}, {Value: "repository"}}), opt("folder", []string{"--folder"}, "Release source folder", "directory"), opt("url", []string{"--url"}, "Release source URL", "url"), opt("repository", []string{"--repository"}, "Release repository", "string"), opt("use-template", []string{"--use-template"}, "Apply an initialization template", "string"), opt("force", []string{"--force", "-f"}, "Overwrite existing initialization where supported", "boolean"), noColor}),
		cmd("upgrade", "Upgrade config", "Upgrade project configuration to the current schema", nil, []Parameter{root, jsonOpt, noColor}),
		cmd("unlock", "Unlock", "Remove a stale update lock", nil, []Parameter{root}),
		cmd("howto", "How-to", "Show extended Update CLI usage and safety notes", nil, []Parameter{}),
		cmd("version", "Version", "Show the Update CLI application version", nil, []Parameter{}),
		setupCommand(root, jsonOpt, details, wait, noWait, noUI, noColor, setupTaskArg, setupWorkflowArg, setupStepArg),
		cmd("convert-yaml", "Convert YAML", "Upgrade update-cli.yaml to the latest supported schema", nil, []Parameter{root, opt("dry-run", []string{"--dry-run", "-n"}, "Preview the converted manifest", "boolean"), opt("force", []string{"--force", "-f"}, "Force replacement where supported", "boolean"), details, noColor}),
		cmd("create-yaml", "Create YAML", "Generate schemaVersion 2 update-cli.yaml", nil, []Parameter{root, enumOpt("from", []string{"--from"}, "Generation source", []Choice{{Value: "project"}, {Value: "setup-script"}}), opt("with-ai", []string{"--with-ai"}, "Refine setup.sh conversion with configured AI provider", "boolean"), opt("force", []string{"--force", "-f"}, "Overwrite an existing manifest", "boolean"), opt("dry-run", []string{"--dry-run", "-n"}, "Preview generated YAML", "boolean"), details, noColor}),
		cmd("create-setup-script", "Create setup script", "Generate a setup.sh bootstrap", nil, []Parameter{root, opt("force", []string{"--force", "-f"}, "Overwrite an existing setup.sh", "boolean"), opt("dry-run", []string{"--dry-run", "-n"}, "Preview generated script", "boolean"), details, noColor}),
		configCommand(root, jsonOpt, noColor),
		templatesCommand(root, details, noColor),
	}
	return CLI{SchemaVersion: 1, Name: "update-cli", Version: version, Description: "Safe release updater and project setup runner", Executable: "update-cli", Capabilities: Capabilities{StructuredOutput: []string{"json"}, StreamingOutput: []string{}}, GlobalOptions: []Parameter{opt("debug", []string{"--debug"}, "Show detailed execution steps", "boolean")}, Commands: commands}
}

func Marshal(version string) ([]byte, error) { return json.MarshalIndent(Build(version), "", "  ") }

func setupCommand(root, jsonOpt, details, wait, noWait, noUI, noColor Parameter, taskArg, workflowArg, stepArg Parameter) Command {
	stepOpt := enumParam("run", []string{"--run"}, stepArg)
	taskOpt := enumParam("task", []string{"--task"}, taskArg)
	workflowOpt := enumParam("workflow", []string{"--workflow"}, workflowArg)
	return Command{Name: "setup", Title: "Setup", Description: "Run project setup automation", Arguments: []Parameter{}, Options: []Parameter{
		root,
		opt("list", []string{"--list"}, "List workflows, tasks and steps", "boolean"),
		stepOpt, taskOpt, workflowOpt,
		opt("manifest", []string{"--manifest"}, "Run setup from an explicit update-cli.yaml manifest", "file"),
		details, jsonOpt, wait, noWait, noUI, noColor,
	}, Commands: []Command{}}
}

func configCommand(root, jsonOpt, noColor Parameter) Command {
	set := opt("set", []string{"--set"}, "Set a config value as KEY=VALUE; repeat for multiple changes", "string")
	set.Repeatable = true
	set.ValueHint = "KEY=VALUE"
	return Command{Name: "config", Title: "Config", Description: "Show, validate, migrate or change project configuration", Arguments: []Parameter{}, Options: []Parameter{
		root, set,
		opt("check", []string{"--check"}, "Validate merged global + project config without changing files", "boolean"),
		opt("migrate", []string{"--migrate"}, "Upgrade .update-cli/config.json in place with backup", "boolean"),
		opt("list", []string{"--list"}, "List project configuration files", "boolean"),
		opt("edit", []string{"--edit"}, "Open .update-cli/config.json in the configured editor", "boolean"),
		opt("use-template", []string{"--use-template"}, "Apply a configuration template", "string"),
		jsonOpt, noColor,
	}, Commands: []Command{}}
}

func templatesCommand(root, details, noColor Parameter) Command {
	return Command{Name: "templates", Title: "Templates", Description: "Manage Update CLI configuration templates", Arguments: []Parameter{}, Options: []Parameter{
		root,
		opt("list", []string{"--list"}, "List configuration templates", "boolean"),
		opt("edit", []string{"--edit"}, "Edit project-local templates.json override file", "boolean"),
		opt("use", []string{"--use"}, "Apply a configuration template", "string"),
		details, noColor,
	}, Commands: []Command{}}
}

func enumParam(name string, flags []string, source Parameter) Parameter {
	p := opt(name, flags, source.Description, source.Type)
	p.Required = false
	p.Choices = source.Choices
	p.Values = source.Values
	return p
}

func cmd(name, title, description string, arguments, options []Parameter) Command {
	if arguments == nil {
		arguments = []Parameter{}
	}
	if options == nil {
		options = []Parameter{}
	}
	return Command{Name: name, Title: title, Description: description, Arguments: arguments, Options: options, Commands: []Command{}}
}
func arg(name, description, typ string, required bool, position int) Parameter {
	return Parameter{Name: name, Description: description, Type: typ, Required: required, Position: position}
}
func opt(name string, flags []string, description, typ string) Parameter {
	return Parameter{Name: name, Flags: flags, Description: description, Type: typ, Required: false}
}
func enumOpt(name string, flags []string, description string, choices []Choice) Parameter {
	p := opt(name, flags, description, "enum")
	p.Choices = choices
	return p
}
func intOpt(name string, flags []string, description string, minimum float64) Parameter {
	p := opt(name, flags, description, "integer")
	p.Minimum = &minimum
	return p
}
func join(base []Parameter, more ...Parameter) []Parameter { return append(base, more...) }
