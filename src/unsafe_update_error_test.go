package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/r14r/update-cli/src/lib/updater"
)

func TestPrintUnsafeUpdateError(t *testing.T) {
	old := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = writer
	defer func() { os.Stderr = old }()
	printUnsafeUpdateError(&updater.UnsafeWorkingDirectoryError{ProjectRoot: "/projects/demo"})
	_ = writer.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	output := string(bytes.TrimSpace(data))
	for _, expected := range []string{
		"Update fehlgeschlagen: Ausführung aus current/",
		"TODO",
		"cd /projects/demo",
		"update-cli update",
		"cd current",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("missing %q in %q", expected, output)
		}
	}
	if strings.Contains(output, "Failure log saved") || strings.Contains(output, "Fehlerprotokoll") {
		t.Errorf("unexpected log banner: %q", output)
	}
	if !errors.As(error(&updater.UnsafeWorkingDirectoryError{}), new(*updater.UnsafeWorkingDirectoryError)) {
		t.Fatal("unsafe working directory error must be discoverable via errors.As")
	}
}
