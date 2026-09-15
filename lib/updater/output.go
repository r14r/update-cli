package updater

import (
	"fmt"
	"github.com/r14r/update-cli/lib/backup"
	"github.com/r14r/update-cli/lib/cleanup"
	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/doctor"
	"github.com/r14r/update-cli/lib/history"
	"github.com/r14r/update-cli/lib/inventory"
	"github.com/r14r/update-cli/lib/projectsetup"
	"github.com/r14r/update-cli/lib/projectstatus"
	"github.com/r14r/update-cli/lib/rollback"
	rsyncutil "github.com/r14r/update-cli/lib/rsync"
	"github.com/r14r/update-cli/lib/ui"
	"github.com/r14r/update-cli/lib/updatecheck"
	"os"
	"sort"
	"strings"
)

type fixResult struct {
	Root      string                             `json:"root"`
	Config    config.RepairResult                `json:"config"`
	Manifest  projectsetup.ProjectRepairResult   `json:"manifest,omitempty"`
	Manifests []projectsetup.ProjectRepairResult `json:"manifests,omitempty"`
}

func printFix(c *ui.Console, r fixResult) {
	c.Header("Update CLI Fix")
	c.Row("Projektwurzel", r.Root)
	printFixConfigFile(c, r.Config.Local)
	if r.Config.Global != nil {
		printFixConfigFile(c, *r.Config.Global)
	}
	manifestChanged := false
	manifests := r.Manifests
	if len(manifests) == 0 && r.Manifest.Path != "" {
		manifests = []projectsetup.ProjectRepairResult{r.Manifest}
	}
	for _, manifest := range manifests {
		c.Row("Manifest", manifest.Path)
		c.Row("Manifest-Schema", fmt.Sprintf("%d → %d", manifest.PreviousSchema, manifest.CurrentSchema))
		if manifest.BackupPath != "" {
			c.Row("Manifest-Backup", manifest.BackupPath)
		}
		for _, value := range manifest.RemovedFields {
			c.Info("Manifest entfernt: " + value)
		}
		for _, value := range manifest.Normalized {
			c.Info("Manifest normalisiert: " + value)
		}
		manifestChanged = manifestChanged || manifest.Changed
	}
	if r.Config.Local.Changed || (r.Config.Global != nil && r.Config.Global.Changed) || manifestChanged {
		c.Success("Projektdateien wurden auf den aktuellen Standard repariert")
	} else {
		c.Success("Keine Reparaturen erforderlich")
	}
}

func printFixConfigFile(c *ui.Console, r config.RepairFileResult) {
	label := "Lokale config.json"
	if r.Scope == "global" {
		label = "Globale config.json"
	}
	c.Row(label, r.Path)
	if r.BackupPath != "" {
		c.Row(label+" Backup", r.BackupPath)
	}
	for _, value := range r.MigratedFields {
		c.Info(label + " migriert: " + value)
	}
	for _, value := range r.RemovedFields {
		c.Info(label + " entfernt: " + value)
	}
	for _, value := range r.Normalized {
		c.Info(label + " normalisiert: " + value)
	}
}

type commandHelp struct {
	Usage       []string
	Description string
	Options     []string
	Examples    []string
}

