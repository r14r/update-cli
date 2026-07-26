package version

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectNewestUsesNumericSemVer(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"demo-v1.2.9.zip",
		"demo-v1.10.0.zip",
		"other-v9.0.0.zip",
		"demo-v2.0.0-rc.1.zip",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	path, selected, err := SelectNewest(dir, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if selected.String() != "1.10.0" || filepath.Base(path) != "demo-v1.10.0.zip" {
		t.Fatalf("unexpected selection: %s %s", path, selected)
	}
}

func TestRejectsSuffixes(t *testing.T) {
	if _, err := ParseArchiveName("demo", "demo-v1.2.3_001.zip"); err == nil {
		t.Fatal("expected suffix to be rejected")
	}
}

func TestParseSemanticVersion(t *testing.T) {
	parsed, err := Parse("v12.3.40\n")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.String() != "12.3.40" {
		t.Fatalf("unexpected version: %s", parsed)
	}
}
