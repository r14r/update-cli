package updater

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

type options struct {
	archive, downloadDir, mode, sourceType, sourceFolder, sourceURL, repository, fromRepository, rootDir, projectName, setupManifest, setupTask, setupWorkflow, setupStep, helpTopic                                                                                                                                                                                                                                                                                                                                               string
	dryRun, plan, allowDowngrade, jsonOutput, update, backup, rollback, history, cleanup, clean, init, upgrade, check, doctor, fix, status, list, verify, setup, noSetup, config, configList, configCheck, configMigrate, templatesMode, templatesList, setupList, convertYAML, createYAML, createSetupScript, withAI, details, debug, edit, force, noColor, noUI, noAsk, wait, noWait, showHelp, showHowTo, showVersion, unlock, run, install, schema, schemaView, schemaVersion, doctorMigrate, doctorFix, noParameterInvocation bool
	rollbackVersion, restore, useTemplate, templateUse, templateName, schemaSave                                                                                                                                                                                                                                                                                                                                                                                                                                                   string
	keep, limit                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    int
	configSet                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      []string
}

func parseOptions(args []string) (options, error) {
	o := options{keep: -1, limit: 20}
	fs := flag.NewFlagSet("update-cli", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&o.archive, "archive", "", "")
	fs.StringVar(&o.archive, "a", "", "")
	fs.StringVar(&o.downloadDir, "downloads", "", "")
	fs.StringVar(&o.downloadDir, "d", "", "")
	fs.StringVar(&o.mode, "mode", "", "")
	fs.StringVar(&o.sourceType, "from", "", "")
	fs.StringVar(&o.sourceFolder, "folder", "", "")
	fs.StringVar(&o.sourceURL, "url", "", "")
	fs.StringVar(&o.repository, "repository", "", "")
	fs.StringVar(&o.projectName, "project", "", "")
	fs.StringVar(&o.rollbackVersion, "rollback-version", "", "")
	fs.StringVar(&o.restore, "snapshot", "", "")
	fs.StringVar(&o.rootDir, "root", "", "")
	fs.StringVar(&o.rootDir, "r", "", "")
	fs.BoolVar(&o.dryRun, "dry-run", false, "")
	fs.BoolVar(&o.dryRun, "n", false, "")
	fs.BoolVar(&o.plan, "plan", false, "")
	fs.BoolVar(&o.allowDowngrade, "allow-downgrade", false, "")
	fs.BoolVar(&o.jsonOutput, "json", false, "")
	fs.BoolVar(&o.update, "update", false, "")
	fs.BoolVar(&o.backup, "backup", false, "")
	fs.BoolVar(&o.rollback, "rollback", false, "")
	fs.StringVar(&o.restore, "restore", "", "")
	fs.BoolVar(&o.history, "history", false, "")
	fs.BoolVar(&o.cleanup, "cleanup", false, "")
	fs.BoolVar(&o.clean, "clean", false, "")
	fs.IntVar(&o.keep, "keep", -1, "")
	fs.IntVar(&o.limit, "limit", 20, "")
	fs.BoolVar(&o.init, "init", false, "")
	fs.StringVar(&o.fromRepository, "from-repository", "", "")
	fs.BoolVar(&o.upgrade, "upgrade", false, "")
	fs.BoolVar(&o.check, "check", false, "")
	fs.BoolVar(&o.doctor, "doctor", false, "")
	fs.BoolVar(&o.fix, "fix", false, "")
	fs.BoolVar(&o.doctorMigrate, "migrate", false, "")
	fs.BoolVar(&o.status, "status", false, "")
	fs.BoolVar(&o.list, "list", false, "")
	fs.BoolVar(&o.verify, "verify", false, "")
	fs.BoolVar(&o.setup, "setup", false, "")
	fs.StringVar(&o.setupManifest, "setup-manifest", "", "")
	fs.StringVar(&o.setupManifest, "manifest", "", "")
	fs.BoolVar(&o.setupList, "setup-list", false, "")
	fs.StringVar(&o.setupTask, "setup-task", "", "")
	fs.StringVar(&o.setupTask, "task", "", "")
	fs.StringVar(&o.setupWorkflow, "setup-workflow", "", "")
	fs.StringVar(&o.setupWorkflow, "workflow", "", "")
	fs.StringVar(&o.setupStep, "setup-step", "", "")
	fs.StringVar(&o.helpTopic, "help-topic", "", "")
	fs.StringVar(&o.helpTopic, "command", "", "")
	fs.BoolVar(&o.convertYAML, "convert-yaml", false, "")
	fs.BoolVar(&o.createYAML, "create-yaml", false, "")
	fs.BoolVar(&o.createSetupScript, "create-setup-script", false, "")
	fs.BoolVar(&o.withAI, "with-ai", false, "")
	fs.BoolVar(&o.noSetup, "no-setup", false, "")
	fs.BoolVar(&o.config, "config", false, "")
	fs.BoolVar(&o.configCheck, "config-check", false, "")
	fs.BoolVar(&o.configMigrate, "config-migrate", false, "")
	fs.Var((*stringListFlag)(&o.configSet), "set", "")
	fs.BoolVar(&o.templatesMode, "templates", false, "")
	fs.BoolVar(&o.details, "details", false, "")
	fs.BoolVar(&o.debug, "debug", false, "")
	fs.StringVar(&o.templateUse, "use", "", "")
	fs.BoolVar(&o.edit, "edit", false, "")
	fs.StringVar(&o.useTemplate, "use-template", "", "")
	fs.BoolVar(&o.force, "force", false, "")
	fs.BoolVar(&o.force, "f", false, "")
	fs.BoolVar(&o.noColor, "no-color", false, "")
	fs.BoolVar(&o.noUI, "no-ui", false, "")
	fs.BoolVar(&o.noUI, "noui", false, "")
	fs.BoolVar(&o.noAsk, "no-ask", false, "")
	fs.BoolVar(&o.wait, "wait", false, "")
	fs.BoolVar(&o.noWait, "no-wait", false, "")
	fs.BoolVar(&o.showHelp, "help", false, "")
	fs.BoolVar(&o.showHelp, "h", false, "")
	fs.BoolVar(&o.showHowTo, "howto", false, "")
	fs.BoolVar(&o.showVersion, "version", false, "")
	fs.BoolVar(&o.showVersion, "V", false, "")
	fs.BoolVar(&o.unlock, "unlock", false, "")
	fs.BoolVar(&o.run, "run", false, "")
	fs.BoolVar(&o.install, "install", false, "")
	fs.BoolVar(&o.schema, "schema", false, "")
	fs.BoolVar(&o.schemaView, "view", false, "")
	fs.StringVar(&o.schemaSave, "save", "", "")
	argsWithoutDebug, debugRequested := stripGlobalDebugArgument(append([]string(nil), args...))
	normalizedArgs := normalizeFlagArguments(normalizeCommandArguments(normalizeLegacyFromRepositoryArguments(argsWithoutDebug)))
	if debugRequested {
		normalizedArgs = append([]string{"--debug"}, normalizedArgs...)
	}
	if err := validateKnownFlags(fs, normalizedArgs); err != nil {
		return o, err
	}
	if err := fs.Parse(normalizedArgs); err != nil {
		return o, err
	}
	// doctor --fix reuses the historical --fix flag as a doctor parameter.
	// Normalize it before primary-mode validation so it is not treated as a
	// second top-level command.
	if o.doctor && o.fix {
		o.doctorFix = true
		o.fix = false
	}
	if o.doctorFix && o.doctorMigrate {
		return o, errors.New("doctor --fix und --migrate schließen sich gegenseitig aus")
	}
	if o.doctorFix && o.jsonOutput {
		return o, errors.New("doctor --fix ist interaktiv und kann nicht mit --json kombiniert werden")
	}
	// Public parameter aliases are translated after flag parsing.
	if o.rollback && o.showVersion {
		return o, errors.New("rollback --version benötigt einen Versionswert")
	}
	o.mode = strings.ToLower(strings.TrimSpace(o.mode))
	if strings.TrimSpace(o.fromRepository) != "" {
		if !o.init {
			return o, errors.New("--from-repository ist nur mit --init zulässig")
		}
		if strings.TrimSpace(o.sourceType) != "" && !strings.EqualFold(strings.TrimSpace(o.sourceType), "repository") {
			return o, errors.New("--from-repository kann nicht mit einer anderen --from-Quelle kombiniert werden")
		}
		if o.mode != "" && o.mode != "pull" {
			return o, errors.New("--from-repository benötigt mode pull")
		}
		if strings.TrimSpace(o.repository) != "" {
			return o, errors.New("--from-repository REPOSITORY und --repository dürfen nicht kombiniert werden")
		}
		o.sourceType = "repository"
		o.mode = "pull"
	}

	if o.schema && o.showVersion {
		o.schemaVersion = true
		o.showVersion = false
	}
	if (o.schemaView || strings.TrimSpace(o.schemaSave) != "" || o.schemaVersion) && !o.schema {
		return o, errors.New("--view/--save sind nur mit schema zulässig")
	}
	if o.doctorMigrate && !o.doctor {
		return o, errors.New("--migrate ist nur mit doctor zulässig; für Runtime-Konfiguration 'config --migrate' verwenden")
	}
	if o.schema {
		actions := boolInt(o.schemaView) + boolInt(strings.TrimSpace(o.schemaSave) != "") + boolInt(o.schemaVersion)
		if actions == 0 {
			return o, errors.New("schema benötigt --view, --save <datei.json> oder --version")
		}
		if actions > 1 {
			return o, errors.New("schema --view, --save und --version schließen sich gegenseitig aus")
		}
	}

	if o.mode != "" && o.mode != "update" && o.mode != "pull" {
		return o, errors.New("--mode unterstützt nur update oder pull")
	}
	if o.config && o.list {
		o.configList = true
		o.list = false
	}
	// `setup --list` is a setup subcommand, not a second top-level mode.
	// Normalize the legacy flag spelling before primary-mode validation.
	if o.setup && o.list {
		o.setupList = true
		o.list = false
		o.setup = false
	}
	if o.setup && (strings.TrimSpace(o.setupManifest) != "" || strings.TrimSpace(o.setupTask) != "" || strings.TrimSpace(o.setupWorkflow) != "" || strings.TrimSpace(o.setupStep) != "") {
		o.setup = false
	}
	if (o.configCheck || o.configMigrate) && !o.config {
		return o, errors.New("config --check/--migrate sind nur mit config zulässig")
	}
	if o.templatesMode && o.list {
		o.templatesList = true
		o.list = false
	}
	rest := fs.Args()
	if len(rest) > 1 {
		return o, errors.New("es darf nur ein positionsabhängiges Argument angegeben werden")
	}
	if len(rest) == 1 {
		switch {
		case o.update || o.verify:
			o.archive = rest[0]
		case o.rollback:
			o.rollbackVersion = rest[0]
		case o.init:
			o.projectName = rest[0]
		case o.templatesMode && o.edit:
			o.templateName = rest[0]
		default:
			return o, errors.New("positionsabhängiges Argument ist mit dieser Betriebsart nicht zulässig")
		}
	}
	standaloneBackup := o.backup && !o.update
	setupSelectorMode := (o.setupList || o.setupTask != "" || o.setupWorkflow != "" || o.setupStep != "") && o.setupManifest == ""
	setupManageMode := o.convertYAML || o.createYAML || o.createSetupScript
	if setupSelectorMode && (o.update || o.rollback || o.restore != "") {
		return o, errors.New("setup --list/task/workflow/run sind eigenständige Befehle; update/rollback/restore entfernen")
	}
	primary := 0
	for _, b := range []bool{o.update, standaloneBackup, o.rollback, o.restore != "", o.history, o.cleanup, o.clean, o.init, o.upgrade, o.check, o.doctor, o.fix, o.status, o.list, o.verify, o.config, o.templatesMode, o.showHelp, o.showHowTo, o.showVersion, o.unlock, o.run, o.install, o.schema, o.setupManifest != "", setupSelectorMode, setupManageMode} {
		if b {
			primary++
		}
	}
	if primary > 1 {
		return o, errors.New("Betriebsarten schließen sich gegenseitig aus")
	}
	if (boolInt(o.convertYAML) + boolInt(o.createYAML) + boolInt(o.createSetupScript)) > 1 {
		return o, errors.New("--convert-yaml, --create-yaml und --create-setup-script schließen sich gegenseitig aus")
	}
	if o.createYAML {
		from := strings.ToLower(strings.TrimSpace(o.sourceType))
		if from == "" {
			from = "project"
		}
		if from != "project" && from != "setup-script" {
			return o, errors.New("--create-yaml --from unterstützt nur project oder setup-script")
		}
		if o.withAI && from != "setup-script" {
			return o, errors.New("--with-ai ist nur mit --create-yaml --from setup-script zulässig")
		}
	} else if o.withAI {
		return o, errors.New("--with-ai ist nur mit --create-yaml --from setup-script zulässig")
	}
	if o.setupManifest != "" && (o.setup || o.update || o.rollback || o.restore != "") {
		return o, errors.New("--setup-manifest kann nicht mit Update-/Setup-Modi kombiniert werden")
	}
	if boolInt(o.setupTask != "")+boolInt(o.setupWorkflow != "")+boolInt(o.setupStep != "") > 1 {
		return o, errors.New("--setup-task, --setup-workflow und setup --run <stepid> schließen sich gegenseitig aus")
	}
	if o.setupManifest == "" && (o.setupList || o.setupTask != "" || o.setupWorkflow != "" || o.setupStep != "") && (o.update || o.rollback || o.restore != "" || o.setup) {
		return o, errors.New("setup --list/task/workflow/run sind eigenständige Setup-Befehle und können nicht mit update/rollback/restore kombiniert werden")
	}
	if o.setup && !(o.update || o.rollback || primary == 0) {
		return o, errors.New("--setup kann nur allein oder mit --update/--rollback verwendet werden")
	}
	if o.noSetup && !o.update {
		return o, errors.New("--no-setup ist nur mit --update zulässig")
	}
	if o.setup && o.noSetup {
		return o, errors.New("--setup und --no-setup schließen sich aus")
	}
	if o.plan && !(o.update || o.cleanup || o.clean) {
		return o, errors.New("--plan ist nur mit --update, --cleanup oder --clean zulässig")
	}
	if o.dryRun && !(o.update || setupManageMode) {
		return o, errors.New("--dry-run ist nur mit --update oder Setup-Dateierzeugung zulässig")
	}
	if o.allowDowngrade && !o.update {
		return o, errors.New("--allow-downgrade ist nur mit --update zulässig")
	}
	if o.keep < -1 {
		return o, errors.New("--keep darf nicht kleiner als -1 sein")
	}
	if o.keep != -1 && !(o.cleanup || o.clean) {
		return o, errors.New("--keep ist nur mit --cleanup oder --clean zulässig")
	}
	if o.limit < 1 {
		return o, errors.New("--limit muss mindestens 1 sein")
	}
	if o.limit != 20 && !o.history {
		return o, errors.New("--limit ist nur mit --history zulässig")
	}
	if o.verify && strings.TrimSpace(o.archive) == "" {
		return o, errors.New("--verify benötigt ein Archiv")
	}
	if o.init && strings.TrimSpace(o.projectName) == "" {
		return o, errors.New("--init benötigt den Projektnamen")
	}
	if o.force && !(o.update || o.init || setupManageMode) {
		return o, errors.New("--force ist nur mit --update, --init oder Setup-Dateierzeugung zulässig")
	}
	if o.jsonOutput && o.update && !o.plan {
		return o, errors.New("--json wird bei --update nur zusammen mit --plan unterstützt")
	}
	if o.details && !o.showHelp && !(o.templatesMode && o.templatesList) && !o.setup && o.setupManifest == "" && !o.setupList && o.setupTask == "" && o.setupWorkflow == "" && o.setupStep == "" && !setupManageMode {
		return o, errors.New("--details ist nur mit --templates --list oder Setup zulässig")
	}
	if o.noAsk && !o.check {
		return o, errors.New("--no-ask ist nur mit --check zulässig")
	}
	if o.wait && o.noWait {
		return o, errors.New("--wait und --no-wait schließen sich aus")
	}
	if (o.wait || o.noWait) && !(o.check || o.update || o.setup || o.setupManifest != "" || o.setupTask != "" || o.setupWorkflow != "" || o.setupStep != "" || (o.rollback && o.setup)) {
		return o, errors.New("--wait/--no-wait sind nur mit --check, --update oder Setup zulässig")
	}
	if len(o.configSet) > 0 && !o.config {
		return o, errors.New("--set ist nur mit config/--config zulässig")
	}
	configActions := boolInt(o.configList) + boolInt(o.configCheck) + boolInt(o.configMigrate) + boolInt(o.edit) + boolInt(o.useTemplate != "")
	if len(o.configSet) > 0 && configActions > 0 {
		return o, errors.New("config --set kann nicht mit --list, --check, --migrate, --edit oder --use-template kombiniert werden")
	}
	if configActions > 1 {
		return o, errors.New("config --list, --check, --migrate, --edit und --use-template schließen sich gegenseitig aus")
	}
	if o.edit && !(o.config || o.templatesMode) {
		return o, errors.New("--edit ist nur mit --config oder --templates zulässig")
	}
	if o.useTemplate != "" && !(o.config || o.init) {
		return o, errors.New("--use-template ist nur mit --config oder --init zulässig")
	}
	if o.templateUse != "" && !o.templatesMode {
		return o, errors.New("--use ist nur mit --templates zulässig")
	}
	if primary == 0 && !o.setup {
		if o.debug {
			o.noParameterInvocation = true
		} else {
			return o, errors.New("keine Betriebsart angegeben")
		}
	}
	return o, nil
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

type stringListFlag []string

func (v *stringListFlag) String() string { return strings.Join(*v, ",") }
func (v *stringListFlag) Set(value string) error {
	*v = append(*v, value)
	return nil
}

func stripGlobalDebugArgument(args []string) ([]string, bool) {
	out := make([]string, 0, len(args))
	debug := false
	for _, arg := range args {
		if arg == "--debug" {
			debug = true
			continue
		}
		out = append(out, arg)
	}
	return out, debug
}

func normalizeCommandArguments(args []string) []string {
	if len(args) == 0 {
		return args
	}
	command := strings.ToLower(strings.TrimSpace(args[0]))
	if strings.HasPrefix(command, "-") {
		return args
	}
	rest := append([]string(nil), args[1:]...)
	prepend := func(values ...string) []string { return append(values, rest...) }
	position := func(flag string) []string {
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			value := rest[0]
			rest = rest[1:]
			return append([]string{flag, value}, rest...)
		}
		return append([]string{flag}, rest...)
	}

	// `<command> --help` should show help for that command instead of
	// activating both the command and the global help mode.
	if command != "help" {
		for _, arg := range rest {
			if arg == "--help" || arg == "-h" {
				out := []string{"--help", "--help-topic", command}
				for _, value := range rest {
					if value == "--details" || value == "--json" {
						out = append(out, value)
					}
				}
				return out
			}
		}
	}

	switch command {
	case "help":
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			return append([]string{"--help", "--help-topic", strings.ToLower(strings.TrimSpace(rest[0]))}, rest[1:]...)
		}
		return prepend("--help")
	case "howto":
		return prepend("--howto")
	case "version":
		return prepend("--version")
	case "check":
		return prepend("--check")
	case "update":
		if len(rest) > 0 && strings.EqualFold(strings.TrimSpace(rest[0]), "plan") {
			return append([]string{"--update", "--plan"}, rest[1:]...)
		}
		return prepend("--update")
	case "backup":
		return prepend("--backup")
	case "rollback":
		return prepend("--rollback")
	case "restore":
		return append([]string{"--restore", "latest"}, rest...)
	case "history":
		return prepend("--history")
	case "cleanup":
		return prepend("--cleanup")
	case "clean":
		return prepend("--clean")
	case "init":
		return prepend("--init")
	case "upgrade":
		return prepend("--upgrade")
	case "doctor":
		if len(rest) > 0 && strings.EqualFold(strings.TrimSpace(rest[0]), "migrate") {
			return append([]string{"--doctor", "--migrate"}, rest[1:]...)
		}
		return prepend("--doctor")
	case "fix":
		return prepend("--fix")
	case "status":
		return prepend("--status")
	case "releases":
		if len(rest) == 0 {
			return []string{"--list"}
		}
		sub := strings.ToLower(strings.TrimSpace(rest[0]))
		if sub == "list" || sub == "--list" {
			return append([]string{"--list"}, rest[1:]...)
		}
		return prepend("--list")
	case "list":
		// Compatibility alias for releases --list.
		return prepend("--list")
	case "verify":
		return prepend("--verify")
	case "unlock":
		return prepend("--unlock")
	case "run":
		return prepend("--run")
	case "install":
		return prepend("--install")
	case "schema":
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			sub := strings.ToLower(strings.TrimSpace(rest[0]))
			switch sub {
			case "view":
				return append([]string{"--schema", "--view"}, rest[1:]...)
			case "version":
				return append([]string{"--schema", "--version"}, rest[1:]...)
			case "save":
				if len(rest) > 1 {
					return append([]string{"--schema", "--save", rest[1]}, rest[2:]...)
				}
				return []string{"--schema", "--save"}
			}
		}
		return prepend("--schema")
	case "convert-yaml":
		return prepend("--convert-yaml")
	case "create-yaml":
		return prepend("--create-yaml")
	case "create-setup-script":
		return prepend("--create-setup-script")
	case "setup":
		// Support both command-first and familiar flag spelling:
		//   setup list / setup --list
		//   setup run STEP / setup --run STEP
		if len(rest) > 0 {
			sub := strings.ToLower(strings.TrimSpace(rest[0]))
			switch sub {
			case "list", "--list":
				return append([]string{"--setup-list"}, rest[1:]...)
			case "run", "--run":
				if len(rest) > 1 {
					return append([]string{"--setup-step", rest[1]}, rest[2:]...)
				}
				return []string{"--setup-step"}
			case "task":
				rest = rest[1:]
				return position("--setup-task")
			case "workflow":
				rest = rest[1:]
				return position("--setup-workflow")
			case "manifest":
				rest = rest[1:]
				return position("--setup-manifest")
			}
		}
		return prepend("--setup")
	case "config":
		if len(rest) > 0 {
			sub := strings.ToLower(strings.TrimSpace(rest[0]))
			switch sub {
			case "check", "--check":
				return append([]string{"--config", "--config-check"}, rest[1:]...)
			case "migrate", "--migrate":
				return append([]string{"--config", "--config-migrate"}, rest[1:]...)
			case "set":
				out := []string{"--config"}
				for _, value := range rest[1:] {
					if strings.HasPrefix(value, "-") {
						out = append(out, value)
					} else {
						out = append(out, "--set", value)
					}
				}
				return out
			}
		}
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			sub := strings.ToLower(strings.TrimSpace(rest[0]))
			rest = rest[1:]
			switch sub {
			case "list":
				return append([]string{"--config", "--list"}, rest...)
			case "edit":
				return append([]string{"--config", "--edit"}, rest...)
			case "use-template":
				if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
					return append([]string{"--config", "--use-template", rest[0]}, rest[1:]...)
				}
				return append([]string{"--config", "--use-template"}, rest...)
			default:
				return append([]string{"--config", sub}, rest...)
			}
		}
		return prepend("--config")
	case "templates":
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			sub := strings.ToLower(strings.TrimSpace(rest[0]))
			rest = rest[1:]
			switch sub {
			case "list":
				return append([]string{"--templates", "--list"}, rest...)
			case "edit":
				return append([]string{"--templates", "--edit"}, rest...)
			case "use":
				if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
					return append([]string{"--templates", "--use", rest[0]}, rest[1:]...)
				}
				return append([]string{"--templates", "--use"}, rest...)
			default:
				return append([]string{"--templates", sub}, rest...)
			}
		}
		return prepend("--templates")
	}
	return args
}

