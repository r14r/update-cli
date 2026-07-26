package updater

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"release-updater/lib/config"
	"release-updater/lib/ui"
	"release-updater/lib/updatecheck"
	versionutil "release-updater/lib/version"
)

func TestParseInitOptions(t *testing.T) {
	opts, err := parseOptions([]string{"--init", "release-updater-go", "--force"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.init || !opts.force || opts.projectName != "release-updater-go" {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestInitRequiresDirectProjectName(t *testing.T) {
	_, err := parseOptions([]string{"--init"})
	if err == nil || !strings.Contains(err.Error(), "--init release-updater-go") {
		t.Fatalf("expected direct project-name hint, got %v", err)
	}
}

func TestParseUpgradeOptions(t *testing.T) {
	opts, err := parseOptions([]string{"--upgrade", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.upgrade || !opts.jsonOutput {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestUpgradeRejectsPositionalArgument(t *testing.T) {
	_, err := parseOptions([]string{"--upgrade", "release-updater-go"})
	if err == nil || !strings.Contains(err.Error(), "nicht zulässig") {
		t.Fatalf("expected positional rejection, got %v", err)
	}
}

func TestParseCheckOptions(t *testing.T) {
	opts, err := parseOptions([]string{"--check", "--downloads", "/tmp/downloads"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.check || opts.downloadDir != "/tmp/downloads" {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestParseDoctorOptions(t *testing.T) {
	opts, err := parseOptions([]string{"--doctor", "--root", "/tmp/project"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.doctor || opts.rootDir != "/tmp/project" {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestOperationalModesConflict(t *testing.T) {
	_, err := parseOptions([]string{"--check", "--doctor"})
	if err == nil || !strings.Contains(err.Error(), "schließen sich") {
		t.Fatalf("expected mode conflict, got %v", err)
	}
}

func TestCheckRejectsArchive(t *testing.T) {
	_, err := parseOptions([]string{"--check", "demo-v1.0.0.zip"})
	if err == nil || !strings.Contains(err.Error(), "Archivargument") {
		t.Fatalf("expected archive rejection, got %v", err)
	}
}

func TestParseSetupOptions(t *testing.T) {
	opts, err := parseOptions([]string{"--setup", "--root", "/tmp/project"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.setup || opts.rootDir != "/tmp/project" {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestParseConfigEditOptions(t *testing.T) {
	opts, err := parseOptions([]string{"--config", "--edit"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.config || !opts.edit {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestEditRequiresConfig(t *testing.T) {
	_, err := parseOptions([]string{"--edit"})
	if err == nil || !strings.Contains(err.Error(), "nur zusammen mit --config") {
		t.Fatalf("expected config requirement, got %v", err)
	}
}

func TestSetupRejectsArchive(t *testing.T) {
	_, err := parseOptions([]string{"--setup", "demo-v1.0.0.zip"})
	if err == nil || !strings.Contains(err.Error(), "Archivargument") {
		t.Fatalf("expected archive rejection, got %v", err)
	}
}

func TestParseConfigTemplateOptions(t *testing.T) {
	opts, err := parseOptions([]string{"--config", "--use-template", "FastAPI"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.config || opts.useTemplate != "FastAPI" {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestUseTemplateRequiresConfig(t *testing.T) {
	_, err := parseOptions([]string{"--use-template", "Laravel"})
	if err == nil || !strings.Contains(err.Error(), "nur zusammen mit --config") {
		t.Fatalf("expected config requirement, got %v", err)
	}
}

func TestConfigTemplateCanOpenEditorAfterApplying(t *testing.T) {
	opts, err := parseOptions([]string{"--config", "--use-template", "Vue", "--edit"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.config || !opts.edit || opts.useTemplate != "Vue" {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestNoArgumentsShowsHelp(t *testing.T) {
	opts, err := parseOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !opts.showHelp {
		t.Fatalf("expected help mode, got %#v", opts)
	}
}

func TestParseHowToOptions(t *testing.T) {
	opts, err := parseOptions([]string{"--howto"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.showHowTo || opts.showHelp {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestHelpIsShortAndHowToIsDetailed(t *testing.T) {
	help := captureStdout(t, func() { printHelp("2.1.2") })
	if !strings.Contains(help, "Verfügbare Befehle:") {
		t.Fatalf("short help misses command overview: %q", help)
	}
	if strings.Contains(help, "Beispiele:") || strings.Contains(help, "Konfiguration:") {
		t.Fatalf("short help contains detailed sections: %q", help)
	}
	if !strings.Contains(help, "update-cli --howto") {
		t.Fatalf("short help misses howto hint: %q", help)
	}

	howto := captureStdout(t, func() { printHowTo("2.1.2") })
	for _, section := range []string{"Verwendung:", "Beispiele:", "Konfiguration:", "Setup-Templates:"} {
		if !strings.Contains(howto, section) {
			t.Fatalf("howto misses %q: %q", section, howto)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	defer func() { os.Stdout = original }()

	fn()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestPrintUpdateCheckUsesRequestedOrderAndPlainStatus(t *testing.T) {
	installed, err := versionutil.Parse("3.24.0")
	if err != nil {
		t.Fatal(err)
	}
	available, err := versionutil.Parse("3.25.0")
	if err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		printUpdateCheck(
			ui.New(true),
			config.Config{ProjectName: "mediastudio"},
			updatecheck.Result{
				Installed:       installed,
				InstalledFound:  true,
				InstalledSource: "/project/current/.release-version",
				Available:       available,
				ArchivePath:     "/downloads/mediastudio-v3.25.0.zip",
				Status:          updatecheck.StatusUpdateAvailable,
			},
		)
	})

	ordered := []string{
		"Projekt",
		"Quelle",
		"Archiv",
		"Installiert",
		"Verfügbar",
		"Status",
	}
	last := -1
	for _, token := range ordered {
		index := strings.Index(output, token)
		if index == -1 {
			t.Fatalf("output misses %q: %q", token, output)
		}
		if index <= last {
			t.Fatalf("%q is out of order in %q", token, output)
		}
		last = index
	}
	if strings.Contains(output, "WARN") || strings.Contains(output, "OK    Status") {
		t.Fatalf("status must not contain a diagnostic marker: %q", output)
	}
	if !strings.Contains(output, "  Status              Update verfügbar: 3.24.0 → 3.25.0") {
		t.Fatalf("unexpected status formatting: %q", output)
	}
}

func TestParseUpdateOptions(t *testing.T) {
	opts, err := parseOptions([]string{"--update", "--dry-run", "--archive", "demo-v1.2.3.zip"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.update || !opts.dryRun || opts.archive != "demo-v1.2.3.zip" {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestArchiveRequiresUpdate(t *testing.T) {
	_, err := parseOptions([]string{"demo-v1.2.3.zip"})
	if err == nil || !strings.Contains(err.Error(), "nur zusammen mit --update") {
		t.Fatalf("expected update requirement, got %v", err)
	}
}

func TestDryRunRequiresUpdate(t *testing.T) {
	_, err := parseOptions([]string{"--dry-run"})
	if err == nil || !strings.Contains(err.Error(), "nur zusammen mit --update") {
		t.Fatalf("expected update requirement, got %v", err)
	}
}

func TestUpdateConflictsWithCheck(t *testing.T) {
	_, err := parseOptions([]string{"--update", "--check"})
	if err == nil || !strings.Contains(err.Error(), "schließen sich") {
		t.Fatalf("expected mode conflict, got %v", err)
	}
}

func TestParseUpdateWithSetupOptions(t *testing.T) {
	opts, err := parseOptions([]string{"--update", "--setup", "--archive", "demo-v1.2.3.zip"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.update || !opts.setup || opts.archive != "demo-v1.2.3.zip" {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestUpdateSetupRejectsDryRun(t *testing.T) {
	_, err := parseOptions([]string{"--update", "--setup", "--dry-run"})
	if err == nil || !strings.Contains(err.Error(), "kann nicht mit --dry-run") {
		t.Fatalf("expected dry-run conflict, got %v", err)
	}
}

func TestSetupConflictsWithDoctor(t *testing.T) {
	_, err := parseOptions([]string{"--doctor", "--setup"})
	if err == nil || !strings.Contains(err.Error(), "nur allein oder zusammen mit --update") {
		t.Fatalf("expected setup mode conflict, got %v", err)
	}
}

func TestParseStatusJSONOptions(t *testing.T) {
	opts, err := parseOptions([]string{"--status", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.status || !opts.jsonOutput {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestParseListOptions(t *testing.T) {
	opts, err := parseOptions([]string{"--list"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.list {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestVerifyRequiresArchive(t *testing.T) {
	_, err := parseOptions([]string{"--verify"})
	if err == nil || !strings.Contains(err.Error(), "benötigt ein Archiv") {
		t.Fatalf("expected archive requirement, got %v", err)
	}
}

func TestParseVerifyJSON(t *testing.T) {
	opts, err := parseOptions([]string{"--verify", "--json", "demo-v1.2.3.zip"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.verify || !opts.jsonOutput || opts.archive != "demo-v1.2.3.zip" {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestParseUpdatePlan(t *testing.T) {
	opts, err := parseOptions([]string{"--update", "--plan", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.update || !opts.plan || !opts.jsonOutput {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestAllowDowngradeRequiresUpdate(t *testing.T) {
	_, err := parseOptions([]string{"--allow-downgrade"})
	if err == nil || !strings.Contains(err.Error(), "nur zusammen mit --update") {
		t.Fatalf("expected update requirement, got %v", err)
	}
}

func TestUpdateJSONRequiresPlan(t *testing.T) {
	_, err := parseOptions([]string{"--update", "--json"})
	if err == nil || !strings.Contains(err.Error(), "nur zusammen mit --plan") {
		t.Fatalf("expected plan requirement, got %v", err)
	}
}

func TestEnforceVersionPolicyBlocksDowngrade(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, ".release-version"), []byte("2.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target, err := versionutil.Parse("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	err = enforceVersionPolicy(config.Config{CurrentDir: current}, target, false, false, false)
	if err == nil || !strings.Contains(err.Error(), "Downgrade wird blockiert") {
		t.Fatalf("expected downgrade block, got %v", err)
	}
	if err := enforceVersionPolicy(config.Config{CurrentDir: current}, target, true, false, false); err != nil {
		t.Fatalf("allowed downgrade failed: %v", err)
	}
}

func TestParseUpdateForce(t *testing.T) {
	opts, err := parseOptions([]string{"--update", "--force"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.update || !opts.force {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestForceRequiresInitOrUpdate(t *testing.T) {
	_, err := parseOptions([]string{"--force"})
	if err == nil || !strings.Contains(err.Error(), "--init oder --update") {
		t.Fatalf("expected force mode requirement, got %v", err)
	}
}

func TestEnforceVersionPolicyBlocksInstalledVersionWithoutForce(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, ".release-version"), []byte("2.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target, err := versionutil.Parse("2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{CurrentDir: current}
	if err := enforceVersionPolicy(cfg, target, false, false, false); err == nil || !strings.Contains(err.Error(), "bereits installiert") {
		t.Fatalf("expected already-installed block, got %v", err)
	}
	if err := enforceVersionPolicy(cfg, target, false, true, false); err != nil {
		t.Fatalf("forced reinstall failed: %v", err)
	}
	if err := enforceVersionPolicy(cfg, target, false, false, true); err != nil {
		t.Fatalf("simulation should remain available: %v", err)
	}
}

func TestForceDoesNotAllowDowngrade(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, ".release-version"), []byte("2.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target, err := versionutil.Parse("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	err = enforceVersionPolicy(config.Config{CurrentDir: current}, target, false, true, false)
	if err == nil || !strings.Contains(err.Error(), "Downgrade wird blockiert") {
		t.Fatalf("force must not bypass downgrade protection: %v", err)
	}
}

func TestParseStandaloneBackup(t *testing.T) {
	opts, err := parseOptions([]string{"--backup", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.backup || opts.update || !opts.jsonOutput {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestParseUpdateBackupSetup(t *testing.T) {
	opts, err := parseOptions([]string{"--update", "--backup", "--setup"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.update || !opts.backup || !opts.setup {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestUpdateBackupRejectsPlan(t *testing.T) {
	_, err := parseOptions([]string{"--update", "--backup", "--plan"})
	if err == nil || !strings.Contains(err.Error(), "kann nicht mit --dry-run oder --plan") {
		t.Fatalf("expected backup plan conflict, got %v", err)
	}
}

func TestParseRollbackVersionAndSetup(t *testing.T) {
	opts, err := parseOptions([]string{"--rollback", "1.2.3", "--setup"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.rollback || opts.rollbackVersion != "1.2.3" || !opts.setup {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestParseRestore(t *testing.T) {
	opts, err := parseOptions([]string{"--restore", "latest", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.restore != "latest" || !opts.jsonOutput {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestParseHistoryLimit(t *testing.T) {
	opts, err := parseOptions([]string{"--history", "--limit", "5"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.history || opts.limit != 5 {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestParseCleanupPlan(t *testing.T) {
	opts, err := parseOptions([]string{"--cleanup", "--keep", "2", "--plan", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.cleanup || opts.keep != 2 || !opts.plan || !opts.jsonOutput {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestRunWithoutParametersExecutesConfiguredSetup(t *testing.T) {
	root := t.TempDir()
	cfg, err := config.Init(root, config.InitOptions{ProjectName: "demo"})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(cfg.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	value["no parameter"] = "setup"
	data, err = json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(cfg.ConfigFile, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(cfg.CurrentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/usr/bin/env bash\nprintf 'ok\\n' > setup-result.txt\n"
	if err := os.WriteFile(filepath.Join(cfg.CurrentDir, "setup.sh"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	if err := Run(context.Background(), "2.0.0", nil); err != nil {
		t.Fatal(err)
	}
	result, err := os.ReadFile(filepath.Join(cfg.CurrentDir, "setup-result.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(result)) != "ok" {
		t.Fatalf("unexpected setup result: %q", result)
	}
}

func TestDisplayPathUsesProjectRelativePath(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "projects", "demo")
	got := displayPath(root, filepath.Join(root, "release", "2.1.0"))
	if got != "./release/2.1.0" {
		t.Fatalf("unexpected relative path: %q", got)
	}
}

func TestDisplayPathKeepsExternalPathAbsolute(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "projects", "demo")
	external := filepath.Join(string(filepath.Separator), "Users", "demo", "Downloads", "demo-v2.1.0.zip")
	if got := displayPath(root, external); got != external {
		t.Fatalf("external path should remain absolute: %q", got)
	}
}

func TestUpdateHeaderShowsVersionTransition(t *testing.T) {
	got := updateHeader("3.24.0", "3.25.0")
	want := "Release Update     from 3.24.0 to 3.25.0"
	if got != want {
		t.Fatalf("unexpected update header: want %q, got %q", want, got)
	}
}

func TestUpdateHeaderUsesNoneForInitialInstallation(t *testing.T) {
	got := updateHeader("", "1.0.0")
	want := "Release Update     from none to 1.0.0"
	if got != want {
		t.Fatalf("unexpected initial-install header: want %q, got %q", want, got)
	}
}

func TestParseInitWithTemplate(t *testing.T) {
	opts, err := parseOptions([]string{"--init", "demo", "--use-template", "Laravel"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.init || opts.projectName != "demo" || opts.useTemplate != "Laravel" {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestParseTemplatesSubcommands(t *testing.T) {
	listOpts, err := parseOptions([]string{"--templates", "--list"})
	if err != nil {
		t.Fatal(err)
	}
	if !listOpts.templatesMode || !listOpts.templatesList || listOpts.list {
		t.Fatalf("unexpected list options: %#v", listOpts)
	}

	detailOpts, err := parseOptions([]string{"--templates", "--list", "--details"})
	if err != nil {
		t.Fatal(err)
	}
	if !detailOpts.templatesMode || !detailOpts.templatesList || !detailOpts.details {
		t.Fatalf("unexpected detail options: %#v", detailOpts)
	}

	useOpts, err := parseOptions([]string{"--templates", "--use", "Laravel"})
	if err != nil {
		t.Fatal(err)
	}
	if !useOpts.templatesMode || useOpts.templateUse != "Laravel" {
		t.Fatalf("unexpected use options: %#v", useOpts)
	}

	editOpts, err := parseOptions([]string{"--templates", "--edit", "Laravel"})
	if err != nil {
		t.Fatal(err)
	}
	if !editOpts.templatesMode || !editOpts.edit || editOpts.templateName != "Laravel" {
		t.Fatalf("unexpected edit options: %#v", editOpts)
	}
}

func TestParseConfigList(t *testing.T) {
	opts, err := parseOptions([]string{"--config", "--list"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.config || !opts.configList || opts.list {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestScopedHelpTopic(t *testing.T) {
	for _, test := range []struct {
		args  []string
		topic string
	}{
		{[]string{"--config", "--help"}, "config"},
		{[]string{"--templates", "--help"}, "templates"},
		{[]string{"--update", "--help"}, "update"},
	} {
		topic, ok := scopedHelpTopic(test.args)
		if !ok || topic != test.topic {
			t.Fatalf("scopedHelpTopic(%#v) = %q, %t", test.args, topic, ok)
		}
	}
}

func TestDetailsRequiresTemplateList(t *testing.T) {
	if _, err := parseOptions([]string{"--templates", "--details"}); err == nil || !strings.Contains(err.Error(), "--templates --list") {
		t.Fatalf("expected --details validation error, got %v", err)
	}
}

func TestSameVersionReturnsTypedAlreadyInstalledError(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, ".release-version"), []byte("2.2.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{CurrentDir: current}
	target, err := versionutil.Parse("2.2.1")
	if err != nil {
		t.Fatal(err)
	}
	err = enforceVersionPolicy(cfg, target, false, false, false)
	var installedErr *VersionAlreadyInstalledError
	if !errors.As(err, &installedErr) {
		t.Fatalf("expected VersionAlreadyInstalledError, got %T: %v", err, err)
	}
}

func TestParseUpdateFromURL(t *testing.T) {
	opts, err := parseOptions([]string{"--update", "--from", "url", "--url", "https://example.test/demo-v1.2.3.zip"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.update || opts.sourceType != "url" || opts.sourceURL == "" {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestParseUpdateFromRepository(t *testing.T) {
	opts, err := parseOptions([]string{"--update", "--from", "repository", "--repository", "https://example.test/repo.git"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.sourceType != "repository" || opts.repository == "" {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestParseInitWithSourceFolder(t *testing.T) {
	opts, err := parseOptions([]string{"--init", "demo", "--from", "download", "--folder", "/tmp/releases"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.init || opts.sourceFolder != "/tmp/releases" {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestExplicitArchiveRejectsSourceOverride(t *testing.T) {
	_, err := parseOptions([]string{"--update", "demo-v1.0.0.zip", "--from", "download"})
	if err == nil || !strings.Contains(err.Error(), "kann nicht mit --from") {
		t.Fatalf("expected source conflict, got %v", err)
	}
}
