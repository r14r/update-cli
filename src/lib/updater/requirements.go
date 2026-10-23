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
