import { describe, expect, it, beforeEach } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { renderHook, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import type { ReactNode } from "react"

import { server } from "@/test/server"
import { useIsSystemAdmin } from "@/features/auth/hooks"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function makeWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: 0 },
      mutations: { retry: false },
    },
  })
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  return { wrapper, queryClient }
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("useIsSystemAdmin", () => {
  it("returns true when /auth/me carries is_system_admin", async () => {
    setAccessToken("good-token")
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({
          id: "u1",
          email: "admin@example.com",
          name: "Admin",
          is_system_admin: true,
        })
      )
    )
    const { wrapper } = makeWrapper()
    const { result } = renderHook(() => useIsSystemAdmin(), { wrapper })
    await waitFor(() => expect(result.current).toBe(true))
  })

  it("returns false for a non-admin user", async () => {
    setAccessToken("good-token")
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({
          id: "u2",
          email: "user@example.com",
          name: "User",
          is_system_admin: false,
        })
      )
    )
    const { wrapper, queryClient } = makeWrapper()
    const { result } = renderHook(() => useIsSystemAdmin(), { wrapper })
    // Give the query time to settle, then assert it never flips to true.
    await waitFor(() => expect(queryClient.isFetching()).toBe(0))
    expect(result.current).toBe(false)
  })

  it("returns false when the flag is absent from the payload", async () => {
    setAccessToken("good-token")
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u3", email: "legacy@example.com", name: "Legacy" })
      )
    )
    const { wrapper, queryClient } = makeWrapper()
    const { result } = renderHook(() => useIsSystemAdmin(), { wrapper })
    await waitFor(() => expect(queryClient.isFetching()).toBe(0))
    expect(result.current).toBe(false)
  })

  it("returns false before the boot probe settles (no token)", () => {
    const { wrapper } = makeWrapper()
    const { result } = renderHook(() => useIsSystemAdmin(), { wrapper })
    expect(result.current).toBe(false)
  })
})
