package updater

import (
	"fmt"
	"github.com/r14r/update-cli/lib/backup"
	"github.com/r14r/update-cli/lib/cleanup"
	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/doctor"
	"github.com/r14r/update-cli/lib/history"
	"github.com/r14r/update-cli/lib/inventory"
	"github.com/r14r/update-cli/lib/projectstatus"
	"github.com/r14r/update-cli/lib/rollback"
	rsyncutil "github.com/r14r/update-cli/lib/rsync"
	"github.com/r14r/update-cli/lib/ui"
	"github.com/r14r/update-cli/lib/updatecheck"
	"os"
	"strings"
)

func printHelp(v string) {
	fmt.Printf(`Update CLI %s

Usage:
  update-cli --check [--no-ask] [--wait|--no-wait] [--no-ui]
  update-cli --update [ARCHIVE.zip] [--backup] [--setup|--no-setup] [--force] [--wait|--no-wait] [--no-ui]
  update-cli --update --plan [--json]
  update-cli --backup
  update-cli --rollback [VERSION] [--setup]
  update-cli --restore latest
  update-cli --status [--json]
  update-cli --list [--json]
  update-cli --verify ARCHIVE.zip
  update-cli --doctor
  update-cli --setup [--details] [--wait|--no-wait] [--no-ui]
  update-cli --setup-list
  update-cli --setup-task NAME [--details] [--no-ui]
  update-cli --setup-workflow NAME [--details] [--no-ui]
  update-cli --setup-manifest ./setup.yaml [--setup-list|--setup-task NAME|--setup-workflow NAME] [--details] [--wait|--no-wait] [--no-ui]
  update-cli --convert-yaml [--dry-run]
  update-cli --create-yaml [--force] [--dry-run]
  update-cli --create-setup-script [--force] [--dry-run]
  update-cli --cleanup [--keep N] [--plan]
  update-cli --history [--limit N]
  update-cli --config [--list|--edit|--use-template NAME]
  update-cli --templates --list [--details]
  update-cli --init PROJECTNAME
  update-cli --upgrade
  update-cli --unlock
  update-cli --howto
  update-cli --version
`, v)
}
func printHowTo(v string) {
	printHelp(v)
	fmt.Printf(`
Safety model:
  * Every real update/rollback/restore uses an exact temporary transaction snapshot.
  * If activation, setup, service restart, or healthcheck fails, current is restored.
  * A Docker Compose stack is restarted only when it was running before the operation.
  * sync.preserve protects persistent paths from overwrite and deletion.
  * ZIP and URL limits protect against oversized downloads and ZIP bombs.
  * HTTPS is required unless security.allowHttp=true.
  * setup.yaml schemaVersion 1 remains supported for legacy id/when/run and typed v3 steps.
  * setup.yaml schemaVersion 2 adds workflows, reusable tasks, dependencies, variables, requirements, structured conditions and typed project operations.
  * --setup runs workflow 'setup'; --setup-list shows available workflows/tasks; --setup-task and --setup-workflow run a selected entry.
  * --convert-yaml upgrades setup.yaml to the newest supported schema and keeps a backup of schemaVersion 1.
  * --create-yaml detects Go, Python, Node, Laravel and Docker files and generates a schemaVersion 2 sample manifest.
  * --create-setup-script generates a generic setup.sh bootstrap that delegates execution to Update CLI.
  * Legacy setup.sh/config.setup.commands remain as fallback.
  * Interactive check/update/setup use the fullscreen TUI by default; --no-ui or UPDATE_CLI_TUI=plain disables it.
  * --no-ui streams setup/process output directly to stdout/stderr without alternate-screen rendering.

Release archive:
  <PROJECT>-v<MAJOR>.<MINOR>.<PATCH>.zip

Configuration:
  .updater-cli/config.json
  setup.yaml
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
		fmt.Fprintf(os.Stdout, "  %-16s %-9s %-16s %-8s %s → %s  %s\n", x.Timestamp.Local().Format("2006-01-02 15:04"), x.Action, x.Phase, x.Status, empty(x.FromVersion), empty(x.ToVersion), detail)
	}
}
func printCleanup(c *ui.Console, r cleanup.Result) {
	title := "Cleanup abgeschlossen"
	if r.Plan {
		title = "Cleanup-Plan"
	}
	c.Header(title)
	c.Row("Releases entfernen", fmt.Sprint(len(r.RemovedRelease)))
	c.Row("Backups entfernen", fmt.Sprint(len(r.RemovedBackup)))
	for _, p := range r.RemovedRelease {
		fmt.Fprintln(os.Stdout, "  -", p)
	}
	for _, p := range r.RemovedBackup {
		fmt.Fprintln(os.Stdout, "  -", p)
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
	c.Row("Quelle", r.SourceType+" — "+r.SourceReference)
	c.Row("Installiert", empty(r.InstalledVersion))
	c.Row("Verfügbar", empty(r.AvailableVersion))
	c.Row("Setup", fmt.Sprintf("%t %s", r.SetupAvailable, r.SetupPath))
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
		fmt.Fprintf(os.Stdout, "  %-10s validated=%-5t%s  %s\n", x.Version, x.Validated, mark, x.Path)
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
	if r.SourceError != "" {
		c.StatusRow("Status", "Quellenfehler: "+r.SourceError)
		return
	}
	c.StatusRow("Status", string(r.Status))
}
func printDoctor(c *ui.Console, r doctor.Report) {
	c.Header("Update CLI Doctor")
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
	title := fmt.Sprintf("Release Update     from %s to %s", empty(s.fromVersion), s.version.String())
	if c.Fullscreen() {
		c.SetInfoTitle(title)
		c.InfoRow("Projekt", s.cfg.ProjectName)
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
	c.Banner(title)
	c.Row("Projekt", s.cfg.ProjectName)
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
		fmt.Fprintln(os.Stdout, "  DELETE", x.Path)
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
	if c.Fullscreen() {
		c.SetFooterSuccess("OK   Update abgeschlossen")
		return
	}
	c.Header("Update abgeschlossen")
	c.Row("Projekt", s.cfg.ProjectName)
	c.Row("Version", empty(s.fromVersion)+" → "+s.version.String())
	c.Row("Quelle", sourceRef(s))
	c.Row("Release", s.releaseDir)
	c.Row("Current", s.cfg.CurrentDir)
	c.Row("Setup", fmt.Sprint(setup))
	c.Success("Update committed")
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
