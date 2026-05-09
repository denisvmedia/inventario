import { describe, expect, it } from "vitest"

import { withGroupQuery } from "@/lib/group-aware-url"

describe("withGroupQuery", () => {
  it("returns the original path when slug is falsy", () => {
    expect(withGroupQuery("/profile", null)).toBe("/profile")
    expect(withGroupQuery("/profile", undefined)).toBe("/profile")
    expect(withGroupQuery("/profile", "")).toBe("/profile")
  })

  it("appends ?g=<slug> on a clean path", () => {
    expect(withGroupQuery("/profile", "household")).toBe("/profile?g=household")
  })

  it("preserves existing query params when adding g=", () => {
    expect(withGroupQuery("/profile?foo=bar", "household")).toBe("/profile?foo=bar&g=household")
  })

  it("replaces an existing g= rather than duplicating it", () => {
    // Without dedup: link from /profile?g=office to itself with a new slug
    // would emit /profile?g=office&g=household — the parser would then see
    // either value depending on URLSearchParams.get() vs .getAll().
    expect(withGroupQuery("/profile?g=office", "household")).toBe("/profile?g=household")
  })

  it("keeps `#fragment` at the tail and puts `?g=` before it", () => {
    // Naive concat would produce /help#shortcuts?g=… which the router
    // never parses as a query — the spec puts hash strictly after query.
    expect(withGroupQuery("/help#shortcuts", "household")).toBe("/help?g=household#shortcuts")
  })

  it("keeps `#fragment` while merging existing query params", () => {
    expect(withGroupQuery("/help?foo=bar#shortcuts", "household")).toBe(
      "/help?foo=bar&g=household#shortcuts"
    )
  })

  it("encodes slugs that contain URL-special characters", () => {
    // Slugs are random base64-ish strings in production, but the helper
    // shouldn't break if it sees a hyphen, plus, or space.
    expect(withGroupQuery("/profile", "a b+c")).toBe("/profile?g=a+b%2Bc")
  })
})