func normalizeLegacyFromRepositoryArguments(args []string) []string {
	fromIndex := -1
	repositoryIndex := -1
	for i, arg := range args {
		switch arg {
		case "--from-repository":
			fromIndex = i
		case "--repository":
			repositoryIndex = i
		}
	}
	if fromIndex < 0 || repositoryIndex < 0 || repositoryIndex+1 >= len(args) {
		return args
	}
	// New syntax already supplies the repository directly after --from-repository.
	if fromIndex+1 < len(args) && !strings.HasPrefix(args[fromIndex+1], "-") {
		return args
	}
	repository := args[repositoryIndex+1]
	if strings.HasPrefix(repository, "-") {
		return args
	}
	out := make([]string, 0, len(args)-1)
	for i := 0; i < len(args); i++ {
		if i == fromIndex {
			out = append(out, "--from-repository", repository)
			continue
		}
		if i == repositoryIndex || i == repositoryIndex+1 {
			continue
		}
		out = append(out, args[i])
	}
	return out
}

func normalizeFlagArguments(args []string) []string {
	hasRollback := false
	for _, arg := range args {
		if arg == "--rollback" {
			hasRollback = true
			break
		}
	}
	if hasRollback {
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "--version" && !strings.HasPrefix(args[i+1], "-") {
				args[i] = "--rollback-version"
			}
		}
	}
	hasSetup := false
	for _, arg := range args {
		if arg == "--setup" {
			hasSetup = true
			break
		}
	}
	if hasSetup {
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "--run" && !strings.HasPrefix(args[i+1], "-") {
				args[i] = "--setup-step"
			}
		}
	}
	hasConfig := false
	for _, arg := range args {
		if arg == "--config" {
			hasConfig = true
			break
		}
	}
	for i, arg := range args {
		if arg == "--migrate" && hasConfig {
			args[i] = "--config-migrate"
			arg = args[i]
		}
		if arg == "---no-ui" {
			args[i] = "--no-ui"
		}
		if arg == "--noui" {
			args[i] = "--no-ui"
		}
		if arg == "-create-setup-script" {
			args[i] = "--create-setup-script"
		}
	}
	value := map[string]bool{"--archive": true, "-a": true, "--downloads": true, "-d": true, "--mode": true, "--from": true, "--folder": true, "--url": true, "--repository": true, "--from-repository": true, "--root": true, "-r": true, "--restore": true, "--keep": true, "--limit": true, "--use-template": true, "--use": true, "--setup-manifest": true, "--setup-task": true, "--setup-workflow": true, "--setup-step": true, "--help-topic": true, "--command": true, "--manifest": true, "--task": true, "--workflow": true, "--project": true, "--rollback-version": true, "--snapshot": true, "--set": true, "--save": true}
	flags := []string{}
	pos := []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if value[a] {
			flags = append(flags, a)
			if i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
		} else {
			pos = append(pos, a)
		}
	}
	return append(flags, pos...)
}

