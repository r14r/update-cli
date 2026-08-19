---
layout: page
title: GitHub Pull
---

# Updating a project from GitHub

Use `mode: pull` when `update-cli` should acquire application content directly from GitHub or another Git server.

## Where the repository URL is stored

The repository URL belongs in:

```text
.updater-cli/config.json
```

It does **not** belong in `update-cli.yaml`. That file controls setup/build/run automation after the source has been acquired and deployed.

## Initialize a GitHub-managed project

```bash
update-cli init demo-app \
  --mode pull \
  --repository https://github.com/acme/demo-app.git
```

The relevant configuration is:

```json
{
  "mode": "pull",
  "source": {
    "type": "repository",
    "repository": "https://github.com/acme/demo-app.git",
    "ref": "main"
  }
}
```

Set or change the branch/ref:

```bash
update-cli config --set source.ref=main
```

Change the repository URL:

```bash
update-cli config \
  --set source.repository=https://github.com/acme/new-demo-app.git
```

Inspect the result:

```bash
update-cli config list
update-cli doctor
```

## Normal recurring workflow

```bash
update-cli check
update-cli update --plan
update-cli update
update-cli status
```

### `check`

`check` refreshes remote Git metadata and determines the target version/commit without deploying the remote state.

Pull mode compares both:

- the semantic version from `VERSION`;
- the deployed Git commit stored in `.release-commit`.

Therefore a new commit can be detected even when `VERSION` remains unchanged:

```text
installed: VERSION 1.4.0 @ aaaaaaa
remote:    VERSION 1.4.0 @ bbbbbbb
```

For formal releases, changing `VERSION` is still recommended.

### `update --plan`

```bash
update-cli update --plan
```

Resolves the target state and transaction plan without committing the deployment. Add `--json` for machine-readable output.

### `update`

The first update clones the repository into:

```text
.updater-cli/repository/
```

Later updates reuse it and perform the equivalent of:

```text
git fetch --prune --tags
git pull --ff-only
```

Fast-forward-only pull is deliberate: `update-cli` does not create merge commits or silently reconcile divergent histories in its managed cache.

After acquisition it:

1. reads `VERSION`;
2. snapshots source content without `.git`;
3. validates the snapshot;
4. creates `release/<version>/`;
5. prepares recovery state;
6. synchronizes to `current/`;
7. optionally executes `update-cli.yaml` setup;
8. performs health checks;
9. records version and commit.

## One-time repository override

```bash
update-cli check \
  --mode pull \
  --repository https://github.com/acme/demo-app.git

update-cli update \
  --mode pull \
  --repository https://github.com/acme/demo-app.git
```

The configured repository is not rewritten by a one-time CLI override.

## Convert an existing ZIP-managed project

```bash
update-cli config \
  --set mode=pull \
  --set source.type=repository \
  --set source.repository=https://github.com/acme/demo-app.git \
  --set source.ref=main
```

No `update-cli.yaml` change is required solely because the acquisition source changed.

## Private repositories

Use authentication already configured for your system Git client. Verify access independently with:

```bash
git ls-remote git@github.com:acme/demo-app.git
```

or:

```bash
git ls-remote https://github.com/acme/demo-app.git
```

## Useful state

```text
.updater-cli/config.json       source/mode configuration
.updater-cli/repository/       persistent Git checkout
.updater-cli/history.jsonl     operation history
release/                       validated release snapshots
current/                       active deployed application
backup/                        persistent backups
```
