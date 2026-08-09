# Release notes

## 3.2.2

- replace fullscreen inline `[j/N]` confirmation prompts with centered modal dialogs
- use the modal for both `Update jetzt installieren?` and `Projekt-Setup ist verfügbar. Jetzt ausführen?`
- show separate `YES` and `NO` button boxes; default-No confirmations visually select `NO`
- keep the footer status-only while a confirmation modal is visible
- repaint the unchanged underlying screen after the answer instead of writing `Eingabe verarbeitet` into the footer
- keep plain/`--no-ui` confirmation syntax compatible with `[j/N]`
- add PTY regression coverage for both update and setup confirmation modals

## 3.2.1

- after an interactive setup confirmation is accepted, clear only the fullscreen scroll/content area before project setup starts
- preserve the update header and project information while setup receives a clean viewport
- change the footer to `RUN  Projekt-Setup läuft` during the handoff
- leave update content untouched when setup is declined
- add UI and PTY regression coverage for the update -> setup content reset

## 3.2.0

- add `--convert-yaml` to migrate schemaVersion 1 setup manifests to schemaVersion 2 with backup and validation
- add `--create-yaml` to detect Go, Python, Node, Laravel and Docker Compose projects and generate a schemaVersion 2 sample manifest
- support multi-stack detection such as Go + Node + Docker
- add `--create-setup-script` plus compatibility alias `-create-setup-script`
- generated setup scripts are schema-aware bootstraps and can use local/platform binaries, PATH or Go source bootstrap
- add overwrite protection with `--force` and preview mode with `--dry-run`
- setup management commands target configured `currentDir` when an Update CLI project config exists, otherwise the current project directory
- add conversion, generation, detection, overwrite and CLI integration tests

## 3.1.2

- Fix macOS project-root regression test by comparing canonical paths (`/var` and `/private/var`).
- Make `setup-template.sh` schema-aware and prevent schemaVersion 2 from being executed by an incompatible 3.0.x global binary.
- Prefer platform-specific packaged binaries for setup bootstrap.
- Add source bootstrap via `go run` when the current checkout contains a newer schema-2-capable Update CLI but the globally installed binary is still old.
- Keep upward `.updater-cli/config.json` discovery for commands executed from `current/` or nested project directories.

## 3.1.0

- introduced `setup.yaml` schemaVersion 2 as a declarative project automation model
- added named workflows and reusable tasks with dependency resolution, de-duplication, and cycle detection
- added `--setup-list`, `--setup-task NAME`, and `--setup-workflow NAME`
- explicit manifests can combine `--setup-manifest` with task/workflow selection
- added task variables and built-ins including `{{ env.NAME | fallback }}`
- added required and optional command requirements
- added structured `when` conditions with `all`, `any`, and `not`
- added per-step `cwd`, environment, timeout, retries, and allow-failure controls
- added typed operations for commands, shell, filesystem preparation, assertions, Python environments/packages, Go, Node package managers, Composer/Artisan, Docker Compose, HTTP checks, downloads, ZIP extraction, and explicit deployment
- retained `command` and `shell` as generic escape hatches so arbitrary project setup operations remain possible
- converted Update CLI's own `setup.yaml` to schemaVersion 2 with `prepare`, `check`, `build`, `verify`, `deploy`, and `clean` tasks and `setup`/`ci` workflows
- `setup.sh` and the global setup template now support `--list`, `--task NAME`, and `--workflow NAME`
- schemaVersion 1 remains fully supported

## 3.0.14

- fullscreen footer is now reserved exclusively for screen state, confirmation questions, and final success/failure
- setup/update step renderers no longer write running/completed/skipped step labels into the footer
- nested setup during an update keeps the update project information fixed instead of replacing it with setup metadata
- nested setup metadata, step rows, stdout and stderr are rendered only in the scrollable content region
- check -> confirmed update -> setup now uses the same update screen model as a direct `--update` invocation
- added regression tests that fail if setup/update/status rows overwrite the footer
- added PTY coverage for check -> update -> setup with visible nested setup output and a stable `RUN  Update läuft` footer

