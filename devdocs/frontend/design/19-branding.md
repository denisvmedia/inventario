# Branding

The Inventario brand mark, the favicon, and email templates — in
production today, with the rules to keep them coherent.

## The mark

`AppLogo` (`frontend/src/components/AppLogo.tsx`) renders a small
inline SVG glyph alongside the wordmark `t("common:brand")`:

- A stylized house with a checklist inside (the visual mark).
- "Inventario" as the wordmark, in the system sans face.

The glyph is hand-authored at `18×18` viewBox; both the silhouette
and the checklist marks resolve to theme tokens (`fill-foreground`,
`fill-background`) so it inverts cleanly across modes without a
per-mode SVG variant.

The mark is committed. The historical directions explored in
[21-logo-directions.md](21-logo-directions.md) are kept as a record
of options considered; the shipping mark is the house-with-checklist.

## Where the mark appears

The current `AppLogo` ships one variant — glyph + wordmark in a
horizontal `flex items-center gap-2` shell. Surfaces compose around it:

| Surface | How it's used |
| --- | --- |
| Sidebar (header) | Full `<AppLogo />` |
| Top bar (mobile) | Full `<AppLogo />` |
| Auth pages | Full `<AppLogo />`, centered above the form |
| Favicon | Glyph-only — sourced from `frontend/public/favicon.svg` |

Variants for OG-style social preview and email-template headers
aren't shipped today; when they land, they should pair the same glyph
with the wordmark on a warm-neutral surface using the canonical
`--foreground` / `--background` tokens.

## Color

The glyph's silhouette is `fill-foreground`; the checklist details
are `fill-background` (so they read as cut-outs against the
silhouette). Both classes resolve through the active `.dark` swap, so
the mark inverts cleanly:

| Mode | `--foreground` | `--background` |
| --- | --- | --- |
| Light | near-black warm | warm off-white |
| Dark | light warm | deep warm |

For surfaces where the mark sits on a *primary* background (rare —
no production surface does this today), wrap and override:

```tsx
<div className="bg-primary text-primary-foreground [&_.fill-foreground]:fill-primary-foreground">
  <AppLogo />
</div>
```

## Hard rules

1. **Tokens, not raw colors.** The mark's color is `currentColor`,
   inheriting from `text-foreground` (or `text-primary-foreground`).
2. **No drop shadow on the mark.** Borders, not shadows
   ([04-elevation-and-effects.md](04-elevation-and-effects.md)).
3. **No gradient fills.** Solid token colors.
4. **No animation.** The mark is static. (One exception: a faint
   `animate-pulse` on the splash screen during the very first
   `/auth/me` probe — TBD; not yet wired.)
5. **Don't outline-stroke the mark to "stand out"**. Choose the
   correct background instead.

## Sizing

The current `AppLogo` ships at a fixed `18×18` glyph next to a
`text-sm` wordmark — the surface decides spacing via the parent's
`gap-*`. Don't render the mark above the equivalent of `h-12` in
product UI — it's a recognition cue, not a hero illustration. When
a new surface needs a larger mark (auth hero, future onboarding
splash), extend `AppLogo` with a size prop rather than scaling via
`className` overrides.

## Clear space

Leave whitespace ≥ 25% of the mark's height on all sides. Don't pack
adjacent UI right against the wordmark.

## Favicon

`frontend/public/favicon.svg` is the source. Browsers cache
aggressively; update the build hash by changing the filename if you
swap it.

The favicon uses the glyph only — the wordmark "Inventario" doesn't
read at 16×16.

## OG / social preview (future)

There is no shipped OG image today (`frontend/public/` carries only
the favicon and the embed sentinel). When per-page OG / social
preview becomes a goal, it lands as a separate issue; the spec then
follows the recipe above (glyph + wordmark on a warm-neutral surface,
1200×630).

The app is a Vite SPA — there's no SSR or static-export pipeline that
would generate per-tenant OG today. A future Cloudflare Worker /
edge function could fill that gap; out of scope for this brief.

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
trustworthy". Per [00-positioning.md](00-positioning.md).

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

- Logo source / direction: [21-logo-directions.md](21-logo-directions.md).
- Email templates BE: `go/email/templates/`.
- Favicon source: `frontend/public/favicon.svg`.
- AppLogo component: `frontend/src/components/AppLogo.tsx`.
