import { describe, expect, it, beforeEach } from "vitest"

import {
  formatCurrency,
  formatDate,
  formatDateTime,
  formatPartialDate,
  safeFilename,
  __resetIntlCachesForTests,
} from "@/lib/intl"
import { i18next, initI18n } from "@/i18n"

beforeEach(async () => {
  __resetIntlCachesForTests()
  await initI18n({ lng: "en" })
  await i18next.changeLanguage("en")
})

describe("formatCurrency", () => {
  it("renders en-US USD with two decimals", () => {
    expect(formatCurrency(1234.5, "USD", { locale: "en-US" })).toBe("$1,234.50")
  })

  it("renders cs-CZ CZK with the locale-specific separators", () => {
    // Intl chooses NBSP/NNBSP as the thousands separator in cs-CZ; assert via
    // a regex so the test isn't fragile to unicode whitespace differences
    // across Node versions.
    const out = formatCurrency(1234.5, "CZK", { locale: "cs-CZ" })
    expect(out).toMatch(/1\s?234,50/)
    expect(out).toMatch(/Kč/)
  })

  it("uses the i18next-resolved locale by default", () => {
    expect(formatCurrency(10, "USD")).toBe("$10.00")
  })

  // `compact: true` drops cents — matches PR #1678's dashboard-hero
  // pattern. JPY-style zero-fraction currencies stay unchanged because
  // they already render without cents.
  it("compact: true drops cents on a six-figure total", () => {
    expect(formatCurrency(329849.3, "CZK", { locale: "cs-CZ", compact: true })).toMatch(
      /^329\s?849\sKč$/
    )
  })

  // notation: "compact" switches to K/M/B form — the #1684 fix for
  // low-denomination currencies (HUF / IDR / VND / KRW / IRR / …) where
  // even cents-dropped totals run to 8–9 chars and still clip the
  // half-screen stat-card cell. We pin `maximumFractionDigits: 1` so a
  // non-round magnitude keeps one fractional digit ("$329.8K", "$1.2B")
  // — Intl's own default, but pinned explicitly to stay stable across
  // Node / browser versions.
  it('notation: "compact" renders six-figure USD as $329.8K', () => {
    expect(formatCurrency(329849, "USD", { locale: "en-US", notation: "compact" })).toBe("$329.8K")
  })

  it('notation: "compact" renders HUF 100,000,000 as the "HUF 100M" hero form', () => {
    // en-US locale on HUF renders as "HUF" + NNBSP + "100M". Compare
    // via regex so the test isn't fragile to which exact whitespace
    // code-point Intl picks across runtimes — the binding constraint
    // from #1684 is total *width*, and an 8-glyph string passes.
    const out = formatCurrency(1e8, "HUF", { locale: "en-US", notation: "compact" })
    expect(out).toMatch(/^HUF\s100M$/)
    expect(out.length).toBeLessThanOrEqual(8)
  })

  it('notation: "compact" surfaces a single fractional digit for non-round magnitudes', () => {
    // 1.23B fits the stat card; 1,234,567,890 does not. Pinning
    // maximumFractionDigits: 1 keeps the cache key stable across Node
    // versions where Intl's compact default has historically wobbled.
    expect(formatCurrency(1.234e9, "USD", { locale: "en-US", notation: "compact" })).toBe("$1.2B")
  })

  // notation: "compact" supersedes compact: true — both options were
  // valid in PR #1678, but Intl's compact form is already integer
  // (or `.x`) so they don't compose. Document the precedence so a
  // caller passing both doesn't accidentally re-introduce cents.
  it('notation: "compact" takes precedence over compact: true', () => {
    const a = formatCurrency(1e8, "USD", { locale: "en-US", notation: "compact" })
    const b = formatCurrency(1e8, "USD", {
      locale: "en-US",
      notation: "compact",
      compact: true,
    })
    expect(b).toBe(a)
  })

  // notation: "standard" is the explicit no-op — same output as
  // omitting the option entirely. Lets callers thread a runtime
  // decision (`useCompactNotation ? "compact" : "standard"`) without a
  // second formatCurrency call site.
  it('notation: "standard" matches the unconfigured default', () => {
    expect(formatCurrency(329849.3, "USD", { locale: "en-US", notation: "standard" })).toBe(
      formatCurrency(329849.3, "USD", { locale: "en-US" })
    )
  })
})

