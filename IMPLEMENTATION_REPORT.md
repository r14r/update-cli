# Implementation Report — Update CLI 1.0.0

## Scope

Full pre-1.0 code review and hardening of the 0.8.23 codebase. The stable release intentionally preserves the public CLI/config/setup contracts and concentrates changes on correctness, recovery safety, security and I/O efficiency.

## Implemented improvements

1. **Stable version transition** — Update CLI-specific release epochs now order legacy 2.x/3.x < transitional 0.8.x+ < stable 1.x. Tests cover 0.8.23 -> 1.0.0 and 3.3.4 -> 1.0.0. Other projects still use strict SemVer.
2. **Crash-safe locks** — atomic lock metadata; incomplete metadata gains a one-minute grace period and then becomes stale/recoverable.
3. **Unique transaction/release staging** — transaction snapshots and release stages use `MkdirTemp`; directory swaps never delete pre-existing `.old-*` recovery directories.
4. **Safer restore** — `latest` skips invalid backups; explicit backup paths are canonicalized and final symlinks are rejected.
5. **Correct post-commit semantics** — history append failures after a successful committed action are warnings, not false operation failures.
6. **ZIP processing optimization** — metadata preflight + one extraction/checksum pass for update/verify, with duplicate normalized path rejection.
7. **rsync optimization** — `--checksum` is retained for existing-tree comparison but omitted for guaranteed-new staging/snapshot destinations.
8. **History crash tolerance** — an unterminated malformed final JSONL record is treated as interrupted append residue; committed malformed lines remain fatal.
9. **Code cleanup** — removed unreachable duplicate return and added focused regression coverage around new safety behavior.

## Compatibility

Unchanged: legacy flag-based commands, command-token aliases, config schemaVersion 6, setup schemaVersion 1 compatibility and schemaVersion 2 execution, machine-readable discovery schemaVersion 1, Docker lifecycle modes, fullscreen TUI, `--no-ui`/`--noui`, protected rsync paths and release naming.

## Review record

See `CODE_REVIEW.md` for findings, severity and rationale.

## Validation

Completed successfully:

- `gofmt` clean
- `bash -n setup.sh`
- `bash -n setup-template.sh`
- build-config validation
- `go vet ./...`
- `go test ./...`
- `go test -race ./...`
- fullscreen PTY smoke suite
- machine-readable `--help --json` parse/contract smoke
- `setup list --json` binary smoke
- native Linux build
- macOS amd64 cross-build
- macOS arm64 cross-build
- Linux amd64 cross-build
- end-to-end Update CLI transition `0.8.23 -> 1.0.0` without downgrade override

`command-ui` is not installed in the build environment, so the external `command-ui validate/inspect` commands were not run. The discovery contract remains covered by the repository's automated tests and executable JSON smoke test.
