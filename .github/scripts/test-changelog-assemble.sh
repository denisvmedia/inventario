#!/bin/bash
# test-changelog-assemble.sh — Exercise .github/scripts/changelog-assemble.sh on a
# small CHANGELOG.md and hand-written fragments, one case at a time.
#
# Usage:
#   ./.github/scripts/test-changelog-assemble.sh
#
# Prerequisites: bash, awk. Runs with the macOS and the GNU tools alike.
#
# The release pull request runs the script to build the release section that
# the release workflow publishes, and Markdown Lint runs its --check on every
# pull request that adds a fragment. A case that fails must leave every file
# as it was.
set -euo pipefail

cd "$(dirname "$0")/../.."
script="$(pwd)/.github/scripts/changelog-assemble.sh"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
root="$tmp/root"

changelog='# Changelog

Intro text.

## [Unreleased]

## [1.0.0] - 2026-01-02

### Added

- First feature.

## [0.9.0] - 2025-12-01

### Fixed

- An old fix.

[Unreleased]: https://github.com/example/repo/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/example/repo/compare/v0.9.0...v1.0.0
[0.9.0]: https://github.com/example/repo/releases/tag/v0.9.0'

# setup [CHANGELOG-TEXT] — a fresh root with CHANGELOG.md and the README.
setup() {
  rm -rf "$root"
  mkdir -p "$root/changelog.d"
  printf '%s\n' "${1:-$changelog}" > "$root/CHANGELOG.md"
  printf '# Changelog fragments\n' > "$root/changelog.d/README.md"
}

# fragment NAME — the fragment's content comes on stdin.
fragment() {
  cat > "$root/changelog.d/$1"
}

snapshot() {
  (cd "$root" && find . -type f | LC_ALL=C sort | while IFS= read -r f; do cksum "$f"; done)
}

failures=0
pass() { printf 'ok   %s\n' "$1"; }
fail() {
  printf 'FAIL %s\n%s\n' "$1" "$2"
  failures=$((failures + 1))
}

# expect_fail DESCRIPTION WANT ARGS... — the script must fail, print WANT,
# and leave every file as it was.
expect_fail() {
  desc="$1" want="$2"
  shift 2
  before="$(snapshot)"
  if out="$("$script" --root "$root" "$@" 2>&1)"; then
    fail "$desc" "     want a failure with: $want
     got success: $out"
  elif ! printf '%s' "$out" | grep -qF -- "$want"; then
    fail "$desc" "     want: $want
     got:  $out"
  elif [ "$(snapshot)" != "$before" ]; then
    fail "$desc" "     the failed run changed files"
  else
    pass "$desc"
  fi
}

# expect_ok DESCRIPTION ARGS... — the script must succeed; its stdout lands
# in $tmp/stdout.
expect_ok() {
  desc="$1"
  shift
  if ! "$script" --root "$root" "$@" > "$tmp/stdout" 2> "$tmp/stderr"; then
    fail "$desc" "     want success, got: $(cat "$tmp/stderr")"
    return 1
  fi
}

# same DESCRIPTION WANT-TEXT FILE
same() {
  if printf '%s\n' "$2" | diff -u - "$3" > "$tmp/diff"; then
    pass "$1"
  else
    fail "$1" "$(cat "$tmp/diff")"
  fi
}

