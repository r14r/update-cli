---
layout: single
title: Code review — 2026-10-26
permalink: /code-review-2026-10-26/
---

# Update CLI — Code Review (26 October 2026)

## Scope

Inspected the Go CLI command grammar, human help, JSON discovery, the runtime/manifest boundary, the default workflow and Jekyll documentation. The baseline is version 3.4.40; the review changes are proposed for 3.4.41.

## Findings

| Priority | Area | Finding | Action |
|---|---|---|---|
| P1 | CLI documentation | The public command registry, command-specific help and JSON discovery are maintained separately. Missing commands could silently disappear from machine-readable help. | Added missing discovery entries, parameter display and an automated coverage test |
| P1 | Lifecycle | The default workflow activates/installs before precommit and tests in the current seven-stage order. Validation after installation may leave an already-activated failing candidate. | **Architectural follow-up:** introduce a pre-activation validation gate and explicit release states; do not reorder the existing workflow without transactional tests |
| P1 | Transaction recovery | Explicit rollback and restore commands are gone, but the update transaction still performs automatic recovery. Confusing these concepts risks unsafe refactors. | Preserve internal recovery; document public `recover` accurately |
| P2 | Configuration | Runtime `.update-cli/config.json` schema 10 and project `update-cli.yaml` schema 2 have different responsibilities. `tasks.install` is not inferred from the Justfile. | Clarified on the website and in CLI reference; add future manifest/Justfile consistency diagnostics |
| P2 | Documentation | Jekyll used Minima and a separate, aging navigation. Several docs/examples still contained historical commands and lifecycle descriptions. | Updated website theme/navigation and primary CLI reference; additional historical page cleanup recommended |
| P2 | Help | Generic help provided names, while detailed command help depended on manually curated option lists. | Supplemented detailed help with registered discovery options and added a cross-registry test |
| P3 | Testing | `just ci` had previously fetched a branch, making it unsuitable for validating rebased, unpushed changes. | Already corrected in 3.4.40; documented the separate `ci-branch` behavior |

## Validation before merge

Run:

```bash
just ci
update-cli help --details
update-cli help --command update --details
update-cli help --json
cd docs && bundle install && bundle exec jekyll build
```

Confirm GitHub Pages deployment after merge. A passing precommit run for 3.4.40 does **not** validate the changes proposed for 3.4.41.

## Next architectural work

1. Consolidate public command definitions into one registry and derive both help and JSON discovery from it instead of maintaining three sources.
2. Define per-command option contracts and test exact accepted flag sets, including negations, aliases and value-taking options.
3. Improve staged-update integrity: test and build the candidate **before** activating `current/`, and keep commit/recovery atomic.
4. Add a manifest/Justfile cross-check that reports missing task definitions before an update starts, with actionable diagnostics.
5. Separate historical release documentation from the current user guide and automatically detect stale commands.
