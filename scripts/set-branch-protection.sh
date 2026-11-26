#!/bin/bash
# set-branch-protection.sh — Apply the required-status-check set to master.
#
# Usage:
#   ./scripts/set-branch-protection.sh            # apply
#   ./scripts/set-branch-protection.sh --show     # print the checks master requires
#
# Prerequisites: gh, authenticated with admin on the repository.
#
# master is guarded by a repository ruleset, not by classic branch protection.
# The ruleset also requires squash merges, linear history and resolved review
# threads. This script changes only its required-status-checks rule and leaves
# the other rules as they are. To undo it, edit the ruleset in the repository
# settings.
#
# Why this is a script and not a settings click: choosing the contexts is the
# hard part, and the choice needs a reason attached. See #2569 for the
# verification; the short version is in the two rules below.
#
#   1. A job skipped by its `if:` reports `skipped`, which GitHub treats as
#      passing. Requiring one is safe.
#   2. A workflow skipped by a workflow-level `paths:` filter creates no check
#      run at all, so a required context stays "Expected — waiting for status"
#      and blocks the merge forever. Requiring one is not safe.
#
# Every context below comes from a workflow with no workflow-level `paths:`,
# and each reports on Go-only, frontend-only, helm-only, workflow-only,
# Dockerfile-only, docs-only and dependency-bump pull requests.
#
# `E2E Tests (chromium)` needs its own note. A matrix expands only when its job
# runs, so a skipped job reports under the unexpanded name
# `E2E Tests (${{ matrix.browser }})` and the per-browser context does not
# exist. `e2e-tests-linux` always runs and decides in its first step, so the
# name is always there, including on a diff that builds no image.
#
# Firefox and webkit stay out: both are genuinely conditional on the diff, and
# their per-browser names come and go with the matrix decision.
set -euo pipefail

REPO="${REPO:-denisvmedia/inventario}"
BRANCH="${BRANCH:-master}"
# The GitHub Actions app. Pinning the integration means a check run with the
# same name from another app cannot satisfy the rule.
ACTIONS_APP_ID=15368

CONTEXTS=(
  "lint"
  "test"
  "test-postgres-and-bootstrap"
  "govulncheck"
  "Check Swagger Docs Sync"
  "Check OpenAPI → TS codegen sync"
  "Lint Frontend (React)"
  "Test Frontend (React)"
  "embed smoke test"
  "markdownlint-cli2"
  "dependency-review"
  "E2E Tests (chromium)"
)

show() {
  # The branch rules endpoint returns every active rule, whichever ruleset it
  # comes from.
  gh api "repos/${REPO}/rules/branches/${BRANCH}" \
    --jq '[.[] | select(.type == "required_status_checks")
              | .parameters.required_status_checks[].context]'
}

case "${1:-apply}" in
  --show)
    show
    ;;
  apply)
    ruleset_id=$(gh api "repos/${REPO}/rules/branches/${BRANCH}" \
      --jq '[.[] | select(.ruleset_source_type == "Repository") | .ruleset_id] | unique | .[]')
    if [ -z "$ruleset_id" ]; then
      echo "No repository ruleset applies to ${REPO}@${BRANCH}; create one first." >&2
      exit 1
    fi
    if [ "$(printf '%s\n' "$ruleset_id" | wc -l)" -ne 1 ]; then
      echo "More than one ruleset applies to ${REPO}@${BRANCH} (${ruleset_id//$'\n'/, }); pick one by hand." >&2
      exit 1
    fi

    checks=$(printf '%s\n' "${CONTEXTS[@]}" \
      | jq -R --argjson app "$ACTIONS_APP_ID" '{context: ., integration_id: $app}' \
      | jq -s .)

    # strict stays false. Requiring every branch to be up to date means a
    # rebase per merge, and the image build alone has taken twenty minutes on
    # a busy fleet.
    gh api "repos/${REPO}/rulesets/${ruleset_id}" \
      | jq --argjson checks "$checks" '
          {name, target, enforcement, conditions, bypass_actors,
           rules: (
             [.rules[] | select(.type != "required_status_checks")]
             + [{type: "required_status_checks",
                 parameters: {required_status_checks: $checks,
                              strict_required_status_checks_policy: false,
                              do_not_enforce_on_create: false}}]
           )}' \
      | gh api -X PUT "repos/${REPO}/rulesets/${ruleset_id}" --input - > /dev/null
    echo "Updated ruleset ${ruleset_id}; ${REPO}@${BRANCH} now requires:"
    show
    ;;
  *)
    echo "usage: $0 [apply|--show]" >&2
    exit 2
    ;;
esac