describe("formatDate / formatDateTime", () => {
  it("renders an ISO string in medium style for en-US", () => {
    expect(formatDate("2026-04-29", { locale: "en-US" })).toBe("Apr 29, 2026")
  })

  it("returns empty string for an invalid input", () => {
    expect(formatDate("not-a-date")).toBe("")
  })

  // Go's zero time (`0001-01-01T00:00:00Z`) is technically a valid Date but
  // semantically a "not set" sentinel — surfacing it as "January 1, 1" on
  // /profile (#1653 follow-up) is worse than rendering nothing, so the
  // formatter treats anything before 1900 as a placeholder.
  it("returns empty string for Go zero time", () => {
    expect(formatDate("0001-01-01T00:00:00Z")).toBe("")
  })

  it("returns empty string for any timestamp before 1900", () => {
    expect(formatDate("1899-12-31T23:59:59Z")).toBe("")
  })

  it("formatDateTime also drops Go zero time", () => {
    expect(formatDateTime("0001-01-01T00:00:00Z")).toBe("")
  })

  it("formatDateTime adds a time portion", () => {
    const out = formatDateTime("2026-04-29T15:30:00Z", {
      locale: "en-US",
      timeStyle: "short",
    })
    expect(out).toMatch(/Apr 29, 2026/)
  })

  // Issue #1680: when the History timeline renders an instant whose UTC
  // calendar day differs from the viewer's local one, the row's date
  // would disagree with the meta-grid "Date added" field that
  // `formatDate` UTC-pins for YYYY-MM-DD strings. Pinning `formatDateTime`
  // to UTC makes them line up. The fixture's UTC instant is May 16
  // (00:24 UTC); in any timezone west of UTC the *local* date is still
  // May 15, so a missing `timeZone: "UTC"` would render "May 15".
  it("formatDateTime pins to the requested timeZone", () => {
    const utc = formatDateTime("2024-05-16T00:24:00Z", {
      locale: "en-US",
      timeZone: "UTC",
    })
    expect(utc).toMatch(/May 16, 2024/)
    // Pin the rendered clock to the UTC instant — 12:24 AM, not the
    // viewer's local time (which would shift the hours by their offset).
    expect(utc).toMatch(/12:24/)
  })
})

describe("formatPartialDate", () => {
  it("renders full year/month/day", () => {
    expect(formatPartialDate({ year: 2026, month: 4, day: 29 }, { locale: "en-US" })).toBe(
      "Apr 29, 2026"
    )
  })

  it("renders year + month with the long month name", () => {
    expect(formatPartialDate({ year: 2026, month: 4 }, { locale: "en-US" })).toBe("April 2026")
  })

  it("renders year only as a plain number", () => {
    expect(formatPartialDate({ year: 1999 })).toBe("1999")
  })

  it("returns empty string for an empty PartialDate", () => {
    expect(formatPartialDate({})).toBe("")
  })
})

describe("safeFilename", () => {
  it("strips path separators and Windows-reserved characters", () => {
    // 9 unsafe chars in the input become 9 underscores: : / \ ? * " < > |.
    expect(safeFilename('my:file/with\\bad?chars*"<>|.txt')).toBe("my_file_with_bad_chars_____.txt")
  })

  it("collapses whitespace into single underscores", () => {
    expect(safeFilename("hello world\tnew  line")).toBe("hello_world_new_line")
  })

  it("trims leading and trailing dots and underscores", () => {
    expect(safeFilename("...weird.name...")).toBe("weird.name")
  })

  it("falls back when the result would be empty", () => {
    expect(safeFilename("///")).toBe("download")
    expect(safeFilename(" ", "fallback")).toBe("fallback")
  })

  it("keeps non-ASCII letters and non-whitespace punctuation intact", () => {
    // Cyrillic + Czech diacritics survive; the em-dash is left alone (it's
    // valid in modern filenames) while the surrounding spaces become "_".
    expect(safeFilename("Příručka — letní 2026")).toBe("Příručka_—_letní_2026")
  })
})