var commandHelpDetails = map[string]commandHelp{
	"check": {
		Usage:       []string{"update-cli check [--no-ask] [--wait|--no-wait] [--no-ui]"},
		Description: "Prüft die installierte gegen die verfügbare Version und kann interaktiv in das Update wechseln.",
		Options:     []string{"--no-ask  nur prüfen, keine Installationsfrage", "--wait/--no-wait  Abschlussansicht steuern", "--no-ui  direkte Terminalausgabe"},
		Examples:    []string{"update-cli check", "update-cli check --no-ask"},
	},
	"update": {
		Usage:       []string{"update-cli update [--archive ARCHIVE.zip] [--backup] [--setup|--no-setup] [--force]", "update-cli update --plan [--json]"},
		Description: "Installiert ein ZIP-Release oder aktualisiert die konfigurierte Repository-Quelle transaktional.",
		Options:     []string{"--backup  persistentes Pre-Update-Backup erstellen", "--setup/--no-setup  Setup nach dem Sync ausführen/überspringen", "--plan  nur Update-Plan anzeigen", "--force  Versions-/Sicherheitsentscheidung explizit überschreiben"},
		Examples:    []string{"update-cli update", "update-cli update --archive ~/Downloads/demo-v1.2.3.zip --no-setup", "update-cli update --plan --json"},
	},
	"setup": {
		Usage: []string{
			"update-cli setup",
			"update-cli setup --list [--json]",
			"update-cli setup --run STEP_ID [--details]",
			"update-cli setup --task NAME [--details]",
			"update-cli setup --workflow NAME [--details]",
		},
		Description: "Analysiert und führt das Projekt-Setup aus. --list zeigt Workflows, Tasks und jeden Setup-Schritt mit seiner id; --run führt exakt einen Schritt anhand dieser id aus.",
		Options: []string{
			"--list  alle Setup-Workflows, Tasks und Steps anzeigen",
			"--run STEP_ID  genau den Setup-Step mit dieser id ausführen; keine Task-Abhängigkeiten werden automatisch ausgeführt",
			"--details  Commands/Operationen während der Ausführung detaillierter anzeigen",
			"--json  strukturierte Ausgabe für setup --list",
			"--no-ui  direkte Terminalausgabe",
		},
		Examples: []string{"update-cli setup --list", "update-cli setup --run go-vet --details", "update-cli setup --task build", "update-cli setup --workflow ci"},
	},
	"config": {
		Usage:       []string{"update-cli config [--set KEY=VALUE ...] [--check|--migrate|--list|--edit|--use-template NAME]"},
		Description: "Verwaltet die lokale .update-cli/config.json und deren effektive Host-/Projektkonfiguration.",
		Options:     []string{"--check  Konfiguration nur validieren", "--migrate  Schema sicher migrieren und Backup in .update-cli/ anlegen", "--list  Pfade und Konfiguration anzeigen", "--set KEY=VALUE  lokalen Wert setzen"},
		Examples:    []string{"update-cli config --check", "update-cli config --migrate", "update-cli config --set no-parameter=update,no-setup"},
	},
	"doctor": {
		Usage:       []string{"update-cli doctor [--migrate] [--json]", "update-cli doctor --fix"},
		Description: "Prüft Runtime-Konfiguration und Root-/current-Manifeste und erklärt exakt, warum der UI-Header 'Migration required' anzeigt oder warum keine Migration notwendig ist. --fix zeigt alle deterministisch reparierbaren Änderungen vorab und führt sie erst nach y/n-Bestätigung aus.",
		Options:     []string{"--migrate  sichere Schema-Migration direkt ausführen", "--fix  Reparaturplan anzeigen, y/n bestätigen und anschließend reparieren", "--json  maschinenlesbare Diagnose; nicht mit --fix kombinierbar"},
		Examples:    []string{"update-cli doctor", "update-cli doctor --fix", "update-cli doctor --migrate"},
	},
	"fix": {
		Usage:       []string{"update-cli fix [--json]"},
		Description: "Repariert bekannte Legacy-Felder und sicher normalisierbare Konfigurations-/Manifestfehler mit Backup.",
		Examples:    []string{"update-cli fix"},
	},
	"install": {
		Usage:       []string{"update-cli install"},
		Description: "Führt exakt `just install` im aktiven current/ bzw. Source-Projekt aus.",
		Examples:    []string{"update-cli install"},
	},
	"run": {
		Usage:       []string{"update-cli run"},
		Description: "Startet die im Manifest unter run.command oder run.steps konfigurierte Anwendung.",
		Examples:    []string{"update-cli run"},
	},
	"rollback": {
		Usage:       []string{"update-cli rollback --version VERSION [--setup]"},
		Description: "Aktiviert ein vorhandenes älteres Release transaktional erneut.",
		Examples:    []string{"update-cli rollback --version 2.14.4", "update-cli rollback --version 2.14.4 --setup"},
	},
	"backup": {
		Usage:       []string{"update-cli backup"},
		Description: "Erstellt ein persistentes Projekt-Backup nach der konfigurierten Backup-Policy.",
		Examples:    []string{"update-cli backup"},
	},
	"status": {
		Usage:       []string{"update-cli status [--json]"},
		Description: "Zeigt installierte Version, Projektzustand und relevante Update-Metadaten.",
		Examples:    []string{"update-cli status", "update-cli status --json"},
	},
	"schema": {
		Usage:       []string{"update-cli schema --view", "update-cli schema --save FILE.json", "update-cli schema --version"},
		Description: "Zeigt, speichert oder versioniert das JSON Schema für update-cli.yaml.",
		Examples:    []string{"update-cli schema --version", "update-cli schema --save update-cli.schema.json"},
	},
	"init": {
		Usage:       []string{"update-cli init --project PROJECTNAME", "update-cli init --project PROJECTNAME --from-repository REPOSITORY"},
		Description: "Initialisiert den aktuellen Ordner als verwaltetes Update-CLI-Projekt und installiert das erste Release.",
		Examples:    []string{"update-cli init --project demo", "update-cli init --project demo --from-repository r14r/demo"},
	},
	"releases": {
		Usage:       []string{"update-cli releases --list [--json]"},
		Description: "Listet lokal vorhandene versionierte Releases.",
		Examples:    []string{"update-cli releases --list"},
	},
	"restore": {
		Usage:       []string{"update-cli restore --snapshot BACKUP|latest"},
		Description: "Stellt einen zuvor erstellten Projekt-Backup-Stand wieder her.",
		Examples:    []string{"update-cli restore --snapshot latest"},
	},
	"verify": {
		Usage:       []string{"update-cli verify --archive ARCHIVE.zip"},
		Description: "Validiert ein Release-Archiv ohne Installation.",
		Examples:    []string{"update-cli verify --archive demo-v1.2.3.zip"},
	},
	"history": {
		Usage:       []string{"update-cli history [--limit N]"},
		Description: "Zeigt die persistente Update-Historie des Projekts.",
		Examples:    []string{"update-cli history --limit 10"},
	},
	"clean": {
		Usage:       []string{"update-cli clean [--keep N] [--plan]"},
		Description: "Entfernt obsolete Release-Verzeichnisse, ohne Backups zu verändern.",
		Examples:    []string{"update-cli clean --plan"},
	},
	"cleanup": {
		Usage:       []string{"update-cli cleanup [--keep N] [--plan]"},
		Description: "Wendet die konfigurierte Retention auf Releases und Backups an.",
		Examples:    []string{"update-cli cleanup --plan"},
	},
	"templates": {
		Usage:       []string{"update-cli templates --list [--details]", "update-cli templates --edit", "update-cli templates --use NAME"},
		Description: "Verwaltet globale und projektlokale Konfigurationstemplates.",
		Examples:    []string{"update-cli templates --list --details"},
	},
	"upgrade": {
		Usage:       []string{"update-cli upgrade"},
		Description: "Aktualisiert die Projekt-Runtime-Konfiguration auf das aktuelle Schema.",
		Examples:    []string{"update-cli upgrade"},
	},
	"unlock": {
		Usage:       []string{"update-cli unlock"},
		Description: "Entfernt einen als veraltet erkannten Update-Lock.",
		Examples:    []string{"update-cli unlock"},
	},
	"howto": {
		Usage:       []string{"update-cli howto"},
		Description: "Zeigt erweiterte Hinweise zu Update-, Setup- und Sicherheitssemantik.",
		Examples:    []string{"update-cli howto"},
	},
	"convert-yaml": {
		Usage:       []string{"update-cli convert-yaml [--dry-run] [--force]"},
		Description: "Konvertiert ein älteres Setup-Manifest auf das aktuelle update-cli.yaml-Schema.",
		Examples:    []string{"update-cli convert-yaml --dry-run"},
	},
	"create-yaml": {
		Usage:       []string{"update-cli create-yaml --from project|setup-script [--with-ai] [--force] [--dry-run]"},
		Description: "Erzeugt ein Schema-2-Manifest aus Projektstruktur oder setup.sh.",
		Examples:    []string{"update-cli create-yaml --from project --dry-run"},
	},
	"create-setup-script": {
		Usage:       []string{"update-cli create-setup-script [--force] [--dry-run]"},
		Description: "Erzeugt ein generisches setup.sh-Bootstrap-Skript.",
		Examples:    []string{"update-cli create-setup-script --dry-run"},
	},
	"version": {
		Usage:       []string{"update-cli version"},
		Description: "Zeigt die Update-CLI-Version aus der einzigen Versionsquelle VERSION.",
		Examples:    []string{"update-cli version"},
	},
}

