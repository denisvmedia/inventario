# Branding

The Inventario brand mark, the favicon, OG image, and email templates
— in production today, with the rules to keep them coherent.

## The mark

`AppLogo` (`frontend/src/components/AppLogo.tsx`) is a small SVG
wordmark + glyph composed inline. It pairs:

- A bracket-and-cube glyph (the visual mark).
- "Inventario" as the wordmark, in the system sans face.

The mark is committed; the alternative directions explored in
`21-logo-directions.md` are kept for historical reference but the
shipping mark is the bracket-cube.

## Where the mark appears

| Surface | Variant |
| --- | --- |
| Sidebar (collapsed) | Glyph only, `size-7` |
| Sidebar (expanded) | Glyph + wordmark, `h-8` |
| Top bar (mobile) | Glyph + wordmark, `h-7` |
| Auth pages | Glyph + wordmark centered above title, `h-9` |
| Print page footer | Wordmark only, `text-xs text-muted-foreground` |
| OG / social preview | Glyph + wordmark on warm-cream bg, fixed JPG |
| Favicon | Glyph only |

The variants are chosen by the surface, not by a prop; `AppLogo`
takes a `variant?: "glyph" | "full"` and a size class.

## Color

Two states, swapped by the active `.dark` class:

| Mode | Glyph | Wordmark |
| --- | --- | --- |
| Light | `text-foreground` (near-black warm) | `text-foreground` |
| Dark | `text-foreground` (light warm) | `text-foreground` |

The mark is **mode-aware via CSS**, not via two SVG sources. It picks
up `currentColor` and the parent's `text-foreground` class swaps with
the theme.

For surfaces where the mark sits on a colored background (auth-page
hero, OG image), use the inverse foreground:

```tsx
<div className="bg-primary text-primary-foreground">
  <AppLogo variant="full" />
</div>
```

## Hard rules

1. **Tokens, not raw colors.** The mark's color is `currentColor`,
   inheriting from `text-foreground` (or `text-primary-foreground`).
2. **No drop shadow on the mark.** Borders, not shadows
   (`04-elevation-and-effects.md`).
3. **No gradient fills.** Solid token colors.
4. **No animation.** The mark is static. (One exception: a faint
   `animate-pulse` on the splash screen during the very first
   `/auth/me` probe — TBD; not yet wired.)
5. **Don't outline-stroke the mark to "stand out"**. Choose the
   correct background instead.

## Sizing

| Use | Size class |
| --- | --- |
| Favicon | 16×16, 32×32 (rasterized from SVG) |
| Sidebar collapsed | `h-7` |
| Sidebar expanded | `h-8` |
| Auth-page hero | `h-9` |
| Email header | `h-10` |
| OG image | `h-24` (the OG itself is 1200×630) |
| Print footer | `h-4` (alongside wordmark text) |

Don't render the mark above `h-12` in product UI — it's a recognition
cue, not a hero illustration.

## Clear space

Leave whitespace ≥ 25% of the mark's height on all sides. Don't pack
adjacent UI right against the wordmark.

## Favicon

`public/favicon.svg` is the source. Browsers cache aggressively;
update the build hash by changing the filename if you swap it.

The favicon uses the glyph only — the wordmark "Inventario" doesn't
read at 16×16.

## OG / social preview

`public/og.png` is a fixed 1200×630 JPG: warm-cream background, mark
centered, single-line subtitle ("Personal inventory ledger"). Update
when the mark or tagline changes.

When the user shares an Inventario page, no per-page OG is generated
today (no SSR, no static export). The default OG is shown for every
URL. A future improvement would add per-tenant OG (via Cloudflare
Workers or similar); out of scope for this brief.

## Email templates

Sent BE-side from `go/email/templates/`:

- Verify email: header (mark + product name) → body → CTA.
- Password reset: same shell.
- Group invite: same shell + group name.

Email design rules:

- 600px max width.
- Inline styles only (no external CSS).
- One color: warm-cream background, near-black text, amber CTA.
- The mark is rendered as a single `<img>` from a CDN URL (the OG
  image's variant), not inline SVG (clients render inline SVG
  inconsistently).
- No web fonts. System fallback chain in inline `font-family`.

## Tone

The mark is **considered, quiet, neutral.** It doesn't communicate
"speed" or "modernity" or "premium". It signals "tool, considered,
trustworthy". Per `00-positioning.md`.

## Don'ts

- A logo on a stat card. Stat cards have icon tiles, not branding.
- A logo as a watermark on prints. Inventario doesn't watermark.
- An animated logo on auth-page entry. Quiet.
- A logo with rotating taglines. Static.
- A "Back to dashboard" link styled as the logo. Make it a real
  `<Link>` with the logo *icon* if needed; the wordmark is
  recognition, not navigation.
- A second mark for "Inventario Pro". There is no Pro tier.

## Cross-refs

- Logo source / direction: `21-logo-directions.md`.
- Email templates BE: `go/email/templates/`.
- Favicon source: `frontend/public/favicon.svg`.
- OG source: `frontend/public/og.png`.
- AppLogo component: `frontend/src/components/AppLogo.tsx`.
