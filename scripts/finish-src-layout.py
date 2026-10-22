#!/usr/bin/env python3
"""Complete src/ layout refactor in a checked-out refactor/src-doc-layout branch."""
from pathlib import Path
import re

root = Path(__file__).resolve().parents[1]
src = root / "src"
assert src.is_dir(), "src/ directory is missing"

def replace(path: Path, before: str, after: str):
    if not path.is_file():
        return
    text = path.read_text()
    new = text.replace(before, after)
    if new != text:
        path.write_text(new)

for path in src.rglob("*.go"):
    replace(path, "github.com/r14r/update-cli/lib/", "github.com/r14r/update-cli/src/lib/")
    replace(path, "github.com/r14r/update-cli/cmd/", "github.com/r14r/update-cli/src/cmd/")

# Root package tests must run against repository-root fixtures.
test_main = src / "layout_test.go"
if not test_main.exists():
    test_main.write_text('''package main

import (
    "os"
    "testing"
)

func TestMain(m *testing.M) {
    if err := os.Chdir(".."); err != nil {
        panic(err)
    }
    os.Exit(m.Run())
}
''')

justfile = root / "justfile"
if justfile.exists():
    text = justfile.read_text()
    text = text.replace("go run .", "go run ./src")
    text = text.replace("go test -count=1 ./...", "go test -count=1 ./src/...")
    text = text.replace("go vet ./...", "go vet ./src/...")
    text = text.replace("gofmt -w .", "gofmt -w src")
    text = text.replace("gofmt -l .", "gofmt -l src")
    text = text.replace("go build -trimpath", "go build -trimpath")
    text = text.replace("-o dist/update-cli .", "-o dist/update-cli ./src")
    text = re.sub(r'(-o dist/update-cli-[\\w-]+) \\.', r'\\1 ./src', text)
    justfile.write_text(text)

precommit = root / "scripts/precommit.sh"
if precommit.exists():
    text = precommit.read_text()
    text = text.replace("gofmt -l .", "gofmt -l src")
    text = text.replace("go vet ./...", "go vet ./src/...")
    text = text.replace("go test -count=1 ./...", "go test -count=1 ./src/...")
    text = text.replace("go build -trimpath -o dist/update-cli .", "go build -trimpath -o dist/update-cli ./src")
    text = text.replace('"dist/update-cli-$os-$arch" .', '"dist/update-cli-$os-$arch" ./src')
    precommit.write_text(text)

package = root / "scripts/package-source.sh"
if package.exists():
    text = package.read_text()
    for old in ("main.go", "lib/backup/backup.go", "lib/config/config.go", "lib/updater/updater.go", "lib/rsync/rsync.go"):
        text = text.replace(old, "src/" + old)
    text = text.replace("VERSION README.md RELEASE_NOTES.md", "VERSION doc/README.md doc/RELEASE_NOTES.md")
    package.write_text(text)

print("Applied Go import, build and test path migration. Run gofmt -w src and just ci.")
