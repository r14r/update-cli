#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-.}"
ROOT="$(cd "$ROOT" && pwd -P)"
VERSION_FILE="$ROOT/VERSION"
[[ -f "$VERSION_FILE" ]] || { echo "ERROR VERSION fehlt: $VERSION_FILE" >&2; exit 1; }
VERSION="$(tr -d '[:space:]' < "$VERSION_FILE")"
case "$VERSION" in
  [0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "ERROR ungültige VERSION: $VERSION" >&2; exit 1 ;;
esac
printf '%s\n' "$VERSION"
