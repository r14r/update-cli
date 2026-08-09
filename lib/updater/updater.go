package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/r14r/update-cli/lib/archive"
	"github.com/r14r/update-cli/lib/backup"
	"github.com/r14r/update-cli/lib/cleanup"
	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/doctor"
	"github.com/r14r/update-cli/lib/editor"
	"github.com/r14r/update-cli/lib/history"
	"github.com/r14r/update-cli/lib/inventory"
	"github.com/r14r/update-cli/lib/projectsetup"
	"github.com/r14r/update-cli/lib/projectstatus"
	"github.com/r14r/update-cli/lib/rollback"
	rsyncutil "github.com/r14r/update-cli/lib/rsync"
	"github.com/r14r/update-cli/lib/source"
	"github.com/r14r/update-cli/lib/templates"
	"github.com/r14r/update-cli/lib/tools"
	"github.com/r14r/update-cli/lib/ui"
	"github.com/r14r/update-cli/lib/updatecheck"
	versionutil "github.com/r14r/update-cli/lib/version"
)

type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}
func (e *ExitError) Unwrap() error { return e.Err }

type VersionAlreadyInstalledError struct{ Version string }

func (e *VersionAlreadyInstalledError) Error() string {
	return fmt.Sprintf("Version %s ist bereits installiert", e.Version)
}

type state struct {
	cfg                                                                                     config.Config
	artifact                                                                                source.Artifact
	version                                                                                 versionutil.Version
	workDir, extractDir, contentDir, releaseDir, releaseStage, dryReleaseDir, dryCurrentDir string
	releaseChanges, currentChanges                                                          int
	currentPlan                                                                             []rsyncutil.Change
	fromVersion                                                                             string
}

