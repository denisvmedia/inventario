# Formatting & i18n

How to write a number, a date, a currency, a plural — once — across
the supported locales. **Committed.** All formatters live in
`frontend/src/lib/intl.ts`; never call `new Intl.*` from a component.

## Locales

`en` (source of truth, bundled), `cs` (lazy), `ru` (lazy). See
[../i18n.md](../i18n.md).

The locale used by formatters is `i18next.resolvedLanguage`, cached at
formatter-construction time. When the user changes locale (Settings →
Language), the helpers re-construct on the next render.

## Numbers

`intl.ts` exposes purpose-built helpers — `formatCurrency`,
`formatBytes`, `formatPartialDate` — rather than a single generic
`formatNumber`. Each wraps a cached `Intl.NumberFormat` /
`Intl.DateTimeFormat` keyed on the resolved locale (see the
`getNumberFormatter` helper in `frontend/src/lib/intl.ts`). Reach for
the helper that matches the value's meaning; don't hand-roll number
formatting in a component.

- **Tabular nums in columns.** `font-mono tabular-nums` (or
  `tabular-nums` on a sans face) so columns align. See [02-typography.md](02-typography.md).
- **Don't `.toFixed()` in components.** Always go through a formatter.
- **Don't `Intl.NumberFormat` inline** — the constructor is expensive
  and the locale is mutable. The cached formatters in `intl.ts` exist
  precisely so callers never construct one ad hoc.

## Currency

```ts
import { formatCurrency } from "@/lib/intl"

formatCurrency(1234.5, "USD")  // "$1,234.50" / "1 234,50 $" / "1 234,50 $"
formatCurrency(-50, "EUR")     // "-€50.00" / "-50,00 €" / "-50,00 €"
```

- **Currency code is always required.** No "default to USD" anywhere
  in the UI.
- **Use `<CurrencyCombobox>`** for currency input — never a plain
  `<Select>` with three hardcoded currencies.
- **Negative values** prefix the locale's negative sign (`−`/`-`)
  consistently. The formatter handles sign placement.

## Dates and times

```ts
import { formatDate, formatDateTime, formatRelative, formatPartialDate } from "@/lib/intl"

formatDate("2026-04-28")           // "Apr 28, 2026" / "28. 4. 2026" / "28 апреля 2026 г."
formatDateTime("2026-04-28T10:00") // "Apr 28, 2026, 10:00 AM" / …
formatRelative(date)               // "2 days ago" / "before 2 days" / "2 дня назад"
formatPartialDate({ year: 2026, month: 4 })  // "April 2026" — year/month/day in any combo
```

There is no standalone `formatTime`: a time always renders through
`formatDateTime` (pass `dateStyle`/`timeStyle` to tune it). Use
`formatPartialDate` for backend `PDate` shapes where only some of
year/month/day are present.

Conventions:

- **Months are spelled out, never numeric**, except in dense column
  contexts. "Apr 28" beats "04/28" — readable in en/cs/ru without a
  format-confusion footgun.
- **24h vs 12h** is the locale's default. Don't override.
- **Relative for ≤ 7 days, absolute otherwise** is the rule of thumb.
  `formatRelative` falls back to absolute past 7 days.
- **Time zones**: dates from the API are UTC. Display in the user's
  local TZ — `Intl` does this by default. Don't add a TZ suffix unless
  the surface explicitly needs it (calendar apps, audit logs).

## Plurals

i18next picks the plural form via the `count` key:

```ts
t("commodities:bulk.deleted", { count })
// _one: "{count} item deleted"
// _other: "{count} items deleted"
```

Closed enums in `preservePatterns` per `frontend/i18next.config.ts`.

For Russian / Czech, i18next picks `_few`, `_many` etc. when the
catalog provides them — see the en bundle for the canonical key shapes.

## Lists

`intl.ts` does not yet ship a list-join helper. When a surface needs
locale-correct "A, B, and C" joining, reach for `Intl.ListFormat`
directly:

