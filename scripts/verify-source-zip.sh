#!/usr/bin/env bash
set -euo pipefail
ZIP="${1:?usage: verify-source-zip.sh update-cli-vX.Y.Z.zip}"
base="$(basename "$ZIP" .zip)"
version="${base#update-cli-v}"
listing="$(mktemp "${TMPDIR:-/tmp}/update-cli-verify.XXXXXX")"
trap 'rm -f "$listing"' EXIT
unzip -l "$ZIP" > "$listing"
[[ "$(unzip -p "$ZIP" "$base/VERSION" | tr -d '[:space:]')" == "$version" ]]
grep -q " $base/README.md$" "$listing"
grep -q " $base/RELEASE_NOTES.md$" "$listing"
grep -q " $base/defaults/config.json$" "$listing"
grep -q " $base/defaults/templates.json$" "$listing"
grep -q " $base/lib/backup/backup.go$" "$listing"
! grep -Eq " $base/(\.update-cli|dist)(/|$)" "$listing"
