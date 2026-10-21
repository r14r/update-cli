# Code Review — 2026-10-21

## Scope

Reviewed the current CLI command dispatcher, workflow execution, public syntax, quality gates, documentation and CI wiring. This is a targeted source review, **not** a claim of an executed end-to-end test.

## Implemented in this release

1. Moved workflow stage execution and argument parsing into `lib/updater/workflow.go`. This reduces the already large `updater.go` and isolates one responsibility.
2. Centralized the actual workflow sequence in `workflowStages()`; the regression test now reads the production sequence rather than checking a copied list.
3. Added regression coverage for malformed `--root`, `--root=PATH`, `--no-wait`, `--no-ui`, and `--debug` options.
4. Continue to abort on non-idempotent update errors. A typed `VersionAlreadyInstalledError` remains the sole allowed update skip.

## Priority 0 — safety and correctness

### Define install versus deploy

Current requested sequence is `update → bootstrap → install → precommit → test → build → run`. This is safe only if `install` means installing dependencies or tooling. In the current CLI, `install` invokes `just install`, which some projects use to deploy a built artifact. That can deploy **before tests or build**.

**Proposal:** Introduce separate `dependencies` / `prepare` and `deploy` phases. Keep existing behavior until a documented migration is approved. Deploy only after successful validation/build with healthcheck and rollback on failure.

### Validate target before activation

An update stage may activate a new `current/` before subsequent tests run. Prefer a staged release tree with prepare/check/build before activation, followed by atomic promotion, healthcheck, and rollback support.

## Priority 1 — robustness and testability

- Add end-to-end tests with a fixture project and injectable stage executor to assert stage order, skip semantics, fail-fast, cancellation and cleanup.
- Avoid redundant work when `just precommit` already runs `go test`; allow project-defined phase policies.
- Ensure `run` can represent long-running processes, detached services and a finite healthcheck rather than assuming it always terminates promptly.
- Test interactive terminal stage transitions in a pseudoterminal: no Enter prompts, correct header, clean output region, final failure state.
- Add explicit dry-run / resume semantics and structured JSON workflow events for CI agents.

## Priority 2 — architecture

- Break `lib/updater/updater.go` into dispatcher, initialization, update transaction, lifecycle and presentation modules.
- Replace large flag lists with typed command options and centralized validation; avoid divergent public syntax versus runtime dispatch.
- Unify manifest schema documentation with generated starter config, schema validation, and executable tests.
- Reduce global mutable runtime state and implicit environment variable behavior; inject services for filesystem, Git, time and terminal I/O.

## Priority 3 — product capabilities

- Workflow `--from` / `--until` stage selection and `--resume` from a recorded checkpoint.
- `workflow --plan` and `workflow --json` with machine-readable stage status and durations.
- Project-level dependency graph; detect cycles and duplicate phases.
- Log retention, redaction of secrets, and a single bundle of diagnostics for bug reports.
- Healthcheck, rollback and post-deployment verification as first-class phase results.
- Add Jekyll build validation and VHS tape smoke checks to CI.

## Verification checklist

```bash
gofmt -l .
go vet ./...
go test -count=1 ./...
go test -race ./...
just precommit
just build
update-cli workflow --noui
```

Do not run the final workflow against a production deployment for a smoke test. Use an isolated fixture environment, and review the semantic meaning of `install` first.
