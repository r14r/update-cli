package ui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	reset           = "\033[0m"
	bold            = "\033[1m"
	dim             = "\033[2m"
	red             = "\033[31m"
	green           = "\033[32m"
	yellow          = "\033[33m"
	blue            = "\033[34m"
	cyan            = "\033[36m"
	brightWhite     = "\033[97m"
	blueBackground  = "\033[44m"
	greenBackground = "\033[42m"
	redBackground   = "\033[41m"
	whiteBackground = "\033[47m"
	eraseToEnd      = "\033[K"
)

type screenLine struct {
	text     string
	kind     string
	progress bool
	step     int
	total    int
	label    string
}

type Console struct {
	color       bool
	interactive bool
	direct      bool
	mu          sync.Mutex

	fullscreen          bool
	title               string
	project             string
	projectVersion      string
	footer              string
	footerKind          string
	finishFooter        string
	finishFooterKind    string
	appVersion          string
	finalProject        string
	finalStatus         string
	suppressFinalStatus bool
	infoTitle           string
	info                []string
	infoHighlight       []string
	content             []screenLine
	details             bool
	errorShown          bool
	directStep          bool
	stepOutputIndent    int
}

func New(noColor bool) *Console {
	interactive := isTerminal(os.Stdout) && isTerminal(os.Stdin)
	return &Console{color: !noColor && os.Getenv("NO_COLOR") == "" && isTerminal(os.Stdout), interactive: interactive}
}

func (c *Console) style(s string) string {
	if c.color {
		return s
	}
	return ""
}

func (c *Console) SetApplicationVersion(version string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.appVersion = strings.TrimSpace(version)
}

func (c *Console) SetFinalStatus(project, status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.finalProject = strings.TrimSpace(project)
	c.finalStatus = strings.TrimSpace(status)
}

func (c *Console) SuppressFinalStatus(suppress bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.suppressFinalStatus = suppress
}

func (c *Console) PrintFinalStatus() {
	c.mu.Lock()
	version := strings.TrimSpace(c.appVersion)
	project := strings.TrimSpace(c.finalProject)
	status := strings.TrimSpace(c.finalStatus)
	suppress := c.suppressFinalStatus
	c.finalStatus = ""
	c.mu.Unlock()
	if suppress {
		return
	}
	if status == "" {
		return
	}
	if version == "" {
		version = "dev"
	}
	if project == "" {
		project = "-"
	}
	fmt.Fprintf(os.Stdout, "Update CLI Version %s | %s | %s\n", version, project, status)
}

func (c *Console) SetDirect(direct bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.direct = direct
}

func (c *Console) Direct() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.direct
}

func (c *Console) SetDetails(details bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.details = details
}

func (c *Console) Details() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.details
}

func (c *Console) StartFullscreen(title string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.direct {
		return false
	}
	if c.fullscreen {
		if strings.TrimSpace(title) != "" {
			if title != c.title {
				c.infoTitle = ""
				c.info = nil
				c.infoHighlight = nil
				c.content = nil
				c.errorShown = false
				c.footer = "RUN  Vorbereitung"
				c.footerKind = "run"
			}
			c.title = title
			c.renderFullscreenLocked()
		}
		return true
	}
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("UPDATE_CLI_TUI")))
	if mode == "" {
		mode = "auto"
	}
	if mode == "plain" || mode == "off" || mode == "0" || !c.interactive || !c.color {
		return false
	}
	if mode != "auto" && mode != "fullscreen" && mode != "full" && mode != "1" {
		return false
	}
	c.fullscreen = true
	c.title = title
	c.footer = "RUN  Vorbereitung"
	c.footerKind = "run"
	c.infoTitle = ""
	c.info = nil
	c.infoHighlight = nil
	c.content = nil
	c.errorShown = false
	fmt.Fprint(os.Stdout, "\033[?1049h\033[?7l\033[?25l\033[2J\033[H")
	c.renderFullscreenLocked()
	return true
}

func (c *Console) Fullscreen() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.fullscreen
}

func (c *Console) Title() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.title
}

// SetProjectName sets the project segment rendered in the fullscreen header.
// It is intentionally independent from the info box so the current project
// remains visible while the content region changes between check, update and
// setup phases.
func (c *Console) SetProjectName(project string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.project = strings.TrimSpace(project)
	if c.fullscreen {
		c.renderFullscreenLocked()
	}
}

// SetProjectVersion sets the currently installed project version rendered next
// to the project name in the fullscreen header. An empty version keeps the
// historic project-name-only presentation, which is useful before a first
// installation or for standalone setup manifests without a VERSION file.
func (c *Console) SetProjectVersion(version string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.projectVersion = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(version), "v"))
	if c.fullscreen {
		c.renderFullscreenLocked()
	}
}

func projectHeaderSegment(project, version string) string {
	project = strings.TrimSpace(project)
	version = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(version), "v"))
	if project == "" || version == "" {
		return project
	}
	return project + " v" + version
}

func (c *Console) SetFooter(status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.fullscreen {
		return
	}
	c.footer = status
	c.footerKind = "run"
	c.renderFullscreenLocked()
}

func (c *Console) SetFooterSuccess(status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.fullscreen {
		return
	}
	c.footer = status
	c.footerKind = "ok"
	c.renderFullscreenLocked()
}

// SetFinishFooter sets a normal, non-error footer that is preserved when the
// fullscreen screen closes. This is used for successful no-op outcomes such as
// selecting a release version that is already installed.
func (c *Console) SetFinishFooter(status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	status = strings.TrimSpace(status)
	if status == "" {
		return
	}
	c.finishFooter = status
	c.finishFooterKind = "run"
	if c.fullscreen {
		c.footer = status
		c.footerKind = "run"
		c.renderFullscreenLocked()
	}
}

