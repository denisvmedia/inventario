#!/bin/bash
# test-image-waiter-verdict.sh — Exercise the producer-job verdict that
# _wait-for-docker-image.yml computes.
#
# Usage:
#   ./scripts/test-image-waiter-verdict.sh
#
# Prerequisites: ruby, awk.
#
# The verdict is ten lines of awk that every image-dependent lane depends on,
# and nothing ran it. Reading a skipped producer as a published image is what
# sent consumers to `docker pull` for a tag that was never pushed — closing a
# pull request starts a second docker.yml run for the same commit whose
# producers are all skipped, and being the newer run it is the one matched.
#
# The program is read out of the workflow rather than copied here, so the two
# cannot drift.
set -euo pipefail

cd "$(dirname "$0")/.."

WORKFLOW=.github/workflows/_wait-for-docker-image.yml
MERGE_JOB="Merge multi-arch manifest"
DARWIN_JOB="Build darwin/arm64 binary for e2e"

PROGRAM="$(
  ruby -ryaml -e '
    wf = YAML.load_file(ARGV[0])
    step = wf["jobs"]["wait"]["steps"].find { |s| s["id"] == "find-run" }
    abort("no find-run step in #{ARGV[0]}") unless step
    run = step["run"]
    i = run.index("awk -F")
    abort("no awk invocation; did the verdict move?") unless i
    open_quote = run.index("\x27\n", i)
    abort("the awk program does not start on its own line") unless open_quote
    body_start = open_quote + 2
    close = run.index("}\x27", body_start)
    abort("the awk program is not closed by }\x27") unless close
    print run[body_start..close]
  ' "$WORKFLOW"
)"
[ -n "$PROGRAM" ] || { echo "empty verdict program" >&2; exit 1; }

verdict() {
  printf '%s\n' "$1" | awk -F'\t' -v merge="$MERGE_JOB" -v darwin="$DARWIN_JOB" "$PROGRAM"
}

failed=0
check() {
  local name="$1" want="$2" jobs="$3"
  local got
  got="$(verdict "$jobs" | awk '{print $1}')"
  if [ "$got" != "$want" ]; then
    echo "FAIL $name: got '$got', expected '$want'" >&2
    failed=1
  else
    printf 'ok   %-46s -> %s\n' "$name" "$got"
  fi
}

jobs() { printf '%s\t%s\t%s\n%s\t%s\t%s' "$MERGE_JOB" "$1" "$2" "$DARWIN_JOB" "$3" "$4"; }

# The manifest says whether an image exists. The darwin binary is a separate
# artifact only the macOS e2e lane reads, and docker.yml skips it unless the
# diff touches frontend, e2e or ci — so a Go-only change legitimately produces
# a successful manifest beside a skipped binary.
check "image built, binary built"              ready   "$(jobs completed success completed success)"
check "image built, binary not needed"         ready   "$(jobs completed success completed skipped)"
check "nothing built at all"                   none    "$(jobs completed skipped completed skipped)"
check "no image, but a binary somehow"         none    "$(jobs completed skipped completed success)"
check "manifest cancelled"                     failed  "$(jobs completed cancelled completed success)"
check "manifest failed"                        failed  "$(jobs completed failure completed skipped)"
check "binary failed"                          failed  "$(jobs completed success completed failure)"
check "binary cancelled"                       failed  "$(jobs completed success completed cancelled)"
check "manifest still running"                 pending "$(jobs in_progress '' completed success)"
check "binary still running"                   pending "$(jobs completed success in_progress '')"
check "binary absent from the run"             pending "$(printf '%s\tcompleted\tsuccess' "$MERGE_JOB")"
check "manifest absent from the run"           pending "$(printf '%s\tcompleted\tsuccess' "$DARWIN_JOB")"
check "unrelated jobs are ignored"             pending "$(printf 'Scan image for vulnerabilities\tcompleted\tsuccess')"

exit "$failed"
