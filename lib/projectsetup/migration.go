package projectsetup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/tools"
	"github.com/r14r/update-cli/lib/ui"
	versionutil "github.com/r14r/update-cli/lib/version"
)

const MigrationScriptName = "migrate.sh"

type MigrationResult struct {
	Script   string `json:"script,omitempty"`
	Marker   string `json:"marker,omitempty"`
	Version  string `json:"version,omitempty"`
	Executed bool   `json:"executed"`
	Skipped  bool   `json:"skipped,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// EnsureMigration executes current/migrate.sh at most once for each installed
// semantic version. Completion markers live below the project root's
// .update-cli directory so replacing current/ does not make an already applied
// migration appear pending again.
func EnsureMigration(ctx context.Context, c config.Config, console *ui.Console) (MigrationResult, error) {
	workDir := strings.TrimSpace(c.CurrentDir)
	if workDir == "" {
		return MigrationResult{}, errors.New("Current-Ordner für Migration fehlt")
	}
	script := filepath.Join(workDir, MigrationScriptName)
	info, err := os.Stat(script)
	if errors.Is(err, os.ErrNotExist) {
		return MigrationResult{Script: script, Skipped: true, Reason: "kein migrate.sh vorhanden"}, nil
	}
	if err != nil {
		return MigrationResult{}, fmt.Errorf("migrate.sh kann nicht geprüft werden: %w", err)
	}
	if info.IsDir() {
		return MigrationResult{}, fmt.Errorf("%s ist ein Ordner", script)
	}

	version, err := migrationVersion(workDir)
	if err != nil {
		return MigrationResult{}, err
	}
	root := migrationProjectRoot(c, workDir)
	stateDir := filepath.Join(root, config.ConfigDirName)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return MigrationResult{}, fmt.Errorf("Migrationsstatus kann nicht angelegt werden: %w", err)
	}
	marker := filepath.Join(stateDir, ".migration.done."+version)
	result := MigrationResult{Script: script, Marker: marker, Version: version}
	if fileExists(marker) {
		result.Skipped = true
		result.Reason = "Migration für Version " + version + " bereits ausgeführt"
		console.Info(result.Reason + "; migrate.sh wird übersprungen")
		return result, nil
	}

	lock, err := tools.AcquireLock(filepath.Join(stateDir, ".migration."+version+".lock"), "migration-"+version)
	if err != nil {
		return result, err
	}
	defer lock.Release()

	// A second setup process may have completed the migration while this process
	// was waiting for the migration lock.
	if fileExists(marker) {
		result.Skipped = true
		result.Reason = "Migration für Version " + version + " bereits ausgeführt"
		console.Info(result.Reason + "; migrate.sh wird übersprungen")
		return result, nil
	}

	bash, err := exec.LookPath("bash")
	if err != nil {
		return result, errors.New("migrate.sh benötigt bash")
	}
	showMigrationHeader(console, workDir, version, marker)
	err = console.Step(ctx, 0, 1, "Projekt-Migration ausführen", func() error {
		return runCommandWithEnv(ctx, workDir, bash, []string{"./" + MigrationScriptName}, console, map[string]string{
			"UPDATE_CLI_MIGRATION":    "1",
			"UPDATE_CLI_PROJECT_ROOT": root,
			"UPDATE_CLI_CURRENT_DIR":  workDir,
			"UPDATE_CLI_VERSION":      version,
		})
	})
	if err != nil {
		return result, fmt.Errorf("Migration für Version %s fehlgeschlagen: %w", version, err)
	}

	markerBody := fmt.Sprintf("version=%s\ncompletedAt=%s\nscript=%s\n", version, time.Now().UTC().Format(time.RFC3339), MigrationScriptName)
	if err := tools.WriteFileAtomic(marker, []byte(markerBody), 0o644); err != nil {
		return result, fmt.Errorf("Migration war erfolgreich, Abschlussmarker konnte aber nicht geschrieben werden: %w", err)
	}
	result.Executed = true
	return result, nil
}

func migrationVersion(workDir string) (string, error) {
	path := filepath.Join(workDir, "VERSION")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("%s benötigt eine VERSION-Datei für den einmaligen Migrationsmarker", MigrationScriptName)
		}
		return "", err
	}
	parsed, err := versionutil.Parse(strings.TrimSpace(string(data)))
	if err != nil {
		return "", fmt.Errorf("VERSION für %s ist ungültig: %w", MigrationScriptName, err)
	}
	return parsed.String(), nil
}

func migrationProjectRoot(c config.Config, workDir string) string {
	if root := strings.TrimSpace(c.RootDir); root != "" {
		return filepath.Clean(root)
	}
	clean := filepath.Clean(workDir)
	if filepath.Base(clean) == "current" {
		return filepath.Dir(clean)
	}
	return clean
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func showMigrationHeader(console *ui.Console, workDir, version, marker string) {
	if console.Fullscreen() {
		console.Append("")
		console.Append("Projekt-Migration")
		console.SetupMeta(1, "Version: "+version)
		console.SetupMeta(1, "Script: "+filepath.Join(workDir, MigrationScriptName))
		console.SetupMeta(1, "Marker: "+marker)
		return
	}
	console.Header("Projekt-Migration")
	console.Row("Version", version)
	console.Row("Script", filepath.Join(workDir, MigrationScriptName))
	console.Row("Marker", marker)
}