valid_fragments() {
  fragment b-feature.md <<'EOF'
### Fixed

- A fix with a continuation line
  ([#2](https://github.com/example/repo/issues/2)).

### Added

- A feature.
- Another feature.
EOF
  # Blank lines around the sections are not part of the entries.
  fragment a-ops.md <<'EOF'


### Operators

- An operator entry.


### Added

- A feature from the first file.

EOF
}

section='### Added

- A feature from the first file.
- A feature.
- Another feature.

### Fixed

- A fix with a continuation line
  ([#2](https://github.com/example/repo/issues/2)).

### Operators

- An operator entry.'

# --- The release -----------------------------------------------------------

setup
valid_fragments
if expect_ok "assemble a release" --date 2026-02-03 v1.1.0; then
  same "assemble a release: CHANGELOG.md" "# Changelog

Intro text.

## [Unreleased]

## [1.1.0] - 2026-02-03

$section

## [1.0.0] - 2026-01-02

### Added

- First feature.

## [0.9.0] - 2025-12-01

### Fixed

- An old fix.

[Unreleased]: https://github.com/example/repo/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/example/repo/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/example/repo/compare/v0.9.0...v1.0.0
[0.9.0]: https://github.com/example/repo/releases/tag/v0.9.0" "$root/CHANGELOG.md"
  left="$(cd "$root/changelog.d" && ls -A)"
  if [ "$left" = README.md ]; then
    pass "assemble a release: deletes the fragments, keeps the README"
  else
    fail "assemble a release: deletes the fragments, keeps the README" "     left: $left"
  fi
fi

setup
expect_fail "assemble without fragments" "no fragments" 1.1.0

setup
valid_fragments
expect_fail "assemble a version that is already released" "version 1.0.0 is already in the file" 1.0.0

setup
valid_fragments
expect_fail "assemble something that is not a version" "not a version: 1.1" 1.1

setup
valid_fragments
expect_fail "assemble a pre-release" "not a version: 1.1.0-rc.1" 1.1.0-rc.1

setup
valid_fragments
expect_fail "assemble a version older than the newest release" \
  "version 0.9.5 is not newer than 1.0.0, the newest release in the file" 0.9.5

# Numeric, not string, order: 1.0.10 is newer than 1.0.9.
setup "$(printf '%s\n' "$changelog" | sed 's/1\.0\.0/1.0.9/g')"
valid_fragments
if expect_ok "check a version that is newer only by numeric order" --check 1.0.10; then
  pass "check a version that is newer only by numeric order"
fi
setup "$(printf '%s\n' "$changelog" | sed 's/1\.0\.0/1.0.9/g')"
expect_fail "check a version that is older only by numeric order" "is not newer than 1.0.9" --check 1.0.8

# A pre-release written by hand ranks below its release.
setup "$(printf '%s\n' "$changelog" | sed 's/^## \[1\.0\.0\]/## [1.1.0-rc.1] - 2026-01-20\
\
### Added\
\
- A candidate.\
\
## [1.0.0]/')"
valid_fragments
if expect_ok "check the release after its hand-written pre-release" --check 1.1.0; then
  pass "check the release after its hand-written pre-release"
fi
expect_fail "check a version older than a hand-written pre-release" "is not newer than 1.1.0-rc.1" --check 1.0.5

setup
valid_fragments
expect_fail "assemble with a bad date" "not a date" --date 2026-2-3 1.1.0

setup
valid_fragments
expect_fail "assemble with month 13" "not a date" --date 2026-13-01 1.1.0

setup
valid_fragments
expect_fail "assemble with day 32" "not a date" --date 2026-02-32 1.1.0

setup
expect_fail "assemble without a version" "usage:"

# --- The check -------------------------------------------------------------

setup
valid_fragments
before="$(snapshot)"
if expect_ok "check valid fragments" --check; then
  if [ -s "$tmp/stdout" ] || [ "$(snapshot)" != "$before" ]; then
    fail "check valid fragments" "     printed $(cat "$tmp/stdout") or changed files"
  else
    pass "check valid fragments"
  fi
fi

setup
valid_fragments
if expect_ok "check with a version" --check --date 2026-02-03 1.1.0; then
  same "check with a version: prints the section" "## [1.1.0] - 2026-02-03

$section" "$tmp/stdout"
fi

setup
if expect_ok "check without fragments" --check; then
  pass "check without fragments"
fi

setup
printf '### Added\r\n\r\n- A line from Windows.\r\n' | fragment crlf.md
if expect_ok "check a fragment with CRLF line endings" --check 1.1.0; then
  if grep -q "$(printf '\r')" "$tmp/stdout"; then
    fail "check a fragment with CRLF line endings" "     the section keeps the carriage returns"
  else
    pass "check a fragment with CRLF line endings"
  fi
fi

setup
expect_fail "check a version that is already released" "version 1.0.0 is already in the file" --check 1.0.0

# --- Fragments the check refuses ---------------------------------------------

setup
printf '### Improved\n\n- Something.\n' | fragment unknown.md
expect_fail "unknown section" 'changelog.d/unknown.md:1: unknown section "Improved"' --check

setup
printf '### added\n\n- Something.\n' | fragment lower.md
expect_fail "section spelled in lowercase" 'unknown section "added"' --check

setup
printf '### Added\n\n### Fixed\n\n- A fix.\n' | fragment no-entries.md
expect_fail "section without entries" "changelog.d/no-entries.md:1: ### Added has no entries" --check

# The check that closes a file runs once awk reads the next one, and must
# still name the file it is about.
setup
printf '### Added\n' | fragment a-heading-only.md
printf '### Fixed\n\n- A fix.\n' | fragment b-valid.md
expect_fail "section without entries, followed by another fragment" \
  "changelog.d/a-heading-only.md:1: ### Added has no entries" --check

setup
: | fragment empty.md
expect_fail "empty file" "changelog.d/empty.md: empty fragment" --check

setup
printf '\n\n' | fragment blank.md
expect_fail "file of blank lines" "changelog.d/blank.md:1: no entries" --check

setup
printf 'Some text.\n\n### Added\n\n- Something.\n' | fragment text.md
expect_fail "text before the first heading" "changelog.d/text.md:1: text before the first ### section heading" --check

setup
printf '### Added\n\nSomething.\n' | fragment paragraph.md
expect_fail "entry that is not a list item" "changelog.d/paragraph.md:3: an entry is a list item" --check

setup
printf '## [1.1.0] - 2026-02-03\n\n### Added\n\n- Something.\n' | fragment release.md
expect_fail "release heading in a fragment" "changelog.d/release.md:1: a fragment has no headings other than ###" --check

setup
printf '### Added\n\n- An entry.\n\nA paragraph that is not indented.\n' | fragment paragraph-after.md
expect_fail "unindented paragraph after an entry" "changelog.d/paragraph-after.md:5: an entry is a list item" --check

setup
printf '### Added\n\n- An entry.\n<!-- a note -->\n' | fragment comment.md
expect_fail "HTML comment line after an entry" "changelog.d/comment.md:4: an entry is a list item" --check

# changelog-section.sh and the What's new parser both end the release at a
# link definition, so everything after it would vanish from both.
setup
printf '### Added\n\n- See [the docs][docs].\n\n[docs]: https://example.com/docs\n\n### Fixed\n\n- A fix.\n' | fragment linkdef.md
expect_fail "link reference definition in a fragment" "changelog.d/linkdef.md:5: a link reference definition" --check

setup
printf '### Added\n\n- An entry.\n[Unreleased]: https://github.com/example/repo/compare/v1.0.0...HEAD\n' | fragment unreleased-link.md
expect_fail "[Unreleased] link in a fragment" "changelog.d/unreleased-link.md:4: a link reference definition" --check

setup
printf '### Added\n\n- An entry\n  with a continuation,\n\n  and a second paragraph.\n- Another entry.\n' | fragment continued.md
if expect_ok "check indented continuation lines, also after a blank line" --check; then
  pass "check indented continuation lines, also after a blank line"
fi

setup
printf '\357\273\277### Added\n\n- Something.\n' | fragment bom.md
expect_fail "fragment with a byte order mark" "changelog.d/bom.md: starts with a UTF-8 byte order mark" --check

setup
printf '### Added\n\n- Something.\n' | fragment Feature.md
expect_fail "upper-case file name" "changelog.d/Feature.md: a fragment name uses" --check

setup
printf '### Added\n\n- Something.\n' | fragment feature.txt
expect_fail "file name without .md" "changelog.d/feature.txt: a fragment name uses" --check

setup
mkdir "$root/changelog.d/nested"
expect_fail "directory in changelog.d" "changelog.d/nested: not a regular file" --check

setup
printf '### Improved\n\n- Something.\n' | fragment a-unknown.md
: | fragment b-empty.md
expect_fail "every problem is reported: the first" 'unknown section "Improved"' --check
expect_fail "every problem is reported: the second" "changelog.d/b-empty.md: empty fragment" --check

# --- CHANGELOG.md the assembly cannot edit -------------------------------------

setup "$(printf '%s\n' "$changelog" | sed 's/^## \[Unreleased\]$/## [Unreleased]\
\
### Added\
\
- Written by hand./')"
expect_fail "Unreleased with entries" "CHANGELOG.md:7: ## [Unreleased] holds text" --check

setup "$(printf '%s\n' "$changelog" | grep -v '^\[Unreleased\]: ')"
expect_fail "no Unreleased link" "no [Unreleased] link definition" --check

setup "$(printf '%s\n' "$changelog" | sed 's#^\[Unreleased\]: .*#[Unreleased]: https://github.com/example/repo/commits/master#')"
expect_fail "Unreleased link that is not a compare link" "does not end in /compare/<previous tag>...HEAD" --check

setup "$(printf '%s\n' "$changelog" | grep -v '^## \[Unreleased\]$')"
expect_fail "no Unreleased heading" 'no "## [Unreleased]" heading' --check

setup "$(printf '%s\n' "$changelog" | sed 's/^## \[Unreleased\]$/## [Unreleased] /')"
expect_fail "Unreleased heading with trailing whitespace" 'CHANGELOG.md:5: "## [Unreleased]" has trailing whitespace' --check

if [ "$failures" -ne 0 ]; then
  echo "$failures case(s) failed" >&2
  exit 1
fi
