#!/usr/bin/env bash
#
# Print "true" when the branch has at least one REQUIRED status check, "false"
# otherwise. Never fails.
#
# This is what decides whether `gh pr merge --auto` means anything. --auto
# defers to required checks, so on a branch with no protection rule the
# required set is empty and --auto merges the moment it is enabled — before a
# single workflow has reported (#1819).
#
# Unreadable is treated as "not configured". Reading a protection rule needs
# more than the default token holds on some repos, and the safe answer to "I
# cannot tell" is to leave the pull request for a human.
#
# Usage: required-checks-configured.sh <owner> <repo> <branch>
set -uo pipefail

owner="${1:?owner required}"
repo="${2:?repo required}"
branch="${3:?branch required}"

count="$(
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
)" || count=""

if [ -n "$count" ] && [ "$count" -gt 0 ] 2>/dev/null; then
  echo "true"
else
  echo "false"
fi
