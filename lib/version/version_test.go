package version

import (
	"os"
	"path/filepath"
	"testing"
)

func mustVersion(t *testing.T, s string) Version {
	t.Helper()
	v, err := Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestUpdateCLINumberingResetIsNotDowngrade(t *testing.T) {
	legacy := mustVersion(t, "3.3.4")
	baseline := mustVersion(t, "0.8.0")
	if CompareForProject("update-cli", baseline, legacy) <= 0 {
		t.Fatal("0.8.0 new numbering line must be newer than legacy 3.3.4")
	}
	if CompareForProject("other-project", baseline, legacy) >= 0 {
		t.Fatal("other projects must retain strict semantic-version ordering")
	}
}

func TestUpdateCLIArchiveSelectionPrefersNewNumberingLine(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"update-cli-v3.3.4.zip", "update-cli-v0.8.0.zip", "update-cli-v0.8.1.zip"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, v, err := SelectNewest(dir, "update-cli")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := v.String(), "0.8.1"; got != want {
		t.Fatalf("newest version = %s, want %s", got, want)
	}
}

func TestCompareForProjectUpdateCLIStable100IsNewerThanTransitionLine(t *testing.T) {
	stable, _ := Parse("1.0.0")
	transition, _ := Parse("0.8.23")
	if got := CompareForProject("update-cli", stable, transition); got <= 0 {
		t.Fatalf("expected 1.0.0 > 0.8.23, got %d", got)
	}
}

func TestCompareForProjectUpdateCLIStablePatchOrdering(t *testing.T) {
	newer, _ := Parse("1.0.1")
	older, _ := Parse("1.0.0")
	if got := CompareForProject("update-cli", newer, older); got <= 0 {
		t.Fatalf("expected 1.0.1 > 1.0.0, got %d", got)
	}
}

func TestCompareForProjectUpdateCLIStable100IsNewerThanLegacy334(t *testing.T) {
	stable, _ := Parse("1.0.0")
	legacy, _ := Parse("3.3.4")
	if got := CompareForProject("update-cli", stable, legacy); got <= 0 {
		t.Fatalf("expected 1.0.0 > legacy 3.3.4, got %d", got)
	}
}

func TestCompareForProjectOtherProjectsRemainStrictSemVer(t *testing.T) {
	one, _ := Parse("1.0.0")
	three, _ := Parse("3.3.4")
	if got := CompareForProject("other-project", one, three); got >= 0 {
		t.Fatalf("expected normal semver 1.0.0 < 3.3.4, got %d", got)
	}
}

func TestUpdateCLIArchiveSelectionPrefersStablePatch(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"update-cli-v3.3.4.zip", "update-cli-v0.8.23.zip", "update-cli-v1.0.0.zip", "update-cli-v1.0.1.zip"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, v, err := SelectNewest(dir, "update-cli")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := v.String(), "1.0.1"; got != want {
		t.Fatalf("newest version = %s, want %s", got, want)
	}
}

func TestUpdateCLIArchiveSelectionPrefersStable100(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"update-cli-v3.3.4.zip", "update-cli-v0.8.23.zip", "update-cli-v1.0.0.zip"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, v, err := SelectNewest(dir, "update-cli")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := v.String(), "1.0.0"; got != want {
		t.Fatalf("newest version = %s, want %s", got, want)
	}
}
