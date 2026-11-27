import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { toast } from "sonner"
import { MutationObserver, type QueryClient } from "@tanstack/react-query"

import { createQueryClient } from "@/lib/query-client"
import { HttpError } from "@/lib/http"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { fakeAccessToken } from "@/test/handlers/auth"

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

// A mutation still running when the signed-in user changes must not write the
// previous user's result into the next user's session (#2878).
describe("a mutation that outlives the signed-in user", () => {
  beforeEach(() => {
    clearAuth()
    setAccessToken(fakeAccessToken("user-a"))
  })

  afterEach(() => {
    clearAuth()
  })

  function deferred<T>() {
    let resolve!: (value: T) => void
    let reject!: (error: unknown) => void
    const promise = new Promise<T>((res, rej) => {
      resolve = res
      reject = rej
    })
    return { promise, resolve, reject }
  }

  // Starts a mutation the way useMutation does, with callbacks at both levels,
  // and reports which of them ran and whether the mutate() promise settled.
  function startMutation(client: QueryClient, meta?: Record<string, unknown>) {
    const response = deferred<string>()
    const ran: string[] = []
    const observer = new MutationObserver<string, unknown, void>(client, {
      mutationFn: () => response.promise,
      meta,
      onSuccess: () => void ran.push("hook onSuccess"),
      onError: () => void ran.push("hook onError"),
      onSettled: () => void ran.push("hook onSettled"),
    })
    const unsubscribe = observer.subscribe(() => {})
    observer
      .mutate(undefined, {
        onSuccess: () => void ran.push("call onSuccess"),
        onError: () => void ran.push("call onError"),
        onSettled: () => void ran.push("call onSettled"),
      })
      .then(
        () => void ran.push("promise resolved"),
        () => void ran.push("promise rejected")
      )
    return { response, ran, unsubscribe }
  }

  // Lets every callback that is going to run, run.
  const settle = () => new Promise((resolve) => setTimeout(resolve, 0))

  it("runs none of its callbacks when it succeeds after a switch to another user", async () => {
    const { response, ran, unsubscribe } = startMutation(createQueryClient())

    setAccessToken(fakeAccessToken("user-b"))
    response.resolve("saved for user A")
    await settle()

    expect(ran).toEqual([])
    unsubscribe()
  })

  it("runs none of its callbacks and shows no toast when it fails after a sign-out", async () => {
    const { response, ran, unsubscribe } = startMutation(createQueryClient())

    clearAuth()
    response.reject(new HttpError("server", 500, "/x", null))
    await settle()

    expect(ran).toEqual([])
    expect(errorMock).not.toHaveBeenCalled()
    unsubscribe()
  })

  it("runs its callbacks when only the same user's token was refreshed", async () => {
    const { response, ran, unsubscribe } = startMutation(createQueryClient())

    setAccessToken(fakeAccessToken("user-a", "refreshed"))
    response.resolve("saved for user A")
    await settle()

    expect(ran).toEqual([
      "hook onSuccess",
      "hook onSettled",
      "call onSuccess",
      "call onSettled",
      "promise resolved",
    ])
    unsubscribe()
  })

  it("runs the callbacks of a mutation that changes the signed-in user itself", async () => {
    const { response, ran, unsubscribe } = startMutation(createQueryClient(), {
      changesIdentity: true,
    })

    setAccessToken(fakeAccessToken("user-b"))
    response.resolve("signed in as user B")
    await settle()

    expect(ran).toContain("hook onSuccess")
    expect(ran).toContain("promise resolved")
    unsubscribe()
  })

  it("runs the callbacks of a mutation started after the switch", async () => {
    const client = createQueryClient()
    setAccessToken(fakeAccessToken("user-b"))
    const { response, ran, unsubscribe } = startMutation(client)

    response.resolve("saved for user B")
    await settle()

    expect(ran).toContain("hook onSuccess")
    expect(ran).toContain("promise resolved")
    unsubscribe()
  })
})
