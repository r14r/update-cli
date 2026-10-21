#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
command -v vhs >/dev/null 2>&1 || {
  echo "ERROR: vhs is required to render CLI demo videos." >&2
  echo "Install VHS from https://github.com/charmbracelet/vhs and rerun." >&2
  exit 1
}

mkdir -p "$ROOT_DIR/docs/videos"
if [[ $# -eq 0 ]]; then
  set -- "$ROOT_DIR/docs/tapes/install.tape" "$ROOT_DIR/docs/tapes/quickstart.tape"
fi
for tape in "$@"; do
  [[ "$tape" = /* ]] || tape="$ROOT_DIR/$tape"
  echo "Rendering $(basename "$tape")"
  (cd "$ROOT_DIR" && vhs "$tape")
done