func Run(ctx context.Context, buildVersion string, args []string) (retErr error) {
	if len(args) == 0 {
		return runNoParameter(ctx, buildVersion)
	}
	o, err := parseOptions(args)
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}
	if o.showHelp {
		printHelp(buildVersion)
		return nil
	}
	if o.showHowTo {
		printHowTo(buildVersion)
		return nil
	}
	if o.showVersion {
		fmt.Printf("Update CLI %s\n", buildVersion)
		return nil
	}
	console := ui.New(o.noColor || o.jsonOutput)
	console.SetDirect(o.noUI)
	console.SetDetails(o.details || o.noUI)
	fullscreenTitle := ""
	switch {
	case o.setupManifest != "" || (o.setup && !o.update) || o.setupList || o.setupTask != "" || o.setupWorkflow != "":
		fullscreenTitle = "Update CLI Setup"
	case o.check:
		fullscreenTitle = "Update CLI — Versionsprüfung"
	case o.update && !o.plan && !o.dryRun:
		fullscreenTitle = "Update CLI — Update"
	}
	fullscreen := false
	if fullscreenTitle != "" && !o.jsonOutput {
		fullscreen = console.StartFullscreen(fullscreenTitle)
		if fullscreen {
			switch fullscreenTitle {
			case "Update CLI Setup":
				console.SetFooter("RUN  Projekt-Setup läuft")
			case "Update CLI — Versionsprüfung":
				console.SetFooter("RUN  Versionsprüfung läuft")
			case "Update CLI — Update":
				console.SetFooter("RUN  Update läuft")
			}
		}
	}
	if fullscreen {
		defer func() {
			if retErr != nil && !console.ErrorShown() {
				console.ErrorNotice("Fehlerdetails", retErr.Error())
			}
			console.FinishFullscreen(retErr == nil, !o.noWait && ctx.Err() == nil)
		}()
	}
	if o.setupManifest != "" {
		if o.setupList {
			catalog, err := projectsetup.CatalogForManifest(o.setupManifest)
			if err != nil {
				return err
			}
			printSetupCatalog(console, catalog)
			return nil
		}
		_, err := projectsetup.RunStandaloneSelected(ctx, o.setupManifest, console, projectsetup.Selection{Workflow: o.setupWorkflow, Task: o.setupTask})
		return err
	}
	root, err := config.ResolveRoot(o.rootDir)
	if err != nil {
		return err
	}
	if o.convertYAML || o.createYAML || o.createSetupScript {
		targetDir, targetErr := setupManagementDirectory(root)
		if targetErr != nil {
			return targetErr
		}
		switch {
		case o.convertYAML:
			manifest, ok, findErr := projectsetup.FindManifest(targetDir)
			if findErr != nil {
				return findErr
			}
			if !ok {
				return fmt.Errorf("kein setup.yaml/setup.yml in %s", targetDir)
			}
			if o.dryRun {
				text, previous, previewErr := projectsetup.PreviewConvertManifest(manifest)
				if previewErr != nil {
					return previewErr
				}
				console.Header("setup.yaml Konvertierung — Dry-Run")
				console.Row("Datei", manifest)
				console.Row("Schema", fmt.Sprintf("%d → 2", previous))
				fmt.Print(text)
				return nil
			}
			res, convertErr := projectsetup.ConvertManifestToLatest(manifest, o.force)
			if convertErr != nil {
				return convertErr
			}
			console.Header("setup.yaml konvertiert")
			console.Row("Datei", res.Path)
			console.Row("Schema", fmt.Sprintf("%d → %d", res.PreviousSchema, res.CurrentSchema))
			if res.BackupPath != "" {
				console.Row("Backup", res.BackupPath)
			}
			if res.Changed {
				console.Success("setup.yaml wurde auf das aktuelle Schema migriert")
			} else {
				console.Success("setup.yaml verwendet bereits das aktuelle Schema")
			}
			return nil
		case o.createYAML:
			if o.dryRun {
				text, tech, previewErr := projectsetup.PreviewGeneratedManifest(targetDir)
				if previewErr != nil {
					return previewErr
				}
				console.Header("setup.yaml Generator — Dry-Run")
				console.Row("Projektordner", targetDir)
				console.Row("Erkannt", strings.Join(tech, ", "))
				fmt.Print(text)
				return nil
			}
			res, createErr := projectsetup.GenerateManifest(targetDir, "", o.force)
			if createErr != nil {
				return createErr
			}
			console.Header("setup.yaml erstellt")
			console.Row("Datei", res.Path)
			console.Row("Erkannt", strings.Join(res.Technologies, ", "))
			if res.Overwritten {
				console.Warn("Vorhandenes setup.yaml wurde mit --force ersetzt")
			}
			console.Success("SchemaVersion 2 Manifest wurde erzeugt; vor produktivem Einsatz prüfen")
			return nil
		case o.createSetupScript:
			if o.dryRun {
				console.Header("setup.sh Generator — Dry-Run")
				console.Row("Projektordner", targetDir)
				fmt.Print(projectsetup.SetupScriptTemplate())
				return nil
			}
			res, createErr := projectsetup.GenerateSetupScript(targetDir, "", o.force)
			if createErr != nil {
				return createErr
			}
			console.Header("setup.sh erstellt")
			console.Row("Datei", res.Path)
			if res.Overwritten {
				console.Warn("Vorhandenes setup.sh wurde mit --force ersetzt")
			}
			console.Success("Setup-Bootstrap wurde erzeugt")
			return nil
		}
	}
	standaloneSetupCommand := (o.setup && !o.update && !o.rollback) || o.setupList || o.setupTask != "" || o.setupWorkflow != ""
	if standaloneSetupCommand {
		selection := projectsetup.Selection{Workflow: o.setupWorkflow, Task: o.setupTask}
		configFile := filepath.Join(root, config.ConfigDirName, config.ConfigFileName)
		_, statErr := os.Stat(configFile)
		if statErr == nil {
			cfg, loadErr := config.Load(root, o.downloadDir)
			if loadErr != nil {
				return loadErr
			}
			if o.setup && !o.setupList && o.setupTask == "" && o.setupWorkflow == "" {
				_, setupErr := projectsetup.Run(ctx, cfg, console)
				return setupErr
			}
			manifest := projectsetup.ManifestPath(cfg)
			if manifest == "" {
				return fmt.Errorf("kein setup.yaml/setup.yml in %s", cfg.CurrentDir)
			}
			if o.setupList {
				catalog, catalogErr := projectsetup.CatalogForManifest(manifest)
				if catalogErr != nil {
					return catalogErr
				}
				printSetupCatalog(console, catalog)
				return nil
			}
			_, setupErr := projectsetup.RunSelected(ctx, cfg, console, selection)
			return setupErr
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
		manifest, ok, findErr := projectsetup.FindManifest(root)
		if findErr != nil {
			return findErr
		}
		if !ok {
			if o.setup && !o.setupList && o.setupTask == "" && o.setupWorkflow == "" {
				cfg := config.Config{ProjectName: filepath.Base(root), CurrentDir: root}
				_, setupErr := projectsetup.Run(ctx, cfg, console)
				return setupErr
			}
			return fmt.Errorf("kein setup.yaml/setup.yml im aktuellen Ordner %s", root)
		}
		if o.setupList {
			catalog, catalogErr := projectsetup.CatalogForManifest(manifest)
			if catalogErr != nil {
				return catalogErr
			}
			printSetupCatalog(console, catalog)
			return nil
		}
		_, setupErr := projectsetup.RunStandaloneSelected(ctx, manifest, console, selection)
		return setupErr
	}
	if o.unlock {
		return tools.UnlockStale(filepath.Join(root, ".release-update.lock"))
	}
	if o.init {
		return initialize(console, root, o)
	}
	if o.upgrade {
		lock, err := tools.AcquireLock(filepath.Join(root, ".release-update.lock"), "upgrade")
		if err != nil {
			return err
		}
		defer lock.Release()
		res, err := config.Upgrade(root)
		if err != nil {
			return err
		}
		if o.jsonOutput {
			return writeJSON(res)
		}
		printUpgrade(console, res)
		return nil
	}
	if o.config {
		return runConfig(ctx, console, root, o)
	}
	if o.templatesMode {
		return runTemplates(ctx, console, root, o, buildVersion)
	}
	cfg, err := config.Load(root, o.downloadDir)
	if err != nil {
		return err
	}
	cfg, err = config.WithSourceOverrides(cfg, o.sourceType, firstNonEmpty(o.sourceFolder, o.downloadDir), o.sourceURL, o.repository)
	if err != nil {
		return err
	}
	switch {
	case o.history:
		entries, err := history.List(cfg.HistoryFile, o.limit)
		if err != nil {
			return err
		}
		if o.jsonOutput {
			return writeJSON(entries)
		}
		printHistory(console, entries)
		return nil
	case o.cleanup:
		lock, err := tools.AcquireLock(filepath.Join(root, ".release-update.lock"), "cleanup")
		if err != nil {
			return err
		}
		defer lock.Release()
		res, err := cleanup.Run(cfg, o.keep, o.plan)
		if err != nil {
			return err
		}
		if !o.plan {
			if err := appendHistory(cfg, history.Entry{Action: "cleanup", ProjectName: cfg.ProjectName, Status: "success", Message: fmt.Sprintf("%d Releases und %d Backups entfernt", len(res.RemovedRelease), len(res.RemovedBackup))}); err != nil {
				return err
			}
		}
		if o.jsonOutput {
			return writeJSON(res)
		}
		printCleanup(console, res)
		return nil
	case o.backup && !o.update:
		return runBackup(ctx, console, cfg, o.jsonOutput)
	case o.rollback:
		return runRollback(ctx, console, cfg, o)
	case o.restore != "":
		return runRestore(ctx, console, cfg, o)
	case o.status:
		res, err := projectstatus.Run(ctx, cfg)
		if err != nil {
			return err
		}
		if o.jsonOutput {
			return writeJSON(res)
		}
		printStatus(console, res)
		return nil
	case o.list:
		res, err := inventory.List(ctx, cfg)
		if err != nil {
			return err
		}
		if o.jsonOutput {
			return writeJSON(res)
		}
		printInventory(console, res)
		return nil
	case o.check:
		res, err := updatecheck.Run(ctx, cfg)
		if err != nil {
			return err
		}
		if o.jsonOutput {
			return writeJSON(res)
		}
		printCheck(console, res)
		if !o.noAsk && console.Interactive() && res.SourceError == "" && (res.Status == updatecheck.StatusUpdateAvailable || res.Status == updatecheck.StatusNotInstalled) {
			yes, confirmErr := console.Confirm("Update jetzt installieren?", false)
			if confirmErr != nil {
				return confirmErr
			}
			if yes {
				console.StartFullscreen("Update CLI — Update")
				console.SetFooter("RUN  Update läuft")
				updateOpts := options{update: true, noWait: o.noWait, wait: o.wait, noUI: o.noUI, noColor: o.noColor}
				for _, action := range cfg.NoParameterActions {
					if action == "setup" {
						updateOpts.setup = true
					}
				}
				return runUpdate(ctx, console, cfg, updateOpts)
			}
		}
		return nil
	case o.doctor:
		res := doctor.Run(ctx, root, cfg)
		if o.jsonOutput {
			if err := writeJSON(res); err != nil {
				return err
			}
		} else {
			printDoctor(console, res)
		}
		if res.ErrorCount() > 0 {
			return &ExitError{Code: 1}
		}
		return nil
	case o.verify:
		res, err := verifyArchive(ctx, cfg, o.archive)
		if err != nil {
			return err
		}
		if o.jsonOutput {
			return writeJSON(res)
		}
		printVerify(console, res)
		return nil
	case o.setup && !o.update:
		lock, err := tools.AcquireLock(filepath.Join(root, ".release-update.lock"), "setup")
		if err != nil {
			return err
		}
		defer lock.Release()
		_, err = projectsetup.Run(ctx, cfg, console)
		return err
	case o.update:
		return runUpdate(ctx, console, cfg, o)
	}
	return errors.New("unbekannte Betriebsart")
}