// ClearContent resets only the scrollable content region of the fullscreen TUI.
// Header, project/setup information and footer remain unchanged. This is used
// when one interactive phase hands off to another (for example update -> setup)
// so setup starts with a clean viewport without losing the surrounding context.
func (c *Console) ClearContent() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.fullscreen {
		return
	}
	c.content = nil
	c.errorShown = false
	c.renderFullscreenLocked()
}

// FinishFullscreen restores the terminal. When wait is true, the fullscreen
// result remains visible until Enter is pressed, matching Update CLI 2.x.
func (c *Console) FinishFullscreen(success bool, wait bool) {
	c.mu.Lock()
	if !c.fullscreen {
		c.mu.Unlock()
		return
	}
	if success {
		if strings.TrimSpace(c.finishFooter) != "" {
			c.footer = c.finishFooter
			c.footerKind = c.finishFooterKind
		} else {
			c.footer = "OK   Abgeschlossen"
			c.footerKind = "ok"
		}
	} else {
		c.footer = "FAIL Vorgang fehlgeschlagen"
		c.footerKind = "error"
	}
	if wait {
		c.footer += " | Enter zum Schließen"
	}
	c.renderFullscreenLocked()
	c.mu.Unlock()

	if wait {
		fmt.Fprint(os.Stdout, "\033[?25h")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	}

	c.mu.Lock()
	title := c.title
	footer := c.footer
	c.fullscreen = false
	fmt.Fprint(os.Stdout, "\033[?25h\033[?7h\033[?1049l")
	c.mu.Unlock()

	// Keep a compact summary in scrollback after leaving the alternate screen.
	// When the caller has registered a richer final status, that status is
	// printed by PrintFinalStatus after the alternate screen has been restored.
	c.mu.Lock()
	hasFinalStatus := strings.TrimSpace(c.finalStatus) != ""
	c.mu.Unlock()
	if !hasFinalStatus && strings.TrimSpace(title) != "" {
		fmt.Fprintf(os.Stdout, "%s — %s\n", title, strings.Split(footer, " | ")[0])
	}
}

func (c *Console) SetInfoTitle(title string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.fullscreen {
		return
	}
	c.infoTitle = strings.TrimSpace(title)
	c.renderFullscreenLocked()
}

func (c *Console) InfoRow(label, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.fullscreen {
		fmt.Fprintf(os.Stdout, "  %s%-19s%s %s\n", c.style(dim), label, c.style(reset), value)
		return
	}
	c.info = append(c.info, infoRowText(label, value))
	c.infoHighlight = append(c.infoHighlight, "")
	c.renderFullscreenLocked()
}

// InfoHighlightedRow renders an ordinary aligned information row while
// highlighting one value fragment. The label/value column layout remains
// identical to InfoRow; only the requested value fragment receives the blue
// background used elsewhere for active Update CLI information.
func (c *Console) InfoHighlightedRow(label, value, highlight string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.fullscreen {
		if c.color && strings.TrimSpace(highlight) != "" {
			rendered := strings.Replace(value, highlight, blueBackground+brightWhite+bold+highlight+reset, 1)
			fmt.Fprintf(os.Stdout, "  %s%-19s%s %s\n", c.style(dim), label, c.style(reset), rendered)
			return
		}
		fmt.Fprintf(os.Stdout, "  %s%-19s%s %s\n", c.style(dim), label, c.style(reset), value)
		return
	}
	c.info = append(c.info, infoRowText(label, value))
	c.infoHighlight = append(c.infoHighlight, strings.TrimSpace(highlight))
	c.renderFullscreenLocked()
}

func infoRowText(label, value string) string {
	return fmt.Sprintf("%-19s %s", label, value)
}

func (c *Console) Header(t string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fullscreen {
		c.appendLocked("")
		c.appendLocked(t)
		c.renderFullscreenLocked()
		return
	}
	fmt.Fprintf(os.Stdout, "\n%s%s%s\n%s\n", c.style(bold+cyan), t, c.style(reset), strings.Repeat("─", 72))
}

func (c *Console) Banner(t string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fullscreen {
		c.appendLocked("")
		c.appendLocked(t)
		c.renderFullscreenLocked()
		return
	}
	if c.color {
		fmt.Fprintf(os.Stdout, "\n%s  %s%s%s\n", blueBackground+brightWhite+bold, t, eraseToEnd, reset)
	} else {
		fmt.Fprintf(os.Stdout, "\n%s\n%s\n", t, strings.Repeat("─", 72))
	}
}

func (c *Console) Row(l, v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fullscreen {
		c.appendLocked(fmt.Sprintf("%-19s %s", l, v))
		c.renderFullscreenLocked()
		return
	}
	fmt.Fprintf(os.Stdout, "  %s%-19s%s %s\n", c.style(dim), l, c.style(reset), v)
}

func (c *Console) StatusRow(l, v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fullscreen {
		c.appendLocked(fmt.Sprintf("%-19s %s", l, v))
		c.renderFullscreenLocked()
		return
	}
	if c.color {
		fmt.Fprintf(os.Stdout, "%s  %-19s %s%s%s\n", blueBackground+brightWhite+bold, l, v, eraseToEnd, reset)
	} else {
		fmt.Fprintf(os.Stdout, "  %-19s %s\n", l, v)
	}
}

func (c *Console) line(f io.Writer, col, level, msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.direct && c.directStep {
		fmt.Fprintf(f, "│  %s%s%-5s%s %s\n", c.style(col), c.style(bold), level, c.style(reset), msg)
		return
	}
	if c.fullscreen {
		// Informational/process output belongs to the active setup step. While a
		// step is running, keep every line behind the same visual output gutter so
		// command output cannot drift horizontally based on child-process padding.
		text := fmt.Sprintf("%-5s %s", level, msg)
		if c.stepOutputIndent > 0 {
			c.appendStepOutputLocked(text, f == os.Stderr)
		} else {
			c.appendLocked(text)
		}
		c.renderFullscreenLocked()
		return
	}
	fmt.Fprintf(f, "%s%s%-5s%s %s\n", c.style(col), c.style(bold), level, c.style(reset), msg)
}

