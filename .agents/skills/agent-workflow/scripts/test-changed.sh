#!/usr/bin/env bash
# test-changed.sh — Run only the tests a branch's changes can affect. For use
# while iterating; before a push run the tests of the touched areas once more
# with the full chain.
#
# Usage:
#   .agents/skills/agent-workflow/scripts/test-changed.sh [--base <ref>] [--race] [--pg] [--list]
#       [-- <go test args>]
#
#   --base <ref>  compare against <ref> (default $CHANGED_BASE, then
#                 origin/master); committed, staged, unstaged and untracked
#                 changes all count
#   --race        add -race to go test
#   --pg          run the postgres registry tests even when nothing under
#                 go/registry/postgres/ or go/registry/registrytest/ changed
#   --list        print what would run, run nothing
#   -- <args>     passed to every go test run, for example -- -run TestSeller
#
# Go: tests the packages that hold a changed file (a non-Go file counts for
# the nearest package above it, so a migration under _sqldata/ selects the
# migrations package) plus the packages that import one of them directly,
# test imports included.
#
# Postgres: the packages under go/registry/postgres/ run only when that tree
# or the shared registrytest contract changed (or with --pg), and only when
# POSTGRES_TEST_DSN is set or go/internal/pgtest has a cached embedded server
# under ~/.embedded-postgres-go. Otherwise the script says what it skipped.
#
# Frontend: vitest related --run on the changed files under frontend/src/.

set -euo pipefail

usage() {
  sed -n '2,/^$/s/^# \{0,1\}//p' "$0"
}

base="${CHANGED_BASE:-origin/master}"
race=no
force_pg=no
list=no
go_args=()
while [ $# -gt 0 ]; do
  case "$1" in
    --base) base="${2:?--base needs a ref}"; shift 2 ;;
    --base=*) base="${1#--base=}"; shift ;;
    --race) race=yes; shift ;;
    --pg) force_pg=yes; shift ;;
    --list) list=yes; shift ;;
    --) shift; go_args=("$@"); break ;;
    -h|--help) usage; exit 0 ;;
    *) echo "test-changed: unknown argument '$1'" >&2; usage >&2; exit 2 ;;
  esac
done

here="$(cd "$(dirname "$0")" && pwd -P)"
# The repository is the one the caller stands in, so the scripts work from
# any checkout or worktree, also when called through the .claude/skills
# symlink or from another checkout's copy.
root="$(git rev-parse --show-toplevel 2>/dev/null)" || {
  echo "test-changed: run it from inside a checkout of the repository" >&2
  exit 2
}
root="$(cd "$root" && pwd -P)"
cd "$root"
LOG_LINES="${LOG_LINES:-150}"
tmp="${TMPDIR:-/tmp}"
logdir="$(mktemp -d "${tmp%/}/test-changed.XXXXXX")"
changed="$logdir/changed.txt"
failed=()
ran=0

cleanup() {
  if [ "${#failed[@]}" -eq 0 ]; then rm -rf "$logdir"; fi
}
trap cleanup EXIT

"$here/changed-files.sh" "$base" >"$changed"
echo "test-changed: $(wc -l <"$changed" | tr -d ' ') changed files vs $base"

# show_log LOG [DROP_REGEX]: print the head of a log, minus noise lines.
show_log() {
  local log=$1 drop=${2:-} shown="$1.shown" total
  if [ -n "$drop" ]; then
    grep -Ev "$drop" "$log" >"$shown" || true
  else
    cp "$log" "$shown"
  fi
  total="$(wc -l <"$shown" | tr -d ' ')"
  head -n "$LOG_LINES" "$shown" | sed 's/^/    /'
  if [ "$total" -gt "$LOG_LINES" ]; then
    echo "    ... $((total - LOG_LINES)) more lines in $log"
  fi
}

# run_step NAME DROP_REGEX SUMMARY_REGEX COMMAND...
run_step() {
  local name=$1 drop=$2 summary=$3 start log extra
  shift 3
  ran=$((ran + 1))
  if [ "$list" = yes ]; then
    echo "would run  $name:"
    shift 2 # in_dir <dir>
    echo "    $*"
    return 0
  fi
  log="$logdir/step-$ran.log"
  start=$SECONDS
  if "$@" >"$log" 2>&1; then
    extra=""
    if [ -n "$summary" ]; then
      extra="$(grep -E "$summary" "$log" | tr -s ' ' | tr '\n' ' ' | sed 's/ *$//' || true)"
    fi
    printf 'ok    %s (%ss)%s\n' "$name" "$((SECONDS - start))" "${extra:+ $extra}"
  else
    printf 'FAIL  %s (%ss)\n' "$name" "$((SECONDS - start))"
    show_log "$log" "$drop"
    failed+=("$name")
  fi
}

in_dir() {
  local dir=$1
  shift
  (cd "$dir" && "$@")
}

# --- Go ---------------------------------------------------------------------
GO_NOISE='^(ok  |\?   )'

