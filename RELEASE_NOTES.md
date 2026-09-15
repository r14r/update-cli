# 2.16.4

## Fixed

- Set a bounded `os/exec.Cmd.WaitDelay` on Docker Compose probe, status, start, and stop commands so a timed-out command cannot keep update-cli blocked on inherited stdout/stderr pipes.
- Fix macOS timeout behavior where a child process could keep `Cmd.Run()` blocked until its natural exit even after the command context expired.
- Make Docker Compose timeout regression tests resilient to scheduler pressure during parallel `go test ./...` runs while still proving that five-second hangs are terminated early.

## Tests

- `lib/projectdocker` timeout tests now complete in a fraction of a second instead of waiting for the simulated five-second child process.
- Full Go test suite passes with the timeout hardening applied.

# 2.16.3

## Fixed

- Accept release archives named both `<project>-v<MAJOR>.<MINOR>.<PATCH>.zip` and `<project>-<MAJOR>.<MINOR>.<PATCH>.zip`.
- Keep the historical `v`-prefixed archive convention fully backward compatible while allowing the shorter form in download-folder discovery, explicit archive validation, and URL filename version parsing.
- Bound Docker Compose availability/status calls so a stalled Docker Desktop or Docker Engine cannot block an update indefinitely.
- Bound Docker Compose `up`/`down` lifecycle actions and report timeout/cancellation context explicitly.

## Tests

- Add regression coverage for archive discovery and version parsing with and without the `v` prefix.
- Add regression coverage proving stalled `docker compose version` and `docker compose ... ps -q` calls are terminated by their configured timeout.

# 2.16.2

## Added

- Add `update-cli doctor --fix` as an interactive repair mode.
- Print every deterministic config/manifest change before repair and ask `Geplante Reparaturen durchführen? [y/N]`; only explicit yes applies changes.
- Re-run Doctor after a successful fix so remaining errors/warnings and the `Migration required` state are immediately visible.

## Changed

- Treat root `update-cli.yaml` and `current/update-cli.yaml` as alternative valid manifest locations in Doctor. A missing root manifest is no longer a warning when the installed current manifest exists.
- Make the `Projektkonfiguration` warning conditional and concrete: it now lists only project-dependent local `config.json` values that do not already have an active YAML override.
- Clarify that the project-configuration placement warning is an architecture/reproducibility recommendation, not a schema-migration error.

## Safety

- `doctor --fix` uses the existing timestamped-backup repair paths and never mutates files before confirmation.
- `doctor --fix` and `--migrate` are mutually exclusive; interactive `--fix` cannot be combined with `--json`.

## Tests

- Add regression coverage that a managed project with only `current/update-cli.yaml` does not emit `WARN Manifest root`.
- Add end-to-end coverage for `doctor --fix` confirming and repairing a structurally invalid current manifest.

# 2.16.1

## Added

- Extend `update-cli doctor` with the exact migration state used by the fullscreen `Migration required` header badge.
- Print `Migration required: JA` plus one or more concrete migration reasons and affected file paths when a migration/repair is pending.
- Print `Migration required: nein` when the shared UI/Doctor migration inspection finds no pending migration.
- Expose `migrationRequired` and structured `migrationReasons` in Doctor JSON output.

## Fixed

- Use a single shared migration-requirement inspection for both the fullscreen UI header and Doctor diagnostics, preventing the two surfaces from making different migration decisions.

## Tests

- Add coverage for a current `current/update-cli.yaml` with a repairable unknown field and verify Doctor reports that exact file/field as the badge reason.
- Add coverage proving a schema-9 runtime config plus a valid schema-2 manifest reports no migration requirement.

# 2.16.0

- Standardize the entire public CLI on `update-cli <command> --<parameter> --<parameter>`.
- Remove public command aliases with a `--` prefix such as `--help`, `--version`, `--setup`, and `--update`.
- Reject bare secondary subcommands such as `schema version`, `config migrate`, and `setup run`; use `schema --version`, `config --migrate`, and `setup --run`.
- Replace positional command values with named parameters (`update --archive`, `rollback --version`, `init --project`, `verify --archive`, `restore --snapshot`).
- Add `help --command COMMAND --details` for detailed command help.
- Update README, help/discovery text, setup bootstrap scripts, and regression coverage to the canonical grammar.

# 2.15.0

## Added

- Add command-specific detailed help with `--details`.
- Use `update-cli help --command COMMAND --details` for command-specific detailed help.
- Add `update-cli setup --list` as a first-class standalone setup inspection command.
- Extend setup listing with every setup step including its manifest `id`, task, name, operation, and index; JSON output exposes the same `steps` collection.
- Add `update-cli setup --run STEP_ID` and the canonical form `update-cli setup --run STEP_ID` to execute exactly one setup step.
- Advertise setup-step discovery and execution through the JSON CLI discovery contract.

## Fixed

- Remove the contradictory parsing of `setup --list` as both setup mode and global `--list` mode.
- Return an explicit standalone-command error when `setup --list`, setup task/workflow selection, or setup step execution is combined with update/rollback/restore.
- Reject duplicate matching step IDs as ambiguous instead of executing an arbitrary step.

## Tests

- Add parser regression coverage for `setup --list`, `setup --run STEP_ID`, and both detailed-help syntaxes.
- Add an integration test proving `setup --run STEP_ID` executes only the selected step and leaves neighboring setup steps untouched.
- Add help-output coverage for command-specific `--details` output.

# 2.14.6

## Fixed

- Fix the fullscreen `Migration required` badge remaining visible after `config migrate` reports the runtime config is already at the current schema.
- Do not classify a valid current-schema `update-cli.yaml` as repairable merely because the canonical YAML renderer would change whitespace, quoting, key order, or other formatting.
- Keep the badge active for genuine schema changes, strict-parser failures, legacy manifest canonicalization, unknown fields, and deterministic value/structure repairs.

## Tests

- Add manifest-inspection regression coverage proving formatting-only differences do not require migration.
- Add updater coverage proving a current schema-9 config plus a valid schema-2 manifest does not set project migration state.

# 2.14.5

## Fixed

- Use `.update-cli/` as the only project runtime-state directory throughout production code, tests, packaging, help/discovery text, and documentation.
- Fix `config migrate` / `config --migrate` backup placement so the timestamped backup is created beside `.update-cli/config.json` instead of any obsolete runtime directory.
- Remove obsolete runtime-directory fallback/discovery logic so project root resolution and config repair no longer diverge between two directory names.

## UI

- Add a right-aligned `Migration required` badge to the fullscreen header.
- Render the badge with a red background and white bold text.
- Detect required runtime-config and root/current manifest migrations before normal setup/update/check work and keep the warning visible across phase changes.

## Tests

- Add regression coverage that config migration backups remain inside `.update-cli/`.
- Add UI coverage for the red right-aligned migration badge.
- Add updater coverage for detecting outdated runtime configs and legacy project manifests.

# 2.14.4

## Fixed

- Fix macOS-only false failures caused by `/var/folders/...` and `/private/var/folders/...` referring to the same filesystem location.
- Canonicalize the expected/actual paths in `TestResolveRootFromCurrentPrefersParentOverNestedRuntimeState`, `TestRunJustInstallPrefersCurrent`, and `TestInstallCommandRunsWithoutRuntimeConfig`.
- Keep the production root/install behavior unchanged; this is a test portability correction, not a path rewrite in normal project configuration.

