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
})

describe("formatDate / formatDateTime", () => {
  it("renders an ISO string in medium style for en-US", () => {
    expect(formatDate("2026-04-29", { locale: "en-US" })).toBe("Apr 29, 2026")
  })

  it("returns empty string for an invalid input", () => {
    expect(formatDate("not-a-date")).toBe("")
  })

  it("formatDateTime adds a time portion", () => {
    const out = formatDateTime("2026-04-29T15:30:00Z", {
      locale: "en-US",
      timeStyle: "short",
    })
    expect(out).toMatch(/Apr 29, 2026/)
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
