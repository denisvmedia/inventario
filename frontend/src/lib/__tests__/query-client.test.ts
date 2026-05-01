import { describe, expect, it } from "vitest"

import { createQueryClient } from "@/lib/query-client"
import { HttpError } from "@/lib/http"

describe("createQueryClient", () => {
  it("returns a fresh client instance per call", () => {
    const a = createQueryClient()
    const b = createQueryClient()
    expect(a).not.toBe(b)
  })

  it("seeds the project's tunables on the queries default options", () => {
    const client = createQueryClient()
    const queries = client.getDefaultOptions().queries ?? {}
    // staleTime keeps cross-page navigation cheap (cached lists stay fresh
    // for a route hop).
    expect(queries.staleTime).toBe(30_000)
    // refetchOnWindowFocus is on — the legacy frontend's habit of
    // re-checking when the user comes back from another tab.
    expect(queries.refetchOnWindowFocus).toBe(true)
  })

  it("retry opts out of 4xx errors and caps at one retry for 5xx", () => {
    const client = createQueryClient()
    const retry = client.getDefaultOptions().queries?.retry
    expect(typeof retry).toBe("function")
    if (typeof retry !== "function") return
    const fourHundred = new HttpError("forbidden", 403, "/x", null)
    const fiveHundred = new HttpError("server", 500, "/x", null)
    expect(retry(0, fourHundred)).toBe(false)
    expect(retry(0, fiveHundred)).toBe(true)
    // failureCount=1 means "we already retried once".
    expect(retry(1, fiveHundred)).toBe(false)
  })

  it("mutations never retry", () => {
    const client = createQueryClient()
    expect(client.getDefaultOptions().mutations?.retry).toBe(false)
  })
})
