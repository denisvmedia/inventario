# Inventario — Design Direction

The visual and interaction contract for the React frontend. Engineering
docs in the parent folder cover *how* code is organized; this folder
covers *why* the product looks and behaves the way it does, and what
the reusable design decisions are.

The visual contract here is anchored to the canonical internal design
mock. When the mock and these docs disagree, the mock wins — file an
issue and update both in lockstep.

## How to read this folder

Every doc carries a one-line **commitment** in its opening paragraph
(`committed`, `floor`, `recommendation`, `proposal`). That tells you
whether the section is taste-locked, a quality bar, an opinionated
default with room to deviate, or a forward-looking idea waiting on a
PR.

Where there is taste room, three or fewer concrete options are
presented with a recommendation. Where there isn't (a11y minimums,
motion timings, OKLCH semantics), one answer is given.

Cross-references between docs are written as bare filenames in
backticks — e.g. *"per [15-form-and-data-ux.md](15-form-and-data-ux.md)"* — not as markdown
links, so the references survive renames cleanly. The hub (this file)
is the only index.

## Foundation

| # | Document | Purpose |
| --- | --- | --- |
| 00 | [Positioning](00-positioning.md) | Audience, voice, what the product *is*. Anchors every taste decision downstream. |
| 01 | [Palette](01-palette.md) | OKLCH tokens, light + dark, semantic colors. `committed`. |
| 02 | [Typography](02-typography.md) | System font stack, type scale, role classes. `committed`. |
| 03 | [Space & layout](03-space-and-layout.md) | Spacing rhythm, page shells, card anatomy. |
| 04 | [Elevation & effects](04-elevation-and-effects.md) | Borders over shadows. `committed`. |
| 05 | [Motion](05-motion.md) | `tw-animate-css` durations, easings, reduced-motion. |

## Visual language

| # | Document | Purpose |
| --- | --- | --- |
| 06 | [Iconography & illustration](06-iconography-and-illustration.md) | `lucide-react` only, size scale, illustration sourcing. `committed`. |
| 07 | [Data visualization](07-data-visualization.md) | Chart palette, chart-type-per-data-shape, sparklines. |

## Interaction & components

| # | Document | Purpose |
| --- | --- | --- |
| 08 | [Interaction states](08-interaction-states.md) | hover / focus / active / disabled / loading / empty / error / selected. |
| 09 | [Component patterns](09-component-patterns.md) | Anatomy + rules per shadcn primitive in use. |
| 10 | [File & media](10-file-and-media.md) | The four-category file model (Photos / Invoices / Documents / Other). |
| 11 | [Page layouts & flows](11-page-layouts-and-flows.md) | Templates: dashboard, list, detail, settings, auth, error. |

## Content & language

| # | Document | Purpose |
| --- | --- | --- |
| 12 | [Tone of voice & copy](12-tone-of-voice-and-copy.md) | Microcopy templates, naming, error/success messages. |
| 13 | [Formatting & i18n](13-formatting-and-i18n.md) | Numbers, dates, currency, plurals across `en` / `cs` / `ru`. |

## Quality bars

| # | Document | Purpose |
| --- | --- | --- |
| 14 | [Accessibility](14-accessibility.md) | WCAG 2.2 AA `floor`. Some surfaces target AAA. |
| 15 | [Form & data UX](15-form-and-data-ux.md) | Validation timing, server errors, draft persistence, optimistic updates. |
| 16 | [Notifications & trust](16-notifications-and-trust.md) | Sonner toast hierarchy, sensitive-data handling, trust signals. |

## Adaptation

| # | Document | Purpose |
| --- | --- | --- |
| 17 | [Density & modes](17-density-and-modes.md) | comfortable / cozy / compact + light / dark. |
| 18 | [Print & export](18-print-and-export.md) | Print stylesheet, the existing `CommodityPrintPage` route, PDF export. |
| 19 | [Branding](19-branding.md) | Logo system (`AppLogo`), favicon, OG image, email templates. |
| 20 | [Edge cases](20-edge-cases.md) | 404 / 500 / offline / no-group / empty-account / deleted-entities. |

## Production assets

| # | Document | Purpose |
| --- | --- | --- |
| 21 | [Logo directions](21-logo-directions.md) | The current bracket-cube mark + variants. |
| 22 | [Illustration prompts](22-illustration-prompts.md) | Empty-state and onboarding illustrations — sourcing recipe. |

## Decisions locked

These are taste-locked across this folder. Don't re-litigate them in a
PR; if one genuinely needs to change, file an issue and update both
this folder and the canonical mock together.

1. **Palette**: warm-neutral OKLCH with amber accents (light) / dark
   warm with light amber (dark). No purple. No raw color names. See
   [01-palette.md](01-palette.md).
2. **Typography**: system font stack (no Google Fonts, no Switzer, no
   Inter). The browser's native sans-serif is the body face. See
   [02-typography.md](02-typography.md).
3. **Iconography**: `lucide-react`, by named import only. Size scale
   `size-3` → `size-10`. See [06-iconography-and-illustration.md](06-iconography-and-illustration.md).
4. **Components**: shadcn/ui (new-york / neutral) on Radix primitives
   via the `radix-ui` umbrella. No `@base-ui/react`, no `next-themes`.
   See [09-component-patterns.md](09-component-patterns.md) and [../imports-and-bans.md](../imports-and-bans.md).
5. **Motion**: `tw-animate-css` only. No Framer Motion / react-spring.
   See [05-motion.md](05-motion.md).
6. **Elevation**: borders, not shadows. The single exception is
   `shadow-xs` on inputs. See [04-elevation-and-effects.md](04-elevation-and-effects.md).
7. **A11y floor**: WCAG 2.2 AA. See [14-accessibility.md](14-accessibility.md).

## Out of scope (deliberately)

- A separate "design system" package or component library. The shadcn
  copies in `frontend/src/components/ui/` and the internal design mock
  together are the system; there is no NPM package to publish.
- Mobile-first redesign. Inventario is a desktop-first inventory app;
  it's responsive but the design bias is desktop. A native app or a
  mobile-first refresh would be a separate epic.
- A theming engine beyond `light` / `dark`. The OKLCH tokens are the
  theming engine; per-tenant theming is not a goal.
- Brand-mark exploration beyond the existing logo. The current logo
  is committed; alternative directions belong in
  [21-logo-directions.md](21-logo-directions.md) as historical exploration only.
