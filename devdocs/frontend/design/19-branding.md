# Branding

The product's visual identity beyond the application UI: logo, marks, social/email/print presence.

## Current state

Inventario currently uses a small QR-cube favicon as the primary mark. It reads as a placeholder, not a brand. The wordmark "Inventario" is set in the default UI font.

The product needs a proper logo system before launching publicly. This is a one-time investment.

## Brand mark requirements

A good Inventario mark should be:

1. **Quiet** — matches the tone (not loud, not playful, not corporate)
2. **Personal** — suggests household, archive, care — not enterprise SaaS
3. **Versatile** — works at 16×16 (favicon) and 256×256 (app icon) and as a wordmark
4. **Monochrome-safe** — single-color version for print, minimal scenarios
5. **Memorable** — distinguishable from generic database/grid icons

## Mark direction options

### Option 1: Type-only (wordmark)

"Inventario" set in the chosen display font (Switzer / GT Walsheim / Tiempos depending on type pairing decision in `02-typography.md`), with a deliberate ligature or kern that makes it distinctive. No icon mark; favicon = first letter.

Pros: cheapest, instantly fits any palette, ages well.
Cons: no scalable icon for app stores, needs strong typography to be distinctive.

### Option 2: Letter-mark (initial in a frame)

Single letter "I" or "i" in a custom-drawn frame — could be a rectangle suggesting a label, a circle suggesting a tag, a small square with a notch suggesting a drawer. Used as both icon and inline with wordmark.

Pros: scalable, distinctive, friendly without being a cartoon.
Cons: harder to design well; tempting to settle for a generic "first letter in a circle".

### Option 3: Pictorial mark (object-based)

A simple, abstract object — a label, a tag, a folded card, a key — that evokes the product's domain. Set alongside the wordmark.

Pros: unique, story-rich.
Cons: easy to date (peak 2018 "logos with a hidden meaning" trend), hard to keep timeless.

### Recommendation

**Letter-mark direction (option 2).** Best balance of distinctiveness, scalability, and timelessness for this product class. Things 3 (an "i" shape), 1Password (a key shape), Notion (an "N") all use related strategies and aged well.

Concept brief for the mark:
- Lowercase "i" or rounded square with a notch suggesting a label slot
- Single color (the palette accent), or two-tone for mark-on-light vs mark-on-dark
- Drawn in the same weight family as the chosen display font
- Optical-balanced: looks centered at small sizes
- Has a subtle implied story (drawer, tag, label) without being literal

This is best done by a brand designer (~€500–1500 on Dribbble for a strong illustrator + 2 revisions) or via a longer trial-and-error with AI tools + manual polish.

## Wordmark

"Inventario" set in display font, weight medium, with carefully tracked spacing. Used:
- In-app sidebar logo lockup (mark + wordmark)
- Footer of the app
- Email headers
- Marketing site (when one exists)

Avoid using stylized letterforms (custom characters) unless inevitable — they don't translate to non-Latin scripts (Cyrillic localization).

## Lockup variations

Six variations needed:

1. **Primary horizontal**: mark + wordmark side-by-side, default
2. **Primary stacked**: mark above wordmark, for square contexts (cards, app stores)
3. **Mark-only**: just the icon, for tight spaces (favicon, sidebar collapsed)
4. **Wordmark-only**: just the type, for contexts where mark would be redundant
5. **Reverse**: light-on-dark variant for dark surfaces
6. **Monochrome**: black-only for print

Spacing rules: clear-space around the lockup equals the height of the mark. Never crowd.

## Favicon set

| Size | Use |
| --- | --- |
| 16×16 | browser tab |
| 32×32 | browser tab on retina |
| 48×48 | Windows |
| 180×180 | iOS home screen |
| 192×192 | Android home screen |
| 512×512 | Android adaptive icons |

ICO file with 16/32/48 for legacy. Modern browsers prefer SVG favicon (one file scales cleanly).

```html
<link rel="icon" type="image/svg+xml" href="/favicon.svg">
<link rel="icon" type="image/png" href="/favicon-32.png" sizes="32x32">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
<link rel="manifest" href="/site.webmanifest">
```

## Open Graph / Twitter card

For when Inventario links are shared:

- 1200×630px image
- Brand mark + tagline + product screenshot framed in palette colors
- Minimal text — the unfurl card is contextualized by the URL and meta description

Example:
```
[mark]                  Inventario
                        A quiet place for the things you own.

[stylized screenshot of dashboard with palette overlay]
```

## Email branding

Per `16-notifications-and-trust.md`. Headers contain the wordmark (image link with alt text). Footer has the address (if business) and unsubscribe.

## Print branding

Per `18-print-and-export.md`. Top of every printed page has small wordmark + date. Footer has page number + "Inventario".

## App theme color

Define a single theme color used for:
- Browser address bar tint on mobile (`<meta name="theme-color">`)
- Splash screens
- App badges

Use the chosen palette accent at full saturation. One value for light mode, one for dark.

```html
<meta name="theme-color" content="#C2410C" media="(prefers-color-scheme: light)">
<meta name="theme-color" content="#F08C5C" media="(prefers-color-scheme: dark)">
```

## Tagline

Several candidates per positioning:

- "A quiet place for the things you own." (recommended — matches positioning doc)
- "Keep track of your things, calmly."
- "Records of what's yours."
- "Your stuff, on the record."

Decision: **"A quiet place for the things you own."** Used on login, in OG card, in email footer.

## Voice in branded surfaces

Marketing voice = product voice (per `12-tone-of-voice-and-copy.md`). Inventario doesn't put on a "marketing voice" for its branded touchpoints. The same plain, calm, respectful tone applies.

Example landing page copy:

> ## A quiet place for the things you own.
>
> Keep track of warranties, prices for insurance, and where you bought things —
> so you don't have to remember.
>
> [Try it free] [Self-host it]

## Design system asset bundle

Sprint 0 / 1 deliverables:

1. SVG mark (master, vector source)
2. SVG wordmark (master)
3. PNG mark exports at 16, 32, 48, 64, 128, 256, 512
4. ICO favicon
5. Apple-touch-icon
6. Lockup files (Figma or sketch source) with all 6 variations
7. Brand guidelines doc (1 page) covering clear space, color usage, no-go transformations
8. OG card template (PSD or Figma)
9. Email header template (HTML + image fallback)

## Decision needed

- Mark direction: type-only / letter-mark / pictorial?
- Designer source: in-house Claude direction / commissioned designer / AI tools + polish?
- Tagline: confirm "A quiet place for the things you own." or alternative

I recommend: letter-mark + commissioned designer (~€500–1000) + the recommended tagline. This is a one-time investment that pays off for the product's lifetime.

## What ships in sprint 0

Realistically, sprint 0 ships **placeholders that look intentional**:

1. Wordmark in the chosen display font, weight medium, in palette accent — used in sidebar, login, email
2. Favicon: a simple "i" in palette accent inside a rounded square — placeholder until real mark exists
3. App theme-color meta tags
4. OG card with wordmark + tagline + palette gradient bg

Real mark replaces the placeholder when commissioned.