func runNoParameter(ctx context.Context, buildVersion string) error {
	root, err := config.ResolveRoot("")
	if err != nil {
		printHelp(buildVersion)
		return nil
	}
	cfg, err := config.Load(root, "")
	if err != nil {
		configFile := filepath.Join(root, config.ConfigDirName, config.ConfigFileName)
		if _, statErr := os.Stat(configFile); errors.Is(statErr, os.ErrNotExist) {
			printHelp(buildVersion)
			return nil
		}
		return err
	}
	if len(cfg.NoParameterActions) == 0 || (len(cfg.NoParameterActions) == 1 && cfg.NoParameterActions[0] == "help") {
		printHelp(buildVersion)
		return nil
	}
	args := []string{}
	actions := make(map[string]bool, len(cfg.NoParameterActions))
	for _, action := range cfg.NoParameterActions {
		actions[action] = true
	}
	switch {
	case actions["check"]:
		// setup is intentionally not forwarded as --setup here. For the
		// historical ["check", "setup"] configuration it is a modifier used
		// by the check flow after the user confirms the offered update.
		args = append(args, "--check")
	case actions["update"]:
		args = append(args, "--update")
		if actions["setup"] {
			args = append(args, "--setup")
		}
	case actions["setup"]:
		args = append(args, "--setup")
	default:
		printHelp(buildVersion)
		return nil
	}
	args = append(args, "--root", root)
	return Run(ctx, buildVersion, args)
}
func runBackup(ctx context.Context, console *ui.Console, cfg config.Config, jsonOut bool) error {
	lock, err := tools.AcquireLock(filepath.Join(cfg.RootDir, ".release-update.lock"), "backup")
	if err != nil {
		return err
	}
	defer lock.Release()
	res, err := backup.Create(ctx, cfg, false)
	if err != nil {
		return err
	}
	if err := appendHistory(cfg, history.Entry{Action: "backup", ProjectName: cfg.ProjectName, FromVersion: res.Backup.Version, Backup: res.Backup.Path, Status: "success"}); err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(res)
	}
	printBackup(console, res)
	return nil
}

