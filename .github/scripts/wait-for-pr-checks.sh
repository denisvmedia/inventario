#!/usr/bin/env bash
#
# Block until every check on a pull request has finished, and fail if any of
# them failed.
#
# `gh pr merge --auto` defers to the branch's REQUIRED status checks. When a
# branch has no protection rule, "required" is the empty set, so --auto merges
# the moment it is enabled — before a single workflow has reported (#1819).
# This script is what makes the bot auto-merge path independent of repo
# settings: the caller only reaches `gh pr merge` once CI is actually green.
#
# Usage: wait-for-pr-checks.sh <pr-url-or-number>
set -euo pipefail

PR="${1:?pull request url or number required}"

# `gh pr checks` exits 8 when the PR has no checks at all, which is what it
# reports in the seconds between a PR opening and its workflows registering.
# Wait for the first check to appear rather than treating that window as
# "nothing to wait for".
APPEAR_TIMEOUT_SECONDS="${APPEAR_TIMEOUT_SECONDS:-300}"
APPEAR_INTERVAL_SECONDS="${APPEAR_INTERVAL_SECONDS:-15}"

deadline=$(($(date +%s) + APPEAR_TIMEOUT_SECONDS))
while :; do
  status=0
  gh pr checks "$PR" >/dev/null 2>/tmp/gh-pr-checks.err || status=$?

  # 0 = checks exist and all passed already; 1 = they exist and at least one
  # failed or is pending. Either way there is something to watch.
  if [ "$status" -eq 0 ] || [ "$status" -eq 1 ]; then
    break
  fi

  # Anything that is not "no checks reported" (8) — auth, network, a deleted
  # PR — should surface now instead of being retried until the deadline.
  if [ "$status" -ne 8 ]; then
    echo "::error::gh pr checks failed (exit $status) for $PR"
    cat /tmp/gh-pr-checks.err >&2 || true
    exit "$status"
  fi

  if [ "$(date +%s)" -ge "$deadline" ]; then
    # Fail closed. A PR whose changes trigger no workflow at all is a real
    # case (a bump that every path filter skips), and merging it unchecked is
    # what this script exists to prevent — so stop and ask for a human rather
    # than falling through to the merge.
    echo "::error::no checks appeared on $PR within ${APPEAR_TIMEOUT_SECONDS}s; not enabling auto-merge"
    exit 1
  fi

  echo "Waiting for checks to be registered on $PR..."
  sleep "$APPEAR_INTERVAL_SECONDS"
done

# --watch polls until every check concludes and exits non-zero if any failed.
# --fail-fast stops at the first failure instead of waiting out the rest.
echo "Watching checks on $PR..."
gh pr checks "$PR" --watch --fail-fast --interval 30