func (c *Console) Interactive() bool { return c.interactive }

func (c *Console) Confirm(prompt string, defaultYes bool) (bool, error) {
	if !c.interactive {
		return false, fmt.Errorf("interaktive Bestätigung ist ohne Terminal nicht verfügbar")
	}
	suffix := confirmSuffix(defaultYes)

	c.mu.Lock()
	fullscreen := c.fullscreen
	width, height := 0, 0
	if fullscreen {
		width, height = terminalSize(os.Stdout)
		c.renderFullscreenLockedWithSize(width, height)
		c.renderConfirmationModalLocked(prompt, defaultYes, width, height)
	} else {
		fmt.Fprintf(os.Stdout, "%s  %s%s %s", c.style(blueBackground+brightWhite+bold), prompt, suffix, c.style(reset))
	}
	c.mu.Unlock()

	var answer bool
	var err error
	if fullscreen {
		answer, err = c.readFullscreenConfirmation(prompt, defaultYes, width, height)
	} else {
		answer, err = readLineConfirmation(defaultYes)
	}
	if err != nil {
		return false, err
	}

	if fullscreen {
		c.mu.Lock()
		// Repaint the underlying screen. The modal never owns footer state, so
		// answering a question cannot replace the current high-level status.
		fmt.Fprint(os.Stdout, reset+"\033[?25l")
		c.renderFullscreenLocked()
		c.mu.Unlock()
	}
	return answer, nil
}

func readLineConfirmation(defaultYes bool) (bool, error) {
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer == "" {
		return defaultYes, nil
	}
	switch answer {
	case "y", "yes", "j", "ja", "left", "links", "\x1b[d", "\x1b[D":
		return true, nil
	case "n", "no", "nein", "right", "rechts", "\x1b[c", "\x1b[C":
		return false, nil
	default:
		return false, nil
	}
}

// readFullscreenConfirmation puts the terminal into character-at-a-time mode so
// LEFT/RIGHT can visibly move the selection before Enter confirms it. If stty
// is unavailable, it falls back to the line-based confirmation behavior.
func (c *Console) readFullscreenConfirmation(prompt string, defaultYes bool, width, height int) (bool, error) {
	state, raw := enableCharacterInput()
	if !raw {
		return readLineConfirmation(defaultYes)
	}
	defer restoreCharacterInput(state)

	selectedYes := defaultYes
	reader := bufio.NewReader(os.Stdin)
	redraw := func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.renderConfirmationModalLocked(prompt, selectedYes, width, height)
	}

	for {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				return selectedYes, nil
			}
			return false, err
		}
		switch b {
		case '\r', '\n', ' ':
			return selectedYes, nil
		case 'j', 'J', 'y', 'Y':
			return true, nil
		case 'n', 'N':
			return false, nil
		case '\t':
			selectedYes = !selectedYes
			redraw()
		case 0x1b:
			second, err := reader.ReadByte()
			if err != nil {
				continue
			}
			if second != '[' && second != 'O' {
				continue
			}
			third, err := reader.ReadByte()
			if err != nil {
				continue
			}
			switch third {
			case 'D': // LEFT -> YES
				if !selectedYes {
					selectedYes = true
					redraw()
				}
			case 'C': // RIGHT -> NO
				if selectedYes {
					selectedYes = false
					redraw()
				}
			}
		}
	}
}

func enableCharacterInput() (string, bool) {
	if _, err := exec.LookPath("stty"); err != nil {
		return "", false
	}
	get := exec.Command("stty", "-g")
	get.Stdin = os.Stdin
	out, err := get.Output()
	if err != nil {
		return "", false
	}
	state := strings.TrimSpace(string(out))
	if state == "" {
		return "", false
	}
	set := exec.Command("stty", "-echo", "-icanon", "min", "1", "time", "0")
	set.Stdin = os.Stdin
	if err := set.Run(); err != nil {
		return "", false
	}
	return state, true
}

func restoreCharacterInput(state string) {
	if strings.TrimSpace(state) == "" {
		return
	}
	cmd := exec.Command("stty", state)
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}

