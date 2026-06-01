import { afterEach, describe, expect, it, vi } from "vitest"

import { inferDefaultCurrency } from "@/lib/currency-default"

// Stub navigator.language for the duration of one call. Returns a restore
// fn so each case cleans up.
function withLocale(locale: string | undefined): () => void {
  const spy = vi.spyOn(navigator, "language", "get").mockReturnValue(locale as string)
  return () => spy.mockRestore()
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
})
