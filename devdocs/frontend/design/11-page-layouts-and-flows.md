# Page Layouts & Flows

Page-shaped templates per surface. **Recommendations.** Each template
has been used in production at least twice; deviating is fine when the
deviation has a reason.

## Auth

Single-column, centered, no sidebar:

```tsx
<AuthLayout>
  <div className="space-y-6">
    <div className="space-y-1.5 text-center">
      <AppLogo className="mx-auto h-9" />
      <h1 className="text-2xl font-semibold tracking-tight">{t("auth:login.title")}</h1>
      <p className="text-sm text-muted-foreground">{t("auth:login.subtitle")}</p>
    </div>
    <form className="space-y-4">{/* fields */}</form>
    <p className="text-center text-sm text-muted-foreground">
      <Link to="/forgot-password" className="hover:text-foreground">…</Link>
    </p>
  </div>
</AuthLayout>
```

- `max-w-md` form container.
- No sidebar, no top bar, no breadcrumbs.
- One CTA per page.
- The footer is a single line linking to the next step
  ("Don't have an account? Sign up.").
- No marketing content. No terms-of-service paragraph. The product
  positioning ([00-positioning.md](00-positioning.md)) is quiet, copy-first.

Variants:

- Login → Register (link below form).
- Login → Forgot password (link below form).
- Register → Verify email (replaces form with a "check your inbox"
  message after submit).
