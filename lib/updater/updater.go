package updater

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"release-updater/lib/archive"
	"release-updater/lib/backup"
	"release-updater/lib/buildconfig"
	"release-updater/lib/cleanup"
	"release-updater/lib/config"
	"release-updater/lib/doctor"
	"release-updater/lib/editor"
	"release-updater/lib/history"
	"release-updater/lib/inventory"
	"release-updater/lib/projectdocker"
	"release-updater/lib/projectsetup"
	"release-updater/lib/projectstatus"
	"release-updater/lib/rollback"
	rsyncutil "release-updater/lib/rsync"
	"release-updater/lib/source"
	"release-updater/lib/templates"
	"release-updater/lib/tools"
	"release-updater/lib/ui"
	"release-updater/lib/updatecheck"
	versionutil "release-updater/lib/version"
)

type ExitError struct {
	Code int
	Err  error
}

// VersionAlreadyInstalledError indicates that the selected release is already
// active and a reinstall was not explicitly requested.
type VersionAlreadyInstalledError struct {
	Version string
}

func (err *VersionAlreadyInstalledError) Error() string {
	return fmt.Sprintf("Version %s ist bereits installiert", err.Version)
}

func (err *ExitError) Error() string {
	if err.Err == nil {
		return ""
	}
	return err.Err.Error()
}

func (err *ExitError) Unwrap() error { return err.Err }

type options struct {
	archive         string
	downloadDir     string
	sourceType      string
	sourceFolder    string
	sourceURL       string
	repository      string
	rootDir         string
	projectName     string
	dryRun          bool
	plan            bool
	allowDowngrade  bool
	jsonOutput      bool
	update          bool
	backup          bool
	rollback        bool
	rollbackVersion string
	restore         string
	history         bool
	cleanup         bool
	keep            int
	limit           int
	init            bool
	upgrade         bool
	check           bool
	doctor          bool
	status          bool
	list            bool
	verify          bool
	setup           bool
	config          bool
	configList      bool
	templatesMode   bool
	templatesList   bool
	details         bool
	templateUse     string
	templateName    string
	edit            bool
	useTemplate     string
	force           bool
	noColor         bool
	showHelp        bool
	showHowTo       bool
	showVersion     bool
}

type state struct {
	config            config.Config
	archivePath       string
	sourceType        string
	sourceReference   string
	repositoryStage   string
	version           versionutil.Version
	workDir           string
	extractDir        string
	contentDir        string
	releaseDir        string
	releaseStage      string
	dryReleaseDir     string
	dryCurrentDir     string
	releaseChanges    int
	currentChanges    int
	currentPlan       []rsyncutil.Change
	fromVersion       string
	backupPath        string
	dockerComposeFile string
	dockerStopped     bool
}

func Run(ctx context.Context, buildVersion string, args []string) error {
	if len(args) == 0 {
		return runNoParameterAction(ctx, buildVersion)
	}
	if topic, ok := scopedHelpTopic(args); ok {
		printCommandHelp(buildVersion, topic)
		return nil
	}

	opts, err := parseOptions(args)
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}
	if opts.showHelp {
		printHelp(buildVersion)
		return nil
	}
	if opts.showHowTo {
		printHowTo(buildVersion)
		return nil
	}
	if opts.showVersion {
		fmt.Printf("Release Updater %s\n", buildVersion)
		return nil
	}

	console := ui.New(opts.noColor || opts.jsonOutput)
	root, err := config.ResolveRoot(opts.rootDir)
	if err != nil {
		return err
	}
	if opts.init {
		return initialize(console, root, opts)
	}
	if opts.upgrade {
		lock, err := tools.AcquireLock(filepath.Join(root, ".release-update.lock"))
		if err != nil {
			return err
		}
		defer lock.Release()
		result, err := config.Upgrade(root)
		if err != nil {
			return err
		}
		if opts.jsonOutput {
			return writeJSON(result)
		}
		printConfigurationUpgrade(console, result)
		return nil
	}
	if opts.templatesMode {
		switch {
		case opts.templatesList:
			return listTemplates(console, root, opts.details)
		case opts.templateUse != "":
			return applyConfigurationTemplate(console, root, opts.templateUse)
		case opts.edit:
			return editTemplate(ctx, console, root, opts.templateName)
		default:
			printCommandHelp(buildVersion, "templates")
			return nil
		}
	}
	if opts.doctor {
		report := doctor.RunWithSource(root, opts.sourceType, firstNonEmpty(opts.sourceFolder, opts.downloadDir), opts.sourceURL, opts.repository)
		if opts.jsonOutput {
			if err := writeJSON(doctorJSON(report)); err != nil {
				return err
			}
		} else {
			printDoctor(console, report)
		}
		if report.ErrorCount() > 0 {
			return &ExitError{Code: 1}
		}
		return nil
	}
	if opts.config {
		if opts.configList {
			return listConfigurationFiles(console, root)
		}
		if opts.useTemplate != "" {
			if err := applyConfigurationTemplate(console, root, opts.useTemplate); err != nil {
				return err
			}
			if !opts.edit {
				return nil
			}
		}
		if opts.edit {
			return editConfiguration(ctx, console, root)
		}
		return showConfiguration(console, root)
	}

	cfg, err := config.Load(root, opts.downloadDir)
	if err != nil {
		return err
	}
	cfg, err = config.WithSourceOverrides(cfg, opts.sourceType, opts.sourceFolder, opts.sourceURL, opts.repository)
	if err != nil {
		return err
	}
	if opts.history {
		entries, err := history.List(cfg.HistoryFile, opts.limit)
		if err != nil {
			return err
		}
		if opts.jsonOutput {
			return writeJSON(map[string]any{"projectName": cfg.ProjectName, "historyFile": cfg.HistoryFile, "entries": entries})
		}
		printHistory(console, cfg, entries)
		return nil
	}
	if opts.cleanup {
		lock, err := tools.AcquireLock(filepath.Join(cfg.RootDir, ".release-update.lock"))
		if err != nil {
			return err
		}
		defer lock.Release()
		result, err := cleanup.Run(cfg, opts.keep, opts.plan)
		if err != nil {
			return err
		}
		if !opts.plan {
			if err := appendHistory(cfg, history.Entry{Action: "cleanup", ProjectName: cfg.ProjectName, Status: "success", Message: fmt.Sprintf("%d Releases und %d Backups entfernt", len(result.RemovedRelease), len(result.RemovedBackup))}); err != nil {
				return err
			}
		}
		if opts.jsonOutput {
			return writeJSON(result)
		}
		printCleanup(console, result)
		return nil
	}
	if opts.backup && !opts.update {
		lock, err := tools.AcquireLock(filepath.Join(cfg.RootDir, ".release-update.lock"))
		if err != nil {
			return err
		}
		defer lock.Release()
		result, err := backup.Create(ctx, cfg, false)
		if err != nil {
			return err
		}
		if err := appendHistory(cfg, history.Entry{Action: "backup", ProjectName: cfg.ProjectName, FromVersion: result.Backup.Version, Backup: result.Backup.Path, Status: "success"}); err != nil {
			return err
		}
		if opts.jsonOutput {
			return writeJSON(result)
		}
		printBackup(console, result)
		return nil
	}
	if opts.rollback {
		lock, err := tools.AcquireLock(filepath.Join(cfg.RootDir, ".release-update.lock"))
		if err != nil {
			return err
		}
		defer lock.Release()
		release, err := rollback.Resolve(cfg, opts.rollbackVersion)
		if err != nil {
			return err
		}
		result, err := rollback.Apply(ctx, cfg, release, false)
		if err != nil {
			return err
		}
		entry := history.Entry{Action: "rollback", ProjectName: cfg.ProjectName, FromVersion: result.FromVersion, ToVersion: result.ToVersion, Setup: opts.setup, Status: "success"}
		if opts.setup {
			if _, err := projectsetup.Run(ctx, cfg, console); err != nil {
				entry.Status = "setup-failed"
				entry.Message = err.Error()
				_ = appendHistory(cfg, entry)
				return err
			}
		}
		if err := appendHistory(cfg, entry); err != nil {
			return err
		}
		if opts.jsonOutput {
			return writeJSON(result)
		}
		printRollback(console, result, opts.setup)
		return nil
	}
	if opts.restore != "" {
		lock, err := tools.AcquireLock(filepath.Join(cfg.RootDir, ".release-update.lock"))
		if err != nil {
			return err
		}
		defer lock.Release()
		item, err := backup.Resolve(cfg, opts.restore)
		if err != nil {
			return err
		}
		from := installedVersion(cfg.CurrentDir)
		syncResult, err := backup.Restore(ctx, cfg, item, false)
		if err != nil {
			return err
		}
		to := installedVersion(cfg.CurrentDir)
		if err := appendHistory(cfg, history.Entry{Action: "restore", ProjectName: cfg.ProjectName, FromVersion: from, ToVersion: to, Backup: item.Path, Status: "success"}); err != nil {
			return err
		}
		result := map[string]any{"backup": item, "fromVersion": from, "toVersion": to, "currentDir": cfg.CurrentDir, "sync": syncResult}
		if opts.jsonOutput {
			return writeJSON(result)
		}
		printRestore(console, item, from, to, cfg.CurrentDir, syncResult)
		return nil
	}
	if opts.status {
		result, err := projectstatus.Run(cfg)
		if err != nil {
			return err
		}
		if opts.jsonOutput {
			return writeJSON(result)
		}
		printStatus(console, result)
		return nil
	}
	if opts.list {
		result, err := inventory.List(cfg)
		if err != nil {
			return err
		}
		if opts.jsonOutput {
			return writeJSON(result)
		}
		printInventory(console, result)
		return nil
	}
	if opts.check {
		result, err := updatecheck.Run(cfg)
		if err != nil {
			return err
		}
		if opts.jsonOutput {
			return writeJSON(updateCheckJSON(result))
		}
		printUpdateCheck(console, cfg, result)
		return nil
	}
	if opts.verify {
		result, err := verifyArchive(cfg, opts.archive)
		if err != nil {
			return err
		}
		if opts.jsonOutput {
			return writeJSON(result)
		}
		printVerification(console, result)
		return nil
	}
	if opts.setup && !opts.update {
		lock, err := tools.AcquireLock(filepath.Join(cfg.RootDir, ".release-update.lock"))
		if err != nil {
			return err
		}
		defer lock.Release()
		_, err = projectsetup.Run(ctx, cfg, console)
		return err
	}

	currentState := &state{config: cfg, fromVersion: installedVersion(cfg.CurrentDir)}
	simulation := opts.dryRun || opts.plan

	lock, err := tools.AcquireLock(filepath.Join(cfg.RootDir, ".release-update.lock"))
	if err != nil {
		return err
	}
	defer lock.Release()

	currentState.workDir, err = os.MkdirTemp("", "release-update-*")
	if err != nil {
		return fmt.Errorf("temporärer Arbeitsordner kann nicht erstellt werden: %w", err)
	}
	defer tools.RemoveTree(currentState.workDir)
	currentState.extractDir = filepath.Join(currentState.workDir, "extract")

	if err := resolveSource(ctx, currentState, opts.archive, simulation); err != nil {
		return err
	}
	if currentState.repositoryStage != "" {
		defer tools.RemoveTree(currentState.repositoryStage)
	}
	if err := enforceVersionPolicy(cfg, currentState.version, opts.allowDowngrade, opts.force, simulation); err != nil {
		var installedErr *VersionAlreadyInstalledError
		if errors.As(err, &installedErr) && !opts.jsonOutput {
			console.ErrorNotice(
				installedErr.Error(),
				"Zur erneuten Installation --update --force verwenden",
			)
			return &ExitError{Code: 1}
		}
		return err
	}
	currentState.releaseDir = filepath.Join(cfg.ReleaseRoot, currentState.version.String())
	dockerDetection, err := projectdocker.Detect(cfg.CurrentDir)
	if err != nil {
		return err
	}
	currentState.dockerComposeFile = dockerDetection.ComposeFile

	if !opts.jsonOutput {
		printPlan(console, currentState, simulation, opts.plan, opts.setup, opts.allowDowngrade)
	}

	// Stop an existing Docker Compose stack before backup, release activation,
	// or current synchronization. A detected Compose project must be stopped
	// successfully; otherwise the update is aborted without changing the project.
	if !simulation && dockerDetection.Detected {
		if !opts.jsonOutput {
			console.Info(fmt.Sprintf("Docker-Compose-Projekt erkannt: %s", displayPath(cfg.RootDir, dockerDetection.ComposeFile)))
		}
		dockerResult, stopErr := projectdocker.Stop(ctx, cfg.CurrentDir)
		if stopErr != nil {
			return stopErr
		}
		currentState.dockerStopped = dockerResult.Stopped
		if !opts.jsonOutput {
			console.Success("Docker-Container vor dem Update gestoppt")
		}
	}

	if opts.backup {
		hasContent, err := currentHasContent(cfg.CurrentDir)
		if err != nil {
			return err
		}
		if !hasContent {
			if !opts.jsonOutput {
				console.Diagnostic("warning", "Backup", "current fehlt oder ist leer; Backup wird bei der Erstinstallation übersprungen")
			}
		} else {
			backupResult, err := backup.Create(ctx, cfg, false)
			if err != nil {
				return fmt.Errorf("Backup vor dem Update fehlgeschlagen: %w", err)
			}
			currentState.backupPath = backupResult.Backup.Path
			if err := appendHistory(cfg, history.Entry{Action: "backup", ProjectName: cfg.ProjectName, FromVersion: backupResult.Backup.Version, Backup: backupResult.Backup.Path, Status: "success", Message: "automatisch vor Update"}); err != nil {
				return err
			}
			if !opts.jsonOutput {
				printBackup(console, backupResult)
			}
		}
	}

	currentState.workDir, err = os.MkdirTemp("", "release-update-*")
	if err != nil {
		return fmt.Errorf("temporärer Arbeitsordner kann nicht erstellt werden: %w", err)
	}
	defer tools.RemoveTree(currentState.workDir)
	currentState.extractDir = filepath.Join(currentState.workDir, "extract")

	steps := []struct {
		label  string
		action func() error
	}{
		{"Voraussetzungen prüfen", func() error { return prerequisites(currentState) }},
	}
	if currentState.sourceType != source.Repository {
		steps = append(steps,
			struct {
				label  string
				action func() error
			}{"ZIP-Archiv validieren", func() error { return archive.Validate(currentState.archivePath) }},
			struct {
				label  string
				action func() error
			}{"Archiv sicher entpacken", func() error { return extract(currentState) }},
		)
	} else {
		steps = append(steps, struct {
			label  string
			action func() error
		}{"Repository-Inhalt validieren", func() error {
			return archive.ValidateVersionFile(currentState.contentDir, currentState.version.String())
		}})
	}
	steps = append(steps,
		struct {
			label  string
			action func() error
		}{"Release-Verzeichnis erstellen", func() error { return prepareRelease(ctx, currentState, simulation) }},
		struct {
			label  string
			action func() error
		}{"Current per rsync synchronisieren", func() error { return syncCurrent(ctx, currentState, simulation) }},
		struct {
			label  string
			action func() error
		}{"Installation verifizieren", func() error { return verify(currentState, simulation) }},
	)

	for index, step := range steps {
		if opts.jsonOutput {
			if err := step.action(); err != nil {
				return fmt.Errorf("%s: %w", step.label, err)
			}
			continue
		}
		if err := console.Step(ctx, index, len(steps), step.label, step.action); err != nil {
			return fmt.Errorf("%s: %w", step.label, err)
		}
	}

	if opts.plan {
		if opts.jsonOutput {
			return writeJSON(updatePlanJSON(currentState))
		}
		printDetailedPlan(console, currentState)
		return nil
	}

	printResult(console, currentState, opts.dryRun)
	if opts.dryRun {
		return nil
	}
	entry := history.Entry{Action: "update", ProjectName: cfg.ProjectName, FromVersion: currentState.fromVersion, ToVersion: currentState.version.String(), Archive: currentState.sourceReference, Backup: currentState.backupPath, Setup: opts.setup, Status: "success"}
	if opts.setup {
		if _, err := projectsetup.Run(ctx, cfg, console); err != nil {
			entry.Status = "setup-failed"
			entry.Message = err.Error()
			_ = appendHistory(cfg, entry)
			return err
		}
	}
	return appendHistory(cfg, entry)
}

