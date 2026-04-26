# Inventario — Design Direction

This folder is the **complete foundation brief** for redesigning Inventario as a modern personal-inventory product. Every decision a designer would normally make for a product of this category is documented here so engineering can implement without "what should this corner radius be?" stops.

The brief is opinionated. Where there is taste room, three or fewer concrete options are presented with a recommendation. Where there is no taste room (accessibility minimums, motion timings, semantic state behavior), one answer is given.

## Reading order

For a quick read: 00, 01, 02, 11, 19.
For implementation: read all in order before opening a code file.

## Index

### Foundation (read first)

| # | Document | Purpose |
| --- | --- | --- |
| 00 | [Positioning](00-positioning.md) | Product identity, audience, tone — anchors every later decision |
| 01 | [Palette](01-palette.md) | Three color directions (light + dark), semantic tokens |
| 02 | [Typography](02-typography.md) | Type scale, font pairings, hierarchy rules, vertical rhythm |
| 03 | [Space & Layout](03-space-and-layout.md) | Spacing scale, radii, borders, breakpoints, container widths, grid |
| 04 | [Elevation & Effects](04-elevation-and-effects.md) | Shadow scale, opacity, blur, glass/frost surfaces |
| 05 | [Motion](05-motion.md) | Durations, easings, motion language per element, reduced-motion |

### Visual language

| # | Document | Purpose |
| --- | --- | --- |
| 06 | [Iconography & Illustration](06-iconography-and-illustration.md) | Icon system (Phosphor), weights, sizes, illustration strategy |
| 07 | [Data Visualization](07-data-visualization.md) | Chart palette, chart types per data shape, sparkline style |

### Interaction & components

| # | Document | Purpose |
| --- | --- | --- |
| 08 | [Interaction States](08-interaction-states.md) | Hover, focus, active, disabled, loading, error, empty, selected |
| 09 | [Component Patterns](09-component-patterns.md) | Buttons, forms, modals, cards, tables, navigation, badges, tooltips |
| 10 | [File & Media Handling](10-file-and-media.md) | File previews, upload, FileViewer fullscreen, drag-drop |
| 11 | [Page Layouts & Flows](11-page-layouts-and-flows.md) | Templates, onboarding, error pages, empty states, dashboard |

### Content & language

| # | Document | Purpose |
| --- | --- | --- |
| 12 | [Tone of Voice & Copy](12-tone-of-voice-and-copy.md) | Voice, microcopy templates, naming, error/success messages |
| 13 | [Formatting & i18n](13-formatting-and-i18n.md) | Date/time, currency, numbers, pluralization, RTL strategy |

### Quality bars

| # | Document | Purpose |
| --- | --- | --- |
| 14 | [Accessibility](14-accessibility.md) | Contrast, focus, keyboard, screen reader, touch targets |
| 15 | [Form & Data UX](15-form-and-data-ux.md) | Auto-save, validation, optimistic UI, conflict, offline, drafts |
| 16 | [Notifications & Trust](16-notifications-and-trust.md) | Severity hierarchy, alerts, sensitive data, trust signals |

### Adaptation & extensibility

| # | Document | Purpose |
| --- | --- | --- |
| 17 | [Density & Theme Modes](17-density-and-modes.md) | Compact/comfortable/cozy, light/dark/system toggle UX |
| 18 | [Print & Export](18-print-and-export.md) | Print stylesheets, PDF export of inventory, sharing |
| 19 | [Branding](19-branding.md) | Logo system, favicon, OG, email templates |
| 20 | [Edge Cases](20-edge-cases.md) | 404, 500, maintenance, offline, empty account, deleted entities |

### Production assets

| # | Document | Purpose |
| --- | --- | --- |
| 21 | [Logo Mark Directions](21-logo-directions.md) | Five icon-mark directions with ready-to-paste ChatGPT Images 2.0 prompts |
| 22 | [Illustration Prompts](22-illustration-prompts.md) | Drop-in ChatGPT Images 2.0 prompts for the full illustration set with shared style preamble |

## Decisions made

All taste decisions are now committed:

1. **Palette** (01) — final cream + navy + terracotta tokens, light + dark
2. **Type pairing** (02) — Switzer (display) + Inter (body), both free
3. **Logo direction** (21) — Direction 3, catalog tag with corner-cut + string-hole
4. **Illustration sourcing** — AI-generation via ChatGPT Images 2.0 (`gpt-image-2`) using prompts in (22)

Everything from foundation to component patterns to copywriting tone is concrete in the documents below. Sprint 0 implementation can begin without further design decisions.

## What's intentionally out of scope

- Specific micro-decisions per view (those are sprint work, not foundation)
- A full pattern library implementation (this brief defines patterns; building them is sprint 1-2)
- Brand strategy beyond visual/voice identity (positioning above is sufficient for this product's stage)
- Marketing site / landing page (this is the in-app product brief)

## Sprint 0 deliverables (what gets built from this brief)

1. Tailwind v4 `@theme` token file with chosen palette + type + spacing + radii + motion
2. `dark` mode with full coverage
3. Replace lucide → Phosphor icons project-wide
4. Build/refresh primitives: `Button`, `Input`, `Select`, `Dialog`, `Toast`, `Skeleton`, `EmptyState`, `Card`, `Badge`, `Tooltip`
5. Fix the system-wide section-header contrast bug (this is bug-fix not foundation, but blocking)
6. Update tone-of-voice on every user-facing string per (12)

Sprint 1+ work is informed by, but not blocked on, this brief.
