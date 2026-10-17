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

## Standard CLI development commands

```sh
just bootstrap        # create .venv, install Python tooling, download Go modules, enable hooks
just check            # Go formatting, vet, unit and race tests
just precommit        # fast commit gate
just precommit-all    # complete CI gate
just build            # compile dist/update-cli
just install          # install binary and system config (permissions may require sudo)
just deploy           # alias of install
```

The default binary destination is `/usr/local/bin/update-cli`, with
configuration under `/usr/local/etc/update-cli`. You may override them using
`INSTALL_DIR` and `UPDATE_CLI_CONFIG_DIR` respectively. Example:

```sh
INSTALL_DIR="$HOME/.local/bin" UPDATE_CLI_CONFIG_DIR="$HOME/.config/update-cli" just install
```

`bootstrap` installs Python `pre-commit` into `.venv` as a development tool.
The Git hooks installed for this project are native Bash hooks; Python pre-commit
is not required for their execution.
