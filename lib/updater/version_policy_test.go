package updater

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/r14r/update-cli/lib/config"
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
	if err := enforceVersionPolicy(config.Config{ProjectName: "update-cli", CurrentDir: current}, target, false, false, false); err != nil {
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
	if err := enforceVersionPolicy(config.Config{ProjectName: "other", CurrentDir: current}, target, false, false, false); err == nil {
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
	if err := enforceVersionPolicy(config.Config{ProjectName: "update-cli", CurrentDir: current}, target, false, false, false); err != nil {
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
	if err := enforceVersionPolicy(config.Config{ProjectName: "update-cli", CurrentDir: current}, target, false, false, false); err != nil {
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
	if err := enforceVersionPolicy(config.Config{ProjectName: "update-cli", CurrentDir: current}, target, false, false, false); err != nil {
		t.Fatalf("stable patch 1.0.0 -> 1.0.1 treated as downgrade: %v", err)
	}
}
