---
name: design-mock-fidelity
description: 'Replicate or extend Inventario UI in `frontend/` to match the `design-mocks/` reference pixel-for-pixel — same Tailwind tokens, same spacing, same shadcn/ui component anatomy. Use when implementing a design, matching a mockup, porting a screen from the mock, building a new page or component for `frontend/src/**`, or fixing visual fidelity drift. Triggers on phrases like "implement design", "match the mockup", "port from design-mocks", "pixel-perfect", "make it look like the mock", "convert mock to code", "UI fidelity", or any request to author markup, classNames, or component composition for `frontend/src/**` when an analogous file exists in `design-mocks/src/**`. Provides a path-to-path index plus ready-to-paste TSX patterns for badges, buttons, dialogs, layout primitives, and typography. Companion to `frontend-work` — that skill handles pre-flight/post-flight; this one carries the replication recipes. Skip for backend/test/types-only changes or when no visual analogue exists in the mock.'
---

# Design mock fidelity

The `design-mocks/` directory is the canonical visual contract for everything in `frontend/`. This skill is the playbook for porting from one to the other identically — same tokens, same spacing rhythm, same component anatomy — without re-reading the entire mock every time.

`frontend-work` is the umbrella process (pre-flight, deviation log, post-flight screenshots). This skill drops in at the *replicate-the-surface* step and tells you exactly what to copy and where.

## When to activate

Activate the moment the next thing you'll do is write JSX/TSX or className strings for a real visual surface under `frontend/src/`:

- A new `pages/` page or `features/<x>/` slice with a UI surface.
- A shadcn primitive being composed for the first time in this repo.
- A layout/spacing/typography refresh.
- App-shell, sidebar, topbar, modal, dialog, sheet, drawer surfaces.
- Empty-state, error-state, or onboarding surfaces.
- Anything where the user said "match the mock", "make it look right", "use the design".

**Skip** when the change is purely:
- Backend (`go/`), tests, migrations, infra, scripts.
- Type generation (`src/types/api.d.ts`).
- i18n value-only edits where no visual structure changes (still review the surface, but no replication needed).

If you only *think* you need it — read **Step 1**. If the surface index resolves to a mock file in five seconds, you needed it.

## Hard rules (never violate)

This is the single checklist. It works as a pre-flight contract and a post-flight drift detector — if you find yourself doing the opposite of any rule, stop and re-identify the pattern in the mock.