func runNoParameterAction(ctx context.Context, buildVersion string) error {
	root, err := config.ResolveRoot("")
	if err != nil {
		printHelp(buildVersion)
		return nil
	}

	configFile := filepath.Join(root, config.ConfigDirName, config.ConfigFileName)
	info, statErr := os.Stat(configFile)
	if errors.Is(statErr, os.ErrNotExist) {
		printHelp(buildVersion)
		return nil
	}
	if statErr != nil {
		return fmt.Errorf("Updater-Konfiguration kann nicht geprüft werden: %w", statErr)
	}
	if info.IsDir() {
		return fmt.Errorf("Updater-Konfiguration ist kein reguläres File: %s", configFile)
	}

	cfg, err := config.Load(root, "")
	if err != nil {
		return err
	}
	if len(cfg.NoParameterActions) == 0 || (len(cfg.NoParameterActions) == 1 && cfg.NoParameterActions[0] == "help") {
		printHelp(buildVersion)
		return nil
	}

	args := make([]string, 0, len(cfg.NoParameterActions)+2)
	for _, action := range cfg.NoParameterActions {
		args = append(args, "--"+action)
	}
	args = append(args, "--root", root)
	return Run(ctx, buildVersion, args)
}

func parseOptions(args []string) (options, error) {
	if len(args) == 0 {
		return options{showHelp: true, keep: -1, limit: 20}, nil
	}

	opts := options{keep: -1, limit: 20}
	set := flag.NewFlagSet("update-cli", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	set.StringVar(&opts.archive, "archive", "", "bestimmtes ZIP-Archiv")
	set.StringVar(&opts.archive, "a", "", "bestimmtes ZIP-Archiv")
	set.StringVar(&opts.downloadDir, "downloads", "", "Download-Ordner")
	set.StringVar(&opts.downloadDir, "d", "", "Download-Ordner (Kompatibilitätsalias für --folder)")
	set.StringVar(&opts.sourceType, "from", "", "Release-Quelle: download, url oder repository")
	set.StringVar(&opts.sourceFolder, "folder", "", "Ordner mit lokalen Release-ZIPs")
	set.StringVar(&opts.sourceURL, "url", "", "direkte HTTP(S)-URL eines Release-ZIPs")
	set.StringVar(&opts.repository, "repository", "", "Git-Repository-URL")
	set.StringVar(&opts.rootDir, "root", "", "Projektordner")
	set.StringVar(&opts.rootDir, "r", "", "Projektordner")
	set.BoolVar(&opts.dryRun, "dry-run", false, "Änderungen simulieren")
	set.BoolVar(&opts.dryRun, "n", false, "Änderungen simulieren")
	set.BoolVar(&opts.plan, "plan", false, "detaillierten Plan anzeigen")
	set.BoolVar(&opts.allowDowngrade, "allow-downgrade", false, "Installation einer älteren Version erlauben")
	set.BoolVar(&opts.jsonOutput, "json", false, "maschinenlesbare JSON-Ausgabe")
	set.BoolVar(&opts.update, "update", false, "Release nach current installieren")
	set.BoolVar(&opts.backup, "backup", false, "current sichern; mit --update vor der Installation")
	set.BoolVar(&opts.rollback, "rollback", false, "vorheriges oder angegebenes Release aktivieren")
	set.StringVar(&opts.restore, "restore", "", "Backup anhand Name oder Pfad wiederherstellen")
	set.BoolVar(&opts.history, "history", false, "Update-Historie anzeigen")
	set.BoolVar(&opts.cleanup, "cleanup", false, "alte Releases und Backups entfernen")
	set.IntVar(&opts.keep, "keep", -1, "bei --cleanup zu behaltende Anzahl")
	set.IntVar(&opts.limit, "limit", 20, "maximale Einträge bei --history")
	set.BoolVar(&opts.init, "init", false, "Projektkonfiguration initialisieren")
	set.BoolVar(&opts.upgrade, "upgrade", false, "config.json auf das aktuelle Schema aktualisieren")
	set.BoolVar(&opts.check, "check", false, "auf verfügbare neue Version prüfen")
	set.BoolVar(&opts.doctor, "doctor", false, "Umgebung und Projekt prüfen")
	set.BoolVar(&opts.status, "status", false, "Projekt- und Versionsstatus anzeigen")
	set.BoolVar(&opts.list, "list", false, "Downloads und Releases auflisten")
	set.BoolVar(&opts.verify, "verify", false, "Release-Archiv vollständig prüfen")
	set.BoolVar(&opts.setup, "setup", false, "Projekt im current-Ordner einrichten")
	set.BoolVar(&opts.config, "config", false, "Konfiguration anzeigen")
	set.BoolVar(&opts.templatesMode, "templates", false, "Projekt-Templates verwalten")
	set.BoolVar(&opts.details, "details", false, "zusätzliche Template-Details anzeigen")
	set.StringVar(&opts.templateUse, "use", "", "Template aus templates.json anwenden")
	set.BoolVar(&opts.edit, "edit", false, "Konfiguration im Editor öffnen")
	set.StringVar(&opts.useTemplate, "use-template", "", "Setup-Template in config.json übernehmen")
	set.BoolVar(&opts.force, "force", false, "bestehende Konfiguration ersetzen oder Release erneut installieren")
	set.BoolVar(&opts.force, "f", false, "bestehende Konfiguration ersetzen oder Release erneut installieren")
	set.BoolVar(&opts.noColor, "no-color", false, "Farben deaktivieren")
	set.BoolVar(&opts.showHelp, "help", false, "verfügbare Befehle anzeigen")
	set.BoolVar(&opts.showHelp, "h", false, "verfügbare Befehle anzeigen")
	set.BoolVar(&opts.showHowTo, "howto", false, "ausführliche Anleitung anzeigen")
	set.BoolVar(&opts.showVersion, "version", false, "Programmversion anzeigen")
	set.BoolVar(&opts.showVersion, "V", false, "Programmversion anzeigen")

	if err := set.Parse(normalizeFlagArguments(args)); err != nil {
		return options{}, err
	}
	if opts.config && opts.list {
		opts.configList = true
		opts.list = false
	}
	if opts.templatesMode && opts.list {
		opts.templatesList = true
		opts.list = false
	}
	remaining := set.Args()
	if len(remaining) > 1 {
		return options{}, errors.New("es darf nur ein positionsabhängiges Argument angegeben werden")
	}
	if len(remaining) == 1 {
		switch {
		case opts.update || opts.verify:
			if opts.archive != "" {
				return options{}, errors.New("Archiv wurde mehrfach angegeben")
			}
			opts.archive = remaining[0]
		case opts.rollback:
			opts.rollbackVersion = remaining[0]
		case opts.init:
			opts.projectName = remaining[0]
		case opts.templatesMode && opts.edit:
			opts.templateName = remaining[0]
		case opts.upgrade || opts.check || opts.doctor || opts.status || opts.list || opts.setup || opts.config || opts.templatesMode || opts.history || opts.cleanup || opts.backup || opts.restore != "":
			return options{}, errors.New("Archivargument ist mit dieser Betriebsart nicht zulässig")
		default:
			return options{}, errors.New("ein Archiv ist nur zusammen mit --update oder --verify zulässig")
		}
	}

	standaloneBackup := opts.backup && !opts.update
	primaryModes := 0
	for _, enabled := range []bool{
		opts.update, standaloneBackup, opts.rollback, opts.restore != "", opts.history, opts.cleanup,
		opts.init, opts.upgrade, opts.check, opts.doctor, opts.status, opts.list, opts.verify, opts.config, opts.templatesMode, opts.showHelp, opts.showHowTo, opts.showVersion,
	} {
		if enabled {
			primaryModes++
		}
	}
	if primaryModes > 1 {
		return options{}, errors.New("Betriebsarten wie --update, --backup, --rollback, --restore, --history, --cleanup, --status, --list, --verify, --check, --doctor, --config, --templates, --init, --upgrade, --help, --howto und --version schließen sich gegenseitig aus")
	}
	if opts.setup && !(opts.update || opts.rollback || primaryModes == 0) {
		return options{}, errors.New("--setup kann nur allein oder zusammen mit --update beziehungsweise --rollback verwendet werden")
	}
	if opts.edit && !(opts.config || opts.templatesMode) {
		return options{}, errors.New("--edit ist nur zusammen mit --config oder --templates zulässig")
	}
	if strings.TrimSpace(opts.useTemplate) != "" && !(opts.config || opts.init) {
		return options{}, errors.New("--use-template ist nur zusammen mit --config oder --init zulässig")
	}
	if strings.TrimSpace(opts.templateUse) != "" && !opts.templatesMode {
		return options{}, errors.New("--use ist nur zusammen mit --templates zulässig")
	}
	if opts.templatesMode {
		actions := 0
		if opts.templatesList {
			actions++
		}
		if strings.TrimSpace(opts.templateUse) != "" {
			actions++
		}
		if opts.edit {
			actions++
		}
		if actions > 1 {
			return options{}, errors.New("--templates unterstützt jeweils nur eine Aktion: --list, --use NAME oder --edit NAME")
		}
		if opts.edit && strings.TrimSpace(opts.templateName) == "" {
			return options{}, errors.New("--templates --edit benötigt einen Template-Namen")
		}
	}
	if opts.details && !(opts.templatesMode && opts.templatesList) {
		return options{}, errors.New("--details ist nur zusammen mit --templates --list zulässig")
	}
	if opts.config && opts.configList && (opts.edit || strings.TrimSpace(opts.useTemplate) != "") {
		return options{}, errors.New("--config --list kann nicht mit --edit oder --use-template kombiniert werden")
	}
	if opts.plan && !(opts.update || opts.cleanup) {
		return options{}, errors.New("--plan ist nur zusammen mit --update oder --cleanup zulässig")
	}
	if opts.allowDowngrade && !opts.update {
		return options{}, errors.New("--allow-downgrade ist nur zusammen mit --update zulässig")
	}
	if opts.keep < -1 {
		return options{}, errors.New("--keep darf nicht negativ sein")
	}
	if opts.keep != -1 && !opts.cleanup {
		return options{}, errors.New("--keep ist nur zusammen mit --cleanup zulässig")
	}
	if opts.limit < 1 {
		return options{}, errors.New("--limit muss mindestens 1 sein")
	}
	if opts.limit != 20 && !opts.history {
		return options{}, errors.New("--limit ist nur zusammen mit --history zulässig")
	}
	if opts.dryRun && !opts.update {
		return options{}, errors.New("--dry-run ist nur zusammen mit --update zulässig")
	}
	if opts.backup && opts.update && (opts.dryRun || opts.plan) {
		return options{}, errors.New("--update --backup kann nicht mit --dry-run oder --plan kombiniert werden")
	}

	if opts.init {
		if strings.TrimSpace(opts.projectName) == "" {
			return options{}, errors.New("--init benötigt den Projektnamen: update-cli --init release-updater-go")
		}
		if opts.archive != "" {
			return options{}, errors.New("--init kann nicht mit einem Archiv kombiniert werden")
		}
		if opts.dryRun || opts.plan || opts.jsonOutput {
			return options{}, errors.New("--init kann nicht mit --dry-run, --plan oder --json kombiniert werden")
		}
	} else if opts.force && !opts.update {
		return options{}, errors.New("--force ist nur zusammen mit --init oder --update zulässig")
	}
	if opts.upgrade {
		if opts.downloadDir != "" || opts.archive != "" || opts.dryRun || opts.plan || opts.allowDowngrade || opts.backup || opts.setup {
			return options{}, errors.New("--upgrade kann nicht mit Update-, Backup- oder Setup-Optionen kombiniert werden")
		}
	}

	if opts.verify && strings.TrimSpace(opts.archive) == "" {
		return options{}, errors.New("--verify benötigt ein Archiv: update-cli --verify ARCHIV.zip")
	}
	if opts.verify && (opts.dryRun || opts.plan || opts.allowDowngrade || opts.backup) {
		return options{}, errors.New("--verify kann nicht mit Update- oder Backup-Optionen kombiniert werden")
	}
	if opts.update && opts.setup && (opts.dryRun || opts.plan) {
		return options{}, errors.New("--update --setup kann nicht mit --dry-run oder --plan kombiniert werden")
	}
	if opts.jsonOutput && opts.update && !opts.plan {
		return options{}, errors.New("--json wird bei --update nur zusammen mit --plan unterstützt")
	}
	if opts.jsonOutput && opts.rollback && opts.setup {
		return options{}, errors.New("--json kann nicht mit --rollback --setup kombiniert werden")
	}
	if opts.jsonOutput && (opts.init || opts.config || opts.templatesMode || (opts.setup && primaryModes == 0) || opts.showVersion) {
		return options{}, errors.New("--json wird für diese Betriebsart nicht unterstützt")
	}
	if primaryModes == 0 && !opts.setup {
		if opts.archive != "" {
			return options{}, errors.New("ein Archiv ist nur zusammen mit --update oder --verify zulässig")
		}
		return options{}, errors.New("keine Betriebsart angegeben; verwende --update, --backup, --rollback, --restore, --history, --cleanup, --status, --list, --verify, --check, --doctor, --setup, --config, --templates, --init, --upgrade, --help oder --howto")
	}
	if opts.downloadDir != "" && opts.sourceFolder != "" {
		return options{}, errors.New("--downloads und --folder sind Aliase und dürfen nicht gemeinsam verwendet werden")
	}
	hasSourceOverride := strings.TrimSpace(opts.sourceType) != "" || strings.TrimSpace(opts.sourceFolder) != "" || strings.TrimSpace(opts.sourceURL) != "" || strings.TrimSpace(opts.repository) != "" || strings.TrimSpace(opts.downloadDir) != ""
	if hasSourceOverride && !(opts.update || opts.check || opts.doctor || opts.status || opts.list || opts.init) {
		return options{}, errors.New("--from, --folder, --url und --repository sind nur mit --update, --check, --doctor, --status, --list oder --init zulässig")
	}
	if strings.TrimSpace(opts.archive) != "" && hasSourceOverride {
		return options{}, errors.New("ein explizites ARCHIV.zip kann nicht mit --from, --folder, --url oder --repository kombiniert werden")
	}
	if _, err := source.NormalizeKind(firstNonEmpty(opts.sourceType, func() string {
		if strings.TrimSpace(opts.sourceURL) != "" {
			return source.URL
		}
		if strings.TrimSpace(opts.repository) != "" {
			return source.Repository
		}
		if strings.TrimSpace(opts.sourceFolder) != "" || strings.TrimSpace(opts.downloadDir) != "" {
			return source.Download
		}
		return ""
	}())); err != nil {
		return options{}, err
	}
	if opts.showVersion && (opts.downloadDir != "" || opts.sourceType != "" || opts.sourceFolder != "" || opts.sourceURL != "" || opts.repository != "" || opts.rootDir != "" || opts.noColor || opts.edit || opts.useTemplate != "" || opts.templateUse != "" || opts.details || opts.jsonOutput) {
		return options{}, errors.New("--version kann nicht mit weiteren Optionen kombiniert werden")
	}
	return opts, nil
}

func normalizeFlagArguments(args []string) []string {
	valueFlags := map[string]bool{
		"--archive": true, "-a": true,
		"--downloads": true, "-d": true,
		"--from": true, "--folder": true, "--url": true, "--repository": true,
		"--root": true, "-r": true,
		"--restore":      true,
		"--keep":         true,
		"--limit":        true,
		"--use-template": true,
		"--use":          true,
	}
	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, 1)
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if valueFlags[argument] {
			flags = append(flags, argument)
			if index+1 < len(args) {
				index++
				flags = append(flags, args[index])
			}
			continue
		}
		if strings.HasPrefix(argument, "-") {
			flags = append(flags, argument)
			continue
		}
		positionals = append(positionals, argument)
	}
	return append(flags, positionals...)
}

