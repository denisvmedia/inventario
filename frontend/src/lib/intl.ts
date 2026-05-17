// Locale-aware formatting helpers backed by the platform Intl APIs. We
// deliberately do NOT pull in date-fns/numeral.js — `Intl.NumberFormat` and
// `Intl.DateTimeFormat` cover the required cases (currency, short/medium/
// long dates) and ship for free with the runtime, so adding 60kb of locale
// data on top would only buy us pretty plurals (which we get from i18next
// instead).
//
// The helpers cache the underlying formatter instances by their (locale,
// options) tuple. Constructing `Intl.NumberFormat` / `Intl.DateTimeFormat`
// is non-trivial; the cache prevents constructing the same one per render.

import { i18next } from "@/i18n"
import {
  getNumberFormatLocaleOverride,
  subscribeNumberFormatLocale,
} from "@/lib/numberFormatLocale"

type FormatterKey = string

const numberFormatters = new Map<FormatterKey, Intl.NumberFormat>()
const dateFormatters = new Map<FormatterKey, Intl.DateTimeFormat>()

// When the override changes drop the cached formatters so the next
// call rebuilds with the new locale. The cache key already includes
// the resolved locale, but holding old entries leaks memory in the
// (uncommon) case the user flips the dropdown repeatedly.
subscribeNumberFormatLocale(() => {
  numberFormatters.clear()
  dateFormatters.clear()
})

// currentLocale resolves the BCP-47 tag used by every formatter in this
// module. It decouples *what language the strings are in*
// (i18next.resolvedLanguage — controls translations) from *what
// rules numbers / dates / currencies follow* (the locale here). Order:
//   1. User-set override from Settings → Appearance → Region & formatting.
//   2. `navigator.language` — the browser's locale (e.g. "cs-CZ" for a
//      Czech user even when the UI is in English). Auto-detect lets a
//      fresh account see locally-correct currency before the user has
//      touched any preference.
//   3. i18next's resolved language as the final fallback.
//   4. "en" if i18next hasn't booted yet (very early errors, tests).
function currentLocale(): string {
  const override = getNumberFormatLocaleOverride()
  if (override) return override
  if (typeof navigator !== "undefined" && navigator.language) {
    return navigator.language
  }
  return i18next.resolvedLanguage || i18next.language || "en"
}

function getNumberFormatter(locale: string, opts: Intl.NumberFormatOptions): Intl.NumberFormat {
  const key = `${locale}::${JSON.stringify(opts)}`
  let f = numberFormatters.get(key)
  if (!f) {
    f = new Intl.NumberFormat(locale, opts)
    numberFormatters.set(key, f)
  }
  return f
}

function getDateFormatter(locale: string, opts: Intl.DateTimeFormatOptions): Intl.DateTimeFormat {
  const key = `${locale}::${JSON.stringify(opts)}`
  let f = dateFormatters.get(key)
  if (!f) {
    f = new Intl.DateTimeFormat(locale, opts)
    dateFormatters.set(key, f)
  }
  return f
}

// formatCurrency renders an amount with the right symbol, separators, and
// digit count for the active locale. `currency` is an ISO 4217 code
// (USD/EUR/CZK/RUB) — Intl picks the symbol and the locale-specific decimal
// rules.
//
// `compact: true` drops decimals entirely (`maximumFractionDigits: 0` plus
// `minimumFractionDigits: 0`) — mirrors the design-mock `formatCurrency`
// pattern for stat-card / hero surfaces where the long form ("CZK
// 329,849.30") clips at narrow widths and the cents are noise next to
// six-figure totals. Detail / print / list surfaces should keep the
// default (full precision) so per-item prices don't lose their cents.
//
// `notation: "compact"` switches to Intl's K/M/B compact form ("$329K",
// "HUF 100M") — needed for low-denomination currencies (HUF, IDR, VND,
// KRW, …) where even "no-cents" totals can run to 8–9 digits and still
// clip the half-screen stat-card cell on mobile (#1684). Use sparingly:
// the reading changes from a precise figure to a magnitude, so reserve
// it for hero surfaces where width pressure is the binding constraint.
// When `notation: "compact"` is set, `compact: true` is moot — Intl's
// compact form is already integer (or `.x`) per its own rules.
export function formatCurrency(
  amount: number,
  currency: string,
  opts: { locale?: string; compact?: boolean; notation?: "standard" | "compact" } = {}
): string {
  const locale = opts.locale ?? currentLocale()
  const isCompactNotation = opts.notation === "compact"
  return getNumberFormatter(locale, {
    style: "currency",
    currency,
    // Most currencies use the ISO-defined fraction digit count; let Intl
    // decide so JPY shows 0 decimals while USD shows 2 — unless the
    // caller asks for the compact whole-amount form or compact-notation
    // K/M/B form.
    ...(isCompactNotation
      ? // `1.2M` reads better than `1M` for non-round magnitudes —
        // matches Intl's own default for `notation: "compact"` but pin
        // it explicitly so the cache key stays stable across Node /
        // browser versions that might differ on the unspecified default.
        { notation: "compact", maximumFractionDigits: 1 }
      : opts.compact
        ? { maximumFractionDigits: 0, minimumFractionDigits: 0 }
        : {}),
  }).format(amount)
}