// renderConfirmationModalLocked overlays a centered confirmation dialog on top
// of the existing fullscreen screen. It intentionally does not mutate content,
// info or footer state; once input is processed the base screen is simply
// repainted.
func (c *Console) renderConfirmationModalLocked(prompt string, selectedYes bool, width, height int) {
	modalWidth := minInt(76, maxInt(44, width-12))
	if modalWidth > width-4 {
		modalWidth = maxInt(36, width-4)
	}
	inner := modalWidth - 2
	questionWidth := maxInt(12, inner-4)
	questionLines := wrapDisplay(prompt, questionWidth)
	if len(questionLines) > 2 {
		questionLines = questionLines[:2]
	}

	buttonWidth := 17
	buttonGap := 7
	buttonsWidth := buttonWidth*2 + buttonGap
	if buttonsWidth > inner-4 {
		buttonWidth = maxInt(11, (inner-8)/2)
		buttonGap = 4
		buttonsWidth = buttonWidth*2 + buttonGap
	}

	// top + blank + question + blank + 3 button rows + hint + bottom
	modalHeight := 8 + len(questionLines)
	top := maxInt(2, (height-modalHeight)/2+1)
	left := maxInt(2, (width-modalWidth)/2+1)

	yesStyle := green + bold
	noStyle := red + bold
	if selectedYes {
		yesStyle = greenBackground + brightWhite + bold
	} else {
		noStyle = redBackground + brightWhite + bold
	}

	center := func(text string, available int) string {
		text = truncateDisplay(text, available)
		gap := maxInt(0, available-displayWidth(text))
		return strings.Repeat(" ", gap/2) + text + strings.Repeat(" ", gap-gap/2)
	}
	buttonTop := "┌" + strings.Repeat("─", buttonWidth-2) + "┐"
	buttonBottom := "└" + strings.Repeat("─", buttonWidth-2) + "┘"
	buttonLine := func(label, style string) string {
		inside := center(label, buttonWidth-2)
		// Keep button borders neutral, like the fullscreen header/footer.
		// Only the content area carries the green/red selection styling.
		return "│" + c.style(style) + inside + c.style(reset) + "│"
	}

	lines := make([]string, 0, modalHeight)
	lines = append(lines, "┌"+strings.Repeat("─", inner)+"┐")
	lines = append(lines, "│"+center("Bestätigung", inner)+"│")
	for _, q := range questionLines {
		lines = append(lines, "│"+center(q, inner)+"│")
	}
	lines = append(lines, "│"+strings.Repeat(" ", inner)+"│")
	buttonPad := maxInt(0, (inner-buttonsWidth)/2)
	rightPad := maxInt(0, inner-buttonPad-buttonsWidth)
	for row, pair := range [][2]string{
		{buttonTop, buttonTop},
		{buttonLine("YES", yesStyle), buttonLine("NO", noStyle)},
		{buttonBottom, buttonBottom},
	} {
		_ = row
		lines = append(lines, "│"+strings.Repeat(" ", buttonPad)+pair[0]+strings.Repeat(" ", buttonGap)+pair[1]+strings.Repeat(" ", rightPad)+"│")
	}
	hint := "←/→ = auswählen   ·   Enter = bestätigen   ·   j/y = YES   ·   n = NO"
	lines = append(lines, "│"+center(hint, inner)+"│")
	lines = append(lines, "└"+strings.Repeat("─", inner)+"┘")

	for i, line := range lines {
		fmt.Fprintf(os.Stdout, "\033[%d;%dH%s", top+i, left, line)
	}
	fmt.Fprint(os.Stdout, "\033[?25l")
}

func confirmSuffix(defaultYes bool) string {
	if defaultYes {
		return " [J/n]"
	}
	return " [j/N]"
}

func (c *Console) Info(s string)    { c.line(os.Stdout, blue, "INFO", s) }
func (c *Console) Warn(s string)    { c.line(os.Stderr, yellow, "WARN", s) }
func (c *Console) Success(s string) { c.line(os.Stdout, green, "OK", s) }

// SuccessBanner renders a successful informational outcome prominently without
// classifying it as an error. In fullscreen mode the content area receives a
// green background; direct/plain output keeps the message readable with or
// without ANSI color support.
func (c *Console) SuccessBanner(message string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	if c.fullscreen {
		c.content = append(c.content, screenLine{text: message, kind: "success-banner"})
		c.renderFullscreenLocked()
		return
	}
	if c.color {
		fmt.Fprintf(os.Stdout, "\n%s  %s%s%s\n", greenBackground+brightWhite+bold, message, eraseToEnd, reset)
		return
	}
	fmt.Fprintf(os.Stdout, "\nOK    %s\n", message)
}

func (c *Console) Diagnostic(status, label, detail string) {
	col, mark := green, "OK"
	if status == "warning" {
		col, mark = yellow, "WARN"
	}
	if status == "error" {
		col, mark = red, "FAIL"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fullscreen {
		c.appendLocked(fmt.Sprintf("%-5s %-22s %s", mark, truncate(label, 22), detail))
		c.renderFullscreenLocked()
		return
	}
	fmt.Fprintf(os.Stdout, "  %s%-5s%s %-22s %s\n", c.style(col+bold), mark, c.style(reset), truncate(label, 22), detail)
}

func (c *Console) ErrorNotice(title, detail string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errorShown = true
	if c.fullscreen {
		c.appendLocked("FAIL  " + title)
		if strings.TrimSpace(detail) != "" {
			c.appendLocked("      " + detail)
		}
		c.footer = "FAIL " + title
		c.footerKind = "error"
		c.renderFullscreenLocked()
		return
	}
	if c.color {
		fmt.Fprintf(os.Stderr, "\n%s  %s%s%s\n  %s\n", redBackground+brightWhite+bold, title, eraseToEnd, reset, detail)
	} else {
		fmt.Fprintf(os.Stderr, "\n%s\n%s\n", title, detail)
	}
}

func (c *Console) ErrorShown() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.errorShown
}

