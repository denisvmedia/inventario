---
name: design-mock-fidelity
description: Use this skill at the moment you sit down to replicate or extend a UI surface in `frontend/` against `design-mocks/`. It is the dense, paste-ready playbook for hitting 1:1 fidelity with the mock without re-deriving spacing, tokens, or component anatomy from scratch. Activates whenever the agent is about to author markup, classNames, or component composition for `frontend/src/**` AND a corresponding (or analogous) surface exists in `design-mocks/src/**`. Companion to `frontend-work`: that skill orchestrates pre-flight / post-flight; this skill carries the replication recipes and the surface-index that lets you open exactly one mock file instead of grepping. Skip when the surface is purely backend/test/types or when there is no visual analogue in the mock at any altitude.
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

1. **`design-mocks/` is read-only.** Do not Edit, Write, move, rename, or `git add` anything under that path. It's a sync mirror of an upstream repo and your edits get wiped. If something looks wrong *in the mock*, surface it verbally to the user — it gets fixed upstream.
2. **Default fidelity is 1:1.** Same DOM structure, same Tailwind classes, same icon imports, same copy structure. Drift is a deliberate, logged decision — see `devdocs/frontend/design-deviations.md` and the entry template at the top of that file.
3. **Tokens, never raw colors.** No `text-amber-500`, `#f59e0b`, `bg-green-100`, etc. The mock uses semantic tokens (`text-status-active`, `bg-chart-1/15`, `text-destructive`). If a needed token doesn't exist, it gets added to `frontend/src/index.css` in both `:root` and `.dark`, registered in `@theme inline`, and matches the OKLCH values in `design-mocks/src/index.css`.
4. **No `forwardRef`, no `@tailwindcss/animate`, no `hsl()` in token *definitions*.** Tailwind v4 uses `React.ComponentProps<>` and `tw-animate-css`. Token definitions in `index.css` are raw OKLCH with no `hsl()` wrapper. The one place where `hsl(var(--…))` legitimately appears is the shadcn sidebar's `<SidebarRail>` outline trick (`shadow-[0_0_0_1px_hsl(var(--sidebar-border))]`, mirrored in `frontend/src/components/ui/sidebar.tsx`) — leave that alone when porting; don't introduce new `hsl()` wrappers anywhere else.
5. **No purple, no indigo, no violet anywhere.** This palette is warm-neutral + amber. Any chart of more than five series uses the chart-1..5 cycle, not new hues.
6. **Don't add elevation shadows on top of shadcn primitives.** shadcn primitives already ship the shadows the design language calls for: `card.tsx` → `shadow-sm`, `dialog.tsx` / `sheet.tsx` → `shadow-lg`, `popover.tsx` / `dropdown-menu.tsx` → `shadow-md`–`shadow-lg`, sidebar floating/inset → `shadow-sm`, sidebar rail → an inset 1px `shadow-[…]` outline. Keep those as-is for 1:1 fidelity. The rule is *don't paint extra `shadow-*` onto your own surfaces*: bespoke cards, custom containers, hand-rolled chips, list rows — those use borders + the token palette, not new shadows. If a primitive already has a shadow and the mock keeps it, you keep it too.

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
| Component catalog (when nothing else fits) | `views/UIShowcaseView.tsx` (1379 lines, every primitive) |

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
| Item slide-over detail panel | `components/ItemDetail.tsx` (982 lines — read sections you need) |
| File detail panel | `components/FileDetail.tsx` |
| File preview dialog | `components/FilePreviewDialog.tsx` |
| Add item flow | `components/AddItemDialog.tsx` (1504 lines — wizard pattern) |
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

**Badge — neutral**:

```tsx
<Badge>Default</Badge>
<Badge variant="secondary">Secondary</Badge>
<Badge variant="outline">Outline</Badge>
```

**Badge — status (domain pattern)**:

The mock uses `WARRANTY_STATUS_CONFIG[status].color / .bg / .label` and the same shape for commodity status. The frontend has *split* these into two differently-shaped constants:

- **Warranty:** `WARRANTY_STATUS_CONFIG` in `frontend/src/components/warranty/config.ts` — fields are `{ i18nKey, icon, text, bg, bgSolid, border }`. Note `text` (not `color`) for the foreground class, and `i18nKey` instead of a literal label so the chip is translatable.
- **Commodity:** `COMMODITY_STATUS_TONES` in `frontend/src/features/commodities/constants.ts` — a flat `Record<status, string>` of pre-joined utility strings (`"text-status-active border-status-active/30 bg-status-active/10"`), no separate fields. Pair with the `commodities:status.*` i18n namespace for the label.

Reach for the existing `WarrantyBadge` component for warranty surfaces — don't compose a chip from scratch. For commodity status, the canonical pattern in the frontend is:

```tsx
const tone = status ? COMMODITY_STATUS_TONES[status] : ""
<Badge variant="outline" className={cn(tone, "border-current/20 font-medium gap-1")}>
  <Icon className="size-3" />
  {t(`commodities:status.${status}`)}
</Badge>
```

