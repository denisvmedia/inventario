# Formatting & Internationalization

How numbers, dates, currencies, and translations are handled.

## Locale strategy

- **Source language:** English (en-US base, en-GB-aware spelling — accept "color" or "colour" but standardize per locale file)
- **Initial supported locales:** en, ru, cs (Czech, given the obvious origin), de
- **Locale detection:** `navigator.language` → user setting (persisted) → fallback en
- **Locale switching:** instant, no full page reload (Vue I18n with composition API)

## Translation infrastructure

- Use `vue-i18n` v9+ with the composition API
- Translation files: `frontend/src/i18n/<locale>.json`, flat keys with namespacing (`form.label.name`, `error.network.offline`)
- ICU MessageFormat for plurals and gender (vue-i18n supports it via `@intlify/message-format`)
- No string concatenation in code — every user-facing string is a key
- Linter forbids hardcoded strings in `.vue` template `<template>` blocks

## Plural patterns

```json
{
  "things.count": "{count, plural, =0 {nothing here yet} one {# thing} other {# things}}"
}
```

Languages with complex plural rules (Russian: zero/one/few/many) are handled by ICU automatically:

```json
{
  "things.count.ru": "{count, plural, =0 {ничего пока} one {# вещь} few {# вещи} many {# вещей} other {# вещи}}"
}
```

Never hard-code "1 thing / 2 things" branches in JS.

## Date and time

### Display formats

| Context | Pattern (en) | Example |
| --- | --- | --- |
| Relative (recent) | `relative` (Intl.RelativeTimeFormat) | "3 hours ago" |
| Same-day exact | `h:mm a` | "4:12 PM" |
| Same-week | `EEEE 'at' h:mm a` | "Tuesday at 4:12 PM" |
| Same-year | `MMM d 'at' h:mm a` | "Apr 12 at 4:12 PM" |
| Older | `MMM d, yyyy` | "Apr 12, 2024" |
| Date-only short | `MMM d, yyyy` | "Apr 12, 2026" |
| Date-only long | `MMMM d, yyyy` | "April 12, 2026" |
| Iso for code/exports | `yyyy-MM-dd` | "2026-04-12" |

### Per-locale variations

| Locale | Date format example |
| --- | --- |
| en-US | Apr 12, 2026 |
| en-GB | 12 Apr 2026 |
| ru | 12 апр. 2026 |
| cs | 12. dub. 2026 |
| de | 12. Apr. 2026 |

### Implementation

Use `Intl.DateTimeFormat` natively, or wrap in a `useDateFormat` composable that:
- Accepts a `Date | string | number`
- Returns the appropriate display per "today" relative to now
- Locale-aware — auto-pulls from current locale
- SSR-safe

### Time zones

- All timestamps stored in UTC server-side (assumed; verify in backend)
- Display in user's local time zone (`Intl.DateTimeFormat` does this automatically)
- For shared/multi-user contexts: tooltip shows UTC offset on hover ("Apr 12 at 4:12 PM (UTC+2)")

### Date input

Always use native `<input type="date">` with the browser's locale. Avoid custom JS date pickers unless required (range pickers for filters use a popover-styled custom). Custom date pickers must:
- Match the browser locale
- Show today by default
- Provide keyboard navigation
- Allow direct typing in `YYYY-MM-DD` or locale-friendly format

## Numbers

### Display rules

| Context | Pattern | Example |
| --- | --- | --- |
| Plain integer | `Intl.NumberFormat` | "1,234" or "1 234" (locale-grouped) |
| Decimal | locale grouping + 2 decimals | "1,234.56" or "1 234,56" |
| Percentage | `style: 'percent', minimumFractionDigits: 1` | "12.4%" |
| Compact | `notation: 'compact'` for large nums | "1.2K", "3.4M" |
| Tabular columns | `font-variant-numeric: tabular-nums` always |
| Negative | en-dash prefix, not minus-hyphen | "−12" not "-12" (typographic) |

### Locale grouping