export type DateStyle = "short" | "medium" | "long" | "full"

// ISO date-only strings ("YYYY-MM-DD") with no time/offset. `new Date()`
// parses these as UTC midnight per the spec, so formatting in the local
// timezone shifts the rendered day west of UTC. We detect this shape and
// pin the formatter to UTC so the calendar date stays stable.
const ISO_DATE_ONLY = /^\d{4}-\d{2}-\d{2}$/

// MIN_PLAUSIBLE_YEAR keeps Go zero-time (`0001-01-01T00:00:00Z`) and other
// epoch-style placeholders from rendering as "January 1, 1" / "1/1/1". A
// 1900 floor leaves real historical dates (vintage commodity purchases,
// inherited estate items) untouched while catching every plausible null
// sentinel a backend might emit. Treat anything older as "no date".
const MIN_PLAUSIBLE_YEAR = 1900

function isPlaceholderDate(date: Date): boolean {
  return Number.isNaN(date.getTime()) || date.getUTCFullYear() < MIN_PLAUSIBLE_YEAR
}

// formatDate handles a typical Date or an ISO string. Style maps to the
// locale-specific short/medium/long pattern (en-US "Apr 29, 2026" vs. cs-CZ
// "29. 4. 2026" etc.). For ISO date-only strings the formatter pins to UTC
// so a backend-supplied calendar date renders the same day for every user.
// For full Date instances and ISO timestamps we leave the local TZ default
// in place (the timestamp carries an instant; the user wants their wall
// clock). Zero / sentinel timestamps (`0001-01-01`, anything before 1900)
// return "" so callers like ProfilePage's "Member since {date}" don't
// surface a bogus year — the same placeholder behaviour as the existing
// NaN branch.
export function formatDate(
  value: Date | string,
  opts: { style?: DateStyle; locale?: string } = {}
): string {
  const locale = opts.locale ?? currentLocale()
  const isDateOnly = typeof value === "string" && ISO_DATE_ONLY.test(value)
  const date = value instanceof Date ? value : new Date(value)
  if (isPlaceholderDate(date)) return ""
  return getDateFormatter(locale, {
    dateStyle: opts.style ?? "medium",
    ...(isDateOnly ? { timeZone: "UTC" } : {}),
  }).format(date)
}

// formatRelative formats an instant relative to now ("5 minutes ago",
// "in 2 days") via Intl.RelativeTimeFormat. Empty or invalid input
// returns the empty string so callers can suffix it conditionally
// without guarding the result themselves. The unit-bucket boundaries
// (minute / hour / day) match what the BE-driven UIs need today — the
// /profile/sessions and /profile/login-history surfaces never need
// week / month granularity inside the 90-day retention window.
const relativeTimeFormatters = new Map<string, Intl.RelativeTimeFormat>()
function getRelativeTimeFormatter(locale: string): Intl.RelativeTimeFormat {
  let f = relativeTimeFormatters.get(locale)
  if (!f) {
    f = new Intl.RelativeTimeFormat(locale, { numeric: "auto" })
    relativeTimeFormatters.set(locale, f)
  }
  return f
}

export function formatRelative(value: Date | string, opts: { locale?: string } = {}): string {
  if (!value) return ""
  const locale = opts.locale ?? currentLocale()
  const date = value instanceof Date ? value : new Date(value)
  const t = date.getTime()
  if (!Number.isFinite(t)) return ""
  const diff = t - Date.now()
  const abs = Math.abs(diff)
  const rtf = getRelativeTimeFormatter(locale)
  const min = 60 * 1000
  const hour = 60 * min
  const day = 24 * hour
  if (abs < min) return rtf.format(Math.round(diff / 1000), "second")
  if (abs < hour) return rtf.format(Math.round(diff / min), "minute")
  if (abs < day) return rtf.format(Math.round(diff / hour), "hour")
  return rtf.format(Math.round(diff / day), "day")
}