1. **`design-mocks/` is read-only.** Do not Edit, Write, move, rename, or `git add` anything under that path. It's a sync mirror of an upstream repo and your edits get wiped. If something looks wrong *in the mock*, surface it verbally to the user — it gets fixed upstream.
2. **Default fidelity is 1:1.** Same DOM structure, same Tailwind classes, same icon imports, same copy structure. Drift is a deliberate, logged decision — see `devdocs/frontend/design-deviations.md` and the entry template at the top of that file.
3. **Tokens, never raw colors.** No `text-amber-500`, `#f59e0b`, `bg-green-100`, `border-red-300`, etc. The mock uses semantic tokens (`text-status-active`, `bg-chart-1/15`, `text-destructive`). If a needed token doesn't exist, add it to `frontend/src/index.css` in both `:root` and `.dark`, register it in `@theme inline`, and match the OKLCH values in `design-mocks/src/index.css`. No purple, no indigo, no violet anywhere — the palette is warm-neutral + amber. Charts beyond five series cycle `chart-1..5`, not new hues.
4. **No `forwardRef`, no `@tailwindcss/animate`, no `hsl()` in token *definitions*.** Tailwind v4 uses `React.ComponentProps<>` and `tw-animate-css`. Token declarations in `index.css` are raw OKLCH with no `hsl()` wrapper. The one place where `hsl(var(--…))` legitimately appears is the shadcn sidebar's `<SidebarRail>` outline trick (`shadow-[0_0_0_1px_hsl(var(--sidebar-border))]`, mirrored in `frontend/src/components/ui/sidebar.tsx`) — preserve that when you see it; don't introduce new `hsl()` wrappers anywhere else.
5. **Don't add elevation shadows on top of shadcn primitives.** shadcn primitives already ship the shadows the design language calls for: `card.tsx` → `shadow-sm`, `dialog.tsx` / `sheet.tsx` → `shadow-lg`, `popover.tsx` / `dropdown-menu.tsx` → `shadow-md`–`shadow-lg`, sidebar floating/inset → `shadow-sm`, sidebar rail → an inset 1px `shadow-[…]` outline, `input.tsx` → `shadow-xs`. Keep those as-is. The rule is *don't paint extra `shadow-*` onto your own surfaces*: bespoke cards, custom containers, hand-rolled chips, list rows — those use borders + tokens, never new shadows.
6. **Use canonical spacing.** `p-6 / gap-6 / py-3.5 / space-y-5` are the recurring values. If they don't fit, you're mismatching the section type — re-identify the pattern instead of inventing a new gap or padding.
7. **No native browser modals.** No `window.confirm`, `alert()`, `prompt()`. Use `<AlertDialog>`, `<Dialog>`, or sonner toasts.
8. **Named exports only for views and components.** No default exports.
9. **No inline `style={{ … }}` for static visual choices.** Static colors, paddings, font sizes, radii belong in Tailwind utilities or `index.css` tokens. Inline styles are legitimate only when a *runtime value* must flow into CSS that utilities can't express: data-driven geometry/dimensions (`width: ${pct}%`, `height: ${dim}`), data-driven colors pulling from a token (`backgroundColor: var(--status-${status})`), background images from runtime URLs, transforms in zoom/pan UIs, and shadcn primitives that take CSS vars via `style` (e.g. `<SidebarProvider style={{ "--sidebar-width": "16rem" }}>`). The dashboard breakdown bar in `views/DashboardView.tsx:236` does both at once — width and a token-derived background — and that's the canonical example.
10. **One icon library.** `lucide-react` is the only one. Pick the closest glyph or change the metaphor; never add a second library.
11. **The primitive set is fixed.** shadcn/ui (`new-york` style) + Radix via the `radix-ui` umbrella + `cmdk` for command palettes. No "I'll use `react-select` instead of `<Combobox>` because…". New primitives only via `radix-ui` or the shadcn CLI.
12. **Reuse domain components.** Reach for `WarrantyBadge`, `CurrencyCombobox`, etc. before composing markup yourself.
13. **Every visible string is `t("…")`-wrapped**, even prototype labels. No hardcoded English copy in `frontend/`.
14. **No new CSS files.** All styling lives in Tailwind utilities + the single `index.css` token sheet. There is no second stylesheet.

## Step 1 — Locate the canonical surface (don't search; index)

Use this index to jump straight to the mock file. **Open one file, not five.**

### Pages / views (mock: `design-mocks/src/views/<X>View.tsx`)

| You're building in `frontend/` | Open this mock file |
|---|---|
| Dashboard / overview | `views/DashboardView.tsx` (stat cards, expiring list, recent list, breakdown bar) |
| Items list / catalog | `views/ItemsView.tsx` |
| Warranties list | `views/WarrantiesView.tsx` |
| Tags page | `views/TagsView.tsx` (tag pills, color picker pattern) |
| Locations / areas | `views/LocationPickerView.tsx` |
| Files browser | `views/FileBrowserView.tsx` (tile vs row toggle, category chips) |
| Members management | `views/MembersView.tsx` (role pills, invite row) |
| Backup / restore | `views/BackupView.tsx` (long-form settings + progress) |
| User profile (read) | `views/UserProfileView.tsx` |
| Edit profile (form) | `views/EditProfileView.tsx` |
| Group settings | `views/GroupSettingsView.tsx` |
| Personal settings (preferences) | `views/SettingsView.tsx` (sidebar of sections + content pane) |
| Plans / billing | `views/PlansView.tsx` |
| Auth (sign-in / sign-up) | `views/AuthView.tsx` |
| Image lightbox | `views/ImageViewerView.tsx` |
| PDF viewer | `views/PdfViewerView.tsx` |
| Insurance report (printable) | `views/InsuranceReportView.tsx` |
| 404 / no-group / no-location / no-area / maintenance / onboarding | `views/EmptyStatesView.tsx` |
| Component catalog (when nothing else fits) | `views/UIShowcaseView.tsx` (every primitive — large file, search before scrolling) |

