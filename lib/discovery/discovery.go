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
	details := opt("details", []string{"--details"}, "Show detailed setup/template output", "boolean")

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
	rollbackArg.Values = &ValueSource{Type: "command", Args: []string{"list", "--json"}, ItemsField: "releases", ValueField: "version", LabelTemplate: "{{version}}"}
	restoreArg := arg("backup", "Backup to restore", "enum", true, 1)
	restoreArg.Choices = []Choice{{Value: "latest", Label: "latest (most recent backup)"}}
	restoreArg.Values = &ValueSource{Type: "command", Args: []string{"list", "--json"}, ItemsField: "backups", ValueField: "name", LabelTemplate: "{{name}} ({{version}})"}

	setupTaskArg := arg("name", "Setup task name", "enum", true, 1)
	setupTaskArg.Values = &ValueSource{Type: "command", Args: []string{"setup", "list", "--json"}, ItemsField: "tasks", ValueField: "name", LabelTemplate: "{{name}}"}
	setupWorkflowArg := arg("name", "Setup workflow name", "enum", true, 1)
	setupWorkflowArg.Values = &ValueSource{Type: "command", Args: []string{"setup", "list", "--json"}, ItemsField: "workflows", ValueField: "name", LabelTemplate: "{{name}}"}

	commands := []Command{
		cmd("check", "Check", "Check for an available project update", nil, join([]Parameter{root, jsonOpt, opt("no-ask", []string{"--no-ask"}, "Do not ask to install an available update", "boolean"), wait, noWait, noUI, noColor}, sourceOpts...)),
		cmd("update", "Update", "Install a new project release", []Parameter{arg("archive", "Optional release ZIP archive", "file", false, 1)}, join([]Parameter{root, opt("archive", []string{"--archive", "-a"}, "Release ZIP archive", "file"), opt("dry-run", []string{"--dry-run", "-n"}, "Preview update without applying it", "boolean"), opt("plan", []string{"--plan"}, "Create an update plan without applying changes", "boolean"), opt("allow-downgrade", []string{"--allow-downgrade"}, "Allow installing an older project version", "boolean"), jsonOpt, opt("backup", []string{"--backup"}, "Create a backup before updating", "boolean"), opt("setup", []string{"--setup"}, "Run project setup after update without asking", "boolean"), opt("no-setup", []string{"--no-setup"}, "Do not run project setup after update", "boolean"), opt("force", []string{"--force", "-f"}, "Force update where supported", "boolean"), wait, noWait, noUI, noColor}, sourceOpts...)),
		cmd("backup", "Backup", "Create a project backup", nil, []Parameter{root, jsonOpt, noColor}),
		cmd("rollback", "Rollback", "Restore a previous validated release", []Parameter{rollbackArg}, []Parameter{root, opt("setup", []string{"--setup"}, "Run project setup after rollback", "boolean"), jsonOpt, wait, noWait, noUI, noColor}),
		cmd("restore", "Restore", "Restore a project backup", []Parameter{restoreArg}, []Parameter{root, jsonOpt, noColor}),
		cmd("status", "Status", "Show project and release status", nil, join([]Parameter{root, jsonOpt, noColor}, sourceOpts...)),
		cmd("list", "List", "List releases and backups", nil, join([]Parameter{root, jsonOpt, noColor}, sourceOpts...)),
		cmd("verify", "Verify", "Verify a release ZIP archive", []Parameter{arg("archive", "Release ZIP archive", "file", true, 1)}, join([]Parameter{root, opt("archive", []string{"--archive", "-a"}, "Release ZIP archive", "file"), jsonOpt, noColor}, sourceOpts...)),
		cmd("doctor", "Doctor", "Run project diagnostics", nil, []Parameter{root, jsonOpt, noColor}),
		cmd("run", "Run", "Run the application command from update-cli.yaml", nil, []Parameter{root, noColor}),
		cmd("clean", "Clean releases", "Remove obsolete release directory entries only", nil, []Parameter{root, intOpt("keep", []string{"--keep"}, "Number of releases to retain", 0), opt("plan", []string{"--plan"}, "Show what would be removed", "boolean"), jsonOpt, noColor}),
		cmd("cleanup", "Cleanup", "Apply configured release and backup retention", nil, []Parameter{root, intOpt("keep", []string{"--keep"}, "Number of releases/backups to retain", 0), opt("plan", []string{"--plan"}, "Show what would be removed", "boolean"), jsonOpt, noColor}),
		cmd("history", "History", "Show update history", nil, []Parameter{root, intOpt("limit", []string{"--limit"}, "Maximum history entries", 1), jsonOpt, noColor}),
		cmd("init", "Initialize", "Initialize Update CLI configuration for a project", []Parameter{arg("projectName", "Project name", "string", true, 1)}, []Parameter{root, modeOpt, enumOpt("from", []string{"--from"}, "Initial source type", []Choice{{Value: "download"}, {Value: "url"}, {Value: "repository"}}), opt("folder", []string{"--folder"}, "Release source folder", "directory"), opt("url", []string{"--url"}, "Release source URL", "url"), opt("repository", []string{"--repository"}, "Release repository", "string"), opt("use-template", []string{"--use-template"}, "Apply an initialization template", "string"), opt("force", []string{"--force", "-f"}, "Overwrite existing initialization where supported", "boolean"), noColor}),
		cmd("upgrade", "Upgrade config", "Upgrade project configuration to the current schema", nil, []Parameter{root, jsonOpt, noColor}),
		cmd("unlock", "Unlock", "Remove a stale update lock", nil, []Parameter{root}),
		setupCommand(root, jsonOpt, details, wait, noWait, noUI, noColor, setupTaskArg, setupWorkflowArg),
		cmd("convert-yaml", "Convert YAML", "Upgrade update-cli.yaml to the latest supported schema", nil, []Parameter{root, opt("dry-run", []string{"--dry-run", "-n"}, "Preview the converted manifest", "boolean"), opt("force", []string{"--force", "-f"}, "Force replacement where supported", "boolean"), details, noColor}),
		cmd("create-yaml", "Create YAML", "Generate schemaVersion 2 update-cli.yaml", nil, []Parameter{root, enumOpt("from", []string{"--from"}, "Generation source", []Choice{{Value: "project"}, {Value: "setup-script"}}), opt("with-ai", []string{"--with-ai"}, "Refine setup.sh conversion with configured AI provider", "boolean"), opt("force", []string{"--force", "-f"}, "Overwrite an existing manifest", "boolean"), opt("dry-run", []string{"--dry-run", "-n"}, "Preview generated YAML", "boolean"), details, noColor}),
		cmd("create-setup-script", "Create setup script", "Generate a setup.sh bootstrap", nil, []Parameter{root, opt("force", []string{"--force", "-f"}, "Overwrite an existing setup.sh", "boolean"), opt("dry-run", []string{"--dry-run", "-n"}, "Preview generated script", "boolean"), details, noColor}),
		configCommand(root, jsonOpt, noColor),
		templatesCommand(root, details, noColor),
	}
	return CLI{SchemaVersion: 1, Name: "update-cli", Version: version, Description: "Safe release updater and project setup runner", Executable: "update-cli", Capabilities: Capabilities{StructuredOutput: []string{"json"}, StreamingOutput: []string{}}, GlobalOptions: []Parameter{}, Commands: commands}
}

