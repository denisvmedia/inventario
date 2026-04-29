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

type FormatterKey = string

const numberFormatters = new Map<FormatterKey, Intl.NumberFormat>()
const dateFormatters = new Map<FormatterKey, Intl.DateTimeFormat>()

function currentLocale(): string {
  // Prefer i18next's resolved language so a Settings-page override flows
  // through here too. Falls back to the browser if i18next hasn't booted
  // (early errors, tests).
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
export function formatCurrency(
  amount: number,
  currency: string,
  opts: { locale?: string } = {}
): string {
  const locale = opts.locale ?? currentLocale()
  return getNumberFormatter(locale, {
    style: "currency",
    currency,
    // Most currencies use the ISO-defined fraction digit count; let Intl
    // decide so JPY shows 0 decimals while USD shows 2.
  }).format(amount)
}

export type DateStyle = "short" | "medium" | "long" | "full"

// formatDate handles a typical Date or an ISO string. Style maps to the
// locale-specific short/medium/long pattern (en-US "Apr 29, 2026" vs. cs-CZ
// "29. 4. 2026" etc.).
export function formatDate(
  value: Date | string,
  opts: { style?: DateStyle; locale?: string } = {}
): string {
  const locale = opts.locale ?? currentLocale()
  const date = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(date.getTime())) return ""
  return getDateFormatter(locale, { dateStyle: opts.style ?? "medium" }).format(date)
}

// formatDateTime: same as formatDate but with the time portion. Use when
// the user needs to see both (e.g. activity log timestamps).
export function formatDateTime(
  value: Date | string,
  opts: { dateStyle?: DateStyle; timeStyle?: DateStyle; locale?: string } = {}
): string {
  const locale = opts.locale ?? currentLocale()
  const date = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(date.getTime())) return ""
  return getDateFormatter(locale, {
    dateStyle: opts.dateStyle ?? "medium",
    timeStyle: opts.timeStyle ?? "short",
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
  if (p.year && p.month && p.day) {
    const d = new Date(Date.UTC(p.year, p.month - 1, p.day))
    return getDateFormatter(locale, { dateStyle: "medium" }).format(d)
  }
  if (p.year && p.month) {
    // Year + month only — use the long month name + year. Intl's "year +
    // month" combo does the right thing per locale.
    const d = new Date(Date.UTC(p.year, p.month - 1, 1))
    return getDateFormatter(locale, { year: "numeric", month: "long" }).format(d)
  }
  if (p.year) {
    return String(p.year)
  }
  return ""
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
}
