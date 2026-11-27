import { beforeEach, describe, expect, it, vi } from "vitest"

import { toast } from "sonner"

import { HttpError, NetworkError } from "@/lib/http"
import { toastMutationError } from "@/lib/mutation-error-toast"

const errorMock = toast.error as ReturnType<typeof vi.fn>

beforeEach(() => {
  errorMock.mockReset()
})

describe("toastMutationError", () => {
  it("shows the server's detail for a refused action", () => {
    toastMutationError(
      new HttpError("conflict", 409, "/x", { errors: [{ detail: "the area still holds items" }] }),
      "Couldn't delete the area"
    )
    expect(errorMock).toHaveBeenCalledTimes(1)
    expect(errorMock).toHaveBeenCalledWith("the area still holds items")
  })

  it("shows a plain-text body", () => {
    toastMutationError(new HttpError("forbidden", 403, "/x", "not allowed\n"), "Couldn't save")
    expect(errorMock).toHaveBeenCalledWith("not allowed")
  })

  it("shows the fallback when a 4xx carries no message", () => {
    toastMutationError(
      new HttpError("invalid", 422, "/x", { errors: [{ status: "Unprocessable Entity" }] }),
      "Couldn't save"
    )
    expect(errorMock).toHaveBeenCalledWith("Couldn't save")
  })

  it.each([
    ["a 5xx", new HttpError("server", 500, "/x", { errors: [{ detail: "db down" }] })],
    ["a 423", new HttpError("locked", 423, "/x", null)],
    ["a request that never reached the server", new NetworkError("/x", new TypeError("x"))],
  ])("leaves %s to the global toast", (_name, err) => {
    toastMutationError(err, "Couldn't save")
    expect(errorMock).not.toHaveBeenCalled()
  })

  it("shows a currency-migration 5xx, which the global toast skips", () => {
    toastMutationError(
      new HttpError("server", 500, "/x", {
        errors: [{ code: "currency_migration.failed", detail: "migration failed" }],
      }),
      "Couldn't save"
    )
    expect(errorMock).toHaveBeenCalledWith("migration failed")
  })
})
