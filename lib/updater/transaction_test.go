package updater

import (
	"context"
	"errors"
	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/ui"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTransactionRecoveryRestoresExactCurrent(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	cfgDir := filepath.Join(root, ".updater-cli")
	_ = os.MkdirAll(current, 0o755)
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(current, "keep.txt"), []byte("old"), 0o644)
	_ = os.WriteFile(filepath.Join(current, ".env"), []byte("secret"), 0o600)
	c := config.Config{RootDir: root, ConfigDir: cfgDir, CurrentDir: current}
	tx, err := beginTransaction(context.Background(), c, ui.New(true))
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(current, "keep.txt"), []byte("new"), 0o644)
	_ = os.WriteFile(filepath.Join(current, "extra.txt"), []byte("x"), 0o644)
	got := tx.recover(errors.New("boom"))
	if got == nil {
		t.Fatal("expected original error")
	}
	b, _ := os.ReadFile(filepath.Join(current, "keep.txt"))
	if string(b) != "old" {
		t.Fatalf("old content not restored: %q", b)
	}
	b, _ = os.ReadFile(filepath.Join(current, ".env"))
	if string(b) != "secret" {
		t.Fatalf("env not restored: %q", b)
	}
	if _, err := os.Stat(filepath.Join(current, "extra.txt")); !os.IsNotExist(err) {
		t.Fatalf("extra file not removed: %v", err)
	}
}

