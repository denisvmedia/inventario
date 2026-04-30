---
name: screenshot-review
description: Generate local screenshots of the Inventario React frontend via `e2e/screenshots-react.mjs` and review them visually for bugs (unresolved i18n keys, broken layout, wrong currency, truncated dates, dark-mode regressions, missing fallbacks, etc). On user request the workflow runs end-to-end. After any frontend change made by the agent in `frontend-react/`, the skill activates to OFFER this review explicitly to the user — never auto-runs. Strict rule: never produce screenshots without an explicit user "yes". The user owns the call because screenshots take time, need a running binary, and may surface unrelated bugs the user might not want to triage right now.
---

# Screenshot review (local)

## What this skill does

Two related jobs. Both end with a visual review of the rendered frontend; neither happens without explicit user consent.

1. **On user request** — run `e2e/screenshots-react.mjs` against a local running build, save the PNGs into `.research/screenshots/pr-NNN/` (uncommitted; see "Where do screenshots live"), then read each one and report what looks right vs. broken.
2. **After any frontend change you just made** — surface the offer to run #1, with a one-line description of what you'd cover. Wait for the user to say go.

You can act on #1. You cannot act on #2 — only offer.

## The hard rules

- **Never run a screenshot pass unsolicited.** If the user hasn't asked, propose it; then wait.
- **The user's last word wins.** A "skip it" earlier in the session means skip it for the rest of the session unless they reverse it.
- **No screenshots in the diff.** They live under `.research/screenshots/pr-NNN/` and that path is in the maintainer's global `.gitignore` — that's intentional. Don't `git add -f` past the ignore. Don't push them to a side branch. Don't embed them via raw URLs in PR comments. (The maintainer litigated this already; the canonical answer is "local-only".)
- **Don't substitute Playwright e2e specs for this.** `e2e/screenshots-react.mjs` is a separate Node script that boots the binary, logs in, and saves PNGs. The Playwright spec suite is unrelated — see the `inventario-e2e` skill for that.

## When to OFFER (don't decide for the user)

Offer after you finish any of these in `frontend-react/`:

- New page or new route
- New component visible on a real page
- Sidebar / app-shell change (nav items, layout, theme, density)
- i18n key additions or renames (a missed key shows up as `namespace:key` in the UI — exactly the kind of thing screenshots catch)
- CSS / Tailwind / design-token changes that could affect spacing, contrast, dark mode
- Router / redirect / route-guard change
- Anything labeled "polish", "fix the look", "should look right"

How to phrase the offer (one short line, no markdown headers, no bullet list for trivial changes):

> "Want me to run the screenshot script and check for visual regressions on \<surfaces touched\>? Takes ~30 sec once the binary is up."

If the user says yes, run job #1. If silence or "later", drop it — don't re-ask later in the session.

When to **skip** the offer entirely:

- Pure-test changes (vitest, jest-axe, Playwright spec) — the test suite is already verifying.
- Type-only changes / API typegen drift (`api.d.ts`) — no rendered UI delta.
- Backend-only changes (`go/`) the frontend doesn't even know about yet.
- Documentation-only changes.
- Legacy `frontend/` (Vue) work — the script is React-only; no equivalent exists for the legacy bundle.

If unsure, lean toward offering — the friction is one sentence and one user "no".

## How to run job #1 (the actual screenshot pass)

Walk this carefully — half the value is catching the user's hidden assumptions before the script burns an iteration.

### 1. Confirm the prerequisites are met