## 3.0.13

- added global `--no-ui` mode for check/update/setup workflows
- `--no-ui` never enters the fullscreen alternate screen and streams setup/process stdout and stderr directly
- direct mode automatically enables setup command details so executed shell commands are visible with their output
- `--no-ui` is forwarded by `setup.sh` and `/usr/local/etc/update-cli/setup-template.sh`
- compatibility spelling `---no-ui` is normalized to `--no-ui`
- confirmed check-to-update transitions retain `--no-ui`
- PTY regression coverage verifies that `--no-ui` overrides even `UPDATE_CLI_TUI=fullscreen`

## 3.0.12

- fullscreen command stdout/stderr is now streamed exclusively into the scrollable content region; child/log lines no longer overwrite the footer
- the footer is reserved for the currently running step, confirmation prompts, and final success/failure status
- setup step rows no longer include an `INFO` prefix and use zero-padded counters such as `[01/08]`
- update transaction progress rows now include zero-padded counters such as `[07/13]` while retaining aligned progress bars, percentage columns, and right-aligned status icons
- multiline `run: |` / `command: |` blocks are supported by the schema-1 setup parser
- malformed non-indented block bodies now produce a specific indentation diagnostic
- copied Markdown-style `when: file\:go.mod` values are normalized to `file:go.mod` for compatibility
- the x-cli migration example now derives its Go module using `go list -m` instead of hard-coding a GitHub module path
- PTY regression coverage verifies visible successful step output, zero-padded counters, no `INFO` step prefix, and the separated footer/content behavior

## 3.0.11

- migrated the supplied x-cli legacy setup flow to a declarative `setup.yaml` example
- legacy child `setup.sh` executions are forced to plain/no-wait mode so a hidden nested Enter prompt cannot freeze the parent TUI
- legacy setup execution is represented as a visible parent-owned setup step
- setup step rows use one persistent completion row instead of separate start/completion lines

## 3.0.10

- Update-Transaktionsphasen verwenden jetzt ein fest ausgerichtetes Einzeilen-Layout mit Progressbar, Prozentwert, Label und rechtsbündigem Statussymbol.
- Erfolgreiche Update-Phasen zeigen ein grünes `✓`, fehlgeschlagene ein rotes `✗`, übersprungene ein gelbes `–`.
- Interne Snapshot-Statusmeldungen werden nicht mehr als zusätzliche INFO/OK-Zeilen zwischen den Transaktionsphasen ausgegeben.
- SKIP-Phasen verwenden dieselbe Progress-Zeile wie ausgeführte Phasen, inklusive Begründung.
- Setup-Schritte behalten das kompakte `INFO [n/N]`-Einzeilenformat aus 3.0.9.

## 3.0.9

Fullscreen TUI layout and compact step-status release.

- fullscreen layout is split into four regions: Header, project/setup information, auto-scrolling install steps, and Footer
- setup and update steps now occupy exactly one persistent row instead of separate `INFO` and `OK ... abgeschlossen` lines
- successful step rows end in a green `✓`; failures use a red `✗`; skipped phases use a yellow dash
- the transaction snapshot is reported by the numbered update phase itself, removing the duplicate inner INFO/OK pair
- setup project metadata stays fixed in the top information area while only the step area scrolls
- switching from `--check` to a confirmed update resets the fullscreen content and presents a clean update screen
- PTY regression coverage verifies the single-row setup status, green completion icon, four-region fullscreen rendering, and compact transaction phase output

## 3.0.8

Standalone setup and global setup-template release.

- `update-cli --setup` can execute `setup.yaml`/`setup.yml` directly when invoked inside a deployed `current/` directory without a project config
- `/usr/local/etc/update-cli/setup-template.sh` delegates to the native `--setup-manifest` fullscreen runner
- the global template supports `--details`, wait/fullscreen controls, and alternate manifest paths
- project setup and `just deploy` install the global setup TUI template

