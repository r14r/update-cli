package updater

// ProjectRequirements returns an actionable prompt for integrating an
// application into the Update CLI project lifecycle.
func ProjectRequirements() string {
	return `# Integrate this project with Update CLI

You are working in an existing application repository. Inspect its actual
language, package manager, build tooling and container setup before editing.
Implement the following requirements without changing unrelated application
behavior. Do not invent commands that the project does not support.

## Files at repository root

1. VERSION: semantic version X.Y.Z (single release version source).
2. update-cli.yaml: valid Update CLI *manifest* schemaVersion: 2.
   Runtime .update-cli/config.json uses a separate schema (currently 10).
3. justfile: portable, documented command recipes using installed tools.
4. SOURCE.md: describe the canonical Git repository or release source.

## update-cli.yaml requirements

- Set schemaVersion: 2, project.name, project.slug and project.type.
- Declare reusable tasks under tasks: and named workflows under workflows:.
- Provide workflows named bootstrap, setup, test, build, ci.
- Keep bootstrap idempotent. It installs missing development dependencies
  and prepares the environment, rather than rebuilding on every invocation.
- setup configures the application. Never overwrite existing secrets or .env.
- test verifies formatting, linting and application tests, returning nonzero
  on failures. Do not silently ignore failed checks.
- build creates reproducible artifacts and must depend on successful checks
  where appropriate.
- ci runs noninteractive validation, tests, build and artifact verification.
- Configure run.command or run.steps if the application can be started.
- Add update.* source, directories, retention and Docker policies only
  where needed; keep machine credentials and host policy out of this file.
- Use explicit working directories, typed steps, dependencies, conditions,
  and timeouts. Do not assume execution from the shell's current directory.
- Do not run destructive migrations or Docker shutdowns as bootstrap tasks.
- If Docker Compose is used, make the compose file/project directory explicit,
  preserve volumes, and add healthchecks when applicable.
- Validate using 'update-cli doctor' and 'update-cli setup --list'.

Minimal structural example (replace with verified project-specific steps):

~~~yaml
schemaVersion: 2
project:
  name: Example App
  slug: example-app
  type: generic
workflows:
  bootstrap:
    tasks: [bootstrap]
  setup:
    tasks: [setup]
  test:
    tasks: [test]
  build:
    tasks: [build]
  ci:
    tasks: [test, build]
tasks:
  bootstrap:
    steps:
      - id: bootstrap
        shell: just bootstrap
  setup:
    steps:
      - id: setup
        shell: just setup
  test:
    steps:
      - id: test
        shell: just test
  build:
    steps:
      - id: build
        shell: just build
run:
  command: just run
~~~

## Complete workflow and step coverage

For each project, inspect which of these lifecycle stages apply and define the
corresponding schemaVersion 2 workflow, reusable task, ordered steps,
preconditions, dependencies and error handling. Never invent unsupported
application commands. Document every intentionally inapplicable stage.

| Workflow | Required or suggested steps |
| --- | --- |
| bootstrap | detect tools, install missing dependencies, prepare environment, install hooks |
| setup | validate environment, prepare configuration, preserve secrets, run non-destructive setup, verify setup |
| precommit | formatting, linting, static analysis, fast tests, fail on any error |
| test | unit tests, integration tests, coverage when supported, race or concurrency checks when supported |
| build | validate VERSION, compile/package application, verify output exists and is executable |
| ci | dependencies, formatting, lint, vet/static checks, tests, race tests, build, smoke tests, artifact verification, cross-build where supported |
| install | verify build, install/deploy artifacts safely, verify installed version and executable |
| run | start application, surface errors, support configured run steps |
| healthcheck | check application availability and readiness; only when a service exists |
| update | inspect versions, download/pull, verify release, stage, optional setup, activate, healthcheck, commit/recover |
| backup | snapshot persistent state, verify backup metadata, preserve retention policy |
| clean | remove generated build output without deleting persistent user data |
| cleanup | apply release/backup retention rules, protect active releases |
| doctor | validate runtime JSON, project manifests, dependencies, paths and migration requirements |
| migrate | inspect schema versions, create backup, migrate, revalidate |

Specify steps as schemaVersion 2 typed operations (shell, go, assert,
deploy, remove, etc.) with meaningful names and stable IDs. Define
requires: relationships so invoking a workflow executes prerequisites
exactly once. Prefer idempotence and fail-fast semantics. Never silently
skip broken steps or run destructive work as part of bootstrap.
Ensure the full pipeline is runnable via both just ci and the manifest
ci workflow; verify that all paths still work after moving source files.

## justfile requirements

- Define recipes: bootstrap, setup, test, build, install, run.
- Prefer recipes precommit, fmt, fmt-check, lint, ci, clean when applicable.
- Recipe names correspond to the tasks in update-cli.yaml.
- Commands must use the project-relative working directory and nonzero exits
  for failures. No hardcoded absolute paths or user-specific home directories.
- Install creates or activates artifacts safely; run starts the application.
- Avoid recreating Python virtual environments on every invocation.
- Use shell quoting correctly. In a justfile with Bash recipes, use
  normal shell variables (for example '"$HOME"', not '"$$HOME"').
- For Docker applications, expose compose build/up/down/ps/logs recipes
  using an explicit project name and compose file.
- Provide helpful errors when a required command is unavailable.
- Never hide test failures with '|| true'.

## Verification / expected deliverables

- Show the complete updated update-cli.yaml and justfile.
- Identify requirements that do not apply to this project.
- Run format checks, tests and build; report each actual result.
- Check 'update-cli bootstrap', 'update-cli setup',
  'update-cli test', 'update-cli build', 'update-cli install',
  and 'update-cli run' (or explain non-applicable stages).
- Keep all operations runnable from the project root via Update CLI.
- Do not claim a command or workflow is implemented unless verified.

`
}
