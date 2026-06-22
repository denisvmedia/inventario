# Edge Cases

The not-happy-path surfaces. Empty states, 404, 500, offline,
no-group, deleted entities, permission denials. Each gets a real
designed surface — not a debug dump.

## The empty-state pattern

Used everywhere a list is genuinely empty. Anatomy in
[08-interaction-states.md](08-interaction-states.md); the copy contract is in
[12-tone-of-voice-and-copy.md](12-tone-of-voice-and-copy.md).

```tsx
<div className="flex flex-col items-center justify-center gap-3 py-16">
  <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10">
    <Icon className="size-5 text-primary" />
  </div>
  <div className="text-center space-y-1">
    <p className="text-base font-semibold">No items yet</p>
    <p className="text-sm text-muted-foreground">When you add one, it'll show up here.</p>
  </div>
  {cta && <Button size="sm" onClick={cta.onClick}>{cta.label}</Button>}
</div>
```

The empty-state taxonomy (which icon, which copy, which CTA) lives in
the per-page local `EmptyState` components — e.g.
`frontend/src/pages/commodities/CommoditiesListPage.tsx`,
`frontend/src/pages/warranties/WarrantiesListPage.tsx`,
`frontend/src/pages/SearchPage.tsx` — there is no shared component.

## 404 — page not found

`frontend/src/pages/NotFound.tsx`. Surface:

- Centered card on a `bg-background` page.
- Hero glyph (`Search` or similar at `size-10`).
- Title: `Couldn't find that page`
- Body: `The link might be broken, or the page might've moved.`
- Primary CTA: `Back to dashboard` (or `Sign in` if not authenticated).
- Secondary: `Go home`.

No 404-themed humor (no "Oops! 404 unicorn ate this page"). Per
[00-positioning.md](00-positioning.md).

## 500 — application crashed

The route boundary blew up. Caught by an `<ErrorBoundary>` in
`Shell.tsx`. Surface:

- Centered card.
- Hero glyph (`AlertOctagon` at `size-10`, `text-destructive`).
- Title: `Something went wrong`
- Body: `We've been notified. You can try again or head back.`
- Primary CTA: `Reload`.
- Secondary: `Back to dashboard`.

The "we've been notified" line is a half-promise — Sentry-style
telemetry isn't wired today. Update the copy when it lands.

## Offline / no network

The user lost connectivity mid-session.

- `navigator.onLine` is checked at the http-wrapper layer; on
  network errors the wrapper throws an `HttpError` with a non-HTTP
  surface (e.g. a thrown `TypeError` from fetch).
- Sonner toast: `You're offline. Changes will retry when you're back.`
- Mutations queued via TanStack Query don't auto-retry on offline —
  user retries manually.
- Persistent banner is *not* used for offline — toasts only. A
  persistent banner over the page would be heavier than the actual
  cost of being offline for 30 seconds.

## No-group state

The user is signed in but doesn't belong to any group. Route:
`/no-group`.

Surface:

- Centered card.
- Hero glyph (`Users` at `size-10` over `bg-primary/10`).
- Title: `You're not in a group yet`
- Body: `Create one to start your inventory, or wait for an invite.`
- Primary CTA: `Create group`.
- Secondary: `Sign out`.

This is the entry point for new users post-registration before they
create a group. Once a group exists, `<UngroupedRedirect>` bounces
them away.

## Deleted entity

The user navigated to `/g/:slug/commodities/:id` for an `:id` that
no longer exists (deleted in another tab, or a stale link).

Surface:

- Same shell as 404.
- Title: `This item is gone`
- Body: `It might've been deleted. Try the items list.`
- Primary: `Back to items`.

Backend returns 404 → page-level error → render this layout. Don't
conflate with the global `NotFoundPage` — feature-specific 404s read
better with feature-specific copy.

## Permission denied

A user tries to access a route they're not allowed to (e.g. group
admin pages without admin role).

