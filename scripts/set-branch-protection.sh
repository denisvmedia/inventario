#!/bin/bash
# set-branch-protection.sh — Apply the required-status-check set to master.
#
# Usage:
#   ./scripts/set-branch-protection.sh            # apply
#   ./scripts/set-branch-protection.sh --show     # print the current protection
#   ./scripts/set-branch-protection.sh --remove   # remove it again
#
# Prerequisites: gh, authenticated with admin on the repository.
#
# Why this is a script and not a settings click: choosing the contexts is the
# hard part, and the choice needs a reason attached. See #2569 for the
# verification; the short version is in the two rules below.
#
#   1. A job skipped by its `if:` reports `skipped`, which branch protection
#      treats as passing. Requiring one is safe.
#   2. A workflow skipped by a workflow-level `paths:` filter creates no check
#      run at all, so a required context stays "Expected — waiting for status"
#      and blocks the merge forever. Requiring one is not safe.
#
# Every context below comes from a workflow with no workflow-level `paths:`,
# and each was confirmed to report on twelve merged pull requests covering
# Go-only, frontend-only, helm-only, workflow-only, Dockerfile-only and
# dependency-bump diffs.
#
# `E2E Tests (chromium)` needs its own note, because it used to be the one
# context that could not be required. A matrix expands only when its job runs,
# so a skipped `e2e-tests-linux` reported under the unexpanded name
# `E2E Tests (${{ matrix.browser }})` and the per-browser context did not
# exist at all. The job now always runs and decides in its first step, so the
# name is always there — verified on a diff that builds no image, where the
# resolver skips and the check still reports as `E2E Tests (chromium)`.
#
# Firefox and webkit stay out: both are genuinely conditional on the diff, and
# their per-browser names come and go with the matrix decision.
set -euo pipefail

REPO="${REPO:-denisvmedia/inventario}"
BRANCH="${BRANCH:-master}"

read -r -d '' PROTECTION <<'JSON' || true
{
  "required_status_checks": {
    "strict": false,
    "contexts": [
      "lint",
      "test",
      "test-postgres-and-bootstrap",
      "govulncheck",
      "Check Swagger Docs Sync",
      "Check OpenAPI → TS codegen sync",
      "Lint Frontend (React)",
      "Test Frontend (React)",
      "embed smoke test",
      "markdownlint-cli2",
      "dependency-review",
      "E2E Tests (chromium)"
    ]
  },
  "enforce_admins": false,
  "required_pull_request_reviews": null,
  "restrictions": null,
  "allow_force_pushes": false,
  "allow_deletions": false
}
JSON

case "${1:-apply}" in
  --show)
    # An unprotected branch answers 404, which is an answer rather than an
    # error — say so instead of leaking the API's message.
    # gh prints the 404 body on stdout, so capture rather than redirect.
    if current=$(gh api "repos/${REPO}/branches/${BRANCH}/protection" \
      --jq '{strict: .required_status_checks.strict,
             contexts: .required_status_checks.contexts,
             enforce_admins: .enforce_admins.enabled}' 2> /dev/null); then
      printf '%s\n' "$current"
    else
      echo "${REPO}@${BRANCH} has no branch protection."
    fi
    ;;
  --remove)
    gh api -X DELETE "repos/${REPO}/branches/${BRANCH}/protection"
    echo "Removed branch protection from ${REPO}@${BRANCH}."
    ;;
  apply)
    # enforce_admins stays false on purpose: if a required context ever
    # misbehaves an admin can still merge, so the worst case is an annoyance
    # rather than a repository nobody can land anything in.
    #
    # strict stays false too. Requiring every branch to be up to date means a
    # rebase per merge, and the image build alone has taken twenty minutes on
    # a busy fleet.
    printf '%s\n' "$PROTECTION" \
      | gh api -X PUT "repos/${REPO}/branches/${BRANCH}/protection" --input - > /dev/null
    echo "Applied branch protection to ${REPO}@${BRANCH}:"
    "$0" --show
    ;;
  *)
    echo "usage: $0 [apply|--show|--remove]" >&2
    exit 2
    ;;
esac
