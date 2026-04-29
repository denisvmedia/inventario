import { describe, expect, it, beforeEach } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { renderHook, waitFor, act } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import type { ReactNode } from "react"

import { server } from "@/test/server"
import { useCurrentUser, useLogout } from "@/features/session/hooks"
import { sessionKeys } from "@/features/session/keys"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function makeWrapper() {
  // Per-test client so cache + retry state never leak between tests.
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: 0 },
      mutations: { retry: false },
    },
  })
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  return { queryClient, wrapper }
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("useCurrentUser", () => {
  it("loads the authenticated user", async () => {
    setAccessToken("good-token")
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u1", email: "denis@example.com", name: "Denis" }),
      ),
    )
    const { wrapper } = makeWrapper()
    const { result } = renderHook(() => useCurrentUser(), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toMatchObject({ email: "denis@example.com" })
  })
})

describe("useLogout (optimistic)", () => {
  it("clears the cached user before the server responds and invalidates after", async () => {
    setAccessToken("good-token")
    let logoutCalls = 0
    let meCalls = 0
    server.use(
      msw.get(api("/auth/me"), () => {
        meCalls++
        return HttpResponse.json({ id: "u1", email: "denis@example.com", name: "Denis" })
      }),
      msw.post(api("/auth/logout"), async () => {
        logoutCalls++
        await new Promise((r) => setTimeout(r, 10))
        return new HttpResponse(null, { status: 204 })
      }),
    )

    const { queryClient, wrapper } = makeWrapper()
    // Seed the cache by running the query first.
    const { result: q } = renderHook(() => useCurrentUser(), { wrapper })
    await waitFor(() => expect(q.current.isSuccess).toBe(true))
    expect(meCalls).toBe(1)

    const { result: m } = renderHook(() => useLogout(), { wrapper })
    await act(async () => {
      m.current.mutate()
    })

    // Optimistic update applied immediately.
    expect(queryClient.getQueryData(sessionKeys.currentUser())).toBeNull()
    await waitFor(() => expect(m.current.isSuccess).toBe(true))
    expect(logoutCalls).toBe(1)
    // After settlement, the query is invalidated (refetch happens; meCalls > 1).
    await waitFor(() => expect(meCalls).toBeGreaterThan(1))
  })

  it("rolls back the cache on error", async () => {
    setAccessToken("good-token")
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u1", email: "denis@example.com", name: "Denis" }),
      ),
      msw.post(api("/auth/logout"), () =>
        HttpResponse.json({ error: "boom" }, { status: 500 }),
      ),
    )

    const { queryClient, wrapper } = makeWrapper()
    const { result: q } = renderHook(() => useCurrentUser(), { wrapper })
    await waitFor(() => expect(q.current.isSuccess).toBe(true))
    const before = queryClient.getQueryData(sessionKeys.currentUser())
    expect(before).toMatchObject({ email: "denis@example.com" })

    const { result: m } = renderHook(() => useLogout(), { wrapper })
    await act(async () => {
      m.current.mutate()
    })
    await waitFor(() => expect(m.current.isError).toBe(true))
    // Cache restored to the pre-mutation snapshot.
    expect(queryClient.getQueryData(sessionKeys.currentUser())).toMatchObject({
      email: "denis@example.com",
    })
  })
})
