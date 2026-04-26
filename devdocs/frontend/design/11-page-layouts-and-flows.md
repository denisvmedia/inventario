# Page Layouts & Flows

Six page templates cover ~90% of Inventario surfaces. Plus three flows: onboarding, error pages, and the global navigation shell.

## Global navigation shell

### Desktop (≥1024px): Sidebar

Replaces the current pipe-separated topbar.

```
┌────────┬────────────────────────────────────────────────┐
│        │                                                │
│  Logo  │  [page content]                                │
│        │                                                │
│ ───────│                                                │
│ Home   │                                                │
│ Things │                                                │
│ Places │                                                │
│ Files  │                                                │
│ Backups│                                                │
│ ───────│                                                │
│ Search │                                                │
│ ───────│                                                │
│ [user] │                                                │
│        │                                                │
└────────┴────────────────────────────────────────────────┘
```

#### Anatomy
- Width: 240px expanded, 64px collapsed (icon-only)
- bg: `--surface-base` (subtly distinct from content area `--surface-raised` if used)
- Logo top, `--space-6` padding
- Nav items: icon + label, `padding: space-2 space-3`, radius `md`
- Active item: bg `--accent-soft`, ink `--accent`, weight medium, leading-icon weight bold
- Hover inactive: bg `--surface-sunken`
- Group separator: thin `--border-subtle` line, `space-3` margin
- Group dropdown (current "Default" pill): renders inside sidebar near the user section
- Search bar pinned in middle section — global keyboard shortcut ⌘K opens command palette
- User section bottom: avatar + name + chevron → dropdown with profile/settings/logout

#### Item terminology rename
Current: Home / Locations / Commodities / Files / Exports / System
Proposed: Home / Things / Places / Files / Backups / (System moved under user menu — admin-only)

Reasoning per `12-tone-of-voice.md`: "Things" reads warmer than "Commodities", "Places" warmer than "Locations", "Backups" clearer than "Exports".

#### Collapsed state
Click chevron at logo → collapses to 64px wide, icon-only. Tooltips on hover show labels.

### Mobile (<1024px): Bottom Navigation

Replaces sidebar. Fixed bottom bar.

```
┌──────────────────────────────────────────┐
│                                          │
│        [page content, scrollable]        │
│                                          │
│                                          │
├──────────────────────────────────────────┤
│ [Home] [Things] [Add+] [Places] [More]  │  bottom nav
└──────────────────────────────────────────┘
```

- 5 items: Home, Things, Add (FAB-like central button), Places, More
- "More" opens a sheet with: Files, Backups, Search, Profile, System
- Add button is centered, slightly raised, opens action sheet ("Add a thing", "Add a place", "Upload a file")
- Active item: ink `--accent`, icon weight `bold`
- Inactive: ink `--ink-secondary`, icon weight `regular`
- Height 56px + safe-area-inset-bottom
- Backdrop-blur on the bar when content scrolls beneath

### Tablet (640–1023px)

Hybrid: collapsed icon-sidebar on the left + the same content area. Or fall to bottom nav. Decision: **collapsed sidebar 64px** at this breakpoint — preserves desktop feel, doesn't sacrifice content width.

## Six page templates

### Template 1: List view

For locations (Places), commodities (Things), files, backups.

```
┌────────────────────────────────────────────┐
│ Places                       [+ Add place] │  page header
│ 4 places · 51 areas                        │  subtitle
├────────────────────────────────────────────┤
│ [search]  [filters]                  [⋯]   │  filter bar
├────────────────────────────────────────────┤
│                                            │
│ [card grid or row list]                    │
│                                            │
├────────────────────────────────────────────┤
│   [pagination]                             │
└────────────────────────────────────────────┘
```

- Page header: title `heading-xl`, subtitle `body-sm muted`, primary action top-right
- Filter bar: per `09-component-patterns.md`
- Content: cards (Things, Files) or rows (Places — fits the hierarchy of areas inside)
- Pagination at bottom

### Template 2: Detail view

For Place / Thing / File / Backup.