func writeFakeDocker(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "docker")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func isolatedPathWithRsync(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if p, err := exec.LookPath("rsync"); err == nil {
		if err := os.Symlink(p, filepath.Join(dir, "rsync")); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func transactionFixture(t *testing.T, lifecycle string, compose bool) config.Config {
	t.Helper()
	root := t.TempDir()
	current := filepath.Join(root, "current")
	cfgDir := filepath.Join(root, ".updater-cli")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "VERSION"), []byte("1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if compose {
		if err := os.WriteFile(filepath.Join(current, "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return config.Config{RootDir: root, ConfigDir: cfgDir, CurrentDir: current, Docker: config.DockerConfig{Lifecycle: lifecycle}}
}

func TestTransactionDockerLifecycleAutoNoComposeContinues(t *testing.T) {
	cfg := transactionFixture(t, "auto", false)
	tx, err := beginTransaction(context.Background(), cfg, ui.New(true))
	if err != nil {
		t.Fatal(err)
	}
	defer tx.commit()
	if tx.dockerManaged || tx.servicesWereRunning {
		t.Fatalf("unexpected docker state: %#v", tx)
	}
}

func TestTransactionDockerLifecycleAutoStatusFailureContinues(t *testing.T) {
	cfg := transactionFixture(t, "auto", true)
	dir := isolatedPathWithRsync(t)
	log := filepath.Join(dir, "docker.log")
	writeFakeDocker(t, dir, `echo "$@" >> "`+log+`"
if [ "$1 $2" = "compose version" ]; then exit 0; fi
case " $* " in *" ps -q "*) echo 'daemon unavailable' >&2; exit 1;; esac
exit 0`)
	t.Setenv("PATH", dir)
	tx, err := beginTransaction(context.Background(), cfg, ui.New(true))
	if err != nil {
		t.Fatalf("auto must degrade instead of failing: %v", err)
	}
	defer tx.commit()
	if tx.dockerManaged || tx.servicesWereRunning {
		t.Fatalf("degraded auto must not manage docker: %#v", tx)
	}
	b, _ := os.ReadFile(log)
	if !strings.Contains(string(b), "ps -q") {
		t.Fatalf("status command not invoked: %s", b)
	}
	beforeRecovery := string(b)
	_ = tx.recover(errors.New("boom"))
	after, _ := os.ReadFile(log)
	if string(after) != beforeRecovery {
		t.Fatalf("degraded auto invoked Docker during recovery; before=%q after=%q", beforeRecovery, after)
	}
}

func TestTransactionDockerLifecycleRequiredStatusFailureFails(t *testing.T) {
	cfg := transactionFixture(t, "required", true)
	dir := isolatedPathWithRsync(t)
	writeFakeDocker(t, dir, `if [ "$1 $2" = "compose version" ]; then exit 0; fi
echo 'bad compose config' >&2
exit 1`)
	t.Setenv("PATH", dir)
	_, err := beginTransaction(context.Background(), cfg, ui.New(true))
	if err == nil || !strings.Contains(err.Error(), "bad compose config") {
		t.Fatalf("required should fail with docker detail, got %v", err)
	}
}

func TestTransactionDockerLifecycleDisabledNeverInvokesDocker(t *testing.T) {
	cfg := transactionFixture(t, "disabled", true)
	dir := isolatedPathWithRsync(t)
	log := filepath.Join(dir, "docker.log")
	writeFakeDocker(t, dir, `echo "$@" >> "`+log+`"; exit 99`)
	t.Setenv("PATH", dir)
	tx, err := beginTransaction(context.Background(), cfg, ui.New(true))
	if err != nil {
		t.Fatalf("disabled lifecycle should ignore docker: %v", err)
	}
	defer tx.commit()
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatalf("docker was invoked in disabled mode")
	}
	if got := tx.recover(errors.New("boom")); got == nil {
		t.Fatal("expected original recovery cause")
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatalf("docker was invoked during disabled recovery")
	}
}

func TestTransactionDockerLifecycleAutoConfirmedRunningRestartsOnRecovery(t *testing.T) {
	cfg := transactionFixture(t, "auto", true)
	dir := isolatedPathWithRsync(t)
	log := filepath.Join(dir, "docker.log")
	writeFakeDocker(t, dir, `echo "$@" >> "`+log+`"
if [ "$1 $2" = "compose version" ]; then exit 0; fi
case " $* " in *" ps -q "*) echo container123; exit 0;; esac
exit 0`)
	t.Setenv("PATH", dir)
	tx, err := beginTransaction(context.Background(), cfg, ui.New(true))
	if err != nil {
		t.Fatal(err)
	}
	if !tx.dockerManaged || !tx.servicesWereRunning {
		t.Fatalf("running stack not recorded: %#v", tx)
	}
	_ = tx.recover(errors.New("boom"))
	b, _ := os.ReadFile(log)
	text := string(b)
	if !strings.Contains(text, "down --remove-orphans") || !strings.Contains(text, "up -d --remove-orphans") {
		t.Fatalf("expected stop and restart, log:\n%s", text)
	}
}

func TestTransactionDockerLifecycleAutoUnavailableContinuesWithoutRecoveryDocker(t *testing.T) {
	cfg := transactionFixture(t, "auto", true)
	dir := isolatedPathWithRsync(t)
	t.Setenv("PATH", dir)
	tx, err := beginTransaction(context.Background(), cfg, ui.New(true))
	if err != nil {
		t.Fatalf("auto must continue when docker is unavailable: %v", err)
	}
	if tx.dockerManaged || tx.servicesWereRunning {
		t.Fatalf("docker should be degraded: %#v", tx)
	}
	if got := tx.recover(errors.New("boom")); got == nil {
		t.Fatal("expected recovery cause")
	}
}

func TestTransactionDockerLifecycleRequiredUnavailableFails(t *testing.T) {
	cfg := transactionFixture(t, "required", true)
	dir := isolatedPathWithRsync(t)
	t.Setenv("PATH", dir)
	_, err := beginTransaction(context.Background(), cfg, ui.New(true))
	if err == nil || !strings.Contains(err.Error(), "weder docker noch docker-compose") {
		t.Fatalf("required should fail when docker is unavailable, got %v", err)
	}
}

func TestTransactionDockerLifecycleRequiredWithoutComposeIsAllowed(t *testing.T) {
	cfg := transactionFixture(t, "required", false)
	dir := isolatedPathWithRsync(t)
	t.Setenv("PATH", dir)
	tx, err := beginTransaction(context.Background(), cfg, ui.New(true))
	if err != nil {
		t.Fatalf("required without compose should be a no-op: %v", err)
	}
	defer tx.commit()
	if tx.dockerManaged {
		t.Fatal("docker should not be managed without a compose file")
	}
}

func captureStderr(fn func() error) (string, error) {
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stderr = w
	done := make(chan []byte, 1)
	go func() { b, _ := io.ReadAll(r); done <- b }()
	runErr := fn()
	_ = w.Close()
	os.Stderr = old
	b := <-done
	_ = r.Close()
	return string(b), runErr
}

func TestTransactionDockerLifecycleAutoWarningVisibleInDirectOutput(t *testing.T) {
	cfg := transactionFixture(t, "auto", true)
	dir := isolatedPathWithRsync(t)
	writeFakeDocker(t, dir, `if [ "$1 $2" = "compose version" ]; then exit 0; fi
echo 'Cannot connect to the Docker daemon' >&2
exit 1`)
	t.Setenv("PATH", dir)
	console := ui.New(true)
	console.SetDirect(true)
	var tx *transaction
	out, beginErr := captureStderr(func() error {
		var err error
		tx, err = beginTransaction(context.Background(), cfg, console)
		return err
	})
	if beginErr != nil {
		t.Fatal(beginErr)
	}
	defer tx.commit()
	if !strings.Contains(out, "Docker Compose Status nicht verfügbar; Lifecycle wird für dieses Update übersprungen") {
		t.Fatalf("warning missing from direct output: %q", out)
	}
}

func TestTransactionDockerLifecycleDisabledEmitsNoDockerWarning(t *testing.T) {
	cfg := transactionFixture(t, "disabled", true)
	dir := isolatedPathWithRsync(t)
	t.Setenv("PATH", dir)
	console := ui.New(true)
	console.SetDirect(true)
	var tx *transaction
	out, beginErr := captureStderr(func() error {
		var err error
		tx, err = beginTransaction(context.Background(), cfg, console)
		return err
	})
	if beginErr != nil {
		t.Fatal(beginErr)
	}
	defer tx.commit()
	if strings.Contains(strings.ToLower(out), "docker") {
		t.Fatalf("disabled mode emitted Docker output: %q", out)
	}
}

func TestBeginTransactionUsesUniqueWorkspace(t *testing.T) {
	cfg := transactionFixture(t, "disabled", false)
	tx1, err := beginTransaction(context.Background(), cfg, ui.New(true))
	if err != nil {
		t.Fatal(err)
	}
	defer tx1.commit()
	tx2, err := beginTransaction(context.Background(), cfg, ui.New(true))
	if err != nil {
		t.Fatal(err)
	}
	defer tx2.commit()
	if tx1.snapshotRoot == tx2.snapshotRoot {
		t.Fatalf("transaction workspaces collided: %s", tx1.snapshotRoot)
	}
}