func printHelp(v string) {
	printCommandHelp(v, "", false)
}

func printCommandHelp(v, topic string, details bool) {
	topic = strings.ToLower(strings.TrimSpace(topic))
	if topic != "" {
		h, ok := commandHelpDetails[topic]
		if !ok {
			fmt.Printf("Update CLI %s\n\nUnbekannter Help-Command: %s\n\n", v, topic)
			printHelpSummary()
			return
		}
		fmt.Printf("Update CLI %s — %s\n\n", v, topic)
		fmt.Println("Usage:")
		for _, usage := range h.Usage {
			fmt.Println("  " + usage)
		}
		if details {
			fmt.Printf("\nBeschreibung:\n  %s\n", h.Description)
			if len(h.Options) > 0 {
				fmt.Println("\nOptionen:")
				for _, option := range h.Options {
					fmt.Println("  " + option)
				}
			}
			if len(h.Examples) > 0 {
				fmt.Println("\nBeispiele:")
				for _, example := range h.Examples {
					fmt.Println("  " + example)
				}
			}
		}
		fmt.Println("\nFür ausführliche Hilfe: update-cli help --command " + topic + " --details")
		return
	}
	fmt.Printf("Update CLI %s\n\n", v)
	printHelpSummary()
	if details {
		fmt.Println("\nCommand details:")
		names := make([]string, 0, len(commandHelpDetails))
		for name := range commandHelpDetails {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Printf("  %-12s %s\n", name, commandHelpDetails[name].Description)
		}
		fmt.Println("\nUse: update-cli help --command COMMAND --details")
	}
}