func (c *Console) Step(ctx context.Context, done, total int, label string, action func() error) error {
	if c.Direct() {
		counter := stepCounter(done+1, total)
		fmt.Fprintf(os.Stdout, "\n%s\n", directStepHeading(counter, label))
		c.mu.Lock()
		c.directStep = true
		c.mu.Unlock()
		err := action()
		c.mu.Lock()
		c.directStep = false
		c.mu.Unlock()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s└─ ✗ %s%s\n", c.style(red+bold), label, c.style(reset))
			return err
		}
		fmt.Fprintf(os.Stdout, "%s└─ ✓ %s%s\n", c.style(green+bold), label, c.style(reset))
		return nil
	}
	if c.Fullscreen() {
		c.mu.Lock()
		index := len(c.content)
		counter := stepCounter(done+1, total)
		c.stepOutputIndent = displayWidth(counter) + 1
		c.content = append(c.content, screenLine{text: fmt.Sprintf("%s %s", counter, label), kind: "run"})
		c.renderFullscreenLocked()
		c.mu.Unlock()
		stop := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(90 * time.Millisecond)
			defer ticker.Stop()
			frames := []rune{'|', '/', '-', '\\'}
			i := 0
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					c.renderProgressLocked(index, done, total, label, frames[i%len(frames)])
					i++
				}
			}
		}()
		err := action()
		close(stop)
		wg.Wait()
		c.mu.Lock()
		c.stepOutputIndent = 0
		c.mu.Unlock()
		if err != nil {
			c.renderProgressLocked(index, done, total, label, '!')
			return err
		}
		c.renderProgressLocked(index, done+1, total, label, '✓')
		return nil
	}

	if !c.interactive {
		fmt.Fprintf(os.Stdout, "  %s %-38s ", stepCounter(done+1, total), label)
		err := action()
		if err != nil {
			fmt.Fprintln(os.Stdout, "FEHLER")
			return err
		}
		fmt.Fprintln(os.Stdout, "OK")
		return nil
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(90 * time.Millisecond)
		defer ticker.Stop()
		frames := []rune{'|', '/', '-', '\\'}
		i := 0
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				c.render(done, total, label, frames[i%len(frames)])
				i++
			}
		}
	}()
	err := action()
	close(stop)
	wg.Wait()
	if err != nil {
		c.render(done, total, label, '!')
		fmt.Fprintln(os.Stdout)
		return err
	}
	c.render(done+1, total, label, '✓')
	fmt.Fprintln(os.Stdout)
	return nil
}

// ProgressStep renders update transaction phases as a fixed-column progress row.
// Unlike Step, which is used for setup's compact [NN/NN] rows, this keeps the
// progress bar, percentage, label and status marker aligned across all phases.
func (c *Console) ProgressStep(ctx context.Context, done, total int, label string, action func() error) error {
	if c.Direct() {
		counter := stepCounter(done+1, total)
		fmt.Fprintf(os.Stdout, "%s %s\n", counter, label)
		err := action()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s%s ✗ %s%s\n", c.style(red+bold), counter, label, c.style(reset))
			return err
		}
		fmt.Fprintf(os.Stdout, "%s%s ✓ %s%s\n", c.style(green+bold), counter, label, c.style(reset))
		return nil
	}
	if c.Fullscreen() {
		c.mu.Lock()
		index := len(c.content)
		c.content = append(c.content, screenLine{progress: true, step: done + 1, total: total, label: label, kind: "run"})
		c.renderFullscreenLocked()
		c.mu.Unlock()

		err := action()
		kind := "ok"
		if err != nil {
			kind = "error"
		}
		c.renderBarProgressLocked(index, done+1, total, label, kind)
		return err
	}

	// Plain/interactive output keeps one terminal row and uses the same columns.
	step := minInt(done+1, total)
	text := progressBarText(step, total, label, 82)
	fmt.Fprintf(os.Stdout, "\r  %-82s", text)
	err := action()
	if err != nil {
		fmt.Fprintf(os.Stdout, " %s✗%s\n", c.style(red+bold), c.style(reset))
		return err
	}
	fmt.Fprintf(os.Stdout, " %s✓%s\n", c.style(green+bold), c.style(reset))
	return nil
}

func (c *Console) renderBarProgressLocked(index, step, total int, label, kind string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := screenLine{progress: true, step: minInt(step, total), total: total, label: label, kind: kind}
	if index >= 0 && index < len(c.content) {
		c.content[index] = entry
	} else {
		c.content = append(c.content, entry)
	}
	c.renderFullscreenLocked()
}

