#!/bin/bash
# changelog-assemble.sh — Move the entries waiting in changelog.d/ into a
# release section of CHANGELOG.md.
#
# Usage:
#   ./scripts/changelog-assemble.sh [--date YYYY-MM-DD] X.Y.Z
#   ./scripts/changelog-assemble.sh --check [X.Y.Z]
#
# The release pull request runs the first form. It inserts
# `## [X.Y.Z] - YYYY-MM-DD` under the empty `## [Unreleased]` heading with the
# entries of every fragment under the headings Added, Changed, Deprecated,
# Removed, Fixed, Security and Operators, in that order. Under one heading the
# entries follow the fragments' file names. It then points the `[Unreleased]`
# link at the foot of the file at vX.Y.Z, adds the link for X.Y.Z, and deletes
# the fragments. The date defaults to today in UTC.
#
# --check changes nothing. It validates the fragments and the parts of
# CHANGELOG.md the assembly edits; with a version it also checks that the
# version is new and prints the section the assembly would insert. Markdown
# Lint runs it on pull requests.
#
# Both forms fail on a fragment with a bad file name, an unknown section, a
# section without entries or no section at all; on an `## [Unreleased]`
# section that holds text; and on a version CHANGELOG.md already has. The
# fragment format is in changelog.d/README.md.
#
# --root DIR works on DIR/CHANGELOG.md and DIR/changelog.d/ instead of the
# repository's own; scripts/test-changelog-assemble.sh uses it.
set -euo pipefail

# The headings a release may have, in the order a release lists them. The
# What's new page test (frontend/src/lib/__tests__/changelog.test.ts) accepts
# the same set and wants Operators last.
SECTIONS="Added Changed Deprecated Removed Fixed Security Operators"

usage() {
  echo "usage: $0 [--root DIR] [--date YYYY-MM-DD] X.Y.Z" >&2
  echo "       $0 [--root DIR] --check [X.Y.Z]" >&2
  exit 2
}

check=false
root="$(dirname "$0")/.."
release_date=""
version=""
while [ $# -gt 0 ]; do
  case "$1" in
    --check) check=true ;;
    --date)
      [ $# -ge 2 ] || usage
      release_date="$2"
      shift
      ;;
    --root)
      [ $# -ge 2 ] || usage
      root="$2"
      shift
      ;;
    -h | --help) usage ;;
    -*)
      echo "unknown option: $1" >&2
      usage
      ;;
    *)
      [ -z "$version" ] || usage
      version="${1#v}"
      ;;
  esac
  shift
done
$check || [ -n "$version" ] || usage

if [ -n "$version" ] && ! printf '%s\n' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'; then
  echo "not a version: $version (want X.Y.Z or X.Y.Z-prerelease)" >&2
  exit 2
fi
[ -n "$release_date" ] || release_date="$(date -u +%Y-%m-%d)"
if ! printf '%s\n' "$release_date" | grep -Eq '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'; then
  echo "not a date: $release_date (want YYYY-MM-DD)" >&2
  exit 2
fi

cd "$root"
[ -f CHANGELOG.md ] || {
  echo "no CHANGELOG.md in $(pwd)" >&2
  exit 1
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

failed=0
count=0
fragments=()
if [ -d changelog.d ]; then
  while IFS= read -r path; do
    name="${path#changelog.d/}"
    case "$name" in
      README.md | .*) continue ;;
    esac
    if [ -L "$path" ] || [ ! -f "$path" ]; then
      echo "$path: not a regular file; a fragment is a file directly in changelog.d/" >&2
      failed=1
    elif ! printf '%s\n' "$name" | grep -Eq '^[a-z0-9][a-z0-9._-]*\.md$'; then
      echo "$path: a fragment name uses a-z, 0-9, '.', '_' and '-' and ends in .md" >&2
      failed=1
    elif [ ! -s "$path" ]; then
      echo "$path: empty fragment" >&2
      failed=1
    else
      fragments+=("$path")
      count=$((count + 1))
    fi
  done < <(find changelog.d -mindepth 1 -maxdepth 1 | LC_ALL=C sort)
fi

