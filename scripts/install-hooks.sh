#!/usr/bin/env bash
# Wire git hooks for this clone — points `core.hooksPath` at the
# committed `.githooks/` directory.
#
# Run once per clone after `git clone`:
#   ./scripts/install-hooks.sh
#
# Idempotent. Worktrees share the repo's hook config, so running it in
# one worktree wires hooks for all of them.
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

if [[ ! -d .githooks ]]; then
  echo "✗ .githooks/ directory missing — are you in the right repo?" >&2
  exit 1
fi

# Make every hook executable. Git only runs hooks that have the +x bit;
# fresh clones don't always preserve it depending on the umask /
# filesystem, so set it explicitly.
chmod +x .githooks/*

# Set the hook path. `--local` keeps the change scoped to this clone.
git config --local core.hooksPath .githooks

echo "✓ Hooks installed → core.hooksPath = .githooks"
echo ""
echo "Pre-commit gate: frontend prettier --check + eslint (staged files);"
echo "                 Go nolintguard + qtlint (./...) + golangci-lint (staged packages)."
echo "Pre-push gate:   frontend typecheck + i18n drift + vitest --coverage"
echo "                 (when FE changed)."
echo ""
echo "Bypass once with --no-verify (commit) or --no-verify (push); CI still runs the full suite."
echo "Same .git/config is shared by all worktrees — installing here covers every worktree."
