set shell := ["bash", "-euo", "pipefail", "-c"]


default:
    @just -l

help:
    go run . help

howto:
    go run . howto

run *args:
    go run . {{args}}

init *args:
    go run . init {{args}}

upgrade *args:
    go run . upgrade {{args}}

update *args:
    go run . update {{args}}

update-plan *args:
    go run . update --plan {{args}}

update-check *args:
    go run . check {{args}}

backup *args:
    go run . backup {{args}}

rollback *args:
    go run . rollback {{args}}

restore backup="latest" *args:
    go run . restore "{{backup}}" {{args}}

history *args:
    go run . history {{args}}

clean *args:
    go run . clean {{args}}

cleanup *args:
    go run . cleanup {{args}}

status *args:
    go run . status {{args}}

releases *args:
    go run . releases --list {{args}}

legacy-list *args:
    go run . list {{args}}

verify archive *args:
    go run . verify "{{archive}}" {{args}}

doctor *args:
    go run . doctor {{args}}

setup:
    ./setup.sh

setup-manifest file="update-cli.yaml":
    go run . setup manifest "{{file}}"

setup-list:
    go run . setup list

setup-task task *args:
    go run . setup task "{{task}}" {{args}}

convert-yaml *args:
    go run . convert-yaml {{args}}

create-yaml from="project" *args:
    go run . create-yaml --from "{{from}}" {{args}}

create-yaml-ai *args:
    go run . create-yaml --from setup-script --with-ai {{args}}

create-setup-script *args:
    go run . create-setup-script {{args}}

setup-workflow workflow *args:
    go run . setup workflow "{{workflow}}" {{args}}

config *args:
    go run . config {{args}}

templates *args:
    go run . templates {{args}}

unlock:
    go run . unlock

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

install:
    test -x dist/update-cli || { echo "ERROR dist/update-cli fehlt; zuerst 'just build' ausführen" >&2; exit 1; }
    destination="${UPDATE_CLI_INSTALL_BIN_DIR:-$(go run ./cmd/buildconfig --field defaultDeploymentPath --expand)}"; \
    install_prefix="$(dirname "$destination")"; \
    config_path="$install_prefix/etc/update-cli"; \
    mkdir -p "$destination" "$config_path" "$config_path/prompts"; \
    install -m 0755 dist/update-cli "$destination/update-cli"; \
    install -m 0755 setup-template.sh "$config_path/setup-template.sh"; \
    install -m 0644 prompts/setup-script-to-yaml.txt "$config_path/prompts/setup-script-to-yaml.txt"; \
    install -m 0644 doc/examples/ai.json "$config_path/ai.json.example"; \
    test -f "$config_path/config.json" || install -m 0644 defaults/config.json "$config_path/config.json"; \
    test -f "$config_path/templates.json" || install -m 0644 defaults/templates.json "$config_path/templates.json"

clear-releases *args:
    go run . clean {{args}}

clear-build:
    rm -rf dist update-cli

package-source output="":
    @if [ -n "{{output}}" ]; then ./scripts/package-source.sh "{{output}}"; else ./scripts/package-source.sh; fi

# Render the terminal demos used by README INSTALL and QUICKSTART.
tape-install:
    ./scripts/render-tapes.sh docs/tapes/install.tape

tape-quickstart:
    ./scripts/render-tapes.sh docs/tapes/quickstart.tape

tapes: tape-install tape-quickstart