```
┌────────────────────────────────────────────┐
│ ← Back to Things                           │  breadcrumb-link
├────────────────────────────────────────────┤
│ [thumbnail/avatar]  Camping Equipment      │  hero
│                     Outdoor · Bedroom      │
│                     [edit] [delete] [⋯]    │
├────────────────────────────────────────────┤
│ ┌──────────────────┐ ┌──────────────────┐ │
│ │ Basic info       │ │ Price            │ │  cards / sections
│ │ ...              │ │ ...              │ │  2-col grid on desktop
│ └──────────────────┘ └──────────────────┘ │
│                                            │
│ Images           [+ Add]                   │  section header + action
│ [file gallery]                             │
│                                            │
│ Manuals          [+ Add]                   │
│ [file gallery]                             │
│                                            │
│ Activity                                   │
│ [activity feed]                            │
└────────────────────────────────────────────┘
```

- Hero band: replaces dark-on-dark current pattern. Item title `heading-xl`, secondary metadata `body muted`. Actions top-right inline. **No** dark band.
- 2-col card grid for primary metadata
- Stacked sections below for relationships (images, manuals, activity)

### Template 3: Form view

Create or edit.

```
┌────────────────────────────────────────────┐
│ ← Cancel                                   │
├────────────────────────────────────────────┤
│ Add a thing                                │  heading-xl
│ Anything you own — tools, electronics,     │
│ furniture, items in a collection.          │  subtitle, body-lg muted
├────────────────────────────────────────────┤
│ Basic                                      │  section header
│ ...fields...                               │
│                                            │
│ Where it lives                             │
│ ...fields...                               │
│                                            │
│ Documentation                              │
│ ...file uploaders...                       │
├────────────────────────────────────────────┤
│ [unsaved hint]    [Cancel] [Save]          │  sticky footer
└────────────────────────────────────────────┘
```

- Form max-width: `--text-measure-form` (42ch ≈ 540px)
- Sections per `09-component-patterns.md` form section
- Sticky footer per `09-component-patterns.md` form footer

### Template 4: Dashboard

Specced in `07-data-visualization.md`. Wide content area, 12-col grid widgets.

### Template 5: Empty / Onboarding

Full-page centered call-to-action.

```
┌────────────────────────────────────────────┐
│                                            │
│                                            │
│         [illustration / icon-tile]         │
│                                            │
│         Title                              │  heading-lg
│         Subtitle copy explaining what      │
│         this surface is for.               │  body-lg muted, max 50ch
│                                            │
│         [primary CTA]  [secondary CTA?]    │
│                                            │
│                                            │
└────────────────────────────────────────────┘
```

- Vertical center within the page (NOT viewport — accounts for sidebar/topbar offset)
- Max-content width 480px
- Used by: empty list views (no things yet), error pages, onboarding screens

### Template 6: Auth / Single-form

Login, register, forgot password, accept invite.

#### Desktop layout (≥1024px): Split-screen

```
┌──────────────────────┬─────────────────────┐
│                      │                     │
│   [logo]             │                     │
│                      │                     │
│   Welcome back.      │   [illustration or  │
│                      │   editorial photo / │
│   ┌────────────────┐ │   warm pattern]     │
│   │ Email          │ │                     │
│   └────────────────┘ │                     │
│   ┌────────────────┐ │                     │
│   │ Password       │ │                     │
│   └────────────────┘ │                     │
│                      │                     │
│   [Sign in]          │                     │
│                      │                     │
│   Forgot password?   │                     │
│                      │                     │
│   New here? Create   │                     │
│   an account.        │                     │
│                      │                     │
└──────────────────────┴─────────────────────┘
```

- Left column 50%, right column 50% (or 40/60 with form on the wider side at very large viewports)
- Form max-width 360px, vertical-centered
- Right column: warm illustration **or** abstract pattern from palette **or** marketing copy. Decided per illustration sourcing (`06-iconography-and-illustration.md`)

#### Mobile: single-column, no split. Form takes full width with `--padding-page-x` gutters.

This replaces the current "lonely card in the void" login.

## Onboarding flow

