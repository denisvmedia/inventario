import { describe, expect, it } from "vitest"

import { HttpError } from "@/lib/http"
import {
  classifyServerError,
  extractFieldErrors,
  isRetryableKind,
  parseServerError,
} from "@/lib/server-error"

// Builds the BE's nested 422 validation envelope around an `attributes`
// object, mirroring what `errormarshal.Marshal(validation.Errors)` emits.
function validationEnvelope(attributes: Record<string, unknown>) {
  return {
    errors: [
      {
        status: "Unprocessable Entity",
        error: {
          type: "validation.Errors",
          error: { data: { attributes } },
        },
      },
    ],
  }
}

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

describe("classifyServerError", () => {
  it("classifies a non-HttpError as network and surfaces the Error message", () => {
    const result = classifyServerError(new TypeError("Failed to fetch"), "fallback")
    expect(result).toEqual({ kind: "network", message: "Failed to fetch" })
  })

  it("classifies a thrown non-Error value as network with the fallback copy", () => {
    expect(classifyServerError(undefined, "Connection lost")).toEqual({
      kind: "network",
      message: "Connection lost",
    })
  })

  it.each([400, 422] as const)("classifies %i as validation", (status) => {
    const err = new HttpError("boom", status, "/x", { errors: [{ detail: "Name required" }] })
    expect(classifyServerError(err, "fallback")).toEqual({
      kind: "validation",
      message: "Name required",
    })
  })

  it.each([409, 412, 423] as const)("classifies %i as conflict", (status) => {
    const err = new HttpError("boom", status, "/x", { error: "Already taken" })
    expect(classifyServerError(err, "fallback")).toEqual({
      kind: "conflict",
      message: "Already taken",
    })
  })

  it.each([403, 404, 500, 502, 503, 504] as const)("classifies %i as unknown", (status) => {
    const err = new HttpError("boom", status, "/x", null)
    expect(classifyServerError(err, "Try again")).toEqual({
      kind: "unknown",
      message: "Try again",
    })
  })
})

describe("extractFieldErrors", () => {
  it("pulls a single field message out of the nested 422 envelope", () => {
    const err = new HttpError("boom", 422, "/x", validationEnvelope({ address: "cannot be blank" }))
    expect(extractFieldErrors(err)).toEqual({ address: "cannot be blank" })
  })

  it("extracts every attribute when several fields fail", () => {
    const err = new HttpError(
      "boom",
      422,
      "/x",
      validationEnvelope({ name: "cannot be blank", short_name: "is too long" })
    )
    expect(extractFieldErrors(err)).toEqual({
      name: "cannot be blank",
      short_name: "is too long",
    })
  })

  it("flattens array-indexed fields into dotted paths", () => {
    const err = new HttpError(
      "boom",
      422,
      "/x",
      validationEnvelope({ urls: { "0": "must be a valid URL", "2": "cannot be blank" } })
    )
    expect(extractFieldErrors(err)).toEqual({
      "urls.0": "must be a valid URL",
      "urls.2": "cannot be blank",
    })
  })

  it("returns null for a non-HttpError value", () => {
    expect(extractFieldErrors(new Error("nope"))).toBeNull()
  })

  it("returns null when the body isn't a field-validation envelope", () => {
    // A plain JSON:API detail error carries no data.attributes map.
    const err = new HttpError("boom", 422, "/x", { errors: [{ detail: "Bad input" }] })
    expect(extractFieldErrors(err)).toBeNull()
  })

  it("returns null when attributes is present but empty", () => {
    const err = new HttpError("boom", 422, "/x", validationEnvelope({}))
    expect(extractFieldErrors(err)).toBeNull()
  })
})

describe("isRetryableKind", () => {
  it("offers Retry for network and unknown", () => {
    expect(isRetryableKind("network")).toBe(true)
    expect(isRetryableKind("unknown")).toBe(true)
  })

  it("hides Retry for validation and conflict", () => {
    // Both kinds need user action before re-submit could succeed —
    // showing Retry would just queue another failure.
    expect(isRetryableKind("validation")).toBe(false)
    expect(isRetryableKind("conflict")).toBe(false)
  })
})
