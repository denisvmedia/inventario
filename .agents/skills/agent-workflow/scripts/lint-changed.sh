#!/usr/bin/env bash
# lint-changed.sh — Lint only what a branch changes. For use while iterating;
# the full chain (make lint-go, eslint ., typecheck) still runs once before a
# push.
#
# Usage:
#   .agents/skills/agent-workflow/scripts/lint-changed.sh [--base <ref>]
#
# The base defaults to $CHANGED_BASE, then origin/master. Committed, staged,
# unstaged and untracked changes all count (see changed-files.sh).
#
# Steps, each run only when a matching file changed:
#   go        nolintguard, qtlint and golangci-lint on the touched packages
#             (the three linters of `make lint-go`)
#   frontend  eslint and prettier --check on the changed files; tsc -b when a
#             .ts/.tsx file changed (tsc cannot be scoped to files)
#   markdown  markdownlint-cli2 on the changed files, with the repo config
#   actions   actionlint on the changed workflows, at the CI severity
#
# Prints one line per step and the head of the output of a failed one. Exits
# non-zero when any step fails or its tool is missing. Set LOG_LINES to show
# more of a failed step's output; the full log path is printed.

set -euo pipefail

usage() {
  sed -n '2,/^$/s/^# \{0,1\}//p' "$0"
}

base="${CHANGED_BASE:-origin/master}"
while [ $# -gt 0 ]; do
  case "$1" in
    --base) base="${2:?--base needs a ref}"; shift 2 ;;
    --base=*) base="${1#--base=}"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "lint-changed: unknown argument '$1'" >&2; usage >&2; exit 2 ;;
  esac
done

here="$(cd "$(dirname "$0")" && pwd -P)"
# The repository is the one the caller stands in, so the scripts work from
# any checkout or worktree, also when called through the .claude/skills
# symlink or from another checkout's copy.
root="$(git rev-parse --show-toplevel 2>/dev/null)" || {
  echo "lint-changed: run it from inside a checkout of the repository" >&2
  exit 2
}
root="$(cd "$root" && pwd -P)"
cd "$root"
LOG_LINES="${LOG_LINES:-60}"
tmp="${TMPDIR:-/tmp}"
logdir="$(mktemp -d "${tmp%/}/lint-changed.XXXXXX")"
changed="$logdir/changed.txt"
failed=()
step_no=0

cleanup() {
  if [ "${#failed[@]}" -eq 0 ]; then rm -rf "$logdir"; fi
}
trap cleanup EXIT

"$here/changed-files.sh" "$base" >"$changed"
echo "lint-changed: $(wc -l <"$changed" | tr -d ' ') changed files vs $base"

# in_dir runs a command in a directory without changing ours.
in_dir() {
  local dir=$1
  shift
  (cd "$dir" && "$@")
}

show_log() {
  local log=$1 total
  total="$(wc -l <"$log" | tr -d ' ')"
  head -n "$LOG_LINES" "$log" | sed 's/^/    /'
  if [ "$total" -gt "$LOG_LINES" ]; then
    echo "    ... $((total - LOG_LINES)) more lines in $log"
  fi
}

# run_step NAME COMMAND...: run, time and report one step.
run_step() {
  local name=$1 start log
  shift
  step_no=$((step_no + 1))
  log="$logdir/step-$step_no.log"
  start=$SECONDS
  if "$@" >"$log" 2>&1; then
    printf 'ok    %s (%ss)\n' "$name" "$((SECONDS - start))"
  else
    printf 'FAIL  %s (%ss)\n' "$name" "$((SECONDS - start))"
    show_log "$log"
    failed+=("$name")
  fi
}

missing_tool() {
  printf 'FAIL  %s: %s\n' "$1" "$2"
  failed+=("$1")
}

# select_files REGEX [PREFIX]: changed files that exist and match REGEX,
# with PREFIX stripped.
select_files() {
  local regex=$1 prefix=${2:-} f
  { grep -E "$regex" "$changed" || true; } | while IFS= read -r f; do
    if [ -f "$f" ]; then echo "${f#"$prefix"}"; fi
  done
}

