#!/usr/bin/env bash
#
# Print the CHANGELOG.md section for one release, so the release workflow can
# publish it as the GitHub Release body instead of asking a human to write the
# same list twice.
#
# Usage: changelog-section.sh <tag> [changelog-path]
#   tag  — e.g. v1.2.0; the leading "v" is optional and the file is expected
#          to use the bare version in its headings ("## [1.2.0] - ...").
#
# Exits 1 when the section is missing, which the caller treats as "no notes to
# publish" rather than a build failure: a tag can still ship hand-written notes
# from .github/release-notes/<tag>.md, and goreleaser can still generate a
# changelog from commits.
set -euo pipefail

TAG="${1:?tag required}"
FILE="${2:-CHANGELOG.md}"
VERSION="${TAG#v}"

[ -f "$FILE" ] || {
  echo "no $FILE" >&2
  exit 1
}

# Everything between this version's heading and whatever ends the section: the
# next `## ` heading, or — for the oldest entry, which has none after it — the
# block of link definitions at the foot of the file.
section="$(
  awk -v version="$VERSION" '
    $0 ~ "^## \\[" version "\\]" { collecting = 1; next }
    collecting && /^## / { exit }
    collecting && /^\[[^]]+\]: / { exit }
    collecting { print }
  ' "$FILE"
)"

# Strip leading and trailing blank lines.
section="$(printf '%s\n' "$section" | sed -e '/./,$!d' | sed -e ':a' -e '/^\n*$/{$d;N;ba' -e '}')"

if [ -z "$section" ]; then
  echo "no section for $VERSION in $FILE" >&2
  exit 1
fi

printf '%s\n' "$section"
