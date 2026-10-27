---
layout: single
title: Staged candidate quality checks
permalink: /preactivate/
---

# Pre-activation checks (1.9.0)

Update CLI already prepares a candidate beside `current/`. Projects using manifest schemaVersion 2 may opt into a quality workflow executed **inside the candidate** before it is activated.

Add the following workflow to the project's `update-cli.yaml`:

```yaml
schemaVersion: 2
project:
  name: Example
  slug: example
  type: generic

workflows:
  preactivate:
    description: Validate the candidate before deployment
    tasks: [precommit, test, build]

tasks:
  precommit:
    steps:
      - id: precommit
        shell: just precommit
  test:
    steps:
      - id: test
        shell: just test
  build:
    steps:
      - id: build
        shell: just build
```

These steps run in the candidate's working directory, after staged setup and before `current/` activation. If a step fails, activation is aborted and transaction recovery is performed. Generated build artifacts remain inside the candidate and are activated with it.

**Opt-in compatibility:** Existing manifests without a `preactivate` workflow continue to use the existing update behavior. Defining a `test` or `build` workflow alone does not opt in.

**Isolation limitations:** A shell command that uses absolute paths, Docker bind mounts, external services, secrets, or deployment commands may affect live resources regardless of its working directory. Only configure staging-safe commands here. The existing public `workflow` command still performs its separate post-update sequence and can duplicate checks; a complete consolidation remains future work.

**Version numbering:** Update CLI itself has deliberately reverted from version 3.5.0 to 1.9.0. Automatic downgrades may be rejected by older installations; use the explicit downgrade option only after reviewing the target and backups.