- Reset password → Success (replaces form with a "you can sign in
  now" message).

## Dashboard

Stat row + recent items + warranty alerts. Glanceable, not
celebratory:

```
┌──────────────────────────────────────────────────────────────────┐
│ Sidebar │  Top bar (title + group switcher)                       │
│         │ ┌─ Stat row: 4 stat cards ─────────────────────────────┐ │
│         │ │ Items │ Locations │ Areas │ Files                    │ │
│         │ └──────────────────────────────────────────────────────┘ │
│         │ ┌─ Recent items (left) ──┐ ┌─ Warranty alerts (right) ─┐ │
│         │ │ list of 5 with thumbs  │ │ list of 3 expiring soon   │ │
│         │ │ "View all"             │ │ "View all"                │ │
│         │ └────────────────────────┘ └───────────────────────────┘ │
│         │ ┌─ Activity feed (full width) ─────────────────────────┐ │
│         │ │ "12 items added · April 18"                          │ │
│         │ └──────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────┘
```

Card decisions:

- `max-w-6xl` outer.
- 4-column stat row at `lg`, 2-column at `md`, 1-column on mobile.
- 2-column body grid on `lg`, single column below.
- Activity feed is a chronological list, not a chart. [00-positioning.md](00-positioning.md)
  commits to "boring on purpose".

## List page

Items, locations, files, tags, exports, search results — same shell:

```
┌─ Page header ────────────────────────────────────────────────────┐
│ <h1>Items</h1>                                                   │
│ <p>Subtitle (e.g. count of items in this group)</p>              │
│                                                                  │
│ [Filter chips ▼] [Sort ▼]                          [+ Add item]  │
├──────────────────────────────────────────────────────────────────┤
│ Search input (full width)                                        │
├──────────────────────────────────────────────────────────────────┤
│ ┌─ Bulk actions bar (only when ≥1 selected) ──────────────────┐ │
│ │ 3 selected · Delete · Move · Tag                           │ │
│ └─────────────────────────────────────────────────────────────┘ │
│ List rows:                                                      │
│   ☐ Icon · Title · Subtitle · Right-meta · ⋯                   │
│   ☐ …                                                          │
│ Pagination footer: « 1 2 3 »                                    │
└──────────────────────────────────────────────────────────────────┘
```

- `max-w-4xl` container for most lists.
- Bulk actions toolbar slides in via `tw-animate-css` `slide-in-from-top`
  when selection state becomes non-empty. Disappears smoothly.
- Filter / sort chips open as Popovers, not fullscreen sheets, on
  desktop.
- Pagination, not infinite scroll. The user can return to a page they
  saw before.

## Detail page

One concept per page. Items, locations, areas, files all use the same
shell:

```
┌─ Breadcrumb ──────────────────────────────────────────────────────┐
│ Home > Locations > Kitchen > Stove                                │
├───────────────────────────────────────────────────────────────────┤
│ ┌─ Hero card ─────────────────────────────────────────────────┐ │
│ │ [thumb] Title         [Status badge]                        │ │
│ │         Subtitle      [Edit]                                │ │
│ └─────────────────────────────────────────────────────────────┘ │
│ ┌─ Section: Details ─────────────────────────────────────────┐ │
│ │ Bought · Apr 2026                                           │ │
│ │ Vendor · …                                                  │ │
│ └─────────────────────────────────────────────────────────────┘ │
│ ┌─ Section: Files ───────────────────────────────────────────┐ │
│ │ Gallery + add                                              │ │
│ └─────────────────────────────────────────────────────────────┘ │
│ ┌─ Section: History ─────────────────────────────────────────┐ │
│ │ Status changes timeline                                    │ │
│ └─────────────────────────────────────────────────────────────┘ │
└───────────────────────────────────────────────────────────────────┘
```

- `max-w-2xl` outer (single-column reading rhythm).
- Each section is a card with `space-y-5` internal rhythm.
- Edit can be inline (per-section "Edit" button → switch to form
  inputs) or a Sheet, depending on the section. Multi-section edit
  uses a Sheet.

## Settings page

```
┌─ Page header ──────────────────────────────────┐
│ <h1>Settings</h1>                              │
│ <p>Customize Inventario.</p>                   │
├────────────────────────────────────────────────┤
│ ┌─ Section: Appearance ──────────────────────┐ │
│ │ Theme         [Light · Dark · System]      │ │
│ │ Density       [Comfortable · Cozy · Compact] │
│ │ Locale        [English · Czech · Russian]  │ │
│ └────────────────────────────────────────────┘ │
│ ┌─ Section: Notifications ───────────────────┐ │
│ │ ⏵ Warranty expiring alerts        [Switch] │ │
│ │ ⏵ Email digest                    [Switch] │ │
│ └────────────────────────────────────────────┘ │
└────────────────────────────────────────────────┘
```

- `max-w-2xl` outer.
- Each section is a card with `divide-y` rows.
- Settings save **immediately** on change. No "Save" button.
- Failure to save → sonner toast with retry CTA, the value rolls back.

## Sheet-based edit / preview

A side-rail that appears next to the list page rather than navigating
away. Used for:

- Quick item preview from the dashboard.
- File detail from a thumbnail click.
- Edit-in-place for a single section.

Width: `sm:max-w-xl` for forms, `sm:max-w-md` for previews. See
[09-component-patterns.md](09-component-patterns.md).

## Multi-step wizard

The Add Item dialog (`features/commodities/`) is the canonical
wizard:

```
┌─ Wizard header ─────────────────────────────────────┐
│ Step 1 of 5: Basics                                 │
│ ●─○─○─○─○                                           │
├─────────────────────────────────────────────────────┤
│ Form fields                                         │
├─────────────────────────────────────────────────────┤
│ [Cancel]                          [Back] [Continue] │
└─────────────────────────────────────────────────────┘
```

- One RHF instance for the whole wizard. Per-step validation via
  `form.trigger([fields])`.
- Draft persisted per-key in `localStorage`
  (`commodity-draft:{slug}:create`). Cancel clears; Save clears on
  success.
- Step dots show progress + completion. Click a dot to jump back to a
  previous step (forward jumps disabled until validated).
- Width: `sm:max-w-2xl`.

## Error / 404 / 500

See [20-edge-cases.md](20-edge-cases.md). The shell is a centered card with the canonical
icon + title + body + (optional) CTA.

## Print

`/g/:slug/commodities/:id/print` is the one print-specific route.
Layout: tabular, single-column, `max-w-3xl`, no sidebar, no actions.
See [18-print-and-export.md](18-print-and-export.md).

## Hard rules

1. **One CTA per page.** Filled-primary is exactly one. Secondary
   actions are outline / ghost.
2. **No celebratory hero.** The dashboard doesn't say "Welcome back!"
3. **Pagination, not infinite scroll.** State preservation matters.
4. **Settings save immediately.** No bottom-of-page Save button.
5. **One concept per detail page.** A commodity detail is a commodity;
   it doesn't try to be a location too.

## Anti-patterns

- A dashboard "Onboarding checklist" widget that gamifies setup. We
  don't gamify.
- A list page with the search input above the page title (search
  belongs *inside* the list, not above the page).
- Inline edit on a list row that turns the row into a form. Use the
  Sheet edit pattern instead — list rows stay scannable.
- A settings page with a "Save changes" footer button. Save on change.

## Cross-refs

- Page wrappers: [03-space-and-layout.md](03-space-and-layout.md).
- Component anatomy: [09-component-patterns.md](09-component-patterns.md).
- Edge cases (404, 500, offline, no-group): [20-edge-cases.md](20-edge-cases.md).
- Auth flow: `frontend/src/pages/auth/`.
- Commodities flow (5-step wizard): `frontend/src/pages/commodities/`.
