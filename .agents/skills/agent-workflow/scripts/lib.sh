# shellcheck shell=bash
# lib.sh — Shared by the agent-workflow scripts. Sourced, not run.

# agent_root NAME: print the root of the checkout the caller stands in.
#
# The scripts act on the caller's checkout, not on the one they are stored in,
# so one copy serves every worktree. That makes the current directory decide
# what is linted, tested or reinstalled, and an agent whose shell went back to
# the main checkout would act there: lint nothing and report a pass, or run
# npm ci over the main checkout's node_modules. So the main checkout is
# refused unless AGENT_ALLOW_MAIN=1. In a linked worktree .git is a file; in
# the main checkout it is a directory.
agent_root() {
  local name=$1 root
  root="$(git rev-parse --show-toplevel 2>/dev/null)" || {
    echo "$name: run it from inside a checkout of the repository" >&2
    return 2
  }
  root="$(cd "$root" && pwd -P)"
  if [ -d "$root/.git" ] && [ "${AGENT_ALLOW_MAIN:-}" != 1 ]; then
    echo "$name: $root is the main checkout, not a worktree." >&2
    echo "$name: cd into your worktree in the same command, for example:" >&2
    echo "$name:   cd .claude/worktrees/<name> && .agents/skills/agent-workflow/scripts/$name.sh" >&2
    echo "$name: AGENT_ALLOW_MAIN=1 runs it on the main checkout anyway." >&2
    return 2
  fi
  echo "$root"
}
