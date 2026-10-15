package updater

import (
	"context"
	"strings"
	"testing"
)

func TestWhatsNewShowsCurrentBuildVersion(t *testing.T) {
	out, err := captureStdout(func() error {
		return Run(context.Background(), "2.16.17", []string{"whatsnew"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "# What's new in Update CLI 2.16.17") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestWhatsNewShowsSelectedVersion(t *testing.T) {
	out, err := captureStdout(func() error {
		return Run(context.Background(), "2.16.17", []string{"whatsnew", "--version", "2.16.16"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "# What's new in Update CLI 2.16.16") {
		t.Fatalf("unexpected output: %q", out)
	}
}