## 3.0.7

Setup compatibility and update-diagnostics release.

- accepts `project.type` as descriptive metadata in `schemaVersion: 1` setup manifests and displays it in the setup TUI
- setup-after-update confirmations now use German `[j/N]` notation and default to **No**
- default-Yes confirmations use `[J/n]`; `j`, `ja`, `y`, and `yes` remain accepted answers
- real updates now expose a numbered 13-step transaction covering source resolution, version policy, validation, staging, snapshot, backup, current sync, verification, setup, service restart, health check, release activation, and commit metadata
- optional phases are shown explicitly as `SKIP` with a reason instead of silently disappearing
- update failures now show the failing named phase, project, installed version, target version, source, concrete cause, and history-file location after transaction recovery completes
- fullscreen errors that were not already rendered by a subsystem are automatically surfaced inside the TUI before the final failure footer
- PTY regression tests verify `[j/N]` default-No setup behavior, `project.type`, numbered update phases, detailed setup-phase update failures, and restoration of the previous `current` tree

## 3.0.6

No-parameter workflow compatibility release.

- restored historical `"no parameter": ["check", "setup"]` semantics
- bare invocation performs the configured check and carries `setup` as an update modifier only after a confirmed update
- explicit modes such as `--upgrade`, `--version`, `--doctor`, and `--status` no longer collide with configured no-parameter actions
- `check + update` remains invalid because both are primary actions

## 3.0.5

macOS build-test and TUI diagnostics release.

- fixed the setup-bootstrap regression test on macOS where `/var/...` and `/private/var/...` refer to the same canonical temporary path
- bootstrap argument tests now compare the canonical manifest path after `filepath.EvalSymlinks`
- fullscreen setup commands now retain a bounded 64 KiB output tail even when `--details` is disabled
- when a setup command fails, the TUI automatically shows the failed command plus the last relevant stdout/stderr lines before the red failure footer
- command output remains hidden during successful fullscreen runs, preserving the clean 2.14-style TUI
- PTY CI smoke coverage now verifies both successful fullscreen rendering and visible diagnostics for a deliberately failing setup command

## 3.0.4

TUI and setup-manifest compatibility release.

- restored the fullscreen TUI contract from the 2.14 line for interactive `--check`, real `--update`, and setup runs
- fixed three-line header/footer, framed scrolling content area, blue confirmation footer, disabled line wrapping during input, and compact scrollback summary
- restored `UPDATE_CLI_TUI=auto|fullscreen|plain`, `--wait`, `--no-wait`, and `--no-ask` compatibility
- setup wrapper again supports `--details`, `--wait`, `--no-wait`, `--fullscreen`, `--no-fullscreen`, and `--config`
- restored `schemaVersion: 1` manifests with `project.description` and `id/name/when/run/cwd/allowFailure`
- restored all 2.14 `when` conditions (`file`, `not-file`, `dir`, `command`, `env`, `compose`, `os`)
- legacy manifest `project.name` is treated as display text instead of requiring the config slug
- retained the typed v3 setup schema in parallel; no project manifest migration is required
- Docker Compose child output is captured so it cannot corrupt the fullscreen update TUI
- added direct update integration coverage using a 2.14-style setup manifest plus PTY smoke validation for setup/check/update TUI flows

## 3.0.3

Self-bootstrap lookup release.

- `setup.sh` now prefers a compatible checkout-local `dist/update-cli` before the globally installed binary
- checkout-local `./update-cli` is used as the second candidate, followed by the global installation and finally `go run`
- an older `/usr/local/bin/update-cli` no longer produces a warning when a compatible local handler is already available
- compatible setup-handler failures remain fatal and are never hidden by trying another candidate
- added an executable regression test with a compatible local binary and an intentionally old global binary

## 3.0.2

