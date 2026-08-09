
## Manifest lifecycle commands

Update CLI can manage setup files directly:

```bash
update-cli --convert-yaml
update-cli --create-yaml
update-cli --create-setup-script
```

`--convert-yaml` converts schema 1 to schema 2 while preserving the old file as a backup. `--create-yaml` detects project markers for Go, Python, Node, Laravel and Docker Compose and generates an editable schema-2 sample. `--create-setup-script` creates the generic executable wrapper. Add `--dry-run` to preview and `--force` to overwrite generated files.

# setup.yaml schema

Update CLI 3.1.0 supports two manifest generations:

- **schemaVersion 1**: full backward compatibility with the established Update CLI 2.14 `id/name/when/run/cwd/allowFailure` format and the typed 3.0 handler format.
- **schemaVersion 2**: declarative workflows, reusable tasks, dependencies, variables, requirements, structured conditions, execution controls, typed operations, and `command`/`shell` escape hatches.

Schema 1 is not deprecated by 3.1.0; existing project archives continue to work unchanged.

## Schema 2 overview

```yaml
schemaVersion: 2

project:
  name: Example Project
  type: go
  description: Build, test and deploy the project

defaults:
  timeout: 10m
  failFast: true

variables:
  binary: example
  distDir: dist
  deployPath: "{{ env.EXAMPLE_DEPLOY_PATH | /usr/local/bin/example }}"

requirements:
  commands:
    - go
  optionalCommands:
    - just

workflows:
  setup:
    tasks: [deploy]
  ci:
    tasks: [verify]

tasks:
  prepare:
    steps:
      - name: Download modules
        go:
          action: mod-download

  check:
    requires: [prepare]
    steps:
      - name: Static analysis
        go:
          action: vet
      - name: Tests
        go:
          action: test

  build:
    requires: [check]
    steps:
      - name: Build
        shell: |
          VERSION="$(cat VERSION)"
          mkdir -p "{{ distDir }}"
          go build -o "{{ distDir }}/{{ binary }}" .

  verify:
    requires: [build]
    steps:
      - name: Binary exists
        assert:
          executable: "{{ distDir }}/{{ binary }}"

  deploy:
    requires: [verify]
    steps:
      - name: Deploy binary
        deploy:
          source: "{{ distDir }}/{{ binary }}"
          target: "{{ deployPath }}"
          mode: "0755"
```

## Workflows and tasks

A workflow is an ordered entry point composed of tasks. Tasks may depend on other tasks using `requires`. Dependencies are executed first, only once, and dependency cycles are rejected.

```yaml
workflows:
  setup:
    tasks: [deploy]
  ci:
    tasks: [verify]
  clean:
    tasks: [clean]

tasks:
  build:
    requires: [check]
    steps:
      - ...
```

CLI:

```bash
update-cli --setup                 # workflow setup
update-cli --setup-list            # workflows/tasks
update-cli --setup-task build
update-cli --setup-workflow ci
update-cli --setup-manifest ./setup.yaml --setup-task test
```

The global `setup-template.sh` also supports:

```bash
./setup.sh --list
./setup.sh --task build
./setup.sh --workflow ci
```

## Variables

Declared variables and built-ins use `{{ ... }}` substitution.

```yaml
variables:
  distDir: dist
  binary: app
  output: "{{ distDir }}/{{ binary }}"
  deployPath: "{{ env.APP_DEPLOY_PATH | /usr/local/bin/app }}"
```

Built-ins:

- `{{ project.root }}`
- `{{ project.name }}`
- `{{ project.type }}`
- `{{ os }}`
- `{{ arch }}`
- `{{ env.NAME }}`
- `{{ env.NAME | fallback }}`

Variables are expanded in step names, working directories, environment values, conditions, and operation configuration.

## Requirements

```yaml
requirements:
  commands: [go, git]
  optionalCommands: [just]
```

Missing required commands stop setup before execution. Optional commands are diagnostic only.

## Step controls

Every schema-2 step supports:

```yaml
- id: tests
  name: Run tests
  cwd: backend
  env:
    APP_ENV: testing
  timeout: 5m
  retries: 2
  allowFailure: false
  when:
    fileExists: backend/go.mod
  command:
    exec: go
    args: [test, ./...]
```

`defaults.timeout` is used when a step has no explicit timeout. `defaults.failFast` defaults to `true`.

## Conditions

Simple conditions:

```yaml
when:
  fileExists: go.mod
```

Supported condition keys:

- `fileExists`
- `fileNotExists`
- `directoryExists`
- `commandExists`
- `envSet`
- `os`
- `arch`
- `compose`

Compound conditions:

```yaml
when:
  all:
    - fileExists: compose.yaml
    - commandExists: docker
    - not:
        envSet: SKIP_DOCKER
```

```yaml
when:
  any:
    - os: darwin
    - os: linux
```

`os` and `arch` may also use lists:

```yaml
when:
  os: [darwin, linux]
```

Paths used by conditions are relative to the project root and may not escape it.

## Command operations

Structured command execution is preferred because arguments are not re-parsed by a shell:

```yaml
command:
  exec: go
  args: [test, -race, ./...]
```

Use `shell` for pipelines, variable assignments, command substitution, or other shell syntax:

```yaml
shell: |
  VERSION="$(cat VERSION)"
  go build -ldflags "-X main.version=${VERSION}" .
```

## Filesystem operations

### mkdir

```yaml
mkdir: build
```

or:

```yaml
mkdir:
  paths: [data, logs, cache]
  mode: "0755"
```

### copy

```yaml
copy:
  source: .env.example
  target: .env
  overwrite: false
```

`copy` destinations must remain inside the project.

### deploy

`deploy` is the explicit operation for destinations outside the project:

```yaml
deploy:
  source: dist/app
  target: /usr/local/bin/app
  mode: "0755"
```

### move / remove / chmod / symlink / touch / write

```yaml
move:
  source: generated/config.yaml
  target: config/config.yaml
```

```yaml
remove:
  paths: [dist, build, cache]
  recursive: true
```

```yaml
chmod:
  path: bin/app
  mode: "0755"
```

```yaml
symlink:
  source: releases/current
  target: active
  replace: true
```

```yaml
touch:
  paths: [data/.gitkeep, logs/.gitkeep]
```

```yaml
write:
  path: config/generated.env
  content: |
    APP_ENV=production
  mode: "0644"
  overwrite: false
```

## Assertions

Assertions are explicit verification steps:

```yaml
assert:
  fileExists: dist/app
```

Supported assertions:

- `fileExists`
- `directoryExists`
- `executable`
- `commandExists`
- `envSet`
- `portAvailable`
- `http`

HTTP assertion:

```yaml
assert:
  http:
    url: http://localhost:8080/health
    status: 200
    timeout: 10s
```

## Environment preparation

Python virtual environment:

```yaml
pythonVenv:
  path: .venv
  python: python3
```

Python packages:

```yaml
pip:
  python: .venv/bin/python
  requirements: requirements.txt
```

A typical `.env` preparation can use `copy` plus a condition:

```yaml
- name: Create local environment file
  copy:
    source: .env.example
    target: .env
    overwrite: false
  when:
    all:
      - fileExists: .env.example
      - not:
          fileExists: .env
```

## Go

Actions:

- `mod-download`
- `vet`
- `test`
- `generate`
- `build`

```yaml
go:
  action: test
  args: [-race]
```

```yaml
go:
  action: build
  package: ./cmd/app
  output: dist/app
  args: [-trimpath]
```

For complex linker flags that require shell command substitution, use `shell`.

## Node package managers

Supported operations: `npm`, `pnpm`, `yarn`.

```yaml
npm:
  action: install
```

```yaml
npm:
  action: build
  args: [--mode, production]
```

For npm install, `package-lock.json` automatically selects `npm ci`.

## PHP / Laravel

Composer:

```yaml
composer:
  action: install
  production: true
```

Artisan:

```yaml
artisan:
  command: migrate
  args: [--force]
```

## Docker Compose

```yaml
dockerCompose:
  action: up
  detach: true
  removeOrphans: true
```

Supported Compose actions are passed to `docker compose`, for example `build`, `pull`, `up`, `down`, `restart`, and other explicit action names. Optional fields include `file`, `profiles`, `args`, `detach`, and `removeOrphans`.

## HTTP checks

```yaml
httpCheck:
  url: http://localhost:8080/health
  status: 200
  timeout: 15s
```

## Download and extract

```yaml
download:
  url: https://example.com/archive.zip
  destination: tmp/archive.zip
  sha256: 012345...
```

Downloads are capped at 512 MiB and may be checksum-verified.

```yaml
extract:
  archive: tmp/archive.zip
  destination: vendor/archive
```

ZIP extraction rejects absolute paths, traversal, and symlinks.

## TUI and direct output

All workflows/tasks use the same Update CLI renderer:

```bash
update-cli --setup
update-cli --setup-task test
update-cli --setup-workflow ci
```

`--no-ui` bypasses the fullscreen TUI and streams commands/stdout/stderr directly. `--details` shows command lines in addition to command output. `--no-wait` closes automatically after fullscreen completion.

## Schema 1 compatibility

The established format remains supported:

```yaml
schemaVersion: 1
project:
  name: Existing Project
  type: go
steps:
  - id: test
    name: Tests
    when: file:go.mod
    run: go test ./...
    cwd: .
    allowFailure: false
```

Schema-1 conditions remain `always`, `file:`, `not-file:`, `dir:`, `command:`, `env:`, `compose`, and `os:`. Existing typed version-1 handlers (`go`, `python`, `node`, `laravel`, `docker-compose`, `copy`, `deploy`, `command`) remain available.