For warranty status outside `WarrantyBadge` (e.g. dashboard breakdowns), pull from `WARRANTY_STATUS_CONFIG[status]`:

```tsx
const visual = WARRANTY_STATUS_CONFIG[status]
const Icon = visual.icon
<Badge variant="outline" className={cn(visual.text, visual.bg, visual.border, "font-medium gap-1")}>
  <Icon className="size-3" />
  {t(visual.i18nKey)}
</Badge>
```

**Tag pill** (with `lucide` `Hash` glyph, color from `chart-*` cycle):

```tsx
<span className="inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-xs font-medium select-none bg-chart-1/15 text-chart-1 border-chart-1/30">
  <Hash className="size-2.5 shrink-0" />
  kitchen
</span>
```

**Button — sizes & icon scale** (icon size MUST match button size):

| Button size | Class | Icon size |
|---|---|---|
| `default` | `h-9` | `size-4` |
| `sm` | `h-8` | `size-3.5` |
| `lg` | `h-10` | `size-4` |
| `xs` | `h-6` | `size-3` |
| `icon` | `size-9` | `size-4` |
| `icon-sm` | `size-8` | `size-3.5` |
| `icon-xs` | `size-6` | `size-3` |

```tsx
<Button>Default</Button>
<Button variant="outline" size="sm">Small outline</Button>
<Button variant="ghost" size="icon" aria-label="Add"><Plus className="size-4" /></Button>
<Button variant="destructive" size="sm" className="gap-1.5">
  <Trash2 className="size-3.5" />
  Delete
</Button>
```

**Field with label** (no validation):

```tsx
<div className="space-y-1.5">
  <Label htmlFor="field-id">Label</Label>
  <Input id="field-id" placeholder="Enter value…" value={val} onChange={(e) => setVal(e.target.value)} />
</div>
```

**Field with validation** (RHF + Zod, the actual frontend pattern — see `devdocs/frontend/forms.md` and any `pages/*Page.tsx`).

The repo does not have shadcn `Field`/`FieldLabel`/`FieldError` primitives. Schemas live in `frontend/src/features/<name>/schemas.ts` and their `message` fields are *i18n keys* (`"auth:validation.emailRequired"`), not English strings — the page resolves them with `t()` at render time. The field shape:

```tsx
<div className="space-y-1.5">
  <Label htmlFor="profile-name">{t("auth:fields.name")}</Label>
  <Input
    id="profile-name"
    aria-invalid={!!form.formState.errors.name}
    {...form.register("name")}
  />
  {form.formState.errors.name ? (
    <p className="field-error text-xs text-destructive">
      {t(form.formState.errors.name.message ?? "")}
    </p>
  ) : null}
</div>
```

`field-error` is a class hook for tests / styling overrides; keep it on every error `<p>`. Pair with `data-testid` on both the input and the error node when the form has tests.

**Save button placement** (always at the bottom of a form section, in `pt-2`):

```tsx
<div className="pt-2">
  <Button size="sm">Save changes</Button>
</div>
```

**Dialog** (icon-headed title is the house style):

```tsx
<Dialog open={open} onOpenChange={(o) => !o && onClose()}>
  <DialogContent className="sm:max-w-md">
    <DialogHeader>
      <DialogTitle className="flex items-center gap-2">
        <div className="flex size-7 items-center justify-center rounded-lg bg-primary/10">
          <Icon className="size-4 text-primary" />
        </div>
        Dialog Title
      </DialogTitle>
      <DialogDescription>Supporting text.</DialogDescription>
    </DialogHeader>
    {/* body */}
    <DialogFooter className="gap-2">
      <Button variant="outline" onClick={onClose}>Cancel</Button>
      <Button onClick={onConfirm}>Confirm</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
```

**AlertDialog (destructive)** — never `window.confirm`:

```tsx
<AlertDialog open={!!deleteId} onOpenChange={(o) => !o && setDeleteId(null)}>
  <AlertDialogContent>
    <AlertDialogHeader>
      <AlertDialogTitle>Delete item</AlertDialogTitle>
      <AlertDialogDescription>This action cannot be undone.</AlertDialogDescription>
    </AlertDialogHeader>
    <AlertDialogFooter>
      <AlertDialogCancel>Cancel</AlertDialogCancel>
      <AlertDialogAction
        onClick={handleDelete}
        className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
      >
        Delete
      </AlertDialogAction>
    </AlertDialogFooter>
  </AlertDialogContent>
</AlertDialog>
```

**Dropdown menu** (with destructive-tinted item):

```tsx
<DropdownMenu>
  <DropdownMenuTrigger asChild>
    <Button variant="ghost" size="icon" aria-label="Actions"><MoreHorizontal className="size-4" /></Button>
  </DropdownMenuTrigger>
  <DropdownMenuContent align="end">
    <DropdownMenuItem>Edit</DropdownMenuItem>
    <DropdownMenuSeparator />
    <DropdownMenuItem className="text-destructive focus:text-destructive">
      <Trash2 className="size-4 mr-2" />Delete
    </DropdownMenuItem>
  </DropdownMenuContent>
</DropdownMenu>
```

**Empty state — inline** (in a list/card):

