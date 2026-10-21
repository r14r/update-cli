---
layout: page
title: Configuration
permalink: /configuration/
---

# Project configuration

Update CLI has **two independent schemas**:

| File | Schema | Responsibility |
|---|---|---|
| `update-cli.yaml` | `schemaVersion: 2` | Versioned project setup, update source, tasks, workflows, run command |
| `.update-cli/config.json` | `schemaVersion: 10` | Local runtime settings and user/host-specific options |

Never use `schema: 10` or `project.repository` inside `update-cli.yaml`. The repository source is nested under `update.source.repository`.

## Minimal project manifest

```yaml
schemaVersion: 2
project:
  name: Example
  slug: example
update:
  mode: pull
  source:
    type: repository
    repository: https://github.com/OWNER/REPO.git
    ref: main
workflows:
  bootstrap:
    tasks: [bootstrap]
  precommit:
    tasks: [precommit]
  test:
    tasks: [test]
  build:
    tasks: [build]
tasks:
  bootstrap:
    steps:
      - id: bootstrap
        shell: just bootstrap
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
run:
  command: just run
```

`update-cli install` invokes `just install` through its native install behavior.

## Local runtime config

```json
{
  "schemaVersion": 10,
  "source": {
    "defaultUser": "YOUR_GITHUB_USERNAME"
  },
  "no parameter": ["workflow"]
}
```

Use `no parameter: ["workflow"]` only when the complete unattended lifecycle is appropriate to this application. The workflow currently runs update, bootstrap, test, build, install and run in order.

## Validate before executing

```bash
update-cli doctor
update-cli schema --view
update-cli setup --list
```

See also [schema documentation]({{ '/setup-schema/' | relative_url }}) and [real manifests](https://github.com/r14r/update-cli/tree/main/docs/examples).
