package updater

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

type options struct {
	archive, downloadDir, sourceType, sourceFolder, sourceURL, repository, rootDir, projectName, setupManifest, setupTask, setupWorkflow                                                                                                                                                                                                                                                 string
	dryRun, plan, allowDowngrade, jsonOutput, update, backup, rollback, history, cleanup, clean, init, upgrade, check, doctor, status, list, verify, setup, noSetup, config, configList, templatesMode, templatesList, setupList, convertYAML, createYAML, createSetupScript, withAI, details, edit, force, noColor, noUI, noAsk, wait, noWait, showHelp, showHowTo, showVersion, unlock bool
	rollbackVersion, restore, useTemplate, templateUse, templateName                                                                                                                                                                                                                                                                                                                     string
	keep, limit                                                                                                                                                                                                                                                                                                                                                                          int
	configSet                                                                                                                                                                                                                                                                                                                                                                            []string
}

func parseOptions(args []string) (options, error) {
	o := options{keep: -1, limit: 20}
	fs := flag.NewFlagSet("update-cli", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&o.archive, "archive", "", "")
	fs.StringVar(&o.archive, "a", "", "")
	fs.StringVar(&o.downloadDir, "downloads", "", "")
	fs.StringVar(&o.downloadDir, "d", "", "")
	fs.StringVar(&o.sourceType, "from", "", "")
	fs.StringVar(&o.sourceFolder, "folder", "", "")
	fs.StringVar(&o.sourceURL, "url", "", "")
	fs.StringVar(&o.repository, "repository", "", "")
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
	fs.BoolVar(&o.upgrade, "upgrade", false, "")
	fs.BoolVar(&o.check, "check", false, "")
	fs.BoolVar(&o.doctor, "doctor", false, "")
	fs.BoolVar(&o.status, "status", false, "")
	fs.BoolVar(&o.list, "list", false, "")
	fs.BoolVar(&o.verify, "verify", false, "")
	fs.BoolVar(&o.setup, "setup", false, "")
	fs.StringVar(&o.setupManifest, "setup-manifest", "", "")
	fs.BoolVar(&o.setupList, "setup-list", false, "")
	fs.StringVar(&o.setupTask, "setup-task", "", "")
	fs.StringVar(&o.setupWorkflow, "setup-workflow", "", "")
	fs.BoolVar(&o.convertYAML, "convert-yaml", false, "")
	fs.BoolVar(&o.createYAML, "create-yaml", false, "")
	fs.BoolVar(&o.createSetupScript, "create-setup-script", false, "")
	fs.BoolVar(&o.withAI, "with-ai", false, "")
	fs.BoolVar(&o.noSetup, "no-setup", false, "")
	fs.BoolVar(&o.config, "config", false, "")
	fs.Var((*stringListFlag)(&o.configSet), "set", "")
	fs.BoolVar(&o.templatesMode, "templates", false, "")
	fs.BoolVar(&o.details, "details", false, "")
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
	normalizedArgs := normalizeFlagArguments(normalizeCommandArguments(append([]string(nil), args...)))
	if err := validateKnownFlags(fs, normalizedArgs); err != nil {
		return o, err
	}
	if err := fs.Parse(normalizedArgs); err != nil {
		return o, err
	}
	if o.config && o.list {
		o.configList = true
		o.list = false
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
	setupSelectorMode := (o.setupList || o.setupTask != "" || o.setupWorkflow != "") && o.setupManifest == ""
	setupManageMode := o.convertYAML || o.createYAML || o.createSetupScript
	primary := 0
	for _, b := range []bool{o.update, standaloneBackup, o.rollback, o.restore != "", o.history, o.cleanup, o.clean, o.init, o.upgrade, o.check, o.doctor, o.status, o.list, o.verify, o.config, o.templatesMode, o.showHelp, o.showHowTo, o.showVersion, o.unlock, o.setupManifest != "", setupSelectorMode, setupManageMode} {
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
	if o.setupTask != "" && o.setupWorkflow != "" {
		return o, errors.New("--setup-task und --setup-workflow schließen sich aus")
	}
	if o.setupManifest == "" && (o.setupList || o.setupTask != "" || o.setupWorkflow != "") && (o.update || o.rollback || o.restore != "" || o.setup) {
		return o, errors.New("--setup-list/--setup-task/--setup-workflow sind eigenständige Setup-Befehle")
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
	if o.details && !(o.templatesMode && o.templatesList) && !o.setup && o.setupManifest == "" && !o.setupList && o.setupTask == "" && o.setupWorkflow == "" && !setupManageMode {
		return o, errors.New("--details ist nur mit --templates --list oder Setup zulässig")
	}
	if o.noAsk && !o.check {
		return o, errors.New("--no-ask ist nur mit --check zulässig")
	}
	if o.wait && o.noWait {
		return o, errors.New("--wait und --no-wait schließen sich aus")
	}
	if (o.wait || o.noWait) && !(o.check || o.update || o.setup || o.setupManifest != "" || o.setupTask != "" || o.setupWorkflow != "" || (o.rollback && o.setup)) {
		return o, errors.New("--wait/--no-wait sind nur mit --check, --update oder Setup zulässig")
	}
	if len(o.configSet) > 0 && !o.config {
		return o, errors.New("--set ist nur mit config/--config zulässig")
	}
	if len(o.configSet) > 0 && (o.configList || o.edit || o.useTemplate != "") {
		return o, errors.New("config --set kann nicht mit --list, --edit oder --use-template kombiniert werden")
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
		return o, errors.New("keine Betriebsart angegeben")
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

	switch command {
	case "help":
		return prepend("--help")
	case "check":
		return prepend("--check")
	case "update":
		return prepend("--update")
	case "backup":
		return prepend("--backup")
	case "rollback":
		return prepend("--rollback")
	case "restore":
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			value := rest[0]
			return append([]string{"--restore", value}, rest[1:]...)
		}
		return prepend("--restore")
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
		return prepend("--doctor")
	case "status":
		return prepend("--status")
	case "list":
		return prepend("--list")
	case "verify":
		return prepend("--verify")
	case "unlock":
		return prepend("--unlock")
	case "convert-yaml":
		return prepend("--convert-yaml")
	case "create-yaml":
		return prepend("--create-yaml")
	case "create-setup-script":
		return prepend("--create-setup-script")
	case "setup":
		if len(rest) == 0 || strings.HasPrefix(rest[0], "-") {
			return prepend("--setup")
		}
		sub := strings.ToLower(strings.TrimSpace(rest[0]))
		rest = rest[1:]
		switch sub {
		case "list":
			return append([]string{"--setup-list"}, rest...)
		case "task":
			return position("--setup-task")
		case "workflow":
			return position("--setup-workflow")
		case "manifest":
			return position("--setup-manifest")
		default:
			return append([]string{"--setup", sub}, rest...)
		}
	case "config":
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

func normalizeFlagArguments(args []string) []string {
	for i, arg := range args {
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
	value := map[string]bool{"--archive": true, "-a": true, "--downloads": true, "-d": true, "--from": true, "--folder": true, "--url": true, "--repository": true, "--root": true, "-r": true, "--restore": true, "--keep": true, "--limit": true, "--use-template": true, "--use": true, "--setup-manifest": true, "--setup-task": true, "--setup-workflow": true, "--set": true}
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
