#!/usr/bin/env bash
# worktree-setup.sh — Make a fresh worktree ready for work. Safe to run
# again at any time: it only acts when something is missing or stale, and it
# prints one line when all is well.
#
# Usage:
#   .agents/skills/agent-workflow/scripts/worktree-setup.sh [--e2e] [--min-free-gib <n>]
#
#   --e2e               also install e2e/ dependencies when they are missing
#                       or stale; the Playwright browser check needs them
#   --min-free-gib <n>  refuse to run below <n> GiB free on the volume that
#                       holds the worktree (default $AGENT_MIN_FREE_GIB, then 15)
#
# What it checks, in order:
#   disk        free space on the worktree's volume
#   frontend    `npm ci --prefer-offline` when frontend/node_modules is missing
#               or was installed from another package-lock.json, Node major or
#               platform (a stamp in node_modules records what was installed)
#   e2e         the same for e2e/, only with --e2e
#   playwright  the browsers the installed @playwright/test needs are in the
#               shared cache; prints the install command when one is missing.
#               It never installs browsers and never deletes anything from the
#               cache, which other worktrees share.
#   go          `go mod download` when a module is missing from the cache
#
# Run it from the worktree. It refuses the main checkout (AGENT_ALLOW_MAIN=1
# overrides) and a node_modules that is a symlink, because npm ci there would
# wipe dependencies another checkout uses.

set -euo pipefail

usage() {
  sed -n '2,/^$/s/^# \{0,1\}//p' "$0"
}

with_e2e=no
min_free_gib="${AGENT_MIN_FREE_GIB:-15}"
while [ $# -gt 0 ]; do
  case "$1" in
    --e2e) with_e2e=yes; shift ;;
    --min-free-gib) min_free_gib="${2:?--min-free-gib needs a number}"; shift 2 ;;
    --min-free-gib=*) min_free_gib="${1#--min-free-gib=}"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "worktree-setup: unknown argument '$1'" >&2; usage >&2; exit 2 ;;
  esac
done
case "$min_free_gib" in
  '' | *[!0-9]*) echo "worktree-setup: --min-free-gib takes a whole number" >&2; exit 2 ;;
esac

here="$(cd "$(dirname "$0")" && pwd -P)"
# shellcheck source=SCRIPTDIR/lib.sh
. "$here/lib.sh"
root="$(agent_root worktree-setup)" || exit 2
cd "$root"
did=()
warn=0
tmp="${TMPDIR:-/tmp}"

say() { echo "worktree-setup: $*"; }

# npm ci deletes node_modules before installing. Through a symlink it would
# empty the directory the link points to, which belongs to another checkout.
for dir in frontend e2e; do
  if [ -L "$dir/node_modules" ]; then
    say "$dir/node_modules is a symlink (to $(readlink "$dir/node_modules")); npm ci would empty its target."
    say "Remove the link yourself (rm $dir/node_modules, not rm -r) and run this again."
    exit 2
  fi
done

# --- Disk -------------------------------------------------------------------
# POSIX df output: the fourth column is the space available, in KiB.
free_kib="$(df -Pk "$root" | awk 'NR == 2 { print $4 }')"
free_gib=$((free_kib / 1024 / 1024))
if [ "$free_gib" -lt "$min_free_gib" ]; then
  mount="$(df -Pk "$root" | awk 'NR == 2 { print $6 }')"
  say "only ${free_gib} GiB free on ${mount}; ${min_free_gib} GiB required."
  say "Free space first (remove your own finished worktrees: git worktree list),"
  say "or lower the bar with --min-free-gib / AGENT_MIN_FREE_GIB if you know the task is small."
  say "Killed go commands leave go-build* directories in ${TMPDIR:-/tmp}; report them rather than"
  say "deleting them, since a running build of another agent may own one."
  say "Do not delete shared caches (Playwright browsers, Go, npm) or other agents' worktrees."
  exit 1
fi

# --- npm dependencies -------------------------------------------------------
# The stamp names the lock file contents, the Node major and the platform:
# a change in any of them can leave node_modules wrong without npm noticing
# (native binaries, a lock bumped on master after the copy was made).
npm_stamp() {
  local dir=$1 node_major
  node_major="$(node -p 'process.versions.node.split(".")[0]')"
  echo "lock=$(git hash-object "$dir/package-lock.json") node=$node_major os=$(uname -sm)"
}