# --- Go ---------------------------------------------------------------------
# Package directories, relative to go/, of the changed Go files the go tool
# builds (it skips testdata/ and directories starting with _ or .). A
# function, because bash 3.2 misparses a case statement inside $(...).
touched_go_pkgs() {
  local f d
  select_files '^go/.+\.go$' | while IFS= read -r f; do
    case "/$f" in
      */testdata/* | */_* | */.*) continue ;;
    esac
    d="$(dirname "$f")"
    d="${d#go}"
    echo ".$d"
  done | sort -u
}
go_pkgs=()
while IFS= read -r p; do go_pkgs+=("$p"); done < <(touched_go_pkgs)
if [ "${#go_pkgs[@]}" -gt 0 ]; then
  n="${#go_pkgs[@]} pkgs"
  if command -v go >/dev/null 2>&1; then
    run_step "go: nolintguard ($n)" in_dir go go tool nolintguard "${go_pkgs[@]}"
    run_step "go: qtlint ($n)" in_dir go go tool qtlint "${go_pkgs[@]}"
  else
    missing_tool "go: nolintguard, qtlint" "go is not installed"
  fi
  if command -v golangci-lint >/dev/null 2>&1; then
    run_step "go: golangci-lint ($n)" in_dir go golangci-lint run "${go_pkgs[@]}"
  else
    missing_tool "go: golangci-lint" "not installed (see .github/workflows/go-lint.yml for the pinned version)"
  fi
fi

# --- Frontend ---------------------------------------------------------------
fe_lint=()
while IFS= read -r f; do fe_lint+=("$f"); done < <(select_files '^frontend/.+\.(ts|tsx|js|jsx|mjs|cjs)$' frontend/)
# Same glob as `npm run format:check`, which CI runs.
fe_format=()
while IFS= read -r f; do fe_format+=("$f"); done < <(select_files '^frontend/src/.+\.(ts|tsx|css|json|md)$' frontend/)
# Deleted files count here: removing a module can break its importers.
fe_types=no
if grep -Eq '^frontend/(.+\.(ts|tsx)|tsconfig[^/]*\.json)$' "$changed"; then fe_types=yes; fi

if [ "${#fe_lint[@]}" -gt 0 ] || [ "${#fe_format[@]}" -gt 0 ] || [ "$fe_types" = yes ]; then
  if [ ! -x frontend/node_modules/.bin/eslint ]; then
    missing_tool "frontend" "frontend/node_modules is missing; run .agents/skills/agent-workflow/scripts/worktree-setup.sh"
  else
    if [ "${#fe_lint[@]}" -gt 0 ]; then
      run_step "frontend: eslint (${#fe_lint[@]} files)" \
        in_dir frontend node_modules/.bin/eslint --no-warn-ignored "${fe_lint[@]}"
    fi
    if [ "${#fe_format[@]}" -gt 0 ]; then
      run_step "frontend: prettier --check (${#fe_format[@]} files)" \
        in_dir frontend node_modules/.bin/prettier --check "${fe_format[@]}"
    fi
    if [ "$fe_types" = yes ]; then
      run_step "frontend: tsc -b" in_dir frontend node_modules/.bin/tsc -b --noEmit
    fi
  fi
fi

# --- Markdown ---------------------------------------------------------------
md=()
while IFS= read -r f; do md+=("$f"); done < <(select_files '\.md$')
if [ "${#md[@]}" -gt 0 ]; then
  if command -v markdownlint-cli2 >/dev/null 2>&1; then
    # From the root, so .markdownlint-cli2.jsonc applies with its ignores;
    # --no-globs drops the config's **/*.md, which would lint the whole repo.
    run_step "markdown: markdownlint-cli2 (${#md[@]} files)" markdownlint-cli2 --no-globs "${md[@]}"
  else
    missing_tool "markdown" "markdownlint-cli2 is not installed (brew install markdownlint-cli2)"
  fi
fi

# --- Workflows --------------------------------------------------------------
wf=()
while IFS= read -r f; do wf+=("$f"); done < <(select_files '^\.github/workflows/[^/]+\.ya?ml$')
if [ "${#wf[@]}" -gt 0 ]; then
  if command -v actionlint >/dev/null 2>&1; then
    # Same severity as the ci-lint workflow.
    run_step "actions: actionlint (${#wf[@]} files)" env SHELLCHECK_OPTS='-S warning' actionlint "${wf[@]}"
  else
    missing_tool "actions" "actionlint is not installed"
  fi
fi

if [ "$step_no" -eq 0 ] && [ "${#failed[@]}" -eq 0 ]; then
  echo "lint-changed: nothing to lint"
fi
if [ "${#failed[@]}" -gt 0 ]; then
  echo "lint-changed: failed: $(printf '%s; ' "${failed[@]}")logs in $logdir"
  exit 1
fi
