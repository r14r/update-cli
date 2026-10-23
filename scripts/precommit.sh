#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$ROOT"

usage() {
  cat <<'HELP'
Usage: scripts/precommit.sh [--commit|--push|--all]
  --commit  Fast mandatory checks before committing (default)
  --push    Full test, race, build, smoke and cross-build checks
  --all     Run both suites (same as CI)
HELP
}
mode="${1:---commit}"
case "$mode" in
  --commit|--push|--all) ;;
  -h|--help) usage; exit 0 ;;
  *) usage >&2; exit 2 ;;
esac

step() {
  printf '\n==> %s\n' "$1"
  shift
  "$@"
}

require() {
  command -v "$1" >/dev/null 2>&1 || {
    printf 'ERROR: required tool missing: %s\n' "$1" >&2
    exit 127
  }
}
require go
require bash
require python3
if [[ "$mode" == "--commit" || "$mode" == "--all" ]]; then
  step "Go module download" go mod download
  printf '\n==> Go formatting\n'
  unformatted="$(gofmt -l .)"
  if [[ -n "$unformatted" ]]; then
    printf '%s\n' "$unformatted" >&2
    while IFS= read -r file; do gofmt -d "$file" >&2 || true; done <<< "$unformatted"
    exit 1
  fi
  printf '\n==> Shell syntax\n'
  for script in setup.sh setup-template.sh scripts/*.sh; do
    bash -n "$script"
  done
  printf '\n==> Justfile escaping\n'
  if grep -n '\$\$' justfile; then
    echo 'ERROR: Justfile contains literal double dollar escaping' >&2
    exit 1
  fi
  step "Version validation" bash scripts/validate-version.sh .
  printf '\n==> Legacy Go import paths\n'
  if grep -Rn --include='*.go' 'github.com/r14r/update-cli/lib/' src; then
    echo 'ERROR: Go imports must use github.com/r14r/update-cli/src/lib/' >&2
    exit 1
  fi
  step "Go vet" go vet ./...
  step "Go tests" go test -count=1 ./...
fi
if [[ "$mode" == "--push" || "$mode" == "--all" ]]; then
  step "Go module download" go mod download
  step "Go race tests" go test -count=1 -race ./...
  mkdir -p dist
  step "Native build" go build -trimpath -o dist/update-cli .
  step "Setup schema v2 catalog" ./dist/update-cli setup --list --no-ui
  step "Fullscreen TUI smoke" python3 tests/tui_smoke.py
  for target in darwin/amd64 darwin/arm64 linux/amd64; do
    os="${target%/*}"
    arch="${target#*/}"
    step "Cross-build $target" env GOOS="$os" GOARCH="$arch" go build -trimpath -o "dist/update-cli-$os-$arch" .
  done
fi
printf '\nPrecommit checks passed (%s).\n' "$mode"
