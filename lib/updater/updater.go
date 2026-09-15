package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/r14r/update-cli/lib/archive"
	"github.com/r14r/update-cli/lib/backup"
	"github.com/r14r/update-cli/lib/cleanup"
	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/discovery"
	"github.com/r14r/update-cli/lib/doctor"
	"github.com/r14r/update-cli/lib/editor"
	"github.com/r14r/update-cli/lib/effectiveconfig"
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
		return runNoParameter(ctx, buildVersion, false)
	}
	o, err := parseOptions(args)
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}
	if o.showHelp {
		if o.jsonOutput {
			b, marshalErr := discovery.Marshal(buildVersion)
			if marshalErr != nil {
				return marshalErr
			}
			_, marshalErr = os.Stdout.Write(append(b, '\n'))
			return marshalErr
		}
		printCommandHelp(buildVersion, o.helpTopic, o.details)
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
	if o.noParameterInvocation {
		return runNoParameter(ctx, buildVersion, o.debug)
	}
	if o.schema {
		if o.schemaVersion {
			fmt.Printf("%d\n", projectsetup.SchemaVersion)
			return nil
		}
		if o.schemaView {
			data, schemaErr := projectsetup.SchemaJSON()
			if schemaErr != nil {
				return schemaErr
			}
			_, schemaErr = os.Stdout.Write(data)
			return schemaErr
		}
		path, schemaErr := projectsetup.SaveSchema(o.schemaSave)
		if schemaErr != nil {
			return schemaErr
		}
		fmt.Printf("Schema gespeichert: %s\n", path)
		return nil
	}
	console := ui.New(o.noColor || o.jsonOutput)
	console.SetApplicationVersion(buildVersion)
	console.SuppressFinalStatus(o.jsonOutput || o.run)
	defer console.PrintFinalStatus()
	console.SetDirect(o.noUI || o.debug)
	console.SetDetails(o.details || o.noUI || o.debug)
	if o.debug {
		console.Info("Debug-Modus aktiv: direkte Ausgabe mit Details")
	}
	fullscreenTitle := ""
	fullscreenBase := fmt.Sprintf("Update CLI Version %s", buildVersion)
	switch {
	case o.setupManifest != "" || (o.setup && !o.update) || o.setupList || o.setupTask != "" || o.setupWorkflow != "":
		fullscreenTitle = fullscreenBase + " — Setup"
	case o.check:
		fullscreenTitle = fullscreenBase + " — Versionsprüfung"
	case o.update && !o.plan && !o.dryRun:
		fullscreenTitle = fullscreenBase + " — Update"
	}
	fullscreen := false
	if fullscreenTitle != "" && !o.jsonOutput {
		fullscreen = console.StartFullscreen(fullscreenTitle)
		if fullscreen {
			switch {
			case strings.HasSuffix(fullscreenTitle, "— Setup"):
				console.SetFooter("RUN  Projekt-Setup läuft")
			case strings.HasSuffix(fullscreenTitle, "— Versionsprüfung"):
				console.SetFooter("RUN  Versionsprüfung läuft")
			case strings.HasSuffix(fullscreenTitle, "— Update"):
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
		inspection := projectsetup.InspectManifest(o.setupManifest)
		console.SetMigrationRequired(inspection.Repairable || (inspection.SchemaVersion > 0 && inspection.SchemaVersion != inspection.CurrentSchema))
		catalog, err := projectsetup.CatalogForManifest(o.setupManifest)
		if err != nil {
			return err
		}
		console.SetProjectName(catalog.Project)
		console.SetProjectVersion(installedVersion(filepath.Dir(o.setupManifest)))
		if o.setupList {
			if o.jsonOutput {
				return writeJSON(catalog)
			}
			printSetupCatalog(console, catalog)
			return nil
		}
		_, err = projectsetup.RunStandaloneSelected(ctx, o.setupManifest, console, projectsetup.Selection{Workflow: o.setupWorkflow, Task: o.setupTask, Step: o.setupStep})
		return err
	}
	var root string
	if o.init {
		root, err = resolveInitRoot(o.rootDir, o.projectName)
	} else if (o.doctor || o.fix) && strings.TrimSpace(o.rootDir) == "" {
		root, err = os.Getwd()
		if err == nil {
			root, err = filepath.Abs(root)
		}
	} else {
		root, err = config.ResolveRoot(o.rootDir)
	}
	if err != nil {
		return err
	}
	console.SetMigrationRequired(projectMigrationRequired(root))
	if o.debug {
		console.Header("Debug")
		console.Row("Projektwurzel", root)
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
				return fmt.Errorf("kein update-cli.yaml/setup.yaml in %s", targetDir)
			}
			if o.dryRun {
				text, previous, previewErr := projectsetup.PreviewConvertManifest(manifest)
				if previewErr != nil {
					return previewErr
				}
				console.Header("update-cli.yaml Konvertierung — Dry-Run")
				console.Row("Datei", manifest)
				console.Row("Schema", fmt.Sprintf("%d → 2", previous))
				fmt.Print(text)
				return nil
			}
			res, convertErr := projectsetup.ConvertManifestToLatest(manifest, o.force)
			if convertErr != nil {
				return convertErr
			}
			console.Header("update-cli.yaml konvertiert")
			console.Row("Datei", res.Path)
			console.Row("Schema", fmt.Sprintf("%d → %d", res.PreviousSchema, res.CurrentSchema))
			if res.BackupPath != "" {
				console.Row("Backup", res.BackupPath)
			}
			if res.Changed {
				console.Success("update-cli.yaml wurde auf das aktuelle Schema migriert")
			} else {
				console.Success("update-cli.yaml verwendet bereits das aktuelle Schema")
			}
			return nil
		case o.createYAML:
			from := strings.ToLower(strings.TrimSpace(o.sourceType))
			if from == "" {
				from = "project"
			}
			if from == "project" {
				if o.dryRun {
					text, tech, previewErr := projectsetup.PreviewGeneratedManifest(targetDir)
					if previewErr != nil {
						return previewErr
					}
					console.Header("update-cli.yaml Generator — Dry-Run")
					console.Row("Quelle", "project")
					console.Row("Projektordner", targetDir)
					console.Row("Erkannt", strings.Join(tech, ", "))
					fmt.Print(text)
					return nil
				}
				res, createErr := projectsetup.GenerateManifest(targetDir, "", o.force)
				if createErr != nil {
					return createErr
				}
				console.Header("update-cli.yaml erstellt")
				console.Row("Quelle", "project")
				console.Row("Datei", res.Path)
				console.Row("Erkannt", strings.Join(res.Technologies, ", "))
				if res.Overwritten {
					console.Warn("Vorhandenes update-cli.yaml wurde mit --force ersetzt")
				}
				console.Success("SchemaVersion 2 Manifest wurde erzeugt; vor produktivem Einsatz prüfen")
				return nil
			}

			scriptPath := filepath.Join(targetDir, "setup.sh")
			targetManifest := filepath.Join(targetDir, "update-cli.yaml")
			if !o.dryRun && !o.force {
				if info, statErr := os.Stat(targetManifest); statErr == nil && !info.IsDir() {
					return fmt.Errorf("update-cli.yaml existiert bereits: %s; --force verwenden", targetManifest)
				} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
					return statErr
				}
			}
			draft, tech, analysis, previewErr := projectsetup.PreviewGeneratedManifestFromSetupScript(targetDir, scriptPath)
			if previewErr != nil {
				return previewErr
			}
			finalText := draft
			var aiResult projectsetup.AIRefineResult
			if o.withAI {
				console.Info("Deterministische setup.sh-Konvertierung abgeschlossen; AI-Verfeinerung starten")
				refined, ai, aiErr := projectsetup.RefineSetupYAMLWithAI(ctx, targetDir, scriptPath, draft)
				if aiErr != nil {
					return fmt.Errorf("AI-Konvertierung fehlgeschlagen: %w", aiErr)
				}
				finalText = refined
				aiResult = ai
			}
			if o.dryRun {
				console.Header("update-cli.yaml Generator — Dry-Run")
				console.Row("Quelle", "setup-script")
				console.Row("Setup-Script", scriptPath)
				console.Row("Erkannte Schritte", fmt.Sprintf("%d", analysis.Steps))
				console.Row("Erkannt", strings.Join(tech, ", "))
				if o.withAI {
					console.Row("AI", aiResult.Provider+" / "+aiResult.Model)
				}
				fmt.Print(finalText)
				return nil
			}
			res, analysis, createErr := projectsetup.GenerateManifestFromSetupScript(targetDir, scriptPath, "", o.force, finalText)
			if createErr != nil {
				return createErr
			}
			console.Header("update-cli.yaml erstellt")
			console.Row("Quelle", "setup-script")
			console.Row("Setup-Script", analysis.Path)
			console.Row("Erkannte Schritte", fmt.Sprintf("%d", analysis.Steps))
			console.Row("Datei", res.Path)
			console.Row("Erkannt", strings.Join(res.Technologies, ", "))
			if o.withAI {
				console.Row("AI", aiResult.Provider+" / "+aiResult.Model)
				if aiResult.PromptPath != "" {
					console.Row("Prompt", aiResult.PromptPath)
				}
			}
			if res.Overwritten {
				console.Warn("Vorhandenes update-cli.yaml wurde mit --force ersetzt")
			}
			console.Success("setup.sh wurde in ein validiertes SchemaVersion-2-Manifest konvertiert")
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
	standaloneSetupCommand := (o.setup && !o.update && !o.rollback) || o.setupList || o.setupTask != "" || o.setupWorkflow != "" || o.setupStep != ""
	if standaloneSetupCommand {
		selection := projectsetup.Selection{Workflow: o.setupWorkflow, Task: o.setupTask, Step: o.setupStep}
		hasState := false
		var statErr error
		stateDir := filepath.Join(root, config.ConfigDirName)
		if _, err := os.Stat(stateDir); err == nil {
			hasState = true
		} else if !errors.Is(err, os.ErrNotExist) {
			statErr = err
		}
		if hasState {
			cfg, _, loadErr := effectiveconfig.Load(root)
			if loadErr != nil {
				return loadErr
			}
			console.SetProjectName(cfg.ProjectName)
			console.SetProjectVersion(installedVersion(cfg.CurrentDir))

			currentAvailable, availableErr := projectsetup.Available(cfg)
			if availableErr != nil {
				return availableErr
			}
			rootManifest, rootManifestOK, rootManifestErr := projectsetup.FindManifest(root)
			if rootManifestErr != nil {
				return rootManifestErr
			}
			if !currentAvailable && rootManifestOK {
				console.SetProjectVersion(installedVersion(root))
				if o.setupList {
					catalog, catalogErr := projectsetup.CatalogForManifest(rootManifest)
					if catalogErr != nil {
						return catalogErr
					}
					console.SetProjectName(catalog.Project)
					if o.jsonOutput {
						return writeJSON(catalog)
					}
					printSetupCatalog(console, catalog)
					return nil
				}
				_, setupErr := projectsetup.RunStandaloneSelected(ctx, rootManifest, console, selection)
				if setupErr != nil {
					return setupErr
				}
				return syncProjectVersionFromCurrent(cfg)
			}

			if o.setup && !o.setupList && o.setupTask == "" && o.setupWorkflow == "" && o.setupStep == "" {
				_, setupErr := projectsetup.Run(ctx, cfg, console)
				if setupErr != nil {
					return setupErr
				}
				return syncProjectVersionFromCurrent(cfg)
			}
			manifest := projectsetup.ManifestPath(cfg)
			if manifest == "" {
				if rootManifestOK {
					manifest = rootManifest
				} else {
					return fmt.Errorf("kein update-cli.yaml/setup.yaml in %s", cfg.CurrentDir)
				}
			}
			if o.setupList {
				catalog, catalogErr := projectsetup.CatalogForManifest(manifest)
				if catalogErr != nil {
					return catalogErr
				}
				console.SetProjectName(catalog.Project)
				if o.jsonOutput {
					return writeJSON(catalog)
				}
				printSetupCatalog(console, catalog)
				return nil
			}
			_, setupErr := projectsetup.RunSelected(ctx, cfg, console, selection)
			if setupErr != nil {
				return setupErr
			}
			return syncProjectVersionFromCurrent(cfg)
		}
		if statErr != nil {
			return statErr
		}
		manifest, ok, findErr := projectsetup.FindManifest(root)
		if findErr != nil {
			return findErr
		}
		if !ok {
			if o.setup && !o.setupList && o.setupTask == "" && o.setupWorkflow == "" && o.setupStep == "" {
				cfg := config.Config{ProjectName: filepath.Base(root), CurrentDir: root}
				console.SetProjectName(cfg.ProjectName)
				console.SetProjectVersion(installedVersion(cfg.CurrentDir))
				_, setupErr := projectsetup.Run(ctx, cfg, console)
				return setupErr
			}
			return fmt.Errorf("kein update-cli.yaml/setup.yaml im aktuellen Ordner %s", root)
		}
		if o.setupList {
			catalog, catalogErr := projectsetup.CatalogForManifest(manifest)
			if catalogErr != nil {
				return catalogErr
			}
			console.SetProjectName(catalog.Project)
			console.SetProjectVersion(installedVersion(filepath.Dir(manifest)))
			if o.jsonOutput {
				return writeJSON(catalog)
			}
			printSetupCatalog(console, catalog)
			return nil
		}
		catalog, catalogErr := projectsetup.CatalogForManifest(manifest)
		if catalogErr != nil {
			return catalogErr
		}
		console.SetProjectName(catalog.Project)
		console.SetProjectVersion(installedVersion(filepath.Dir(manifest)))
		_, setupErr := projectsetup.RunStandaloneSelected(ctx, manifest, console, selection)
		return setupErr
	}
	if o.fix {
		lock, lockErr := tools.AcquireLock(filepath.Join(root, ".release-update.lock"), "fix")
		if lockErr != nil {
			return lockErr
		}
		defer lock.Release()
		configResult, fixErr := config.RepairProjectConfig(root)
		if fixErr != nil {
			return fixErr
		}
		baseConfig, loadErr := config.Load(root, "")
		if loadErr != nil {
			return fmt.Errorf("reparierte config.json kann nicht geladen werden: %w", loadErr)
		}
		manifestResults, fixErr := repairProjectManifests(root, baseConfig.CurrentDir)
		if fixErr != nil {
			return fixErr
		}
		result := fixResult{Root: root, Config: configResult, Manifests: manifestResults}
		if len(manifestResults) > 0 {
			result.Manifest = manifestResults[0]
		}
		if o.jsonOutput {
			return writeJSON(result)
		}
		printFix(console, result)
		return nil
	}
	if o.doctor {
		if o.doctorFix {
			res := doctor.RunProject(root, false)
			printDoctor(console, res)
			plan, planErr := buildDoctorFixPlan(root, res)
			if planErr != nil {
				return planErr
			}
			printDoctorFixPlan(console, plan)
			if len(plan.PlannedChanges) == 0 {
				if res.ErrorCount() > 0 {
					return &ExitError{Code: 1}
				}
				return nil
			}
			ok, confirmErr := confirmDoctorFix(console)
			if confirmErr != nil {
				return confirmErr
			}
			if !ok {
				console.Info("Doctor-Fix abgebrochen; es wurden keine Dateien geändert")
				return nil
			}
			lock, lockErr := tools.AcquireLock(filepath.Join(root, ".release-update.lock"), "doctor-fix")
			if lockErr != nil {
				return lockErr
			}
			defer lock.Release()
			if fixErr := applyDoctorFix(root, plan); fixErr != nil {
				return fixErr
			}
			console.Success("Doctor-Fix wurde durchgeführt")
			after := doctor.RunProject(root, false)
			printDoctor(console, after)
			if after.ErrorCount() > 0 {
				return &ExitError{Code: 1}
			}
			return nil
		}
		res := doctor.RunProject(root, o.doctorMigrate)
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
	}
	if o.unlock {
		return tools.UnlockStale(filepath.Join(root, ".release-update.lock"))
	}
	if o.init {
		return initialize(ctx, console, root, o)
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
	if o.install {
		installErr := projectsetup.RunJustInstall(ctx, root, console)
		if installErr == nil {
			return nil
		}
		var exitErr *exec.ExitError
		if errors.As(installErr, &exitErr) {
			return &ExitError{Code: exitErr.ExitCode(), Err: installErr}
		}
		return installErr
	}
	if o.run {
		stateDir := filepath.Join(root, config.ConfigDirName)
		if _, statErr := os.Stat(stateDir); errors.Is(statErr, os.ErrNotExist) {
			runErr := projectsetup.RunApplicationInDirectory(ctx, root, console)
			if runErr == nil {
				return nil
			}
			var exitErr *exec.ExitError
			if errors.As(runErr, &exitErr) {
				return &ExitError{Code: exitErr.ExitCode(), Err: runErr}
			}
			return runErr
		} else if statErr != nil {
			return statErr
		}
	}
	cfg, _, err := effectiveconfig.Load(root)
	if err != nil {
		return err
	}
	console.SetProjectName(cfg.ProjectName)
	console.SetProjectVersion(installedVersion(cfg.CurrentDir))
	cfg, err = config.WithSourceOverrides(cfg, o.mode, o.sourceType, firstNonEmpty(o.sourceFolder, o.downloadDir), o.sourceURL, o.repository)
	if err != nil {
		return err
	}
	if o.debug {
		debugEffectiveConfig(console, cfg)
	}
	if o.update && cfg.Mode == config.ModePull && console.Fullscreen() {
		console.StartFullscreen(fullscreenBase + " — Pull")
		console.SetFooter("RUN  Git Pull läuft")
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
	case o.clean:
		lock, err := tools.AcquireLock(filepath.Join(root, ".release-update.lock"), "clean")
		if err != nil {
			return err
		}
		defer lock.Release()
		res, err := cleanup.RunReleases(cfg, o.keep, o.plan)
		if err != nil {
			return err
		}
		if !o.plan {
			appendSuccessHistory(console, cfg, history.Entry{Action: "clean", ProjectName: cfg.ProjectName, Status: "success", Message: fmt.Sprintf("%d obsolete Releases entfernt", len(res.RemovedRelease))})
		}
		if o.jsonOutput {
			return writeJSON(res)
		}
		printCleanup(console, res)
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
			appendSuccessHistory(console, cfg, history.Entry{Action: "cleanup", ProjectName: cfg.ProjectName, Status: "success", Message: fmt.Sprintf("%d Releases und %d Backups entfernt", len(res.RemovedRelease), len(res.RemovedBackup))})
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
			yes, confirmErr := console.Confirm("Update jetzt installieren?", true)
			if confirmErr != nil {
				return confirmErr
			}
			if yes {
				console.StartFullscreen(fullscreenBase + " — Update")
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
		setCheckFinalStatus(console, res)
		return nil
	case o.run:
		runErr := projectsetup.RunApplication(ctx, cfg, console)
		if runErr == nil {
			return nil
		}
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return &ExitError{Code: exitErr.ExitCode(), Err: runErr}
		}
		return runErr
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
		if err != nil {
			return err
		}
		return syncProjectVersionFromCurrent(cfg)
	case o.update:
		return runUpdate(ctx, console, cfg, o)
	}
	return errors.New("unbekannte Betriebsart")
}

func projectMigrationRequired(root string) bool {
	return doctor.InspectMigrationRequirement(root).Required
}

func runNoParameter(ctx context.Context, buildVersion string, debug bool) error {
	root, err := config.ResolveRoot("")
	if err != nil {
		printHelp(buildVersion)
		return nil
	}
	cfg, repair, err := config.LoadResilient(root, "")
	if err != nil {
		stateDir := filepath.Join(root, config.ConfigDirName)
		projectFile := filepath.Join(root, config.ProjectFileName)
		_, stateErr := os.Stat(stateDir)
		_, projectErr := os.Stat(projectFile)
		if errors.Is(stateErr, os.ErrNotExist) && errors.Is(projectErr, os.ErrNotExist) {
			printHelp(buildVersion)
			return nil
		}
		return err
	}
	if repair != nil && (repair.Local.Changed || (repair.Global != nil && repair.Global.Changed)) {
		fmt.Fprintln(os.Stderr, "WARN  config.json wurde automatisch auf den aktuellen Standard migriert; Backup wurde erstellt")
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
		if actions["no-setup"] {
			args = append(args, "--no-setup")
		} else if actions["setup"] {
			args = append(args, "--setup")
		}
	case actions["setup"]:
		args = append(args, "--setup")
	default:
		printHelp(buildVersion)
		return nil
	}
	args = append(args, "--root", root)
	if debug {
		args = append(args, "--debug")
	}
	return Run(ctx, buildVersion, args)
}
func repairProjectManifests(root, currentDir string) ([]projectsetup.ProjectRepairResult, error) {
	dirs := []string{root}
	if currentDir != "" {
		if absCurrent, err := filepath.Abs(currentDir); err == nil && filepath.Clean(absCurrent) != filepath.Clean(root) {
			dirs = append(dirs, absCurrent)
		}
	}
	seen := map[string]bool{}
	results := []projectsetup.ProjectRepairResult{}
	for _, dir := range dirs {
		dir = filepath.Clean(dir)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		hasManifest := false
		for _, name := range []string{config.ProjectFileName, "setup.yaml"} {
			if info, err := os.Stat(filepath.Join(dir, name)); err == nil && !info.IsDir() {
				hasManifest = true
				break
			} else if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
		}
		if !hasManifest {
			continue
		}
		result, err := projectsetup.RepairProjectManifest(dir)
		if err != nil {
			return nil, fmt.Errorf("Manifest %s kann nicht repariert werden: %w", filepath.Join(dir, config.ProjectFileName), err)
		}
		results = append(results, result)
	}
	return results, nil
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
	appendSuccessHistory(console, cfg, history.Entry{Action: "backup", ProjectName: cfg.ProjectName, FromVersion: res.Backup.Version, Backup: res.Backup.Path, Status: "success"})
	if jsonOut {
		return writeJSON(res)
	}
	printBackup(console, res)
	return nil
}

func runUpdate(ctx context.Context, console *ui.Console, cfg config.Config, o options) (retErr error) {
	if cfg.Mode == config.ModePull && strings.TrimSpace(o.archive) != "" {
		return errors.New("mode pull verwendet ein Git-Repository und akzeptiert kein ZIP-Archiv; für ZIP-Dateien --mode update verwenden")
	}
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
	sourceLabel := "ZIP-Release auflösen"
	if cfg.Mode == config.ModePull {
		sourceLabel = "Git-Repository aktualisieren"
	}
	if err := progress.run(sourceLabel, func() error {
		return resolveArtifact(ctx, s, o.archive)
	}); err != nil {
		return failUpdateBeforeTransaction(console, cfg, s, phase, err)
	}

	phase = "version-policy"
	var alreadyInstalled *VersionAlreadyInstalledError
	if err := progress.run("Zielversion und Update-Regeln prüfen", func() error {
		policyErr := enforceVersionPolicy(cfg, s.artifact, o.allowDowngrade, o.force, o.plan || o.dryRun)
		if errors.As(policyErr, &alreadyInstalled) {
			// Selecting the currently installed version is a successful no-op, not
			// a failed update phase. --force still bypasses this branch and performs
			// the existing reinstall path.
			return nil
		}
		return policyErr
	}); err != nil {
		return failUpdateBeforeTransaction(console, cfg, s, phase, err)
	}
	if alreadyInstalled != nil {
		return finishAlreadyInstalledUpdate(console, cfg, s, alreadyInstalled)
	}

	s.releaseDir = filepath.Join(cfg.ReleaseRoot, s.version.String())
	if !o.jsonOutput {
		printUpdatePlan(console, s, o)
	}

	phase = "validate-artifact"
	if err := progress.run("Release-Inhalt validieren", func() error {
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
	if err := progress.run("Transaktions-Rollback vorbereiten", func() error {
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
	setupDeclinedInteractively := false
	if !o.noSetup && !o.setup && setupAvailable && console.Interactive() {
		yes, confirmErr := console.Confirm("Projekt-Setup ist verfügbar. Jetzt ausführen?", true)
		if confirmErr != nil {
			return recoverOnError(confirmErr)
		}
		runSetup = yes
		setupConfirmedInteractively = yes
		setupDeclinedInteractively = !yes
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
			if cfg.KeepRsyncOnSetupError {
				return failSetupKeepingSynced(console, cfg, s, tx, phase, err)
			}
			return recoverOnError(err)
		}
	} else if o.noSetup {
		progress.skip("Projekt-Setup ausführen", "mit --no-setup deaktiviert")
	} else if !setupAvailable {
		progress.skip("Projekt-Setup ausführen", "kein update-cli.yaml/setup.sh vorhanden")
	} else {
		progress.skip("Projekt-Setup ausführen", "vom Benutzer nicht ausgewählt")
	}

	phase = "services-start"
	if err := runPostSetupActivation(ctx, cfg, tx, progress, setupDeclinedInteractively, recoverOnError); err != nil {
		return err
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
			console.Warn("Temporäre Transaktionsdaten konnten nach erfolgreichem Commit nicht entfernt werden: " + err.Error())
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
	appendSuccessHistory(console, cfg, entry)
	console.SetProjectVersion(s.version.String())
	printUpdateResult(console, s, runSetup)
	return nil
}

func runPostSetupActivation(ctx context.Context, cfg config.Config, tx *transaction, progress *updateProgress, setupDeclinedInteractively bool, recoverOnError func(error) error) error {
	if setupDeclinedInteractively {
		progress.skip("Vorher laufende Docker-Dienste starten", "Setup vom Benutzer nicht ausgewählt; Dienste bleiben für die manuelle Einrichtung gestoppt")
		progress.skip("Healthcheck der neuen Installation ausführen", "Setup vom Benutzer nicht ausgewählt; Aktivierung wird manuell abgeschlossen")
		return nil
	}

	if tx.servicesWereRunning {
		if err := progress.run("Vorher laufende Docker-Dienste starten", func() error {
			return tx.startPreviousServiceState(ctx)
		}); err != nil {
			return recoverOnError(err)
		}
	} else {
		reason := tx.dockerSkipReason
		if reason == "" {
			reason = "vor dem Update war kein Compose-Stack aktiv"
		}
		progress.skip("Vorher laufende Docker-Dienste starten", reason)
	}

	if cfg.Healthcheck.Type != "" && cfg.Healthcheck.Type != "none" {
		if err := progress.run("Healthcheck der neuen Installation ausführen", func() error {
			return runHealthcheck(ctx, cfg)
		}); err != nil {
			return recoverOnError(err)
		}
	} else {
		progress.skip("Healthcheck der neuen Installation ausführen", "kein Healthcheck konfiguriert")
	}
	return nil
}

func finishAlreadyInstalledUpdate(console *ui.Console, cfg config.Config, s *state, same *VersionAlreadyInstalledError) error {
	version := strings.TrimSpace(same.Version)
	if version == "" {
		version = s.version.String()
	}
	if err := writeProjectVersion(cfg, version); err != nil {
		return fmt.Errorf("Projekt-VERSION konnte nicht mit current synchronisiert werden: %w", err)
	}
	console.SuccessBanner(fmt.Sprintf("Version %s ist bereits installiert", version))
	console.SetFinishFooter("Update beenden")
	console.SetFinalStatus(cfg.ProjectName, "Installierte Version: v"+version)
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

// failSetupKeepingSynced records a setup failure while intentionally keeping
// the already verified rsync result in current/. The staged release is
// activated as the matching versioned release so release/ and current/ stay
// consistent. The command still returns an error and the history entry remains
// failed; only file-state rollback is suppressed for the setup phase.
func failSetupKeepingSynced(console *ui.Console, cfg config.Config, s *state, tx *transaction, phase string, cause error) error {
	var releaseSwap *tools.DirectorySwap
	if s.releaseStage != "" {
		var err error
		releaseSwap, err = tools.SwapDirectory(s.releaseStage, s.releaseDir)
		if err != nil {
			return failUpdateWithRecovery(console, cfg, s, tx, phase, fmt.Errorf("%w; synchronisierter Stand konnte nicht als Release aktiviert werden: %v", cause, err), releaseSwap)
		}
		s.releaseStage = ""
	}
	if err := writeReleaseState(s); err != nil {
		return failUpdateWithRecovery(console, cfg, s, tx, phase, fmt.Errorf("%w; Release-Status konnte für den beibehaltenen Stand nicht geschrieben werden: %v", cause, err), releaseSwap)
	}
	if err := tx.commit(); err != nil {
		console.Warn("Temporäre Transaktionsdaten konnten nach beibehaltenem Setup-Fehler nicht vollständig entfernt werden: " + err.Error())
	}
	if releaseSwap != nil {
		if err := releaseSwap.Commit(); err != nil {
			console.Warn("Vorheriges Release-Staging konnte nicht vollständig entfernt werden: " + err.Error())
		}
	}
	if err := writeLegacyRootMarkers(cfg, s.version.String(), sourceRef(s)); err != nil {
		console.Warn("Legacy-Release-Marker konnten für den beibehaltenen Stand nicht vollständig geschrieben werden: " + err.Error())
	}
	recorded := recordFailure(cfg, "update", phase, s.fromVersion, updateTargetVersion(s), sourceRef(s), cause)
	console.Warn("Projekt-Setup fehlgeschlagen; rsync-Stand bleibt wegen setup.keepRsyncOnError=true in current/ erhalten")
	showUpdateFailure(console, cfg, s, phase, recorded)
	return recorded
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
		"validate-artifact": "Release-Inhalt validieren",
		"prepare-release":   "Versioniertes Release vorbereiten",
		"plan-current":      "Änderungen an current ermitteln",
		"transaction-begin": "Transaktions-Rollback vorbereiten",
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
		console.Warn("Temporäre Transaktionsdaten konnten nach Commit nicht entfernt werden: " + err.Error())
	}
	if err := writeLegacyRootMarkers(cfg, rel.Version, "rollback:"+rel.Version); err != nil {
		console.Warn("Legacy-Release-Marker konnten nicht vollständig geschrieben werden: " + err.Error())
	}
	appendSuccessHistory(console, cfg, history.Entry{Action: "rollback", ProjectName: cfg.ProjectName, FromVersion: from, ToVersion: rel.Version, Setup: o.setup, Status: "success", Phase: "committed"})
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
		console.Warn("Temporäre Transaktionsdaten konnten nach Commit nicht entfernt werden: " + err.Error())
	}
	if err := writeLegacyRootMarkers(cfg, to, "backup:"+item.Name); err != nil {
		console.Warn("Legacy-Release-Marker konnten nicht vollständig geschrieben werden: " + err.Error())
	}
	appendSuccessHistory(console, cfg, history.Entry{Action: "restore", ProjectName: cfg.ProjectName, FromVersion: from, ToVersion: to, Backup: item.Path, Status: "success", Phase: "committed"})
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
	a, err := source.Fetch(ctx, source.Options{ProjectName: s.cfg.ProjectName, Mode: s.cfg.Mode, Source: s.cfg.Source, WorkDir: s.workDir, ReleaseRoot: s.cfg.ReleaseRoot, RepositoryCacheDir: s.cfg.RepositoryCacheDir, AllowHTTP: s.cfg.Security.AllowHTTP, MaxArchiveBytes: s.cfg.Security.MaxArchiveBytes})
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
	stage, err := os.MkdirTemp(s.cfg.ReleaseRoot, "."+s.version.String()+".new-")
	if err != nil {
		return fmt.Errorf("Release-Staging kann nicht erstellt werden: %w", err)
	}
	if err := os.Chmod(stage, 0o755); err != nil {
		_ = tools.RemoveTree(stage)
		return fmt.Errorf("Release-Staging kann nicht vorbereitet werden: %w", err)
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
	if commit := strings.TrimSpace(s.artifact.Commit); commit != "" {
		b, err := os.ReadFile(filepath.Join(s.cfg.CurrentDir, ".release-commit"))
		if err != nil {
			return fmt.Errorf("Release-Commit-Marker fehlt in %s: %w", s.cfg.CurrentDir, err)
		}
		if strings.TrimSpace(string(b)) != commit {
			return fmt.Errorf("Release-Commit-Marker in %s ist inkonsistent", s.cfg.CurrentDir)
		}
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
	// VERSION is the only version source. Remove the historical parallel marker
	// when staging a release so it cannot become authoritative again.
	if err := os.Remove(filepath.Join(dir, ".release-version")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("legacy .release-version kann nicht entfernt werden: %w", err)
	}
	markers := map[string]string{".release-project": s.cfg.ProjectName, ".release-source": sourceRef(s)}
	if commit := strings.TrimSpace(s.artifact.Commit); commit != "" {
		markers[".release-commit"] = commit
	}
	for n, v := range markers {
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
	Mode        string `json:"mode,omitempty"`
	Commit      string `json:"commit,omitempty"`
}

func writeRootState(cfg config.Config, version, source string) error {
	data, err := json.MarshalIndent(rootReleaseState{ProjectName: cfg.ProjectName, Version: version, Source: source, Mode: cfg.Mode}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return tools.WriteFileAtomic(filepath.Join(cfg.ReleaseRoot, ".last-state.json"), data, 0o644)
}

func writeProjectVersion(cfg config.Config, version string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		return errors.New("Projektversion darf nicht leer sein")
	}
	if _, err := versionutil.Parse(version); err != nil {
		return fmt.Errorf("Projektversion %q ist ungültig: %w", version, err)
	}
	return tools.WriteFileAtomic(filepath.Join(cfg.RootDir, "VERSION"), []byte(version+"\n"), 0o644)
}

func syncProjectVersionFromCurrent(cfg config.Config) error {
	v, sourcePath, found, err := updatecheck.DetectInstalled(cfg.CurrentDir)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	version := v.String()
	// VERSION is the canonical installed project version. A legacy
	// .release-version file is read only as a compatibility fallback when
	// VERSION is missing; in that case migrate the value into VERSION.
	if filepath.Base(sourcePath) == ".release-version" {
		currentVersion := filepath.Join(cfg.CurrentDir, "VERSION")
		if err := tools.WriteFileAtomic(currentVersion, []byte(version+"\n"), 0o644); err != nil {
			return fmt.Errorf("legacy .release-version konnte nicht nach current/VERSION migriert werden: %w", err)
		}
	}
	return writeProjectVersion(cfg, version)
}

func writeLegacyRootMarkers(cfg config.Config, version, source string) error {
	// VERSION is a project-root compatibility marker as well as the canonical
	// version input for many Justfiles. Keep it synchronized with current/ only
	// after an operation has reached its final state.
	if err := writeProjectVersion(cfg, version); err != nil {
		return err
	}
	for n, v := range map[string]string{".project-name": cfg.ProjectName, ".last-version": version, ".last-source": source} {
		if err := tools.WriteMarker(cfg.ReleaseRoot, n, v); err != nil {
			return err
		}
	}
	return nil
}

func writeReleaseState(s *state) error {
	data, err := json.MarshalIndent(rootReleaseState{ProjectName: s.cfg.ProjectName, Version: s.version.String(), Source: sourceRef(s), Mode: s.cfg.Mode, Commit: strings.TrimSpace(s.artifact.Commit)}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return tools.WriteFileAtomic(filepath.Join(s.cfg.ReleaseRoot, ".last-state.json"), data, 0o644)
}

func verifyMarker(dir, expected string) error {
	b, err := os.ReadFile(filepath.Join(dir, "VERSION"))
	if err != nil {
		return fmt.Errorf("VERSION fehlt in %s: %w", dir, err)
	}
	if strings.TrimSpace(string(b)) != expected {
		return fmt.Errorf("VERSION in %s ist inkonsistent", dir)
	}
	return nil
}
func enforceVersionPolicy(c config.Config, artifact source.Artifact, allow, force, simulation bool) error {
	target := artifact.Version
	installed, _, found, err := updatecheck.DetectInstalled(c.CurrentDir)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	cmp := versionutil.CompareForProject(c.ProjectName, target, installed)
	if cmp < 0 && !allow {
		return fmt.Errorf("Downgrade wird blockiert: installiert %s, ausgewählt %s; --allow-downgrade verwenden", installed.String(), target.String())
	}
	if cmp == 0 && c.Mode == config.ModePull && strings.TrimSpace(artifact.Commit) != "" {
		installedCommit := updatecheck.DetectInstalledCommit(c.CurrentDir)
		if installedCommit != strings.TrimSpace(artifact.Commit) {
			return nil
		}
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

// appendSuccessHistory treats history as audit metadata, not as part of the
// filesystem transaction. Once an operation has committed successfully, an
// unavailable history file must not turn that success into a false operational
// failure. The warning is written to stderr and therefore preserves JSON-only
// stdout for structured commands.
func appendSuccessHistory(console *ui.Console, c config.Config, e history.Entry) {
	if err := appendHistory(c, e); err != nil {
		console.Warn("Vorgang erfolgreich, Historie konnte nicht geschrieben werden: " + err.Error())
	}
}
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
	tmp, err := os.MkdirTemp("", "update-cli-verify-*")
	if err != nil {
		return verificationResult{}, err
	}
	defer tools.RemoveTree(tmp)
	stats, err := archive.ExtractWithStats(ctx, s.artifact.ArchivePath, tmp, archiveLimits(c))
	if err != nil {
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
	Mode                             string `json:"mode"`
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
	r := updatePlanResult{ProjectName: s.cfg.ProjectName, Mode: s.cfg.Mode, SourceType: s.artifact.Type, Source: sourceRef(s), FromVersion: s.fromVersion, ToVersion: s.version.String(), ReleaseDir: s.releaseDir, CurrentDir: s.cfg.CurrentDir, Created: []rsyncutil.Change{}, Updated: []rsyncutil.Change{}, Deleted: []rsyncutil.Change{}, Other: []rsyncutil.Change{}, Protected: append([]string(nil), s.cfg.Preserve...)}
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

func debugEffectiveConfig(console *ui.Console, cfg config.Config) {
	console.Header("Debug — effektive Konfiguration")
	console.Row("Globale config.json", cfg.GlobalConfigFile)
	console.Row("Lokale config.json", cfg.ConfigFile)
	if cfg.ProjectManifestFile != "" {
		console.Row("Projekt update-cli.yaml", cfg.ProjectManifestFile)
		if len(cfg.ManifestOverrides) > 0 {
			console.Row("Manifest überschreibt", strings.Join(cfg.ManifestOverrides, ", "))
		}
	}
	console.Row("Config-Merge", "global -> lokal (lokal überschreibt; sync.preserve wird vereinigt)")
	console.Row("Globale templates.json", cfg.GlobalTemplatesFile)
	console.Row("Lokale templates.json", cfg.TemplatesFile)
	console.Row("Templates-Merge", "global -> lokal (gleichnamige lokale Templates überschreiben)")
	console.Row("History", cfg.HistoryFile)
	console.Row("Quelle", fmt.Sprintf("%s: %s", cfg.Source.Type, sourceConfigReference(cfg)))
	console.Row("Release-Verzeichnis", cfg.ReleaseRoot)
	console.Row("Current-Verzeichnis", cfg.CurrentDir)
	console.Row("Preserve/Exclude", strings.Join(cfg.Preserve, ", "))
	console.Row("Setup-Fehler/Rsync", fmt.Sprintf("keepRsyncOnError=%t", cfg.KeepRsyncOnSetupError))
	console.Row("Rsync Release", "Artefakt -> versioniertes release (Preserve-Dateien bleiben im Release enthalten)")
	console.Row("Rsync Current", "release -> current (bestehende Preserve-Pfade schützen; fehlende werden initial übernommen)")
}

func sourceConfigReference(cfg config.Config) string {
	switch cfg.Source.Type {
	case "repository":
		return cfg.Source.Repository
	case "url":
		return cfg.Source.URL
	default:
		return cfg.Source.Folder
	}
}

func resolveInitRoot(explicit, _ string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return config.ResolveRoot(explicit)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Abs(cwd)
}

func initialize(ctx context.Context, console *ui.Console, root string, o options) error {
	var cfg config.Config
	var err error
	configExists := false
	if info, statErr := os.Stat(filepath.Join(root, config.ConfigDirName, config.ConfigFileName)); statErr == nil && !info.IsDir() {
		configExists = true
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}

	repository := strings.TrimSpace(o.repository)
	if strings.TrimSpace(o.fromRepository) != "" {
		defaultUser := config.DefaultRepositoryUser
		if configExists {
			if existing, loadErr := config.Load(root, ""); loadErr == nil && strings.TrimSpace(existing.Source.DefaultUser) != "" {
				defaultUser = existing.Source.DefaultUser
			}
		}
		repository, err = config.NormalizeRepositorySpec(o.fromRepository, defaultUser)
		if err != nil {
			return err
		}
		if o.debug {
			console.Row("Repository Eingabe", o.fromRepository)
			console.Row("Repository DefaultUser", defaultUser)
			console.Row("Repository normalisiert", repository)
		}
	}

	if configExists && !o.force {
		cfg, err = config.Load(root, "")
		if err != nil {
			return err
		}
		if cfg.ProjectName != strings.TrimSpace(o.projectName) {
			return fmt.Errorf("Projekt %q ist bereits als %q initialisiert; --force zum Ersetzen verwenden", root, cfg.ProjectName)
		}
		if strings.TrimSpace(o.fromRepository) != "" || strings.TrimSpace(o.repository) != "" || strings.TrimSpace(o.sourceType) != "" || strings.TrimSpace(o.sourceFolder) != "" || strings.TrimSpace(o.downloadDir) != "" || strings.TrimSpace(o.sourceURL) != "" || strings.TrimSpace(o.mode) != "" {
			requested, overrideErr := config.WithSourceOverrides(cfg, o.mode, o.sourceType, firstNonEmpty(o.sourceFolder, o.downloadDir), o.sourceURL, repository)
			if overrideErr != nil {
				return overrideErr
			}
			if requested.Mode != cfg.Mode || requested.Source.Type != cfg.Source.Type || requested.Source.Folder != cfg.Source.Folder || requested.Source.URL != cfg.Source.URL || requested.Source.Repository != cfg.Source.Repository {
				return errors.New("Projekt ist bereits mit einer anderen Quelle initialisiert; --force zum Neuinitialisieren verwenden")
			}
		}
		console.Info("Vorhandene Projektkonfiguration wird für den Bootstrap weiterverwendet")
	} else {
		cfg, err = config.Init(root, config.InitOptions{ProjectName: o.projectName, Mode: o.mode, SourceType: o.sourceType, Folder: firstNonEmpty(o.sourceFolder, o.downloadDir), URL: o.sourceURL, Repository: repository, Force: o.force})
		if err != nil {
			return err
		}
	}

	if err := templates.EnsureLocal(cfg.TemplatesFile); err != nil {
		return err
	}
	if o.useTemplate != "" {
		t, err := templates.LookupMerged(cfg.GlobalTemplatesFile, cfg.TemplatesFile, o.useTemplate)
		if err != nil {
			return err
		}
		if err := config.ApplyTemplate(root, t.NoParameter, t.Preserve); err != nil {
			return err
		}
		cfg, err = config.Load(root, "")
		if err != nil {
			return err
		}
	}

	if o.debug {
		debugEffectiveConfig(console, cfg)
	}

	console.Header("Update CLI initialisiert")
	console.Row("Projekt", cfg.ProjectName)
	console.Row("Projektordner", cfg.RootDir)
	console.Row("Konfiguration", cfg.ConfigFile)
	console.Row("Quelle", initSourceDescription(cfg))
	console.Row("Release", cfg.ReleaseRoot)
	console.Row("Current", cfg.CurrentDir)
	console.Row("Backup", cfg.BackupRoot)
	console.Info("Initiales Release wird installiert und vorhandenes Projekt-Setup automatisch ausgeführt")

	return runUpdate(ctx, console, cfg, options{
		update:  true,
		setup:   true,
		force:   o.force,
		noUI:    o.noUI,
		noColor: o.noColor,
		wait:    o.wait,
		noWait:  o.noWait,
	})
}

func initSourceDescription(cfg config.Config) string {
	switch cfg.Source.Type {
	case source.Repository:
		return "repository: " + cfg.Source.Repository
	case source.URL:
		return "url: " + cfg.Source.URL
	default:
		return fmt.Sprintf("download: %s (%s-v<MAJOR>.<MINOR>.<PATCH>.zip)", cfg.Source.Folder, cfg.ProjectName)
	}
}

func runConfig(ctx context.Context, console *ui.Console, root string, o options) error {
	if o.configCheck {
		result, err := config.Check(root)
		if err != nil {
			return fmt.Errorf("config.json ist ungültig: %w", err)
		}
		if o.jsonOutput {
			return writeJSON(result)
		}
		console.Header("Updater-Konfiguration geprüft")
		console.Row("Datei", result.ConfigFile)
		console.Row("Projekt", result.ProjectName)
		console.Row("Schema", fmt.Sprintf("%d", result.SchemaVersion))
		console.Row("Aktuelles Schema", fmt.Sprintf("%d", result.CurrentSchema))
		console.Row("Modus", result.Mode)
		console.Row("Quelle", result.SourceType)
		if result.MigrationNeeded {
			console.Warn("Konfiguration ist gültig, aber eine Migration ist verfügbar: update-cli config --migrate")
		} else {
			console.Success("config.json ist gültig und aktuell")
		}
		return nil
	}
	if o.configMigrate {
		lock, err := tools.AcquireLock(filepath.Join(root, ".release-update.lock"), "config-migrate")
		if err != nil {
			return err
		}
		defer lock.Release()
		result, err := config.Upgrade(root)
		if err != nil {
			return err
		}
		if o.jsonOutput {
			return writeJSON(result)
		}
		printUpgrade(console, result)
		return nil
	}
	if len(o.configSet) > 0 {
		result, err := config.Set(root, o.configSet)
		if err != nil {
			return err
		}
		if o.jsonOutput {
			return writeJSON(result)
		}
		console.Header("Updater-Konfiguration geändert")
		console.Row("Datei", result.ConfigFile)
		for _, change := range result.Changes {
			b, _ := json.Marshal(change.Value)
			console.Row(change.Key, string(b))
		}
		console.Success("config.json wurde aktualisiert und validiert")
		return nil
	}
	cfg, err := config.Load(root, "")
	if err != nil {
		return err
	}
	if o.configList {
		console.Header("Konfigurationsdateien")
		console.Row("config.json:", cfg.GlobalConfigFile)
		console.Row("", cfg.ConfigFile)
		console.Row("templates.json:", cfg.GlobalTemplatesFile)
		console.Row("", cfg.TemplatesFile)
		console.Row("history.jsonl", cfg.HistoryFile)
		return nil
	}
	if o.useTemplate != "" {
		if err := templates.EnsureLocal(cfg.TemplatesFile); err != nil {
			return err
		}
		t, err := templates.LookupMerged(cfg.GlobalTemplatesFile, cfg.TemplatesFile, o.useTemplate)
		if err != nil {
			return err
		}
		if err := config.ApplyTemplate(root, t.NoParameter, t.Preserve); err != nil {
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
			return fmt.Errorf("Editor %s geschlossen, aber config.json ungültig: %w", used, err)
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
	if o.templatesList {
		f, err := templates.LoadMerged(cfg.GlobalTemplatesFile, cfg.TemplatesFile)
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
		t, err := templates.LookupMerged(cfg.GlobalTemplatesFile, cfg.TemplatesFile, o.templateUse)
		if err != nil {
			return err
		}
		if err := config.ApplyTemplate(root, t.NoParameter, t.Preserve); err != nil {
			return err
		}
		if _, err := config.Load(root, ""); err != nil {
			return err
		}
		console.Success("Template angewendet")
		return nil
	}
	if o.edit {
		if err := templates.EnsureLocal(cfg.TemplatesFile); err != nil {
			return err
		}
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
	if len(catalog.Steps) > 0 {
		console.Append("Steps")
		for _, step := range catalog.Steps {
			id := step.ID
			if strings.TrimSpace(id) == "" {
				id = "(keine id)"
			}
			detail := step.Name
			if strings.TrimSpace(step.Task) != "" {
				detail = step.Task + " | " + detail
			}
			if strings.TrimSpace(step.Operation) != "" {
				detail += " | " + step.Operation
			}
			console.Append(fmt.Sprintf("  %-24s %s", id, strings.TrimSpace(detail)))
		}
	}
}

func setupManagementDirectory(root string) (string, error) {
	stateDir := filepath.Join(root, config.ConfigDirName)
	if _, err := os.Stat(stateDir); err == nil {
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
