---
name: frontend-work
description: Orchestrate any task that touches `frontend/` (React 19 + Tailwind v4 + shadcn/ui). Activates whenever the agent is about to read, modify, add, or design files under `frontend/src/`, including new pages, new components, layout/styling work, sidebar/app-shell changes, route additions, i18n key edits, theme/density work, and form/data work that has a visual surface. Enforces design fidelity against `design-mocks/` (read-only mirror of upstream `inventario-design`), points at `devdocs/frontend/` as the operating manual, requires logging deviations in `devdocs/frontend/design-deviations.md`, and offers post-change visual review plus optional Issue-comment publication via the `screenshot-review` skill and `e2e/push-screenshots.sh`. Skip for backend-only changes (`go/`), type-only generation (`src/types/api.d.ts`), pure docs changes, or test-only edits.
---

# Frontend work

## What this skill does

Wraps every task that touches `frontend/` with three obligations:

1. **Pre-flight.** Read the operating manual, find the matching mock, then code.
2. **Design fidelity.** Default 1:1 with `design-mocks/`. Any divergence is an explicit, user-approved decision and gets logged.
3. **Post-flight.** After visible changes, offer a screenshot pass — and, on explicit user request, publish the captures to an `assets/screenshots-<issue>` branch and embed them as an Issue comment.

The skill orchestrates these stages. The actual capture/review mechanics live in [`screenshot-review`](../screenshot-review/SKILL.md); this skill just makes sure that flow is offered at the right moment.

## When this activates

Activate when the task involves:

- Building, refactoring, or styling pages or components under `frontend/src/`.
- Adding new routes or modifying route guards (`frontend/src/app/`).
- Touching feature slices (`frontend/src/features/<x>/`) where a UI surface is affected.
- Editing the app shell, sidebar, navigation, theme, density, dark mode.
- Adding/removing/renaming i18n keys that surface visually (`frontend/src/i18n/locales/**`).
- Importing a new shadcn/ui primitive or component-library piece.

**Skip** when:

- Changes are confined to `go/`, migrations, CI, or scripts.
- Only `src/types/api.d.ts` (typegen output) drifts.
- Only tests change (`*.test.ts(x)`, `e2e/`, MSW handlers, jest-axe specs) without a visual surface change.
- Pure docs work (`devdocs/`, `README.md`).

If unsure, lean toward activating — the cost is reading two files.

## Step 1 — Pre-flight (mandatory before editing `frontend/src/`)

1. **Read [`devdocs/frontend/README.md`](../../../devdocs/frontend/README.md).** It indexes 17 docs (coding standards, components, styles & tokens, forms, icons, accessibility, data, routing, i18n, imports & bans, testing, perf, screenshots, migration history, PR checklist, design-language brief, and `design-deviations.md`). Open the doc that matches the task. Don't skip this even if you've worked on `frontend/` before — the docs change.

2. **Locate the mock for the surface you're touching.** The mock lives at `design-mocks/` (repo root). Two cases:

   - **Surface exists in the mock.** Find it under `design-mocks/src/views/` (pages — Dashboard, Items, Warranties, Tags, Members, Locations, Settings, etc.) or `design-mocks/src/components/` (shared components). Open it. Read it. That's your contract.
   - **Surface is missing or partial in the mock.** Fall back to [`design-mocks/src/views/UIShowcaseView.tsx`](../../../design-mocks/src/views/UIShowcaseView.tsx) — a 1379-line catalog of every primitive (Buttons, Badges, Alerts, Cards, Tabs, Forms, Menus, Tables, Charts, Typography, Color tokens, Empty states, etc.). Pick the closest pattern. Treat the gap itself as a deviation that must be logged (see Step 3).

3. **Read [`devdocs/frontend/design-deviations.md`](../../../devdocs/frontend/design-deviations.md)** for any prior decisions about the surface you're about to touch — they may already constrain or pre-approve part of what you're planning.

## Step 2 — `design-mocks/` is read-only (no exceptions)

Do not edit, create, delete, or move any file under `design-mocks/`. It is a mirror of `github.com/denisvmedia/inventario-design` synchronized by an external tool — local edits are wiped on the next sync, so they are forbidden.

- Don't include `design-mocks/` paths in any `Edit`/`Write` call.
- Don't `git add` files under `design-mocks/` that you wrote.
- If a refactor (rename, codemod, search-and-replace) would touch `design-mocks/`, scope it to `frontend/` only.
- If you spot a real bug *in the mock itself*, surface it to the user verbally — it gets fixed upstream, not here.

If you catch yourself about to write under `design-mocks/`, **stop and ask the user**. There is no scenario in which the right answer is "edit it anyway."

## Step 3 — Design fidelity = 1:1 by default