Backend returns 403 → page-level error.

- Title: `Not allowed`
- Body: `You don't have permission to view this. Ask a group admin to
  invite you with a higher role.`
- Primary: `Back`.

## Deleted-account / suspended-tenant

These come from the auth flow. The /auth/me probe returns a 410 or
similar terminal. Sonner toast + auto-bounce to `/login` with
`?reason=`:

- `?reason=account_deleted` → "Your account has been deleted."
- `?reason=tenant_suspended` → "This tenant has been suspended."
- `?reason=auth_required` → "Please sign in to continue." (default)

Reason copy lives in `auth:session.*` per
[12-tone-of-voice-and-copy.md](12-tone-of-voice-and-copy.md).

## Rate-limited

`429 Too Many Requests`. Sonner toast: `Too many requests. Try again
in <30s>.` Use `Retry-After` header if present; default to 30s.
Don't auto-retry — let the user breathe.

## Maintenance / scheduled downtime

Out of scope today. When the BE adds a maintenance flag, surface as a
top-bar banner (yes, persistent, exception to the toast-only rule
because the user can't act around it):

```
[ Maintenance: Inventario will be briefly unavailable on Sun 2 May at 02:00 UTC ]
```

Banner uses `bg-status-expiring/10 text-status-expiring border-b
border-status-expiring/30`. Dismissible; reappears on next session.

## Webview / iframed

Inventario isn't designed to be iframed. If `window.top !==
window.self`, render a single-line surface saying "Open Inventario in
a new window." with a link. Don't ship the full app inside an
iframe.

## Browser too old

The build targets ES2022 (per `vite.config.ts`). Anything older
should fail loudly with a single page of fallback HTML — but Vite's
`<script type="module">` already does this gracefully (older browsers
ignore the module script and the noscript / fallback `<div id=root>`
empties).

A polite version: `frontend/index.html` could carry a `<noscript>`
banner. Currently doesn't — file an issue if it becomes a problem.

## Empty search

The user searched and there are no matches.

```
[ Empty-state shape, but with the search term echoed in the body ]
"No items match \"foo\"."
"Try a different term, or clear filters."
[ Clear filters CTA ]
```

The CTA clears filter chips, not the search term — clearing the term
itself is a backspace.

## Hard rules

1. **Every error has a designed surface.** No bare exception text.
2. **Specific 404s over generic 404s** when the feature has the
   context (deleted-item beats "Couldn't find that page").
3. **Toast for transient, page for terminal.** Offline = toast;
   account deleted = page.
4. **Empty states have CTAs** when there's an obvious next step.
5. **No humor in errors.** Per [00-positioning.md](00-positioning.md).

## Anti-patterns

- `throw new Error(JSON.stringify(err))` rendered raw in the UI. Use
  `parseServerError`.
- 404 page with a 500-line illustration. Quiet glyph.
- A persistent "You're offline" banner that won't dismiss when the
  network is back.
- A 500 page that says "Don't worry, your data is safe!" without
  evidence. The user already lost trust by hitting a 500.
- An empty state that says "Loading…". Loading and empty are
  different states.

## Cross-refs

- States: [08-interaction-states.md](08-interaction-states.md).
- Notifications: [16-notifications-and-trust.md](16-notifications-and-trust.md).
- Voice / copy: [12-tone-of-voice-and-copy.md](12-tone-of-voice-and-copy.md).
- Page templates: [11-page-layouts-and-flows.md](11-page-layouts-and-flows.md).
- Routing guards (no-group, protected): [../routing.md](../routing.md).
- Surfaces in source:
  - `frontend/src/pages/NotFound.tsx`
  - `frontend/src/pages/NoGroupPage.tsx`
  - `frontend/src/components/coming-soon/ComingSoonPage.tsx`
  - Per-page local `EmptyState` components (e.g.
    `frontend/src/pages/commodities/CommoditiesListPage.tsx`)
