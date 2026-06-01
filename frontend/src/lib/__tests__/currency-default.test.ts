import { afterEach, describe, expect, it, vi } from "vitest"
import i18next from "i18next"

import { inferDefaultCurrency } from "@/lib/currency-default"

// Stub navigator.language for the duration of one call. Returns a restore
// fn so each case cleans up.
function withLocale(locale: string | undefined): () => void {
  const spy = vi.spyOn(navigator, "language", "get").mockReturnValue(locale as string)
  return () => spy.mockRestore()
}

// Override the app's resolved UI language (what i18next persists from
// localStorage) for one case. Returns a restore fn.
function withAppLanguage(lang: string | undefined): () => void {
  const target = i18next as { resolvedLanguage?: string }
  const orig = target.resolvedLanguage
  target.resolvedLanguage = lang
  return () => {
    target.resolvedLanguage = orig
  }
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe("inferDefaultCurrency", () => {
  it("maps a region-tagged locale to its ISO-4217 currency", () => {
    const restore = withLocale("cs-CZ")
    expect(inferDefaultCurrency()).toBe("CZK")
    restore()
  })

  it("maps ru-RU to RUB", () => {
    const restore = withLocale("ru-RU")
    expect(inferDefaultCurrency()).toBe("RUB")
    restore()
  })

  it("maps en-US to USD", () => {
    const restore = withLocale("en-US")
    expect(inferDefaultCurrency()).toBe("USD")
    restore()
  })

  it("falls back to the bare-language map when there is no region tag", () => {
    const restore = withLocale("cs")
    // "cs" has no region segment → LANG_TO_CURRENCY lookup → CZK.
    expect(inferDefaultCurrency()).toBe("CZK")
    restore()
  })

  it("falls back to USD for an unknown locale", () => {
    const restore = withLocale("xx-ZZ")
    expect(inferDefaultCurrency()).toBe("USD")
    restore()
  })

  it("falls back to USD for an empty/garbage locale", () => {
    const restore = withLocale("")
    expect(inferDefaultCurrency()).toBe("USD")
    restore()
  })

  it("prefers a deliberately-chosen non-English UI language over the browser region", () => {
    // App switched to Russian (persisted in localStorage → i18next) on an
    // en-US browser. The explicit choice wins: RUB, not USD.
    const restoreLang = withAppLanguage("ru")
    const restoreLocale = withLocale("en-US")
    expect(inferDefaultCurrency()).toBe("RUB")
    restoreLocale()
    restoreLang()
  })

  it("keeps the browser region when the UI language is the default English", () => {
    // en is the default/fallback, not a deliberate switch, so the browser
    // region stays authoritative: en-GB → GBP.
    const restoreLang = withAppLanguage("en")
    const restoreLocale = withLocale("en-GB")
    expect(inferDefaultCurrency()).toBe("GBP")
    restoreLocale()
    restoreLang()
  })
})