# Directory of each changed path under go/, moved up to the nearest directory
# that holds Go files. The go tool ignores testdata/ and directories starting
# with _ or ., so those, like deleted directories, belong to the package
# above them.
go_dirs="$logdir/go-dirs.txt"
{ grep -E '^go/' "$changed" || true; } | while IFS= read -r f; do
  case "$f" in go/go.mod | go/go.sum) continue ;; esac
  d="$(dirname "$f" | sed -E 's#/(testdata|_[^/]*|\.[^/]*)(/.*)?$##')"
  while [ "$d" != go ] && [ "$d" != . ]; do
    set -- "$d"/*.go
    if [ -e "$1" ]; then break; fi
    d="$(dirname "$d")"
  done
  if [ "$d" = go ]; then
    set -- go/*.go
    [ -e "$1" ] || continue
  fi
  [ "$d" = . ] && continue
  echo "$root/$d"
done | sort -u >"$go_dirs"

if grep -Eq '^go/go\.(mod|sum)$' "$changed"; then
  echo "note  go.mod/go.sum changed: run the whole Go suite before the push"
fi

if [ -s "$go_dirs" ]; then
  if ! command -v go >/dev/null 2>&1; then
    printf 'FAIL  go: go is not installed\n'
    failed+=("go")
  else
    # One `go list` gives every package's directory, import path and direct
    # imports (test imports included); awk keeps the changed packages and
    # their direct importers.
    pkgs="$logdir/go-pkgs.txt"
    (cd go && go list -e -f '{{.Dir}}|{{.ImportPath}}|{{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...) |
      awk -F'|' -v dirs="$go_dirs" '
        BEGIN { while ((getline d < dirs) > 0) want[d] = 1 }
        {
          n++; path[n] = $2; imports[n] = $3
          if ($1 in want) changed[$2] = 1
        }
        END {
          for (i = 1; i <= n; i++) {
            if (path[i] in changed) { print path[i]; continue }
            m = split(imports[i], list, " ")
            for (j = 1; j <= m; j++) if (list[j] in changed) { print path[i]; break }
          }
        }' | sort -u >"$pkgs"

    module="$(cd go && go list -m)"
    pg_prefix="$module/registry/postgres"
    pg_changed=no
    if grep -Eq '^go/registry/(postgres|registrytest)/' "$changed"; then pg_changed=yes; fi

    plain=()
    pg=()
    while IFS= read -r p; do
      case "$p" in
        "$pg_prefix" | "$pg_prefix"/*) pg+=("$p") ;;
        *) plain+=("$p") ;;
      esac
    done <"$pkgs"
    if [ "$force_pg" = yes ] && [ "${#pg[@]}" -eq 0 ]; then
      pg=("$pg_prefix/...")
    fi

    test_flags=()
    if [ "$race" = yes ]; then test_flags+=(-race); fi
    if [ "${#go_args[@]}" -gt 0 ]; then test_flags+=("${go_args[@]}"); fi

    if [ "${#plain[@]}" -gt 0 ]; then
      run_step "go test (${#plain[@]} pkgs)" "$GO_NOISE" "" \
        in_dir go go test ${test_flags[@]+"${test_flags[@]}"} "${plain[@]}"
    fi

    if [ "${#pg[@]}" -gt 0 ]; then
      set -- "$HOME"/.embedded-postgres-go/embedded-postgres-binaries-*.txz
      if [ "$pg_changed" = no ] && [ "$force_pg" = no ]; then
        echo "skip  postgres registry tests: nothing under go/registry/postgres/ or registrytest/ changed (--pg runs them)"
      elif [ -n "${POSTGRES_TEST_DSN:-}" ]; then
        # One shared database: the packages must not run in parallel.
        run_step "go test postgres (${#pg[@]} pkgs, POSTGRES_TEST_DSN)" "$GO_NOISE" "" \
          in_dir go go test -p 1 ${test_flags[@]+"${test_flags[@]}"} "${pg[@]}"
      elif [ -e "$1" ] || [ "$force_pg" = yes ]; then
        run_step "go test postgres (${#pg[@]} pkgs, embedded server)" "$GO_NOISE" "" \
          in_dir go go test ${test_flags[@]+"${test_flags[@]}"} "${pg[@]}"
      else
        echo "skip  postgres registry tests: POSTGRES_TEST_DSN is unset and no embedded server is cached (--pg downloads one)"
      fi
    fi
  fi
fi

# --- Frontend ---------------------------------------------------------------
fe=()
while IFS= read -r f; do
  if [ -f "$f" ]; then fe+=("${f#frontend/}"); fi
done < <(grep -E '^frontend/src/.+\.(ts|tsx|js|jsx|css|json)$' "$changed" || true)

if [ "${#fe[@]}" -gt 0 ]; then
  if [ ! -x frontend/node_modules/.bin/vitest ]; then
    printf 'FAIL  frontend: frontend/node_modules is missing; run .agents/skills/agent-workflow/scripts/worktree-setup.sh\n'
    failed+=("frontend")
  else
    run_step "vitest related (${#fe[@]} files)" "" '^ +(Test Files|Tests) ' \
      in_dir frontend node_modules/.bin/vitest related --run --passWithNoTests "${fe[@]}"
  fi
fi

if [ "$ran" -eq 0 ] && [ "${#failed[@]}" -eq 0 ]; then
  echo "test-changed: nothing to test"
fi
if [ "${#failed[@]}" -gt 0 ]; then
  echo "test-changed: failed: $(printf '%s; ' "${failed[@]}")logs in $logdir"
  exit 1
fi