## Tests

- Document why the integration suite intentionally executes multiple setup/migration/update scenarios.
- Clarify that standard `go test` only dumps those scenario logs when a package fails; after the false path failures are fixed, normal `just test` output remains compact.

# 2.14.3

## Fixed

- Treat `no-param`, `no-parameter`, and `noParameter` in `config.json` as compatibility aliases for the canonical `no parameter` setting.
- Preserve a configured parameterless `update` action instead of silently falling back to `check`, which could display `Update jetzt installieren?`.
- Accept both scalar (`"update"`) and list forms for the aliases.
- Make config repair/migration normalize alias spellings back to the canonical `no parameter` key.

## Tests

- Add strict-loader coverage for `"no-param": "update"`.
- Add an integration test proving that parameterless execution installs an available ZIP directly when the alias selects `update`.

# 2.14.2

## Fixed

- Make command-first syntax (`update-cli version`, `update`, `setup`, `doctor`, `schema version`, `config check`, `releases list`, etc.) a tested first-class interface while retaining historical `--...` action aliases.
- Add a resilient runtime-config bootstrap loader: strict parse first, then backup + safe repair + strict retry when an existing local/global config uses legacy/transition fields or schema metadata.
- Prevent parameterless `update-cli` from being permanently blocked by historical top-level `defaultUser`, `keepRsyncOnError` aliases, obsolete fields, or safely normalizable runtime-schema mismatches.
- Preserve strict validation after automatic repair; malformed JSON or structures that cannot be safely repaired still fail with a clear error.
- Keep `VERSION` as the only release-version source.

## Tests

- Add regression coverage for automatic repair of a future/legacy runtime config and for parameterless execution after repair.
- Retain full command-vs-legacy flag equivalence coverage, including `version`, nested schema/config/doctor/setup/release commands, and update plan.

# 2.14.1

## Changed

- Make `VERSION` the single and only release-version source for update-cli.
- Remove the parallel `RELEASE_VERSION` file and the old `scripts/sync-release-version.sh` synchronization mechanism.
- Build and `go run` no longer inject a version through `-ldflags`; the binary reports the `VERSION` file embedded at compile time.
- Make source packaging derive the directory name and ZIP filename exclusively from `VERSION`.
- Make installed `current/VERSION` authoritative for project version detection and release validation.
- Stop writing new `.release-version` markers. Existing `.release-version` is read only as a compatibility fallback when `current/VERSION` is absent, and is then migrated into `VERSION`.
- Add regression coverage that rejects parallel version-source files and verifies that build/package metadata uses only `VERSION`.

# 2.14.0

## Added

- All public CLI actions can be used as command-first syntax without a `--` prefix. This includes `version`, `update`, `upgrade`, `setup`, `doctor`, `fix`, `install`, `run`, `schema`, `config`, `templates`, `releases`, and their command-like sub-actions such as `doctor migrate`, `schema version`, `schema view`, `schema save`, `config check`, `config migrate`, `config set`, `releases list`, and `update plan`.

## Fixed

- Self-versioning no longer trusts a potentially corrupted `VERSION`, `.release-version`, or linker `-X main.version` value. `RELEASE_VERSION` is packaged as immutable source-release metadata and is preferred by the binary.
- `scripts/sync-release-version.sh` repairs `VERSION` before build/install and repairs managed update-cli `.release-version` markers when applicable.
- Source packaging now fails if `VERSION` and `RELEASE_VERSION` differ.
- `u version` is covered as a first-class command alias and no longer falls through to positional-argument handling in the current release.

# 2.13.3

- Normalize known historical `keepRsyncOnError` / `keepOnSetupError` runtime-config placements before strict `config.json` decoding, so normal commands no longer fail before `fix` can be invoked.
- Keep canonical runtime JSON at `setup.keepRsyncOnError`; accept top-level, `sync.keepRsyncOnError`, `sync.keepOnSetupError`, and `setup.keepOnSetupError` compatibility aliases in memory.
- Extend `update-cli fix` so those compatibility aliases are persisted back to canonical `setup.keepRsyncOnError` with the normal backup behavior.
- Make the embedded source-release `VERSION` authoritative for `update-cli version`; stale or corrupted build-time `-ldflags` values can no longer override the embedded release version.
- Remove the Update CLI project's setup-time mutation that rewrote Source `VERSION` from `.release-version` before building.
- Add regression coverage for runtime-config aliases, persisted repair, canonical-value precedence, and embedded-version precedence over a deliberately wrong `12.2.0` linker value.

# 2.13.2

- Make the Update CLI project's own `setup` workflow use the same canonical `just install` pipeline as manual installation instead of maintaining a second duplicated Go vet/test/race/build/deploy path.
- Remove the internal `UPDATE_CLI_SETUP_RUNNING` marker before the nested `just install` process so setup-driven builds/tests run with the same relevant environment as direct `just build` / `just install`.
- Document that updater recovery warnings printed during tests are expected simulated failure-path output and are not themselves test failures.
- Add `docs/tapes/install.tape` and `docs/tapes/quickstart.tape` plus `scripts/render-tapes.sh` and deterministic demo-release fixture generation.
- Add `just tape-install`, `just tape-quickstart`, and `just tapes` recipes.
- Run the Justfile test and race-test recipes with `-count=1` so manual validation cannot report a stale cached pass after setup-driven tests have failed.
- Allow `just install` to use `UPDATE_CLI_INSTALL_BIN_DIR` for safe project-local demo/CI installations while retaining `/usr/local/bin` as the default.
- Rewrite and expand the README INSTALL and QUICKSTART sections around the canonical source install, download bootstrap, repository bootstrap, normal post-init commands, and VHS demo generation.
- Add regression checks for the canonical setup workflow, README tape files and release metadata.

# 2.13.1

- Add `no-setup` as a supported modifier in the runtime `"no parameter"` action list.
- Support the canonical parameterless configuration `"no parameter": ["update", "no-setup"]`.
- Ensure parameterless update with `no-setup` never asks whether project setup should run and never executes project setup.
- Reject invalid no-parameter combinations such as standalone `no-setup` or `setup` together with `no-setup`; `fix` normalizes safely repairable legacy/invalid combinations.
- Change newly initialized project defaults and the shipped default config to parameterless `update + no-setup`.
- Add regression coverage for parsing, persisted defaults, repair normalization, and parameterless update execution without setup.

# 2.13.0

- Replace the normal full `current/` transaction snapshot with release-based rollback preparation when the installed version has a matching valid `release/<VERSION>/`. This avoids copying large generated trees before every update.
- Keep the historical exact `current/` snapshot as an automatic fallback when the previous versioned release is unavailable or invalid.
- Restore failed transactions from the immutable previous release using the configured `sync.preserve` policy, so persistent files/directories remain protected.
- Rename the visible update step from `Transaktions-Snapshot von current erstellen` to `Transaktions-Rollback vorbereiten`.
- Add non-mutating `update-cli.yaml` inspection that reports multiple unknown/removable fields and safely normalizable values instead of exposing only the first parser error.
- Enrich setup/run/effective-config manifest errors with direct `update-cli fix` / `doctor --migrate` guidance.
- Extend `doctor` so tolerated legacy/transition fields are reported even when the strict executable subset can still be parsed; `doctor --migrate` now falls back to deterministic structural repair when schema migration alone is insufficient.
- Extend `fix` to repair both `./update-cli.yaml` and `./current/update-cli.yaml` when both exist.
- Add regression coverage for fast release-based rollback, fallback exact snapshots, preserve behavior, multi-error manifest inspection, doctor structural migration, and dual-manifest repair.