func runUpdate(ctx context.Context, console *ui.Console, cfg config.Config, o options) (retErr error) {
	lock, err := tools.AcquireLock(filepath.Join(cfg.RootDir, ".release-update.lock"), "update")
	if err != nil {
		return err
	}
	defer lock.Release()

	s := &state{cfg: cfg, fromVersion: installedVersion(cfg.CurrentDir)}
	s.workDir, err = os.MkdirTemp("", "update-cli-*")
	if err != nil {
		return err
	}
	defer tools.RemoveTree(s.workDir)
	s.extractDir = filepath.Join(s.workDir, "extract")

	totalSteps := 13
	if o.plan || o.dryRun {
		totalSteps = 5
	}
	progress := newUpdateProgress(ctx, console, !o.jsonOutput, totalSteps)
	phase := "source"
	if err := progress.run("Release-Quelle auflösen", func() error {
		return resolveArtifact(ctx, s, o.archive)
	}); err != nil {
		return failUpdateBeforeTransaction(console, cfg, s, phase, err)
	}

	phase = "version-policy"
	if err := progress.run("Zielversion und Update-Regeln prüfen", func() error {
		return enforceVersionPolicy(cfg, s.version, o.allowDowngrade, o.force, o.plan || o.dryRun)
	}); err != nil {
		var same *VersionAlreadyInstalledError
		if errors.As(err, &same) && !o.jsonOutput {
			console.ErrorNotice(err.Error(), "Phase: "+updatePhaseLabel(phase)+"\nZur erneuten Installation --update --force verwenden")
			return &ExitError{Code: 1}
		}
		return failUpdateBeforeTransaction(console, cfg, s, phase, err)
	}

	s.releaseDir = filepath.Join(cfg.ReleaseRoot, s.version.String())
	if !o.jsonOutput {
		printUpdatePlan(console, s, o)
	}

	phase = "validate-artifact"
	if err := progress.run("Release-Inhalt und Archiv validieren", func() error {
		return prepareContent(ctx, s)
	}); err != nil {
		return failUpdateBeforeTransaction(console, cfg, s, phase, err)
	}

	phase = "prepare-release"
	if err := progress.run("Versioniertes Release vorbereiten", func() error {
		return prepareRelease(ctx, s, o.plan || o.dryRun)
	}); err != nil {
		return failUpdateBeforeTransaction(console, cfg, s, phase, err)
	}
	defer func() {
		if s.releaseStage != "" {
			_ = tools.RemoveTree(s.releaseStage)
		}
	}()

	if o.plan || o.dryRun {
		phase = "plan-current"
		if err := progress.run("Änderungen an current ermitteln", func() error {
			return syncCurrent(ctx, s, true)
		}); err != nil {
			return failUpdateBeforeTransaction(console, cfg, s, phase, err)
		}
		if o.jsonOutput {
			return writeJSON(updatePlanJSON(s))
		}
		if o.plan {
			printDetailedPlan(console, s)
		} else {
			printDryRun(console, s)
		}
		return nil
	}

	phase = "transaction-begin"
	var tx *transaction
	if err := progress.run("Transaktions-Snapshot von current erstellen", func() error {
		var beginErr error
		tx, beginErr = beginTransaction(ctx, cfg, console)
		return beginErr
	}); err != nil {
		return failUpdateBeforeTransaction(console, cfg, s, phase, err)
	}

	backupPath := ""
	phase = "backup"
	if o.backup && tx.currentExisted {
		if err := progress.run("Persistentes Pre-Update-Backup erstellen", func() error {
			b, backupErr := backup.Create(ctx, cfg, false)
			if backupErr != nil {
				return backupErr
			}
			backupPath = b.Backup.Path
			return nil
		}); err != nil {
			return failUpdateWithRecovery(console, cfg, s, tx, phase, err, nil)
		}
	} else if o.backup {
		progress.skip("Persistentes Pre-Update-Backup erstellen", "Erstinstallation ohne vorhandenes current")
	} else {
		progress.skip("Persistentes Pre-Update-Backup erstellen", "nicht angefordert")
	}

	var releaseSwap *tools.DirectorySwap
	recoverOnError := func(cause error) error {
		return failUpdateWithRecovery(console, cfg, s, tx, phase, cause, releaseSwap)
	}

	phase = "sync-current"
	if err := progress.run("Release nach current synchronisieren", func() error {
		return syncCurrent(ctx, s, false)
	}); err != nil {
		return recoverOnError(err)
	}

	phase = "verify-current"
	if err := progress.run("Installierten current-Zustand verifizieren", func() error {
		return verifyCurrent(ctx, s)
	}); err != nil {
		return recoverOnError(err)
	}

	phase = "setup-detect"
	runSetup := o.setup
	setupAvailable := false
	if !o.noSetup {
		available, detectErr := projectsetup.Available(cfg)
		if detectErr != nil {
			return recoverOnError(detectErr)
		}
		setupAvailable = available
	}
	setupConfirmedInteractively := false
	if !o.noSetup && !o.setup && setupAvailable && console.Interactive() {
		yes, confirmErr := console.Confirm("Projekt-Setup ist verfügbar. Jetzt ausführen?", false)
		if confirmErr != nil {
			return recoverOnError(confirmErr)
		}
		runSetup = yes
		setupConfirmedInteractively = yes
	}

	if setupConfirmedInteractively && console.Fullscreen() {
		// The update decision is complete. Start setup with an empty scroll area
		// while keeping the update header/info frame and the high-level footer.
		console.ClearContent()
		console.SetFooter("RUN  Projekt-Setup läuft")
	}

	phase = "setup"
	if runSetup {
		if err := progress.run("Projekt-Setup ausführen", func() error {
			_, setupErr := projectsetup.Run(ctx, cfg, console)
			return setupErr
		}); err != nil {
			return recoverOnError(err)
		}
	} else if o.noSetup {
		progress.skip("Projekt-Setup ausführen", "mit --no-setup deaktiviert")
	} else if !setupAvailable {
		progress.skip("Projekt-Setup ausführen", "kein setup.yaml/setup.sh vorhanden")
	} else {
		progress.skip("Projekt-Setup ausführen", "vom Benutzer nicht ausgewählt")
	}

	phase = "services-start"
	if tx.servicesWereRunning {
		if err := progress.run("Vorher laufende Docker-Dienste starten", func() error {
			return tx.startPreviousServiceState(ctx)
		}); err != nil {
			return recoverOnError(err)
		}
	} else {
		progress.skip("Vorher laufende Docker-Dienste starten", "vor dem Update war kein Compose-Stack aktiv")
	}

	phase = "healthcheck"
	if cfg.Healthcheck.Type != "" && cfg.Healthcheck.Type != "none" {
		if err := progress.run("Healthcheck der neuen Installation ausführen", func() error {
			return runHealthcheck(ctx, cfg)
		}); err != nil {
			return recoverOnError(err)
		}
	} else {
		progress.skip("Healthcheck der neuen Installation ausführen", "kein Healthcheck konfiguriert")
	}

	phase = "activate-release"
	if err := progress.run("Versioniertes Release aktivieren", func() error {
		if s.releaseStage == "" {
			return nil
		}
		var swapErr error
		releaseSwap, swapErr = tools.SwapDirectory(s.releaseStage, s.releaseDir)
		if swapErr == nil {
			s.releaseStage = ""
		}
		return swapErr
	}); err != nil {
		return recoverOnError(err)
	}

	phase = "metadata"
	if err := progress.run("Status schreiben und Transaktion abschließen", func() error {
		if err := writeReleaseState(s); err != nil {
			return err
		}
		if err := tx.commit(); err != nil {
			console.Warn("Transaktions-Snapshot konnte nach erfolgreichem Commit nicht entfernt werden: " + err.Error())
		}
		if releaseSwap != nil {
			if err := releaseSwap.Commit(); err != nil {
				console.Warn("Vorheriges Release-Staging konnte nach Commit nicht entfernt werden: " + err.Error())
			}
			releaseSwap = nil
		}
		return nil
	}); err != nil {
		return recoverOnError(err)
	}

	if err := writeLegacyRootMarkers(cfg, s.version.String(), sourceRef(s)); err != nil {
		console.Warn("Legacy-Release-Marker konnten nicht vollständig geschrieben werden: " + err.Error())
	}
	entry := history.Entry{Action: "update", ProjectName: cfg.ProjectName, FromVersion: s.fromVersion, ToVersion: s.version.String(), Source: sourceRef(s), Backup: backupPath, Setup: runSetup, Status: "success", Phase: "committed"}
	if err := appendHistory(cfg, entry); err != nil {
		console.ErrorNotice("Update installiert, Historie konnte nicht geschrieben werden", err.Error())
		return err
	}
	printUpdateResult(console, s, runSetup)
	return nil
}

type updateProgress struct {
	ctx     context.Context
	console *ui.Console
	enabled bool
	total   int
	current int
}

func newUpdateProgress(ctx context.Context, console *ui.Console, enabled bool, total int) *updateProgress {
	return &updateProgress{ctx: ctx, console: console, enabled: enabled, total: total}
}

func (p *updateProgress) run(label string, action func() error) error {
	index := p.current
	p.current++
	if !p.enabled {
		return action()
	}
	return p.console.ProgressStep(p.ctx, index, p.total, label, action)
}

