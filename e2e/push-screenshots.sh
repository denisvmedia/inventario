#!/usr/bin/env bash
# Publish a captured screenshot run to a GitHub branch + print raw URLs.
#
# The default screenshot output of e2e/screenshots.mjs lives at
# .research/screenshots/<git-branch>/, which is in the maintainer's
# global gitignore (intentional — visual review is local-only by
# default per the screenshot-review skill). When you DO want reviewers
# to see them (issue comments, design audits, mock-vs-real
# comparisons), this script:
#
#   1. Reads PNGs from .research/screenshots/<src-label>/
#   2. Creates a worktree on `assets/screenshots-<dest-label>` from
#      master (or fast-forwards an existing branch with the same name)
#   3. Replaces the assets/screenshots-<dest-label>/ folder with the
#      latest captures, commits, pushes
#   4. Prints the commit SHA + a raw-URL prefix you can paste straight
#      into a GitHub issue/PR comment
#
# Usage:
#   e2e/push-screenshots.sh <dest-label> [src-label] [glob...]
#
# Trailing glob args filter which PNGs from <src-label> get published —
# useful when the source folder is a full screenshots.mjs run but the
# destination branch should only carry the slices relevant to the issue.
# When no globs are given, everything in the source folder is published.
#
# Examples:
#   e2e/push-screenshots.sh 1381           # source = current git branch, all PNGs
#   e2e/push-screenshots.sh 1527 1527      # explicit source label, all PNGs
#   e2e/push-screenshots.sh pr-1583 bold-valley
#   e2e/push-screenshots.sh 1381 bold-valley '*register*' '*reset*' '*profile-edit*'
#
# Notes:
# - Pinning raw URLs to the printed commit SHA (not the branch HEAD)
#   makes the embeds stable across re-pushes — the commit-pinned blob
#   is the canonical pattern in #1527 / #1529 / #1549.
# - This script uses the local git CLI directly because the github_and_git
#   MCP path-allowlist + binary-content limits keep it from pushing
#   PNGs. The skill rule allows fallback "if the operation can't be
#   expressed via the MCP" — that applies here.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if [[ $# -lt 1 ]]; then
  echo "usage: e2e/push-screenshots.sh <dest-label> [src-label] [glob...]" >&2
  exit 64
fi

dest_label="$1"

# Default source label = current branch (slug-ified the same way
# screenshots.mjs derives its OUT folder). When the second arg is
# omitted we DON'T fall through to glob-mode — globs only kick in
# from arg 3 onward, so `e2e/push-screenshots.sh 1381` still works.
if [[ $# -ge 2 ]]; then
  src_label="$2"
  shift 2
else
  src_label="$(git -C "$REPO_ROOT" rev-parse --abbrev-ref HEAD)"
  src_label="${src_label//[^a-zA-Z0-9._-]/-}"
  if [[ "$src_label" == "HEAD" || -z "$src_label" ]]; then
    src_label="latest"
  fi
  shift
fi

# Remaining args (if any) are glob patterns to filter the source PNGs.
include_globs=("$@")

src_dir="$REPO_ROOT/.research/screenshots/$src_label"
if [[ ! -d "$src_dir" ]]; then
  echo "error: source folder not found: $src_dir" >&2
  echo "       run e2e/screenshots.mjs first, or pass an explicit src-label." >&2
  exit 1
fi

# Build the list of PNGs to publish — full directory by default, or a
# glob-filtered subset when patterns were supplied. Patterns are
# matched against the basename only (e.g. `*register*` matches every
# file with "register" in its name).
shopt -s nullglob
if [[ ${#include_globs[@]} -eq 0 ]]; then
  src_pngs=("$src_dir"/*.png)
else
  declare -A seen=()
  src_pngs=()
  for pattern in "${include_globs[@]}"; do
    for f in "$src_dir"/$pattern; do
      [[ -f "$f" ]] || continue
      [[ "$f" == *.png ]] || continue
      if [[ -z "${seen[$f]+x}" ]]; then
        seen["$f"]=1
        src_pngs+=("$f")
      fi
    done
  done
fi
shopt -u nullglob
if [[ ${#src_pngs[@]} -eq 0 ]]; then
  echo "error: no PNGs to publish from $src_dir" >&2
  if [[ ${#include_globs[@]} -gt 0 ]]; then
    echo "       (no files matched the supplied glob patterns: ${include_globs[*]})" >&2
  fi
  exit 1
fi

branch="assets/screenshots-$dest_label"
worktree_path="$REPO_ROOT/.dev/worktree/assets-screenshots-$dest_label"
target_subdir="assets/screenshots-$dest_label"

# Fetch so the branch-exists check sees the remote state, then create
# the worktree off either the remote branch (if it already exists) or
# off origin/master. Removing any stale local worktree avoids the
# "already exists" failure on re-runs.
git -C "$REPO_ROOT" fetch origin --quiet

if git -C "$REPO_ROOT" worktree list --porcelain | grep -q "^worktree $worktree_path$"; then
  git -C "$REPO_ROOT" worktree remove --force "$worktree_path"
fi

if git -C "$REPO_ROOT" ls-remote --exit-code --heads origin "$branch" >/dev/null 2>&1; then
  # Branch exists — check it out, fast-forward if possible, or rebase
  # onto master to keep history linear.
  git -C "$REPO_ROOT" worktree add "$worktree_path" "origin/$branch"
  git -C "$REPO_ROOT" -C "$worktree_path" checkout -B "$branch" "origin/$branch"
else
  git -C "$REPO_ROOT" worktree add -b "$branch" "$worktree_path" origin/master
fi

# Replace the screenshots subdir wholesale so renames/deletes are
# captured, not just additions. mkdir -p is idempotent. Only the PNGs
# selected by the optional glob filter get copied (everything by
# default).
mkdir -p "$worktree_path/$target_subdir"
rm -f "$worktree_path/$target_subdir"/*.png
cp "${src_pngs[@]}" "$worktree_path/$target_subdir/"

cd "$worktree_path"
git add "$target_subdir/"

# No-op guard: if the on-disk PNGs match the previous commit byte-for-
# byte, skip the empty commit + push.
if git diff --cached --quiet; then
  echo "[push-screenshots] no changes vs origin/$branch — skipping commit/push"
  sha="$(git rev-parse HEAD)"
else
  git commit -m "[#$dest_label] visual proof: refresh captures from $src_label"
  git push origin "$branch"
  sha="$(git rev-parse HEAD)"
fi

cd "$REPO_ROOT"
git worktree remove "$worktree_path"

cat <<EOF

[push-screenshots] published $branch
  commit: $sha
  files:  ${#src_pngs[@]} PNG(s) under $target_subdir/

Paste-ready raw URL prefix (commit-pinned, stable across re-pushes):
  https://raw.githubusercontent.com/denisvmedia/inventario/$sha/$target_subdir/<file>.png

Image markdown for one shot:
  <img src="https://raw.githubusercontent.com/denisvmedia/inventario/$sha/$target_subdir/01-FILENAME.png" width="320">
EOF
