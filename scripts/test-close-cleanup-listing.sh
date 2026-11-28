#!/bin/bash
# test-close-cleanup-listing.sh — Exercise the PR-close reclaim step of
# docker.yml against a fake `gh` that serves a package listing page by page.
#
# Usage:
#   ./scripts/test-close-cleanup-listing.sh
#
# Prerequisites: ruby, jq, bash 4 or newer.
#
# The step stops paging at the first page older than the pull request, which
# is what keeps a close from reading every version of the package
# (devdocs/ci/github-api-budget.md). A wrong stop deletes nothing, but it
# leaves the pull request's images behind; a missing stop costs a request per
# 100 versions. The script is read out of the workflow rather than copied
# here, so the two cannot drift.
set -euo pipefail

cd "$(dirname "$0")/.."

WORKFLOW=.github/workflows/docker.yml
STEP='Delete all GHCR image versions published by this PR'

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

ruby -ryaml -e '
  wf = YAML.load_file(ARGV[0])
  step = wf.dig("jobs", "cleanup", "steps")&.find { |s| s["name"] == ARGV[1] }
  abort("no step #{ARGV[1].inspect} in the cleanup job of #{ARGV[0]}") unless step
  print step["run"]
' "$WORKFLOW" "$STEP" > "$tmp/step.sh"

mkdir -p "$tmp/bin" "$tmp/pages"
cat > "$tmp/bin/gh" <<'FAKE'
#!/bin/bash
args="$*"
echo "$args" >> "$FAKE_DIR/calls.log"
case "$args" in
  *"/pulls/"*"/commits"*) printf '%s\n' aaaaaaa1111111111111111111111111111111111 bbbbbbb2222222222222222222222222222222222 ;;
  *"/users/"*"--jq .type"*) echo User ;;
  *"--method DELETE"*) ;;
  *"/versions?per_page=100&page="*)
    p="${args##*page=}"; p="${p%% *}"
    if [ -f "$FAKE_DIR/pages/$p.json" ]; then cat "$FAKE_DIR/pages/$p.json"; else echo '[]'; fi ;;
  *) echo "unexpected gh call: $args" >&2; exit 1 ;;
esac
FAKE
chmod +x "$tmp/bin/gh"

# pages ORDER COUNT: writes COUNT versions, 100 a page, two minutes apart,
# the newest at 2026-11-28T12:00:00Z. Version 1005 carries pr-42 and the
# sha of the first commit (10 minutes old), 1110 the sha of the second
# (220 minutes old), 1007 master's tags. ORDER is "newest-first",
# "oldest-first", "swapped" (pages 2 and 4 exchanged) or "bad-dates" (page 2
# without readable dates).
pages() {
  rm -f "$tmp/pages/"*.json
  ruby -rjson -rtime -e '
    order, count, dir = ARGV[0], ARGV[1].to_i, ARGV[2]
    base = Time.utc(2026, 11, 28, 12, 0, 0)
    tags = { 1005 => %w[pr-42 sha-aaaaaaa], 1110 => %w[sha-bbbbbbb], 1007 => %w[edge master sha-ccccccc] }
    all = (0...count).map do |i|
      id = 1000 + i
      { "id" => id, "created_at" => (base - i * 120).iso8601,
        "metadata" => { "container" => { "tags" => tags.fetch(id, []) } } }
    end
    all.reverse! if order == "oldest-first"
    pages = all.each_slice(100).to_a
    pages[1], pages[3] = pages[3], pages[1] if order == "swapped"
    pages[1].each { |v| v["created_at"] = "garbage" } if order == "bad-dates"
    pages.each_with_index { |pg, i| File.write("#{dir}/#{i + 1}.json", JSON.generate(pg)) }
  ' "$1" "$2" "$tmp/pages"
}

failed=0
# check NAME PR_CREATED_AT WANT_REQUESTS WANT_DELETED_IDS
check() {
  local name="$1" created="$2" want_requests="$3" want_ids="$4" got_requests got_ids
  : > "$tmp/calls.log"
  if ! FAKE_DIR="$tmp" PATH="$tmp/bin:$PATH" GH_TOKEN=x OWNER=o REPO=o/r \
    PACKAGE_NAME=inventario PR_NUMBER=42 PR_CREATED_AT="$created" \
    bash -e "$tmp/step.sh" > "$tmp/log" 2>&1; then
    echo "FAIL $name: the step failed" >&2
    sed 's/^/     /' "$tmp/log" >&2
    failed=1
    return
  fi
  got_requests="$(grep -c '/versions?per_page=' "$tmp/calls.log" || true)"
  got_ids="$(grep -- '--method DELETE' "$tmp/calls.log" | sed 's|.*/||' | sort | tr '\n' ' ' | sed 's/ $//' || true)"
  if [ "$got_requests" != "$want_requests" ] || [ "$got_ids" != "$want_ids" ]; then
    echo "FAIL $name: got $got_requests request(s) deleting [$got_ids], expected $want_requests deleting [$want_ids]" >&2
    sed 's/^/     /' "$tmp/log" >&2
    failed=1
  else
    printf 'ok   %-62s -> %s request(s)\n' "$name" "$got_requests"
  fi
}

# Five hours before the newest version: the cutoff (an hour earlier) falls on
# page 2, so page 3 is the first page wholly older than the pull request.
RECENT=2026-11-28T07:00:00Z
# Older than every version: nothing may stop the listing early.
OLD=2026-11-27T16:00:00Z

pages newest-first 500
check "newest first: stops at the first page older than the PR"    "$RECENT" 3 "1005 1110"
check "newest first, PR older than the package: reads every page"  "$OLD"    6 "1005 1110"
pages oldest-first 500
check "oldest first: never stops early"                            "$RECENT" 6 "1005 1110"
pages bad-dates 500
check "unreadable dates: never stops early"                        "$RECENT" 6 "1005 1110"
pages newest-first 40
check "one short page: no request past it"                         "$OLD"    1 "1005"
# The limit of the check: disorder on a page it never reads is invisible.
# The version there is left in place, not deleted by mistake.
pages swapped 500
check "pages out of order beyond the stop: leaves the version"     "$RECENT" 2 "1005"

exit "$failed"
