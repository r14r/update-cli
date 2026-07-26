set shell := ["bash", "-euo", "pipefail", "-c"]

version := `tr -d '[:space:]' < VERSION`

# Verfügbare Befehle anzeigen.
default:
    @just --list

# Kurze Übersicht der verfügbaren CLI-Befehle anzeigen.
help:
    go run -ldflags "-X main.version={{version}}" . --help

# Ausführliche Verwendung, Optionen und Beispiele anzeigen.
howto:
    go run -ldflags "-X main.version={{version}}" . --howto

# Projektkonfiguration initialisieren. Beispiel: just init mediastudio
init *args:
    go run -ldflags "-X main.version={{version}}" . --init {{args}}

# Bestehende config.json auf das aktuelle Schema aktualisieren.
upgrade *args:
    go run -ldflags "-X main.version={{version}}" . --upgrade {{args}}

# Updater direkt mit Go starten. Beispiel: just run --howto
run *args:
    go run -ldflags "-X main.version={{version}}" . {{args}}

# Neueste oder angegebene Release-Version installieren.
update *args:
    go run -ldflags "-X main.version={{version}}" . --update {{args}}

# Release installieren und anschließend das Projekt-Setup ausführen.
update-setup *args:
    go run -ldflags "-X main.version={{version}}" . --update --setup {{args}}

# Release direkt von einer HTTP(S)-URL installieren.
update-url url *args:
    go run -ldflags "-X main.version={{version}}" . --update --from url --url "{{url}}" {{args}}

# Release direkt aus einem Git-Repository installieren.
update-repository repository *args:
    go run -ldflags "-X main.version={{version}}" . --update --from repository --repository "{{repository}}" {{args}}

# Current als Backup sichern.
backup *args:
    go run -ldflags "-X main.version={{version}}" . --backup {{args}}

# Vorheriges oder angegebenes Release aktivieren.
rollback *args:
    go run -ldflags "-X main.version={{version}}" . --rollback {{args}}

# Backup wiederherstellen. Beispiel: just restore latest
restore backup *args:
    go run -ldflags "-X main.version={{version}}" . --restore "{{backup}}" {{args}}

# Update-Historie anzeigen.
history *args:
    go run -ldflags "-X main.version={{version}}" . --history {{args}}

# Alte Releases und Backups entfernen.
cleanup *args:
    go run -ldflags "-X main.version={{version}}" . --cleanup {{args}}

# Cleanup nur planen.
cleanup-plan *args:
    go run -ldflags "-X main.version={{version}}" . --cleanup --plan {{args}}

# Projekt- und Versionsstatus anzeigen.
status *args:
    go run -ldflags "-X main.version={{version}}" . --status {{args}}

# Release-Quellen und entpackte Releases auflisten.
list *args:
    go run -ldflags "-X main.version={{version}}" . --list {{args}}

# Release-Archiv prüfen. Beispiel: just verify ~/Downloads/mediastudio-v3.8.1.zip
verify archive *args:
    go run -ldflags "-X main.version={{version}}" . --verify {{args}} "{{archive}}"

# Detaillierten Update-Plan ohne Änderungen anzeigen.
update-plan *args:
    go run -ldflags "-X main.version={{version}}" . --update --plan {{args}}

# Prüfen, ob eine neue Release-Version verfügbar ist.
update-check *args:
    go run -ldflags "-X main.version={{version}}" . --check {{args}}

# Umgebung und Projektkonfiguration diagnostizieren.
doctor *args:
    go run -ldflags "-X main.version={{version}}" . --doctor {{args}}

# Projekt-Setup im current-Ordner ausführen.
setup *args:
    go run -ldflags "-X main.version={{version}}" . --setup {{args}}

# Aktuelle Updater-Konfiguration anzeigen.
config *args:
    go run -ldflags "-X main.version={{version}}" . --config {{args}}

# Konfigurationsdateien auflisten.
config-list *args:
    go run -ldflags "-X main.version={{version}}" . --config --list {{args}}

# Aktuelle Updater-Konfiguration im Editor öffnen.
config-edit *args:
    go run -ldflags "-X main.version={{version}}" . --config --edit {{args}}

# Template in config.json übernehmen. Beispiel: just config-template Laravel
config-template template *args:
    go run -ldflags "-X main.version={{version}}" . --config --use-template "{{template}}" {{args}}

# Templates aus .updater-cli/templates.json auflisten.
templates-list *args:
    go run -ldflags "-X main.version={{version}}" . --templates --list {{args}}

# Templates mit Aktionen und Setup-Kommandos auflisten.
templates-details *args:
    go run -ldflags "-X main.version={{version}}" . --templates --list --details {{args}}

# Template aus templates.json anwenden.
templates-use template *args:
    go run -ldflags "-X main.version={{version}}" . --templates --use "{{template}}" {{args}}

# templates.json im Editor öffnen und benanntes Template validieren.
templates-edit template *args:
    go run -ldflags "-X main.version={{version}}" . --templates --edit "{{template}}" {{args}}

# CLI-Version anzeigen.
version-info:
    go run -ldflags "-X main.version={{version}}" . --version

# Go-Quellcode formatieren.
fmt:
    gofmt -w .

# Statische Prüfung ausführen.
vet:
    go vet ./...

# Unit-Tests ausführen.
test:
    go test ./...

# Build-Konfiguration validieren.
build-config:
    go run ./cmd/buildconfig --validate

# Eingebettete Build-Standards anzeigen.
build-config-show:
    go run ./cmd/buildconfig

# Formatierung, Build-Konfiguration, Vet und Tests ausführen.
check: fmt build-config vet test

# Lokales Binary unter ./dist/update-cli erstellen.
build: check
    mkdir -p dist
    go build -trimpath -ldflags "-s -w -X main.version={{version}}" -o dist/update-cli .
    @echo "Erstellt: dist/update-cli"

# Binary als ./update-cli in den Projektordner installieren.
install: check
    go build -trimpath -ldflags "-s -w -X main.version={{version}}" -o update-cli .
    @echo "Installiert: ./update-cli"

# Binary in den in build-config.json definierten Deployment-Ordner installieren.
deploy: build
    destination="$(go run ./cmd/buildconfig --field defaultDeploymentPath --expand)"; \
    config_path="$(go run ./cmd/buildconfig --field defaultConfigPath --expand)"; \
    mkdir -p "$destination" "$config_path"; \
    install -m 0755 dist/update-cli "$destination/update-cli"; \
    echo "Binary:          $destination/update-cli"; \
    echo "Globale Config: $config_path"

# macOS Intel-Binary erstellen.
build-macos-amd64: check
    mkdir -p dist
    GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version={{version}}" -o dist/update-cli-darwin-amd64 .

# macOS Apple-Silicon-Binary erstellen.
build-macos-arm64: check
    mkdir -p dist
    GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w -X main.version={{version}}" -o dist/update-cli-darwin-arm64 .

# Linux x86-64-Binary erstellen.
build-linux-amd64: check
    mkdir -p dist
    GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version={{version}}" -o dist/update-cli-linux-amd64 .

# Alle unterstützten Plattform-Binaries erstellen.
build-all: build-macos-amd64 build-macos-arm64 build-linux-amd64

# Generierte Dateien entfernen.
clean:
    rm -rf dist update-cli