func (c *Console) SkipProgressStep(done, total int, label, reason string) {
	if c.Direct() {
		fullLabel := label
		if strings.TrimSpace(reason) != "" {
			fullLabel += " — " + reason
		}
		fmt.Fprintf(os.Stdout, "%s%s – %s%s\n", c.style(yellow+bold), stepCounter(done+1, total), fullLabel, c.style(reset))
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	fullLabel := label
	if strings.TrimSpace(reason) != "" {
		fullLabel += " — " + reason
	}
	entry := screenLine{progress: true, step: minInt(done+1, total), total: total, label: fullLabel, kind: "skip"}
	if c.fullscreen {
		c.content = append(c.content, entry)
		c.renderFullscreenLocked()
		return
	}
	text := progressBarText(done+1, total, fullLabel, 82)
	fmt.Fprintf(os.Stdout, "  %-82s %s–%s\n", text, c.style(yellow+bold), c.style(reset))
}

func (c *Console) renderProgressLocked(index, done, total int, label string, marker rune) {
	step := minInt(done+1, total)
	if marker == '✓' {
		step = minInt(done, total)
		if step == 0 {
			step = 1
		}
	}
	line := fmt.Sprintf("%s %s", stepCounter(step, total), label)
	kind := "run"
	if marker == '✓' {
		kind = "ok"
	} else if marker == '!' {
		kind = "error"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := screenLine{text: line, kind: kind}
	if index >= 0 && index < len(c.content) {
		c.content[index] = entry
	} else {
		c.content = append(c.content, entry)
	}
	c.renderFullscreenLocked()
}

func (c *Console) SkipStep(done, total int, label, reason string) {
	if c.Direct() {
		fullLabel := label
		if strings.TrimSpace(reason) != "" {
			fullLabel += " — " + reason
		}
		fmt.Fprintf(os.Stdout, "\n%s\n", directStepHeading(stepCounter(done+1, total), label))
		fmt.Fprintf(os.Stdout, "%s└─ – %s%s\n", c.style(yellow+bold), fullLabel, c.style(reset))
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	text := fmt.Sprintf("%s %s", stepCounter(done+1, total), label)
	if strings.TrimSpace(reason) != "" {
		text += " — " + reason
	}
	if c.fullscreen {
		c.content = append(c.content, screenLine{text: text, kind: "skip"})
		c.renderFullscreenLocked()
		return
	}
	fmt.Fprintf(os.Stdout, "  %s %-38s SKIP — %s\n", stepCounter(done+1, total), label, reason)
}

// directStepHeading keeps the step title and its visual separator on one line.
// A fixed 72-column target matches the task separators used by direct/no-UI setup
// output while still giving long labels a short trailing rule.
func directStepHeading(counter, label string) string {
	heading := strings.TrimSpace(counter + " " + strings.TrimSpace(label))
	dashes := 72 - displayWidth(heading) - 1
	if dashes < 3 {
		dashes = 3
	}
	return heading + " " + strings.Repeat("─", dashes)
}

func (c *Console) render(done, total int, label string, m rune) {
	w := 28
	p := 100
	filled := w
	if total > 0 {
		p = done * 100 / total
		filled = done * w / total
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", w-filled)
	c.mu.Lock()
	defer c.mu.Unlock()
	fmt.Fprintf(os.Stdout, "\r  %s[%s]%s %3d%%  %-34s %c", c.style(cyan), bar, c.style(reset), p, truncate(label, 34), m)
}

func (c *Console) ProcessWriters() (io.Writer, io.Writer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.direct && c.directStep {
		return &directLineWriter{console: c, out: os.Stdout, prefix: "│  "}, &directLineWriter{console: c, out: os.Stderr, prefix: "│  ! "}
	}
	if !c.fullscreen {
		return os.Stdout, os.Stderr
	}
	// Child-process output always belongs to the scrollable content area. The
	// --details flag additionally controls command/header detail, but must not
	// make a long-running setup appear frozen by hiding all process output.
	return &consoleLineWriter{console: c}, &consoleLineWriter{console: c, stderr: true}
}

type directLineWriter struct {
	console *Console
	out     io.Writer
	prefix  string
	buffer  string
}

func (w *directLineWriter) Write(p []byte) (int, error) {
	text := w.buffer + string(p)
	parts := strings.Split(text, "\n")
	w.buffer = parts[len(parts)-1]
	for _, line := range parts[:len(parts)-1] {
		w.console.mu.Lock()
		_, err := fmt.Fprintln(w.out, w.prefix+strings.TrimSuffix(line, "\r"))
		w.console.mu.Unlock()
		if err != nil {
			return len(p), err
		}
	}
	return len(p), nil
}

func (w *directLineWriter) Flush() {
	if w.buffer == "" {
		return
	}
	w.console.mu.Lock()
	_, _ = fmt.Fprintln(w.out, w.prefix+strings.TrimSuffix(w.buffer, "\r"))
	w.console.mu.Unlock()
	w.buffer = ""
}

type consoleLineWriter struct {
	console *Console
	stderr  bool
	buffer  string
}

func (w *consoleLineWriter) Write(p []byte) (int, error) {
	text := w.buffer + string(p)
	parts := strings.Split(text, "\n")
	w.buffer = parts[len(parts)-1]
	for _, line := range parts[:len(parts)-1] {
		w.console.appendProcessLine(strings.TrimSuffix(line, "\r"), w.stderr)
	}
	return len(p), nil
}

func (w *consoleLineWriter) Flush() {
	if w.buffer == "" {
		return
	}
	w.console.appendProcessLine(strings.TrimSuffix(w.buffer, "\r"), w.stderr)
	w.buffer = ""
}

// SetupMeta appends one fullscreen setup metadata row using the same
// horizontal gutter as setup step output. This keeps project/schema metadata
// aligned with command stdout/stderr beneath numbered setup steps.
func (c *Console) SetupMeta(total int, text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.fullscreen {
		fmt.Fprintln(os.Stdout, strings.TrimSpace(text))
		return
	}
	c.appendLocked(setupMetaText(total, text))
	c.renderFullscreenLocked()
}

func setupMetaText(total int, text string) string {
	indent := displayWidth(stepCounter(1, total)) + 1
	return strings.Repeat(" ", indent) + "│ " + strings.TrimSpace(text)
}

func (c *Console) Append(s string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fullscreen {
		if c.stepOutputIndent > 0 {
			c.appendStepOutputLocked(s, false)
		} else {
			c.appendLocked(s)
		}
		c.renderFullscreenLocked()
		return
	}
	if c.direct && c.directStep {
		for _, line := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
			fmt.Fprintln(os.Stdout, "│  "+line)
		}
		return
	}
	fmt.Fprintln(os.Stdout, s)
}

// appendProcessLine routes fullscreen child-process output through the active
// setup-step gutter. Leading child padding is normalized so output produced by
// different scripts starts at the same column.
func (c *Console) appendProcessLine(line string, stderr bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fullscreen {
		if c.stepOutputIndent > 0 {
			c.appendStepOutputLocked(line, stderr)
		} else {
			prefix := "    "
			if stderr {
				prefix = "    ! "
			}
			c.appendLocked(prefix + strings.TrimLeft(line, " \t"))
		}
		c.renderFullscreenLocked()
		return
	}
	f := io.Writer(os.Stdout)
	if stderr {
		f = os.Stderr
	}
	fmt.Fprintln(f, line)
}

func (c *Console) appendStepOutputLocked(s string, stderr bool) {
	indent := c.stepOutputIndent
	if indent <= 0 {
		indent = 4
	}
	prefix := strings.Repeat(" ", indent) + "│ "
	if stderr {
		prefix = strings.Repeat(" ", indent) + "│ ! "
	}
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
		line = strings.TrimLeft(line, " \t")
		c.content = append(c.content, screenLine{text: prefix + line, kind: "step-output"})
	}
	if len(c.content) > 2000 {
		c.content = append([]screenLine(nil), c.content[len(c.content)-1500:]...)
	}
}

// Task separates setup tasks from their individual steps. In direct/no-UI mode
// it adds a compact rule so long command output cannot visually merge one task
// into the next. Fullscreen rendering retains the existing content behavior.
func (c *Console) Task(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fullscreen {
		c.appendLocked("")
		c.appendLocked("Task: " + name)
		c.renderFullscreenLocked()
		return
	}
	if c.direct {
		// Direct/--no-ui output is intentionally step-centric. Task headings are
		// omitted so each visible block starts with [NN/NN] and its own output
		// gutter, avoiding redundant separators between consecutive steps.
		return
	}
	fmt.Fprintf(os.Stdout, "\nTask: %s\n", name)
}

func (c *Console) appendLocked(s string) {
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
		c.content = append(c.content, screenLine{text: line})
	}
	if len(c.content) > 2000 {
		c.content = append([]screenLine(nil), c.content[len(c.content)-1500:]...)
	}
}

func (c *Console) renderFullscreenLocked() {
	width, height := terminalSize(os.Stdout)
	c.renderFullscreenLockedWithSize(width, height)
}

func (c *Console) renderFullscreenLockedWithSize(width, height int) {
	if !c.fullscreen {
		return
	}
	if width < 40 {
		width = 40
	}
	if height < 14 {
		height = 14
	}
	inner := width - 2
	contentWidth := width - 4

	type renderedInfoLine struct {
		plain    string
		rendered string
	}
	infoLines := make([]renderedInfoLine, 0, len(c.info)+1)
	if c.infoTitle != "" {
		infoLines = append(infoLines, renderedInfoLine{plain: c.infoTitle, rendered: c.infoTitle})
	}
	for i, line := range c.info {
		highlight := ""
		if i < len(c.infoHighlight) {
			highlight = c.infoHighlight[i]
		}
		for _, part := range wrapDisplay(line, contentWidth) {
			rendered := part
			if c.color && highlight != "" && strings.Contains(part, highlight) {
				rendered = strings.Replace(part, highlight, blueBackground+brightWhite+bold+highlight+reset, 1)
			}
			infoLines = append(infoLines, renderedInfoLine{plain: part, rendered: rendered})
		}
	}
	if len(infoLines) > 8 {
		infoLines = infoLines[:8]
	}
	infoRows := len(infoLines)
	if infoRows < 2 {
		infoRows = 2
	}

	// 3 header rows + info box (infoRows+2) + content box borders + 3 footer rows.
	contentRows := height - (3 + infoRows + 2 + 2 + 3)
	if contentRows < 1 {
		contentRows = 1
	}

	wrapped := make([]screenLine, 0, len(c.content))
	for _, entry := range c.content {
		if entry.progress {
			wrapped = append(wrapped, screenLine{text: progressBarText(entry.step, entry.total, entry.label, contentWidth-4), kind: entry.kind})
			continue
		}
		parts := wrapDisplay(entry.text, contentWidth-2)
		if entry.kind == "step-output" {
			parts = wrapStepOutput(entry.text, contentWidth-2)
		}
		for i, part := range parts {
			kind := ""
			if entry.kind == "step-output" {
				kind = "step-output"
			} else if i == len(parts)-1 {
				kind = entry.kind
			}
			wrapped = append(wrapped, screenLine{text: part, kind: kind})
		}
	}
	if len(wrapped) > contentRows {
		wrapped = wrapped[len(wrapped)-contentRows:]
	}

	var b strings.Builder
	b.WriteString("\033[H")
	// Header
	b.WriteString(boxTop(width))
	b.WriteByte('\n')
	b.WriteString(headerBoxLine(c.title, projectHeaderSegment(c.project, c.projectVersion), inner, c.color))
	b.WriteByte('\n')
	b.WriteString(boxBottom(width))
	b.WriteByte('\n')
	// Project / setup information
	b.WriteString(boxTop(width))
	b.WriteByte('\n')
	for i := 0; i < infoRows; i++ {
		line := renderedInfoLine{}
		if i < len(infoLines) {
			line = infoLines[i]
		}
		b.WriteString(styledPlainBoxLine(line.plain, line.rendered, inner))
		b.WriteByte('\n')
	}
	b.WriteString(boxBottom(width))
	b.WriteByte('\n')
	// Scrollable step content. The newest rows stay visible when the list exceeds the area.
	b.WriteString(boxTop(width))
	b.WriteByte('\n')
	for i := 0; i < contentRows; i++ {
		entry := screenLine{}
		if i < len(wrapped) {
			entry = wrapped[i]
		}
		b.WriteString(statusBoxLine(entry.text, entry.kind, inner, c.color))
		b.WriteByte('\n')
	}
	b.WriteString(boxBottom(width))
	b.WriteByte('\n')
	// Footer
	b.WriteString(boxTop(width))
	b.WriteByte('\n')
	background := blueBackground + brightWhite + bold
	switch c.footerKind {
	case "ok":
		background = greenBackground + brightWhite + bold
	case "error":
		background = redBackground + brightWhite + bold
	case "warn":
		background = yellow + bold
	case "question":
		background = blueBackground + brightWhite + bold
	}
	b.WriteString(coloredBoxLine(c.footer, inner, background))
	b.WriteByte('\n')
	b.WriteString(boxBottom(width))
	fmt.Fprint(os.Stdout, b.String())
}

func boxTop(width int) string    { return "┌" + strings.Repeat("─", maxInt(0, width-2)) + "┐" }
func boxBottom(width int) string { return "└" + strings.Repeat("─", maxInt(0, width-2)) + "┘" }

func coloredBoxLine(text string, inner int, style string) string {
	visible := truncateDisplay(" "+text, inner)
	visible = padDisplay(visible, inner)
	return "│" + style + visible + reset + "│"
}

func headerBoxLine(title, project string, inner int, color bool) string {
	header := headerDisplayText(title, project, inner)
	if color {
		return coloredBoxLine(header, inner, blueBackground+brightWhite+bold)
	}
	visible := padDisplay(truncateDisplay(" "+header, inner), inner)
	return "│" + visible + "│"
}

// headerDisplayText renders the header as three clearly readable segments:
//
//	Update CLI Version X.Y.Z   |   project   |   phase
//
// The updater keeps the internal title in the historic "base — phase" form;
// this function only changes presentation. On narrow terminals the project
// segment is truncated first so the Update CLI version and current phase remain
// visible.
func headerDisplayText(title, project string, inner int) string {
	base := strings.TrimSpace(title)
	phase := ""
	if before, after, ok := strings.Cut(base, " — "); ok {
		base = strings.TrimSpace(before)
		phase = strings.TrimSpace(after)
	}
	project = strings.TrimSpace(project)

	const sep = "   |   "
	compose := func(projectText string) string {
		parts := []string{base}
		if projectText != "" {
			parts = append(parts, projectText)
		}
		if phase != "" {
			parts = append(parts, phase)
		}
		return strings.Join(parts, sep)
	}

	header := compose(project)
	available := maxInt(0, inner-1) // coloredBoxLine adds one leading blank
	if displayWidth(header) <= available {
		return header
	}

	if project != "" {
		fixedWidth := displayWidth(compose(""))
		// Adding a project introduces one additional separator compared with
		// compose(""). Reserve that separator, then spend the remaining width
		// on the project name.
		projectWidth := available - fixedWidth - displayWidth(sep)
		if projectWidth > 0 {
			project = truncateDisplay(project, projectWidth)
			header = compose(project)
			if displayWidth(header) <= available {
				return header
			}
		}
	}

	return truncateDisplay(header, available)
}

func plainBoxLine(text string, inner int) string {
	return styledPlainBoxLine(text, text, inner)
}

func styledPlainBoxLine(plain, rendered string, inner int) string {
	available := maxInt(0, inner-2)
	if displayWidth(plain) > available {
		plain = truncateDisplay(plain, available)
		// If truncation is necessary, prefer correct geometry over retaining a
		// partial ANSI-highlighted fragment. Normal info rows are pre-wrapped.
		rendered = plain
	}
	padding := strings.Repeat(" ", maxInt(0, available-displayWidth(plain)))
	return "│ " + rendered + padding + " │"
}

func progressBarText(step, total int, label string, available int) string {
	if total <= 0 {
		total = 1
	}
	if step < 0 {
		step = 0
	}
	if step > total {
		step = total
	}
	percent := step * 100 / total
	barWidth := 26
	if available < 72 {
		barWidth = maxInt(10, available-38)
	}
	if barWidth > 26 {
		barWidth = 26
	}
	filled := step * barWidth / total
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	prefix := fmt.Sprintf("%s [%s] %3d%%  ", stepCounter(step, total), bar, percent)
	labelWidth := maxInt(1, available-displayWidth(prefix))
	return prefix + padDisplay(truncateDisplay(label, labelWidth), labelWidth)
}

func statusBoxLine(text, kind string, inner int, color bool) string {
	if kind == "success-banner" {
		visible := padDisplay(truncateDisplay(" "+text, inner), inner)
		if color {
			return "│" + greenBackground + brightWhite + bold + visible + reset + "│"
		}
		return "│" + visible + "│"
	}
	available := maxInt(0, inner-2)
	marker := ""
	style := ""
	switch kind {
	case "ok":
		marker = "✓"
		style = green + bold
	case "error":
		marker = "✗"
		style = red + bold
	case "run":
		marker = "…"
		style = cyan + bold
	case "skip":
		marker = "–"
		style = yellow + bold
	}
	if marker == "" {
		visible := padDisplay(truncateDisplay(text, available), available)
		return "│ " + visible + " │"
	}
	textWidth := maxInt(0, available-2)
	visible := padDisplay(truncateDisplay(text, textWidth), textWidth)
	if color {
		marker = style + marker + reset
	}
	return "│ " + visible + " " + marker + " │"
}

func wrapStepOutput(s string, width int) []string {
	if width <= 0 || displayWidth(s) <= width {
		return []string{s}
	}
	r := []rune(s)
	contentStart := 0
	for i, ch := range r {
		if ch != '│' {
			continue
		}
		contentStart = i + 1
		for contentStart < len(r) && (r[contentStart] == ' ' || r[contentStart] == '!') {
			contentStart++
		}
		break
	}
	if contentStart <= 0 || contentStart >= width {
		return wrapDisplay(s, width)
	}
	out := []string{}
	first := minInt(width, len(r))
	out = append(out, string(r[:first]))
	r = r[first:]
	continuationWidth := maxInt(1, width-contentStart)
	continuationPrefix := strings.Repeat(" ", contentStart)
	for len(r) > 0 {
		n := minInt(continuationWidth, len(r))
		out = append(out, continuationPrefix+string(r[:n]))
		r = r[n:]
	}
	return out
}

func wrapDisplay(s string, width int) []string {
	if width <= 0 {
		return []string{""}
	}
	if s == "" {
		return []string{""}
	}
	r := []rune(s)
	out := []string{}
	for len(r) > 0 {
		n := width
		if len(r) < n {
			n = len(r)
		}
		out = append(out, string(r[:n]))
		r = r[n:]
	}
	return out
}

func displayWidth(s string) int { return utf8.RuneCountInString(s) }

func padDisplay(s string, width int) string {
	return s + strings.Repeat(" ", maxInt(0, width-displayWidth(s)))
}

func truncateDisplay(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return string(r[:1])
	}
	return string(r[:width-1]) + "…"
}

func isTerminal(f *os.File) bool {
	i, e := f.Stat()
	return e == nil && i.Mode()&os.ModeCharDevice != 0
}

func truncate(s string, w int) string { return truncateDisplay(s, w) }

func stepCounter(step, total int) string {
	return fmt.Sprintf("[%02d/%02d]", step, total)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
