#!/usr/bin/env bash
set -euo pipefail

branch="${1:?Usage: just ci-branch <branch>}"
root="$(git rev-parse --show-toplevel)"
cd "$root"

# Avoid checkout/pull collisions with staged, unstaged or untracked work.
if [[ -n "$(git status --porcelain --untracked-files=normal)" ]]; then
  echo "ERROR: Working tree is not clean. Commit or stash local changes first." >&2
  git status --short >&2
  exit 1
fi

git fetch origin "$branch"
if git show-ref --verify --quiet "refs/heads/$branch"; then
  git switch "$branch"
else
  git switch --track -c "$branch" "origin/$branch"
fi
git pull --ff-only origin "$branch"
just ci
