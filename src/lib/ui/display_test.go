package ui

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestReplaceHomePath(t *testing.T) {
	home := "/Users/Ralph.Goestenmeier"
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "descendant",
			in:   "/Users/Ralph.Goestenmeier/Downloads/DigitalProductsPlatform-v4.5.0.zip",
			want: "$HOME/Downloads/DigitalProductsPlatform-v4.5.0.zip",
		},
		{name: "home itself", in: home, want: "$HOME"},
		{
			name: "multiple paths",
			in:   "copy /Users/Ralph.Goestenmeier/source to /Users/Ralph.Goestenmeier/target",
			want: "copy $HOME/source to $HOME/target",
		},
		{
			name: "similar lexical prefix",
			in:   "/Users/Ralph.Goestenmeier-old/file.zip",
			want: "/Users/Ralph.Goestenmeier-old/file.zip",
		},
		{
			name: "embedded path fragment",
			in:   "/Volumes/Users/Ralph.Goestenmeier/file.zip",
			want: "/Volumes/Users/Ralph.Goestenmeier/file.zip",
		},
		{name: "unrelated", in: "/tmp/file.zip", want: "/tmp/file.zip"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := replaceHomePath(tt.in, home); got != tt.want {
				t.Fatalf("replaceHomePath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDisplayTextUsesCurrentHome(t *testing.T) {
	t.Setenv("HOME", "/Users/Ralph.Goestenmeier")
	got := DisplayText("archive=/Users/Ralph.Goestenmeier/Downloads/app.zip")
	want := "archive=$HOME/Downloads/app.zip"
	if got != want {
		t.Fatalf("DisplayText() = %q, want %q", got, want)
	}
}

func TestConsoleRowShortensHomePathForDisplay(t *testing.T) {
	t.Setenv("HOME", "/Users/Ralph.Goestenmeier")

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	defer func() { os.Stdout = oldStdout }()

	c := New(true)
	c.Row("Quelle", "/Users/Ralph.Goestenmeier/Downloads/app.zip")
	_ = writer.Close()
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if strings.Contains(got, "/Users/Ralph.Goestenmeier") {
		t.Fatalf("console output leaked absolute home path: %q", got)
	}
	if !strings.Contains(got, "$HOME/Downloads/app.zip") {
		t.Fatalf("console output does not contain shortened home path: %q", got)
	}
}

func TestFullscreenStepOutputShortensHomePath(t *testing.T) {
	t.Setenv("HOME", "/Users/Ralph.Goestenmeier")
	c := &Console{stepOutputIndent: 8}
	c.appendStepOutputLocked("archive: /Users/Ralph.Goestenmeier/Downloads/app.zip", false)
	if len(c.content) != 1 {
		t.Fatalf("content lines = %d, want 1", len(c.content))
	}
	got := c.content[0].text
	if strings.Contains(got, "/Users/Ralph.Goestenmeier") {
		t.Fatalf("fullscreen output leaked absolute home path: %q", got)
	}
	if !strings.Contains(got, "$HOME/Downloads/app.zip") {
		t.Fatalf("fullscreen output does not contain shortened home path: %q", got)
	}
}

func TestProcessWriterShortensHomePath(t *testing.T) {
	t.Setenv("HOME", "/Users/Ralph.Goestenmeier")
	var out bytes.Buffer
	c := &Console{}
	w := &directLineWriter{console: c, out: &out}
	if _, err := w.Write([]byte("source=/Users/Ralph.Goestenmeier/Downloads/app.zip\n")); err != nil {
		t.Fatal(err)
	}
	w.Flush()
	got := out.String()
	if strings.Contains(got, "/Users/Ralph.Goestenmeier") {
		t.Fatalf("process output leaked absolute home path: %q", got)
	}
	if !strings.Contains(got, "source=$HOME/Downloads/app.zip") {
		t.Fatalf("process output does not contain shortened home path: %q", got)
	}
}