func (p *updateProgress) skip(label, reason string) {
	index := p.current
	p.current++
	if !p.enabled {
		return
	}
	p.console.SkipProgressStep(index, p.total, label, reason)
}

func failUpdateBeforeTransaction(console *ui.Console, cfg config.Config, s *state, phase string, cause error) error {
	recorded := recordFailure(cfg, "update", phase, s.fromVersion, updateTargetVersion(s), sourceRef(s), cause)
	showUpdateFailure(console, cfg, s, phase, recorded)
	return recorded
}

func failUpdateWithRecovery(console *ui.Console, cfg config.Config, s *state, tx *transaction, phase string, cause error, releaseSwap *tools.DirectorySwap) error {
	if releaseSwap != nil {
		if swapErr := releaseSwap.Rollback(); swapErr != nil {
			cause = fmt.Errorf("%w; Release-Recovery fehlgeschlagen: %v", cause, swapErr)
		}
	}
	recorded := recordFailure(cfg, "update", phase, s.fromVersion, updateTargetVersion(s), sourceRef(s), cause)
	recovered := tx.recover(recorded)
	showUpdateFailure(console, cfg, s, phase, recovered)
	return recovered
}

func showUpdateFailure(console *ui.Console, cfg config.Config, s *state, phase string, cause error) {
	lines := []string{
		"Phase: " + updatePhaseLabel(phase) + " (" + phase + ")",
		"Projekt: " + cfg.ProjectName,
	}
	if from := strings.TrimSpace(s.fromVersion); from != "" {
		lines = append(lines, "Installiert: "+from)
	}
	if target := updateTargetVersion(s); target != "" {
		lines = append(lines, "Zielversion: "+target)
	}
	if src := strings.TrimSpace(sourceRef(s)); src != "" {
		lines = append(lines, "Quelle: "+src)
	}
	lines = append(lines, "Ursache: "+cause.Error())
	lines = append(lines, "Historie: "+cfg.HistoryFile)
	console.ErrorNotice("Update fehlgeschlagen", strings.Join(lines, "\n"))
}

func updateTargetVersion(s *state) string {
	if s == nil || strings.TrimSpace(s.artifact.VersionText) == "" {
		return ""
	}
	return s.version.String()
}

func updatePhaseLabel(phase string) string {
	labels := map[string]string{
		"source":            "Release-Quelle auflösen",
		"version-policy":    "Zielversion und Update-Regeln prüfen",
		"validate-artifact": "Release-Inhalt und Archiv validieren",
		"prepare-release":   "Versioniertes Release vorbereiten",
		"plan-current":      "Änderungen an current ermitteln",
		"transaction-begin": "Transaktion vorbereiten und Snapshot erstellen",
		"backup":            "Persistentes Pre-Update-Backup erstellen",
		"sync-current":      "Release nach current synchronisieren",
		"verify-current":    "Installierten current-Zustand verifizieren",
		"setup-detect":      "Projekt-Setup erkennen und bestätigen",
		"setup":             "Projekt-Setup ausführen",
		"services-start":    "Vorher laufende Docker-Dienste starten",
		"healthcheck":       "Healthcheck der neuen Installation ausführen",
		"activate-release":  "Versioniertes Release aktivieren",
		"metadata":          "Status schreiben und Transaktion abschließen",
	}
	if label := labels[phase]; label != "" {
		return label
	}
	return phase
}

func runRollback(ctx context.Context, console *ui.Console, cfg config.Config, o options) error {
	lock, err := tools.AcquireLock(filepath.Join(cfg.RootDir, ".release-update.lock"), "rollback")
	if err != nil {
		return err
	}
	defer lock.Release()
	rel, err := rollback.Resolve(cfg, o.rollbackVersion)
	if err != nil {
		return err
	}
	from := installedVersion(cfg.CurrentDir)
	tx, err := beginTransaction(ctx, cfg, console)
	if err != nil {
		return err
	}
	phase := "sync-current"
	res, err := rollback.Apply(ctx, cfg, rel)
	if err != nil {
		return tx.recover(recordFailure(cfg, "rollback", phase, from, rel.Version, "rollback:"+rel.Version, err))
	}
	if o.setup {
		phase = "setup"
		if _, err := projectsetup.Run(ctx, cfg, console); err != nil {
			return tx.recover(recordFailure(cfg, "rollback", phase, from, rel.Version, "rollback:"+rel.Version, err))
		}
	}
	phase = "services-start"
	if err := tx.startPreviousServiceState(ctx); err != nil {
		return tx.recover(err)
	}
	phase = "healthcheck"
	if err := runHealthcheck(ctx, cfg); err != nil {
		return tx.recover(err)
	}
	phase = "metadata"
	if err := writeRootState(cfg, rel.Version, "rollback:"+rel.Version); err != nil {
		return tx.recover(recordFailure(cfg, "rollback", phase, from, rel.Version, "rollback:"+rel.Version, err))
	}
	if err := tx.commit(); err != nil {
		console.Warn("Transaktions-Snapshot konnte nach Commit nicht entfernt werden: " + err.Error())
	}
	if err := writeLegacyRootMarkers(cfg, rel.Version, "rollback:"+rel.Version); err != nil {
		console.Warn("Legacy-Release-Marker konnten nicht vollständig geschrieben werden: " + err.Error())
	}
	if err := appendHistory(cfg, history.Entry{Action: "rollback", ProjectName: cfg.ProjectName, FromVersion: from, ToVersion: rel.Version, Setup: o.setup, Status: "success", Phase: "committed"}); err != nil {
		return err
	}
	if o.jsonOutput {
		return writeJSON(res)
	}
	printRollback(console, res, o.setup)
	return nil
}
func runRestore(ctx context.Context, console *ui.Console, cfg config.Config, o options) error {
	lock, err := tools.AcquireLock(filepath.Join(cfg.RootDir, ".release-update.lock"), "restore")
	if err != nil {
		return err
	}
	defer lock.Release()
	item, err := backup.Resolve(cfg, o.restore)
	if err != nil {
		return err
	}
	from := installedVersion(cfg.CurrentDir)
	tx, err := beginTransaction(ctx, cfg, console)
	if err != nil {
		return err
	}
	res, err := backup.Restore(ctx, cfg, item, false)
	if err != nil {
		return tx.recover(recordFailure(cfg, "restore", "sync-current", from, item.Version, item.Path, err))
	}
	to := installedVersion(cfg.CurrentDir)
	if err := tx.startPreviousServiceState(ctx); err != nil {
		return tx.recover(err)
	}
	if err := runHealthcheck(ctx, cfg); err != nil {
		return tx.recover(err)
	}
	if err := writeRootState(cfg, to, "backup:"+item.Name); err != nil {
		return tx.recover(recordFailure(cfg, "restore", "metadata", from, to, item.Path, err))
	}
	if err := tx.commit(); err != nil {
		console.Warn("Transaktions-Snapshot konnte nach Commit nicht entfernt werden: " + err.Error())
	}
	if err := writeLegacyRootMarkers(cfg, to, "backup:"+item.Name); err != nil {
		console.Warn("Legacy-Release-Marker konnten nicht vollständig geschrieben werden: " + err.Error())
	}
	if err := appendHistory(cfg, history.Entry{Action: "restore", ProjectName: cfg.ProjectName, FromVersion: from, ToVersion: to, Backup: item.Path, Status: "success", Phase: "committed"}); err != nil {
		return err
	}
	if o.jsonOutput {
		return writeJSON(map[string]any{"backup": item, "sync": res, "fromVersion": from, "toVersion": to})
	}
	printRestore(console, item, from, to, res)
	return nil
}

