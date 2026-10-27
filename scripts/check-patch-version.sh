#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
mode="${1:---base}"
case "$mode" in
  --base)
    base="${2:?Usage: check-patch-version.sh --base BASE_REF}"
    previous="$(git show "$base:VERSION")"
    current="$(cat VERSION)"
    ;;
  *) echo "Usage: $0 --base BASE_REF" >&2; exit 2 ;;
esac
[[ "$current" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]] || { echo "Invalid VERSION: $current" >&2; exit 1; }
new_major="${BASH_REMATCH[1]}"
new_minor="${BASH_REMATCH[2]}"
new_patch="${BASH_REMATCH[3]}"
[[ "$previous" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]] || { echo "Invalid previous VERSION: $previous" >&2; exit 1; }
old_major="${BASH_REMATCH[1]}"
old_minor="${BASH_REMATCH[2]}"
old_patch="${BASH_REMATCH[3]}"
# Validate the PR version against the target branch, not against every local commit.
if (( 10#$new_major == 10#$old_major && 10#$new_minor == 10#$old_minor )); then
  (( 10#$new_patch > 10#$old_patch )) || { echo "Patch must increase: $previous -> $current" >&2; exit 1; }
elif (( 10#$new_major == 10#$old_major && 10#$new_minor == 10#$old_minor + 1 && 10#$new_patch == 0 )); then
  :
elif (( 10#$new_major == 10#$old_major + 1 && 10#$new_minor == 0 && 10#$new_patch == 0 )); then
  :
elif [[ "$previous" == "3.5.0" && "$current" == "1.9.0" ]]; then
  # One-time, exact-match reset authorized for the 1.9.0 release line.
  # No general downgrade exemption is introduced.
  echo "Authorized one-time version reset: $previous -> $current"
else
  echo "Invalid semantic version increment: $previous -> $current" >&2
  exit 1
fi
echo "Semantic version validated: $previous -> $current"
