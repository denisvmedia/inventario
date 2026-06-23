# Density & Theme Modes

Three density steps, two theme modes, one persistence model. Every
surface respects both.

## Density

```ts
type Density = "comfortable" | "cozy" | "compact"
```

| Density | Row padding (settings) | List-row padding | When |
| --- | --- | --- | --- |
| `comfortable` (default) | `py-3.5` | `px-4 py-3` | Most users; spacious, scannable. |
| `cozy` | `py-3` | `px-4 py-2.5` | Power users; mid. |
| `compact` | `py-2.5` | `px-4 py-2` | Spreadsheet-style power users. |

Density is set on `<html data-density="cozy">` by `DensityProvider`
(`frontend/src/hooks/useDensity.tsx`). Persisted to `localStorage`
and mirrored across tabs via the `storage` event. The Settings page
(#1414) writes through `PATCH /settings` and mirrors back to
`localStorage` on boot.

## Density-aware utilities

Use Tailwind's `data-[density=*]:` arbitrary variant when the rule
is one-off:

```tsx
<div className="py-3.5 data-[density=cozy]:py-3 data-[density=compact]:py-2.5">…</div>
```

For pattern-shaped utilities (every list row, every settings row),
build them into the component primitive in `src/components/ui/` so
the call site stays terse.

What scales with density:

- List-row vertical padding.
- Settings-row vertical padding (`py-3.5` → `py-3` → `py-2.5`).
- Stat-card outer padding (`px-4 py-3` → `px-3 py-2.5` → `px-3 py-2`).
- Icon sizes inside lists (e.g. row chevrons can drop from
  `size-4` → `size-3.5` in compact).
- Section spacing (`space-y-5` → `space-y-4` → `space-y-3`).

What doesn't scale with density:

- Page wrapper padding (`p-6` always — see [03-space-and-layout.md](03-space-and-layout.md)).
- Card outer padding (`p-6` always).
- Type scale (`text-sm` body always).
- Icon sizes outside dense lists.
- Border thickness (always 1px).
- Focus ring thickness (always 3px — [14-accessibility.md](14-accessibility.md)).

The product positioning ([00-positioning.md](00-positioning.md)) commits to a coherent
visual rhythm; only the **vertical density** of dense surfaces shifts.

## Theme modes

`light` (default), `dark`, and `system` (follows OS).

Set via `useTheme()` (`src/components/theme-provider.tsx`). Persisted
to `localStorage` under the same idempotency contract as density.
`system` doesn't persist a value beyond the choice itself; it reads
`(prefers-color-scheme: dark)` on every mount.

The `.dark` class on `<html>` is the toggle — every token swaps via
the variants in `frontend/src/index.css`. See [01-palette.md](01-palette.md).

## Mode-aware code

| Surface | Behavior |
| --- | --- |
| Tokens (`--color-*`) | Auto-swap via `:root` / `.dark`. |
| Component overrides | Avoid `dark:` utilities; tokens cover it. |
| User-supplied images | Don't tint. The image is what the user uploaded. |
| Charts | Tokens swap automatically; no per-mode override. |
| Brand mark | The `AppLogo` component picks light vs. dark variant via the active `.dark` class. See [19-branding.md](19-branding.md). |

The one place we use `dark:` utilities is layout adjustments — e.g. a
border that should disappear in dark mode. Used sparingly:

```tsx
<div className="border border-border dark:border-transparent">
```

## Mode + density combinatorics

Four modes × three densities = 12 surfaces to test on every visual
PR. See [../screenshots.md](../screenshots.md) for the helper.

The density contract is mode-agnostic: a `compact` surface in dark
mode looks the same as a `compact` surface in light mode, modulo the
token swap.

## System preference matching

| OS pref | Default behavior |
| --- | --- |
| `prefers-color-scheme: dark` | Mode `system` resolves to dark |
| `prefers-color-scheme: light` | Mode `system` resolves to light |
| `prefers-reduced-motion: reduce` | `tw-animate-css` and `motion-reduce:` honor it (per [05-motion.md](05-motion.md)) |
| `prefers-contrast: more` | Not yet honored — TBD. Tokens have headroom for an "AAA-only" variant when we wire it. |
| Touch vs. pointer | `useIsMobile()` (`src/hooks/use-mobile.ts`) detects the mobile breakpoint; surfaces use it for adjusted hover/layout state. |

## Switching at runtime

Both density and theme mode swap immediately on user action — no page
reload. The `<html>` attribute changes; CSS variables resolve via the
new context; React doesn't re-render because the change is purely
stylistic.

Tests:

- `useDensity` test asserts the `<html>` attr changes on `setDensity`.
- `useTheme` test asserts `.dark` adds/removes on `setTheme`.
- `setup.ts` provides a `matchMedia` stub for jsdom.

## SSR / pre-paint

Inventario is a Vite SPA — no SSR. Both providers run on `useEffect`
in the client; the very first render uses the `<ThemeProvider
defaultTheme="system">` and `<DensityProvider defaultDensity="comfortable">`
defaults, then the providers reconcile with `localStorage` + system
preference on mount.

A momentary mismatch ("flash of light theme") can happen on a
`prefers-color-scheme: dark` cold load. To avoid it, the
`index.html` can hoist a tiny inline script that reads
`localStorage.theme` and applies `.dark` before React mounts. This
isn't currently wired — file an issue if it becomes a problem.

## Hard rules

1. **Density on `<html>`**, never per-component context. The
   attribute selector is the API.
2. **Theme via the `.dark` class**, not via `dark:` utilities for
   color tokens. Tokens swap automatically.
3. **System preference is the default theme**, not light. Mac users
   in dark mode get a dark UI on first paint.
4. **Persist to localStorage**, mirror via the `storage` event.
5. **`matchMedia` stubbed in tests.** Don't reach for `window.matchMedia`
   directly — it's not in jsdom.

## Anti-patterns

- Per-component `useState<Density>` — use the provider.
- `dark:bg-gray-900` overrides for color tokens — use the token.
- A "high-contrast mode" toggle. The dark-mode token contrast already
  passes AAA on body text.
- A density preview swatch that animates between densities. Density
  is a state, not motion.
- Hardcoding `prefers-color-scheme: dark` in a media query inside a
  component — the `.dark` class is the cue.

## Cross-refs

- Tokens: [01-palette.md](01-palette.md).
- Spacing scale (the contract density modulates): [03-space-and-layout.md](03-space-and-layout.md).
- Reduced motion: [05-motion.md](05-motion.md), [14-accessibility.md](14-accessibility.md).
- Settings page that exposes the controls: [11-page-layouts-and-flows.md](11-page-layouts-and-flows.md).
- Implementation:
  - `frontend/src/components/theme-provider.tsx`
  - `frontend/src/hooks/useDensity.tsx`
  - `frontend/src/components/ModeToggle.tsx`
  - `frontend/src/components/DensityToggle.tsx`