# 2.12.3

- Fix project-root detection when commands are invoked from `current/`: a parent `.update-cli/config.json` now takes precedence over an accidentally copied `current/.update-cli/config.json`, preventing `current/current/update-cli.yaml`.
- Treat `.update-cli/` as reserved project-root runtime state. They are excluded from release creation and current synchronization.
- Remove stale nested runtime-state directories from `current/` during synchronization so projects affected by 2.12.2 self-heal on the next update.
- Restore the accidentally omitted `lib/backup` package in the source release.
- Add reproducible source packaging/verification scripts that require core source files, reject runtime/build state, and verify the embedded VERSION after ZIP creation.
- Keep installed `VERSION` aligned with the authoritative `.release-version` marker once setup runs in the corrected current directory.
- Re-run gofmt, vet, all tests, required race tests, direct `current/` root-resolution regression tests, rsync runtime-state tests, and final ZIP validation.

# 2.12.2

- Replace the bootstrap-breaking project YAML layout `update.setup.keepRsyncOnError` with canonical `update.sync.keepOnSetupError`.
- Keep read compatibility for the 2.11.x/2.12.0-2.12.1 transitional layout and migrate it automatically via `fix` and `doctor --migrate`.
- Preserve `.update-cli/config.json` compatibility with `setup.keepRsyncOnError`; the canonical project YAML value still has higher effective-config priority.
- Make `.release-version` authoritative for managed installations and repair mismatching `current/VERSION` plus project-root `VERSION` before/after standalone setup.
- Add an early Update CLI `version-sync` setup step so older installed binaries can bootstrap a corrected release even when `current/VERSION` was stale or malformed by a previous workflow.
- Add regression coverage for canonical and transitional YAML parsing, structural migration, effective-config priority, and the reported `12.2.0` versus release-marker mismatch.
- Re-run gofmt, vet, full tests, required race tests, old-parser bootstrap compatibility and final ZIP consistency checks.

# 2.12.1

- Historical: added compatibility around the transitional `update.setup.keepRsyncOnError` layout; superseded by `update.sync.keepOnSetupError` in 2.12.2.
- Synchronize `<project-root>/VERSION` with the active `current/VERSION` after successful update, rollback and restore operations.
- Synchronize the root `VERSION` after a successful standalone managed setup and when an update discovers that the target version is already installed.
- Keep root `VERSION` aligned with a deliberately retained new `current/` when `setup.keepRsyncOnError=true` suppresses rollback after setup failure.
- Add root-version consistency tests and update README/release metadata.
- Re-run formatting, vet, full tests, required race tests, functional setup/update checks and final ZIP consistency validation.

# 2.12.0

- Add an automatic pre-setup migration workflow based on a project-provided `migrate.sh` in the active release/source directory.
- Execute migration after the new release has been synchronized and verified, but before any project setup task/script/Justfile fallback is started.
- Track successful migrations persistently per semantic release version with `<project-root>/.update-cli/.migration.done.<VERSION>`, keeping migration state outside replaceable `current/`.
- Run each release migration at most once during normal Update CLI operation; a different installed version gets a separate marker and therefore its own migration run.
- Make standalone `update-cli setup` check the migration marker first and execute a pending `migrate.sh` before setup, including calls started from a managed `current/` directory.
- Invoke `migrate.sh` through `bash` and provide `UPDATE_CLI_MIGRATION`, `UPDATE_CLI_PROJECT_ROOT`, `UPDATE_CLI_CURRENT_DIR`, and `UPDATE_CLI_VERSION` environment variables.
- Create the done marker only after successful migration; migration errors prevent setup and leave the migration pending for a later retry.
- Keep `--no-setup` and an interactively declined setup migration-free until setup is explicitly run later.
- Add migration once-per-version, failure/no-marker, version-validation, update-before-setup, and standalone-setup regression coverage.
- Update README/release metadata and rerun formatting, vet, full tests, required race tests, integration tests, and final ZIP consistency checks.

# 2.11.0

- Add project-versioned runtime update settings to `update-cli.yaml` with priority `global config.json < local .update-cli/config.json < update-cli.yaml < explicit CLI source options`.
- Support `project.slug`, `update.mode/source`, release/current paths, backup/retention, `sync.preserve`, `setup.keepRsyncOnError`, Docker lifecycle and healthcheck as YAML overrides.
- Keep host/user controls such as `security.*`, `source.defaultUser`, no-parameter behavior and templates outside the project-controlled YAML; reject `update.security` in the manifest.
- Make the root `update-cli.yaml` the preferred runtime manifest and fall back to `current/update-cli.yaml` only when the root manifest is absent.
- Expand the canonical schemaVersion-2 JSON Schema with the project `update:` settings while preserving backward compatibility with existing schema-2 manifests.
- Extend `update-cli doctor` to inspect `.update-cli/config.json`, root `update-cli.yaml`, and `current/update-cli.yaml`, including execution from inside `current/` using the parent runtime config.
- Report root/current manifest differences, effective override provenance, current schema versions and concrete migrate/fix suggestions; extend `doctor --migrate` to cover runtime config and both manifest locations.
- Add regression tests for YAML-over-JSON priority, root-vs-current manifest priority, current-directory doctor resolution and dual-manifest diagnostics.
- Update README, sample project manifest and release metadata, then rerun formatting, vet, full tests, required race tests and final ZIP consistency checks.

# 2.10.0

- Add canonical `update-cli install` command to execute exactly `just install` for the project.
- Prefer the active `current/` directory when it contains `justfile`/`Justfile`; otherwise execute from the project root.
- Return explicit errors when `just` or a project justfile is unavailable and propagate the recipe exit code on failure.
- Keep `--install` as a compatibility alias while advertising command-first `install` in help and machine-readable CLI discovery.
- Add parser, project-directory selection, command-execution, and discovery regression coverage.
- Update README and release metadata, then re-run formatting, vet, full tests, required race tests, and ZIP consistency checks.

# 2.9.1

- Fix interactive update behavior when the user answers `n` to the project-setup confirmation.
- Keep `Projekt-Setup ausführen` skipped and also defer the subsequent Docker restart and healthcheck activation steps for that interactive decision.
- Prevent a Docker restart failure from rolling the verified rsync update back after the user explicitly declined setup.
- Leave a previously running Compose stack stopped when setup is declined so manual setup/start can safely complete the deployment.
- Continue activating the versioned release and committing update metadata after the intentionally deferred activation steps.
- Keep explicit `--no-setup` automation semantics unchanged; the fix is scoped to the interactive decline path.
- Add regression coverage proving that Docker restart, healthcheck, and recovery are not entered after interactive setup rejection.
- Re-run formatting, vet, full tests, required race tests and final ZIP/version consistency checks before packaging.

