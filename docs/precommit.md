# Local quality gates

The same checks run locally and in GitHub Actions. The hooks are **opt-in** and
run only after installation. A failed check rejects the commit or push.

## Setup

Requires Go 1.26.5 (as declared in go.mod), Python 3 and Bash.

```sh
just hooks-install
just precommit
```

`just hooks-install` sets `core.hooksPath` to `.githooks` for this clone.
If you already use custom hooks, review this setting before installation.
To disable these project hooks, use `just hooks-uninstall`.

## Commands

- `just precommit` — formatting, shell syntax, Justfile escaping, version,
  Go vet and unit tests. Also runs automatically before `git commit`.
- `just prepush` — race tests, native build, setup catalog, TUI smoke and
  macOS/Linux cross-builds. Also runs automatically before `git push`.
- `just precommit-all` — runs both suites, matching CI.
- `just fmt` — fixes Go formatting, then rerun `just precommit`.

Hooks invoke `bash scripts/precommit.sh` so executable file modes are not
required for this entrypoint. Git executes the hook scripts themselves;
if copied through tooling that drops execute bits, run:
```sh
chmod +x .githooks/pre-commit .githooks/pre-push
```

These commands do not install dependencies silently; run `go mod download`
or provision Go/Python before running them. Cross-compilation tests compilation,
not execution on remote architectures. GitHub CI remains the enforcement point
for contributions that bypass hooks.