func resolveArtifact(ctx context.Context, s *state, explicit string) error {
	if strings.TrimSpace(explicit) != "" {
		p := explicit
		if strings.HasPrefix(p, "~/") {
			h, _ := os.UserHomeDir()
			p = filepath.Join(h, strings.TrimPrefix(p, "~/"))
		}
		a, err := filepath.Abs(p)
		if err != nil {
			return err
		}
		i, err := os.Stat(a)
		if err != nil {
			return err
		}
		if i.IsDir() {
			return fmt.Errorf("Archivpfad ist ein Ordner: %s", a)
		}
		if s.cfg.Security.MaxArchiveBytes > 0 && i.Size() > s.cfg.Security.MaxArchiveBytes {
			return fmt.Errorf("Archiv ist zu groß: %d > %d Bytes", i.Size(), s.cfg.Security.MaxArchiveBytes)
		}
		v, err := versionutil.ParseArchiveName(s.cfg.ProjectName, filepath.Base(a))
		if err != nil {
			return err
		}
		s.artifact = source.Artifact{Metadata: source.Metadata{Type: source.Download, Reference: a, Version: v, VersionText: v.String(), Size: i.Size()}, ArchivePath: a}
		s.version = v
		return nil
	}
	a, err := source.Fetch(ctx, source.Options{ProjectName: s.cfg.ProjectName, Source: s.cfg.Source, WorkDir: s.workDir, ReleaseRoot: s.cfg.ReleaseRoot, AllowHTTP: s.cfg.Security.AllowHTTP, MaxArchiveBytes: s.cfg.Security.MaxArchiveBytes})
	if err != nil {
		return err
	}
	s.artifact = a
	s.version = a.Version
	return nil
}
func prepareContent(ctx context.Context, s *state) error {
	limits := archiveLimits(s.cfg)
	if s.artifact.Type == source.Repository {
		if _, err := archive.ValidateTree(ctx, s.artifact.ContentDir, limits); err != nil {
			return err
		}
		if err := archive.ValidateVersionFile(s.artifact.ContentDir, s.version.String()); err != nil {
			return err
		}
		s.contentDir = s.artifact.ContentDir
		return nil
	}
	if err := archive.Validate(ctx, s.artifact.ArchivePath, limits); err != nil {
		return err
	}
	if err := archive.Extract(ctx, s.artifact.ArchivePath, s.extractDir, limits); err != nil {
		return err
	}
	root, err := archive.ResolveContentRoot(s.extractDir)
	if err != nil {
		return err
	}
	if _, err := archive.ValidateTree(ctx, root, limits); err != nil {
		return err
	}
	if err := archive.ValidateVersionFile(root, s.version.String()); err != nil {
		return err
	}
	s.contentDir = root
	return nil
}
func prepareRelease(ctx context.Context, s *state, simulation bool) error {
	log := filepath.Join(s.workDir, "rsync-release.log")
	if simulation {
		s.dryReleaseDir = filepath.Join(s.workDir, "dry-release")
		r, err := rsyncutil.Release(ctx, s.contentDir, s.dryReleaseDir, log)
		if err != nil {
			return err
		}
		s.releaseChanges = r.Changes
		return writeReleaseMarkers(s.dryReleaseDir, s)
	}
	if err := os.MkdirAll(s.cfg.ReleaseRoot, 0o755); err != nil {
		return err
	}
	stage := filepath.Join(s.cfg.ReleaseRoot, fmt.Sprintf(".%s.new-%d", s.version.String(), os.Getpid()))
	_ = tools.RemoveTree(stage)
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return err
	}
	r, err := rsyncutil.Release(ctx, s.contentDir, stage, log)
	if err != nil {
		_ = tools.RemoveTree(stage)
		return err
	}
	s.releaseChanges = r.Changes
	if err := writeReleaseMarkers(stage, s); err != nil {
		_ = tools.RemoveTree(stage)
		return err
	}
	s.releaseStage = stage
	return nil
}
func syncCurrent(ctx context.Context, s *state, dry bool) error {
	src := s.releaseStage
	if src == "" {
		src = s.releaseDir
	}
	dest := s.cfg.CurrentDir
	if dry {
		src = s.dryReleaseDir
		if _, err := os.Stat(dest); errors.Is(err, os.ErrNotExist) {
			s.dryCurrentDir = filepath.Join(s.workDir, "dry-current")
			dest = s.dryCurrentDir
		}
	}
	r, err := rsyncutil.Current(ctx, src, dest, filepath.Join(s.workDir, "rsync-current.log"), dry, s.cfg.Preserve)
	if err != nil {
		return err
	}
	s.currentChanges = r.Changes
	s.currentPlan = r.Items
	return nil
}
func verifyCurrent(ctx context.Context, s *state) error {
	releaseSource := s.releaseStage
	if releaseSource == "" {
		releaseSource = s.releaseDir
	}
	if err := verifyMarker(releaseSource, s.version.String()); err != nil {
		return err
	}
	if err := verifyMarker(s.cfg.CurrentDir, s.version.String()); err != nil {
		return err
	}
	r, err := rsyncutil.Current(ctx, releaseSource, s.cfg.CurrentDir, filepath.Join(s.workDir, "verify-current.log"), true, s.cfg.Preserve)
	if err != nil {
		return err
	}
	if r.Changes != 0 {
		return fmt.Errorf("Current stimmt nach Installation nicht vollständig mit Release überein: %d ungeklärte Änderungen", r.Changes)
	}
	return nil
}
func writeReleaseMarkers(dir string, s *state) error {
	for n, v := range map[string]string{".release-project": s.cfg.ProjectName, ".release-version": s.version.String(), ".release-source": sourceRef(s)} {
		if err := tools.WriteMarker(dir, n, v); err != nil {
			return err
		}
	}
	return nil
}

