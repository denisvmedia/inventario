---
name: screenshot-review
description: 'Generate local screenshots of the Inventario React frontend via `e2e/screenshots.mjs` and review them visually for bugs (unresolved i18n keys, broken layout, wrong currency, truncated dates, dark-mode regressions, missing fallbacks). Use for visual testing, UI review, dark-mode checks, verifying frontend appearance, checking how a page looks, or eyeballing visual regressions. Triggers on phrases like "screenshot the app", "check how it looks", "visual review", "verify the UI", "post screenshots on the issue", or "make sure the page renders right". After a frontend change in `frontend/`, OFFER this review to the user — never auto-runs. On further explicit user request, captures may be published to an `assets/screenshots-ISSUE` branch via `e2e/push-screenshots.sh` and embedded as an Issue/PR comment via `mcp__github_and_git__add_issue_comment`. Strict rule — never produce screenshots, push them, or comment on Issues without an explicit user "yes" per step. Skip for backend-only, docs-only, type-only, or pure-test changes.'
---

# Screenshot review (local)

## What this skill does

Three related jobs. All end with the user owning the decision; none happens without explicit consent for that specific step.

1. **On user request — capture + review.** Run `e2e/screenshots.mjs` against a local running build, save the PNGs under `.research/screenshots/<label>/` (gitignored — see "Where do screenshots live"), then read each one and report what looks right vs. broken.
2. **After any frontend change you just made — offer #1.** Surface the offer to run job #1 with a one-line description of what you'd cover. Wait for the user to say go.
3. **On further explicit user request — publish.** When the user asks to share captures (e.g. "post these on issue #1527"), push the PNGs to an `assets/screenshots-<issue>` branch via `e2e/push-screenshots.sh`, then embed the printed raw URLs in an Issue comment via `mcp__github_and_git__add_issue_comment`.

You can act on #1 and #3. You cannot act on #2 — only offer. Each of #1 and #3 needs its own "yes"; an earlier yes for capture is **not** consent for publishing.

## The hard rules

- **Never run a screenshot pass unsolicited.** If the user hasn't asked, propose it; then wait.
- **The user's last word wins.** A "skip it" earlier in the session means skip it for the rest of the session unless they reverse it.
- **No screenshots in the working-branch diff.** Captures live under `.research/screenshots/<label>/` and that path is in the maintainer's global `.gitignore` — that's intentional. Don't `git add -f` past the ignore.
- **Publishing is a separate, opt-in step.** The flow is `e2e/push-screenshots.sh <issue>` → `assets/screenshots-<issue>` branch → Issue comment with raw URLs. See "Optional: sharing the captures" for mechanics.
- **Don't substitute Playwright e2e specs for this.** `e2e/screenshots.mjs` is a separate Node script that boots the binary, logs in, and saves PNGs. The Playwright spec suite is unrelated — see the `inventario-e2e` skill for that.

## When to OFFER (don't decide for the user)

Offer after you finish any of these in `frontend/`:

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

If unsure, lean toward offering — the friction is one sentence and one user "no".

## How to run job #1 (the actual screenshot pass)

Walk this carefully — half the value is catching the user's hidden assumptions before the script burns an iteration.

### 1. Confirm the prerequisites are met

