import { describe, expect, it } from "vitest"

import { searchKeys } from "../keys"

describe("searchKeys", () => {
  it("scopes the query cache slot by group slug", () => {
    // Same (type, query, limit) must hash to a different key for different
    // groups — otherwise group-A's results would leak into group-B's
    // SearchPage on navigation. lib/http rewrites `/search` to a
    // group-prefixed URL, so the cache must follow that split.
    const alpha = searchKeys.query("alpha", "commodities", "drill", 5)
    const beta = searchKeys.query("beta", "commodities", "drill", 5)
    expect(alpha).not.toEqual(beta)
    expect(alpha[1]).toBe("alpha")
    expect(beta[1]).toBe("beta")
  })

  it("keys differ per type, query, and limit within the same group", () => {
    const a = searchKeys.query("alpha", "commodities", "drill", 5)
    const b = searchKeys.query("alpha", "files", "drill", 5)
    const c = searchKeys.query("alpha", "commodities", "saw", 5)
    const d = searchKeys.query("alpha", "commodities", "drill", 3)
    expect(a).not.toEqual(b)
    expect(a).not.toEqual(c)
    expect(a).not.toEqual(d)
  })
})
