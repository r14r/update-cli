set shell := ["bash", "-euo", "pipefail", "-c"]

# -----------------------------------------------------------------------------
# Setup and dependencies
# -----------------------------------------------------------------------------

default:
    @just -l

bootstrap:
    @command -v python3 >/dev/null || { echo "Python 3 required" >&2; exit 1; }
    @command -v go >/dev/null || { echo "Go required" >&2; exit 1; }
    python3 -m venv .venv
    .venv/bin/python -m pip install --upgrade pip pre-commit
    go mod download
    just hooks-install

deps:
    go mod download

setup:
    ./setup.sh

setup-manifest file="update-cli.yaml":
    go run . setup --manifest "{{file}}"

setup-list:
    go run . setup --list

setup-task task *args:
    go run . setup --task "{{task}}" {{args}}

setup-workflow workflow *args:
    go run . setup --workflow "{{workflow}}" {{args}}

init *args:
    go run . init {{args}}

# -----------------------------------------------------------------------------
# Development and diagnostics
# -----------------------------------------------------------------------------

run *args:
    go run . {{args}}

help:
    go run . help

howto:
    go run . howto

doctor *args:
    go run . doctor {{args}}

status *args:
    go run . status {{args}}

config *args:
    go run . config {{args}}

templates *args:
    go run . templates {{args}}

unlock:
    go run . unlock

# -----------------------------------------------------------------------------
# Quality and Git hooks
# -----------------------------------------------------------------------------

fmt:
    gofmt -w .

fmt-check:
    @out="$(gofmt -l .)"; test -z "$out" || { printf '%s\n' "$out"; exit 1; }

vet:
    go vet ./...

test:
    go test -count=1 ./...

test-race:
    go test -count=1 -race ./...

check: fmt-check vet test test-race

validate-version:
    ./scripts/validate-version.sh . >/dev/null

hooks-install:
    chmod +x .githooks/pre-commit .githooks/pre-push
    git config --local core.hooksPath .githooks

# Disable the project Git hooks in this clone.

hooks-uninstall:
    git config --local --unset core.hooksPath || true

# Run the same checks as the local pre-commit hook.

precommit:
    bash scripts/precommit.sh --commit

# Run the more expensive checks before pushing.

pre-commit: precommit

prepush:
    bash scripts/precommit.sh --push

# Run the complete CI quality gate locally.

precommit-full: precommit-all

precommit-all:
    bash scripts/precommit.sh --all

pre-commit-all: precommit-all

ci: precommit-all

# -----------------------------------------------------------------------------
# Build installation and release
# -----------------------------------------------------------------------------

build: validate-version check
    mkdir -p dist
    go build -trimpath -ldflags "-s -w" -o dist/update-cli .

build-macos-amd64: check
    mkdir -p dist
    GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/update-cli-darwin-amd64 .

build-macos-arm64: check
    mkdir -p dist
    GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o dist/update-cli-darwin-arm64 .

build-linux-amd64: check
    mkdir -p dist
    GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/update-cli-linux-amd64 .

build-all: build-macos-amd64 build-macos-arm64 build-linux-amd64

install: build
    @destination="${INSTALL_DIR:-/usr/local/bin}"; \
    config_path="${UPDATE_CLI_CONFIG_DIR:-/usr/local/etc/update-cli}"; \
    mkdir -p "$destination" "$config_path/prompts"; \
    install -m 0755 dist/update-cli "$destination/update-cli"; \
    install -m 0755 setup-template.sh "$config_path/setup-template.sh"; \
    install -m 0644 prompts/setup-script-to-yaml.txt "$config_path/prompts/setup-script-to-yaml.txt"; \
    install -m 0644 doc/examples/ai.json "$config_path/ai.json.example"; \
    test -f "$config_path/config.json" || install -m 0644 defaults/config.json "$config_path/config.json"; \
    test -f "$config_path/templates.json" || install -m 0644 defaults/templates.json "$config_path/templates.json"; \
    echo "Installed $destination/update-cli"

deploy: install

release: build-all

package-source output="":
    @if [ -n "{{output}}" ]; then ./scripts/package-source.sh "{{output}}"; else ./scripts/package-source.sh; fi

# Render the terminal demos used by README INSTALL and QUICKSTART.

clear-build:
    rm -rf dist update-cli

# -----------------------------------------------------------------------------
# Upgrade commands
# -----------------------------------------------------------------------------

upgrade *args:
    go run . upgrade {{args}}

migrate *args:
    go run . migrate {{args}}

recover *args:
    go run . recover {{args}}

whatsnew *args:
    go run . whatsnew {{args}}

update *args:
    go run . update {{args}}

# No-argument lifecycle: update, then prompt for bootstrap and install.
lifecycle:
    go run .

update-plan *args:
    go run . update --plan {{args}}

update-check *args:
    go run . check {{args}}

releases *args:
    go run . releases --list {{args}}

legacy-list *args:
    go run . list {{args}}

verify archive *args:
    go run . verify --archive "{{archive}}" {{args}}

# -----------------------------------------------------------------------------
# Backup and maintenance
# -----------------------------------------------------------------------------

backup *args:
    go run . backup {{args}}

rollback *args:
    go run . rollback {{args}}

restore backup="latest" *args:
    go run . restore --snapshot "{{backup}}" {{args}}

history *args:
    go run . history {{args}}

clean *args:
    go run . clean {{args}}

cleanup *args:
    go run . cleanup {{args}}

clear-releases *args:
    go run . clean {{args}}

# -----------------------------------------------------------------------------
# Manifest generation
# -----------------------------------------------------------------------------

convert-yaml *args:
    go run . convert-yaml {{args}}

create-yaml from="project" *args:
    go run . create-yaml --from "{{from}}" {{args}}

create-yaml-ai *args:
    go run . create-yaml --from setup-script --with-ai {{args}}

create-setup-script *args:
    go run . create-setup-script {{args}}

# -----------------------------------------------------------------------------
# Documentation demos
# -----------------------------------------------------------------------------

tape-install:
    ./scripts/render-tapes.sh docs/tapes/install.tape

tape-quickstart:
    ./scripts/render-tapes.sh docs/tapes/quickstart.tape

tapes: tape-install tape-quickstart

# Install native Git hooks for this clone.
