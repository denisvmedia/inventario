# Density & Theme Modes

How users adjust the product to their preferences.

## Theme modes

Three options:
- **Light** (default for first-time users)
- **Dark**
- **System** (follows OS preference; default for returning users who haven't picked)

### Toggle UX

Theme toggle accessible from:
1. **User dropdown** (top-right or sidebar bottom) — "Appearance" submenu with three radio options + "System" preview
2. **Quick toggle** keyboard shortcut: `⌘⇧D` cycles light / dark / system
3. **Profile / Appearance** settings page — full settings including density, language, theme

The toggle is **never** in a hidden "UI Settings" expander like the current `/system` page. Theme is a primary user preference.

### Toggle anatomy

```
Appearance
─────────────────────────
( ) Light       ☀
( ) Dark        ☾
(•) System      ⌘
```

Radio cards with icon and label. Selected state shows accent ring.

### Theme application

Implementation via:

```css
:root {
  /* light tokens */
}

:root[data-theme="dark"] {
  /* dark tokens */
}

@media (prefers-color-scheme: dark) {
  :root[data-theme="system"] {
    /* dark tokens */
  }
}
```

JS sets `data-theme` on `<html>` based on user setting (persisted in localStorage). On first paint, inline script reads localStorage to avoid flash-of-wrong-theme.

### Transition between themes

When user toggles, root CSS transitions for `background-color`, `color`, `border-color` over `--duration-slow` with `--ease-default`. Not abrupt; not slow.

```css
:root {
  transition:
    background-color var(--duration-slow) var(--ease-default),
    color var(--duration-slow) var(--ease-default);
}
```

Disable transition during initial paint to prevent flash. Disable during route changes for performance.

## Density modes

Three options:
- **Compact** — for power users with lots of inventory
- **Comfortable** (default)
- **Cozy** — for users who prefer breathing room

Per `03-space-and-layout.md`, density is a single CSS variable shift.

### Toggle UX

Accessible from:
1. **User dropdown** → Appearance → Density
2. **Profile / Appearance** settings page

Less prominent than theme — most users never change this.

### Anatomy

```
Density
─────────────────────────
( ) Compact      ▤▤▤
(•) Comfortable  ▤ ▤ ▤
( ) Cozy         ▤  ▤  ▤
```

### Implementation

```css
:root {
  --space-card-padding: var(--padding-card);
  --row-height: 48px;
  --gap-stack: var(--gap-stack-default);
}

:root[data-density="compact"] {
  --space-card-padding: var(--padding-card-sm);
  --row-height: 36px;
  --gap-stack: var(--gap-stack-tight);
}

:root[data-density="cozy"] {
  --space-card-padding: var(--padding-card-lg);
  --row-height: 56px;
  --gap-stack: var(--gap-stack-relaxed);
}
```

Components reference the abstract token (`--space-card-padding`), not the size-specific one.

## Language

Per `13-formatting-and-i18n.md`:
- en (default)
- ru, cs, de (initial set)
- More via community translation contributions

### Toggle UX

Profile / Appearance / Language. Dropdown with native names:
- English
- Русский
- Čeština
- Deutsch

Switching is instant, persisted. Prompt user to refresh if they had unsaved drafts in another language (translation could change form behavior in edge cases — rare).

## Time format

- 24-hour vs 12-hour
- Default: locale-derived (en-US → 12-hour, others → 24-hour)
- Override in profile

## First day of week

- Sunday vs Monday vs Saturday
- Locale-derived default
- Override in profile (used only in date-range pickers)

## Currency display preferences

Already in `13-formatting-and-i18n.md`:
- Primary currency
- Currency display: code (CZK) vs symbol (Kč)
- Decimal places for currency (always 2 in Inventario)

## Notification preferences

Per `16-notifications-and-trust.md`:
- Email notifications on/off per type (warranty, backup, security, digest)
- Quiet hours
- Push notifications (deferred to PWA v2)

## Accessibility preferences

System preferences mostly handled automatically:
- `prefers-reduced-motion` (per `14-accessibility.md`)
- `prefers-color-scheme` (theme system mode)
- `prefers-reduced-transparency`

In-app overrides:
- High-contrast mode (toggle for users who want stronger contrast than AA)
- Large-text mode (scale `--text-*` tokens by 1.125)

These are deferred to v2 unless user demand emerges.

## Profile / Appearance page layout

Single profile / settings page with sections:

```
Profile
─────────────────────────
Identity (name, email, password)
Workspace (default workspace, currency)

Appearance
─────────────────────────
Theme (light/dark/system)
Density (compact/comfortable/cozy)
Language
Time format
First day of week

Notifications
─────────────────────────
Email preferences
Quiet hours

Security
─────────────────────────
Active sessions
Two-factor auth
Account deletion
```

Each section in a card, per `09-component-patterns.md` Form section pattern.

## Onboarding initial setup

First-time login captures preferences via a small "Make it yours" step:

```
Make it yours
─────────────────────────
Theme:    ( ) Light  ( ) Dark  (•) Match my system
Density:  Comfortable (most people)  [More options ▾]
Language: English  [More ▾]

[Continue]
```

Skippable. Defaults are sensible — if user skips, system theme + comfortable density + en.

## What ships in sprint 0

1. Theme system: light + dark + system mode, toggle in user dropdown
2. Density system: data-attribute switching, three modes
3. Persist preferences in localStorage + sync to server when authenticated
4. Profile / Appearance settings page (replaces hidden current "UI Settings" expander)
5. Onboarding "Make it yours" step

Sprint 1+:
- Time format toggle
- First-day-of-week toggle
- High-contrast mode
- Large-text mode

## Anti-patterns

- ❌ Theme hidden inside "UI Settings" expander (current state — fix)
- ❌ Theme toggle without a "system" option
- ❌ Density modes that drastically change the layout (only spacing changes; never element visibility)
- ❌ Forcing a theme based on time of day without user consent
- ❌ Theme that ignores user's saved preference and defaults to system on every load