# 2.9.0

- Add `update-cli fix` as a project-wide repair command for runtime configuration and project manifests.
- Run the repair path before strict runtime-config loading so broken/legacy `config.json` files can be repaired even when normal commands fail to start.
- Repair both project-local `.update-cli/config.json` and the installation-wide `config.json` when present.
- Migrate legacy top-level `defaultUser` to canonical `source.defaultUser`, including the configuration shape that previously caused `json: unknown field "defaultUser"`.
- Remove unknown runtime-config fields at supported top-level and nested sections and normalize safe legacy/wrong values to current schema 9 structure.
- Normalize source/mode combinations, booleans, numeric security limits, Docker lifecycle, healthcheck values, no-parameter actions, rsync preserve entries and legacy runtime paths without enabling unsafe HTTP sources implicitly.
- Create a current local runtime config when it is missing and preserve every modified config file with a timestamped backup.
- Repair `update-cli.yaml` to manifest schema 2, remove unknown fields from supported project/default/requirements/workflow/task/run/step sections, normalize safe values, prune invalid task references and validate the generated manifest before replacement.
- Keep malformed JSON/YAML syntax as an explicit error when automatic repair would require guessing user data.
- Add JSON output and CLI discovery/help coverage for `fix`, plus regression tests for strict post-repair loading and the historical `defaultUser` failure.
- Make command-first syntax the canonical CLI surface: `update-cli upgrade`, `unlock`, `howto`, `version`, `check`, `update`, `backup`, `rollback`, `restore`, `status`, `verify`, `setup`, `run`, `init`, `clean`, `cleanup`, and `history` work without a command-level `--` prefix.
- Rename the public release-list command from `update-cli list` to `update-cli releases --list`; retain `list`/`--list` only as compatibility aliases.
- Update help output and machine-readable CLI discovery so completion/tooling advertises `releases`, `howto`, and `version` as commands and uses `releases --list --json` as the release/backups value source.
- Keep regular options such as `--json`, `--debug`, `--setup`, `--root`, and `--no-ui` unchanged.
- Update the project `justfile` to exercise the canonical command-first forms and rename its release inventory recipe to `releases`.
- Re-run formatting, vet, full tests, required race tests and final ZIP/version consistency checks before packaging.

# 2.8.0

- Add runtime configuration `setup.keepRsyncOnError` with a default of `false`.
- Keep the verified rsync deployment in `current/` when project setup fails and `setup.keepRsyncOnError=true`, while still returning an error and recording a failed setup-phase history entry.
- Promote the corresponding staged release into `release/<version>/` when the failed setup deployment is intentionally retained, keeping release/current file state consistent.
- Continue normal transactional recovery for every non-setup failure phase.
- Keep `sync.preserve` semantics unchanged so persistent local files remain protected in the retained deployment.
- Support global inheritance and explicit local true/false override for the new setup recovery policy.
- Bump `.update-cli/config.json` runtime schemaVersion to 9 and update shipped defaults/documentation.
- Add config merge/set regression tests and an end-to-end failed-setup retention test.
- Re-run formatting, vet, full tests, required race tests, and release ZIP consistency checks before packaging.
- Synchronize the ZIP filename, `VERSION`, README current release, and release-notes heading to 2.8.0.

# 2.7.0

- Make `update-cli doctor` a project-folder manifest validator that works without `.update-cli/config.json`.
- Validate canonical `update-cli.yaml` directly from the working project folder and recognize legacy `setup.yaml` as a migration source.
- Add `update-cli doctor --migrate` to upgrade older project manifests to the latest supported schema.
- Preserve an existing `update-cli.yaml` with a timestamped backup before schema migration.
- Canonicalize legacy `setup.yaml` by leaving the legacy file untouched and creating a new latest-schema `update-cli.yaml`.
- Keep project-manifest migration separate from `config --migrate`, including compatibility for the flag-style `--config --migrate` form.
- Extend doctor output, JSON data, CLI discovery, help text, and regression tests for the new behavior.
- Re-run formatting, vet, full tests, required race tests, functional doctor/migration checks, and release ZIP consistency checks before packaging.
- Synchronize the ZIP filename, `VERSION`, README current release, and release-notes heading to 2.7.0.

# 2.6.2

- Add `update-cli schema --version` to print the canonical `update-cli.yaml` manifest schema version.
- Support the equivalent compatibility form `update-cli schema --version`.
- Preserve standalone `update-cli version` as the application-version command.
- Add schema-version CLI discovery metadata and regression coverage for alias and mutual-exclusion behavior.
- Re-run formatting, vet, full tests, and required race tests before packaging.
- Synchronize the ZIP filename, `VERSION`, README current release, and release-notes heading to 2.6.2.

# 2.6.1

- Publish the verified 2.6.x implementation as patch release 2.6.1.
- Keep runtime behavior unchanged from 2.6.0.
- Re-run the complete formatting, vet, test, race-test, init, debug, schema, and setup-fallback validation suite.
- Synchronize the ZIP filename, `VERSION`, README current release, and release-notes heading to 2.6.1.

# 2.6.0

- Add global `--debug` support to every command and to the configured no-parameter invocation.
- In debug mode, use direct detailed output instead of the fullscreen TUI path and show resolved config/template paths, merge semantics, source, release/current directories, preserve rules, and rsync flow.
- Accept historical top-level `defaultUser` in `config.json` and normalize it to canonical `source.defaultUser`.
- Keep `update-cli --init PROJECT --from-repository --repository URL` fully supported and cover it end-to-end.
- Keep compact `--from-repository REPOSITORY` syntax and the download bootstrap `update-cli --init PROJECT`.
- Verify download bootstrap selects only the newest exact `PROJECT-v<MAJOR>.<MINOR>.<PATCH>.zip` from the configured/default download folder.
- Make setup-manifest parsing tolerant of unknown top-level transition/extension fields while preserving strict validation for supported sections and executable steps.
- Synchronize release metadata to 2.6.0.

# 2.5.0

- Add installation-wide configuration resolved as `INSTALLFOLDER/../etc/update-cli/config.json`; `/usr/local/bin/update-cli` maps to `/usr/local/etc/update-cli/config.json`.
- Load global `config.json` first and recursively merge project-local `.update-cli/config.json` over it.
- Make `sync.preserve` cumulative across global and local config so central rsync preserve/exclude defaults remain active while projects add their own paths.
- Load global `templates.json` first and merge project-local templates by name; local definitions replace same-name global templates.
- Extend `config --list` to display both global and local config/template paths and the project-local history file.
- Install default global `config.json` and `templates.json` with `just install` only when they do not already exist.
- Add tests for path resolution, config merge semantics, preserve union, templates merge, and config listing.

# 2.4.3

- Fix release synchronization so source `.env`, `.env.example`, and other `.env.*` files are retained in `release/`.
- Change `sync.preserve` from "always exclude" to "preserve if present, otherwise seed once from release".
- Keep existing local protected files/directories unchanged on subsequent updates.
- Report seeded protected paths in dry-run plans without creating them.
- Keep ordinary backup snapshot secret exclusions unchanged.
- Add regression tests for release dotfiles and first-install preserve behavior, plus verification against the supplied GrapesJS release artifact.

