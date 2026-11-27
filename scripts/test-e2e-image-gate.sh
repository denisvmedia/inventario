#!/bin/bash
# test-e2e-image-gate.sh — Exercise the first step of the e2e lanes in
# docker.yml, which decides from the build and manifest job results whether
# there is an image to test.
#
# Usage:
#   ./scripts/test-e2e-image-gate.sh
#
# Prerequisites: ruby, bash.
#
# `E2E Tests (chromium)` is a required check and runs on every pull request,
# so this step is what stands between a failed build and a green required
# check (#2536). The script is read out of the workflow rather than copied
# here, so the two cannot drift. The Linux and OAuth lanes must carry the same
# step; a difference fails the test.
set -euo pipefail

cd "$(dirname "$0")/.."

WORKFLOW=.github/workflows/docker.yml

PROGRAM="$(
  ruby -ryaml -e '
    wf = YAML.load_file(ARGV[0])
    runs = %w[e2e-tests-linux e2e-tests-oauth].map do |job|
      j = wf["jobs"][job] or abort("no #{job} job in #{ARGV[0]}")
      step = j["steps"].find { |s| s["id"] == "gate" }
      abort("no gate step in #{job}") unless step
      abort("the gate step of #{job} is not the first step") unless j["steps"].first.equal?(step)
      step["run"]
    end
    abort("the gate steps of the e2e lanes differ") unless runs.uniq.size == 1
    print runs.first
  ' "$WORKFLOW"
)"
[ -n "$PROGRAM" ] || { echo "empty gate program" >&2; exit 1; }

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

failed=0
# check NAME WANT BUILD_RESULT MERGE_RESULT IMAGE
#   WANT is "run", "skip" (green, nothing to run) or "fail".
check() {
  local name="$1" want="$2" build="$3" merge="$4" image="$5"
  local out="$tmp/out" got status
  : > "$out"
  if BUILD_RESULT="$build" MERGE_RESULT="$merge" IMAGE="$image" GITHUB_OUTPUT="$out" \
    bash -e -c "$PROGRAM" > "$tmp/log" 2>&1; then
    status=0
  else
    status=$?
  fi
  if [ "$status" -ne 0 ]; then
    got=fail
  elif grep -qx 'run=true' "$out"; then
    got=run
  elif grep -qx 'run=false' "$out"; then
    got=skip
  else
    got="no-output"
  fi
  if [ "$got" != "$want" ]; then
    echo "FAIL $name: got '$got', expected '$want'" >&2
    sed 's/^/     /' "$tmp/log" >&2
    failed=1
  else
    printf 'ok   %-48s -> %s\n' "$name" "$got"
  fi
}

IMG=ghcr.io/example/inventario:sha-0123456

check "image published"                        run  success   success   "$IMG"
check "no image for this diff or a fork"       skip skipped   skipped   ""
check "build failed"                           fail failure   skipped   ""
check "build cancelled"                        fail cancelled skipped   ""
check "manifest failed"                        fail success   failure   ""
check "manifest cancelled"                     fail success   cancelled ""
check "manifest succeeded without a reference" fail success   success   ""

exit "$failed"