func printHelpSummary() {
	fmt.Print(`Usage:
  update-cli check [--no-ask] [--wait|--no-wait] [--no-ui]
  update-cli update [--archive ARCHIVE.zip] [--backup] [--setup|--no-setup] [--force] [--wait|--no-wait] [--no-ui]
  update-cli update --plan [--json]
  update-cli backup
  update-cli rollback --version VERSION [--setup]
  update-cli restore --snapshot latest
  update-cli status [--json]
  update-cli releases --list [--json]
  update-cli verify --archive ARCHIVE.zip
  update-cli doctor [--migrate|--fix] [--json]
  update-cli fix [--json]
  update-cli run
  update-cli install
  update-cli schema --view|--save FILE.json|--version
  update-cli setup [--details] [--wait|--no-wait] [--no-ui]
  update-cli setup --list [--json]
  update-cli setup --run STEP_ID [--details] [--no-ui]
  update-cli setup --task NAME [--details] [--no-ui]
  update-cli setup --workflow NAME [--details] [--no-ui]
  update-cli config [--set KEY=VALUE ...] [--check|--migrate|--list|--edit|--use-template NAME]
  update-cli init --project PROJECTNAME
  update-cli upgrade
  update-cli unlock
  update-cli howto
  update-cli version

Global options:
  --debug     show direct detailed execution steps for any command
  --details   with help: show purpose, options and examples for the selected command
`)
}

