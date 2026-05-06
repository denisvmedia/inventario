import { beforeEach, describe, expect, it, vi } from "vitest"

vi.mock("sonner", () => {
  const error = vi.fn()
  return {
    toast: { error },
  }
})

import { toast } from "sonner"

import { notifyGlobalServerError } from "@/lib/global-error-toast"
import { HttpError } from "@/lib/http"

const errorMock = toast.error as ReturnType<typeof vi.fn>

beforeEach(() => {
  errorMock.mockReset()
})

describe("notifyGlobalServerError", () => {
  it.each([500, 502, 503, 504])("fires a toast for HttpError status %i", (status) => {
    notifyGlobalServerError(new HttpError("server", status, "/x", null), undefined)
    expect(errorMock).toHaveBeenCalledTimes(1)
  })

  it.each([400, 401, 403, 404, 422])("stays quiet for HttpError status %i", (status) => {
    notifyGlobalServerError(new HttpError("client", status, "/x", null), undefined)
    expect(errorMock).not.toHaveBeenCalled()
  })

  it("ignores non-HttpError throwables (network errors, aborts)", () => {
    notifyGlobalServerError(new TypeError("Failed to fetch"), undefined)
    notifyGlobalServerError("boom", undefined)
    expect(errorMock).not.toHaveBeenCalled()
  })

  it("respects suppressGlobalErrorToast in meta", () => {
    notifyGlobalServerError(new HttpError("server", 500, "/x", null), {
      suppressGlobalErrorToast: true,
    })
    expect(errorMock).not.toHaveBeenCalled()
  })

  it("falls back to the generic message when the body has no useful detail", () => {
    notifyGlobalServerError(
      new HttpError("server", 500, "/x", { errors: [{ status: "Internal Server UserError" }] }),
      undefined
    )
    expect(errorMock).toHaveBeenCalledWith("Server error. Please try again later.")
  })

  it("surfaces the JSON:API detail when the BE provides one", () => {
    notifyGlobalServerError(
      new HttpError("server", 500, "/x", {
        errors: [{ detail: "database is on fire" }],
      }),
      undefined
    )
    expect(errorMock).toHaveBeenCalledWith("database is on fire")
  })
})
