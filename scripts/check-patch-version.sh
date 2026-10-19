#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
mode="${1:---staged}"
case "$mode" in
  --staged)
    previous="$(git show HEAD:VERSION 2>/dev/null || true)"
    current="$(git show :VERSION 2>/dev/null || true)"
    [[ -n "$current" ]] || { echo "VERSION must be staged (run: just bump-patch && git add VERSION)" >&2; exit 1; }
    ;;
  --base)
    base="${2:?Usage: check-patch-version.sh --base BASE_REF}"
    previous="$(git show "$base:VERSION")"
    current="$(cat VERSION)"
    ;;
  *) echo "Usage: $0 --staged | --base BASE_REF" >&2; exit 2 ;;
esac
[[ "$current" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]] || { echo "Invalid VERSION: $current" >&2; exit 1; }
new_major="${BASH_REMATCH[1]}"
new_minor="${BASH_REMATCH[2]}"
new_patch="${BASH_REMATCH[3]}"
[[ "$previous" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]] || { echo "Invalid previous VERSION: $previous" >&2; exit 1; }
old_major="${BASH_REMATCH[1]}"
old_minor="${BASH_REMATCH[2]}"
old_patch="${BASH_REMATCH[3]}"
# Allow sequential patch, minor and major releases. The staged hook requires
# a single version increment; a PR may contain several patch increments.
if (( 10#$new_major == 10#$old_major && 10#$new_minor == 10#$old_minor )); then
  if [[ "$mode" == "--staged" ]]; then
    (( 10#$new_patch == 10#$old_patch + 1 )) || { echo "Patch must increase exactly once: $previous -> $current" >&2; exit 1; }
  else
    (( 10#$new_patch > 10#$old_patch )) || { echo "Patch must increase: $previous -> $current" >&2; exit 1; }
  fi
elif (( 10#$new_major == 10#$old_major && 10#$new_minor == 10#$old_minor + 1 && 10#$new_patch == 0 )); then
  :
elif (( 10#$new_major == 10#$old_major + 1 && 10#$new_minor == 0 && 10#$new_patch == 0 )); then
  :
else
  echo "Invalid semantic version increment: $previous -> $current" >&2
  exit 1
fi
echo "Semantic version validated: $previous -> $current"