func validateKnownFlags(fs *flag.FlagSet, args []string) error {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			continue
		}

		nameAndValue := strings.TrimLeft(arg, "-")
		if nameAndValue == "" {
			continue
		}
		name, _, hasInlineValue := strings.Cut(nameAndValue, "=")
		f := fs.Lookup(name)
		if f == nil {
			unknown := formatFlagName(name)
			if strings.HasPrefix(arg, "--") {
				unknown = "--" + name
			}
			if suggestion := closestFlagName(fs, name); suggestion != "" {
				return fmt.Errorf("unbekannter Parameter %q; meinten Sie %q?", unknown, suggestion)
			}
			return fmt.Errorf("unbekannter Parameter %q", unknown)
		}
		if !hasInlineValue && flagNeedsValue(f) && i+1 < len(args) {
			i++
		}
	}
	return nil
}

func flagNeedsValue(f *flag.Flag) bool {
	type boolFlag interface {
		IsBoolFlag() bool
	}
	if bf, ok := f.Value.(boolFlag); ok && bf.IsBoolFlag() {
		return false
	}
	return true
}

func closestFlagName(fs *flag.FlagSet, unknown string) string {
	unknown = strings.ToLower(strings.TrimSpace(unknown))
	if unknown == "" {
		return ""
	}

	bestName := ""
	bestDistance := len(unknown) + 1
	fs.VisitAll(func(f *flag.Flag) {
		candidate := strings.ToLower(f.Name)
		distance := levenshteinDistance(unknown, candidate)
		if distance < bestDistance || (distance == bestDistance && len(f.Name) > 1 && len(bestName) == 1) {
			bestDistance = distance
			bestName = f.Name
		}
	})

	threshold := 2
	if len(unknown) >= 9 {
		threshold = 3
	}
	if bestName == "" || bestDistance > threshold {
		return ""
	}
	return formatFlagName(bestName)
}

func formatFlagName(name string) string {
	if len(name) == 1 {
		return "-" + name
	}
	return "--" + name
}

func levenshteinDistance(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	previous := make([]int, len(br)+1)
	current := make([]int, len(br)+1)
	for j := range previous {
		previous[j] = j
	}
	for i, ra := range ar {
		current[0] = i + 1
		for j, rb := range br {
			cost := 0
			if ra != rb {
				cost = 1
			}
			deletion := previous[j+1] + 1
			insertion := current[j] + 1
			substitution := previous[j] + cost
			current[j+1] = minInt(deletion, insertion, substitution)
		}
		previous, current = current, previous
	}
	return previous[len(br)]
}

func minInt(values ...int) int {
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}
	return minimum
}