Ask the user (don't try to set these up yourself unless asked):

- Is the local binary already running?
  ```
  ./bin/inventario run --db-dsn=memory:// \
    --no-auth-rate-limit --no-global-rate-limit
  ```
  Header comment in `e2e/screenshots.mjs` is the source of truth.
- Is the database seeded?
  ```
  curl -X POST http://localhost:3333/api/v1/seed
  ```
- Is `frontend/` built into the embed?
  ```
  make build-frontend
  cd go/cmd/inventario && go build -tags with_frontend -o ../../../bin/inventario .
  ```

If any of these is "no", say so and stop — don't pivot to "I'll set it up". The user may have a reason their binary isn't running (other branch, different port, dev server vs binary).

### 2. Run the script

```
BASE_URL=http://localhost:3333 \
  OUT=.research/screenshots/<label>/ \
  node e2e/screenshots.mjs
```

The `OUT` env var lets the script write directly into the canonical local-only folder — no second `cp` step. Pick a `<label>` that's easy to reuse later: usually the issue number (e.g. `1527`) or a feature slug. The same label is what `e2e/push-screenshots.sh` reads from when (and only when) the user later asks to publish. If the user has a different convention or out-path preference, defer.

### 3. Read every PNG

Use the `Read` tool on each `.png` in **numeric order** (the script prefixes 01-, 02-, …) so you walk the user's flow as they would experience it.

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

### 5. Verify the captures stay local by default

The captures should remain under `.research/screenshots/<label>/` and untracked by the working branch. Verify with `mcp__github_and_git__git_status` that `.research/screenshots/<label>/` is invisible — the maintainer's global gitignore on `.research/` keeps it untracked. If you see it staged on the working branch, stop and unstage before continuing. (Publishing is a separate flow that operates on a *different* branch — see below.)

## Optional: sharing the captures

The default is local-only — review on screen, never push. Publishing is the opt-in flow defined in "The hard rules" above. Run it only when the user explicitly authorizes it for that step.

### Step A — Push the captures to a side branch

Run `e2e/push-screenshots.sh <issue> [src-label] [glob...]` via Bash. The script (canonical reference: the header comment at the top of the file):

- creates or fast-forwards a branch named `assets/screenshots-<issue>` rooted on `origin/master`,
- replaces `assets/screenshots-<issue>/` with the selected PNGs from `.research/screenshots/<src-label>/`,
- commits with `[#<issue>] visual proof: refresh captures from <src-label>`,
- pushes to `origin`,
- prints a **commit-pinned** raw URL prefix:
  `https://raw.githubusercontent.com/denisvmedia/inventario/<sha>/assets/screenshots-<issue>/<file>.png`.

`<src-label>` defaults to the slugified current branch — same default `screenshots.mjs` uses when `OUT` is omitted. Trailing globs (matched against basenames) filter which captures get published if the source folder serves several issues.

The script uses the local git CLI directly because the GitHub MCP path-allowlist + binary-content limits prevent it from pushing PNGs through the MCP. The `git-github-mcp-only` skill explicitly allows this fallback because the operation cannot be expressed through the MCP — but the *only* reason this fallback is OK is that the user just authorized it. Don't extend the exception to anything else.

These `assets/screenshots-<issue>` branches are intentionally throwaway — the maintainer prunes them once visual review is settled.

### Step B — Draft and post the Issue comment

Once you have the raw URL prefix and commit SHA from Step A:

1. Draft a short comment body — one or two lines of context, then inline `<img>` tags using the printed URL prefix. Width hints (`width="320"` or `width="480"`) help readers scan multiple captures at once.
2. Confirm the body with the user before posting.
3. Post via `mcp__github_and_git__add_issue_comment` on the same Issue.

Example body:

```html
Visual proof for #1527 — dashboard + locations after the layout fix.

<img src="https://raw.githubusercontent.com/denisvmedia/inventario/<sha>/assets/screenshots-1527/10-dashboard.png" width="480">
<img src="https://raw.githubusercontent.com/denisvmedia/inventario/<sha>/assets/screenshots-1527/11-locations.png" width="480">
```

### Step C — Hands off

Don't re-push, re-comment, or rename the branch later in the session unless the user explicitly asks. The branch belongs to the maintainer once published.

## Surfaces the script covers

`e2e/screenshots.mjs` walks (header comment is canonical — re-read it if it drifts):

- Unauth: `/login`, `/register`, `/forgot-password`, catch-all 404
- Group-scoped: `/g/:slug/` (dashboard), `/locations`, `/locations/new`, location detail, area detail
- Commodities (#1410): list, sheet preview, add dialog step 1, detail, print
- Personal: `/profile`, `/settings`

When you offer the pass, name the surfaces relevant to the change, not the full set. "Dashboard + locations" is more useful than "all 16".

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

**On explicit publish request ("post these on issue #1527"):**
> "Pushing `.research/screenshots/1527/` to `assets/screenshots-1527`…
> Published. Commit `<sha>`. Drafting Issue comment with 4 inline images at width 480 — want me to post it as is, or trim/reorder?"
