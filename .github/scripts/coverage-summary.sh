#!/usr/bin/env bash
# Render Go coverage as a job summary table.
#
#   coverage-summary.sh <go-test-output> <profile> <lane name>
#
# Both inputs come straight from the toolchain: the per-package percentages are
# the ones `go test` printed, and the total is what `go tool cover -func`
# computes. Nothing here does arithmetic of its own — a coverage number that
# was massaged on the way to the screen is worse than no number.
#
# Per lane, not merged. The unit lane and the PostgreSQL lane run different
# tests over overlapping packages, and text profiles cannot be concatenated
# without double-counting the blocks they share. Two honest tables beat one
# wrong percentage.
#
# Nothing here gates a build. It exists because #2114 kept being re-measured by
# hand, and a number nobody can see is a number people argue about.
set -euo pipefail

output="${1:?usage: coverage-summary.sh <go-test-output> <profile> <lane>}"
profile="${2:?usage: coverage-summary.sh <go-test-output> <profile> <lane>}"
lane="${3:?usage: coverage-summary.sh <go-test-output> <profile> <lane>}"

summary="${GITHUB_STEP_SUMMARY:-/dev/stdout}"

{
  echo "## Go coverage — ${lane}"
  echo
} >> "$summary"

if [ -s "$profile" ]; then
  total="$(go tool cover -func="$profile" | awk '$1 == "total:" { print $NF }')"
  echo "Total: **${total:-unknown}**" >> "$summary"
else
  echo "No coverage profile was produced." >> "$summary"
fi

{
  echo
  echo "| Package | Coverage |"
  echo "| --- | ---: |"
} >> "$summary"

# `go test` prints one line per package:
#   ok    pkg/path   0.42s  coverage: 61.6% of statements
#   ?     pkg/path   [no test files]
#         pkg/path            coverage: 0.0% of statements
# The last shape is a package whose tests all skipped, which is exactly the
# case worth seeing.
awk '
  /coverage: [0-9.]+% of statements/ {
    for (i = 1; i <= NF; i++) {
      if ($i == "coverage:") { pct = $(i + 1); break }
    }
    pkg = ($1 == "ok" || $1 == "FAIL") ? $2 : $1
    if (pkg != "" && pct != "") printf "| `%s` | %s |\n", pkg, pct
  }
' "$output" | sort -u >> "$summary"

{
  echo
  echo "_The profile is attached to this run as an artifact: \`go tool cover"
  echo "-html=<profile>\` for the annotated source._"
} >> "$summary"