# 2.4.2

- Fix release metadata consistency.
- Add a regression test that requires `VERSION`, README current release, and the top release-notes version to match.
- Verify ZIP filename/version against the packaged `VERSION` before delivery.

# 2.4.1

- Hardened local bootstrap regression coverage.
- Verified `update-cli --init <project>` installs the newest matching `<project>-v<semver>.zip` from the configured/default download folder.
- Verified `update-cli --init <project> --from-repository <repository>` works as the canonical repository bootstrap syntax without a separate `--repository` flag.
- Both init paths continue to use the current working directory as the project root by default.

# 2.4.0

- Change repository bootstrap syntax to `--init PROJECT --from-repository REPOSITORY`.
- Accept `https://github.com/USER/REPO`, `USER/REPO`, and `REPO` repository specifications.
- Expand `REPO` using `source.defaultUser` from `.update-cli/config.json`; default value is `r1r`.
- Bump updater config schema to 8 and persist `source.defaultUser` in the source section.
- Normalize shorthand repository specifications to canonical HTTPS GitHub clone URLs.
- Keep the previous `--from-repository --repository URL` form as a compatibility alias.
- Update README/help and add parser/config normalization tests.

# 2.3.1

- Fix `--init <project>` so the current working directory is always the project root unless `--root` is explicitly provided.
- Stop creating an implicit nested `<cwd>/<project>` directory during init.
- Keep the project argument as the logical project name used by updater configuration and release matching.
- Update download and repository bootstrap integration tests for current-directory initialization.
- Update README/help text to document the corrected init semantics.

# 2.3.0

- Add `update-cli schema --view` to print the canonical `update-cli.yaml` schemaVersion-2 JSON Schema.
- Add `update-cli schema --save <file.json>` to save the same schema and create missing parent directories.
- Keep updater source/update policy outside the manifest schema in `.update-cli/config.json`.
- Expose the schema command in help and CLI discovery.

# 2.2.0

- `--init <project>` now performs a full local bootstrap instead of only creating updater config.
- default init source is the configured download folder and automatically selects the newest exact `<project>-v<MAJOR>.<MINOR>.<PATCH>.zip`.
- added `--from-repository --repository <url>` shortcut for repository bootstrap.
- init creates/uses `<cwd>/<project>` when no explicit `--root` is provided, unless the current directory already has the project name.
- initial bootstrap deploys transactionally to `current/` and automatically runs available setup automation.
- existing matching init configuration can be reused; source changes require `--force`.
- fixed Update CLI's project-specific release ordering so current 2.x releases are newer than 1.x releases.
- README updated for both bootstrap workflows.

# Update CLI 2.1.0

- Restore `.update-cli/config.json` as the canonical persistent updater configuration.
- Move `mode` and `source` back out of `update-cli.yaml`; setup YAML is again dedicated to setup/run/tasks/workflows.
- Migrate older `.update-cli/config.json` schemas in place with `config --migrate`.
- Discover legacy `setup.yaml` after `update-cli.yaml`; prefer `update-cli.yaml` when both exist.
- Infer a missing legacy `setup.yaml` schema from its structure.
- Add explicit setup fallback `just build` followed by `just install` when no manifest or `setup.sh` is available but a Justfile exists.
- Keep schemaVersion-2 parsing tolerant of transitional `update:` and `cli:` blocks from the short-lived YAML-only model.
- Preserve the macOS `/var` vs `/private/var` root canonicalization fix from 2.0.1.
- Update README and setup documentation to the hybrid JSON + YAML compatibility model.

# Update CLI 2.0.1

- Fix macOS root-resolution test failures caused by `/var/...` and `/private/var/...` referring to the same temporary directory.
- Canonicalize the expected temporary root with `filepath.EvalSymlinks` before comparison.
- Update the root-discovery comment for the YAML-only 2.x configuration model.

# Update CLI 2.0.0

- Redesign project configuration so `update-cli.yaml` is the single source of project settings.
- Keep project runtime state and cache exclusively below `.update-cli/`.
- Move project identity to `project.slug`, source/mode/directories/backup/retention/sync/security/Docker/healthcheck to `update`, and no-parameter behavior to `cli.noParameter`.
- `config`, `config edit`, `config --check` and `config --set` now operate on `update-cli.yaml`.
- `config --migrate` imports legacy schema-1..7 `config.json`, creates a timestamped backup, merges settings into YAML, validates the result, and removes the active JSON file.
- Preserve existing setup/run/tasks/workflows when writing or migrating project configuration.
- Keep historical config key aliases for CLI compatibility while documenting YAML-native dotted paths.
- Pin GitHub Actions to Go 1.26.5 to avoid macOS `dyld: missing LC_UUID load command` failures from older Go toolchains.

---

# 1.5.0

- allow schemaVersion-2 `update-cli.yaml` to declare a top-level `update` source block
- support `update.mode` plus `update.source.type`, `folder`, `url`, `repository`, `ref`, `commit`, `version`, and `sha256`
- use source precedence `CLI override > update-cli.yaml > .update-cli/config.json`
- configure the update-cli project itself to pull from `https://github.com/r14r/update-cli.git` on `main`
- keep `.update-cli/config.json` for local updater state and machine-specific policy
- stop producing ZIP artifacts for normal project changes; GitHub commits are the release source

# 1.4.0

- Formalizes `check`, `update`, and `run` as first-class command aliases for `--check`, `--update`, and `--run`.
- Adds explicit alias-equivalence test coverage for all three primary commands.
- Documents that options and positional arguments behave identically in command and flag form.

# 1.3.0

- extend `update-cli run` / `update-cli run` to accept structured `run.steps` in schemaVersion-2 `update-cli.yaml`
- support `run.description` and reuse typed setup-step syntax such as `command.exec`, `command.args`, `cwd`, `env`, `timeout`, `retries`, `when` and `allowFailure`
- keep the compact `run.command` form fully compatible; reject ambiguous manifests that define both `command` and `steps`
- add `update-cli config --check` / `update-cli config --check` for read-only config validation and migration-needed reporting
- add `update-cli config --migrate` / `update-cli config --migrate` as the config-scoped migration command with backup semantics
- expose the new config commands through `--help --json` and update README documentation

# 1.2.1

- rename the Justfile installation recipe from `just deploy` to `just install`
- keep the underlying `deploy` operation in `update-cli.yaml` unchanged
- update README and project-file regression tests for the new command name

# 1.2.0

- add `update-cli run` and `update-cli run` to launch the active application
- read the launch command from top-level `run.command` in `update-cli.yaml`
- support optional `run.cwd` and `run.env`
- execute run commands from the active `current/` release and preserve interactive stdin/stdout/stderr
- propagate child-process exit codes through Update CLI
- rename the canonical project automation file from `setup.yaml` to `update-cli.yaml` across runtime discovery, generators, conversion, templates, examples, tests and documentation
- allow schemaVersion-2 run-only manifests without setup tasks

# Update CLI 1.1.2

