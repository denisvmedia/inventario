import { afterEach, beforeEach, describe, expect, it } from "vitest"

import {
  clearPendingFirstItem,
  consumePendingFirstItem,
  peekPendingFirstItem,
  savePendingFirstItem,
  type PendingFirstItem,
} from "@/features/auth/firstItemHandoff"

const STORAGE_KEY = "inventario_pending_first_item"

const sample: PendingFirstItem = {
  draftKey: "commodity-draft:anon:create",
  currency: "CZK",
  savedAt: 1_700_000_000_000,
}

beforeEach(() => {
  window.localStorage.clear()
})

afterEach(() => {
  window.localStorage.clear()
})

describe("firstItemHandoff", () => {
  it("save → peek round-trips the marker without removing it", () => {
    savePendingFirstItem(sample)
    expect(peekPendingFirstItem()).toEqual(sample)
    // Peek is non-destructive.
    expect(peekPendingFirstItem()).toEqual(sample)
  })

  it("consume returns the marker and clears it", () => {
    savePendingFirstItem(sample)
    expect(consumePendingFirstItem()).toEqual(sample)
    // Gone after consume.
    expect(peekPendingFirstItem()).toBeNull()
    expect(consumePendingFirstItem()).toBeNull()
  })

  it("clear removes a stored marker", () => {
    savePendingFirstItem(sample)
    clearPendingFirstItem()
    expect(peekPendingFirstItem()).toBeNull()
  })

  it("peek returns null when nothing is stored", () => {
    expect(peekPendingFirstItem()).toBeNull()
  })

  it("tolerates corrupt JSON by returning null", () => {
    window.localStorage.setItem(STORAGE_KEY, "{not valid json")
    expect(peekPendingFirstItem()).toBeNull()
    expect(consumePendingFirstItem()).toBeNull()
  })

  it("rejects a marker missing the draftKey", () => {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify({ currency: "USD", savedAt: 1 }))
    expect(peekPendingFirstItem()).toBeNull()
  })

  it("rejects a marker with an empty draftKey", () => {
    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ draftKey: "", currency: "USD", savedAt: 1 })
    )
    expect(peekPendingFirstItem()).toBeNull()
  })
})