### App-shell components (mock: `design-mocks/src/components/<X>.tsx`)

| Surface | Mock file |
|---|---|
| Left sidebar (groups, nav, user dropdown) | `components/AppSidebar.tsx` |
| Brand mark + wordmark | `components/AppLogo.tsx` |
| Group switcher in header | `components/LocationGroupSwitcher.tsx` |
| Dark/light mode toggle | `components/mode-toggle.tsx` |
| Theme provider | `components/theme-provider.tsx` |
| Onboarding tour | `components/OnboardingTour.tsx` |
| Invite banner above main | `components/InviteBanner.tsx` |

### Domain components

| Surface | Mock file |
|---|---|
| Item slide-over detail panel | `components/ItemDetail.tsx` (large — read the sections you need) |
| File detail panel | `components/FileDetail.tsx` |
| File preview dialog | `components/FilePreviewDialog.tsx` |
| Add item flow | `components/AddItemDialog.tsx` (large, wizard pattern — read step by step) |
| Add/edit area | `components/AreaDialog.tsx` |
| Add/edit location | `components/LocationDialog.tsx` |
| Currency picker | `components/CurrencyCombobox.tsx` |
| Warranty status badge | `components/WarrantyBadge.tsx` |
| Items panel (left of detail) | `components/ItemsPanel.tsx` |

### When the mock is silent

If your surface has no analogue here, fall back in this order:

1. **`views/UIShowcaseView.tsx`** — every primitive in catalog form. Pick the closest pattern.
2. **`design-mocks/CLAUDE.md` §11 (Visual Language Summary)** — the taste rubric for "what makes it feel right" vs "what breaks it."
3. **`devdocs/frontend/design/`** (22 docs, indexed by `README.md`) — design-direction docs covering palette, type, motion, components, etc., for cases the mock didn't cover.

In all three fallback cases, log a deviation entry: `Why: not present in mock` + the showcase pattern you used.

## Step 2 — Inspect, don't re-derive

Before writing a single className, in the mock file:

1. **Find the page wrapper.** It will be one of two shapes (see §"Layout primitives" below). Copy the exact class string.
2. **Identify the section structure.** Cards? Divide list? Stats grid? Tabs? Each has a fixed anatomy — see §"Pattern micro-library."
3. **Note the header pattern.** Page heading + lede paragraph almost always. Same classes, every time.
4. **Catalog the icons.** Every `lucide-react` icon used. Match exactly — do not substitute "close" `lucide` icons for the wrong ones.
5. **Read the tokens used.** `text-muted-foreground`, `bg-muted`, `border-border`, `text-status-*`, `bg-chart-*`. Note them; you'll mirror.

If a sub-block looks reusable, check `design-mocks/src/components/` first — it may already exist as `WarrantyBadge`, `CurrencyCombobox`, etc. Match the abstraction, not just the markup.

## Step 3 — Replicate identically

The next four sections are the paste-ready playbook. They ARE the mock at the patterns layer; values are taken verbatim from `design-mocks/CLAUDE.md` and the actual mock source. Use them and you don't need to re-read.

### Layout primitives

**Standard page wrapper.** Default to `max-w-2xl` for settings/detail/forms, `max-w-4xl` for lists, `max-w-5xl` for dashboard:

```tsx
<div className="flex flex-col gap-6 p-6 max-w-2xl mx-auto w-full">
  {/* content */}
</div>
```

Dashboard uses `gap-8` (more breathing room between large sections):

```tsx
<div className="flex flex-col gap-8 p-6 max-w-5xl mx-auto w-full">
```

**Page header.** Every view starts with this — exact classes:

```tsx
<div>
  <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">Page Title</h1>
  <p className="mt-1 text-muted-foreground">Short, single-sentence lede.</p>
</div>
```

**Card shell** (the bread-and-butter container):

```tsx
<div className="rounded-xl border border-border bg-card p-6 space-y-5">
  {/* content */}
</div>
```

**shadcn `<Card>`** is preferred when you need a CardHeader/Title/Description/Content/Footer split (see `views/DashboardView.tsx` for the canonical layout).

**Stats row** (icon + label + value, side by side):

```tsx
<div className="grid grid-cols-3 gap-3">
  <div className="rounded-xl border border-border bg-card px-4 py-3 flex items-center gap-3">
    <div className="flex size-8 items-center justify-center rounded-lg bg-muted shrink-0">
      <Icon className="size-4 text-muted-foreground" />
    </div>
    <div>
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="text-lg font-semibold leading-tight">{value}</p>
    </div>
  </div>
</div>
```

For the *dashboard-style* hero stat cards (quad-grid, large value, sub-line), use `<Card>` with `gap-3` and the dashboard pattern in `views/DashboardView.tsx:112-131`.

**Divide list** (settings rows):

```tsx
<div className="divide-y divide-border">
  <div className="flex items-center justify-between py-3.5">
    <p className="text-sm font-medium">Row label</p>
    <Switch checked={val} onCheckedChange={setVal} />
  </div>
</div>
```

`py-3.5` is the row height. Don't use `py-3` or `py-4`.

**Icon-headed block** (used in dialogs, list items, onboarding cards):

```tsx
<div className="flex items-center gap-3">
  <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10 shrink-0">
    <Icon className="size-5 text-primary" />
  </div>
  <div>
    <p className="font-semibold text-sm">Title</p>
    <p className="text-xs text-muted-foreground">Subtitle</p>
  </div>
</div>
```

**List row with chevron** (settings navigation, file list, etc):

```tsx
<button
  onClick={() => onNavigate("target-view")}
  className="flex w-full items-center gap-3 py-3.5 text-left hover:text-foreground transition-colors"
>
  <Icon className="size-4 text-muted-foreground shrink-0" />
  <span className="text-sm font-medium flex-1">Label</span>
  <ChevronRight className="size-4 text-muted-foreground" />
</button>
```

**Responsive grid** (stat cards, tile views):

```tsx
<div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
  {/* tiles */}
</div>
```

### Token reference card

Don't think in colors; think in tokens. **Open `design-mocks/src/index.css` only when adding a new token to `frontend/src/index.css`.**

| Need | Token (use as `text-`, `bg-`, `border-`) |
|---|---|
| Page background | `background` |
| Body text | `foreground` |
| Card surface | `card` / `card-foreground` |
| Popover/dropdown surface | `popover` / `popover-foreground` |
| Brand / primary action | `primary` / `primary-foreground` |
| Quiet surface (hovered rows, inert chips) | `secondary` |
| Subdued text / inactive icons | `muted-foreground` |
| Subdued surface | `muted` |
| Highlighted moment ("this matters") | `accent` (warm amber) |
| Destructive (delete, error) | `destructive` / `destructive-foreground` |
| Borders / input borders | `border`, `input` |
| Focus ring (3px, baked into shadcn) | `ring` |
| Sidebar surface (warmer than `card`) | `sidebar*` family |

**Domain status tokens** — these are the *only* way to color status in this app:

| Token | Means | Typical use |
|---|---|---|
| `status-active` | green: in-use, valid warranty | active items, healthy systems |
| `status-expiring` | amber: within 60 days | warranties approaching expiry |
| `status-expired` | red: past due | expired warranties, written-off items |
| `status-none` | gray: no data | no warranty / unknown state |