Match the mock exactly: layout, spacing, copy structure, color tokens, component composition, interaction patterns. Use OKLCH tokens, no `forwardRef`, no `hsl()` wrappers, no `@tailwindcss/animate` — the rules from `design-mocks/CLAUDE.md` are mirrored in [`devdocs/frontend/styles-and-tokens.md`](../../../devdocs/frontend/styles-and-tokens.md) and [`devdocs/frontend/components.md`](../../../devdocs/frontend/components.md).

For the actual replication — the paste-ready snippets, the surface-index that maps "what you're building" to "which mock file is canonical", the token cheatsheet, and the drift markers — use [`design-mock-fidelity`](../design-mock-fidelity/SKILL.md). It's the dense playbook for hitting 1:1 without re-deriving spacing or component anatomy from scratch.

When you must diverge:

- **Agent-initiated deviation.** Stop. Surface to the user *before* implementing: name the deviation, explain the technical reason, propose alternatives, wait for explicit approval.
- **User-initiated deviation.** Accept it, but first explain the consequences: visual drift from the mock, future review/maintenance friction, possible re-litigation when the mock updates. Confirm the user understands the tradeoff before coding.

**Every accepted deviation gets logged.** Append an entry to the right section of [`devdocs/frontend/design-deviations.md`](../../../devdocs/frontend/design-deviations.md) using the entry format documented at the top of that file (date, Issue/PR, Mock, Reality, Why, Approved by, Reversion plan). The change is not finished until the entry exists.

When the mock is silent on a surface, log the gap as `Why: not present in mock` plus the showcase pattern you fell back to.

## Step 4 — Post-flight: offer screenshots

After landing visible changes, **offer** a screenshot pass — never run one without an explicit user "yes". Use the offer phrasing from [`screenshot-review`](../screenshot-review/SKILL.md) (one short line, naming the surfaces touched). The mechanics live there; this skill just makes sure the offer happens.

If the user accepts and the captures look right, optionally offer a second step: **publish the captures so they can be embedded in the Issue comment.**

Phrasing for the publish offer:

> "Want me to push these to `assets/screenshots-<NNNN>` and post a preview comment on Issue #NNNN?"

If yes, the flow is:

1. Run `e2e/push-screenshots.sh <issue-number> [src-label] [glob...]` via Bash. The script creates/updates the `assets/screenshots-<issue-number>` branch from `origin/master`, commits the PNGs, pushes, and prints a commit-pinned raw URL prefix (`https://raw.githubusercontent.com/denisvmedia/inventario/<sha>/assets/screenshots-<issue-number>/<file>.png`).
2. Draft the Issue comment body — short context line plus inline `<img>` tags using the printed URL prefix.
3. Confirm the body with the user.
4. Post via `mcp__github_and_git__add_issue_comment` on the same Issue.

Each step needs its own "yes". Never collapse capture + push + comment into a single approval.

These `assets/screenshots-<NNNN>` branches are intentionally throwaway — the maintainer prunes them once the visual review is settled. Don't worry about churn; do worry about pushing without consent.

## What this skill does NOT do

- It does not write or run tests — see `inventario-e2e` for Playwright, and `devdocs/frontend/testing.md` for Vitest/RTL.
- It does not run the screenshot script itself — that's `screenshot-review`.
- It does not modify backend code (`go/`), migrations, or infrastructure.
- It does not bypass the design-mock check just because the change feels small. A one-line padding tweak still warrants opening the mock.

## Cross-references

- [`AGENTS.md`](../../../AGENTS.md) — high-level rules including the read-only `design-mocks/` clause.
- [`devdocs/frontend/README.md`](../../../devdocs/frontend/README.md) — frontend operating manual, indexes everything.
- [`devdocs/frontend/design-deviations.md`](../../../devdocs/frontend/design-deviations.md) — append-only deviation log.
- [`devdocs/frontend/screenshots.md`](../../../devdocs/frontend/screenshots.md) — end-to-end screenshot workflow including `push-screenshots.sh` usage.
- [`design-mocks/src/views/UIShowcaseView.tsx`](../../../design-mocks/src/views/UIShowcaseView.tsx) — UI-primitive catalog, the fallback when the mock is silent.
- [`.claude/skills/design-mock-fidelity/SKILL.md`](../design-mock-fidelity/SKILL.md) — replication playbook with surface index, token reference, and paste-ready patterns.
- [`.claude/skills/screenshot-review/SKILL.md`](../screenshot-review/SKILL.md) — capture + review mechanics.
- [`.claude/skills/inventario-e2e/SKILL.md`](../inventario-e2e/SKILL.md) — Playwright e2e workflow.
- [`e2e/push-screenshots.sh`](../../../e2e/push-screenshots.sh) — publishes captures to `assets/screenshots-<label>`.