# Validates the fragments, in the order given, and prints their entries
# grouped by section. Every problem is reported as FILE:LINE: message.
read -r -d '' FRAGMENTS_AWK <<'AWK' || true
BEGIN {
  n = split(sections, order, " ")
  for (i = 1; i <= n; i++) known[order[i]] = i
  list = order[1]
  for (i = 2; i <= n; i++) list = list ", " order[i]
}
# `file` rather than FILENAME: the checks that close a file run when awk
# has already moved on to the next one.
function err(line, msg) {
  printf "%s:%d: %s\n", file, line, msg > "/dev/stderr"
  errors++
}
function end_block() {
  if (cur && !has_entry) err(cur_line, "### " cur_name " has no entries")
  cur = 0
}
function end_file() {
  end_block()
  if (file != "" && !headings && !text) err(1, "no entries; a fragment has at least one ### section heading with entries under it")
}
FNR == 1 { end_file(); file = FILENAME; headings = 0; text = 0 }
{ sub(/\r$/, "") }
/^#+([ \t]|$)/ {
  if (substr($0, 1, 4) == "### ") {
    name = substr($0, 5)
    sub(/[ \t]+$/, "", name)
    if (name in known) {
      end_block()
      cur = known[name]; cur_name = name; cur_line = FNR
      has_entry = 0; pending = 0; headings++
      next
    }
    err(FNR, "unknown section \"" name "\"; use one of " list)
  } else {
    err(FNR, "a fragment has no headings other than ### with one of " list)
  }
  next
}
/^[ \t]*$/ { if (cur && has_entry) pending++; next }
{
  if (!cur) {
    if (!text) err(FNR, "text before the first ### section heading")
    text = 1
    next
  }
  if (!has_entry && substr($0, 1, 2) != "- ") err(FNR, "an entry is a list item starting with \"- \"")
  has_entry = 1
  for (; pending > 0; pending--) body[cur] = body[cur] "\n"
  body[cur] = body[cur] $0 "\n"
}
END {
  end_file()
  if (errors) exit 1
  out = 0
  for (i = 1; i <= n; i++) {
    if (body[i] == "") continue
    if (out) printf "\n"
    printf "### %s\n\n%s", order[i], body[i]
    out = 1
  }
}
AWK

# Checks the parts of CHANGELOG.md the assembly edits and prints the file.
# With `section` set to a file, the printed file has the new release under
# `## [Unreleased]` and the link definitions point at it.
read -r -d '' CHANGELOG_AWK <<'AWK' || true
BEGIN { tag = "v" version }
function err(line, msg) {
  if (line) printf "CHANGELOG.md:%d: %s\n", line, msg > "/dev/stderr"
  else printf "CHANGELOG.md: %s\n", msg > "/dev/stderr"
  errors++
}
{
  line = $0
  if (skip_blank) {
    if (line ~ /^[ \t]*$/) next
    print ""
    skip_blank = 0
  }
  if (line ~ /^## /) {
    in_unreleased = 0
    if (line == "## [Unreleased]") {
      unreleased++
      in_unreleased = 1
      print
      if (section != "") {
        print ""
        print "## [" version "] - " date
        print ""
        while ((getline entry < section) > 0) print entry
        close(section)
        skip_blank = 1
      }
      next
    }
    if (version != "" && index(line, "## [" version "]") == 1) err(FNR, "version " version " is already in the file")
  } else if (line ~ /^\[[^]]+\]: /) {
    in_unreleased = 0
    if (index(line, "[Unreleased]: ") == 1) {
      links++
      rest = substr(line, 15)
      if (!match(rest, "/compare/[^/]+[.][.][.]HEAD$")) {
        err(FNR, "the [Unreleased] link does not end in /compare/<previous tag>...HEAD")
      } else if (section != "") {
        base = substr(rest, 1, RSTART - 1)
        prev = substr(rest, RSTART + 9, RLENGTH - 16)
        print "[Unreleased]: " base "/compare/" tag "...HEAD"
        print "[" version "]: " base "/compare/" prev "..." tag
        next
      }
    } else if (version != "" && index(line, "[" version "]: ") == 1) {
      err(FNR, "a link for " version " is already in the file")
    }
  } else if (in_unreleased && line ~ /[^ \t]/) {
    err(FNR, "## [Unreleased] holds text; new entries go in changelog.d/ (see changelog.d/README.md)")
    in_unreleased = 0
  }
  print
}
END {
  if (unreleased == 0) err(0, "no \"## [Unreleased]\" heading")
  if (unreleased > 1) err(0, "more than one \"## [Unreleased]\" heading")
  if (links == 0) err(0, "no [Unreleased] link definition at the foot of the file")
  if (links > 1) err(0, "more than one [Unreleased] link definition")
  if (errors) exit 1
}
AWK

: > "$tmp/section.md"
if [ "$count" -gt 0 ] &&
  ! awk -v sections="$SECTIONS" "$FRAGMENTS_AWK" "${fragments[@]}" > "$tmp/section.md"; then
  failed=1
fi
if ! $check && [ "$count" -eq 0 ] && [ "$failed" -eq 0 ]; then
  echo "changelog.d/ has no fragments, so there is nothing to release" >&2
  failed=1
fi

section=""
$check || section="$tmp/section.md"
if ! awk -v version="$version" -v date="$release_date" -v section="$section" \
  "$CHANGELOG_AWK" CHANGELOG.md > "$tmp/CHANGELOG.md"; then
  failed=1
fi

if [ "$failed" -ne 0 ]; then
  echo "nothing was changed" >&2
  exit 1
fi

if $check; then
  echo "changelog.d/: $count fragment(s), all valid" >&2
  if [ -n "$version" ] && [ "$count" -gt 0 ]; then
    printf '## [%s] - %s\n\n' "$version" "$release_date"
    cat "$tmp/section.md"
  fi
  exit 0
fi

# Rewrite in place rather than move the temporary file over it, so the file
# keeps its mode.
cat "$tmp/CHANGELOG.md" > CHANGELOG.md
rm -f "${fragments[@]}"
echo "CHANGELOG.md: added [$version] - $release_date from $count fragment(s) and deleted them from changelog.d/" >&2
