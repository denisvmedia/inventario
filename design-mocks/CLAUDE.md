# Inventario — Frontend Design & Engineering Guide

> Everything a developer (or AI agent) needs to continue this codebase in the same spirit: tokens, patterns, rules, decisions, and traps to avoid.

---

## Table of Contents

1. [Stack](#1-stack)
2. [Project Structure](#2-project-structure)
3. [Design Tokens & Theme](#3-design-tokens--theme)
4. [Typography](#4-typography)
5. [Spacing & Layout](#5-spacing--layout)
6. [Component Patterns](#6-component-patterns)
7. [View Architecture](#7-view-architecture)
8. [Forms](#8-forms)
9. [Data Layer](#9-data-layer)
10. [Accessibility](#10-accessibility)
11. [Do's & Don'ts](#11-dos--donts)
12. [Adding a New View](#12-adding-a-new-view)
13. [Adding a New Component](#13-adding-a-new-component)

---

## 1. Stack

| Layer | Technology |
|---|---|
| Framework | React 19 + TypeScript + Vite |
| Styling | Tailwind CSS v4 (OKLCH color space) |
| Components | shadcn/ui (new-york style), Radix UI primitives |
| Icons | Lucide React |
| Forms | React Hook Form + Zod |
| Charts | Recharts v3 via shadcn chart wrapper |
| Notifications | Sonner |
| Animation | tw-animate-css |

**Key invariants:**
- No `forwardRef` — use `React.ComponentProps<>` instead (Tailwind v4 pattern)
- No `hsl()` wrappers in CSS — raw OKLCH values only
- No `@tailwindcss/animate` — replaced by `tw-animate-css`
- No inline `style={}` for colors or spacing — always use Tailwind utilities

---

## 2. Project Structure

```
src/
├── components/
│   ├── ui/              # shadcn primitives — never edit directly unless extending
│   ├── AppSidebar.tsx   # main navigation shell
│   ├── AppLogo.tsx      # logo mark SVG + wordmark
│   ├── ItemDetail.tsx   # slide-over item detail panel
│   ├── WarrantyBadge.tsx
│   ├── CurrencyCombobox.tsx
│   └── ...
├── views/               # full-page views, one per route-like state
├── hooks/               # custom hooks
├── data/
│   └── mock.ts          # all types, enums, config maps, and seed data
├── lib/
│   └── utils.ts         # cn() utility
└── index.css            # SINGLE source of truth for all CSS variables
```

### Rules

- **One concern per file.** A view does layout + wiring. Business logic and data transformations belong in hooks or `mock.ts`.
- **Views live in `src/views/`**, named `<Feature>View.tsx`. They receive props for callbacks, no router dependency.
- **Shared non-ui components** live in `src/components/`. They are not views and not shadcn primitives.
- **Never add a new CSS file.** All styling flows through `index.css` tokens + Tailwind utilities.

---

## 3. Design Tokens & Theme

The design uses a **warm neutral** palette with **amber accents**. Colors are in OKLCH for perceptual uniformity across light/dark modes.

### Core Color Tokens

```css
/* src/index.css — :root */

/* Surfaces */
--background: oklch(0.985 0.004 75);   /* warm off-white page bg */
--foreground: oklch(0.18 0.012 60);    /* near-black text */
--card: oklch(1 0 0);                   /* pure white cards */
--card-foreground: oklch(0.18 0.012 60);
--popover: oklch(1 0 0);
--popover-foreground: oklch(0.18 0.012 60);

/* Brand */
--primary: oklch(0.26 0.02 60);        /* dark amber-tinted primary */
--primary-foreground: oklch(0.985 0.004 75);

/* Supporting */
--secondary: oklch(0.95 0.008 70);
--secondary-foreground: oklch(0.26 0.02 60);
--muted: oklch(0.945 0.008 70);
--muted-foreground: oklch(0.5 0.018 60);
--accent: oklch(0.85 0.12 75);         /* ← THE amber accent, use for highlights */
--accent-foreground: oklch(0.22 0.04 60);
--destructive: oklch(0.577 0.245 27.325);
--destructive-foreground: oklch(0.985 0 0);

/* Structural */
--border: oklch(0.9 0.008 70);
--input: oklch(0.9 0.008 70);
--ring: oklch(0.65 0.08 75);           /* amber focus ring */
--radius: 0.5rem;
```

### Domain-Specific Tokens

These go beyond shadcn defaults — they encode business semantics:

```css
/* Warranty / inventory status */
--status-active:   oklch(0.72 0.17 145);   /* green  — in use / valid */
--status-expiring: oklch(0.78 0.18 75);    /* amber  — within 60 days */
--status-expired:  oklch(0.65 0.22 25);    /* red    — past due */
--status-none:     oklch(0.6 0 0);         /* gray   — no warranty */

/* Data visualization */
--chart-1: oklch(0.7 0.16 75);    /* amber */
--chart-2: oklch(0.65 0.14 145);  /* green */
--chart-3: oklch(0.6 0.14 220);   /* blue */
--chart-4: oklch(0.75 0.18 55);   /* warm yellow */
--chart-5: oklch(0.62 0.18 25);   /* red */

/* Sidebar surface — distinct from page bg */
--sidebar: oklch(0.97 0.006 70);
--sidebar-foreground: oklch(0.18 0.012 60);
--sidebar-primary: oklch(0.26 0.02 60);
--sidebar-primary-foreground: oklch(0.985 0.004 75);
--sidebar-accent: oklch(0.92 0.012 70);
--sidebar-accent-foreground: oklch(0.26 0.02 60);
--sidebar-border: oklch(0.9 0.008 70);
--sidebar-ring: oklch(0.65 0.08 75);
```

### Dark Mode

Dark mode is toggled via the `.dark` class on `<html>`. All tokens automatically switch:

```css
/* .dark — key values */
--background: oklch(0.155 0.01 55);
--foreground: oklch(0.96 0.006 70);
--card: oklch(0.195 0.012 55);
--primary: oklch(0.88 0.09 75);         /* light amber in dark */
--accent: oklch(0.72 0.14 75);
--border: oklch(1 0 0 / 10%);           /* white at 10% opacity */
--input: oklch(1 0 0 / 12%);
--ring: oklch(0.72 0.1 75);
```

> **Rule:** Never hardcode a color anywhere. If a token doesn't exist, add it to `index.css` with both `:root` and `.dark` values, then register it in `@theme inline`.

### Radius Tokens

| Token | Value | Use |
|---|---|---|
| `--radius-sm` | `calc(var(--radius) - 4px)` = 0.125rem | Small chips, badges |
| `--radius-md` | `calc(var(--radius) - 2px)` = 0.25rem | Inputs, small cards |
| `--radius-lg` | `var(--radius)` = 0.5rem | Cards, modals (default) |
| `--radius-xl` | `calc(var(--radius) + 4px)` = 0.75rem | Large containers |

Use via Tailwind: `rounded-sm`, `rounded-md`, `rounded-lg`, `rounded-xl`, `rounded-full`.

---

## 4. Typography

shadcn provides no default element styles. Apply these Tailwind classes explicitly:

| Role | Classes |
|---|---|
| Page title (h1) | `scroll-m-20 text-3xl font-semibold tracking-tight` |
| Section heading (h2) | `text-base font-semibold` |
| Sub-heading (h3) | `text-sm font-semibold` |
| Body | `text-sm leading-relaxed` |
| Muted / secondary | `text-sm text-muted-foreground` |
| Label / overline | `text-xs font-semibold uppercase tracking-widest text-muted-foreground` |
| Stat value | `text-2xl font-bold tracking-tight` |
| Stat label | `text-xs font-medium uppercase tracking-wide text-muted-foreground` |
| Code / mono | `font-mono text-xs` |

### Patterns in Use

```tsx
{/* Page heading — every view starts with this */}
<h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">Tags</h1>
<p className="mt-1 text-muted-foreground">Organise your inventory with custom labels.</p>

{/* Section within a card */}
<h2 className="text-base font-semibold">Notifications</h2>
<p className="text-sm text-muted-foreground mt-0.5">Push and email preferences.</p>

{/* Overline label (used above stat rows, section dividers) */}
<p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">Preview</p>
```

---

## 5. Spacing & Layout

### Page Wrapper

Every view uses this outer shell:

```tsx
<div className="flex flex-col gap-6 p-6 max-w-2xl mx-auto w-full">
  {/* content */}
</div>
```

- `p-6` — consistent page padding
- `max-w-2xl` for settings/detail pages, `max-w-4xl` for list/data pages
- `mx-auto w-full` — centered, full-width up to max

### Card Shell

```tsx
<div className="rounded-xl border border-border bg-card p-6 space-y-5">
  {/* card content */}
</div>
```

### Stats Row

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

### Divide List (settings-style rows)

```tsx
<div className="divide-y divide-border">
  <div className="flex items-center justify-between py-3.5">
    <p className="text-sm font-medium">Row label</p>
    <Switch checked={val} onCheckedChange={setVal} />
  </div>
</div>
```

### Icon + Content Pattern

Reused across dialogs, list items, and onboarding:

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

### Responsive Grid

```tsx
<div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
  {/* stat cards */}
</div>
```

---

## 6. Component Patterns

### Button

```tsx
import { Button } from "@/components/ui/button"

<Button>Default</Button>
<Button variant="outline" size="sm">Small outline</Button>
<Button variant="ghost" size="icon"><Plus className="size-4" /></Button>
<Button variant="destructive" size="sm" className="gap-1.5">
  <Trash2 className="size-3.5" />
  Delete
</Button>
```

**Variants:** `default` | `outline` | `secondary` | `ghost` | `destructive` | `link`
**Sizes:** `default` (h-9) | `sm` (h-8) | `lg` (h-10) | `xs` (h-6) | `icon` (size-9) | `icon-sm` (size-8) | `icon-xs` (size-6)

**Icon sizing by button size:**
- `size="default"` → icon `size-4`
- `size="sm"` → icon `size-3.5`
- `size="xs"` → icon `size-3`
- `size="icon"` → icon `size-4`

### Badge

```tsx
import { Badge } from "@/components/ui/badge"

<Badge>Default</Badge>
<Badge variant="secondary">Secondary</Badge>
<Badge variant="outline">Outline</Badge>
```

**Status Badge Pattern (domain-specific):**
```tsx
<Badge variant="outline" className={`${config.color} ${config.bg} border-current/20 font-medium`}>
  <Icon className="size-3" />
  {config.label}
</Badge>
```

Where `config` comes from `WARRANTY_STATUS_CONFIG` or `COMMODITY_STATUS_CONFIG` in `mock.ts`.

### Input

```tsx
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

<div className="space-y-1.5">
  <Label htmlFor="field-id">Label</Label>
  <Input id="field-id" placeholder="Enter value…" value={val} onChange={(e) => setVal(e.target.value)} />
</div>
```

### Select

```tsx
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

<Select value={value} onValueChange={setValue}>
  <SelectTrigger><SelectValue /></SelectTrigger>
  <SelectContent>
    <SelectItem value="option">Option</SelectItem>
  </SelectContent>
</Select>
```

### CurrencyCombobox

This project-level component replaces plain `<Select>` for currency fields. It has search, full currency names, and symbols:

```tsx
import { CurrencyCombobox } from "@/components/CurrencyCombobox"

<CurrencyCombobox value={currency} onValueChange={setCurrency} />
<CurrencyCombobox value={currency} onValueChange={setCurrency} variant="compact" />
```

### Dialog

```tsx
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "@/components/ui/dialog"

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

### AlertDialog (destructive confirmation)

```tsx
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog"

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

### Dropdown Menu

```tsx
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"

<DropdownMenu>
  <DropdownMenuTrigger asChild>
    <Button variant="ghost" size="icon"><MoreHorizontal className="size-4" /></Button>
  </DropdownMenuTrigger>
  <DropdownMenuContent align="end">
    <DropdownMenuItem>Action</DropdownMenuItem>
    <DropdownMenuSeparator />
    <DropdownMenuItem className="text-destructive focus:text-destructive">
      <Trash2 className="size-4 mr-2" />Delete
    </DropdownMenuItem>
  </DropdownMenuContent>
</DropdownMenu>
```

### Empty State

```tsx
{/* Inline empty (in a list) */}
<div className="flex flex-col items-center justify-center gap-3 py-16">
  <Icon className="size-8 text-muted-foreground/30" />
  <p className="text-sm text-muted-foreground">Nothing here yet.</p>
</div>

{/* Full-page empty (EmptyStatesView exports) */}
import { NoLocationView, NoAreaView } from "@/views/EmptyStatesView"
```

### WarrantyBadge

```tsx
import { WarrantyBadge } from "@/components/WarrantyBadge"

<WarrantyBadge item={item} />
<WarrantyBadge item={item} showIcon={false} />
```

### Tag Pill

The `TagsView` defines a `TagPill` pattern that can be extracted if needed:
```tsx
<span className="inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-xs font-medium select-none bg-chart-1/15 text-chart-1 border-chart-1/30">
  <Hash className="size-2.5 shrink-0" />
  kitchen
</span>
```

### Switch Row (settings)

```tsx
<div className="flex items-center justify-between py-3.5">
  <p className="text-sm font-medium">Warranty expiring alerts</p>
  <Switch checked={val} onCheckedChange={setVal} />
</div>
```

### List Row with Chevron Navigation

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

---

## 7. View Architecture

### State Machine

All navigation is managed in `App.tsx` via a `view` state string — no router. Views are rendered conditionally:

```tsx
{view === "tags" && <TagsView />}
```

Adding a new view requires:
1. Adding to the `View` union type
2. Adding to `VIEW_TITLES` and `VIEW_ICONS`
3. Importing and rendering in `App.tsx`
4. Adding `SidebarMenuItem` in `AppSidebar.tsx` (if it should be in nav)

### View Props Convention

Views receive only what they need via explicit props. No context consumers unless it's `ThemeProvider` or `SidebarProvider`:

```tsx
interface MyFeatureViewProps {
  activeGroupId: string
  onNavigate?: (view: string) => void
  onItemClick?: (id: string) => void
}
```

### Fullscreen Views

`auth`, `image-viewer`, `pdf-viewer`, `insurance-report` render outside the sidebar shell. They are listed in `fullscreenViews` in `App.tsx` and render directly without `SidebarInset`.

---

## 8. Forms

### Field Pattern

```tsx
<div className="space-y-1.5">
  <label className="text-sm font-medium">Field label</label>
  <Input value={val} onChange={(e) => setVal(e.target.value)} placeholder="…" />
</div>
```

For forms with validation, use the `Field` + `FieldLabel` + `FieldError` components from shadcn:

```tsx
import { Field, FieldLabel, FieldError } from "@/components/ui/field"

<Field data-invalid={fieldState.invalid}>
  <FieldLabel htmlFor={field.name}>Title</FieldLabel>
  <Input {...field} id={field.name} aria-invalid={fieldState.invalid} />
  {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
</Field>
```

### Save Button Placement

Always in a `<div className="pt-2">` at the bottom of the form section:

```tsx
<div className="pt-2">
  <Button size="sm">Save changes</Button>
</div>
```

---

## 9. Data Layer

### Types (src/data/mock.ts)

All shared types live here. Never redefine locally what's already in `mock.ts`.

```ts
// Status enums
type WarrantyStatus = "active" | "expiring" | "expired" | "none"
type CommodityStatus = "in_use" | "sold" | "lost" | "disposed" | "written_off"
type ItemCategory = "appliance" | "electronics" | "tool" | "furniture" | "vehicle" | "other"
type MemberRole = "admin" | "user"
type FileCategory = "image" | "invoice" | "document" | "other"
```

### Config Maps

These encode the mapping from type value → display properties. Always use them instead of inline ternaries:

```ts
WARRANTY_STATUS_CONFIG[status].label   // "Active" | "Expiring Soon" | "Expired" | "No Warranty"
WARRANTY_STATUS_CONFIG[status].color   // "text-status-active" | ...
WARRANTY_STATUS_CONFIG[status].bg      // "bg-status-active/10" | ...

COMMODITY_STATUS_CONFIG[status].label
COMMODITY_STATUS_CONFIG[status].color
COMMODITY_STATUS_CONFIG[status].bg
COMMODITY_STATUS_CONFIG[status].description
```

### Helper Functions

```ts
import { warrantyStatus, areaLabel, areaName } from "@/data/mock"

warrantyStatus(item)          // WarrantyStatus — computed from today
areaLabel("area-id")          // "Kitchen · Cabinet" — location · area
areaName("area-id")           // "Cabinet"
```

---

## 10. Accessibility

- **Focus rings:** Every interactive element gets `focus-visible:ring-[3px] focus-visible:ring-ring/50` — this is baked into shadcn components. Don't override with `outline-none` without replacing with `focus-visible:` equivalent.
- **Icon-only buttons** need an `aria-label` or `title`:
  ```tsx
  <Button size="icon" aria-label="Delete item"><Trash2 className="size-4" /></Button>
  ```
- **Form labels:** Always use `<label htmlFor>` or `<Label htmlFor>` linked to the input `id`.
- **Disabled state:** Use `disabled` prop on buttons/inputs — never fake-disable with opacity alone.
- **Destructive color:** Never convey error state through color alone. Pair with icon + text.
- **Tooltips on icon buttons:** Use `<Tooltip>` for icon-only buttons in non-obvious positions.

---

## 11. Do's & Don'ts

### Colors

| Do | Don't |
|---|---|
| `text-status-active` | `text-green-500` |
| `bg-chart-1/15 text-chart-1` | `bg-amber-100 text-amber-700` |
| `text-destructive` | `text-red-500` |
| `oklch(0.7 0.16 75)` in index.css | `#f59e0b` anywhere |
| `border-current/20` for badge borders | `border-amber-200` |

### Spacing

| Do | Don't |
|---|---|
| `gap-6` between sections | `mt-6` then `mb-6` |
| `space-y-4` inside cards | `my-4` on each child |
| `py-3.5` for list rows | `py-3 md:py-4` |
| `px-4 py-3` for stat cards | custom padding per card |

### Components

| Do | Don't |
|---|---|
| `<Button variant="outline">` | `<button className="border rounded …">` |
| `<Input>` from ui/ | `<input className="border rounded …">` |
| `<AlertDialog>` for destructive confirm | `window.confirm()` |
| `<CurrencyCombobox>` for currency fields | `<Select>` with hardcoded currencies |
| `<WarrantyBadge item={item}>` | inline status badge from scratch |
| `cn(base, conditional)` | template literals for className |

### Icons

| Do | Don't |
|---|---|
| `size-4` on standard body icons | `w-4 h-4` |
| `size-3.5` on sm button icons | `w-3 h-3` |
| `size-3` on xs/badge icons | mismatched icon sizes |
| `text-muted-foreground` on decorative icons | `text-gray-400` |
| Icons from `lucide-react` | Other icon libraries |

### Structure

| Do | Don't |
|---|---|
| View files in `src/views/` | Inline views as functions in App.tsx |
| `MOCK_ITEMS.find(...)` for data access | Copying mock data into components |
| Separate file per view | Views > 300 lines without good reason |
| Export named functions only | Default exports from view files |

### Dependencies

| Do | Don't |
|---|---|
| `radix-ui` (umbrella) for all UI primitives | `@base-ui/react` — **never add this**; it is not in package.json and must stay out |
| `cmdk` for `<Command>` / searchable lists | Any other headless combobox library |
| Add new primitives only from `radix-ui` or shadcn CLI | Pull in a competing primitive library for a single use case |

> **Why no `@base-ui/react`:** The project uses `radix-ui` (the official shadcn/new-york primitive set). `@base-ui/react` is a separate, incompatible library that was considered but never wired. Keeping both would create two conflicting primitive sources, inflate the bundle, and confuse future contributors. If a specific primitive is ever needed that `radix-ui` cannot provide, file an explicit decision record here before installing anything.

### Interactions

| Do | Don't |
|---|---|
| `transition-colors` on hover states | No transition on interactive elements |
| `opacity-0 group-hover:opacity-100` for reveal actions | Always-visible action buttons cluttering rows |
| `disabled={!value.trim()}` on submit buttons | Let empty form submit |
| `onOpenChange={(o) => !o && onClose()}` for dialogs | Custom close logic without handling backdrop click |

---

## 12. Adding a New View

### Step 1 — Create the view file

```tsx
// src/views/MyFeatureView.tsx

interface MyFeatureViewProps {
  // explicit props only
}

export function MyFeatureView({ }: MyFeatureViewProps) {
  return (
    <div className="flex flex-col gap-6 p-6 max-w-2xl mx-auto w-full">
      <div>
        <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">My Feature</h1>
        <p className="mt-1 text-muted-foreground">Short description.</p>
      </div>
      {/* content */}
    </div>
  )
}
```

### Step 2 — Register in App.tsx

```tsx
// Add to View type
type View = ... | "my-feature"

// Add to VIEW_TITLES
const VIEW_TITLES: Record<View, string> = {
  ...
  "my-feature": "My Feature",
}

// Add to VIEW_ICONS
const VIEW_ICONS: Record<View, React.ElementType> = {
  ...
  "my-feature": FeatureIcon,
}

// Import the view
import { MyFeatureView } from "@/views/MyFeatureView"

// Render it
{view === "my-feature" && <MyFeatureView />}
```

### Step 3 — Add to sidebar (if navigable)

In `AppSidebar.tsx`, add to the appropriate nav group array:

```tsx
const MANAGE_ITEMS = [
  ...
  { id: "my-feature", label: "My Feature", icon: FeatureIcon },
]
```

---

## 13. Adding a New Component

### When to create a new component

- Logic or markup is used in 2+ places
- A view's render section exceeds ~80 lines and has a clear sub-unit
- A domain concept needs a consistent visual representation (like `WarrantyBadge`)

### Template

```tsx
// src/components/MyThing.tsx

import { cn } from "@/lib/utils"

interface MyThingProps {
  value: string
  className?: string
}

export function MyThing({ value, className }: MyThingProps) {
  return (
    <div className={cn("rounded-lg border border-border bg-card px-3 py-2", className)}>
      {value}
    </div>
  )
}
```

### Rules

- **Accept `className?`** for layout flexibility at call sites.
- **Use `cn()`** to merge it with base classes.
- **Don't accept `style`** prop unless absolutely necessary (e.g. dynamic values from data).
- **Export only named functions** — no default exports.
- **Keep it dumb** — no internal data fetching, no global state. Props in, JSX out.

---

## Visual Language Summary

The Inventario UI is **warm, precise, and understated.** The amber accent is reserved for moments that matter — primary actions, focus states, active highlights. Everything else defers to the neutral warm-gray palette.

**What makes it feel right:**
- Card-on-background elevation (white card on off-white bg)
- Status colors that are never used decoratively — only when they carry semantic meaning
- Generous padding inside cards (p-6) but tight list row spacing (py-3.5)
- Icon sizes that match their context (size-4 in body, size-3.5 in small/tight UI, size-8+ in hero)
- Hover states that reveal rather than transform — opacity reveals, subtle bg fills
- Consistent overline labels (`text-xs uppercase tracking-widest`) before grouped content

**What breaks it:**
- Using `text-green-*` instead of `text-status-active`
- Purple, indigo, or violet — this theme has no purple. Never add it.
- Drop shadows — this UI uses borders, not shadows (except `shadow-xs` on inputs)
- Full-black text — foreground is `oklch(0.18)`, not pure black
- Inline styles for any color
