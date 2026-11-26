#!/usr/bin/env bash
#
# Print "true" when the branch has at least one REQUIRED status check, "false"
# otherwise. Never fails.
#
# This is what decides whether `gh pr merge --auto` means anything. --auto
# defers to required checks, so on a branch with no required checks --auto
# merges the moment it is enabled — before a single workflow has reported
# (#1819).
#
# Required checks can come from a repository ruleset or from a classic branch
# protection rule; master uses a ruleset. Both are counted.
#
# Unreadable is treated as "not configured". The safe answer to "I cannot
# tell" is to leave the pull request for a human.
#
# Usage: required-checks-configured.sh <owner> <repo> <branch>
set -uo pipefail

owner="${1:?owner required}"
repo="${2:?repo required}"
branch="${3:?branch required}"

# Active rules for the branch, from every ruleset that targets it.
ruleset_count="$(
  gh api "repos/${owner}/${repo}/rules/branches/${branch}" \
    --jq '[.[] | select(.type == "required_status_checks")
              | .parameters.required_status_checks[]] | length' \
    2>/dev/null
)" || ruleset_count=""

protection_count="$(
  gh api graphql \
    -f query='
      query($owner: String!, $repo: String!, $branch: String!) {
        repository(owner: $owner, name: $repo) {
          ref(qualifiedName: $branch) {
            branchProtectionRule {
              requiredStatusCheckContexts
            }
          }
        }
      }' \
    -F owner="$owner" -F repo="$repo" -F branch="$branch" \
    --jq '(.data.repository.ref.branchProtectionRule.requiredStatusCheckContexts // []) | length' \
    2>/dev/null
)" || protection_count=""

for count in "$ruleset_count" "$protection_count"; do
  if [ -n "$count" ] && [ "$count" -gt 0 ] 2>/dev/null; then
    echo "true"
    exit 0
  fi
done
echo "false"