Pair color with a `bg-<token>/10` tint (or `/15` for chart-* family), and `border-current/20` if it's a bordered badge.

**Chart palette** (`chart-1` through `chart-5`) — used in order, not picked by hue:

| Token | Hue | When |
|---|---|---|
| `chart-1` | amber | first / primary series |
| `chart-2` | green | second / positive series |
| `chart-3` | blue | third / info series |
| `chart-4` | warm yellow | fourth |
| `chart-5` | red | fifth / contrast |

Note: never use chart tokens to convey status — they're for data viz only. For "this is healthy/expiring/dead", always reach for `status-*`.

**Radius scale**:

| Class | Use |
|---|---|
| `rounded-sm` | small chips, tight badges |
| `rounded-md` | inputs, small cards |
| `rounded-lg` | cards, modals (default for shadcn primitives) |
| `rounded-xl` | larger containers, stat cards, dialog content |
| `rounded-full` | avatars, pill badges |

### Typography scale

| Role | Classes |
|---|---|
| Page title (h1) | `scroll-m-20 text-3xl font-semibold tracking-tight` |
| Section heading (h2) | `text-base font-semibold` |
| Sub-heading (h3) | `text-sm font-semibold` |
| Body | `text-sm leading-relaxed` |
| Muted/secondary | `text-sm text-muted-foreground` |
| Overline / label | `text-xs font-semibold uppercase tracking-widest text-muted-foreground` |
| Stat value (hero) | `text-2xl font-bold tracking-tight` |
| Stat label (above value) | `text-xs font-medium uppercase tracking-wide text-muted-foreground` |
| Code / mono | `font-mono text-xs` |

### Pattern micro-library

See [`PATTERNS.md`](./PATTERNS.md) for ready-to-paste TSX for every recurring surface: badges (neutral + status), tag pills, button sizing, fields (with/without validation), save-button placement, dialogs, alert dialogs, dropdown menus, empty states, hoverable rows, reveal-on-hover actions, and sidebar nav groups. Open that file once when you start a surface — the patterns are organized by visual element, not by view.

### Data-layer conventions

The mock encodes domain config in `design-mocks/src/data/mock.ts`. The frontend has analogous domain data — when porting:

- Use the *config-map* idiom — never inline a switch/ternary for label/color mapping. The mock and frontend keep these in different shapes; reach for whichever the surface needs:
  - **Warranty:** `WARRANTY_STATUS_CONFIG[status]` (`frontend/src/components/warranty/config.ts`) → `{ icon, i18nKey, text, bg, bgSolid, border }`. Resolve the label with `t(visual.i18nKey)`; `bgSolid` is the unbordered fill the dashboard breakdown bar wants, `bg` is the tinted chip surface.
  - **Commodity status:** `COMMODITY_STATUS_TONES[status]` (`frontend/src/features/commodities/constants.ts`) → a flat utility-string record. Pair with the `commodities:status.*` namespace for the label.
- Helper functions live alongside the config: `warrantyStatus(item)`, `areaLabel(id)`, `areaName(id)`. Reuse the equivalents in `frontend/src/features/`.
- Currencies: there is *one* canonical currency list (the mock has 30 entries in `CURRENCIES`). The `<CurrencyCombobox>` reads from it — don't pass a custom list.

## Step 4 — Adapt only what backend / router demands

The mock has no router (it uses a `view` state string in `App.tsx`) and no real backend (it has `mock.ts`). Frontend has both. Adaptation rules:

- **Routing**: `frontend/` uses `react-router-dom` (see `frontend/src/app/router.tsx` — `Routes`, `Route`, `Navigate`, `useNavigate`, `useLocation`, `useParams`). Replace the mock's `setView("x")` with a `react-router-dom` navigation — `useNavigate()` for imperative pushes, `<Navigate to="…">` for redirects, `<Link>` / `<NavLink>` for in-tree links. Keep the *callback shape* the view uses (`onNavigate?: (view: string) => void`) and resolve it to the right route inside the page wrapper. This isn't a deviation; it's translation.
- **Data**: Replace `MOCK_ITEMS.find(...)` with a query hook. The component shape stays identical: props in, JSX out, no global state coupling beyond router/i18n/theme.
- **i18n**: The mock has hardcoded English copy. The frontend wraps every visible string in a translation key. Map every visible string to `t("namespace:key")` and add it to `frontend/src/i18n/locales/<lang>/<namespace>.json`. Match the *exact* mock copy in the `en` locale; keep keys flat unless the namespace is heavily nested already.
- **Forms**: Mock uses local `useState`. Frontend uses RHF + Zod (`frontend/src/features/<x>/schemas/`). The visible markup must still match — Field/Label/Input order, spacing, save-button placement.
- **Auth/permissions**: The mock pretends every user has every right. Frontend gates real surfaces. Don't remove a control because of perms — render it disabled or hide it, depending on what `design-deviations.md` already established for that feature.

If the backend forces a divergence (e.g. an extra field, missing field, different cardinality), that's a logged deviation. Note it in `devdocs/frontend/design-deviations.md` using the entry template at the top of the file.

## Step 5 — Verify, then offer screenshots

Before declaring done:

1. **Side-by-side check.** With the mock view file open and the frontend file open, scan top to bottom for: header structure → spacing rhythm → component primitives → token use → icon set → copy structure. Five minutes of eyeballing catches 90% of drift.
2. **Run typecheck/tests.** `make test` from repo root or the frontend-specific scripts. Visual fidelity ≠ correctness — both must pass.
3. **Offer the screenshot pass.** This is the `screenshot-review` skill's job. Phrase it as one short line naming the surfaces you touched. Wait for explicit "yes." Mechanics live in [`screenshot-review`](../screenshot-review/SKILL.md).

## What this skill does NOT do

- It doesn't run tests or screenshots — see `inventario-e2e` and `screenshot-review`.
- It doesn't modify the mock — that's read-only, period.
- It doesn't write the deviation log entry for you — you do that in `devdocs/frontend/design-deviations.md` using the template at the top of that file.
- It doesn't replace `design-mocks/CLAUDE.md`. That file is the source of truth and includes longer prose on philosophy, structure, and the full visual-language summary. This skill is the *replication* lens. When in genuine doubt, the mock and `CLAUDE.md` win.

## Cross-references

- [`design-mocks/CLAUDE.md`](../../../design-mocks/CLAUDE.md) — full design contract written by the mock authors. Source of truth when this skill and it conflict.
- [`design-mocks/src/index.css`](../../../design-mocks/src/index.css) — the OKLCH values, `:root` and `.dark`. Open only when adding/syncing a token.
- [`design-mocks/src/views/UIShowcaseView.tsx`](../../../design-mocks/src/views/UIShowcaseView.tsx) — every primitive in catalog form. Fall back here when no view matches.
- [`devdocs/frontend/README.md`](../../../devdocs/frontend/README.md) — frontend operating manual (17 docs).
- [`devdocs/frontend/design/README.md`](../../../devdocs/frontend/design/README.md) — design-direction docs (22 files), the *why* behind the *what*.
- [`devdocs/frontend/design-deviations.md`](../../../devdocs/frontend/design-deviations.md) — append-only divergence log; entry template is at the top.
- [`.claude/skills/frontend-work/SKILL.md`](../frontend-work/SKILL.md) — the orchestrator skill (pre-flight, post-flight, screenshot offer).
- [`.claude/skills/screenshot-review/SKILL.md`](../screenshot-review/SKILL.md) — capture + review mechanics.
- [`AGENTS.md`](../../../AGENTS.md) — the read-only `design-mocks/` clause and other repo-wide rules.
