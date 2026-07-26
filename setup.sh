#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd -P)"
VERSION_FILE="${ROOT_DIR}/VERSION"
BUILD_CONFIG_FILE="${ROOT_DIR}/build-config.json"
DIST_DIR="${ROOT_DIR}/dist"
BINARY="${DIST_DIR}/update-cli"

if [[ -t 1 && "${NO_COLOR:-}" == "" ]]; then
    BOLD=$'\033[1m'
    GREEN=$'\033[32m'
    BLUE=$'\033[34m'
    YELLOW=$'\033[33m'
    RED=$'\033[31m'
    RESET=$'\033[0m'
else
    BOLD=""
    GREEN=""
    BLUE=""
    YELLOW=""
    RED=""
    RESET=""
fi

info() {
    printf '%s==>%s %s\n' "${BLUE}${BOLD}" "${RESET}" "$*"
}

success() {
    printf '%sOK%s  %s\n' "${GREEN}${BOLD}" "${RESET}" "$*"
}

warn() {
    printf '%sWARN%s %s\n' "${YELLOW}${BOLD}" "${RESET}" "$*" >&2
}

fail() {
    printf '%sERROR%s %s\n' "${RED}${BOLD}" "${RESET}" "$*" >&2
    exit 1
}

require_command() {
    command -v "$1" >/dev/null 2>&1 || fail "Erforderliches Kommando fehlt: $1"
}

on_error() {
    local exit_code=$?
    printf '%sERROR%s Setup fehlgeschlagen (Zeile %s, Exit-Code %s).\n' \
        "${RED}${BOLD}" "${RESET}" "${BASH_LINENO[0]:-?}" "${exit_code}" >&2
    exit "${exit_code}"
}
trap on_error ERR

cd -- "${ROOT_DIR}"

require_command go
require_command rsync

[[ -f "${VERSION_FILE}" ]] || fail "VERSION-Datei fehlt: ${VERSION_FILE}"
[[ -f "${BUILD_CONFIG_FILE}" ]] || fail "Build-Konfiguration fehlt: ${BUILD_CONFIG_FILE}"
VERSION="$(tr -d '[:space:]' < "${VERSION_FILE}")"
[[ "${VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "Ungültige Version in VERSION: ${VERSION}"

printf '\n%sRelease Updater Setup%s\n' "${BOLD}" "${RESET}"
printf '%-18s %s\n' "Projektordner" "${ROOT_DIR}"
printf '%-18s %s\n' "Version" "${VERSION}"
printf '%-18s %s\n' "Go" "$(go version)"
printf '%-18s %s\n\n' "Ziel" "${BINARY}"

info "Go-Module laden"
go mod download
success "Go-Module geladen"

info "Build-Konfiguration prüfen"
go run ./cmd/buildconfig --validate >/dev/null
success "build-config.json ist gültig"
printf '%-18s %s
' "Download" "$(go run ./cmd/buildconfig --field defaultDownloadFolder)"
printf '%-18s %s
' "Deployment" "$(go run ./cmd/buildconfig --field defaultDeploymentPath)"
printf '%-18s %s

' "Globale Config" "$(go run ./cmd/buildconfig --field defaultConfigPath)"

info "Quellcode prüfen"
go vet ./...
success "go vet erfolgreich"

info "Tests ausführen"
go test ./...
success "Tests erfolgreich"

info "update-cli bauen"
mkdir -p -- "${DIST_DIR}"
go build \
    -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o "${BINARY}" \
    .
chmod 0755 "${BINARY}"
success "Binary erstellt: ${BINARY}"

if ! "${BINARY}" --version | grep -Fq "${VERSION}"; then
    fail "Das erzeugte Binary meldet nicht die erwartete Version ${VERSION}"
fi

printf '\n%sSetup abgeschlossen%s\n' "${GREEN}${BOLD}" "${RESET}"
printf '%-18s %s\n' "Binary" "${BINARY}"
printf '%-18s %s\n' "Version" "$("${BINARY}" --version)"

if command -v just >/dev/null 2>&1; then
    printf '%-18s %s\n' "Weitere Befehle" "just --list"
else
    warn "just ist optional und wurde nicht gefunden"
fi
