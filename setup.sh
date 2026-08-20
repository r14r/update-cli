#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_NAME="Update CLI"
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd -P)"
MANIFEST="${ROOT_DIR}/update-cli.yaml"
FORWARD_ARGS=()

usage() {
    cat <<'TXT'
Usage: ./setup.sh [--details] [--wait|--no-wait] [--fullscreen|--no-fullscreen] [--no-ui|--noui] [--list|--task NAME|--workflow NAME] [--config FILE]
TXT
}

while (($# > 0)); do
    case "$1" in
        --details|--wait|--no-wait)
            FORWARD_ARGS+=("$1")
            shift
            ;;
        --list)
            FORWARD_ARGS+=("--setup-list")
            shift
            ;;
        --task)
            (($# >= 2)) || { printf 'ERROR --task benötigt einen Namen\n' >&2; exit 2; }
            FORWARD_ARGS+=("--setup-task" "$2")
            shift 2
            ;;
        --workflow)
            (($# >= 2)) || { printf 'ERROR --workflow benötigt einen Namen\n' >&2; exit 2; }
            FORWARD_ARGS+=("--setup-workflow" "$2")
            shift 2
            ;;
        --no-ui|--noui|---no-ui)
            export UPDATE_CLI_TUI=plain
            FORWARD_ARGS+=("--no-ui")
            shift
            ;;
        --fullscreen)
            export UPDATE_CLI_TUI=fullscreen
            shift
            ;;
        --no-fullscreen)
            export UPDATE_CLI_TUI=plain
            shift
            ;;
        --config)
            (($# >= 2)) || { printf 'ERROR --config benötigt eine Datei\n' >&2; exit 2; }
            if [[ "$2" = /* ]]; then
                MANIFEST="$2"
            else
                MANIFEST="${ROOT_DIR}/$2"
            fi
            shift 2
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        *)
            printf 'ERROR unbekannte Option: %s\n' "$1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

manifest_schema() {
    awk '
        /^[[:space:]]*schemaVersion[[:space:]]*:/ {
            line=$0
            sub(/^[^:]*:[[:space:]]*/, "", line)
            gsub(/[[:space:]#].*$/, "", line)
            if (line ~ /^[0-9]+$/) { print line; found=1; exit }
        }
        /^[[:space:]]*version[[:space:]]*:/ {
            line=$0
            sub(/^[^:]*:[[:space:]]*/, "", line)
            gsub(/[[:space:]#].*$/, "", line)
            if (line ~ /^[0-9]+$/) { legacy=line }
        }
        END { if (!found && legacy != "") print legacy }
    ' "$1"
}

platform_suffix() {
    local os arch
    case "$(uname -s 2>/dev/null || true)" in
        Darwin) os="darwin" ;;
        Linux) os="linux" ;;
        *) return 1 ;;
    esac
    case "$(uname -m 2>/dev/null || true)" in
        x86_64|amd64) arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *) return 1 ;;
    esac
    printf '%s-%s\n' "${os}" "${arch}"
}

if [[ -t 1 && "${NO_COLOR:-}" == "" ]]; then
    BOLD=$'\033[1m'; GREEN=$'\033[32m'; BLUE=$'\033[34m'; YELLOW=$'\033[33m'; RED=$'\033[31m'; RESET=$'\033[0m'
else
    BOLD=""; GREEN=""; BLUE=""; YELLOW=""; RED=""; RESET=""
fi
info(){ printf '%s==>%s %s\n' "${BLUE}${BOLD}" "${RESET}" "$*"; }
success(){ printf '%sOK%s  %s\n' "${GREEN}${BOLD}" "${RESET}" "$*"; }
warn(){ printf '%sWARN%s %s\n' "${YELLOW}${BOLD}" "${RESET}" "$*" >&2; }
fail(){ printf '%sERROR%s %s\n' "${RED}${BOLD}" "${RESET}" "$*" >&2; exit 1; }

