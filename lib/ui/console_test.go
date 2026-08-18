package ui

import (
	"context"
	"io"
	"os"
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

func TestHeaderRendersVersionProjectAndPhaseSegments(t *testing.T) {
	line := headerBoxLine("Update CLI Version 0.8.10 — Versionsprüfung", "x-cli", 78, true)
	want := blueBackground + brightWhite + bold + " Update CLI Version 0.8.10   |   x-cli   |   Versionsprüfung"
	if !strings.Contains(line, want) {
		t.Fatalf("header does not render version/project/phase segments: %q", line)
	}
	if strings.Contains(line, whiteBackground) {
		t.Fatalf("project must no longer use an inverted white badge: %q", line)
	}
}

func TestProjectSegmentSurvivesFullscreenTitleChange(t *testing.T) {
	c := &Console{fullscreen: true, color: true, title: "Update CLI Version 0.8.10 — Versionsprüfung", project: "x-cli"}
	c.StartFullscreen("Update CLI Version 0.8.10 — Update")
	if c.project != "x-cli" {
		t.Fatalf("project segment was cleared during fullscreen phase change: %q", c.project)
	}
}

func TestHeaderTruncatesLongProjectBeforeLosingPhase(t *testing.T) {
	header := headerDisplayText("Update CLI Version 0.8.10 — Setup", "a-very-long-project-name-that-does-not-fit", 62)
	if !strings.Contains(header, "Update CLI Version 0.8.10") || !strings.HasSuffix(header, "Setup") {
		t.Fatalf("version or phase was lost while truncating project: %q", header)
	}
	if displayWidth(" "+header) > 62 {
		t.Fatalf("header exceeds available width: %d: %q", displayWidth(" "+header), header)
	}
}

func TestConfirmationModalColorsOnlyButtonContent(t *testing.T) {
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	defer func() { os.Stdout = oldStdout }()

	c := &Console{color: true}
	c.renderConfirmationModalLocked("Update jetzt installieren?", true, 100, 30)
	_ = writer.Close()
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)

	if strings.Contains(text, greenBackground+brightWhite+bold+"┌") || strings.Contains(text, greenBackground+brightWhite+bold+"│") {
		t.Fatalf("selected YES background must not color button borders: %q", text)
	}
	if !strings.Contains(text, "│"+greenBackground+brightWhite+bold) {
		t.Fatalf("selected YES content has no green background: %q", text)
	}
	if strings.Contains(text, redBackground+brightWhite+bold+"┌") || strings.Contains(text, redBackground+brightWhite+bold+"│") {
		t.Fatalf("unselected NO must not color button borders: %q", text)
	}
}

func TestFinalStatusLineIncludesCLIProjectAndInstalledVersion(t *testing.T) {
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	defer func() { os.Stdout = oldStdout }()

	c := New(true)
	c.SetApplicationVersion("0.8.13")
	c.SetFinalStatus("nvidia-cli", "Aktualisiert auf Version: v1.2.4")
	c.PrintFinalStatus()
	_ = writer.Close()
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(output))
	want := "Update CLI Version 0.8.13 | nvidia-cli | Aktualisiert auf Version: v1.2.4"
	if got != want {
		t.Fatalf("final status = %q, want %q", got, want)
	}
}

func TestFinalStatusCanBeSuppressedForMachineOutput(t *testing.T) {
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	defer func() { os.Stdout = oldStdout }()

	c := New(true)
	c.SetApplicationVersion("0.8.13")
	c.SetFinalStatus("demo", "Installierte Version: v1.0.0")
	c.SuppressFinalStatus(true)
	c.PrintFinalStatus()
	_ = writer.Close()
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 0 {
		t.Fatalf("suppressed final status produced output: %q", output)
	}
}

func TestInfoRowsAlignValuesAtSameColumn(t *testing.T) {
	release := infoRowText("Release Update", "from 1.0.0 to 1.0.1")
	project := infoRowText("Projekt", "DigitalProductsPlatform")
	source := infoRowText("Quelle", "/Users/Ralph.Goestenmeier/Downloads/example.zip")

	wantColumn := strings.Index(release, "from")
	if wantColumn < 0 {
		t.Fatalf("release value not found: %q", release)
	}
	if got := strings.Index(project, "DigitalProductsPlatform"); got != wantColumn {
		t.Fatalf("project value starts at column %d, want %d: %q", got, wantColumn, project)
	}
	if got := strings.Index(source, "/Users/"); got != wantColumn {
		t.Fatalf("source value starts at column %d, want %d: %q", got, wantColumn, source)
	}
}