func scopedHelpTopic(args []string) (string, bool) {
	hasHelp := false
	for _, argument := range args {
		if argument == "--help" || argument == "-h" {
			hasHelp = true
			break
		}
	}
	if !hasHelp {
		return "", false
	}
	topics := map[string]string{
		"--update": "update", "--backup": "backup", "--rollback": "rollback",
		"--restore": "restore", "--history": "history", "--cleanup": "cleanup",
		"--status": "status", "--list": "list", "--verify": "verify",
		"--check": "check", "--doctor": "doctor", "--init": "init",
		"--upgrade": "upgrade", "--setup": "setup", "--config": "config",
		"--templates": "templates",
	}
	for _, argument := range args {
		if topic, ok := topics[argument]; ok {
			return topic, true
		}
	}
	return "", false
}

func mustGlobalTemplatesDisplay() string {
	path, err := buildconfig.GlobalTemplatesFile()
	if err != nil {
		return filepath.Join(buildconfig.Current().DefaultConfigPath, templates.FileName)
	}
	return path
}

func printHelp(buildVersion string) {
	fmt.Printf(`Release Updater %s

Verfügbare Befehle:
  --update [--from TYPE] [--folder DIR|--url URL|--repository REPO] [Optionen]
  --backup [--json]
  --rollback [VERSION] [--setup] [--json]
  --restore BACKUP [--json]
  --history [--limit N] [--json]
  --cleanup [--keep N] [--plan] [--json]
  --status [--from TYPE] [--json]
  --list [--from TYPE] [--json]
  --verify ARCHIV.zip [--json]
  --check [--from TYPE] [--json]
  --doctor [--from TYPE] [--json]
  --init PROJECTNAME [--from TYPE] [--use-template NAME] [--force]
  --upgrade [--json]
  --setup
  --config [--list|--edit|--use-template NAME|--help]
  --templates [--list [--details]|--use NAME|--edit NAME|--help]
  --version
  --help
  --howto

Quellenparameter:
  --from TYPE           download, url oder repository; Standard: config.json/download
  --folder ORDNER       lokaler Release-Ordner; Standard: %s
  --url URL             direkte HTTP(S)-URL eines Release-ZIPs
  --repository REPO     Git-Repository-URL oder lokaler Repository-Pfad

Globale Parameter:
  --root ORDNER         Projektordner; Standard: aktueller Ordner
  --downloads ORDNER    Kompatibilitätsalias für --folder
  --no-color            Farben deaktivieren
  --json                maschinenlesbare Ausgabe, soweit unterstützt

Detaillierte Hilfe zu einem Befehl:
  update-cli --config --help
  update-cli --templates --help
  update-cli --update --help

Ausführliche Gesamtanleitung:
  update-cli --howto
`, buildVersion, buildconfig.Current().DefaultDownloadFolder)
}

