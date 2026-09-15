#!/usr/bin/env bash
set -euo pipefail

OUTPUT_DIR="${1:?output directory required}"
PROJECT="${2:-demo-app}"
VERSION="${3:-1.2.3}"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/update-cli-demo-release.XXXXXX")"
trap 'rm -rf "$TMP"' EXIT

mkdir -p "$OUTPUT_DIR" "$TMP/$PROJECT"
printf '%s\n' "$VERSION" > "$TMP/$PROJECT/VERSION"
printf 'Demo release %s for update-cli QUICKSTART.\n' "$VERSION" > "$TMP/$PROJECT/README.txt"
printf 'hello from %s\n' "$VERSION" > "$TMP/$PROJECT/app.txt"
(
  cd "$TMP/$PROJECT"
  zip -qr "$OUTPUT_DIR/${PROJECT}-v${VERSION}.zip" .
)
printf '%s\n' "$OUTPUT_DIR/${PROJECT}-v${VERSION}.zip"