npm_install() {
  local dir=$1 stamp_file="$1/node_modules/.agent-setup-stamp" want log
  if ! command -v npm >/dev/null 2>&1; then
    say "npm is not installed; cannot set up $dir/"
    exit 1
  fi
  want="$(npm_stamp "$dir")"
  if [ -f "$stamp_file" ] && [ "$(cat "$stamp_file")" = "$want" ]; then
    return 0
  fi
  log="$(mktemp "${tmp%/}/agent-setup-npm.XXXXXX")"
  if ! (cd "$dir" && npm ci --prefer-offline --no-audit --no-fund --loglevel=error) >"$log" 2>&1; then
    say "npm ci failed in $dir/:"
    tail -n 30 "$log" | sed 's/^/    /'
    say "full log: $log"
    exit 1
  fi
  rm -f "$log"
  echo "$want" >"$stamp_file"
  did+=("npm ci in $dir/")
}

npm_install frontend
if [ "$with_e2e" = yes ]; then npm_install e2e; fi

# --- Playwright browsers ----------------------------------------------------
pw_manifest=e2e/node_modules/playwright-core/browsers.json
if [ -f "$pw_manifest" ]; then
  want_pw="$(node -p 'const p = require("./e2e/package.json"); ({ ...p.dependencies, ...p.devDependencies })["@playwright/test"] || ""')"
  have_pw="$(node -p 'require("./e2e/node_modules/playwright-core/package.json").version')"
  if [ "$want_pw" != "$have_pw" ]; then
    say "e2e/node_modules has Playwright $have_pw but e2e/package.json pins $want_pw; rerun with --e2e"
    warn=1
  else
    if [ -n "${PLAYWRIGHT_BROWSERS_PATH:-}" ] && [ "$PLAYWRIGHT_BROWSERS_PATH" != 0 ]; then
      cache="$PLAYWRIGHT_BROWSERS_PATH"
    elif [ "${PLAYWRIGHT_BROWSERS_PATH:-}" = 0 ]; then
      cache="e2e/node_modules/playwright-core/.local-browsers"
    elif [ "$(uname -s)" = Darwin ]; then
      cache="$HOME/Library/Caches/ms-playwright"
    else
      cache="${XDG_CACHE_HOME:-$HOME/.cache}/ms-playwright"
    fi
    # One line per browser the tests use: the accepted directory names,
    # space-separated (a platform override can change WebKit's revision).
    missing=()
    while IFS=' ' read -r name dirs; do
      found=no
      for d in $dirs; do
        if [ -d "$cache/$d" ]; then found=yes; break; fi
      done
      if [ "$found" = no ]; then missing+=("$name"); fi
    done < <(node -e '
      const m = require(process.argv[1]);
      const used = ["chromium", "chromium-headless-shell", "firefox", "webkit", "ffmpeg"];
      for (const b of m.browsers) {
        if (!used.includes(b.name)) continue;
        const revs = [b.revision, ...Object.values(b.revisionOverrides || {})];
        const dir = b.name.replace(/-/g, "_");
        console.log(b.name + " " + [...new Set(revs)].map((r) => dir + "-" + r).join(" "));
      }' "$root/$pw_manifest")
    if [ "${#missing[@]}" -gt 0 ]; then
      # `playwright install chromium` also brings the headless shell and ffmpeg.
      install=""
      for b in "${missing[@]}"; do
        case "$b" in chromium-headless-shell | ffmpeg) b=chromium ;; esac
        case " $install " in *" $b "*) ;; *) install="$install $b" ;; esac
      done
      say "Playwright $have_pw browsers missing from $cache: ${missing[*]}"
      say "install them with: (cd e2e && npx playwright install$install)"
      warn=1
    fi
  fi
elif [ "$with_e2e" = yes ]; then
  say "e2e/node_modules has no playwright-core; cannot check the browsers"
  warn=1
fi

# --- Go modules -------------------------------------------------------------
if command -v go >/dev/null 2>&1; then
  # With the proxy off, download succeeds only from the local module cache.
  if ! (cd go && GOPROXY=off GOFLAGS=-mod=mod go mod download) >/dev/null 2>&1; then
    log="$(mktemp "${tmp%/}/agent-setup-go.XXXXXX")"
    if ! (cd go && go mod download) >"$log" 2>&1; then
      say "go mod download failed:"
      tail -n 30 "$log" | sed 's/^/    /'
      exit 1
    fi
    rm -f "$log"
    did+=("go mod download")
  fi
else
  say "go is not installed; skipped the Go module check"
  warn=1
fi

summary="${free_gib} GiB free"
if [ "${#did[@]}" -gt 0 ]; then
  summary="$summary; ran $(printf '%s, ' "${did[@]}" | sed 's/, $//')"
fi
if [ "$warn" -eq 0 ]; then
  say "ready ($summary)"
else
  say "done with warnings above ($summary)"
  if [ "$with_e2e" = yes ]; then exit 1; fi
fi