```ts
new Intl.ListFormat("en", { type: "conjunction" }).format(
  ["Kitchen", "Bedroom", "Garage"]
) // "Kitchen, Bedroom, and Garage" (en)
  // "Kitchen, Bedroom a Garage" (cs) · "Kitchen, Bedroom и Garage" (ru)
```

For "X, Y, and 3 others" surfaces, build the truncation in code first,
then join. If a list join becomes common, promote it to a cached
`formatList` helper in `intl.ts` (mirroring the existing formatters)
rather than scattering `Intl.ListFormat` constructions.

## File sizes

```ts
import { formatBytes } from "@/lib/intl"

formatBytes(1024)        // "1.00 KiB"
formatBytes(1_500_000)   // "1.43 MiB"
```

Always binary (IEC) units — 1024-stepped suffixes `B`/`KiB`/`MiB`/
`GiB`/`TiB`, matching OS file managers. The fraction-digit count
adapts to the magnitude (2 below 10, 1 below 100, 0 otherwise). Don't
write "1,500,000 bytes" in user-facing copy.

## Identifiers

API identifiers (UUIDs, slugs) are **never** formatted. Show them
verbatim if shown at all (settings page, debug surfaces). Don't
truncate ("a1b2c…f9").

## Form input

| Field | Input behavior |
| --- | --- |
| Currency | Numeric input + currency selector (`<CurrencyCombobox>`). The decimal separator is the locale's; the formatter writes back the user's input on blur. |
| Date | `<input type="date">` is the floor. A custom `<Popover>` calendar can replace it for surfaces that need range selection. Always store ISO 8601 (`YYYY-MM-DD`). |
| Time | `<input type="time">`. Locale-aware 12h/24h. |
| Numeric quantity | `<input type="number" inputMode="numeric">`. Don't pre-format with thousands separators inside a number input — the browser parses it as one number, the user sees grouped digits. |

## Locale-specific gotchas

- **`cs` decimal is comma**, thousand sep is non-breaking space.
- **`ru` decimal is comma**, thousand sep is non-breaking space.
- **`en` decimal is period**, thousand sep is comma.

Don't hand-write decimals — `Intl.NumberFormat` does it. The bug
people hit: typing "1,234.5" into a parsed input on a `cs` locale —
the comma is the decimal separator, so `1.234,5` is what `Intl` would
write. Round-tripping through the formatter avoids the issue.

- **Russian month genitive**: "28 апреля 2026 г." (genitive case) —
  `Intl` handles this with `dateStyle: "long"`. Don't manually cap
  month names.
- **Czech ordering**: "28. 4. 2026" with periods. `Intl` produces this
  with `dateStyle: "short"`.

## RTL

Inventario doesn't currently ship an RTL locale. The CSS uses logical
properties where possible (`ms-*`, `me-*`, `ps-*`, `pe-*`) so an RTL
locale could land without a refit. Adding one is out of scope of this
brief; see [00-positioning.md](00-positioning.md)'s "Out of scope" list.

## Hard rules

1. **All formatting via `src/lib/intl.ts`.** No `Intl.NumberFormat`,
   `Intl.DateTimeFormat`, `Intl.RelativeTimeFormat`,
   `Intl.PluralRules` inside components.
2. **Currency code is always explicit.** No silent USD default.
3. **Months are spelled** outside dense columns.
4. **ISO 8601 for storage.** Never store locale-formatted strings.
5. **Plurals via i18next.** No hand-rolled `count === 1 ? "item" : "items"`.

## Anti-patterns

- `${value.toLocaleString()}` — uses the *runtime* locale, not the
  i18next-resolved one. Subtle bug after the user changes language.
- `value.toFixed(2)` for currency — locale-blind, drops the symbol.
- `new Date().toLocaleDateString()` — same locale-blindness.
- `${count} item${count === 1 ? "" : "s"}` — i18next handles this.
- Concatenating `formatDate` with a hand-built time string instead of
  `formatDateTime` — the joiner ("at", " · ") is locale-specific.

## Cross-refs

- i18n engineering rules: [../i18n.md](../i18n.md).
- Voice / copy: [12-tone-of-voice-and-copy.md](12-tone-of-voice-and-copy.md).
- `intl.ts` source: `frontend/src/lib/intl.ts`.
