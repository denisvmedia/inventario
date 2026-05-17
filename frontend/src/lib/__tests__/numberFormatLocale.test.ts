import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import {
  NUMBER_FORMAT_LOCALE_STORAGE_KEY,
  getNumberFormatLocaleOverride,
  setNumberFormatLocaleOverride,
  subscribeNumberFormatLocale,
  __resetNumberFormatLocaleOverrideForTests,
} from "@/lib/numberFormatLocale"

// Vitest's jsdom env wipes localStorage between files but not between
// `it()` cases, so each test owns its own reset.
beforeEach(() => {
  __resetNumberFormatLocaleOverrideForTests()
})

afterEach(() => {
  __resetNumberFormatLocaleOverrideForTests()
})

describe("numberFormatLocale store", () => {
  it("reads back the value set via setNumberFormatLocaleOverride", () => {
    setNumberFormatLocaleOverride("cs-CZ")
    expect(getNumberFormatLocaleOverride()).toBe("cs-CZ")
  })

  it("treats empty string and null as clears (null returned, storage removed)", () => {
    setNumberFormatLocaleOverride("cs-CZ")
    setNumberFormatLocaleOverride("")
    expect(getNumberFormatLocaleOverride()).toBeNull()
    expect(window.localStorage.getItem(NUMBER_FORMAT_LOCALE_STORAGE_KEY)).toBeNull()
  })

  it("persists non-empty values to localStorage so cold-boot picks them up", () => {
    setNumberFormatLocaleOverride("de-DE")
    expect(window.localStorage.getItem(NUMBER_FORMAT_LOCALE_STORAGE_KEY)).toBe("de-DE")
  })

  it("hydrates from localStorage on first read after a reset", () => {
    window.localStorage.setItem(NUMBER_FORMAT_LOCALE_STORAGE_KEY, "ja-JP")
    // Reset the in-memory cache without touching storage to simulate a
    // page reload.
    __resetNumberFormatLocaleOverrideForTests()
    window.localStorage.setItem(NUMBER_FORMAT_LOCALE_STORAGE_KEY, "ja-JP")
    expect(getNumberFormatLocaleOverride()).toBe("ja-JP")
  })

  it("notifies subscribers on change and stops once unsubscribed", () => {
    const fn = vi.fn()
    const unsubscribe = subscribeNumberFormatLocale(fn)
    setNumberFormatLocaleOverride("hu-HU")
    expect(fn).toHaveBeenCalledTimes(1)
    unsubscribe()
    setNumberFormatLocaleOverride("ru-RU")
    expect(fn).toHaveBeenCalledTimes(1)
  })

  it("does not notify when the value is unchanged", () => {
    setNumberFormatLocaleOverride("cs-CZ")
    const fn = vi.fn()
    subscribeNumberFormatLocale(fn)
    setNumberFormatLocaleOverride("cs-CZ")
    expect(fn).not.toHaveBeenCalled()
  })
})
