import { describe, expect, it, vi } from "vitest"

import { applyServerFieldErrors, shouldShowGenericError } from "@/lib/form-errors"
import { HttpError } from "@/lib/http"

function validationEnvelope(attributes: Record<string, unknown>, codes?: Record<string, unknown>) {
  return {
    errors: [
      {
        status: "Unprocessable Entity",
        error: {
          type: "validation.Errors",
          error: { data: { attributes } },
          ...(codes ? { errorCodes: { data: { attributes: codes } } } : {}),
        },
      },
    ],
  }
}

describe("applyServerFieldErrors", () => {
  it("sets known fields on the form and reports them as mapped", () => {
    const err = new HttpError("boom", 422, "/x", validationEnvelope({ address: "cannot be blank" }))
    const setError = vi.fn()

    const result = applyServerFieldErrors(err, setError, { fields: ["name", "address"] })

    expect(setError).toHaveBeenCalledWith("address", {
      type: "server",
      message: "cannot be blank",
    })
    expect(result).toEqual({ mapped: ["address"], unmapped: {} })
  })

  it("routes unknown server fields to unmapped without touching the form", () => {
    const err = new HttpError(
      "boom",
      422,
      "/x",
      validationEnvelope({ address: "cannot be blank", tenant_id: "must be set" })
    )
    const setError = vi.fn()

    const result = applyServerFieldErrors(err, setError, { fields: ["name", "address"] })

    expect(setError).toHaveBeenCalledTimes(1)
    expect(result).toEqual({
      mapped: ["address"],
      unmapped: { tenant_id: "must be set" },
    })
  })

  it("remaps server snake_case roots to camelCase form fields via map", () => {
    const err = new HttpError(
      "boom",
      422,
      "/x",
      validationEnvelope({ default_group_id: "does not exist" })
    )
    const setError = vi.fn()

    const result = applyServerFieldErrors(err, setError, {
      fields: ["name", "defaultGroupId"],
      map: { default_group_id: "defaultGroupId" },
    })

    expect(setError).toHaveBeenCalledWith("defaultGroupId", {
      type: "server",
      message: "does not exist",
    })
    expect(result).toEqual({ mapped: ["defaultGroupId"], unmapped: {} })
  })

  it("preserves compound array suffixes when mapping", () => {
    const err = new HttpError(
      "boom",
      422,
      "/x",
      validationEnvelope({ urls: { "0": "must be a valid URL" } })
    )
    const setError = vi.fn()

    const result = applyServerFieldErrors(err, setError, { fields: ["urls"] })

    expect(setError).toHaveBeenCalledWith("urls.0", {
      type: "server",
      message: "must be a valid URL",
    })
    expect(result).toEqual({ mapped: ["urls.0"], unmapped: {} })
  })

  it("returns null when the error is not a field-validation envelope", () => {
    const err = new HttpError("boom", 500, "/x", null)
    const setError = vi.fn()

    expect(applyServerFieldErrors(err, setError, { fields: ["name"] })).toBeNull()
    expect(setError).not.toHaveBeenCalled()
  })
})

describe("shouldShowGenericError", () => {
  it("shows the banner when the error wasn't a field envelope", () => {
    expect(shouldShowGenericError(null)).toBe(true)
  })

  it("shows the banner when nothing mapped to a form field", () => {
    expect(shouldShowGenericError({ mapped: [], unmapped: { foo: "bar" } })).toBe(true)
  })

  it("shows the banner when some field errors were left unmapped", () => {
    expect(shouldShowGenericError({ mapped: ["address"], unmapped: { foo: "bar" } })).toBe(true)
  })

  it("hides the banner when every error mapped cleanly", () => {
    expect(shouldShowGenericError({ mapped: ["address"], unmapped: {} })).toBe(false)
  })
})

describe("applyServerFieldErrors localization (#1990)", () => {
  it("localizes the field message by its validation code, interpolating params", () => {
    const err = new HttpError(
      "boom",
      422,
      "/x",
      validationEnvelope(
        { short: "RAW BE MESSAGE" },
        { short: { code: "validation_length_out_of_range", params: { min: 2, max: 50 } } }
      )
    )
    const setError = vi.fn()
    applyServerFieldErrors(err, setError, { fields: ["short"] })
    // The en validation key wins over the raw BE message, params interpolated.
    expect(setError).toHaveBeenCalledWith("short", {
      type: "server",
      message: "the length must be between 2 and 50",
    })
  })

  it("falls back to the BE message when the code is empty (codeless By-validator)", () => {
    const err = new HttpError(
      "boom",
      422,
      "/x",
      validationEnvelope(
        { weird: "some custom rule failed" },
        { weird: { code: "", message: "some custom rule failed" } }
      )
    )
    const setError = vi.fn()
    applyServerFieldErrors(err, setError, { fields: ["weird"] })
    expect(setError).toHaveBeenCalledWith("weird", {
      type: "server",
      message: "some custom rule failed",
    })
  })
})