type rootReleaseState struct {
	ProjectName string `json:"projectName"`
	Version     string `json:"version"`
	Source      string `json:"source"`
}

func writeRootState(cfg config.Config, version, source string) error {
	data, err := json.MarshalIndent(rootReleaseState{ProjectName: cfg.ProjectName, Version: version, Source: source}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return tools.WriteFileAtomic(filepath.Join(cfg.ReleaseRoot, ".last-state.json"), data, 0o644)
}

func writeLegacyRootMarkers(cfg config.Config, version, source string) error {
	for n, v := range map[string]string{".project-name": cfg.ProjectName, ".last-version": version, ".last-source": source} {
		if err := tools.WriteMarker(cfg.ReleaseRoot, n, v); err != nil {
			return err
		}
	}
	return nil
}

func writeReleaseState(s *state) error {
	return writeRootState(s.cfg, s.version.String(), sourceRef(s))
}

func verifyMarker(dir, expected string) error {
	b, err := os.ReadFile(filepath.Join(dir, ".release-version"))
	if err != nil {
		return fmt.Errorf("Release-Marker fehlt in %s: %w", dir, err)
	}
	if strings.TrimSpace(string(b)) != expected {
		return fmt.Errorf("Release-Marker in %s ist inkonsistent", dir)
	}
	return nil
}
func enforceVersionPolicy(c config.Config, target versionutil.Version, allow, force, simulation bool) error {
	installed, _, found, err := updatecheck.DetectInstalled(c.CurrentDir)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	cmp := target.Compare(installed)
	if cmp < 0 && !allow {
		return fmt.Errorf("Downgrade wird blockiert: installiert %s, ausgewählt %s; --allow-downgrade verwenden", installed.String(), target.String())
	}
	if cmp == 0 && !force && !simulation {
		return &VersionAlreadyInstalledError{Version: installed.String()}
	}
	return nil
}
func archiveLimits(c config.Config) archive.Limits {
	return archive.Limits{MaxEntries: c.Security.MaxEntries, MaxUncompressedBytes: c.Security.MaxUncompressedBytes, MaxFileBytes: c.Security.MaxFileBytes, MaxCompressionRatio: c.Security.MaxCompressionRatio}
}
func installedVersion(dir string) string {
	v, _, found, _ := updatecheck.DetectInstalled(dir)
	if !found {
		return ""
	}
	return v.String()
}
func sourceRef(s *state) string {
	if s.artifact.Reference != "" {
		return s.artifact.Reference
	}
	return s.artifact.ArchivePath
}
func appendHistory(c config.Config, e history.Entry) error { return history.Append(c.HistoryFile, e) }
func recordFailure(c config.Config, action, phase, from, to, src string, cause error) error {
	hErr := appendHistory(c, history.Entry{Action: action, Phase: phase, ProjectName: c.ProjectName, FromVersion: from, ToVersion: to, Source: src, Status: "failed", Message: cause.Error()})
	if hErr != nil {
		return fmt.Errorf("%w; zusätzlich konnte Fehlerhistorie nicht geschrieben werden: %v", cause, hErr)
	}
	return cause
}
func writeJSON(v any) error {
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	return e.Encode(v)
}
func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

type verificationResult struct {
	ProjectName string        `json:"projectName"`
	ArchivePath string        `json:"archivePath"`
	Version     string        `json:"version"`
	ContentRoot string        `json:"contentRoot"`
	Stats       archive.Stats `json:"stats"`
	Valid       bool          `json:"valid"`
}

func verifyArchive(ctx context.Context, c config.Config, explicit string) (verificationResult, error) {
	s := &state{cfg: c}
	if err := resolveArtifact(ctx, s, explicit); err != nil {
		return verificationResult{}, err
	}
	if s.artifact.Type == source.Repository {
		return verificationResult{}, errors.New("--verify erwartet ein ZIP-Archiv")
	}
	stats, err := archive.Inspect(ctx, s.artifact.ArchivePath, archiveLimits(c))
	if err != nil {
		return verificationResult{}, err
	}
	tmp, err := os.MkdirTemp("", "update-cli-verify-*")
	if err != nil {
		return verificationResult{}, err
	}
	defer tools.RemoveTree(tmp)
	if err := archive.Extract(ctx, s.artifact.ArchivePath, tmp, archiveLimits(c)); err != nil {
		return verificationResult{}, err
	}
	root, err := archive.ResolveContentRoot(tmp)
	if err != nil {
		return verificationResult{}, err
	}
	if err := archive.ValidateVersionFile(root, s.version.String()); err != nil {
		return verificationResult{}, err
	}
	return verificationResult{ProjectName: c.ProjectName, ArchivePath: s.artifact.ArchivePath, Version: s.version.String(), ContentRoot: filepath.Base(root), Stats: stats, Valid: true}, nil
}

type updatePlanResult struct {
	ProjectName                      string `json:"projectName"`
	SourceType                       string `json:"sourceType"`
	Source                           string `json:"source"`
	FromVersion                      string `json:"fromVersion,omitempty"`
	ToVersion                        string `json:"toVersion"`
	ReleaseDir                       string `json:"releaseDir"`
	CurrentDir                       string `json:"currentDir"`
	Created, Updated, Deleted, Other []rsyncutil.Change
	Protected                        []string `json:"protected"`
}

func updatePlanJSON(s *state) updatePlanResult {
	r := updatePlanResult{ProjectName: s.cfg.ProjectName, SourceType: s.artifact.Type, Source: sourceRef(s), FromVersion: s.fromVersion, ToVersion: s.version.String(), ReleaseDir: s.releaseDir, CurrentDir: s.cfg.CurrentDir, Created: []rsyncutil.Change{}, Updated: []rsyncutil.Change{}, Deleted: []rsyncutil.Change{}, Other: []rsyncutil.Change{}, Protected: append([]string(nil), s.cfg.Preserve...)}
	for _, x := range s.currentPlan {
		switch x.Kind {
		case rsyncutil.ChangeCreated:
			r.Created = append(r.Created, x)
		case rsyncutil.ChangeUpdated:
			r.Updated = append(r.Updated, x)
		case rsyncutil.ChangeDeleted:
			r.Deleted = append(r.Deleted, x)
		default:
			r.Other = append(r.Other, x)
		}
	}
	return r
}

func initialize(console *ui.Console, root string, o options) error {
	cfg, err := config.Init(root, config.InitOptions{ProjectName: o.projectName, SourceType: o.sourceType, Folder: firstNonEmpty(o.sourceFolder, o.downloadDir), URL: o.sourceURL, Repository: o.repository, Force: o.force})
	if err != nil {
		return err
	}
	if err := templates.Ensure(cfg.TemplatesFile); err != nil {
		return err
	}
	if o.useTemplate != "" {
		if err := templates.Apply(cfg.ConfigFile, cfg.TemplatesFile, o.useTemplate); err != nil {
			return err
		}
		cfg, err = config.Load(root, "")
		if err != nil {
			return err
		}
	}
	console.Header("Update CLI initialisiert")
	console.Row("Projekt", cfg.ProjectName)
	console.Row("Konfiguration", cfg.ConfigFile)
	console.Row("Quelle", cfg.Source.Type)
	console.Row("Release", cfg.ReleaseRoot)
	console.Row("Current", cfg.CurrentDir)
	console.Row("Backup", cfg.BackupRoot)
	return nil
}
func runConfig(ctx context.Context, console *ui.Console, root string, o options) error {
	cfg, err := config.Load(root, "")
	if err != nil {
		return err
	}
	if o.configList {
		console.Header("Konfigurationsdateien")
		for _, p := range []string{cfg.ConfigFile, cfg.TemplatesFile, cfg.HistoryFile} {
			console.Row(filepath.Base(p), p)
		}
		return nil
	}
	if o.useTemplate != "" {
		if err := templates.Ensure(cfg.TemplatesFile); err != nil {
			return err
		}
		if err := templates.Apply(cfg.ConfigFile, cfg.TemplatesFile, o.useTemplate); err != nil {
			return err
		}
		if _, err := config.Load(root, ""); err != nil {
			return err
		}
		console.Success("Template angewendet")
		if !o.edit {
			return nil
		}
	}
	if o.edit {
		used, err := editor.Open(ctx, cfg.ConfigFile)
		if err != nil {
			return err
		}
		if _, err := config.Load(root, ""); err != nil {
			return fmt.Errorf("Editor %s geschlossen, aber config ungültig: %w", used, err)
		}
		console.Success("config.json ist gültig")
		return nil
	}
	path, text, err := config.Format(root)
	if err != nil {
		return err
	}
	console.Header("Updater-Konfiguration")
	console.Row("Datei", path)
	fmt.Print("\n" + text)
	return nil
}
func runTemplates(ctx context.Context, console *ui.Console, root string, o options, buildVersion string) error {
	cfg, err := config.Load(root, "")
	if err != nil {
		return err
	}
	if err := templates.Ensure(cfg.TemplatesFile); err != nil {
		return err
	}
	if o.templatesList {
		f, err := templates.Load(cfg.TemplatesFile)
		if err != nil {
			return err
		}
		console.Header("Updater-Templates")
		for _, t := range templates.Sorted(f) {
			console.Row(t.Name, t.Description)
			if o.details {
				if len(t.NoParameter) > 0 {
					console.Row("  no parameter", strings.Join(t.NoParameter, " + "))
				}
				if len(t.Preserve) > 0 {
					console.Row("  preserve", strings.Join(t.Preserve, ", "))
				}
			}
		}
		return nil
	}
	if o.templateUse != "" {
		if err := templates.Apply(cfg.ConfigFile, cfg.TemplatesFile, o.templateUse); err != nil {
			return err
		}
		if _, err := config.Load(root, ""); err != nil {
			return err
		}
		console.Success("Template angewendet")
		return nil
	}
	if o.edit {
		used, err := editor.Open(ctx, cfg.TemplatesFile)
		if err != nil {
			return err
		}
		if _, err := templates.Load(cfg.TemplatesFile); err != nil {
			return fmt.Errorf("Editor %s geschlossen, aber templates.json ungültig: %w", used, err)
		}
		console.Success("templates.json ist gültig")
		return nil
	}
	printHelp(buildVersion)
	return nil
}

func printSetupCatalog(console *ui.Console, catalog projectsetup.Catalog) {
	console.Header("Setup-Katalog")
	if catalog.Project != "" {
		console.Row("Projekt", catalog.Project)
	}
	if len(catalog.Workflows) > 0 {
		console.Append("Workflows")
		for _, workflow := range catalog.Workflows {
			detail := strings.Join(workflow.Tasks, ", ")
			if workflow.Description != "" {
				detail += " — " + workflow.Description
			}
			console.Append(fmt.Sprintf("  %-16s %s", workflow.Name, detail))
		}
	}
	if len(catalog.Tasks) > 0 {
		console.Append("Tasks")
		for _, task := range catalog.Tasks {
			detail := fmt.Sprintf("%d Schritte", task.Steps)
			if len(task.Requires) > 0 {
				detail += " | requires: " + strings.Join(task.Requires, ", ")
			}
			if task.Description != "" {
				detail += " — " + task.Description
			}
			console.Append(fmt.Sprintf("  %-16s %s", task.Name, detail))
		}
	}
}

func setupManagementDirectory(root string) (string, error) {
	configFile := filepath.Join(root, config.ConfigDirName, config.ConfigFileName)
	if _, err := os.Stat(configFile); err == nil {
		cfg, loadErr := config.Load(root, "")
		if loadErr != nil {
			return "", loadErr
		}
		return cfg.CurrentDir, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return root, nil
}
