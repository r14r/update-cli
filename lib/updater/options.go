package updater

import (
	"errors"
	"flag"
	"os"
	"strings"
)

type options struct {
	archive, downloadDir, sourceType, sourceFolder, sourceURL, repository, rootDir, projectName, setupManifest, setupTask, setupWorkflow                                                                                                                                                                                                                                  string
	dryRun, plan, allowDowngrade, jsonOutput, update, backup, rollback, history, cleanup, init, upgrade, check, doctor, status, list, verify, setup, noSetup, config, configList, templatesMode, templatesList, setupList, convertYAML, createYAML, createSetupScript, details, edit, force, noColor, noUI, noAsk, wait, noWait, showHelp, showHowTo, showVersion, unlock bool
	rollbackVersion, restore, useTemplate, templateUse, templateName                                                                                                                                                                                                                                                                                                      string
	keep, limit                                                                                                                                                                                                                                                                                                                                                           int
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
	fs.BoolVar(&o.noSetup, "no-setup", false, "")
	fs.BoolVar(&o.config, "config", false, "")
	fs.BoolVar(&o.templatesMode, "templates", false, "")
	fs.BoolVar(&o.details, "details", false, "")
	fs.StringVar(&o.templateUse, "use", "", "")
	fs.BoolVar(&o.edit, "edit", false, "")
	fs.StringVar(&o.useTemplate, "use-template", "", "")
	fs.BoolVar(&o.force, "force", false, "")
	fs.BoolVar(&o.force, "f", false, "")
	fs.BoolVar(&o.noColor, "no-color", false, "")
	fs.BoolVar(&o.noUI, "no-ui", false, "")
	fs.BoolVar(&o.noAsk, "no-ask", false, "")
	fs.BoolVar(&o.wait, "wait", false, "")
	fs.BoolVar(&o.noWait, "no-wait", false, "")
	fs.BoolVar(&o.showHelp, "help", false, "")
	fs.BoolVar(&o.showHelp, "h", false, "")
	fs.BoolVar(&o.showHowTo, "howto", false, "")
	fs.BoolVar(&o.showVersion, "version", false, "")
	fs.BoolVar(&o.showVersion, "V", false, "")
	fs.BoolVar(&o.unlock, "unlock", false, "")
	if err := fs.Parse(normalizeFlagArguments(args)); err != nil {
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
	for _, b := range []bool{o.update, standaloneBackup, o.rollback, o.restore != "", o.history, o.cleanup, o.init, o.upgrade, o.check, o.doctor, o.status, o.list, o.verify, o.config, o.templatesMode, o.showHelp, o.showHowTo, o.showVersion, o.unlock, o.setupManifest != "", setupSelectorMode, setupManageMode} {
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
	if o.plan && !(o.update || o.cleanup) {
		return o, errors.New("--plan ist nur mit --update oder --cleanup zulässig")
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
	if o.keep != -1 && !o.cleanup {
		return o, errors.New("--keep ist nur mit --cleanup zulässig")
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

func normalizeFlagArguments(args []string) []string {
	for i, arg := range args {
		if arg == "---no-ui" {
			args[i] = "--no-ui"
		}
		if arg == "-create-setup-script" {
			args[i] = "--create-setup-script"
		}
	}
	value := map[string]bool{"--archive": true, "-a": true, "--downloads": true, "-d": true, "--from": true, "--folder": true, "--url": true, "--repository": true, "--root": true, "-r": true, "--restore": true, "--keep": true, "--limit": true, "--use-template": true, "--use": true, "--setup-manifest": true, "--setup-task": true, "--setup-workflow": true}
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
