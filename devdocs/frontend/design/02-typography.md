# Typography

**Committed.** Inventario uses the **system font stack** — no Google
Fonts, no Switzer, no Inter, no custom face. The browser's native
sans-serif is the body face; the `mono` Tailwind family is the system
mono.

## Why no custom font

Three reasons, in descending order of importance:

1. **Performance.** A web-font is 80–200 KB before the first paint
   shifts. Inventario's entry-bundle budget is 200 KB gzip total — a
   font we don't need would eat the room a real feature should claim.
   See `../perf.md`.
2. **Privacy.** Loading from `fonts.googleapis.com` means an HTTP
   request to Google on every cold load, with referer + user-agent
   exposed. The product positioning (`00-positioning.md`) commits to
   data ownership; a third-party font request is a small contradiction.
3. **Resilience.** The system stack always works — no FOUT, no CSP
   surprise, no maintenance when Google rotates a CDN endpoint.

The system stack on a 2026 user looks like SF Pro on macOS, Segoe UI
Variable on Windows, Roboto on Android, system-ui on most Linux
desktops. All four are excellent at body sizes and ship pre-installed.
The visual rhythm of the design (case, tracking, line-height) is what
carries the look — not a chosen typeface.

## Stack

Tailwind v4's default `font-sans` resolves to:

```css
font-family: ui-sans-serif, system-ui, sans-serif, "Apple Color Emoji",
  "Segoe UI Emoji", "Segoe UI Symbol", "Noto Color Emoji";
```

`font-mono` resolves to:

```css
font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
  "Liberation Mono", "Courier New", monospace;
```

Both are taken verbatim from Tailwind's defaults. Don't override in
`tailwind.config` or `@theme`.

## Type scale

Tailwind's default scale (in rem). Pick from this set; never write
`text-[15px]`:

| Class | Size | Use |
| --- | --- | --- |
| `text-xs` | 0.75 / 12px | Overline, label, badge, caption |
| `text-sm` | 0.875 / 14px | Body (default), list rows, form fields |
| `text-base` | 1 / 16px | Card section heading, dialog body |
| `text-lg` | 1.125 / 18px | Stat value secondary, dialog title supporting |
| `text-xl` | 1.25 / 20px | Section title in dense layouts |
| `text-2xl` | 1.5 / 24px | Stat value, dialog title |
| `text-3xl` | 1.875 / 30px | Page title (h1) |
| `text-4xl` | 2.25 / 36px | Hero / empty-state title (rare) |

`text-sm` is the body default. Most JSX should never need `text-base`
directly — `<p>` inside a card uses `text-sm`; `<DialogDescription>`
uses `text-sm text-muted-foreground` etc.

## Role classes

shadcn provides no default element styles. Apply Tailwind classes
explicitly:

| Role | Classes |
| --- | --- |
| Page title (h1) | `scroll-m-20 text-3xl font-semibold tracking-tight` |
| Section heading (h2) | `text-base font-semibold` |
| Sub-heading (h3) | `text-sm font-semibold` |
| Body | `text-sm leading-relaxed` |
| Muted / secondary | `text-sm text-muted-foreground` |
| Label / overline | `text-xs font-semibold uppercase tracking-widest text-muted-foreground` |
| Stat value | `text-2xl font-bold tracking-tight` |
| Stat label | `text-xs font-medium uppercase tracking-wide text-muted-foreground` |
| Code / mono | `font-mono text-xs` |
| Dialog title | `text-base font-semibold` |
| Empty-state title | `text-base font-semibold text-foreground` |
| Empty-state body | `text-sm text-muted-foreground` |

These are the canonical patterns. When you copy the page-title
recipe, copy *all* of `scroll-m-20 text-3xl font-semibold
tracking-tight` — not "just the size". `scroll-m-20` makes anchor
navigation land below the top bar; `tracking-tight` is the rhythm
adjustment that makes the system font read as headline-y.

## Weights

- `font-normal` (400) — body, default.
- `font-medium` (500) — list-row labels, settings-row labels, anything
  semibold-feeling but not loud.
- `font-semibold` (600) — section headings, stat labels, page titles.
- `font-bold` (700) — stat values, hero numbers. Used sparingly.

Don't reach for `font-extrabold` / `font-black`; they look heavy in
the system stack.

## Tracking and line-height

| Class | Use |
| --- | --- |
| `tracking-tight` | Page title, dialog title (any large size > 1.5 rem) |
| `tracking-normal` (default) | Body, sections |
| `tracking-wide` | Stat labels (`text-xs uppercase tracking-wide`) |
| `tracking-widest` | Overlines (`text-xs font-semibold uppercase tracking-widest`) |
| `leading-relaxed` | Body paragraphs |
| `leading-tight` | Stat values, dense rows |
| `leading-none` | Single-line truncation surfaces |

## Lining figures

`font-variant-numeric: tabular-nums` for any column of numbers
(stats, exports, file sizes) so they line up across rows:

```tsx
<span className="font-mono text-xs tabular-nums">{formatBytes(size)}</span>
```

When the value is decorative (a stat hero), tabular-nums is
unnecessary — the number stands alone. When it's repeated in a column,
tabular-nums is the difference between "considered" and "amateur".

## i18n: variable-width text

The system font handles cs / ru / latin-ext glyphs natively — no fallback
required. Keep titles short enough that the longest translation
(usually de or ru) doesn't wrap. See `13-formatting-and-i18n.md` for
the heuristics on copy length budgets.

## Hard rules

1. **Don't load a custom font.** Not via Google Fonts, not via
   `@font-face`, not via npm. The system stack is the answer.
2. **Don't write `text-[15px]`** — pick from the type scale.
3. **Don't combine `tracking-tight` with `text-sm`** — the rhythm
   only works at `text-2xl` and above. `tracking-normal` for body.
4. **Don't bold body text** for emphasis. Use a different role class
   or wrap in `<strong>` (which the browser renders bold by default).
5. **Don't `<h1>` everything.** Each page has exactly one `<h1>`,
   styled with the page-title role class.

## Anti-patterns

- `style={{ fontSize: 15 }}` — pick `text-sm` (14) or `text-base` (16).
- `<h1 className="text-2xl">` — the page-title class is `text-3xl`.
  Halving the size breaks the page rhythm.
- Loading a brand-y display font for the auth-pages hero. The auth
  pages are deliberately quiet (per `00-positioning.md`); display type
  contradicts that.
- Mixing `font-bold` with `font-semibold` headings on the same page.
  Pick one weight per role.

## Cross-refs

- Page wrapper anchor: `03-space-and-layout.md`.
- Empty-state typography: `20-edge-cases.md`.
- Date / number formatting: `13-formatting-and-i18n.md`.
- Mock canonical: `denisvmedia/inventario-design/CLAUDE.md` §4 (Typography).
