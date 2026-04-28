# Space & Layout

The single biggest delta between "wireframe" and "designed product" is spacing discipline. This document specifies every value.

## Base unit

```
1 base unit = 0.25rem = 4px
```

All spacing, sizing, and radii are integer multiples of the base unit. No magic numbers in CSS — if you find yourself typing `padding: 13px`, you're wrong.

## Spacing scale

A 13-step scale covering everything from icon padding to page-section gaps. T-shirt sizes for memorability, with explicit pixel values.

```css
@theme {
  --space-0:   0;
  --space-px:  1px;        /* hairlines */
  --space-0_5: 0.125rem;   /*  2px */
  --space-1:   0.25rem;    /*  4px */
  --space-1_5: 0.375rem;   /*  6px */
  --space-2:   0.5rem;     /*  8px */
  --space-3:   0.75rem;    /* 12px */
  --space-4:   1rem;       /* 16px */
  --space-5:   1.25rem;    /* 20px */
  --space-6:   1.5rem;     /* 24px */
  --space-8:   2rem;       /* 32px */
  --space-10:  2.5rem;     /* 40px */
  --space-12:  3rem;       /* 48px */
  --space-16:  4rem;       /* 64px */
  --space-20:  5rem;       /* 80px */
  --space-24:  6rem;       /* 96px */
  --space-32:  8rem;       /* 128px */
}
```

## Spacing semantics

To prevent inconsistency, use **semantic spacing tokens** at the component level. Components reference these, never raw values.

```css
/* Component-internal spacing */
--gap-component-tight:    var(--space-1);    /* between icon and label inside a button */
--gap-component-default:  var(--space-2);    /* between adjacent inline elements */
--gap-component-relaxed:  var(--space-3);    /* between form field and label */

/* Inside-of-card spacing */
--padding-card-sm:  var(--space-3);   /* compact list item */
--padding-card:     var(--space-5);   /* standard card content padding */
--padding-card-lg:  var(--space-8);   /* hero card / dashboard widget */

/* Between-component spacing */
--gap-stack-tight:    var(--space-2);    /* tightly grouped */
--gap-stack-default:  var(--space-4);    /* default vertical rhythm */
--gap-stack-relaxed:  var(--space-6);    /* loose, breathing */
--gap-stack-section:  var(--space-12);   /* between major page sections */

/* Page-level spacing */
--padding-page-x:        var(--space-6);    /* desktop ≥1024 */
--padding-page-x-mobile: var(--space-4);    /* mobile <640 */
--padding-page-y-top:    var(--space-8);
--padding-page-y-bottom: var(--space-16);
```

## Border radius scale

7 steps, geometric progression. **No 4px-everywhere shadcn default** — that flattens the whole product. Different surfaces deserve different roundness.

```css
@theme {
  --radius-none: 0;
  --radius-xs:   0.25rem;   /*  4px — badges, chips, tags */
  --radius-sm:   0.375rem;  /*  6px — small buttons, inline controls */
  --radius-md:   0.5rem;    /*  8px — buttons, inputs, list items */
  --radius-lg:   0.75rem;   /* 12px — cards, panels */
  --radius-xl:   1rem;      /* 16px — large dialogs, hero surfaces */
  --radius-2xl:  1.5rem;    /* 24px — only for marketing/empty-state surfaces */
  --radius-full: 9999px;    /* avatars, status dots, pills */
}
```

**Radius semantic mapping:**

| Surface | Radius |
| --- | --- |
| Badge, chip, tag | `xs` |
| Inline icon button | `sm` |
| Button (default) | `md` |
| Input, select | `md` |
| List row, table row hover-bg | `sm` |
| Card | `lg` |
| Dialog, popover | `lg` |
| Toast | `lg` |
| Modal (file viewer fullscreen) | `xl` |
| Avatar, status dot | `full` |
| Pill (status pill, filter pill) | `full` |

**Anti-pattern:** mixing radii within the same composition (a card with `lg` containing a button with `xs`). Stay within one step difference at most.

## Border weights

```css
--border-width-thin:    1px;    /* default */
--border-width-medium:  1.5px;  /* slight emphasis (focus rings adjacent borders) */
--border-width-thick:   2px;    /* selected state, error state */
```