// formatDateTime: same as formatDate but with the time portion. Use when
// the user needs to see both (e.g. activity log timestamps). Pass an
// explicit `timeZone` (e.g. "UTC") to align the rendered calendar day
// with a sibling date-only field that was already UTC-pinned by
// `formatDate` — without it, an instant near UTC midnight straddles the
// day boundary in the viewer's local TZ and disagrees with a YYYY-MM-DD
// counterpart (issue #1680). Intl forbids combining `dateStyle`/`timeStyle`
// with `timeZoneName`, so callers that need a "UTC" suffix should append
// it themselves rather than asking for it via Intl options.
export function formatDateTime(
  value: Date | string,
  opts: {
    dateStyle?: DateStyle
    timeStyle?: DateStyle
    locale?: string
    timeZone?: string
  } = {}
): string {
  const locale = opts.locale ?? currentLocale()
  const date = value instanceof Date ? value : new Date(value)
  if (isPlaceholderDate(date)) return ""
  return getDateFormatter(locale, {
    dateStyle: opts.dateStyle ?? "medium",
    timeStyle: opts.timeStyle ?? "short",
    ...(opts.timeZone ? { timeZone: opts.timeZone } : {}),
  }).format(date)
}

// PartialDate matches the backend's PDate shape: any combination of year /
// month / day. We render whatever's present, locale-aware. "2024" → "2024".
// "2024-04" → "April 2024". "2024-04-29" → full date.
export interface PartialDate {
  year?: number
  month?: number // 1-12 (NOT zero-indexed — backend convention)
  day?: number
}

export function formatPartialDate(p: PartialDate, opts: { locale?: string } = {}): string {
  const locale = opts.locale ?? currentLocale()
  // PDate is a calendar date with no timezone — we build the underlying
  // Date in UTC and pin the formatter to UTC so a user east or west of the
  // construction tz never sees the day or month shift one over.
  if (p.year && p.month && p.day) {
    const d = new Date(Date.UTC(p.year, p.month - 1, p.day))
    return getDateFormatter(locale, { dateStyle: "medium", timeZone: "UTC" }).format(d)
  }
  if (p.year && p.month) {
    // Year + month only — use the long month name + year. Intl's "year +
    // month" combo does the right thing per locale.
    const d = new Date(Date.UTC(p.year, p.month - 1, 1))
    return getDateFormatter(locale, {
      year: "numeric",
      month: "long",
      timeZone: "UTC",
    }).format(d)
  }
  if (p.year) {
    return String(p.year)
  }
  return ""
}

// formatBytes renders a byte count with locale-aware decimal separators
// and a binary suffix (KiB, MiB, GiB). The threshold of 1024 matches the
// numbers users see in OS file managers and in the BE storage backends.
// Negative or non-finite values fall back to "—" so a malformed
// payload doesn't crash the row.
const BINARY_SUFFIXES = ["B", "KiB", "MiB", "GiB", "TiB"] as const
export function formatBytes(value: number, opts: { locale?: string } = {}): string {
  if (!Number.isFinite(value) || value < 0) return "—"
  const locale = opts.locale ?? currentLocale()
  let unit = 0
  let amount = value
  while (amount >= 1024 && unit < BINARY_SUFFIXES.length - 1) {
    amount /= 1024
    unit += 1
  }
  const fractionDigits = unit === 0 ? 0 : amount >= 100 ? 0 : amount >= 10 ? 1 : 2
  const number = getNumberFormatter(locale, {
    minimumFractionDigits: fractionDigits,
    maximumFractionDigits: fractionDigits,
  }).format(amount)
  return `${number} ${BINARY_SUFFIXES[unit]}`
}

// safeFilename converts a label into something safe to use in a download
// `filename=` header. Strips path separators and Windows-reserved chars,
// collapses any whitespace into a single underscore, and trims leading/
// trailing punctuation. Non-ASCII letters are kept — the browser
// percent-encodes them via the RFC 5987 `filename*=UTF-8''...` form.
const FILENAME_UNSAFE = /[\\/:*?"<>|]/g
const FILENAME_WHITESPACE = /\s+/g
export function safeFilename(label: string, fallback = "download"): string {
  const cleaned = label
    .replace(FILENAME_UNSAFE, "_")
    .replace(FILENAME_WHITESPACE, "_")
    .replace(/^[._]+|[._]+$/g, "")
  return cleaned || fallback
}

// Test-only: drop the formatter caches so test cases that flip the locale
// don't see stale formatters from a previous test.
export function __resetIntlCachesForTests(): void {
  numberFormatters.clear()
  dateFormatters.clear()
  relativeTimeFormatters.clear()
}