func printCommandHelp(buildVersion, topic string) {
	header := fmt.Sprintf("Release Updater %s — %s", buildVersion, topic)
	var body string
	switch topic {
	case "update":
		body = fmt.Sprintf(`Verwendung:
  update-cli --update
  update-cli --update --from download --folder %s
  update-cli --update --from url --url https://example.test/project-v1.2.3.zip
  update-cli --update --from repository --repository https://github.com/org/repo.git
  update-cli --update --backup --setup
  update-cli --update --force
  update-cli --update --plan [--json]

Quellenparameter:
  --from TYPE           download, url oder repository
  --folder DIR          Ordner mit <project>-vX.Y.Z.zip; Standard: %s
  --url URL             direkte HTTP(S)-URL; Dateiname oder Content-Disposition muss passen
  --repository REPO     Git-URL oder lokaler Pfad; VERSION im Repository-Stamm ist Pflicht
  --downloads, -d DIR   Kompatibilitätsalias für --folder

Update-Parameter:
  --archive, -a DATEI   bestimmtes lokales Archiv verwenden
  --backup              current vor dem Update sichern
  --setup               nach erfolgreichem Update setup ausführen
  --plan                exakte rsync-Änderungen nur anzeigen
  --dry-run, -n         Update simulieren
  --allow-downgrade     ältere Version ausdrücklich zulassen
  --force, -f           bereits installierte Version erneut installieren
  --root, -r DIR        Projektordner
  --no-color            Farben deaktivieren

Docker-Sicherheit:
  Enthält current eine compose.yml, compose.yaml, docker-compose.yml oder
  docker-compose.yaml, führt update-cli vor Backup und Dateisynchronisation
  docker compose down --remove-orphans aus. Schlägt dies fehl, wird das Update
  ohne Änderungen abgebrochen. --plan und --dry-run stoppen keine Container.`, buildconfig.Current().DefaultDownloadFolder, buildconfig.Current().DefaultDownloadFolder)
	case "backup":
		body = `Verwendung:
  update-cli --backup [--json]
  update-cli --update --backup

Sichert current nach backup/<Zeitstempel>-v<VERSION>. Regenerierbare
Abhängigkeiten wie .venv, node_modules, vendor, dist und build werden nicht gesichert.`
	case "rollback":
		body = `Verwendung:
  update-cli --rollback
  update-cli --rollback VERSION [--setup] [--json]

Ohne VERSION wird das vorherige validierte Release aktiviert.`
	case "restore":
		body = `Verwendung:
  update-cli --restore latest [--json]
  update-cli --restore BACKUP-NAME [--json]

Stellt einen Snapshot aus dem konfigurierten Backup-Ordner wieder her.`
	case "history":
		body = `Verwendung:
  update-cli --history [--limit N] [--json]

Liest .updater-cli/history.jsonl. Standardmäßig werden 20 Einträge angezeigt.`
	case "cleanup":
		body = `Verwendung:
  update-cli --cleanup [--keep N] [--plan] [--json]

Das aktive und das vorherige Release werden immer geschützt.`
	case "status":
		body = `Verwendung:
  update-cli --status [--from TYPE] [--json]

Zeigt Projekt, konfigurierte Quelle, installierte und verfügbare Version, Setup und Backup-Status.`
	case "list":
		body = `Verwendung:
  update-cli --list [--from TYPE] [--json]

Listet die konfigurierte Release-Quelle, installierte Releases und Backups.`
	case "verify":
		body = `Verwendung:
  update-cli --verify ARCHIV.zip [--json]

Prüft Dateiname, SemVer, ZIP-CRC, sichere Pfade, Symlinks und interne VERSION.`
	case "check":
		body = `Verwendung:
  update-cli --check [--from TYPE] [--folder DIR|--url URL|--repository REPO] [--json]

Vergleicht die installierte Version mit der konfigurierten oder temporär überschriebenen Release-Quelle.`
	case "doctor":
		body = `Verwendung:
  update-cli --doctor [--from TYPE] [--folder DIR|--url URL|--repository REPO] [--json]

Prüft Konfiguration, templates.json, rsync/git, Release-Quelle, Pfade,
Schreibrechte, Locks, Docker Compose, Releases, Backups und Setup.`
	case "init":
		body = fmt.Sprintf(`Verwendung:
  update-cli --init PROJECTNAME
  update-cli --init PROJECTNAME --from download --folder %s
  update-cli --init PROJECTNAME --from url --url https://example.test/project-v1.2.3.zip
  update-cli --init PROJECTNAME --from repository --repository https://github.com/org/repo.git
  update-cli --init PROJECTNAME --use-template NAME
  update-cli --init PROJECTNAME --force

Erstellt:
  .updater-cli/config.json
  .updater-cli/templates.json

Ohne Quellenparameter wird source.type=download und source.folder=%s verwendet.
Globale Zusatztemplates werden aus %s geladen, sofern die Datei existiert.`, buildconfig.Current().DefaultDownloadFolder, buildconfig.Current().DefaultDownloadFolder, mustGlobalTemplatesDisplay())
	case "upgrade":
		body = `Verwendung:
  update-cli --upgrade [--json]

Migriert config.json auf das aktuelle Schema, legt vorher ein Backup an und
erstellt templates.json aus den im Binary eingebetteten Basistemplates, falls sie fehlt.`
	case "setup":
		body = `Verwendung:
  update-cli --setup
  update-cli --update --setup
  update-cli --rollback VERSION --setup

Führt zuerst current/setup.sh und danach setup.commands aus config.json aus.`
	case "config":
		body = `Verwendung:
  update-cli --config
  update-cli --config --list
  update-cli --config --edit
  update-cli --config --use-template NAME
  update-cli --config --help

Unterparameter:
  --list                 config.json, templates.json und history.jsonl auflisten
  --edit                 config.json im Editor öffnen und anschließend validieren
  --use-template NAME    Template aus templates.json auf config.json anwenden
  --help                 diese Detailhilfe anzeigen`
	case "templates":
		body = `Verwendung:
  update-cli --templates --list
  update-cli --templates --list --details
  update-cli --templates --use NAME
  update-cli --templates --edit NAME
  update-cli --templates --help

Unterparameter:
  --list          Template-Name und Beschreibung zweispaltig anzeigen
  --details       mit --list zusätzlich Aktionen und Setup-Kommandos anzeigen
  --use NAME      Template auf config.json anwenden
  --edit NAME     templates.json im Editor öffnen und das benannte Template validieren
  --help          diese Detailhilfe anzeigen

Basistemplates werden beim Build in update-cli eingebettet. Zusätzliche globale
Templates werden aus dem konfigurierten Build-Pfad geladen. --init und --upgrade
erzeugen daraus eine lokale, frei bearbeitbare templates.json.`
	default:
		printHelp(buildVersion)
		return
	}
	fmt.Printf("%s\n\n%s\n", header, body)
}