- Added a new README Introduction explaining why Update CLI is used and how the transactional deployment model separates acquisition from deployment.
- Documented the two primary sources: Download Folder (`mode: update`) and GitHub/Git repository (`mode: pull`).
- Expanded Quickstart with complete setup and recurring-use workflows for both source modes.
- Added detailed GitHub pull documentation covering config.json, repository URL, `source.ref`, source switching, `check`, `update --plan`, `git pull --ff-only`, private repository authentication, commit detection, setup integration and troubleshooting.
- Runtime behavior is unchanged from 1.1.1.

# Update CLI 1.1.1

## 1.1.1 README update

- Update the project `README.md` for the `mode: update` / `mode: pull` feature introduced in 1.1.0.
- Split Quickstart into complete ZIP and Git workflows.
- Add an explicit update-mode/source matrix and one-time CLI override examples.
- Document persistent Git checkout and `.release-commit` behavior in the primary onboarding path.
- Runtime behavior is unchanged from 1.1.0.

# Update CLI 1.1.0

Minor release adding explicit ZIP update and Git pull acquisition modes.

## 1.1.0 update/pull modes

- Add project configuration `mode` with values `update` and `pull`; config schema is now version 7.
- `mode=update` uses the established transactional ZIP workflow with `download` or `url` sources.
- `mode=pull` requires a `repository` source, keeps a persistent checkout in `.update-cli/repository`, and updates it with `git pull --ff-only`.
- Git content is snapshotted without `.git`, validated, versioned under `release/`, and synchronized to `current/` through the existing transaction/recovery pipeline.
- Persist the deployed commit as `.release-commit` and expose installed/available commit changes during `check`.
- Treat a changed repository commit as an available pull update even when `VERSION` is unchanged.
- Add `--mode update|pull` to initialization and source overrides; positional ZIP archives are rejected in pull mode.
- Migrate schemaVersion-6 repository sources to `mode=pull`; download/URL sources migrate to `mode=update`.
- Add pull-mode configuration, version-policy, persistent-checkout and real Git pull regression tests.

# Update CLI 1.0.3

Patch release that normalizes user-home paths in terminal presentation.

## 1.0.3 UI path presentation

- Replace the absolute current-user home prefix with `$HOME` in fullscreen TUI, plain/no-UI console rows, diagnostics, errors, confirmation prompts, setup/process output, history, inventory, cleanup, and update-plan path presentation.
- Example: `/Users/Ralph.Goestenmeier/Downloads/DigitalProductsPlatform-v4.5.0.zip` is rendered as `$HOME/Downloads/DigitalProductsPlatform-v4.5.0.zip`.
- Keep filesystem operations and machine-readable JSON output on canonical absolute paths; the change is presentation-only.
- Avoid replacing lexical lookalikes such as `/Users/Ralph.Goestenmeier-old/...` or embedded non-home path fragments.
- Add UI regression coverage for exact home paths, descendants, multiple paths, unrelated paths, and plain console rendering.
- Add stable-line version-policy coverage for `1.0.2 -> 1.0.3`.

## 1.0.2
Documentation-focused patch release for the stable 1.x CLI contract. Runtime behavior and schemas are unchanged.

## 1.0.2 README and command documentation

- Reworked the Quickstart into a tested onboarding/update workflow using the preferred command-token syntax while retaining flag equivalents.
- Clarified that `create-yaml`, `convert-yaml`, and `create-setup-script` operate on the configured `current/` project directory.
- Added a complete `Documentation` section covering command forms, subcommands, command-specific modifiers, short options, compatibility aliases, JSON discovery, release-source overrides, setup selectors, configuration/templates, history/retention, recovery, and no-parameter behavior.
- Replaced the Quickstart images with terminal captures generated from an isolated `1.0.2` demo project.
- Added 16 documentation screenshots covering the full command surface in grouped terminal sessions.
- Preserved the existing detailed architecture, Docker lifecycle, TUI, transaction, setup schema, and security documentation below the new command reference.

## 1.0.1

Patch release for validating the corrected Update CLI version-ordering policy on the stable 1.x line.

## 1.0.1 version-policy verification

- Bumped the release version from `1.0.0` to `1.0.1`.
- Added regression coverage proving `1.0.1 > 1.0.0` for the `update-cli` project.
- Added archive-selection coverage proving `1.0.1` is selected ahead of `1.0.0`, `0.8.23`, and historical `3.3.4`.
- Added updater policy coverage proving `1.0.0 -> 1.0.1` is accepted without `--allow-downgrade`.
- No runtime behavior, configuration schema, setup schema, or CLI contract changes.

## 1.0.0

Stable 1.0 release after the 0.8.x hardening cycle. Existing flag commands, command-token aliases, config schema 6, setup schema 2, command-ui discovery, TUI and `--no-ui` behavior remain compatible.

## 1.0.0 review and hardening

- Fixed Update CLI's release-epoch comparison so `1.0.0` is a normal upgrade from both `0.8.x` and the historical pre-reset `2.x/3.x` line.
- Release swaps no longer pre-delete a PID-derived `.old-*` path; every swap uses a unique recovery path.
- Transaction snapshot and release staging directories use unique temporary directories instead of reusable PID names.
- Lock metadata is written atomically; incomplete locks become recoverable after a grace period instead of remaining permanently ambiguous.
- `restore latest` selects only validated backups, and explicit backup paths reject symlink escapes.
- Post-commit history write failures are warnings instead of falsely turning a successful update/rollback/restore/backup/cleanup into a failed operation.
- ZIP update/verify processing now uses a metadata preflight plus one extraction/checksum pass instead of repeated full decompression; duplicate ZIP paths are rejected.
- rsync checksum comparison is skipped for guaranteed-new release/snapshot destinations while remaining enabled for existing-tree synchronization and restore verification.
- An incomplete unterminated final `history.jsonl` record from a crash is ignored; malformed committed history lines remain errors.
- Removed dead duplicate no-parameter normalization code.

See `CODE_REVIEW.md` and `IMPLEMENTATION_REPORT.md` for details and validation results.

## 0.8.23

- Align fullscreen `Projekt-Setup` metadata with the setup-step output gutter.
- Render `Projekt:` and `Schema:` at the same horizontal position as child command stdout/stderr below `[NN/NN]` step rows.
- Derive the metadata gutter from the actual step-counter width instead of hard-coding indentation, including manifests with three-digit step counts.
- Apply the same content-region alignment to nested schemaVersion 1 setup manifests.
- Add UI regression coverage for metadata/output alignment.

## 0.8.22

- Fullscreen TUI header now shows the currently installed project version next to the project name, e.g. `life-os v0.1.1`.
- Project name and project version remain separate internal values; status/history output keeps the original project name.
- Successful updates refresh the header to the newly installed version before the TUI closes.
- Standalone setup manifests show a version when a sibling `VERSION` file can be detected.

## 0.8.21