Avoid 3px+ borders. They look chunky.

## Container widths

Inventario uses a **content-centered layout**, not full-bleed. Max widths scale with viewport but cap to keep line lengths legible.

```css
--container-narrow: 640px;   /* auth pages, single-form views */
--container-default: 1120px; /* most list/detail views */
--container-wide:   1440px;  /* dashboards, file galleries */
--container-full:    100%;   /* file viewer fullscreen, gallery overlay only */
```

Side gutters scale:

| Viewport | Side gutter |
| --- | --- |
| <640px (mobile) | `space-4` (16px) |
| 640–1024 (tablet) | `space-6` (24px) |
| ≥1024 (desktop) | `space-8` (32px) — but content centered within container max-width |

## Breakpoints

```css
@theme {
  --breakpoint-sm: 640px;   /* portrait tablet, large phone landscape */
  --breakpoint-md: 768px;   /* tablet */
  --breakpoint-lg: 1024px;  /* small desktop, sidebar appears here */
  --breakpoint-xl: 1280px;  /* desktop */
  --breakpoint-2xl: 1536px; /* large desktop */
}
```

**Layout shifts at breakpoints:**

| <640 (mobile) | 640–1023 (tablet) | ≥1024 (desktop) |
| --- | --- | --- |
| Bottom navigation (5 items) | Bottom navigation (5 items) | Sidebar (collapsible) |
| Single-column lists | 2-col card grids | 3–4 col card grids |
| Stacked forms | Stacked forms | Two-column forms where natural |
| Drawer-style filters | Drawer-style filters | Persistent filter rail |
| Full-screen modals | Centered modals (90vw) | Centered modals (max 720px) |

## Grid system

Inside a container, content uses a **12-column grid** with **24px gutters at desktop**, **16px at tablet**, **none at mobile (single column)**.

CSS-grid based, not flex hacks:

```css
.layout-grid {
  display: grid;
  grid-template-columns: repeat(12, 1fr);
  gap: var(--space-6);
}

@media (max-width: 768px) {
  .layout-grid { gap: var(--space-4); grid-template-columns: 1fr; }
}
```

**Common compositions:**

| View | Grid usage |
| --- | --- |
| Dashboard | 12-col, widgets span 4 / 6 / 8 / 12 |
| Detail two-pane | Main content `col-span-8`, side panel `col-span-4` |
| Form layout | Single column up to `--text-measure-form` width, never grid for forms |
| Card list | Auto-fit minmax(280px, 1fr) — grid auto-fills |

## Touch target minimums

- **Interactive surface:** min 44×44px (iOS HIG / WCAG 2.5.5)
- **Visible button:** can be smaller (e.g., 32px tall) but the click target via padding extends to 44px
- **Inline icon buttons in dense tables:** acceptable to be 32×32 on desktop hover, but pair with row-click for primary action

## Density modes

The default density is **comfortable**. Two alternates available via user setting (per `17-density-and-modes.md`):

| Mode | Adjustments vs default |
| --- | --- |
| Compact | All `--padding-card-*` -1 step; row heights -25%; gap-stack-default → -tight |
| Comfortable (default) | Values as specified above |
| Cozy | All `--padding-card-*` +1 step; gap-stack-default → -relaxed |

Density swap is a single CSS variable change at the root — no per-component JS.

## Layout templates

Six page templates cover ~90% of views. Documented in `11-page-layouts-and-flows.md`:

1. **List view** (locations, commodities, files, exports)
2. **Detail view** (entity with sections)
3. **Form view** (create / edit)
4. **Dashboard** (data widgets)
5. **Empty/onboarding** (full-screen call-to-action)
6. **Auth/single-form** (login, register, forgot password)

## Anti-patterns to flag in PR review

- Inline arbitrary spacing values: `style="padding: 11px"` or `class="p-[13px]"`
- Mixed radii within a single composition
- Touch targets <44px on mobile
- Forms wider than `--text-measure-form` (causes scanning fatigue)
- Page-level content extending edge-to-edge on desktop (use container max-widths)
- Grid + flex nesting where one would suffice (CSS-grid for 2D layouts, flex for 1D)
