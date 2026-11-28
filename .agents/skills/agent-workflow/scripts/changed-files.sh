#!/usr/bin/env bash
# changed-files.sh — Print the files a branch changes, for scoped checks.
#
# Usage:
#   .agents/skills/agent-workflow/scripts/changed-files.sh [base-ref]
#
# The base defaults to $CHANGED_BASE, then origin/master. The list covers
# commits since the merge base with the base, staged and unstaged edits, and
# untracked files that are not ignored: one repository-relative path per
# line, sorted, without duplicates. Deleted paths are included, so callers
# filter by existence where a tool needs the file. Renames show as a deletion
# plus an addition, which keeps the package a file moved out of in scope.
#
# Used by lint-changed.sh and test-changed.sh next to it.

set -euo pipefail

base="${1:-${CHANGED_BASE:-origin/master}}"
# The repository is the one the caller stands in, so the scripts work from
# any checkout or worktree, also when called through the .claude/skills
# symlink or from another checkout's copy.
root="$(git rev-parse --show-toplevel 2>/dev/null)" || {
  echo "changed-files: run it from inside a checkout of the repository" >&2
  exit 2
}
root="$(cd "$root" && pwd -P)"
cd "$root"

if ! git rev-parse --verify --quiet "${base}^{commit}" >/dev/null; then
  echo "changed-files: unknown base ref '${base}' (fetch it, or pass another ref)" >&2
  exit 2
fi

merge_base="$(git merge-base "$base" HEAD)"
{
  git -c core.quotePath=false diff --name-only --no-renames "$merge_base"
  git -c core.quotePath=false ls-files --others --exclude-standard
} | sort -u