[[ -f "${MANIFEST}" ]] || fail "update-cli.yaml fehlt: ${MANIFEST}"
MANIFEST_SCHEMA="$(manifest_schema "${MANIFEST}")"
[[ "${MANIFEST_SCHEMA}" =~ ^[0-9]+$ ]] || MANIFEST_SCHEMA=1

# The setup handler itself owns the fullscreen UI. Keep this wrapper quiet in
# interactive fullscreen mode so it does not leave redundant bootstrap output.
if [[ "${UPDATE_CLI_TUI:-auto}" == "plain" || ! -t 1 ]]; then
    printf '\n%s%s Setup%s\n' "${BOLD}" "${PROJECT_NAME}" "${RESET}"
    printf '%-18s %s\n' "Projektordner" "${ROOT_DIR}"
    printf '%-18s %s\n\n' "Manifest" "${MANIFEST}"
fi

# Return success only when the candidate advertises update-cli.yaml support.
# If a compatible handler starts and the setup itself fails, set -e propagates
# that failure instead of silently trying another implementation.
run_manifest_if_supported() {
    local candidate="$1"
    local label="$2"
    local candidate_help

    [[ -x "${candidate}" ]] || return 1
    candidate_help="$("${candidate}" --help 2>&1 || true)"
    grep -q -- '--setup-manifest' <<<"${candidate_help}" || return 1
    if (( MANIFEST_SCHEMA >= 2 )); then
        grep -q -- '--setup-list' <<<"${candidate_help}" || return 1
        grep -q -- '--setup-task' <<<"${candidate_help}" || return 1
        grep -q -- '--setup-workflow' <<<"${candidate_help}" || return 1
    fi

    if [[ "${UPDATE_CLI_TUI:-auto}" == "plain" || ! -t 1 ]]; then
        info "update-cli.yaml mit ${label} ausführen"
    fi
    if ! "${candidate}" --setup-manifest "${MANIFEST}" "${FORWARD_ARGS[@]}"; then
        fail "Setup mit ${label} fehlgeschlagen"
    fi
    if [[ "${UPDATE_CLI_TUI:-auto}" == "plain" || ! -t 1 ]]; then
        success "Setup abgeschlossen"
    fi
    exit 0
}
# Prefer a platform-matching binary from this checkout. This is important while
# Update CLI upgrades itself: the globally installed binary may still be an
# older schema-1-only release, and dist/update-cli may target another platform.
if suffix="$(platform_suffix 2>/dev/null)"; then
    run_manifest_if_supported "${ROOT_DIR}/dist/update-cli-${suffix}" "lokalem dist/update-cli-${suffix}" || true
fi
run_manifest_if_supported "${ROOT_DIR}/dist/update-cli" "lokalem dist/update-cli" || true
run_manifest_if_supported "${ROOT_DIR}/update-cli" "lokalem update-cli" || true

installed_cli="$(command -v update-cli 2>/dev/null || true)"
if [[ -n "${installed_cli}" ]]; then
    if run_manifest_if_supported "${installed_cli}" "installiertem update-cli"; then
        exit 0
    fi
    if [[ "${UPDATE_CLI_TUI:-auto}" == "plain" || ! -t 1 ]]; then
        info "Installiertes update-cli unterstützt update-cli.yaml Schema ${MANIFEST_SCHEMA} nicht; Bootstrap über Go"
    fi
fi

command -v go >/dev/null 2>&1 || fail "Weder ein kompatibles update-cli noch Go ist verfügbar"
cd -- "${ROOT_DIR}"
if [[ "${UPDATE_CLI_TUI:-auto}" == "plain" || ! -t 1 ]]; then
    info "Update CLI aus dem Quellcode für den Setup-Handler starten"
fi
go run . --setup-manifest "${MANIFEST}" "${FORWARD_ARGS[@]}"
if [[ "${UPDATE_CLI_TUI:-auto}" == "plain" || ! -t 1 ]]; then
    success "Setup abgeschlossen"
fi
