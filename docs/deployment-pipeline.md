---
layout: page
title: Deployment pipeline
permalink: /deployment-pipeline/
---

# Validated deployment pipeline (experimental)

The original `update-cli workflow` remains unchanged:

```text
update → bootstrap → install → precommit → test → build → run
```

An **opt-in** pipeline changes the installation order and adds a mandatory healthcheck:

```bash
update-cli workflow --pipeline --plan
update-cli workflow --pipeline --noui
```

```text
update → bootstrap → precommit → test → build → install → run → healthcheck
```

The pipeline checks the configured healthcheck before starting, fails on the first stage error, and requires a nonempty healthcheck type other than `none` in the effective runtime configuration.

**Safety limitation:** `update` still activates the release using the existing update transaction **before** precommit, test and build. The pipeline is *not yet* a full pre-activation validation transaction, and a failed post-run healthcheck does **not** automatically roll back the deployment. Do not use `--pipeline` as a zero-downtime or fully atomic production release guarantee.

For true transactional promotion, the remaining architecture work is staging the release, building and testing it *in staging*, atomically switching `current/`, and restoring the prior release if post-activation checks fail.

The `--plan` option only prints steps. It does not update, build or install anything.

See [code review](./code-review-2026-10-21.md) for roadmap and risks.
