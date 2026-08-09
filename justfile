set shell := ["bash", "-euo", "pipefail", "-c"]

version := `tr -d '[:space:]' < VERSION`

default:
    @just -l

help:
    go run -ldflags "-X main.version={{version}}" . --help

howto:
    go run -ldflags "-X main.version={{version}}" . --howto

run *args:
    go run -ldflags "-X main.version={{version}}" . {{args}}

init *args:
    go run -ldflags "-X main.version={{version}}" . --init {{args}}

upgrade *args:
    go run -ldflags "-X main.version={{version}}" . --upgrade {{args}}

update *args:
    go run -ldflags "-X main.version={{version}}" . --update {{args}}

update-plan *args:
    go run -ldflags "-X main.version={{version}}" . --update --plan {{args}}

update-check *args:
    go run -ldflags "-X main.version={{version}}" . --check {{args}}

backup *args:
    go run -ldflags "-X main.version={{version}}" . --backup {{args}}

rollback *args:
    go run -ldflags "-X main.version={{version}}" . --rollback {{args}}

restore backup="latest" *args:
    go run -ldflags "-X main.version={{version}}" . --restore "{{backup}}" {{args}}

history *args:
    go run -ldflags "-X main.version={{version}}" . --history {{args}}

cleanup *args:
    go run -ldflags "-X main.version={{version}}" . --cleanup {{args}}

status *args:
    go run -ldflags "-X main.version={{version}}" . --status {{args}}

list *args:
    go run -ldflags "-X main.version={{version}}" . --list {{args}}

verify archive *args:
    go run -ldflags "-X main.version={{version}}" . --verify "{{archive}}" {{args}}

doctor *args:
    go run -ldflags "-X main.version={{version}}" . --doctor {{args}}

setup:
    ./setup.sh

setup-manifest file="setup.yaml":
    go run -ldflags "-X main.version={{version}}" . --setup-manifest "{{file}}"

setup-list:
    go run -ldflags "-X main.version={{version}}" . --setup-list

setup-task task *args:
    go run -ldflags "-X main.version={{version}}" . --setup-task "{{task}}" {{args}}

setup-workflow workflow *args:
    go run -ldflags "-X main.version={{version}}" . --setup-workflow "{{workflow}}" {{args}}

config *args:
    go run -ldflags "-X main.version={{version}}" . --config {{args}}

templates *args:
    go run -ldflags "-X main.version={{version}}" . --templates {{args}}

unlock:
    go run -ldflags "-X main.version={{version}}" . --unlock

fmt:
    gofmt -w .

fmt-check:
    @out="$(gofmt -l .)"; test -z "$out" || { printf '%s\n' "$out"; exit 1; }

vet:
    go vet ./...

test:
    go test ./...

test-race:
    go test -race ./...

check: fmt-check vet test test-race

build: check
    mkdir -p dist
    go build -trimpath -ldflags "-s -w -X main.version={{version}}" -o dist/update-cli .

build-macos-amd64: check
    mkdir -p dist
    GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version={{version}}" -o dist/update-cli-darwin-amd64 .

build-macos-arm64: check
    mkdir -p dist
    GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w -X main.version={{version}}" -o dist/update-cli-darwin-arm64 .

build-linux-amd64: check
    mkdir -p dist
    GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version={{version}}" -o dist/update-cli-linux-amd64 .

build-all: build-macos-amd64 build-macos-arm64 build-linux-amd64

deploy: build
    destination="$(go run ./cmd/buildconfig --field defaultDeploymentPath --expand)"; \
    config_path="$(go run ./cmd/buildconfig --field defaultConfigPath --expand)"; \
    mkdir -p "$destination" "$config_path"; \
    install -m 0755 dist/update-cli "$destination/update-cli"; \
    install -m 0755 setup-template.sh "$config_path/setup-template.sh"

clean:
    rm -rf dist update-cli