func printHowTo(buildVersion string) {
	fmt.Printf(`Release Updater %s

Verwendung:
  update-cli --init PROJECTNAME [--from TYPE] [--use-template NAME]
  update-cli --check [--from TYPE]
  update-cli --update [--from TYPE] [--backup] [--setup] [--force]
  update-cli --config [--list|--edit|--use-template NAME]
  update-cli --templates [--list [--details]|--use NAME|--edit NAME]

Beispiele:
  update-cli --init mediastudio
  update-cli --init mediastudio --from url --url https://example.test/mediastudio-v3.0.0.zip
  update-cli --init mediastudio --from repository --repository https://github.com/org/mediastudio.git
  update-cli --init mediastudio --use-template update-and-setup
  update-cli --templates --list
  update-cli --templates --use Laravel
  update-cli --templates --edit Laravel
  update-cli --config --list
  update-cli --config --edit
  update-cli --update --backup --setup

Konfiguration:
  .updater-cli/config.json

  "source": {"type":"download","folder":"%s"}
  "source": {"type":"url","url":"https://example.test/project-v1.2.3.zip"}
  "source": {"type":"repository","repository":"https://github.com/org/repo.git"}
  .updater-cli/templates.json
  .updater-cli/history.jsonl

Build-Standards:
  Download-Ordner: %s
  Deployment:      %s
  Globale Config:  %s
  Zusatztemplates: %s

  "no parameter": ["help"]
  "no parameter": ["setup"]
  "no parameter": ["update", "setup"]

Setup-Templates:
  Die Basistemplates sind im Binary eingebettet. --init und --upgrade erzeugen
  daraus .updater-cli/templates.json. Die lokale Datei kann erweitert und
  mit update-cli --templates --edit NAME bearbeitet werden.

Docker-Sicherheit:
  Vor einem echten Update erkennt update-cli Compose-Dateien in current und
  stoppt den Stack mit docker compose down --remove-orphans. Erst danach werden
  Backup, Release-Aktivierung und rsync ausgeführt. Bei einem Fehler bleibt das
  Projekt unverändert. Plan und Dry-Run stoppen keine Container.

Akzeptiertes Archivformat:
  <PROJECTNAME>-v<MAJOR>.<MINOR>.<PATCH>.zip

Geschützt in current:
  .git/
  .venv/
  .env

Alle Befehle und Unterparameter:
  update-cli --help

Detailhilfe:
  update-cli --update --help
  update-cli --config --help
  update-cli --templates --help
`, buildVersion,
		buildconfig.Current().DefaultDownloadFolder,
		buildconfig.Current().DefaultDownloadFolder,
		buildconfig.Current().DefaultDeploymentPath,
		buildconfig.Current().DefaultConfigPath,
		mustGlobalTemplatesDisplay(),
	)
}

func showConfiguration(console *ui.Console, root string) error {
	path, formatted, err := config.Format(root)
	if err != nil {
		return err
	}
	console.Header("Updater-Konfiguration")
	console.Row("Datei", path)
	fmt.Printf("\n%s", formatted)
	return nil
}

func listTemplates(console *ui.Console, root string, details bool) error {
	path := filepath.Join(root, config.ConfigDirName, config.TemplatesFileName)
	file, err := templates.Load(path)
	if err != nil {
		return err
	}
	console.Header("Updater-Templates")
	console.Row("Template", "Beschreibung")
	for _, template := range templates.Sorted(file) {
		description := strings.TrimSpace(template.Description)
		if description == "" {
			description = "—"
		}
		console.Row(template.Name, description)
		if !details {
			continue
		}
		if len(template.NoParameter) > 0 {
			console.Row("  no parameter", strings.Join(template.NoParameter, " + "))
		}
		if template.Setup != nil {
			for index, command := range template.Setup.Commands {
				console.Row(fmt.Sprintf("  setup[%d]", index), command)
			}
		}
		fmt.Println()
	}
	return nil
}

func applyConfigurationTemplate(console *ui.Console, root, templateName string) error {
	path, selected, err := config.ApplyTemplate(root, templateName)
	if err != nil {
		return err
	}

	console.Header("Template übernommen")
	console.Row("Template", selected.Name)
	console.Row("Konfiguration", displayPath(root, path))
	if len(selected.NoParameter) > 0 {
		console.Row("Ohne Parameter", strings.Join(selected.NoParameter, " + "))
	}
	if selected.Setup != nil {
		console.Row("Setup-Kommandos", fmt.Sprintf("%d", len(selected.Setup.Commands)))
		for index, command := range selected.Setup.Commands {
			console.Row(fmt.Sprintf("setup.commands[%d]", index), command)
		}
	}
	return nil
}

func editTemplate(ctx context.Context, console *ui.Console, root, templateName string) error {
	path := filepath.Join(root, config.ConfigDirName, config.TemplatesFileName)
	if _, err := templates.LookupFile(path, templateName); err != nil {
		return err
	}
	used, err := editor.Open(ctx, path)
	if err != nil {
		return err
	}
	file, err := templates.Load(path)
	if err != nil {
		return fmt.Errorf("Editor wurde geschlossen, aber templates.json ist ungültig: %w", err)
	}
	selected, err := templates.LookupFile(path, templateName)
	if err != nil {
		return fmt.Errorf("Editor wurde geschlossen, aber das Template %q fehlt: %w", templateName, err)
	}
	console.Header("Template-Datei gespeichert")
	console.Row("Datei", displayPath(root, path))
	console.Row("Editor", used)
	console.Row("Template", selected.Name)
	console.Row("Templates", fmt.Sprintf("%d", len(file.Templates)))
	console.Success("templates.json ist gültig")
	return nil
}

func listConfigurationFiles(console *ui.Console, root string) error {
	cfg, err := config.Load(root, "")
	if err != nil {
		return err
	}
	console.Header("Updater-Konfigurationsdateien")
	for _, item := range []struct {
		name string
		path string
	}{
		{"config.json", cfg.ConfigFile},
		{"templates.json", cfg.TemplatesFile},
		{"globale templates.json", cfg.GlobalTemplatesFile},
		{"history.jsonl", cfg.HistoryFile},
	} {
		status := "vorhanden"
		if _, err := os.Stat(item.path); errors.Is(err, os.ErrNotExist) {
			status = "noch nicht vorhanden"
		} else if err != nil {
			status = "Fehler: " + err.Error()
		}
		console.Row(item.name, fmt.Sprintf("%s — %s", displayPath(root, item.path), status))
	}
	return nil
}

func editConfiguration(ctx context.Context, console *ui.Console, root string) error {
	path, _, err := config.Format(root)
	if err != nil {
		return err
	}
	used, err := editor.Open(ctx, path)
	if err != nil {
		return err
	}
	if _, _, err := config.Format(root); err != nil {
		return fmt.Errorf("Editor wurde geschlossen, aber config.json ist ungültig: %w", err)
	}
	console.Header("Konfiguration gespeichert")
	console.Row("Datei", path)
	console.Row("Editor", used)
	console.Success("config.json ist gültig")
	return nil
}

func printConfigurationUpgrade(console *ui.Console, result config.UpgradeResult) {
	console.Header("Updater-Konfiguration aktualisiert")
	console.Row("Projekt", result.ProjectName)
	console.Row("Konfiguration", result.ConfigFile)
	console.Row("Schema", fmt.Sprintf("%d → %d", result.PreviousSchema, result.CurrentSchema))
	console.Row("Templates", displayPath(filepath.Dir(filepath.Dir(result.ConfigFile)), result.TemplatesFile))
	if result.TemplatesCreated {
		console.Row("Template-Katalog", "aus eingebetteten Basistemplates erstellt")
	} else if result.TemplatesUpdated {
		console.Row("Template-Katalog", "fehlende Basistemplates ergänzt")
	}
	if result.Changed {
		if result.BackupFile != "" {
			console.Row("Sicherung", result.BackupFile)
		}
		console.Success("Updater-Konfiguration wurde aktualisiert")
		return
	}
	console.Success("Updater-Konfiguration ist bereits aktuell")
}

