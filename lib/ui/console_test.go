package ui

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestStepDoesNotReturnBeforeActionOnCancellation(t *testing.T) {
	c := &Console{interactive: true}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	err := c.Step(ctx, 0, 1, "slow", func() error { time.Sleep(60 * time.Millisecond); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(started) < 50*time.Millisecond {
		t.Fatal("Step returned before action finished")
	}
}

func TestBoxAlignmentUsesRuneWidthForUmlauts(t *testing.T) {
	line := plainBoxLine("Überprüfung äöü", 38)
	if got, want := displayWidth(line), 40; got != want {
		t.Fatalf("box width mismatch: got %d want %d: %q", got, want, line)
	}
}

func TestWrapDisplayPreservesUmlauts(t *testing.T) {
	parts := wrapDisplay("ÄÖÜabcdef", 4)
	if len(parts) != 3 || parts[0] != "ÄÖÜa" || parts[1] != "bcde" || parts[2] != "f" {
		t.Fatalf("unexpected wrapped output: %#v", parts)
	}
}

func TestConfirmSuffixUsesGermanJaNeinNotation(t *testing.T) {
	if got := confirmSuffix(false); got != " [j/N]" {
		t.Fatalf("default-no suffix = %q", got)
	}
	if got := confirmSuffix(true); got != " [J/n]" {
		t.Fatalf("default-yes suffix = %q", got)
	}
}

func TestStatusBoxLineUsesGreenSuccessIcon(t *testing.T) {
	line := statusBoxLine("[1/8] Go-Module laden", "ok", 58, true)
	if !strings.Contains(line, green+bold+"✓"+reset) {
		t.Fatalf("success row has no green check icon: %q", line)
	}
	if strings.Contains(line, "abgeschlossen") {
		t.Fatalf("success row must not emit a second completion message: %q", line)
	}
}

func TestProgressBarTextUsesFixedColumns(t *testing.T) {
	first := progressBarText(3, 13, "Release-Inhalt und Archiv validieren", 96)
	second := progressBarText(5, 13, "Transaktion vorbereiten und Snapshot erstellen", 96)
	if displayWidth(first) != 96 || displayWidth(second) != 96 {
		t.Fatalf("progress rows must fill requested width: %d / %d", displayWidth(first), displayWidth(second))
	}
	if strings.Index(first, "%") != strings.Index(second, "%") {
		t.Fatalf("percentage columns are not aligned:\n%q\n%q", first, second)
	}
	if !strings.HasPrefix(first, "[03/13] [") || !strings.Contains(first, "]  23%  ") {
		t.Fatalf("unexpected progress row: %q", first)
	}
}

func TestStepCounterUsesTwoDigits(t *testing.T) {
	if got, want := stepCounter(1, 8), "[01/08]"; got != want {
		t.Fatalf("step counter = %q, want %q", got, want)
	}
	if got, want := stepCounter(13, 13), "[13/13]"; got != want {
		t.Fatalf("step counter = %q, want %q", got, want)
	}
}

func TestFullscreenLogLineDoesNotOverwriteFooter(t *testing.T) {
	c := &Console{fullscreen: true, footer: "RUN  03/08 Go-Tests ausführen", footerKind: "run"}
	c.line(io.Discard, blue, "INFO", "child output")
	if c.footer != "RUN  03/08 Go-Tests ausführen" || c.footerKind != "run" {
		t.Fatalf("content output overwrote footer: %q (%s)", c.footer, c.footerKind)
	}
	if len(c.content) != 1 || !strings.Contains(c.content[0].text, "child output") {
		t.Fatalf("content output not retained in scroll region: %#v", c.content)
	}
}

func TestDirectModeDisablesFullscreen(t *testing.T) {
	c := &Console{interactive: true, color: true}
	c.SetDirect(true)
	if c.StartFullscreen("Update CLI Setup") {
		t.Fatal("direct mode must not enter fullscreen TUI")
	}
	if !c.Direct() {
		t.Fatal("direct mode was not retained")
	}
}

func TestStepAndProgressRowsNeverOverwriteFullscreenFooter(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*Console) error
	}{
		{name: "setup-step", run: func(c *Console) error {
			return c.Step(context.Background(), 0, 1, "Go-Tests ausführen", func() error { return nil })
		}},
		{name: "update-step", run: func(c *Console) error {
			return c.ProgressStep(context.Background(), 0, 13, "Release-Quelle auflösen", func() error { return nil })
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &Console{fullscreen: true, footer: "RUN  Update läuft", footerKind: "run"}
			if err := tc.run(c); err != nil {
				t.Fatal(err)
			}
			if c.footer != "RUN  Update läuft" || c.footerKind != "run" {
				t.Fatalf("step renderer overwrote screen footer: %q (%s)", c.footer, c.footerKind)
			}
		})
	}
}

func TestSkippedRowsNeverOverwriteFullscreenFooter(t *testing.T) {
	c := &Console{fullscreen: true, footer: "RUN  Update läuft", footerKind: "run"}
	c.SkipProgressStep(5, 13, "Backup", "nicht angefordert")
	c.SkipStep(0, 1, "Optional", "nicht benötigt")
	if c.footer != "RUN  Update läuft" || c.footerKind != "run" {
		t.Fatalf("skip renderer overwrote screen footer: %q (%s)", c.footer, c.footerKind)
	}
}

func TestStatusRowNeverOverwritesFullscreenFooter(t *testing.T) {
	c := &Console{fullscreen: true, footer: "RUN  Versionsprüfung läuft", footerKind: "run"}
	c.StatusRow("Status", "update-available")
	if c.footer != "RUN  Versionsprüfung läuft" || c.footerKind != "run" {
		t.Fatalf("status row overwrote screen footer: %q (%s)", c.footer, c.footerKind)
	}
}

func TestClearContentPreservesFullscreenFrameState(t *testing.T) {
	c := &Console{
		fullscreen: true,
		title:      "Update CLI — Update",
		footer:     "RUN  Update läuft",
		footerKind: "run",
		infoTitle:  "Release Update",
		info:       []string{"Projekt             demo"},
		content: []screenLine{
			{text: "[01/13] Release-Quelle auflösen", kind: "ok"},
			{text: "[02/13] Zielversion prüfen", kind: "ok"},
		},
		errorShown: true,
	}

	c.ClearContent()

	if len(c.content) != 0 {
		t.Fatalf("content was not cleared: %#v", c.content)
	}
	if c.title != "Update CLI — Update" || c.infoTitle != "Release Update" || len(c.info) != 1 {
		t.Fatalf("frame metadata changed while clearing content: %#v", c)
	}
	if c.footer != "RUN  Update läuft" || c.footerKind != "run" {
		t.Fatalf("footer changed while clearing content: %q (%s)", c.footer, c.footerKind)
	}
	if c.errorShown {
		t.Fatal("content error state was not reset")
	}
}