- Added project-level Docker lifecycle configuration: `auto`, `disabled`, `required`.
- Existing projects without a `docker` block resolve to `auto`.
- `auto` no longer aborts filesystem updates when Docker/Compose/status detection is unavailable; it warns and disables Docker lifecycle management for that transaction.
- `disabled` guarantees that update transactions and recovery do not invoke Docker, even when a Compose file exists.
- `required` retains strict Docker behavior when a Compose file exists; Docker failures abort the transaction. No Compose file remains a valid no-op.
- Transaction recovery now remembers whether Docker lifecycle management was actually established, so degraded `auto` and `disabled` never attempt Docker recovery actions.
- Docker command diagnostics preserve command, working directory, exit code, stdout and stderr, including failures of `docker compose version`.
- `status` exposes the configured Docker lifecycle; `doctor` distinguishes disabled/auto/required severity.
- Added deterministic transaction, recovery, doctor, config and Life OS integration coverage without requiring a real Docker daemon.

# Release Notes

## 0.8.20

- Remove `Task: <name>` headings and task separator rules from `--setup --no-ui`; direct setup output now starts immediately with numbered step blocks.
- Add `--noui` as a compatibility alias for the canonical `--no-ui` option while retaining `---no-ui`.
- Expose `--noui` in machine-readable command discovery and generated setup-script wrappers.
- Expand Docker Compose transaction failures with the exact command, working directory, exit code, stderr, and stdout.
- Apply the same detailed command diagnostics to Docker Compose status, start, and stop operations.
- Add parser, direct-output, Docker failure-detail, and PTY regression coverage.

## 0.8.19

- Align fullscreen setup stdout/stderr beneath each `[NN/NN]` step using a fixed visual output gutter.
- Normalize leading child-process whitespace so `VALIDATE`, `CHECK`, build output, and other tool messages start at one consistent column.
- Render stderr in the same gutter with an additional `!` marker.
- Keep wrapped long process-output lines aligned with a hanging indent instead of jumping back to the left edge.
- Align command-detail lines (`❯ ...`) with the same step-output gutter.
- Add regression tests for output alignment, stderr alignment, and wrapped continuation indentation.

## 0.8.18

- Treat an update to the already installed project version as a successful no-op instead of an error.
- Fullscreen TUI marks the version-policy step successful and shows `Version <version> ist bereits installiert` on a green content background.
- Same-version no-op uses the normal `Update beenden | Enter zum Schließen` footer and exits with code 0.
- `--force` retains its existing behavior and explicitly reinstalls the same version.
- `--no-ui` reports the already-installed state as normal information and leaves the installed version in the final shell status line.
- Added unit, integration, and PTY regression coverage for the already-installed update path.

## 0.8.17

- Refined `--setup --no-ui` step rendering: each step heading now carries its horizontal separator inline, e.g. `[04/09] Validate project ─────`.
- Command/stdout/stderr remain grouped below the heading with the existing `│` guide and `└─` completion marker.
- Added regression coverage for the 72-column inline step rule and skipped-step rendering.

## 0.8.16

- Redesign `--setup --no-ui` step output into visually grouped step blocks.
- Add a vertical `│` guide for command stdout/stderr so process output remains attached to the step that produced it.
- Close each setup step with `└─ ✓ <step>` or `└─ ✗ <step>` instead of repeating the old flat `[NN/NN] ✓ ...` row.
- Separate setup tasks with a visible `Task: <name>` heading and horizontal rule.
- Render skipped direct-mode steps with the same closed-block layout.
- Add unit regressions for direct step grouping, process-output indentation, task separation, and skipped-step rendering.
- Document the new plain/no-UI setup layout in README.md.

## 0.8.15

- Add machine-readable CLI discovery through `update-cli help --json` and `update-cli help --json` using command-ui schemaVersion 1.
- Add non-breaking command-token aliases (`check`, `update`, `rollback`, `restore`, `status`, `list`, `doctor`, `init`, and others) that normalize into the existing flag-based execution path.
- Add the `setup list/task/workflow/manifest` command hierarchy while retaining all legacy setup flags.
- Add command aliases for YAML lifecycle, configuration, and template operations supported by the current implementation.
- Add structured `setup list --json` output for dynamic setup task/workflow selectors.
- Describe rollback releases and restore backups through dynamic `list --json` value sources in the discovery contract.
- Guarantee that help discovery is deterministic, side-effect free, ANSI/TUI free, and writes JSON only to stdout.
- Add discovery-contract, JSON-safety, dynamic-source, setup-catalog, and flag/token equivalence regressions.
- Document command-ui validation/launch commands and the backward-compatible command aliases in README.md.

## 0.8.14

- Align all fullscreen project/update information values to one fixed second column.
- Render `Release Update` as a normal information row so `from ...`, project name, source path, release path, current path, and protected paths start at the same value column.
- Highlight only the target version in `from A to B` with a blue background and white bold text.
- Apply the same aligned release-update row to plain output.
- Add rendering regressions for column alignment and target-version highlighting.

## 0.8.13

- Added a persistent final console status line after version checks and successful updates.
- Successful updates now end with `Update CLI Version X.Y.Z | <project> | Aktualisiert auf Version: vA.B.C`.
- Version checks that do not install a release end with `Installierte Version: vA.B.C` (or `Keine Version installiert`).
- In fullscreen mode the final status is printed only after leaving the alternate screen, so it remains visible in shell scrollback.
- `--no-ui` receives the same final status line; JSON output is intentionally unchanged.
- Added unit, integration, and PTY regression coverage for the final status contract.

## 0.8.12

- Added `update-cli config --set KEY=VALUE` for generic CLI-based editing of `.update-cli/config.json`.
- Added `config` as the preferred subcommand spelling while retaining `--config` compatibility.
- Added dotted-path updates for nested fields such as `backup.keep`, `security.allowHttp`, and `source.url`.
- Added tolerant key matching, so `no-parameter`, `no_parameter`, and the JSON key `no parameter` resolve to the same setting.
- Added typed value conversion for strings, booleans, integers, floating-point values, and string lists.
- Added repeatable `--set` assignments; all changes are validated together and written atomically only when the resulting configuration is valid.
- Updated README Quickstart to configure automatic post-update setup with `update-cli config --set no-parameter="check,setup"`.


## 0.8.11

- Rewrite the README Quickstart around the current no-parameter workflow introduced in 0.8.x.
- Document that new `--init` projects default to `"no parameter": ["check"]` and can normally be operated with a bare `update-cli` command.
- Document `"no parameter": ["check", "setup"]` as the streamlined workflow that automatically runs setup after an accepted update without a second setup confirmation.
- Clarify that update and setup confirmation modals default to YES and Enter immediately accepts the highlighted action.
- Add explicit examples for `--update --setup`, `--update --no-setup`, setup-only execution, and CI/`--no-ui`.
- Add a compact workflow comparison table to the Quickstart.

## 0.8.10

- Replace the low-contrast inverted project badge in the fullscreen header with a single consistent blue/white header line.
- Render the header as `Update CLI Version X.Y.Z   |   <project>   |   <phase>` for version check, update, and setup screens.
- Keep the version and current phase visible when terminal width is constrained by truncating the project segment first.
- Add unit and PTY regression coverage for the three-part header layout.

## 0.8.9

- Update confirmation modal now selects **YES** by default.
- Project-setup confirmation modal now selects **YES** by default.
- Pressing Enter immediately confirms the highlighted YES action; cursor, Tab, and explicit j/n controls remain unchanged.
- PTY regression coverage verifies the default-YES behavior for both update and setup confirmation flows.

## 0.8.8

