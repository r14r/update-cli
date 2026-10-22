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

func TestUpdateCLISemVerTreatsHistorical3xAsNewer(t *testing.T) {
	legacy := mustVersion(t, "3.3.4")
	baseline := mustVersion(t, "0.8.0")
	if CompareForProject("update-cli", baseline, legacy) >= 0 {
		t.Fatal("semantic versioning orders 0.8.0 below 3.3.4")
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
	if got, want := v.String(), "3.3.4"; got != want {
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

func TestCompareForProjectUpdateCLIStable103IsNewerThan102(t *testing.T) {
	newer, _ := Parse("1.0.3")
	older, _ := Parse("1.0.2")
	if got := CompareForProject("update-cli", newer, older); got <= 0 {
		t.Fatalf("expected 1.0.3 > 1.0.2, got %d", got)
	}
}

func TestCompareForProjectUpdateCLIStable100IsOlderThanLegacy334(t *testing.T) {
	stable, _ := Parse("1.0.0")
	legacy, _ := Parse("3.3.4")
	if got := CompareForProject("update-cli", stable, legacy); got >= 0 {
		t.Fatalf("expected semver 1.0.0 < 3.3.4, got %d", got)
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
	if got, want := v.String(), "3.3.4"; got != want {
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
	if got, want := v.String(), "3.3.4"; got != want {
		t.Fatalf("newest version = %s, want %s", got, want)
	}
}

func TestCompareForProjectUpdateCLI130IsNewerThan121(t *testing.T) {
	newer, _ := Parse("1.3.0")
	older, _ := Parse("1.2.1")
	if got := CompareForProject("update-cli", newer, older); got <= 0 {
		t.Fatalf("expected 1.3.0 > 1.2.1, got %d", got)
	}
}

func TestCompareForProjectUpdateCLI2xIsNewerThan1x(t *testing.T) {
	newer := mustVersion(t, "2.2.0")
	older := mustVersion(t, "1.5.1")
	if got := CompareForProject("update-cli", newer, older); got <= 0 {
		t.Fatalf("expected 2.2.0 > 1.5.1, got %d", got)
	}
}

func TestUpdateCLIArchiveSelectionPrefersCurrent2x(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"update-cli-v3.3.4.zip", "update-cli-v0.8.23.zip", "update-cli-v1.5.1.zip", "update-cli-v2.1.0.zip", "update-cli-v2.2.0.zip"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, v, err := SelectNewest(dir, "update-cli")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := v.String(), "3.3.4"; got != want {
		t.Fatalf("newest version = %s, want %s", got, want)
	}
}

func TestParseArchiveNameAcceptsOptionalVPrefix(t *testing.T) {
	for _, name := range []string{"demo-v1.2.3.zip", "demo-1.2.3.zip"} {
		v, err := ParseArchiveName("demo", name)
		if err != nil {
			t.Fatalf("ParseArchiveName(%q): %v", name, err)
		}
		if got, want := v.String(), "1.2.3"; got != want {
			t.Fatalf("ParseArchiveName(%q) = %s, want %s", name, got, want)
		}
	}
}

func TestListArchivesAcceptsBothFilenameFormsAndSelectsNewest(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"demo-v1.2.3.zip",
		"demo-1.2.4.zip",
		"demo-v1.1.9.zip",
		"demo-v1.2.zip",
		"other-9.9.9.zip",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	archives, err := ListArchives(dir, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(archives), 3; got != want {
		t.Fatalf("archive count = %d, want %d", got, want)
	}
	if got, want := archives[0].Name, "demo-1.2.4.zip"; got != want {
		t.Fatalf("newest archive = %q, want %q", got, want)
	}
}

func TestUpdateCLIMajor3IsUpgradeFrom2170(t *testing.T) {
	installed := mustVersion(t, "2.17.0")
	selected := mustVersion(t, "3.0.1")
	if got := CompareForProject("update-cli", selected, installed); got <= 0 {
		t.Fatalf("3.0.1 must be newer than 2.17.0; compare=%d", got)
	}
	if got := CompareForProject("update-cli", installed, selected); got >= 0 {
		t.Fatalf("2.17.0 must be older than 3.0.1; compare=%d", got)
	}
}