Three screens, plus a starter selection.

### Screen 1: Welcome

```
[logo] Inventario

A quiet place to keep track of your things.

What's the first thing you'd like to remember?
   ◯ The stuff in my home
   ◯ My collection
   ◯ A property's documentation
   ◯ Something else

[Continue]
```

- Choice drives the **starter preset** for categories, dashboard widgets, sample copy
- Use Reka UI Radio with each option styled as a card with icon + title + subtitle

### Screen 2: First thing

Based on Screen 1 choice, prompt the first inventory entry inline (not a full form, just the essentials).

For "stuff in my home":
```
Let's add your first thing.
What is it?  [______________________]   e.g. Dishwasher, sofa, your guitar
Where does it live?  [Bedroom ▾]
[Skip] [Add it]
```

Cancellable. Result: one entry in the inventory plus a celebratory toast on Screen 3.

### Screen 3: Quick tour

Three slides explaining: where to find things, how to add files, how warranties/expiry work. Each slide shows a screenshot + caption. Skippable.

After this, user lands on dashboard with their first thing in "Recently added".

## Error pages

### 404

```
[illustration: empty drawer / lost label]

We couldn't find that.

The page may have moved, or you might have
followed an old link.

[Take me home]   [Search]
```

### 500

```
[illustration: paper falling]

Something on our end broke.

We're sorry. Try again in a moment, or refresh
the page.

[Refresh]   [Report this]
```

### Maintenance

```
[illustration: do-not-disturb / closed-shutters]

We're tidying up.

Inventario is briefly unavailable while we
update. Back in a few minutes.

[Try again]
```

### Offline (PWA-aware, deferred to v2)

```
[illustration: unplugged]

You're offline.

We'll sync your changes when you're back.
```

## Confirmation dialogs

For destructive actions.

### Anatomy
```
Delete "Camping Equipment"?

This will permanently remove the item, its 5
images, and its 3 manuals. This cannot be undone.

[Cancel]   [Delete]
```

- Title: action-as-question, no exclamation
- Body: explicit consequences (count of children deleted, irreversibility)
- Primary action: destructive variant button on the right
- Cancel on the left, ghost variant
- Focus default: cancel button (not delete — accidental Enter shouldn't destroy)
- Type-to-confirm for high-risk: "Type 'Camping Equipment' to confirm" — used for archive/delete of >10 children

## Dashboard layout

Specced in `07-data-visualization.md`. Don't repeat here.

## Print layout

Inventory printout (per `18-print-and-export.md`). Reset all chrome (sidebar, topbar, action buttons) for print stylesheet. Print template:
- Title block with date
- Each item as a row with photo, name, location, value
- Footer page numbers
- Black-and-white safe colors

## Empty state copy templates

Per surface, here are first-time empty copies. These ship in sprint 0:

| Surface | Title | Description | Primary CTA |
| --- | --- | --- | --- |
| `/things` (Things list) empty | Nothing here yet. | Start with one thing — an appliance, a piece of furniture, anything you'd want to remember. | + Add a thing |
| `/places` empty | No places yet. | Add a place — your home, an office, a storage unit — to organize your things. | + Add a place |
| `/files` empty | No files yet. | Files attach to things and places. Add a thing first, then upload its receipts and manuals. | + Add a thing |
| `/files` filtered empty | No files match. | Try a different filter or clear them all. | Clear filters |
| `/backups` empty | No backups yet. | Make your first backup so your records are safe. | + New backup |
| Search empty | Nothing matches "[query]". | Try different words, or check spelling. | — |

## What ships in sprint 0

1. Replace pipe-topbar with sidebar (desktop) + bottom nav (mobile) shell
2. Fix login template (split-screen with palette-aware right column)
3. Build EmptyState primitive and apply to all empty-list surfaces
4. Replace dark-band detail-view headers with the proper hero pattern
5. Add 404 / 500 / maintenance error pages

## Decision needed

- Sidebar item rename (Things / Places / Backups) — yes or keep current?
- Onboarding flow — ship in sprint 0 or sprint 2?
