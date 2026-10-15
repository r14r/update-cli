package whatsnew

import (
	"strings"
	"testing"
)

func TestForVersionLoadsEmbeddedMarkdown(t *testing.T) {
	text, err := ForVersion("2.16.16")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "# What's new in Update CLI 2.16.16") {
		t.Fatalf("unexpected whatsnew text: %q", text)
	}
}

func TestForVersionAcceptsVPrefix(t *testing.T) {
	if _, err := ForVersion("v2.16.16"); err != nil {
		t.Fatal(err)
	}
}

func TestForVersionRejectsUnknownAndUnsafeVersions(t *testing.T) {
	for _, version := range []string{"2.16.999", "../2.16.16", "latest"} {
		if _, err := ForVersion(version); err == nil {
			t.Fatalf("ForVersion(%q) unexpectedly succeeded", version)
		}
	}
}