func TestReleaseTargetVersionUsesBlueBackgroundOnlyForNewVersion(t *testing.T) {
	plain := infoRowText("Release Update", "from 1.0.0 to 1.0.1")
	rendered := strings.Replace(plain, "1.0.1", blueBackground+brightWhite+bold+"1.0.1"+reset, 1)
	line := styledPlainBoxLine(plain, rendered, 100)

	want := blueBackground + brightWhite + bold + "1.0.1" + reset
	if !strings.Contains(line, want) {
		t.Fatalf("new version is not highlighted: %q", line)
	}
	if strings.Contains(line, blueBackground+brightWhite+bold+"1.0.0") {
		t.Fatalf("old version must not be highlighted: %q", line)
	}
}

func TestDirectSetupStepGroupsCommandOutputWithGuide(t *testing.T) {
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	defer func() { os.Stdout = oldStdout }()

	c := &Console{direct: true}
	c.Task("prepare")
	err = c.Step(context.Background(), 0, 2, "Install Python dependencies", func() error {
		c.Append(`❯ python -m pip install -r requirements.txt`)
		stdout, _ := c.ProcessWriters()
		if _, err := io.WriteString(stdout, "INSTALL   Python dependencies ... OK\n"); err != nil {
			return err
		}
		if flusher, ok := stdout.(interface{ Flush() }); ok {
			flusher.Flush()
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	got := string(output)
	for _, want := range []string{
		directStepHeading("[01/02]", "Install Python dependencies"),
		"│  ❯ python -m pip install -r requirements.txt",
		"│  INSTALL   Python dependencies ... OK",
		"└─ ✓ Install Python dependencies",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("direct step output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Task: prepare") {
		t.Fatalf("direct setup output must not render task headings:\n%s", got)
	}
	if strings.Contains(got, "[01/02] ✓ Install Python dependencies") {
		t.Fatalf("completion should close the visual block rather than repeat the old flat step row:\n%s", got)
	}
}

func TestDirectSkippedStepUsesClosedVisualBlock(t *testing.T) {
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	defer func() { os.Stdout = oldStdout }()

	c := &Console{direct: true}
	c.SkipStep(1, 3, "Optional check", "nicht benötigt")
	_ = writer.Close()
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	got := string(output)
	if !strings.Contains(got, directStepHeading("[02/03]", "Optional check")) || !strings.Contains(got, "└─ – Optional check — nicht benötigt") {
		t.Fatalf("unexpected skipped-step rendering:\n%s", got)
	}
}

func TestDirectStepHeadingUsesInlineRule(t *testing.T) {
	got := directStepHeading("[04/09]", "Validate project")
	wantPrefix := "[04/09] Validate project "
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("unexpected heading prefix: %q", got)
	}
	if !strings.HasSuffix(got, "───") {
		t.Fatalf("step heading should end with a visible rule: %q", got)
	}
	if displayWidth(got) != 72 {
		t.Fatalf("short step heading width = %d, want 72: %q", displayWidth(got), got)
	}
}

func TestFullscreenSetupMetadataUsesStepOutputGutter(t *testing.T) {
	if got, want := setupMetaText(14, "Projekt: Update CLI"), "        │ Projekt: Update CLI"; got != want {
		t.Fatalf("setup project metadata = %q, want %q", got, want)
	}
	if got, want := setupMetaText(14, "Schema: 2 | Tasks: 5 | Schritte: 14"), "        │ Schema: 2 | Tasks: 5 | Schritte: 14"; got != want {
		t.Fatalf("setup schema metadata = %q, want %q", got, want)
	}
	// The gutter grows with three-digit step counters instead of relying on a
	// hard-coded eight-space indent.
	if got, want := setupMetaText(120, "Projekt: Large"), "         │ Projekt: Large"; got != want {
		t.Fatalf("three-digit setup metadata = %q, want %q", got, want)
	}
}

func TestFullscreenSetupStepOutputUsesAlignedGutter(t *testing.T) {
	c := &Console{fullscreen: true, color: false, title: "Update CLI Version 0.8.19 — Setup", stepOutputIndent: 8}
	c.appendStepOutputLocked("      VALIDATE  105 JSON + 54 Typst sources + 5 fact topics ... OK", false)
	c.appendStepOutputLocked("        CHECK     typst 0.15.1 (unknown commit) ... OK", false)
	c.appendStepOutputLocked("   broken", true)

	want := []string{
		"        │ VALIDATE  105 JSON + 54 Typst sources + 5 fact topics ... OK",
		"        │ CHECK     typst 0.15.1 (unknown commit) ... OK",
		"        │ ! broken",
	}
	if len(c.content) != len(want) {
		t.Fatalf("content lines=%d, want %d: %#v", len(c.content), len(want), c.content)
	}
	for i, expected := range want {
		if c.content[i].text != expected {
			t.Fatalf("line %d = %q, want %q", i, c.content[i].text, expected)
		}
		if c.content[i].kind != "step-output" {
			t.Fatalf("line %d kind=%q, want step-output", i, c.content[i].kind)
		}
	}
}

func TestFullscreenStepOutputWrapKeepsHangingIndent(t *testing.T) {
	line := "        │ " + strings.Repeat("x", 80)
	parts := wrapStepOutput(line, 40)
	if len(parts) < 2 {
		t.Fatalf("expected wrapped output, got %#v", parts)
	}
	indent := strings.Repeat(" ", 10)
	for i, part := range parts[1:] {
		if !strings.HasPrefix(part, indent) {
			t.Fatalf("continuation %d not aligned: %q", i+1, part)
		}
	}
}

func TestSuccessBannerUsesGreenBackgroundAcrossContentRow(t *testing.T) {
	line := statusBoxLine("Version 1.0.3 ist bereits installiert", "success-banner", 78, true)
	want := greenBackground + brightWhite + bold + " Version 1.0.3 ist bereits installiert"
	if !strings.Contains(line, want) {
		t.Fatalf("success banner has no green content background: %q", line)
	}
	if strings.HasPrefix(line, greenBackground) || strings.HasSuffix(line, greenBackground) {
		t.Fatalf("success banner must keep box borders neutral: %q", line)
	}
	if got := displayWidth(stripANSIForTest(line)); got != 80 {
		t.Fatalf("success banner box width = %d, want 80: %q", got, line)
	}
}

func TestFinishFooterCanUseNormalUpdateCloseState(t *testing.T) {
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	defer func() { os.Stdout = oldStdout }()

	c := &Console{fullscreen: true, color: false, title: "Update CLI Version 0.8.18 — Update"}
	c.SetFinishFooter("Update beenden")
	c.FinishFullscreen(true, false)
	_ = writer.Close()
	_, _ = io.ReadAll(reader)

	if c.footer != "Update beenden" {
		t.Fatalf("finish footer = %q", c.footer)
	}
	if c.footerKind != "run" {
		t.Fatalf("finish footer kind = %q, want run", c.footerKind)
	}
}

func stripANSIForTest(s string) string {
	for {
		start := strings.IndexByte(s, '\x1b')
		if start < 0 {
			return s
		}
		end := start + 1
		for end < len(s) && s[end] != 'm' {
			end++
		}
		if end >= len(s) {
			return s[:start]
		}
		s = s[:start] + s[end+1:]
	}
}

func TestProjectHeaderSegmentIncludesInstalledVersion(t *testing.T) {
	got := projectHeaderSegment("life-os", "0.1.1")
	if got != "life-os v0.1.1" {
		t.Fatalf("unexpected project/version header segment: %q", got)
	}
	if got := projectHeaderSegment("life-os", "v0.1.1"); got != "life-os v0.1.1" {
		t.Fatalf("version prefix must not be duplicated: %q", got)
	}
	if got := projectHeaderSegment("life-os", ""); got != "life-os" {
		t.Fatalf("empty version must preserve project-only header: %q", got)
	}
}

func TestProjectVersionSurvivesFullscreenTitleChange(t *testing.T) {
	c := &Console{
		fullscreen:     true,
		color:          false,
		title:          "Update CLI Version 0.8.23 — Versionsprüfung",
		project:        "life-os",
		projectVersion: "0.1.1",
	}
	c.StartFullscreen("Update CLI Version 0.8.23 — Update")
	if c.projectVersion != "0.1.1" {
		t.Fatalf("project version was cleared during fullscreen phase change: %q", c.projectVersion)
	}
	header := headerDisplayText(c.title, projectHeaderSegment(c.project, c.projectVersion), 100)
	if !strings.Contains(header, "life-os v0.1.1") {
		t.Fatalf("header does not contain project version after phase change: %q", header)
	}
}