```tsx
<div className="flex flex-col items-center justify-center gap-3 py-16">
  <Icon className="size-8 text-muted-foreground/30" />
  <p className="text-sm text-muted-foreground">Nothing here yet.</p>
</div>
```

**Empty state — full-page**: use the named exports from `design-mocks/src/views/EmptyStatesView.tsx` as the recipe. Patterns: `NotFoundView`, `NoLocationGroupView`, `NoGroupOnboardingView`, `NoLocationView`, `NoAreaView`, `MaintenanceView`. Don't compose your own — port the matching one.

**Hoverable list row** (Dashboard "Expiring Warranties" pattern):

```tsx
<button
  className="flex w-full items-center justify-between px-6 py-3.5 text-left transition-colors hover:bg-muted/50"
  onClick={() => onItemClick(item.id)}
>
  <div>
    <p className="text-sm font-medium">{item.name}</p>
    <p className="text-xs text-muted-foreground">{item.brand} · {areaName(item.areaId)}</p>
  </div>
  <Badge variant="outline" className="text-status-expiring bg-status-expiring/10 border-current/20 shrink-0 ml-4">
    {days} days left
  </Badge>
</button>
```

**Reveal-on-hover actions** (rows that show extra controls only when hovered):

```tsx
<div className="group flex items-center justify-between py-3 hover:bg-muted/40">
  <span className="text-sm">Item</span>
  <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
    <Button size="icon-sm" variant="ghost" aria-label="Edit"><Pencil className="size-3.5" /></Button>
    <Button size="icon-sm" variant="ghost" aria-label="Delete"><Trash2 className="size-3.5" /></Button>
  </div>
</div>
```

**Sidebar nav group** (the canonical AppSidebar pattern):

Open `design-mocks/src/components/AppSidebar.tsx` and copy the `<SidebarGroup>` block — that file is short (~260 lines) and you'll want to read it in full for the user-dropdown footer + rail + collapsed-mode classes (`group-data-[collapsible=icon]:*`).

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

## Drift markers — catch yourself

Stop if you find yourself doing any of these. Each one is a sign you've slipped from the contract:

- **Picking a Tailwind color name.** `text-amber-500`, `bg-green-100`, `border-red-300`. Always tokens.
- **Adding a bespoke shadow.** shadcn primitives already ship the right elevations (`card.tsx` `shadow-sm`, `dialog.tsx` / `sheet.tsx` `shadow-lg`, `popover.tsx` / `dropdown-menu.tsx` `shadow-md`–`shadow-lg`, sidebar floating/inset `shadow-sm`, sidebar rail's inset 1px outline, `input.tsx` `shadow-xs`) — keep them. The drift signal is decorating *your own* surfaces with elevation: hand-rolled cards/chips/list rows. Those use borders + tokens, no extra `shadow-*`.
- **Inventing a new gap or padding.** If `p-6 / gap-6 / py-3.5 / space-y-5` doesn't fit, you're mismatching the section type. Re-identify the pattern.
- **`forwardRef`, `@tailwindcss/animate`, or `hsl()` in token *definitions*.** All three are banned. The narrow exception for `hsl(var(--…))` is the sidebar-rail outline (`shadow-[0_0_0_1px_hsl(var(--sidebar-border))]`) — preserve that when you see it; don't add `hsl()` wrappers anywhere else, and never in `index.css` token declarations.
- **`window.confirm`, `alert()`, `prompt()`.** Use `<AlertDialog>`, `<Dialog>`, sonner toasts.
- **Default-export from a view.** Named exports only.
- **Inline `style={{ … }}` for arbitrary visual choices.** Static colors, paddings, font sizes, radii — those belong in Tailwind utilities or `index.css` tokens. Inline styles are legitimate only when a *runtime value* has to flow into CSS that utilities can't express: data-driven geometry/dimensions (`width: ${pct}%`, `height: ${dim}`), data-driven colors that pull from a token (`backgroundColor: var(--status-${status})`), background images from runtime URLs, transforms in zoom/pan UIs, and shadcn primitives that take CSS vars via `style` (e.g. `<SidebarProvider style={{ "--sidebar-width": "16rem" }}>`). The dashboard breakdown bar in `views/DashboardView.tsx:236` does both at once — width and a token-derived background — and that's the canonical example. If your inline style isn't pulling a runtime value through, it's the wrong tool.
- **A new icon library.** `lucide-react` is the only one. Pick the closest glyph or change the metaphor; never add a second library.
- **Substituting a primitive.** No "I'll use `react-select` instead of `<Combobox>` because…". The primitive set is fixed: shadcn/ui (`new-york` style) + Radix via the `radix-ui` umbrella + `cmdk` for command palettes. New primitives only via `radix-ui` or the shadcn CLI.
- **Re-rolling a domain component from scratch.** Reach for `WarrantyBadge`, `CurrencyCombobox` etc. before composing markup yourself.
- **Hardcoded English copy in `frontend/`.** Every visible string is `t("…")`-wrapped, even prototype labels.
- **A new CSS file.** All styling lives in Tailwind utilities + the single `index.css` token sheet. There is no second stylesheet.

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