`1,234,567.89` (en-US, en-GB) vs `1 234 567,89` (cs, fr, ru) vs `1.234.567,89` (de, es). Handled automatically by `Intl.NumberFormat`.

## Currency

### Storage

- Money stored as integer minor units (cents) on backend
- Currency code (ISO 4217) stored alongside per-record
- Multi-currency support implied: a wine collection valued in EUR while household in CZK

### Display

```
17 500.00 CZK     ← preferred Inventario format (space-separated, currency code suffix)
17,500.00 €       ← alternative if symbol-leading; not recommended (mixes en-US grouping with non-USD)
```

**Decision:** use ISO code as **suffix** with non-breaking space, locale-grouped digits, always 2 decimals.

```ts
new Intl.NumberFormat(userLocale, {
  style: 'currency',
  currency: record.currency,
  currencyDisplay: 'code',
}).format(record.amount / 100);
```

### Currency symbols vs codes

Symbols ($, €, £) ambiguous (USD vs CAD vs AUD all show "$"). Codes (USD, EUR, GBP, CZK) explicit. Inventario uses **codes**, not symbols.

### Mixed-currency aggregates

When totaling across multi-currency records (dashboard "Total inventory value"):
- Show in **user's primary currency** (settable in profile)
- Apply latest exchange rate (server-side, cached daily)
- Display with hint: "Approximately 167,000 CZK" with hover-tooltip showing the underlying mix
- Never silently sum without conversion — would mislead users

## Units

Inventario doesn't currently surface lengths/weights/volumes for items, but if added (e.g., "wine bottle 750ml"):
- User profile: imperial / metric preference
- Display per preference, store as one canonical unit (likely metric)
- Intl support is weak for units beyond simple cases — wrap in helper

## Address formatting

User addresses (for Place entities) and supplier addresses:
- Free-form multi-line text fields, not structured (avoids the rabbit hole of country-specific address schemas)
- Display preserves line breaks
- For invoices/exports, optional structured fields (street / city / postal / country) added in a v2

## Phone numbers

If added (supplier contact, warranty registration):
- Store as E.164 (`+420 123 456 789` → `+420123456789`)
- Display with locale-aware spacing via `libphonenumber-js`

## Right-to-left (RTL) support

Inventario does not need RTL in v1, but the design system is RTL-ready:

- Use logical CSS properties: `padding-inline-start`, `margin-inline-end`, `inset-inline`, etc.
- Avoid `left` / `right`; use `start` / `end`
- Icons that have directionality (chevrons, arrows) flip via `dir="rtl"` selector on body
- Tailwind v4 supports logical properties out of the box

## File and ID formatting

- File sizes: `Intl.NumberFormat` with custom unit logic (`1.2 MB`, `345 KB`, `2.3 GB`)
- UUIDs: truncate to first 8 + last 4 with em-dash separator: `3fd5fc3e—b67d`
- Click-to-copy on truncated IDs

## Sort order

Locale-aware string sort via `Intl.Collator`:
- "Áéí" sorts properly in Spanish/French
- Numerics within strings (file-1, file-2, file-10) sort naturally with `numeric: true`

```ts
const collator = new Intl.Collator(userLocale, { numeric: true, sensitivity: 'base' });
items.sort((a, b) => collator.compare(a.name, b.name));
```

## Plural-aware UI strings

Avoid the trap of "1 thing(s)" or "0 things found". Even simple cases use plural format:
- "1 result" / "5 results" / "0 results" — handled via ICU
- "Showing X of Y" → "Showing 1–12 of 142 things" (where "things" is ICU-pluralized)

## What ships in sprint 0

1. Set up vue-i18n with composition API
2. Extract every hard-coded user-facing string into `i18n/en.json`
3. Stub locale files for ru, cs, de (translations come later)
4. Implement `useDateFormat`, `useNumberFormat`, `useCurrencyFormat` composables
5. Add `<output role="status">` for live regions in async-update areas
6. CI lint forbids hard-coded strings in `<template>`
