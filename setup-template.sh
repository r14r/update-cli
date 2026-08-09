#!/usr/bin/env bash
set -Eeuo pipefail

# Generic Update CLI setup bootstrap.
# It resolves setup.yaml from the current project and delegates parsing,
# execution, TUI rendering and diagnostics to a compatible update-cli binary.

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd -P)"
PWD_DIR="$(pwd -P)"
MANIFEST=""
FORWARD_ARGS=()

usage() {
    cat <<'TXT'
Usage: setup-template.sh [--details] [--wait|--no-wait] [--fullscreen|--no-fullscreen] [--no-ui] [--list|--task NAME|--workflow NAME] [--config FILE]

The default manifest is ./setup.yaml (or ./setup.yml) in the current project
folder. If this template was copied into a project, its script directory is
also searched.
TXT
}

resolve_manifest() {
    local requested="${1:-}"
    local candidate
    if [[ -n "${requested}" ]]; then
        if [[ "${requested}" = /* ]]; then
            printf '%s\n' "${requested}"
        else
            printf '%s\n' "${PWD_DIR}/${requested}"
        fi
        return
    fi
    for candidate in "${PWD_DIR}/setup.yaml" "${PWD_DIR}/setup.yml"; do
        [[ -f "${candidate}" ]] && { printf '%s\n' "${candidate}"; return; }
    done
    if [[ "${SCRIPT_DIR}" != "${PWD_DIR}" ]]; then
        for candidate in "${SCRIPT_DIR}/setup.yaml" "${SCRIPT_DIR}/setup.yml"; do
            [[ -f "${candidate}" ]] && { printf '%s\n' "${candidate}"; return; }
        done
    fi
    return 1
}

manifest_schema() {
    awk '
        /^[[:space:]]*(schemaVersion|version)[[:space:]]*:/ {
            line=$0
            sub(/^[^:]*:[[:space:]]*/, "", line)
            gsub(/[[:space:]#].*$/, "", line)
            if (line ~ /^[0-9]+$/) { print line; exit }
        }
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

candidate_help() {
    "$1" --help 2>&1 || true
}

candidate_supports_manifest() {
    local candidate="$1"
    local schema="$2"
    local help_text
    [[ -x "${candidate}" ]] || return 1
    help_text="$(candidate_help "${candidate}")"
    grep -q -- '--setup-manifest' <<<"${help_text}" || return 1
    if (( schema >= 2 )); then
        grep -q -- '--setup-list' <<<"${help_text}" || return 1
        grep -q -- '--setup-task' <<<"${help_text}" || return 1
        grep -q -- '--setup-workflow' <<<"${help_text}" || return 1
    fi
    return 0
}

requested_config=""
while (($# > 0)); do
    case "$1" in
        --details|--wait|--no-wait)
            FORWARD_ARGS+=("$1"); shift ;;
        --list)
            FORWARD_ARGS+=("--setup-list"); shift ;;
        --task)
            (($# >= 2)) || { printf 'ERROR --task benötigt einen Namen\n' >&2; exit 2; }
            FORWARD_ARGS+=("--setup-task" "$2"); shift 2 ;;
        --workflow)
            (($# >= 2)) || { printf 'ERROR --workflow benötigt einen Namen\n' >&2; exit 2; }
            FORWARD_ARGS+=("--setup-workflow" "$2"); shift 2 ;;
        --no-ui|---no-ui)
            export UPDATE_CLI_TUI=plain
            FORWARD_ARGS+=("--no-ui"); shift ;;
        --fullscreen)
            export UPDATE_CLI_TUI=fullscreen; shift ;;
        --no-fullscreen)
            export UPDATE_CLI_TUI=plain; shift ;;
        --config)
            (($# >= 2)) || { printf 'ERROR --config benötigt eine Datei\n' >&2; exit 2; }
            requested_config="$2"; shift 2 ;;
        --help|-h)
            usage; exit 0 ;;
        *)
            printf 'ERROR unbekannte Option: %s\n' "$1" >&2
            usage >&2
            exit 2 ;;
    esac
done

if ! MANIFEST="$(resolve_manifest "${requested_config}")"; then
    printf 'ERROR setup.yaml/setup.yml im aktuellen Projektordner nicht gefunden: %s\n' "${PWD_DIR}" >&2
    exit 1
fi
MANIFEST="$(cd -- "$(dirname -- "${MANIFEST}")" >/dev/null 2>&1 && pwd -P)/$(basename -- "${MANIFEST}")"
SCHEMA="$(manifest_schema "${MANIFEST}")"
[[ "${SCHEMA}" =~ ^[0-9]+$ ]] || SCHEMA=1

export UPDATE_CLI_TUI="${UPDATE_CLI_TUI:-auto}"

# UPDATE_CLI_BIN is an explicit override. Do not silently ignore an incompatible
# binary because that would hide a broken deployment configuration.
if [[ -n "${UPDATE_CLI_BIN:-}" ]]; then
    if ! candidate_supports_manifest "${UPDATE_CLI_BIN}" "${SCHEMA}"; then
        printf 'ERROR UPDATE_CLI_BIN unterstützt setup.yaml Schema %s nicht: %s\n' "${SCHEMA}" "${UPDATE_CLI_BIN}" >&2
        exit 1
    fi
    exec "${UPDATE_CLI_BIN}" --setup-manifest "${MANIFEST}" "${FORWARD_ARGS[@]}"
fi

candidates=()
if suffix="$(platform_suffix 2>/dev/null)"; then
    candidates+=("${PWD_DIR}/dist/update-cli-${suffix}")
    [[ "${SCRIPT_DIR}" == "${PWD_DIR}" ]] || candidates+=("${SCRIPT_DIR}/dist/update-cli-${suffix}")
fi
candidates+=("${PWD_DIR}/dist/update-cli" "${PWD_DIR}/update-cli")
if [[ "${SCRIPT_DIR}" != "${PWD_DIR}" ]]; then
    candidates+=("${SCRIPT_DIR}/dist/update-cli" "${SCRIPT_DIR}/update-cli")
fi
installed_cli="$(command -v update-cli 2>/dev/null || true)"
[[ -z "${installed_cli}" ]] || candidates+=("${installed_cli}")

for candidate in "${candidates[@]}"; do
    [[ -n "${candidate}" ]] || continue
    if candidate_supports_manifest "${candidate}" "${SCHEMA}"; then
        exec "${candidate}" --setup-manifest "${MANIFEST}" "${FORWARD_ARGS[@]}"
    fi
done

# A source checkout is itself a valid bootstrap path. This breaks the upgrade
# dependency on an already-installed schema-v2 binary: when current/ contains
# the newer Update CLI sources but PATH still points to a 3.0.x installation,
# execute the setup engine from source and let the setup workflow deploy the
# freshly built binary.
if command -v go >/dev/null 2>&1 \
    && [[ -f "${PWD_DIR}/go.mod" ]] \
    && [[ -f "${PWD_DIR}/main.go" ]]; then
    version_ldflags=()
    if [[ -f "${PWD_DIR}/VERSION" ]]; then
        bootstrap_version="$(tr -d '[:space:]' < "${PWD_DIR}/VERSION")"
        [[ -z "${bootstrap_version}" ]] || version_ldflags=(-ldflags "-X main.version=${bootstrap_version}")
    fi
    cd -- "${PWD_DIR}"
    exec go run "${version_ldflags[@]}" . --setup-manifest "${MANIFEST}" "${FORWARD_ARGS[@]}"
fi

if (( SCHEMA >= 2 )); then
    printf 'ERROR setup.yaml verwendet Schema %s, aber kein kompatibles Update CLI wurde gefunden.\n' "${SCHEMA}" >&2
    printf '      Benötigt wird Update CLI 3.1.0 oder neuer bzw. ein passendes lokales dist/update-cli-<os>-<arch>.\n' >&2
else
    printf 'ERROR kein kompatibles update-cli für setup.yaml gefunden.\n' >&2
fi
exit 1