func initialize(console *ui.Console, root string, opts options) error {
	cfg, err := config.Init(root, config.InitOptions{
		ProjectName: opts.projectName,
		UseTemplate: opts.useTemplate,
		SourceType:  opts.sourceType,
		Folder:      firstNonEmpty(opts.sourceFolder, opts.downloadDir),
		URL:         opts.sourceURL,
		Repository:  opts.repository,
		Force:       opts.force,
	})
	if err != nil {
		return err
	}

	console.Header("Updater initialisiert")
	console.Row("Projekt", cfg.ProjectName)
	console.Row("Konfiguration", cfg.ConfigFile)
	console.Row("Templates", cfg.TemplatesFile)
	console.Row("Globale Templates", cfg.GlobalTemplatesFile)
	if strings.TrimSpace(opts.useTemplate) != "" {
		console.Row("Template", opts.useTemplate)
	}
	console.Row("Quelle", cfg.SourceType)
	switch cfg.SourceType {
	case source.Download:
		console.Row("Ordner", cfg.SourceFolder)
	case source.URL:
		console.Row("URL", cfg.SourceURL)
	case source.Repository:
		console.Row("Repository", cfg.SourceRepository)
	}
	console.Row("Release", cfg.ReleaseRoot)
	console.Row("Current", cfg.CurrentDir)
	console.Row("Ohne Parameter", strings.Join(cfg.NoParameterActions, " + "))
	console.Row("Backup", cfg.BackupRoot)
	console.Row("Retention", fmt.Sprintf("Releases=%d, Backups=%d", cfg.KeepReleases, cfg.KeepBackups))
	return nil
}

func resolveSource(ctx context.Context, state *state, explicit string, simulation bool) error {
	if strings.TrimSpace(explicit) != "" {
		if err := resolveArchive(state, explicit); err != nil {
			return err
		}
		state.sourceType = source.Download
		state.sourceReference = state.archivePath
		return nil
	}

	artifact, err := source.Resolve(ctx, source.Options{
		Type:        state.config.SourceType,
		ProjectName: state.config.ProjectName,
		Folder:      state.config.SourceFolder,
		URL:         state.config.SourceURL,
		Repository:  state.config.SourceRepository,
		WorkDir:     state.workDir,
		ReleaseRoot: state.config.ReleaseRoot,
		Simulation:  simulation,
	})
	if err != nil {
		return err
	}
	state.sourceType = artifact.Type
	state.sourceReference = artifact.Reference
	state.archivePath = artifact.ArchivePath
	state.contentDir = artifact.ContentDir
	state.repositoryStage = artifact.StagingDir
	state.version = artifact.Version
	return nil
}

func resolveArchive(state *state, explicit string) error {
	if strings.TrimSpace(explicit) == "" {
		path, selectedVersion, err := versionutil.SelectNewest(
			state.config.DownloadDir,
			state.config.ProjectName,
		)
		if err != nil {
			return err
		}
		state.archivePath = path
		state.version = selectedVersion
		return nil
	}

	path := explicit
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("Archivpfad ist ungültig: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return fmt.Errorf("Archiv wurde nicht gefunden: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("Archivpfad ist ein Ordner: %s", absolute)
	}

	selectedVersion, err := versionutil.ParseArchiveName(state.config.ProjectName, filepath.Base(absolute))
	if err != nil {
		return err
	}
	state.archivePath = absolute
	state.version = selectedVersion
	return nil
}

func prerequisites(state *state) error {
	if err := rsyncutil.Require(); err != nil {
		return err
	}
	if state.sourceType == source.Repository {
		if _, err := os.Stat(state.contentDir); err != nil {
			return fmt.Errorf("geklontes Repository ist nicht verfügbar: %w", err)
		}
		return nil
	}
	if _, err := os.Stat(state.archivePath); err != nil {
		return fmt.Errorf("Release-Archiv ist nicht verfügbar: %w", err)
	}
	return nil
}

func extract(state *state) error {
	if err := tools.RemoveTree(state.extractDir); err != nil {
		return err
	}
	if err := archive.Extract(state.archivePath, state.extractDir); err != nil {
		return err
	}
	root, err := archive.ResolveContentRoot(state.extractDir)
	if err != nil {
		return err
	}
	if err := archive.ValidateVersionFile(root, state.version.String()); err != nil {
		return err
	}
	state.contentDir = root
	return nil
}

func prepareRelease(ctx context.Context, state *state, dryRun bool) error {
	logFile := filepath.Join(state.workDir, "rsync-release.log")
	if state.sourceType == source.Repository && !dryRun {
		if state.repositoryStage == "" || state.contentDir == "" {
			return errors.New("Repository-Stagingordner fehlt")
		}
		if err := writeReleaseMarkers(state.contentDir, state); err != nil {
			return err
		}
		changes, err := countFiles(state.contentDir)
		if err != nil {
			return err
		}
		state.releaseChanges = changes
		if err := tools.ReplaceDirectory(state.repositoryStage, state.releaseDir); err != nil {
			return err
		}
		state.repositoryStage = ""
		state.contentDir = state.releaseDir
		return nil
	}
	if dryRun {
		state.dryReleaseDir = filepath.Join(state.workDir, "dry-release")
		result, err := rsyncutil.Release(ctx, state.contentDir, state.dryReleaseDir, logFile)
		if err != nil {
			return err
		}
		state.releaseChanges = result.Changes
		return writeReleaseMarkers(state.dryReleaseDir, state)
	}

	if err := tools.EnsureDir(state.config.ReleaseRoot); err != nil {
		return err
	}
	state.releaseStage = filepath.Join(
		state.config.ReleaseRoot,
		fmt.Sprintf(".%s.new-%d", state.version.String(), os.Getpid()),
	)
	if err := tools.RemoveTree(state.releaseStage); err != nil {
		return err
	}
	if err := tools.EnsureDir(state.releaseStage); err != nil {
		return err
	}

	result, err := rsyncutil.Release(ctx, state.contentDir, state.releaseStage, logFile)
	if err != nil {
		return err
	}
	state.releaseChanges = result.Changes
	if err := writeReleaseMarkers(state.releaseStage, state); err != nil {
		return err
	}
	if err := tools.ReplaceDirectory(state.releaseStage, state.releaseDir); err != nil {
		return err
	}
	state.releaseStage = ""
	return nil
}

func syncCurrent(ctx context.Context, state *state, dryRun bool) error {
	logFile := filepath.Join(state.workDir, "rsync-current.log")
	source := state.releaseDir
	destination := state.config.CurrentDir
	if dryRun {
		source = state.dryReleaseDir
		if _, err := os.Stat(destination); errors.Is(err, os.ErrNotExist) {
			state.dryCurrentDir = filepath.Join(state.workDir, "dry-current")
			destination = state.dryCurrentDir
		}
	}

	result, err := rsyncutil.Current(ctx, source, destination, logFile, dryRun)
	if err != nil {
		return err
	}
	state.currentChanges = result.Changes
	state.currentPlan = result.Items
	return nil
}

func verify(state *state, dryRun bool) error {
	if dryRun {
		return verifyMarker(state.dryReleaseDir, state.version.String())
	}
	if err := verifyMarker(state.releaseDir, state.version.String()); err != nil {
		return err
	}
	if err := verifyMarker(state.config.CurrentDir, state.version.String()); err != nil {
		return err
	}
	if err := tools.WriteMarker(state.config.ReleaseRoot, ".project-name", state.config.ProjectName); err != nil {
		return err
	}
	if err := tools.WriteMarker(state.config.ReleaseRoot, ".last-version", state.version.String()); err != nil {
		return err
	}
	if err := tools.WriteMarker(state.config.ReleaseRoot, ".last-source", state.sourceReference); err != nil {
		return err
	}
	return tools.WriteMarker(state.config.ReleaseRoot, ".last-archive", state.sourceReference)
}

func writeReleaseMarkers(directory string, state *state) error {
	markers := map[string]string{
		".release-project": state.config.ProjectName,
		".release-version": state.version.String(),
		".release-source":  state.sourceReference,
		".release-archive": state.sourceReference,
	}
	for name, value := range markers {
		if err := tools.WriteMarker(directory, name, value); err != nil {
			return err
		}
	}
	return nil
}

func verifyMarker(directory, expected string) error {
	data, err := os.ReadFile(filepath.Join(directory, ".release-version"))
	if err != nil {
		return fmt.Errorf("Release-Marker fehlt in %s: %w", directory, err)
	}
	actual := strings.TrimSpace(string(data))
	if actual != expected {
		return fmt.Errorf("installierte Version %s stimmt nicht mit %s überein", actual, expected)
	}
	return nil
}

type verificationResult struct {
	ProjectName string        `json:"projectName"`
	ArchivePath string        `json:"archivePath"`
	Version     string        `json:"version"`
	ContentRoot string        `json:"contentRoot"`
	Stats       archive.Stats `json:"stats"`
	Valid       bool          `json:"valid"`
}

type updatePlanResult struct {
	ProjectName       string             `json:"projectName"`
	SourceType        string             `json:"sourceType"`
	Source            string             `json:"source"`
	FromVersion       string             `json:"fromVersion,omitempty"`
	ToVersion         string             `json:"toVersion"`
	ArchivePath       string             `json:"archivePath"`
	ReleaseDir        string             `json:"releaseDir"`
	CurrentDir        string             `json:"currentDir"`
	Created           []rsyncutil.Change `json:"created"`
	Updated           []rsyncutil.Change `json:"updated"`
	Deleted           []rsyncutil.Change `json:"deleted"`
	Other             []rsyncutil.Change `json:"other,omitempty"`
	Protected         []string           `json:"protected"`
	DockerComposeFile string             `json:"dockerComposeFile,omitempty"`
	DockerWouldStop   bool               `json:"dockerWouldStop"`
}

func verifyArchive(cfg config.Config, explicit string) (verificationResult, error) {
	state := &state{config: cfg}
	if err := resolveArchive(state, explicit); err != nil {
		return verificationResult{}, err
	}
	stats, err := archive.Inspect(state.archivePath)
	if err != nil {
		return verificationResult{}, err
	}
	temporary, err := os.MkdirTemp("", "release-verify-*")
	if err != nil {
		return verificationResult{}, err
	}
	defer tools.RemoveTree(temporary)
	if err := archive.Extract(state.archivePath, temporary); err != nil {
		return verificationResult{}, err
	}
	contentRoot, err := archive.ResolveContentRoot(temporary)
	if err != nil {
		return verificationResult{}, err
	}
	if err := archive.ValidateVersionFile(contentRoot, state.version.String()); err != nil {
		return verificationResult{}, err
	}
	return verificationResult{
		ProjectName: cfg.ProjectName,
		ArchivePath: state.sourceReference,
		Version:     state.version.String(),
		ContentRoot: filepath.Base(contentRoot),
		Stats:       stats,
		Valid:       true,
	}, nil
}

