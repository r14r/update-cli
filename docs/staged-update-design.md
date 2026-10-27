---
layout: single
title: Staged update design for 3.5
permalink: /staged-update-design/
---

# Staged updates: 3.5.0 foundation

## Current behavior

The updater already fetches and validates the artifact, prepares a versioned release in a staging folder, creates a separate `current.next-<transaction>` candidate, and can execute the project's setup against that candidate. It then activates the candidate through atomic directory replacement or an in-place rsync. Transaction recovery remains internal to the updater.

The normal seven-step `workflow` still executes `update → bootstrap → install → precommit → test → build → run`. That workflow is **not yet a pre-activation quality gate**: build/test run after `update` has completed.

## 3.5.0 first increment (implemented)

Before the updater enters `activate-current`, a new `activation-gate` phase verifies:

1. The candidate staging path is not `current/` and is not a child of `current/`.
2. The candidate path is a real directory rather than a symbolic link.
3. The candidate has a `VERSION` marker matching the expected target release.
4. Failures flow through existing transactional cleanup/recovery and failure history, without invoking activation.

This adds a concrete safety boundary without changing project manifest behavior, running new scripts, or reordering the public workflow.

## Planned next increments

- **Candidate checks:** define an opt-in manifest workflow executed entirely against the staged candidate before activation. Distinguish safe offline validation from operations that require network, services, or secrets.
- **Build artifacts:** ensure test and build outputs in staging are preserved during activation, without allowing symlinks to escape the candidate.
- **Activation and health:** preserve an immutable previous release and recover if activation, service startup, or health verification fails.
- **Compatibility:** test ZIP/pull sources, fresh installation, existing persistent `current/` data, Docker lifecycle, `sync.preserve`, in-place and atomic activation, and interrupted-transaction recovery.
- **Observability:** expose candidate validation and activation state in readable and machine-readable update plans/history.

## Release policy

The first 3.5.0 implementation is intentionally incremental. A passing candidate gate is **not** proof that `test`, `precommit`, or `build` have passed before activation.
