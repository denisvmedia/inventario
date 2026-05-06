import { beforeEach, describe, expect, it, vi } from "vitest"

vi.mock("sonner", () => {
  const error = vi.fn()
  return {
    toast: { error },
  }
})

import { toast } from "sonner"

import { createQueryClient } from "@/lib/query-client"
import { HttpError } from "@/lib/http"

const errorMock = toast.error as ReturnType<typeof vi.fn>

beforeEach(() => {
  errorMock.mockReset()
})

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

  it("fires a global toast when a query bubbles an HTTP 500 (issue #1210)", async () => {
    const client = createQueryClient()
    await client
      .fetchQuery({
        queryKey: ["1210", "query-500"],
        queryFn: () => Promise.reject(new HttpError("server", 500, "/x", null)),
        retry: false,
      })
      .catch(() => {
        // expected — we just need the cache's onError to fire.
      })
    expect(errorMock).toHaveBeenCalledTimes(1)
  })

  it("fires a global toast when a mutation bubbles an HTTP 500 (issue #1210)", async () => {
    const client = createQueryClient()
    await client
      .getMutationCache()
      .build(client, {
        mutationFn: () => Promise.reject(new HttpError("server", 500, "/x", null)),
      })
      .execute(undefined)
      .catch(() => {
        // expected — we just need the cache's onError to fire.
      })
    expect(errorMock).toHaveBeenCalledTimes(1)
  })

  it("does not fire a global toast on a 4xx", async () => {
    const client = createQueryClient()
    await client
      .fetchQuery({
        queryKey: ["1210", "query-422"],
        queryFn: () => Promise.reject(new HttpError("validation", 422, "/x", null)),
        retry: false,
      })
      .catch(() => {})
    expect(errorMock).not.toHaveBeenCalled()
  })

  it("respects meta.suppressGlobalErrorToast on a 5xx mutation", async () => {
    const client = createQueryClient()
    await client
      .getMutationCache()
      .build(client, {
        mutationFn: () => Promise.reject(new HttpError("server", 500, "/x", null)),
        meta: { suppressGlobalErrorToast: true },
      })
      .execute(undefined)
      .catch(() => {})
    expect(errorMock).not.toHaveBeenCalled()
  })
})