func enforceVersionPolicy(cfg config.Config, target versionutil.Version, allowDowngrade, force, simulation bool) error {
	installed, _, found, err := updatecheck.DetectInstalled(cfg.CurrentDir)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	comparison := target.Compare(installed)
	if comparison < 0 && !allowDowngrade {
		return fmt.Errorf(
			"Downgrade wird blockiert: installiert ist %s, ausgewählt wurde %s\nZum ausdrücklichen Downgrade --allow-downgrade verwenden",
			installed.String(), target.String(),
		)
	}
	if comparison == 0 && !force && !simulation {
		return &VersionAlreadyInstalledError{Version: installed.String()}
	}
	return nil
}

func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("JSON-Ausgabe fehlgeschlagen: %w", err)
	}
	return nil
}

func doctorJSON(report doctor.Report) map[string]any {
	return map[string]any{
		"root":         report.Root,
		"config":       report.Config,
		"checks":       report.Checks,
		"errorCount":   report.ErrorCount(),
		"warningCount": report.WarningCount(),
		"healthy":      report.ErrorCount() == 0,
	}
}

func updateCheckJSON(result updatecheck.Result) map[string]any {
	value := map[string]any{
		"projectName":     result.ProjectName,
		"sourceType":      result.SourceType,
		"installed":       nil,
		"available":       result.Available.String(),
		"archivePath":     result.ArchivePath,
		"status":          result.Status,
		"updateAvailable": result.Status == updatecheck.StatusUpdateAvailable || result.Status == updatecheck.StatusNotInstalled,
	}
	if result.InstalledFound {
		value["installed"] = result.Installed.String()
		value["installedSource"] = result.InstalledSource
	}
	return value
}

func updatePlanJSON(state *state) updatePlanResult {
	result := updatePlanResult{
		ProjectName:       state.config.ProjectName,
		SourceType:        state.sourceType,
		Source:            state.sourceReference,
		ToVersion:         state.version.String(),
		ArchivePath:       state.sourceReference,
		ReleaseDir:        state.releaseDir,
		CurrentDir:        state.config.CurrentDir,
		Created:           []rsyncutil.Change{},
		Updated:           []rsyncutil.Change{},
		Deleted:           []rsyncutil.Change{},
		Other:             []rsyncutil.Change{},
		Protected:         []string{".git/", ".venv/", ".env"},
		DockerComposeFile: state.dockerComposeFile,
		DockerWouldStop:   state.dockerComposeFile != "",
	}
	if installed, _, found, _ := updatecheck.DetectInstalled(state.config.CurrentDir); found {
		result.FromVersion = installed.String()
	}
	for _, change := range state.currentPlan {
		switch change.Kind {
		case rsyncutil.ChangeCreated:
			result.Created = append(result.Created, change)
		case rsyncutil.ChangeUpdated:
			result.Updated = append(result.Updated, change)
		case rsyncutil.ChangeDeleted:
			result.Deleted = append(result.Deleted, change)
		default:
			result.Other = append(result.Other, change)
		}
	}
	return result
}

func printStatus(console *ui.Console, result projectstatus.Result) {
	console.Header("Updater-Status")
	console.Row("Projekt", result.ProjectName)
	console.Row("Quellentyp", emptyDash(result.SourceType))
	if result.SourceReference != "" {
		console.Row("Quelle", result.SourceReference)
	}
	console.Row("Konfiguration", result.ConfigFile)
	console.Row("Current", result.CurrentDir)
	console.Row("Release", result.ReleaseDir)
	if result.InstalledVersion == "" {
		console.Row("Installiert", "keine Version erkannt")
	} else {
		console.Row("Installiert", result.InstalledVersion)
	}
	if result.AvailableVersion == "" {
		console.Row("Verfügbar", "keine passende Release-Quelle")
	} else {
		console.Row("Verfügbar", result.AvailableVersion)
		console.Row("Release-Quelle", result.ArchivePath)
	}
	console.Row("Setup", fmt.Sprintf("setup.sh=%t, commands=%d", result.SetupScript, result.SetupCommands))
	console.Row("Backups", fmt.Sprintf("%d", result.BackupCount))
	if result.LatestBackup != "" {
		console.Row("Letztes Backup", result.LatestBackup)
	}
	console.Row("Historie", fmt.Sprintf("%d Einträge", result.HistoryEntries))
	switch result.State {
	case "update-available":
		console.Diagnostic("warning", "Status", fmt.Sprintf("Update verfügbar: %s → %s", result.InstalledVersion, result.AvailableVersion))
	case "current":
		console.Diagnostic("ok", "Status", "installierte Version ist aktuell")
	case "local-newer":
		console.Diagnostic("warning", "Status", "lokale Version ist neuer als das Download-Archiv")
	case "not-installed":
		console.Diagnostic("warning", "Status", "noch keine Installation; Release ist verfügbar")
	case "no-download":
		console.Diagnostic("warning", "Status", "Installation vorhanden, aber kein passendes Download-Archiv")
	default:
		console.Diagnostic("warning", "Status", "noch keine Installation und kein Release-Archiv")
	}
}

func printInventory(console *ui.Console, result inventory.Result) {
	console.Header("Release-Inventar")
	console.Row("Projekt", result.ProjectName)
	sourceType := result.SourceType
	if sourceType == "" {
		sourceType = source.Download
	}
	fmt.Fprintf(os.Stdout, "\n  Quellen (%s)\n", sourceType)
	if len(result.Downloads) == 0 {
		fmt.Fprintln(os.Stdout, "    keine passende Quelle")
	} else if sourceType == source.Download {
		fmt.Fprintf(os.Stdout, "    %-12s %-11s %-19s %s\n", "VERSION", "GRÖSSE", "GEÄNDERT", "ARCHIV")
		for _, item := range result.Downloads {
			fmt.Fprintf(os.Stdout, "    %-12s %-11s %-19s %s\n", item.VersionS, humanBytes(item.Size), item.Modified.Format("2006-01-02 15:04"), item.Path)
		}
	} else {
		fmt.Fprintf(os.Stdout, "    %-12s %-12s %s\n", "VERSION", "TYP", "QUELLE")
		for _, item := range result.Downloads {
			fmt.Fprintf(os.Stdout, "    %-12s %-12s %s\n", item.VersionS, sourceType, item.Path)
		}
	}
	fmt.Fprintln(os.Stdout, "\n  Entpackte Releases")
	if len(result.Releases) == 0 {
		fmt.Fprintln(os.Stdout, "    keine Releases")
	} else {
		fmt.Fprintf(os.Stdout, "    %-12s %-8s %-10s %-19s %s\n", "VERSION", "AKTIV", "VALIDIERT", "GEÄNDERT", "ORDNER")
		for _, item := range result.Releases {
			fmt.Fprintf(os.Stdout, "    %-12s %-8t %-10t %-19s %s\n", item.Version, item.Active, item.Validated, item.Modified.Format("2006-01-02 15:04"), item.Path)
		}
	}
	fmt.Fprintln(os.Stdout, "\n  Backups")
	if len(result.Backups) == 0 {
		fmt.Fprintln(os.Stdout, "    keine Backups")
	} else {
		fmt.Fprintf(os.Stdout, "    %-24s %-12s %-10s %-19s %s\n", "NAME", "VERSION", "VALIDIERT", "ERSTELLT", "ORDNER")
		for _, item := range result.Backups {
			fmt.Fprintf(os.Stdout, "    %-24s %-12s %-10t %-19s %s\n", item.Name, emptyDash(item.Version), item.Validated, item.CreatedAt.Local().Format("2006-01-02 15:04"), item.Path)
		}
	}
}

func printVerification(console *ui.Console, result verificationResult) {
	console.Header("Archivprüfung")
	console.Row("Projekt", result.ProjectName)
	console.Row("Archiv", result.ArchivePath)
	console.Row("Version", result.Version)
	console.Row("Einträge", fmt.Sprintf("%d", result.Stats.Entries))
	console.Row("Dateien", fmt.Sprintf("%d", result.Stats.Files))
	console.Row("Ordner", fmt.Sprintf("%d", result.Stats.Directories))
	console.Row("Entpackte Größe", humanBytes(result.Stats.UncompressedBytes))
	console.Success("ZIP, Pfade, Prüfsummen und VERSION sind gültig")
}

func printDetailedPlan(console *ui.Console, state *state) {
	result := updatePlanJSON(state)
	console.Header("Detaillierter Update-Plan")
	console.Row("Projekt", result.ProjectName)
	if result.FromVersion == "" {
		console.Row("Version", "Neuinstallation → "+result.ToVersion)
	} else {
		console.Row("Version", result.FromVersion+" → "+result.ToVersion)
	}
	console.Row("Neu", fmt.Sprintf("%d", len(result.Created)))
	console.Row("Geändert", fmt.Sprintf("%d", len(result.Updated)))
	console.Row("Gelöscht", fmt.Sprintf("%d", len(result.Deleted)))
	console.Row("Sonstige", fmt.Sprintf("%d", len(result.Other)))
	console.Row("Geschützt", strings.Join(result.Protected, ", "))
	printChanges := func(marker string, changes []rsyncutil.Change) {
		for _, change := range changes {
			fmt.Fprintf(os.Stdout, "  %s %s\n", marker, change.Path)
		}
	}
	if len(state.currentPlan) > 0 {
		fmt.Fprintln(os.Stdout)
		printChanges("+", result.Created)
		printChanges("~", result.Updated)
		printChanges("-", result.Deleted)
		printChanges("·", result.Other)
	}
	fmt.Fprintln(os.Stdout)
	console.Success("Plan abgeschlossen; das Projekt wurde nicht verändert")
}

