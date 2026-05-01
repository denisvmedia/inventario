import { describe, expect, it } from "vitest"

import { HttpError } from "@/lib/http"
import { parseServerError } from "@/lib/server-error"

describe("parseServerError", () => {
  it("returns the trimmed string body when the server replied with plain text", () => {
    const err = new HttpError("boom", 400, "/x", "Email already taken  ")
    expect(parseServerError(err, "fallback")).toBe("Email already taken")
  })

  it("returns the first JSON:API error detail", () => {
    const err = new HttpError("boom", 400, "/x", {
      errors: [{ detail: "Email already taken" }],
    })
    expect(parseServerError(err, "fallback")).toBe("Email already taken")
  })

  it("falls back to envelope.error when no JSON:API errors are present", () => {
    const err = new HttpError("boom", 400, "/x", { error: "BadRequest" })
    expect(parseServerError(err, "fallback")).toBe("BadRequest")
  })

  it("returns the fallback when nothing useful is in the body", () => {
    const err = new HttpError("boom", 500, "/x", null)
    expect(parseServerError(err, "Try again later")).toBe("Try again later")
  })

  it("returns the fallback for non-HttpError values without a message", () => {
    expect(parseServerError({}, "fallback")).toBe("fallback")
  })

  it("returns plain Error message when present", () => {
    expect(parseServerError(new Error("network"), "fallback")).toBe("network")
  })
})