func printHowTo(v string) {
	printHelp(v)
	fmt.Printf(`
Safety model:
  * Updates/rollback/restore prefer an immutable previous release/<VERSION>/ as fast rollback basis; an exact current/ snapshot is only the fallback.
  * If activation, setup, service restart, or healthcheck fails, current is restored.
  * A Docker Compose stack is restarted only when it was running before the operation.
  * sync.preserve protects persistent paths from overwrite and deletion.
  * init --project PROJECT uses the current working directory as the project root, writes .update-cli/config.json there, installs the initial release and runs available setup automation.
  * init --project PROJECT uses the configured/default download folder and selects the newest matching PROJECT-v<MAJOR>.<MINOR>.<PATCH>.zip or PROJECT-<MAJOR>.<MINOR>.<PATCH>.zip archive.
  * init --project PROJECT --from-repository URL bootstraps from an explicit repository URL.
  * init --project PROJECT --from-repository REPOSITORY is the compact equivalent and also accepts USER/REPO or REPO (using source.defaultUser).
  * --debug is global and prints detailed execution steps directly for every command.
  * mode=update installs versioned ZIP releases from download folders or HTTPS URLs.
  * mode=pull updates a persistent internal Git checkout with git pull --ff-only and deploys it transactionally.
  * ZIP and URL limits protect against oversized downloads and ZIP bombs.
  * HTTPS is required unless security.allowHttp=true.
  * update-cli.yaml/setup.yaml schemaVersion 1 remains supported for legacy id/when/run and typed steps.
  * update-cli.yaml/setup.yaml schemaVersion 2 adds workflows, reusable tasks, dependencies, variables, requirements, structured conditions and typed project operations.
  * schema --view prints the canonical update-cli.yaml JSON Schema; schema --save FILE.json writes the identical schema to disk; schema --version prints the manifest schema version.
  * doctor validates update-cli.yaml directly in the current project folder without requiring .update-cli/config.json; doctor --migrate upgrades older manifest schemas and canonicalizes legacy setup.yaml.
  * fix repairs local/global config.json plus root/current update-cli.yaml before strict parsing, migrates known legacy fields such as defaultUser -> source.defaultUser, removes unknown fields, normalizes safely repairable values, and creates backups before changes.
  * run executes run.command or structured run.steps from the discovered update-cli.yaml/setup.yaml in the active current/ release.
  * install runs exactly 'just install' in current/ when that active release contains a justfile, otherwise in the project root.
  * setup runs workflow 'setup'; setup --list shows available workflows/tasks; setup --task and setup --workflow run a selected entry.
  * Before setup, a project-provided migrate.sh runs once per installed semantic version; completion is stored persistently as .update-cli/.migration.done.<VERSION>.
  * convert-yaml upgrades update-cli.yaml to the newest supported schema and keeps a backup of schemaVersion 1.
  * create-yaml --from project detects Go, Python, Node, Laravel and Docker files and generates a schemaVersion 2 sample manifest.
  * create-yaml --from setup-script analyzes setup.sh and converts the detected ordered operations into schemaVersion 2.
  * --with-ai optionally refines the deterministic setup.sh conversion using a configured Ollama or OpenAI-compatible model; the AI result must validate as schemaVersion 2.
  * create-setup-script generates a generic setup.sh bootstrap that delegates execution to Update CLI.
  * clean removes obsolete release-directory entries only; installed and rollback-safe previous releases are preserved and backups are untouched.
  * cleanup applies the broader configured retention policy to releases and backups.
  * Global config/templates are loaded from INSTALLFOLDER/../etc/update-cli before project-local overrides; /usr/local/bin/update-cli maps to /usr/local/etc/update-cli.
  * config --check validates the effective global+local configuration without changing files; config --migrate upgrades .update-cli/config.json in place and keeps its backup in .update-cli/.
  * config --set KEY=VALUE changes only the project-local .update-cli/config.json by dotted JSON path.
  * Legacy setup.sh/config.setup.commands remain as fallback; if none exist and a Justfile is present, setup runs just build then just install.
  * Interactive check/update/setup use the fullscreen TUI by default; --no-ui or UPDATE_CLI_TUI=plain disables it.
  * --noui is accepted as an alias for --no-ui.
  * --no-ui streams setup/process output directly to stdout/stderr without alternate-screen rendering.

Release archive:
  <PROJECT>-v<MAJOR>.<MINOR>.<PATCH>.zip
  <PROJECT>-<MAJOR>.<MINOR>.<PATCH>.zip

Configuration:
  INSTALLFOLDER/../etc/update-cli/config.json      global updater defaults
  INSTALLFOLDER/../etc/update-cli/templates.json   global templates
  .update-cli/config.json                          project source/policy overrides
  .update-cli/templates.json                       project template overrides
  update-cli.yaml                                  setup/run/tasks/workflows
  setup.yaml                                       legacy setup manifest fallback
  .update-cli/config.json                         legacy updater config (read/migrate)
`)
}
func printUpgrade(c *ui.Console, r config.UpgradeResult) {
	c.Header("Konfiguration aktualisiert")
	c.Row("Schema", fmt.Sprintf("%d → %d", r.PreviousSchema, r.CurrentSchema))
	c.Row("Datei", r.ConfigFile)
	if r.BackupFile != "" {
		c.Row("Backup", r.BackupFile)
	}
	if r.Changed {
		c.Success("Konfiguration wurde migriert")
	} else {
		c.Success("Konfiguration ist bereits aktuell")
	}
}
func printHistory(c *ui.Console, e []history.Entry) {
	c.Header("Update-Historie")
	for _, x := range e {
		detail := x.Message
		if detail == "" {
			detail = x.Source
		}
		fmt.Fprintf(os.Stdout, "  %-16s %-9s %-16s %-8s %s → %s  %s\n", x.Timestamp.Local().Format("2006-01-02 15:04"), x.Action, x.Phase, x.Status, empty(x.FromVersion), empty(x.ToVersion), ui.DisplayText(detail))
	}
}
func printCleanup(c *ui.Console, r cleanup.Result) {
	title := "Cleanup abgeschlossen"
	if r.ReleaseOnly {
		title = "Release-Cleanup abgeschlossen"
	}
	if r.Plan {
		if r.ReleaseOnly {
			title = "Release-Cleanup-Plan"
		} else {
			title = "Cleanup-Plan"
		}
	}
	c.Header(title)
	c.Row("Releases entfernen", fmt.Sprint(len(r.RemovedRelease)))
	if !r.ReleaseOnly {
		c.Row("Backups entfernen", fmt.Sprint(len(r.RemovedBackup)))
	}
	for _, p := range r.RemovedRelease {
		fmt.Fprintln(os.Stdout, "  -", ui.DisplayText(p))
	}
	for _, p := range r.RemovedBackup {
		fmt.Fprintln(os.Stdout, "  -", ui.DisplayText(p))
	}
}
func printBackup(c *ui.Console, r backup.Result) {
	c.Header("Backup erstellt")
	c.Row("Name", r.Backup.Name)
	c.Row("Version", r.Backup.Version)
	c.Row("Ordner", r.Backup.Path)
	c.Row("Änderungen", fmt.Sprint(r.Sync.Changes))
	c.Success("Backup abgeschlossen")
}
func printRollback(c *ui.Console, r rollback.Result, setup bool) {
	c.Header("Rollback abgeschlossen")
	c.Row("Version", empty(r.FromVersion)+" → "+r.ToVersion)
	c.Row("Release", r.ReleaseDir)
	c.Row("Änderungen", fmt.Sprint(r.Sync.Changes))
	c.Row("Setup", fmt.Sprint(setup))
	c.Success("Rollback committed")
}
func printRestore(c *ui.Console, item backup.Item, from, to string, r rsyncutil.Result) {
	c.Header("Backup wiederhergestellt")
	c.Row("Backup", item.Name)
	c.Row("Version", empty(from)+" → "+empty(to))
	c.Row("Änderungen", fmt.Sprint(r.Changes))
	c.Success("Restore committed")
}
func printStatus(c *ui.Console, r projectstatus.Result) {
	c.Header("Updater-Status")
	c.Row("Projekt", r.ProjectName)
	c.Row("Modus", r.Mode)
	c.Row("Quelle", r.SourceType+" — "+r.SourceReference)
	c.Row("Installiert", empty(r.InstalledVersion))
	c.Row("Verfügbar", empty(r.AvailableVersion))
	c.Row("Setup", fmt.Sprintf("%t %s", r.SetupAvailable, r.SetupPath))
	c.Row("Docker lifecycle", r.DockerLifecycle)
	c.Row("Backups", fmt.Sprint(r.BackupCount))
	c.Row("Status", r.State)
	if r.SourceError != "" {
		c.Diagnostic("warning", "Release-Quelle", r.SourceError)
	}
}
func printInventory(c *ui.Console, r inventory.Result) {
	c.Header("Release-Inventar")
	c.Row("Projekt", r.ProjectName)
	c.Row("Releases", fmt.Sprint(len(r.Releases)))
	for _, x := range r.Releases {
		mark := ""
		if x.Active {
			mark = " active"
		}
		fmt.Fprintf(os.Stdout, "  %-10s validated=%-5t%s  %s\n", x.Version, x.Validated, mark, ui.DisplayText(x.Path))
	}
	c.Row("Backups", fmt.Sprint(len(r.Backups)))
	if r.Remote != nil {
		c.Row("Remote", r.Remote.Type+" "+r.Remote.VersionText+" — "+r.Remote.Reference)
	}
	if r.SourceError != "" {
		c.Diagnostic("warning", "Release-Quelle", r.SourceError)
	}
}
func printCheck(c *ui.Console, r updatecheck.Result) {
	if c.Fullscreen() {
		c.SetInfoTitle("Versionsprüfung")
		c.InfoRow("Projekt", r.ProjectName)
		c.InfoRow("Installiert", empty(r.InstalledVersion))
		c.InfoRow("Verfügbar", empty(r.AvailableVersion))
		if r.AvailableCommit != "" {
			c.InfoRow("Commit", shortCommit(r.InstalledCommit)+" → "+shortCommit(r.AvailableCommit))
		}
		if r.SourceError != "" {
			c.InfoRow("Status", "Quellenfehler: "+r.SourceError)
			return
		}
		c.InfoRow("Status", string(r.Status))
		return
	}
	c.Header("Versionsprüfung")
	c.Row("Projekt", r.ProjectName)
	c.Row("Installiert", empty(r.InstalledVersion))
	c.Row("Verfügbar", empty(r.AvailableVersion))
	if r.AvailableCommit != "" {
		c.Row("Commit", shortCommit(r.InstalledCommit)+" → "+shortCommit(r.AvailableCommit))
	}
	if r.SourceError != "" {
		c.StatusRow("Status", "Quellenfehler: "+r.SourceError)
		return
	}
	c.StatusRow("Status", string(r.Status))
}
func printDoctor(c *ui.Console, r doctor.Report) {
	c.Header("Update CLI Doctor")
	if r.WorkingDirectory != "" {
		c.Row("Arbeitsordner", r.WorkingDirectory)
	}
	c.Row("Projektwurzel", r.Root)
	if r.RuntimeConfig != nil {
		state := r.RuntimeConfig.Path
		if !r.RuntimeConfig.Exists {
			state += " (fehlt)"
		}
		c.Row("Runtime config", state)
	}
	manifestExists := false
	for _, manifest := range r.Manifests {
		manifestExists = manifestExists || manifest.Exists
	}
	for _, manifest := range r.Manifests {
		// Root and current are alternative valid manifest locations. Once one
		// exists, omit the missing alternate from the visible Doctor inventory.
		if manifestExists && !manifest.Exists {
			continue
		}
		state := manifest.Path
		if !manifest.Exists {
			state += " (fehlt)"
		} else if manifest.SchemaVersion > 0 {
			state += fmt.Sprintf(" (Schema %d)", manifest.SchemaVersion)
		}
		c.Row("Manifest "+manifest.Location, state)
	}
	if r.MigrationRequired {
		c.Row("Migration required", "JA")
		for _, reason := range r.MigrationReasons {
			c.Row("Migration reason", reason.Scope+" — "+reason.Detail)
			if strings.TrimSpace(reason.Path) != "" {
				c.Row("Migration file", reason.Path)
			}
		}
	} else {
		c.Row("Migration required", "nein")
	}
	for _, x := range r.Checks {
		c.Diagnostic(string(x.Level), x.Name, x.Detail)
	}
	c.Row("Fehler", fmt.Sprint(r.ErrorCount()))
	c.Row("Warnungen", fmt.Sprint(r.WarningCount()))
}
func printVerify(c *ui.Console, r verificationResult) {
	c.Header("Archivprüfung")
	c.Row("Archiv", r.ArchivePath)
	c.Row("Version", r.Version)
	c.Row("Dateien", fmt.Sprint(r.Stats.Files))
	c.Row("Entpackt", humanBytes(r.Stats.UncompressedBytes))
	c.Success("Archiv ist gültig")
}
func printUpdatePlan(c *ui.Console, s *state, o options) {
	fromVersion := empty(s.fromVersion)
	toVersion := s.version.String()
	updateValue := fmt.Sprintf("from %s to %s", fromVersion, toVersion)
	if c.Fullscreen() {
		c.SetInfoTitle("")
		c.InfoHighlightedRow("Release Update", updateValue, toVersion)
		c.InfoRow("Projekt", s.cfg.ProjectName)
		c.InfoRow("Update-Modus", s.cfg.Mode)
		c.InfoRow("Quelle", sourceRef(s))
		c.InfoRow("Release", s.releaseDir)
		c.InfoRow("Current", s.cfg.CurrentDir)
		c.InfoRow("Geschützt", strings.Join(s.cfg.Preserve, ", "))
		if o.plan {
			c.InfoRow("Modus", "Plan")
		} else if o.dryRun {
			c.InfoRow("Modus", "Dry-Run")
		}
		return
	}
	c.Banner("Update-Plan")
	c.InfoHighlightedRow("Release Update", updateValue, toVersion)
	c.Row("Projekt", s.cfg.ProjectName)
	c.Row("Update-Modus", s.cfg.Mode)
	c.Row("Quelle", sourceRef(s))
	c.Row("Release", s.releaseDir)
	c.Row("Current", s.cfg.CurrentDir)
	c.Row("Geschützt", strings.Join(s.cfg.Preserve, ", "))
	if o.plan {
		c.Row("Modus", "Plan")
	} else if o.dryRun {
		c.Row("Modus", "Dry-Run")
	}
}
func printDetailedPlan(c *ui.Console, s *state) {
	r := updatePlanJSON(s)
	c.Header("Detaillierter Update-Plan")
	c.Row("Erstellen", fmt.Sprint(len(r.Created)))
	c.Row("Aktualisieren", fmt.Sprint(len(r.Updated)))
	c.Row("Löschen", fmt.Sprint(len(r.Deleted)))
	for _, x := range r.Deleted {
		fmt.Fprintln(os.Stdout, "  DELETE", ui.DisplayText(x.Path))
	}
	c.Success("Plan abgeschlossen; keine Änderungen ausgeführt")
}
func printDryRun(c *ui.Console, s *state) {
	c.Header("Dry-Run abgeschlossen")
	c.Row("Release-Änderungen", fmt.Sprint(s.releaseChanges))
	c.Row("Current-Änderungen", fmt.Sprint(s.currentChanges))
	c.Success("Dateisystem unverändert")
}
func printUpdateResult(c *ui.Console, s *state, setup bool) {
	c.SetFinalStatus(s.cfg.ProjectName, "Aktualisiert auf Version: v"+s.version.String())
	if c.Fullscreen() {
		c.SetFooterSuccess("OK   Update abgeschlossen")
		return
	}
	c.Header("Update abgeschlossen")
	c.Row("Projekt", s.cfg.ProjectName)
	c.Row("Modus", s.cfg.Mode)
	c.Row("Version", empty(s.fromVersion)+" → "+s.version.String())
	c.Row("Quelle", sourceRef(s))
	c.Row("Release", s.releaseDir)
	c.Row("Current", s.cfg.CurrentDir)
	c.Row("Setup", fmt.Sprint(setup))
	c.Success("Update committed")
}

func setCheckFinalStatus(c *ui.Console, r updatecheck.Result) {
	if r.InstalledFound && strings.TrimSpace(r.InstalledVersion) != "" {
		c.SetFinalStatus(r.ProjectName, "Installierte Version: v"+r.InstalledVersion)
		return
	}
	c.SetFinalStatus(r.ProjectName, "Keine Version installiert")
}
func shortCommit(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "none"
	}
	if len(v) > 12 {
		return v[:12]
	}
	return v
}

func empty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "none"
	}
	return s
}
func humanBytes(v int64) string {
	const u = 1024
	if v < u {
		return fmt.Sprintf("%d B", v)
	}
	d := int64(u)
	e := 0
	for q := v / u; q >= u && e < 5; q /= u {
		d *= u
		e++
	}
	return fmt.Sprintf("%.1f %ciB", float64(v)/float64(d), "KMGTPE"[e])
}
