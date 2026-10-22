# Update CLI 1.0.0 — Code Review

## Scope

Review of the 0.8.23 codebase before promotion to 1.0.0. The review covered CLI parsing and aliases, update transactions, release staging, Docker lifecycle handling, rsync synchronization, ZIP validation/extraction, backup/restore, history, locking, setup execution, configuration, discovery JSON, TUI/direct output, and release build automation.

## Findings addressed for 1.0.0

### Critical — 1.0.0 would be classified as a downgrade

The historical Update CLI version policy treated the 0.8.x reset line as newer than the old 1.x/2.x/3.x line. Without another epoch transition, 1.0.0 would be ordered below 0.8.23 and could not be selected as a normal update.

**Fix:** introduce explicit Update CLI release epochs so legacy 2.x/3.x < transitional 0.8.x+ < stable 1.x. Other managed projects continue to use strict SemVer. Tests cover 0.8.23 -> 1.0.0 and 3.3.4 -> 1.0.0.

### High — stale swap path could delete crash-recovery data

`SwapDirectory` previously used a PID-derived `.old-<pid>` directory and deleted that path before a new swap. PID reuse after a crash could therefore erase a previous recovery directory.

**Fix:** every directory swap receives a unique timestamp/PID/counter recovery path. Existing `.old-*` directories are never pre-emptively removed.

### High — incomplete lock could become permanent

A crash between creating `.release-update.lock/` and writing `lock.json` left an unidentifiable lock that could never be considered stale automatically. Metadata write errors were also ignored after the lock directory had been created.

**Fix:** lock metadata is written atomically; metadata write failure removes the just-created lock. Missing/invalid metadata is protected by a one-minute grace period and then becomes recoverable as stale.

### High — `restore latest` could choose an invalid backup

Backup inventory intentionally lists invalid backup directories for diagnostics, but `Resolve(..., "latest")` previously returned the first item regardless of its validation state.

**Fix:** `latest` now selects the newest validated backup only. If backups exist but none validate, restore fails clearly with `kein validiertes Backup vorhanden`.

### High — backup symlink traversal during explicit restore

An explicitly named backup directory was checked lexically but then inspected with `os.Stat`, which follows symlinks.

**Fix:** explicit backup paths now pass through canonical-inside validation and reject a final symlink before restore.

### Medium — successful committed operation could be reported as failed

After update/rollback/restore/backup/cleanup had already succeeded, failure to append the optional audit history returned an operation error. This could tell the user that an update failed even though the new release was already committed.

**Fix:** post-success history append failures are warnings on stderr. Failure-history recording remains strict while an operation is still failing.

### Medium — ZIP data was decompressed repeatedly

Normal ZIP update validation performed a full `Validate`, then `Extract` performed another full `Inspect`, then extraction decompressed the archive again. `--verify` similarly inspected and extracted separately.

**Fix:** extraction now performs a metadata preflight followed by exactly one extraction/checksum pass and returns archive statistics. Normal updates and `--verify` use this path. Standalone `Inspect` remains a full integrity scan. Duplicate normalized ZIP paths are also rejected.

### Medium — unnecessary rsync checksums on guaranteed-new targets

Release staging and snapshot/backup destinations are newly created directories. `--checksum` forced rsync to hash source data even though there was no existing destination data to compare.

**Fix:** checksum comparison remains enabled for `current`, restore, exact restore, and verification paths, but is disabled for newly created release and snapshot targets.

### Medium — partial final history record could break status/history

A crash during the final append to `history.jsonl` could leave a truncated last JSON object. The entire history then became unreadable.

**Fix:** only an unterminated malformed final record is ignored as crash residue. Malformed records that have a newline (committed history lines) still fail validation.

### Low — duplicate unreachable return

Removed unreachable duplicate code in no-parameter normalization.

## Architecture observations

The application is dependency-light and testable, with clear package boundaries for archive, backup, config, Docker, setup, source, UI and updater behavior. The main remaining maintainability hotspot is orchestration size: `lib/updater/updater.go`, `lib/ui/console.go`, and `lib/projectsetup/setup_v2.go` are large files. They should be split by operation/renderer in a later minor release, but a broad pre-1.0 rewrite was deliberately avoided because it would add regression risk without changing externally visible behavior.

The 1.0.0 review therefore focuses on transaction safety, recovery correctness, data integrity and measurable I/O reduction while preserving the established CLI/config/setup contracts.
