package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	reset          = "\033[0m"
	bold           = "\033[1m"
	dim            = "\033[2m"
	red            = "\033[31m"
	green          = "\033[32m"
	yellow         = "\033[33m"
	blue           = "\033[34m"
	cyan           = "\033[36m"
	brightWhite    = "\033[97m"
	blueBackground = "\033[44m"
	redBackground  = "\033[41m"
	eraseToEnd     = "\033[K"
)

type Console struct {
	color       bool
	interactive bool
	mu          sync.Mutex
}

func New(noColor bool) *Console {
	interactive := isTerminal(os.Stdout)
	colorEnabled := !noColor && os.Getenv("NO_COLOR") == "" && interactive
	return &Console{color: colorEnabled, interactive: interactive}
}

func (console *Console) Header(title string) {
	console.mu.Lock()
	defer console.mu.Unlock()
	fmt.Fprintf(os.Stdout, "\n%s%s%s\n", console.style(bold+cyan), title, console.style(reset))
	fmt.Fprintln(os.Stdout, strings.Repeat("─", 72))
}

func (console *Console) Row(label, value string) {
	console.mu.Lock()
	defer console.mu.Unlock()
	fmt.Fprintf(os.Stdout, "  %s%-19s%s %s\n", console.style(dim), label, console.style(reset), value)
}

func (console *Console) StatusRow(label, detail string) {
	console.mu.Lock()
	defer console.mu.Unlock()
	fmt.Fprint(os.Stdout, formatStatusRow(label, detail, console.color))
}

func (console *Console) Banner(title string) {
	console.mu.Lock()
	defer console.mu.Unlock()
	fmt.Fprint(os.Stdout, formatBanner(title, console.color))
}

func (console *Console) ErrorNotice(title, detail string) {
	console.mu.Lock()
	defer console.mu.Unlock()
	fmt.Fprint(os.Stderr, formatErrorNotice(title, detail, console.color))
}

func (console *Console) Diagnostic(status, label, detail string) {
	color := green
	marker := "OK"
	switch status {
	case "warning":
		color = yellow
		marker = "WARN"
	case "error":
		color = red
		marker = "FAIL"
	}

	console.mu.Lock()
	defer console.mu.Unlock()
	fmt.Fprintf(
		os.Stdout,
		"  %s%-5s%s %-22s %s\n",
		console.style(color+bold),
		marker,
		console.style(reset),
		truncate(label, 22),
		detail,
	)
}

func (console *Console) Info(message string) {
	console.line(os.Stdout, blue, "INFO", message)
}

func (console *Console) Warn(message string) {
	console.line(os.Stderr, yellow, "WARN", message)
}

func (console *Console) Success(message string) {
	console.line(os.Stdout, green, "OK", message)
}

func (console *Console) Step(
	ctx context.Context,
	completed int,
	total int,
	label string,
	action func() error,
) error {
	if !console.interactive {
		fmt.Fprintf(os.Stdout, "  [%d/%d] %-38s ", completed+1, total, label)
		err := action()
		if err != nil {
			fmt.Fprintln(os.Stdout, "FEHLER")
			return err
		}
		fmt.Fprintln(os.Stdout, "OK")
		return nil
	}

	result := make(chan error, 1)
	go func() { result <- action() }()

	ticker := time.NewTicker(90 * time.Millisecond)
	defer ticker.Stop()
	frames := []rune{'|', '/', '-', '\\'}
	frame := 0

	for {
		select {
		case <-ctx.Done():
			console.renderProgress(completed, total, label, '!')
			fmt.Fprintln(os.Stdout)
			return ctx.Err()
		case err := <-result:
			if err != nil {
				console.renderProgress(completed, total, label, '!')
				fmt.Fprintln(os.Stdout)
				return err
			}
			console.renderProgress(completed+1, total, label, '✓')
			fmt.Fprintln(os.Stdout)
			return nil
		case <-ticker.C:
			console.renderProgress(completed, total, label, frames[frame%len(frames)])
			frame++
		}
	}
}

func (console *Console) renderProgress(completed, total int, label string, marker rune) {
	width := 28
	percent := 100
	filled := width
	if total > 0 {
		percent = completed * 100 / total
		filled = completed * width / total
	}
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	console.mu.Lock()
	defer console.mu.Unlock()
	fmt.Fprintf(
		os.Stdout,
		"\r  %s[%s]%s %3d%%  %-34s %c",
		console.style(cyan),
		bar,
		console.style(reset),
		percent,
		truncate(label, 34),
		marker,
	)
}

func (console *Console) line(file *os.File, color, level, message string) {
	console.mu.Lock()
	defer console.mu.Unlock()
	fmt.Fprintf(
		file,
		"%s%s%-5s%s %s\n",
		console.style(color),
		console.style(bold),
		level,
		console.style(reset),
		message,
	)
}

func (console *Console) style(value string) string {
	if !console.color {
		return ""
	}
	return value
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func truncate(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func formatStatusRow(label, detail string, color bool) string {
	if !color {
		return fmt.Sprintf("  %-19s %s\n", label, detail)
	}

	return fmt.Sprintf(
		"%s  %-19s %s%s%s\n",
		blueBackground+brightWhite+bold,
		label,
		detail,
		eraseToEnd,
		reset,
	)
}

func formatBanner(title string, color bool) string {
	if !color {
		return fmt.Sprintf("\n%s\n%s\n", title, strings.Repeat("─", 72))
	}

	return fmt.Sprintf(
		"\n%s  %s%s%s\n",
		blueBackground+brightWhite+bold,
		title,
		eraseToEnd,
		reset,
	)
}

func formatErrorNotice(title, detail string, color bool) string {
	if !color {
		return fmt.Sprintf("\n%s\n%s\n", title, detail)
	}

	return fmt.Sprintf(
		"\n%s  %s%s%s\n  %s\n",
		redBackground+brightWhite+bold,
		title,
		eraseToEnd,
		reset,
		detail,
	)
}
