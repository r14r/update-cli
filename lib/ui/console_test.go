package ui

import (
	"strings"
	"testing"
)

func TestFormatStatusRowWithoutColor(t *testing.T) {
	got := formatStatusRow("Status", "Update verfügbar: 1.0.0 → 2.0.0", false)
	want := "  Status              Update verfügbar: 1.0.0 → 2.0.0\n"
	if got != want {
		t.Fatalf("unexpected status row:\nwant %q\n got %q", want, got)
	}
}

func TestFormatStatusRowUsesFullWidthBlueBackground(t *testing.T) {
	got := formatStatusRow("Status", "Update verfügbar", true)
	if !strings.HasPrefix(got, blueBackground+brightWhite+bold) {
		t.Fatalf("status row does not start with blue background and white foreground: %q", got)
	}
	if !strings.Contains(got, eraseToEnd+reset+"\n") {
		t.Fatalf("status row does not extend background to end of line: %q", got)
	}
}

func TestFormatBannerWithoutColor(t *testing.T) {
	got := formatBanner("Release Update     from 1.0.0 to 2.0.0", false)
	want := "\nRelease Update     from 1.0.0 to 2.0.0\n" + strings.Repeat("─", 72) + "\n"
	if got != want {
		t.Fatalf("unexpected banner:\nwant %q\n got %q", want, got)
	}
}

func TestFormatBannerUsesFullWidthBlueBackground(t *testing.T) {
	got := formatBanner("Release Update     from 1.0.0 to 2.0.0", true)
	if !strings.HasPrefix(got, "\n"+blueBackground+brightWhite+bold) {
		t.Fatalf("banner does not start with blue background and white foreground: %q", got)
	}
	if !strings.Contains(got, eraseToEnd+reset+"\n") {
		t.Fatalf("banner does not extend background to end of line: %q", got)
	}
}

func TestFormatErrorNoticeWithoutColor(t *testing.T) {
	got := formatErrorNotice("Version 2.2.1 ist bereits installiert", "Zur erneuten Installation --update --force verwenden", false)
	want := "\nVersion 2.2.1 ist bereits installiert\nZur erneuten Installation --update --force verwenden\n"
	if got != want {
		t.Fatalf("unexpected error notice:\nwant %q\n got %q", want, got)
	}
}

func TestFormatErrorNoticeUsesFullWidthRedBackground(t *testing.T) {
	got := formatErrorNotice("Version 2.2.1 ist bereits installiert", "weiter", true)
	if !strings.HasPrefix(got, "\n"+redBackground+brightWhite+bold) {
		t.Fatalf("error notice does not start with red background and white foreground: %q", got)
	}
	if !strings.Contains(got, eraseToEnd+reset+"\n") {
		t.Fatalf("error notice does not extend background to end of line: %q", got)
	}
	if strings.Contains(got, "FEHLER") {
		t.Fatalf("error notice contains forbidden FEHLER prefix: %q", got)
	}
}
