# Explicit project lifecycle

The top-level commands deliberately separate source synchronization, local
environment preparation, compilation and launching the application:

| Command | Behavior | Expected project recipe |
| --- | --- | --- |
| `update-cli update` | Fetch and activate the configured release without running setup/build/install | none |
| `update-cli bootstrap` | Prepare the project's toolchain, Python venv, modules and Git hooks | `just bootstrap` via manifest workflow |
| `update-cli build` | Validate/compile the application without installing it | `just build` via manifest workflow |
| `update-cli run` | Execute the manifest's `run` definition | project-specific |
| `update-cli install` | Install separately from build | `just install` |

`update-cli update --setup` remains available for the earlier transactional
update-plus-setup behavior. `update-cli setup` and named setup workflows are
also retained. The `update-cli build` and `bootstrap` commands use the
manifest workflow of the same name; other projects must declare them in their
own `update-cli.yaml`.

## Example Docker project

A web application can define its run behavior explicitly. Never assume every
application is Docker-based or that opening a browser is appropriate in CI.

```yaml
schemaVersion: 2
project:
  slug: my-web-app
run:
  description: Start containers and open the frontend
  cwd: .
  steps:
    - name: Start containers
      shell: just docker-start
    - name: Open frontend
      shell: just docker-open
      when:
        os: [darwin]
workflows:
  bootstrap:
    tasks: [bootstrap]
  build:
    tasks: [build]
tasks:
  bootstrap:
    steps:
      - name: Prepare dependencies
        shell: just bootstrap
  build:
    steps:
      - name: Compile application
        shell: just build
```

For headless automation, keep the browser-open operation in a separately
selected workflow or add explicit config so it can be disabled. Compose
startup should normally be followed by a health/readiness check.

## Recommended follow-up capabilities

- `update-cli validate`: manifest validation, formatter, lint and tests
  without modifying source or installing
- `update-cli start / stop / restart`: persistent process/Compose lifecycle;
  keep `run` as interactive/foreground when configured
- `update-cli health`: verify installed binary, HTTP endpoint and/or Compose
  service health before reporting success
- `update-cli deploy`: explicit install/restart/health/rollback transaction
- `update-cli pipeline`: opt-in end-to-end `update → bootstrap → build → deploy → health`
- `--dry-run`, `--no-open`, `--json`, and structured exit codes for automation

The command-level aliases intentionally do not invent or auto-run a Docker
recipe when no manifest exists.