Ask the user (don't try to set these up yourself unless asked):

- Is the local binary already running with the React bundle?
  ```
  ./bin/inventario run --frontend-bundle=new --db-dsn=memory:// \
    --no-auth-rate-limit --no-global-rate-limit
  ```
  Header comment in `e2e/screenshots-react.mjs` is the source of truth.
- Is the database seeded?
  ```
  curl -X POST http://localhost:3333/api/v1/seed
  ```
- Is `frontend-react/` built into the embed?
  ```
  make build-frontend-react
  cd go/cmd/inventario && go build -tags with_frontend -o ../../../bin/inventario .
  ```

If any of these is "no", say so and stop — don't pivot to "I'll set it up". The user may have a reason their binary isn't running (other branch, different port, dev server vs binary).

### 2. Run the script

```
BASE_URL=http://localhost:3333 \
  OUT=.research/screenshots/pr-<NNN>/ \
  node e2e/screenshots-react.mjs
```

The `OUT` env var lets the script write directly into the canonical local-only folder — no second `cp` step. If the user has a different PR number convention or a different out-path preference, defer.

### 3. Read every PNG

Use the `Read` tool on each `.png` — Claude is multimodal, the image content goes into your context. Open them in **numeric order** (the script prefixes 01-, 02-, …) so you walk the user's flow as they would experience it.

For each one, ask yourself:

- **Unresolved i18n keys.** Do you see anything that looks like `namespace:key.path` rendered as literal text? That's a missing translation. Always flag.
- **Truncated text.** Dates shown as "January 1, 1" or numbers cut off mid-digit are formatter bugs.
- **Currency formatting.** Different surfaces (list, detail, print, sheet preview) should agree on currency split — purchase currency vs. group main currency. Compare across screenshots, not just within one.
- **Empty / placeholder states.** Does an empty state read like "No items" or like "[empty]" / "—" that escaped i18n?
- **Dark-mode contrast.** If the screenshot is dark, does any text disappear into background?
- **Layout breakage.** Overlapping, clipped, or pushed-off-screen elements. Sidebar items rendering in the main content area.
- **Hover / focus state stuck on.** A button that should be neutral stuck in pressed state.
- **Off-route content.** A page that's supposed to be group-scoped showing a broken sidebar; or vice versa.

### 4. Report — separate "from this PR" from "pre-existing"

Critical: the maintainer wants to act on bugs from **the change they just made**, not get a list of every UI imperfection in the codebase. Separate the two:

- **From this PR:** what your change introduced or made worse. Block on these unless trivial.
- **Pre-existing:** what was already broken before your change. Surface, don't block. Offer to file a tracking issue or add an extra to a related open issue (the maintainer prefers folding small polish items into the next active FE issue rather than spawning new ones for one-line fixes — see how follow-ups for #1410 ended up under #1411).

Mark surfaces that look correct, too. The maintainer reads the screenshots themselves once you've called out what to look at; if you only list bugs they have to scroll to verify nothing's wrong.

### 5. Don't push the screenshots

Verify with the `git_status` MCP tool that `.research/screenshots/pr-NNN/` is invisible — the maintainer's global gitignore on `.research/` keeps it untracked. If you see it staged or tracked anywhere, stop and undo before continuing.

## Where do screenshots live

`.research/screenshots/pr-NNN/` on the working branch, **uncommitted** because `.research/` is in the maintainer's global gitignore (it's their personal scratch tree). Three settings of this rule have been re-litigated already:

| Tried | Result |
|---|---|
| Separate `assets/screenshots-NNN` branch + raw URL embeds | Rejected. Don't do it. |
| `.research/screenshots/pr-NNN/` committed via `git add -f` past the ignore | Rejected. The whole point of `.research/` is local-only. |
| `.research/screenshots/pr-NNN/` left untracked | **Correct.** |

If the maintainer ever wants reviewers to see screenshots, they'll drag-drop into the PR description in the GitHub web UI themselves. Don't initiate that.

## Surfaces the script covers

`e2e/screenshots-react.mjs` walks (header comment is canonical — re-read it if it drifts):

- Unauth: `/login`, `/register`, `/forgot-password`, catch-all 404
- Group-scoped: `/g/:slug/` (dashboard), `/locations`, `/locations/new`, location detail, area detail
- Commodities (#1410): list, sheet preview, add dialog step 1, detail, print
- Personal: `/profile`, `/settings`

When you offer the pass, name the surfaces relevant to the change, not the full set. "Dashboard + locations" is more useful than "all 16".

## What this skill is NOT

- **Not** a way to autonomously verify the user's frontend changes. Visual review needs human input ("does that look right?") that you can't reliably substitute for.
- **Not** the right tool for first-time scaffolding work where there's nothing visually meaningful to compare against. Offer it once there's actually a page that renders.
- **Not** a replacement for accessibility / contrast tooling. Axe via Playwright is what catches a11y violations in CI (`@react-only` axe specs).
- **Not** pre-merge gating. Even a successful screenshot review doesn't block merge — that's CI's job.
- **Not** for the legacy Vue frontend. `frontend/` has no equivalent screenshot script and isn't getting one — the rewrite (#1397) is approaching cutover.

## Phrasing examples

**After landing a new page (offer-don't-do):**
> "Want me to run the screenshot script and eyeball the new commodity detail page + print view? Takes ~30 sec once the binary is running."

**On explicit user request:**
> "On it — running against `http://localhost:3333` (override with BASE_URL). Reading PNGs as they land."

**Reporting findings:**
> "**From this PR:** detail page tabs render correctly; print page hides chrome as intended; currency split is right.
> **Pre-existing (not blocking):** sidebar's Personal section shows literal `common:nav.preferences` (came from #1414, not us). Profile date reads 'Member since January 1, 1' — date formatter dropping the year.
> Want me to add those two as extras to #1411 (next active FE issue)?"

**On user passing:**
> Stop. Don't re-offer this session.
