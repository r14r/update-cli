# 2.6.2 implementation

- Adds schema-version reporting with `update-cli schema --version` and `update-cli schema --version`.
- Uses the exported canonical `projectsetup.SchemaVersion` constant for both JSON Schema generation and CLI output.
- Keeps standalone `update-cli version` behavior unchanged.
- Extends CLI discovery metadata and parser regression tests for the new schema action.

# 2.6.1 implementation

> 2.6.1 is a metadata-only patch release of the verified 2.6.0 implementation; runtime behavior is unchanged.

- Global `--debug` parser support with direct/detailed UI execution for all commands, including resolved root/config/template/source/release/current/preserve/rsync diagnostics.
- Debug-only invocation forwards into the configured no-parameter action.
- Backward-compatible `defaultUser` decoding and canonical migration to `source.defaultUser`.
- End-to-end coverage for the explicit repository bootstrap command `--from-repository --repository <url/path>`.
- Existing end-to-end coverage for download bootstrap remains in place and enforces exact SemVer ZIP naming.
- Setup manifests ignore unknown top-level transition/extension metadata while keeping supported nested fields strict.

# 2.5.0 global configuration layering

- Global configuration is derived from the invoked executable location: `<binary-dir>/../etc/update-cli`.
- Runtime configuration uses a recursive JSON merge: global first, then project-local. The local project file remains the mutation/edit target.
- `sync.preserve` has special additive semantics so installation-wide exclusions form a minimum baseline.
- Template loading now composes global and local files by template name.
- `config --list` exposes both layers explicitly.
- `just install` seeds global defaults non-destructively.
- Missing global files remain valid: built-in configuration/template defaults provide fallback behavior for development or portable use.

# 2.4.3 release/current synchronization fix

- Release staging no longer hard-excludes `.env` or `.env.*`; files present in the selected ZIP/repository snapshot are part of the immutable release.
- `sync.preserve` now has conditional semantics: existing destinations are protected, missing protected paths are seeded from the release before the normal rsync pass.
- Wildcard preserve entries such as `.env.*` are expanded relative to the release root without interpreting wildcard characters in the absolute project path.
- Dry runs report missing protected paths as creations without modifying them.
- Restore uses the same missing-protected-path seeding behavior for consistency.
- Ordinary backup snapshots still exclude `.env`/`.env.*`; transaction snapshots remain exact for rollback safety.

# 2.4.2 release metadata consistency

- `VERSION`, README current release, and the top `RELEASE_NOTES.md` heading are now guarded by an automated project-file test.
- Packaging is verified against the semantic version encoded in the ZIP filename before delivery.

# 2.4.1 bootstrap verification

The local bootstrap contract is now protected by exact end-to-end regression tests for both supported command forms: download bootstrap with `--init <project>` and repository bootstrap with `--init <project> --from-repository <repository>`. The repository integration test no longer relies on the legacy separate `--repository` option.

# Update CLI 2.4.0 implementation

## Configuration split

- `.update-cli/config.json` is again the canonical persistent updater configuration.
- `mode`, `source`, directories, backup/retention, preserve rules, security, Docker, healthcheck and no-parameter policy are JSON configuration.
- `update-cli.yaml` is reserved for setup/run/tasks/workflows and project display metadata.
- `config migrate` upgrades `.update-cli/config.json` in place and writes the backup beside it.

## Setup compatibility

- Manifest discovery prefers `update-cli.yaml` and falls back to `setup.yaml`.
- Legacy `setup.yaml` without an explicit schema is inferred from structure.
- `setup.yaml` schema-1/simple step structures and schema-2 task/workflow structures remain supported.
- Transitional schema-2 top-level `update:`/`cli:` fields are tolerated by the setup parser, but are not active persistent source configuration.
- When no manifest/setup.sh exists and a Justfile is present, explicit setup runs `just build` followed by `just install`.

## Robustness

- Root discovery recognizes `.update-cli/config.json` as the single project runtime-state location.
- macOS temporary path tests canonicalize `/var` and `/private/var` aliases.
- setup bootstrap scripts recognize both YAML filenames.
- config mutations remain transactional and validate the complete JSON configuration before replacement.

## Validation

- Go release directive remains 1.26.5.
- Full unit/integration suite passes under the available local Go 1.23.2 compatibility runner.
- `go vet ./...` passes under the available local toolchain.
- Race tests pass for the modified `config`, `projectsetup`, and `updater` packages.

## Bootstrap and repository shorthand

- `--init <project>` now creates/uses a local project directory and performs the initial transactional installation.
- default bootstrap discovers the newest `<project>-v<MAJOR>.<MINOR>.<PATCH>.zip` in the configured download folder.
- `--from-repository <repository>` accepts a full GitHub URL, `USER/REPO`, or `REPO`; the one-part form uses `.update-cli/config.json` `source.defaultUser`.
- init automatically executes available setup automation after the first deployment.
- project-specific version ordering now treats current 2.x Update CLI releases as newer than 1.x.
