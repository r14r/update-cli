# Proposed root-driven lifecycle

All CLI commands should resolve the project root without changing the calling shell's current directory. Workflow actions execute using an explicit working directory, never by replacing the caller's shell directory.

## Stage order

1. update: resolve and fetch the requested source into an isolated release candidate; do not change current/.
2. bootstrap: prepare the candidate's build-time tools/dependencies.
3. setup: prepare configuration and pre-deployment resources, avoiding irreversible schema changes.
4. test: static analysis, tests, security checks.
5. build: produce a verifiable artifact.
6. install: transactionally activate verified artifacts, apply compatible deployment migration, perform readiness checks, then commit or rollback.
7. run: start application/services according to explicit desired state.
8. open: open a configured URL or UI (opt-in, never automatically in headless CI).

The proposed stages are logical stages, not a guarantee that every deployment should execute every stage. CI should use bootstrap, test, and build but not install. Production may use a prebuilt artifact and skip development-only bootstrap/test.

## Repository layout

- ROOT/.update-cli: durable project configuration, state, logs and locks
- ROOT/release/VERSION: immutable source or release artifacts
- ROOT/current: activated release; never assumed to be the caller's working directory

Every process should use an explicit cwd (or Git -C) pointing to the candidate for build/test and current/ for runtime operations. No command should use os.Chdir in the host process.

## Required invariants

- Stage execution logs identify stage, source, cwd and version.
- The update lock covers installation/activation.
- Failures before install preserve current unchanged.
- Activation and failures after activation use a rollback transaction.
- Healthchecks are required for configured runtime deployments and must not be replaced by mere process startup.
- Persistent volumes and user data are excluded from release synchronization.
- Database migrations must be rollback-aware, generally backward compatible.
- Docker Compose commands pass an explicit project directory or working directory; avoid accidentally creating a different Compose project.
- Run/open are optional deployment post-actions, never executed unconditionally in CI.
- Workflow definitions are validated before destructive actions.
- Unknown or missing required tasks cause explicit errors; no silent skip.

## Existing implementation gaps

- update --no-setup currently installs a release without running the full proposed lifecycle.
- test and install use dedicated justfile runners rather than a unified stage engine.
- bootstrap, build, ci and precommit resolve to manifest workflows.
- run uses manifest-defined application execution.
- A new open command and a unified lifecycle orchestrator remain to be implemented.
- The current manifest schema is 2, independent of runtime configuration schema 10.

The migration to a unified orchestrator should preserve backwards compatibility with the existing workflow grammar and the established source/rollback machinery.