func Marshal(version string) ([]byte, error) { return json.MarshalIndent(Build(version), "", "  ") }

func setupCommand(root, jsonOpt, details, wait, noWait, noUI, noColor Parameter, taskArg, workflowArg Parameter) Command {
	return Command{Name: "setup", Title: "Setup", Description: "Run project setup automation", Arguments: []Parameter{}, Options: []Parameter{root, details, wait, noWait, noUI, noColor}, Commands: []Command{
		cmd("list", "List setup", "List available setup workflows and tasks", nil, []Parameter{root, jsonOpt, noColor}),
		cmd("task", "Run task", "Run one setup task", []Parameter{taskArg}, []Parameter{root, details, wait, noWait, noUI, noColor}),
		cmd("workflow", "Run workflow", "Run one setup workflow", []Parameter{workflowArg}, []Parameter{root, details, wait, noWait, noUI, noColor}),
		cmd("manifest", "Run manifest", "Run setup from an explicit manifest file", []Parameter{arg("file", "update-cli.yaml manifest", "file", true, 1)}, []Parameter{details, wait, noWait, noUI, noColor}),
	}}
}

func configCommand(root, jsonOpt, noColor Parameter) Command {
	set := opt("set", []string{"--set"}, "Set a config value as KEY=VALUE; repeat for multiple changes", "string")
	set.Repeatable = true
	set.ValueHint = "KEY=VALUE"
	return Command{Name: "config", Title: "Config", Description: "Show, validate, migrate or change project configuration", Arguments: []Parameter{}, Options: []Parameter{root, set, jsonOpt, noColor}, Commands: []Command{
		cmd("check", "Check config", "Validate config.json without changing it", nil, []Parameter{root, jsonOpt, noColor}),
		cmd("migrate", "Migrate config", "Migrate config.json to the current schema with backup", nil, []Parameter{root, jsonOpt, noColor}),
		cmd("list", "List config files", "List project configuration files", nil, []Parameter{root, noColor}),
		cmd("edit", "Edit config", "Open config.json in the configured editor", nil, []Parameter{root, noColor}),
		cmd("use-template", "Use config template", "Apply a configuration template", []Parameter{arg("name", "Template name", "string", true, 1)}, []Parameter{root, noColor}),
	}}
}

func templatesCommand(root, details, noColor Parameter) Command {
	return Command{Name: "templates", Title: "Templates", Description: "Manage Update CLI configuration templates", Arguments: []Parameter{}, Options: []Parameter{root, noColor}, Commands: []Command{
		cmd("list", "List templates", "List configuration templates", nil, []Parameter{root, details, noColor}),
		cmd("edit", "Edit templates", "Open templates.json in the configured editor", nil, []Parameter{root, noColor}),
		cmd("use", "Use template", "Apply a configuration template", []Parameter{arg("name", "Template name", "string", true, 1)}, []Parameter{root, noColor}),
	}}
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