func currentHasContent(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("current-Verzeichnis kann nicht gelesen werden: %w", err)
	}
	return len(entries) > 0, nil
}

func installedVersion(currentDir string) string {
	installed, _, found, err := updatecheck.DetectInstalled(currentDir)
	if err != nil || !found {
		return ""
	}
	return installed.String()
}

func appendHistory(cfg config.Config, entry history.Entry) error {
	if entry.ProjectName == "" {
		entry.ProjectName = cfg.ProjectName
	}
	return history.Append(cfg.HistoryFile, entry)
}

func printHistory(console *ui.Console, cfg config.Config, entries []history.Entry) {
	console.Header("Update-Historie")
	console.Row("Projekt", cfg.ProjectName)
	console.Row("Datei", cfg.HistoryFile)
	if len(entries) == 0 {
		console.Diagnostic("warning", "Historie", "noch keine Einträge")
		return
	}
	fmt.Fprintf(os.Stdout, "\n  %-19s %-11s %-12s %-12s %-13s %s\n", "ZEIT", "AKTION", "VON", "NACH", "STATUS", "DETAIL")
	for _, entry := range entries {
		detail := entry.Backup
		if detail == "" {
			detail = entry.Archive
		}
		if detail == "" {
			detail = entry.Message
		}
		fmt.Fprintf(os.Stdout, "  %-19s %-11s %-12s %-12s %-13s %s\n",
			entry.Timestamp.Local().Format("2006-01-02 15:04"), entry.Action, emptyDash(entry.FromVersion), emptyDash(entry.ToVersion), entry.Status, detail)
	}
}

func printBackup(console *ui.Console, result backup.Result) {
	console.Header("Backup erstellt")
	console.Row("Name", result.Backup.Name)
	console.Row("Version", emptyDash(result.Backup.Version))
	console.Row("Ordner", result.Backup.Path)
	console.Row("Änderungen", fmt.Sprintf("%d", result.Sync.Changes))
	console.Success("current wurde gesichert")
}

func printRollback(console *ui.Console, result rollback.Result, setup bool) {
	console.Header("Rollback abgeschlossen")
	console.Row("Version", emptyDash(result.FromVersion)+" → "+result.ToVersion)
	console.Row("Release", result.ReleaseDir)
	console.Row("Current", result.CurrentDir)
	console.Row("Änderungen", fmt.Sprintf("%d", result.Sync.Changes))
	console.Row("Setup", fmt.Sprintf("%t", setup))
	console.Success("Release wurde aktiviert")
}

func printRestore(console *ui.Console, item backup.Item, fromVersion, toVersion, currentDir string, result rsyncutil.Result) {
	console.Header("Backup wiederhergestellt")
	console.Row("Backup", item.Name)
	console.Row("Version", emptyDash(fromVersion)+" → "+emptyDash(toVersion))
	console.Row("Quelle", item.Path)
	console.Row("Current", currentDir)
	console.Row("Änderungen", fmt.Sprintf("%d", result.Changes))
	console.Success("Backup wurde nach current synchronisiert")
}

func printCleanup(console *ui.Console, result cleanup.Result) {
	title := "Cleanup abgeschlossen"
	if result.Plan {
		title = "Cleanup-Plan"
	}
	console.Header(title)
	console.Row("Behalte Releases", fmt.Sprintf("%d", result.KeepReleases))
	console.Row("Behalte Backups", fmt.Sprintf("%d", result.KeepBackups))
	console.Row("Releases entfernen", fmt.Sprintf("%d", len(result.RemovedRelease)))
	console.Row("Backups entfernen", fmt.Sprintf("%d", len(result.RemovedBackup)))
	for _, path := range result.RemovedRelease {
		fmt.Fprintf(os.Stdout, "  - Release %s\n", path)
	}
	for _, path := range result.RemovedBackup {
		fmt.Fprintf(os.Stdout, "  - Backup  %s\n", path)
	}
	if result.Plan {
		console.Success("Plan abgeschlossen; es wurden keine Dateien entfernt")
	} else {
		console.Success("Alte Releases und Backups wurden bereinigt")
	}
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "–"
	}
	return value
}

func humanBytes(value int64) string {
	const unit = int64(1024)
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	divisor := unit
	exponent := 0
	for quotient := value / unit; quotient >= unit && exponent < 5; quotient /= unit {
		divisor *= unit
		exponent++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(divisor), "KMGTPE"[exponent])
}

func printUpdateCheck(console *ui.Console, cfg config.Config, result updatecheck.Result) {
	console.Header("Versionsprüfung")
	console.Row("Projekt", cfg.ProjectName)
	if result.InstalledFound {
		console.Row("Quelle", result.InstalledSource)
	} else {
		console.Row("Quelle", "keine Versionsquelle erkannt")
	}
	console.Row("Archiv", result.ArchivePath)
	if result.InstalledFound {
		console.Row("Installiert", result.Installed.String())
	} else {
		console.Row("Installiert", "keine Version erkannt")
	}
	console.Row("Verfügbar", result.Available.String())

	status := ""
	switch result.Status {
	case updatecheck.StatusUpdateAvailable:
		status = fmt.Sprintf("Update verfügbar: %s → %s", result.Installed.String(), result.Available.String())
	case updatecheck.StatusCurrent:
		status = "installierte Version ist aktuell"
	case updatecheck.StatusLocalNewer:
		status = fmt.Sprintf("lokale Version %s ist neuer als Quelle %s", result.Installed.String(), result.Available.String())
	case updatecheck.StatusNotInstalled:
		status = fmt.Sprintf("keine Installation erkannt; Version %s ist verfügbar", result.Available.String())
	}
	console.StatusRow("Status", status)
}

func printDoctor(console *ui.Console, report doctor.Report) {
	console.Header("Updater Doctor")
	console.Row("Projektordner", report.Root)
	fmt.Fprintln(os.Stdout)
	for _, check := range report.Checks {
		console.Diagnostic(string(check.Level), check.Name, check.Detail)
	}
	fmt.Fprintln(os.Stdout)
	console.Row("Fehler", fmt.Sprintf("%d", report.ErrorCount()))
	console.Row("Warnungen", fmt.Sprintf("%d", report.WarningCount()))
	if report.ErrorCount() == 0 {
		console.Success("Umgebung ist einsatzbereit")
	}
}

func updateHeader(fromVersion, toVersion string) string {
	if strings.TrimSpace(fromVersion) == "" {
		fromVersion = "none"
	}
	return fmt.Sprintf("Release Update     from %s to %s", fromVersion, toVersion)
}

func printPlan(console *ui.Console, state *state, dryRun, detailedPlan, runSetup, allowDowngrade bool) {
	console.Banner(updateHeader(state.fromVersion, state.version.String()))
	console.Row("Projekt", state.config.ProjectName)
	console.Row("Root", state.config.RootDir)
	console.Row("Konfiguration", displayPath(state.config.RootDir, state.config.ConfigFile))
	console.Row("Von", state.sourceType)
	console.Row("Quelle", sourceDisplay(state))
	console.Row("Version", state.version.String())
	console.Row("Release", displayPath(state.config.RootDir, state.releaseDir))
	console.Row("Current", displayPath(state.config.RootDir, state.config.CurrentDir))
	if state.dockerComposeFile != "" {
		dockerText := displayPath(state.config.RootDir, state.dockerComposeFile) + " — wird vor dem Update gestoppt"
		if dryRun || detailedPlan {
			dockerText = displayPath(state.config.RootDir, state.dockerComposeFile) + " — würde vor dem Update gestoppt"
		}
		console.Row("Docker", dockerText)
	} else {
		console.Row("Docker", "kein Compose-Projekt erkannt")
	}
	console.Row("Geschützt", ".git/, .venv/, .env")
	if detailedPlan {
		console.Row("Modus", "Detaillierter Update-Plan")
	} else if dryRun {
		console.Row("Modus", "Dry-Run")
	}
	if allowDowngrade {
		console.Row("Downgrade", "ausdrücklich erlaubt")
	}
	if runSetup {
		console.Row("Nach Update", "Projekt-Setup ausführen")
	}
	fmt.Fprintln(os.Stdout)
}

func printResult(console *ui.Console, state *state, dryRun bool) {
	if dryRun {
		console.Header("Dry-Run abgeschlossen")
		console.Row("Release-Plan", fmt.Sprintf("%d Änderungen", state.releaseChanges))
		console.Row("Current-Plan", fmt.Sprintf("%d Änderungen", state.currentChanges))
		console.Row("Dateisystem", "unverändert")
		return
	}

	console.Header("Update abgeschlossen")
	console.Row("Projekt", state.config.ProjectName)
	console.Row("Root", state.config.RootDir)
	console.Row("Von", state.sourceType)
	console.Row("Quelle", sourceDisplay(state))
	console.Row("Version", state.version.String())
	console.Row("Release", displayPath(state.config.RootDir, state.releaseDir))
	console.Row("Current", displayPath(state.config.RootDir, state.config.CurrentDir))
	if state.dockerComposeFile != "" {
		status := "erkannt"
		if state.dockerStopped {
			status = "vor dem Update gestoppt"
		}
		console.Row("Docker", status+" — "+displayPath(state.config.RootDir, state.dockerComposeFile))
	}
	console.Row("Release-Dateien", fmt.Sprintf("%d Änderungen", state.releaseChanges))
	console.Row("Current-Dateien", fmt.Sprintf("%d Änderungen", state.currentChanges))
	console.Row("Beibehalten", ".git/, .venv/, .env")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sourceDisplay(state *state) string {
	if state.sourceType == source.Download {
		return displayPath(state.config.RootDir, state.sourceReference)
	}
	return state.sourceReference
}

func countFiles(root string) (int, error) {
	count := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != root {
			count++
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("Repository-Dateien können nicht gezählt werden: %w", err)
	}
	return count, nil
}

func displayPath(root, path string) string {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if path == root {
		return root
	}
	relative, err := filepath.Rel(root, path)
	if err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative) {
		return "./" + filepath.ToSlash(relative)
	}
	return path
}