- Add the project name to the right edge of the fullscreen TUI header.
- Render the project badge with a white background and blue bold text while preserving the existing blue/white header title.
- Keep the project badge visible when the TUI changes from version check to update or setup.
- Populate the header project name from configured projects and standalone `update-cli.yaml` manifests.
- Add unit and PTY regression coverage for the project badge.

## 0.8.7

- New projects created with `--init` now default `"no parameter"` to `["check"]`.
- Built-in project templates also use `check`, so applying a template during initialization preserves the new default behavior.
- Existing project configurations are not modified.
- Add regression coverage for both plain initialization and built-in templates.

## 0.8.6

- Protect `.gitignore` during rsync synchronization and restore.
- Add `.gitignore` to the default `sync.preserve` list and generated project templates.
- Automatically append `.gitignore` to older/custom preserve lists at runtime.
- Add regression coverage that verifies a project-local `.gitignore` is never overwritten by a release.

## 0.8.5

- Add compatibility parsing for the structured schemaVersion-1 setup manifest used by older generated setup files.
- Accept `version` as a map with `file`, `required`, and `pattern` instead of misinterpreting it as the manifest schema version.
- Translate legacy `build`, `runtime`, `go`, `setup.steps`, and grouped `commands` sections into executable setup steps.
- Preserve folded multiline `ldflagsTemplate` and command-list block scalars.
- Support the legacy built-in step IDs used by generated setup scripts, including Go, Node, Python, Composer, Docker, command groups, build, version verification, and post commands.
- Add regression coverage based on the x-cli structured schemaVersion-1 manifest that previously failed with `ungültige version`.

## 0.8.3

- Fix the Justfile duplicate `clean` recipe that prevented commands such as `just deploy` from parsing.
- Keep `clean` for `update-cli --clean`, add `clear-releases` as an explicit release-cleanup alias, and preserve local build-artifact cleanup as `clear-build`.
- Detect unknown or misspelled CLI flags before standard flag parsing and suggest the closest supported option when confidence is high.
- Example: `--vesion` now reports `Meinten Sie --version?` instead of only returning a generic unknown-flag error.
- Add regression tests for unique Justfile recipe names and typo suggestions.

## 0.8.2

- Hardens setup bootstrap selection by validating the actual `update-cli.yaml` with each candidate binary before using it.
- Prevents older schemaVersion-2 binaries that reject newer metadata such as `project.slug` from being selected just because they expose workflow/task flags.
- Adds a regression test for fallback from a slug-incompatible candidate to a compatible local binary.


## 0.8.1

- refine fullscreen confirmation modal styling so YES/NO button borders remain neutral
- apply green/red styling only to the selected button content area, matching the header/footer visual treatment
- preserve LEFT/RIGHT selection, Enter confirmation, direct j/y/n shortcuts, and default-NO behavior
- add regression coverage preventing selection background colors from leaking into button borders

## 3.3.4

- Accept optional `project.slug` in schemaVersion 1 and 2 setup manifests.
- Expose the slug as `{{ project.slug }}` to schemaVersion 2 variables and operations.
- Preserve `project.slug` during schemaVersion 1 to 2 conversion.
- Show the project slug in setup metadata when present.
- Add regression coverage for manifests containing `project.slug`.


## 3.3.3

- add a polished repository/README hero image under `docs/update-cli-readme-header.png`
- add a 2:1 GitHub social-preview asset under `docs/update-cli-social-preview.png`
- display the README hero directly below the project title while keeping Quickstart as the first functional section
- ignore `.release-project`, `.release-source`, and `.release-version` generated release-state files

## 3.3.2

- move Quickstart to the beginning of the GitHub README, immediately after the product introduction
- document a complete typical workflow: initialize project, generate setup automation, check for a release, run the transactional update, execute project setup, and verify the final status
- add six GitHub-renderable terminal screenshots under `doc/images/quickstart/`, one for each Quickstart step
- show the interactive YES/NO update modal with cursor-key controls in the Quickstart
- explain the default local release naming convention and `--no-ui` alternative in the walkthrough

## 3.3.1

- refresh the GitHub-facing README to document the complete current CLI surface, transactional update model, TUI/`--no-ui`, schemaVersion-2 setup engine, setup-file lifecycle, deterministic setup-script conversion, and optional AI refinement
- add interactive LEFT/RIGHT selection to fullscreen confirmation modals
- LEFT selects `YES`, RIGHT selects `NO`, and Enter confirms the highlighted button
- repaint the selected button immediately while cursor keys are used
- retain direct `j`/`y` and `n` shortcuts plus Tab selection toggling
- keep both update and post-update setup confirmations defaulted to `NO`
- add PTY regression coverage for LEFT -> YES and LEFT -> RIGHT -> NO confirmation sequences

## 3.3.0

- extend `--create-yaml` with `--from project|setup-script`
- deterministically analyze legacy `setup.sh` files and produce ordered schemaVersion-2 setup drafts
- add `--with-ai` refinement for `--create-yaml --from setup-script`
- pass both the original setup script and deterministic draft to AI and accept the result only after schemaVersion-2 parser validation
- support `ollama`, `openai-compatible`, and `nvidia` AI provider identifiers
- install an editable setup-script conversion prompt and AI configuration example under the global Update CLI config path
- preserve unrecognized setup-script behavior as `shell: |` rather than inventing typed semantics

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
- Keep upward `.update-cli/config.json` discovery for commands executed from `current/` or nested project directories.

## 3.1.0

- introduced `update-cli.yaml` schemaVersion 2 as a declarative project automation model
- added named workflows and reusable tasks with dependency resolution, de-duplication, and cycle detection
- added `--setup-list`, `--setup-task NAME`, and `--setup-workflow NAME`
- explicit manifests can combine `setup --manifest` with task/workflow selection
- added task variables and built-ins including `{{ env.NAME | fallback }}`
- added required and optional command requirements
- added structured `when` conditions with `all`, `any`, and `not`
- added per-step `cwd`, environment, timeout, retries, and allow-failure controls
- added typed operations for commands, shell, filesystem preparation, assertions, Python environments/packages, Go, Node package managers, Composer/Artisan, Docker Compose, HTTP checks, downloads, ZIP extraction, and explicit deployment
- retained `command` and `shell` as generic escape hatches so arbitrary project setup operations remain possible
- converted Update CLI's own `update-cli.yaml` to schemaVersion 2 with `prepare`, `check`, `build`, `verify`, `deploy`, and `clean` tasks and `setup`/`ci` workflows
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

- migrated the supplied x-cli legacy setup flow to a declarative `update-cli.yaml` example
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

- `update-cli setup` can execute `update-cli.yaml` directly when invoked inside a deployed `current/` directory without a project config
- `/usr/local/etc/update-cli/setup-template.sh` delegates to the native `setup --manifest` fullscreen runner
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
- `setup.sh` now probes `update-cli help` for `setup --manifest` before invoking the installed binary, so older installations no longer emit an unknown-flag usage dump
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

- introduced strict `update-cli.yaml` schema
- reusable `setup.sh` bootstrap template
- typed setup handlers for Go, Python, Node, Laravel, Docker Compose, copy, deploy, and shell commands
- direct `setup --manifest` execution mode
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
