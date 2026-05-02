import { describe, expect, it } from "vitest"

import { sanitizeRedirectPath } from "@/lib/safe-redirect"

describe("sanitizeRedirectPath", () => {
  it("returns the value unchanged for a normal in-app path", () => {
    expect(sanitizeRedirectPath("/g/household/items")).toBe("/g/household/items")
  })

  it("falls back to / for null / undefined / empty / whitespace", () => {
    expect(sanitizeRedirectPath(null)).toBe("/")
    expect(sanitizeRedirectPath(undefined)).toBe("/")
    expect(sanitizeRedirectPath("")).toBe("/")
    expect(sanitizeRedirectPath("   ")).toBe("/")
  })

  it("rejects values that don't start with /", () => {
    expect(sanitizeRedirectPath("g/household")).toBe("/")
    expect(sanitizeRedirectPath("https://evil.example")).toBe("/")
    expect(sanitizeRedirectPath("javascript:alert(1)")).toBe("/")
  })

  it("rejects protocol-relative URLs like //evil.example", () => {
    expect(sanitizeRedirectPath("//evil.example")).toBe("/")
    expect(sanitizeRedirectPath("//evil.example/foo")).toBe("/")
  })

  it("rejects Windows-style \\\\host backslash paths", () => {
    expect(sanitizeRedirectPath("/\\evil.example")).toBe("/")
  })

  it("preserves the query string and fragment of in-app paths", () => {
    expect(sanitizeRedirectPath("/items?filter=active#top")).toBe("/items?filter=active#top")
  })
})