No-parameter compatibility release.

- restored `"no parameter": ["check"]` as a valid schema v6 configuration
- `check` is valid as a standalone no-parameter action; `update` + `setup` remains the supported combined workflow
- invalid configured project files are no longer silently replaced by the generic help screen when the CLI is started without arguments
- added a regression test using the historical JSON key and `check` value exactly as used by existing projects

## 3.0.1

Build/bootstrap correctness release.

- fixed `just fmt-check`: shell variables now use normal `$` syntax instead of `$$`, which Bash interpreted as its PID (for example `65232files`)
- fixed the same shell-variable escaping defect in `just deploy`
- simplified `fmt-check` to use recursive `gofmt -l .` without unsafe filename word splitting
- `setup.sh` now probes `update-cli --help` for `--setup-manifest` before invoking the installed binary, so older installations no longer emit an unknown-flag usage dump
- failures from a compatible installed setup handler are now propagated instead of being silently retried through the source bootstrap
- template help now reports the actual embedded build version rather than a hard-coded `3.0.0`

## 3.0.0

Major safety and setup-architecture release.

### Transactional deployment

- real update, rollback, and restore operations now create exact temporary transaction snapshots
- failed activation, setup, service restart, or health checks restore the previous `current/`
- transaction snapshots are removed after commit
- immutable release content remains staged until the new `current/` has passed setup and health validation
- persistent `--backup` remains optional and separate from automatic transaction recovery
- failed operations are recorded in history with the failing phase

### Docker Compose lifecycle

- detects whether a Compose stack is actually running before update
- stops running stacks before mutable deployment work
- restores the previous running state after successful deployment
- restarts the previous stack after recovery
- recovery stops a newly started stack before restoring previous files

### Persistent data protection

- schema v6 adds `sync.preserve`
- default protection includes `.env`, `.env.*`, `data/`, `storage/`, `uploads/`, `media/`, `logs/`, and `var/`
- protected paths are not overwritten or deleted by release synchronization
- user backups no longer capture `.env` or `.env.*`

### Artifact and source security

- HTTPS required for URL sources unless explicitly overridden
- URL downloads have a configurable maximum size
- `--check`/status metadata uses HEAD with range fallback instead of full URL downloads
- optional SHA-256 verification for configured download/URL sources
- ZIP entry count, expanded bytes, per-file bytes, and compression ratio limits
- repository content now receives the same symlink/special-file policy as ZIP content
- repository sources support pinned `ref`, `commit`, and `version`
- configured project directories are checked against canonical/symlink escapes

### Offline recovery behavior

- rollback and cleanup use local inventory only
- remote source discovery is separated from local inventory
- status preserves source errors instead of converting all failures to “no release”

### Setup redesign

- introduced strict `setup.yaml` schema
- reusable `setup.sh` bootstrap template
- typed setup handlers for Go, Python, Node, Laravel, Docker Compose, copy, deploy, and shell commands
- direct `--setup-manifest` execution mode
- interactive post-update setup prompt
- legacy `setup.sh` and `config.setup.commands` remain supported as migration fallbacks

### Locking and cancellation

- lock metadata now contains PID, hostname, start time, and command
- stale local locks can be removed with `--unlock`
- UI progress no longer executes the mutable action in a detached goroutine, preventing lock/workspace cleanup races during cancellation

### Verification and diagnostics

- installed `current/` is checksum-compared against staged release content after synchronization
- configurable HTTP/command health checks participate in transaction commit
- `--verify` reports the actual archive path
- doctor reports stale locks, setup manifest validity, local inventory, source metadata, canonical paths, Docker availability, and health-check configuration

### Project maintenance

- Go module renamed to `github.com/r14r/update-cli`
- product branding standardized on **Update CLI**
- default deployment path `/usr/local/bin`
- default global config path `/usr/local/etc/update-cli`
- GitHub Actions CI added for format, vet, tests, race tests, and multi-platform builds
