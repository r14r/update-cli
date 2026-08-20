package updater

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/r14r/update-cli/lib/config"
	"github.com/r14r/update-cli/lib/source"
	versionutil "github.com/r14r/update-cli/lib/version"
)

func TestEnforceVersionPolicyAllowsUpdateCLINumberingReset(t *testing.T) {
	current := filepath.Join(t.TempDir(), "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "VERSION"), []byte("3.3.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target, err := versionutil.Parse("0.8.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := enforceVersionPolicy(config.Config{ProjectName: "update-cli", CurrentDir: current}, source.Artifact{Metadata: source.Metadata{Version: target, VersionText: target.String()}}, false, false, false); err != nil {
		t.Fatalf("numbering reset treated as downgrade: %v", err)
	}
}

func TestEnforceVersionPolicyStillBlocksOtherProjectDowngrade(t *testing.T) {
	current := filepath.Join(t.TempDir(), "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "VERSION"), []byte("3.3.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target, _ := versionutil.Parse("0.8.0")
	if err := enforceVersionPolicy(config.Config{ProjectName: "other", CurrentDir: current}, source.Artifact{Metadata: source.Metadata{Version: target, VersionText: target.String()}}, false, false, false); err == nil {
		t.Fatal("ordinary project downgrade unexpectedly allowed")
	}
}

func TestEnforceVersionPolicyAllowsUpdateCLIStable100From0823(t *testing.T) {
	current := filepath.Join(t.TempDir(), "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "VERSION"), []byte("0.8.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target, _ := versionutil.Parse("1.0.0")
	if err := enforceVersionPolicy(config.Config{ProjectName: "update-cli", CurrentDir: current}, source.Artifact{Metadata: source.Metadata{Version: target, VersionText: target.String()}}, false, false, false); err != nil {
		t.Fatalf("stable 1.0.0 transition treated as downgrade: %v", err)
	}
}

func TestEnforceVersionPolicyAllowsUpdateCLIStable100FromLegacy334(t *testing.T) {
	current := filepath.Join(t.TempDir(), "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "VERSION"), []byte("3.3.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target, _ := versionutil.Parse("1.0.0")
	if err := enforceVersionPolicy(config.Config{ProjectName: "update-cli", CurrentDir: current}, source.Artifact{Metadata: source.Metadata{Version: target, VersionText: target.String()}}, false, false, false); err != nil {
		t.Fatalf("legacy 3.3.4 -> 1.0.0 transition treated as downgrade: %v", err)
	}
}

func TestEnforceVersionPolicyAllowsUpdateCLIStablePatchFrom100(t *testing.T) {
	current := filepath.Join(t.TempDir(), "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "VERSION"), []byte("1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target, _ := versionutil.Parse("1.0.1")
	if err := enforceVersionPolicy(config.Config{ProjectName: "update-cli", CurrentDir: current}, source.Artifact{Metadata: source.Metadata{Version: target, VersionText: target.String()}}, false, false, false); err != nil {
		t.Fatalf("stable patch 1.0.0 -> 1.0.1 treated as downgrade: %v", err)
	}
}

func TestEnforceVersionPolicyAllowsUpdateCLIStable103From102(t *testing.T) {
	current := filepath.Join(t.TempDir(), "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "VERSION"), []byte("1.0.2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target, _ := versionutil.Parse("1.0.3")
	if err := enforceVersionPolicy(config.Config{ProjectName: "update-cli", CurrentDir: current}, source.Artifact{Metadata: source.Metadata{Version: target, VersionText: target.String()}}, false, false, false); err != nil {
		t.Fatalf("stable patch 1.0.2 -> 1.0.3 treated as downgrade: %v", err)
	}
}

func TestEnforceVersionPolicyPullAllowsNewCommitAtSameVersion(t *testing.T) {
	current := filepath.Join(t.TempDir(), "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, ".release-commit"), []byte("aaaaaaaaaaaaaaaa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target, _ := versionutil.Parse("1.2.3")
	artifact := source.Artifact{Metadata: source.Metadata{Type: source.Repository, Version: target, VersionText: target.String(), Commit: "bbbbbbbbbbbbbbbb"}}
	if err := enforceVersionPolicy(config.Config{ProjectName: "demo", Mode: config.ModePull, CurrentDir: current}, artifact, false, false, false); err != nil {
		t.Fatalf("same-version new pull commit should be allowed: %v", err)
	}
}

func TestEnforceVersionPolicyPullTreatsSameCommitAsInstalled(t *testing.T) {
	current := filepath.Join(t.TempDir(), "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, ".release-commit"), []byte("aaaaaaaaaaaaaaaa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target, _ := versionutil.Parse("1.2.3")
	artifact := source.Artifact{Metadata: source.Metadata{Type: source.Repository, Version: target, VersionText: target.String(), Commit: "aaaaaaaaaaaaaaaa"}}
	var same *VersionAlreadyInstalledError
	err := enforceVersionPolicy(config.Config{ProjectName: "demo", Mode: config.ModePull, CurrentDir: current}, artifact, false, false, false)
	if !errors.As(err, &same) {
		t.Fatalf("expected already-installed error, got %v", err)
	}
}
