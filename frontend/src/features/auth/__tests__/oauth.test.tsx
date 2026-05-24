import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { renderHook, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import type { ReactNode } from "react"

import { server } from "@/test/server"
import {
  isLastMethodError,
  oauthLinkStartUrl,
  oauthStartUrl,
  useOAuthIdentities,
  useOAuthProviders,
  useUnlinkOAuthIdentity,
} from "@/features/auth/oauth"
import { authKeys } from "@/features/auth/keys"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests, HttpError } from "@/lib/http"

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
  return { queryClient, wrapper }
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("oauthStartUrl", () => {
  it("builds the /api/v1 start URL with no redirect", () => {
    expect(oauthStartUrl("google")).toBe("/api/v1/auth/oauth/google/start")
  })

  it("encodes the provider name and appends the redirect query", () => {
    expect(oauthStartUrl("github", "/g/household")).toBe(
      "/api/v1/auth/oauth/github/start?redirect=%2Fg%2Fhousehold"
    )
  })
})

describe("oauthLinkStartUrl", () => {
  it("targets the authenticated link/start endpoint", () => {
    expect(oauthLinkStartUrl("google", "/settings")).toBe(
      "/api/v1/auth/oauth/google/link/start?redirect=%2Fsettings"
    )
  })
})

describe("useOAuthProviders", () => {
  it("returns the BE's enabled providers normalised to {name, displayName}", async () => {
    server.use(
      msw.get(api("/auth/oauth/providers"), () =>
        HttpResponse.json({
          providers: [
            { name: "google", display_name: "Google" },
            { name: "github", display_name: "GitHub" },
          ],
        })
      )
    )
    const { wrapper } = makeWrapper()
    const { result } = renderHook(() => useOAuthProviders(), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toEqual([
      { name: "google", displayName: "Google" },
      { name: "github", displayName: "GitHub" },
    ])
  })

  it("filters out unknown provider names so a misconfigured BE can't smuggle a fake button in", async () => {
    server.use(
      msw.get(api("/auth/oauth/providers"), () =>
        HttpResponse.json({
          providers: [
            { name: "google", display_name: "Google" },
            { name: "facebook", display_name: "Facebook" },
          ],
        })
      )
    )
    const { wrapper } = makeWrapper()
    const { result } = renderHook(() => useOAuthProviders(), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toEqual([{ name: "google", displayName: "Google" }])
  })

  it("returns an empty array when the BE has no providers configured", async () => {
    server.use(msw.get(api("/auth/oauth/providers"), () => HttpResponse.json({ providers: [] })))
    const { wrapper } = makeWrapper()
    const { result } = renderHook(() => useOAuthProviders(), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toEqual([])
  })
})

describe("useOAuthIdentities", () => {
  it("returns the caller's linked identities", async () => {
    setAccessToken("good-token")
    server.use(
      msw.get(api("/auth/oauth/identities"), () =>
        HttpResponse.json({
          identities: [
            { provider: "google", email: "denis@example.com", linked_at: "2026-04-01T00:00:00Z" },
          ],
        })
      )
    )
    const { wrapper } = makeWrapper()
    const { result } = renderHook(() => useOAuthIdentities(), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toEqual([
      { provider: "google", email: "denis@example.com", linked_at: "2026-04-01T00:00:00Z" },
    ])
  })

  it("drops malformed rows (missing fields) rather than crashing", async () => {
    setAccessToken("good-token")
    server.use(
      msw.get(api("/auth/oauth/identities"), () =>
        HttpResponse.json({
          identities: [
            { provider: "google", email: "denis@example.com", linked_at: "2026-04-01T00:00:00Z" },
            { provider: "github" }, // missing email + linked_at
          ],
        })
      )
    )
    const { wrapper } = makeWrapper()
    const { result } = renderHook(() => useOAuthIdentities(), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toHaveLength(1)
    expect(result.current.data?.[0].provider).toBe("google")
  })
})

describe("useUnlinkOAuthIdentity", () => {
  it("calls DELETE /auth/oauth/{provider} and invalidates the identities cache", async () => {
    setAccessToken("good-token")
    let deleteCalls = 0
    server.use(
      msw.get(api("/auth/oauth/identities"), () =>
        HttpResponse.json({
          identities: [
            { provider: "google", email: "denis@example.com", linked_at: "2026-04-01T00:00:00Z" },
          ],
        })
      ),
      msw.delete(api("/auth/oauth/google"), () => {
        deleteCalls++
        return new HttpResponse(null, { status: 204 })
      })
    )
    const { queryClient, wrapper } = makeWrapper()
    // Seed the cache.
    const { result: q } = renderHook(() => useOAuthIdentities(), { wrapper })
    await waitFor(() => expect(q.current.isSuccess).toBe(true))
    expect(queryClient.getQueryData(authKeys.oauthIdentities())).toHaveLength(1)

    const { result: m } = renderHook(() => useUnlinkOAuthIdentity(), { wrapper })
    await m.current.mutateAsync("google")
    expect(deleteCalls).toBe(1)
    // The mutation's onSuccess invalidated the identities query — the
    // refetch is the observable consequence the caller relies on. We
    // assert via waitFor on the cached value rather than the transient
    // `isInvalidated` flag (which races the immediate refetch).
    await waitFor(() => {
      expect(queryClient.getQueryState(authKeys.oauthIdentities())?.dataUpdateCount).toBeGreaterThan(
        1
      )
    })
  })

  it("surfaces the BE's 409 'last sign-in method' as HttpError so callers can map it to a dedicated message", async () => {
    setAccessToken("good-token")
    server.use(
      msw.delete(api("/auth/oauth/google"), () =>
        HttpResponse.text("Cannot remove the last sign-in method", { status: 409 })
      )
    )
    const { wrapper } = makeWrapper()
    const { result: m } = renderHook(() => useUnlinkOAuthIdentity(), { wrapper })
    await expect(m.current.mutateAsync("google")).rejects.toBeInstanceOf(HttpError)
  })
})

describe("isLastMethodError", () => {
  it("recognises an HttpError with status 409", () => {
    const err = new HttpError("x", 409, "/api/v1/auth/oauth/google", null)
    expect(isLastMethodError(err)).toBe(true)
  })

  it("rejects non-409 statuses", () => {
    expect(isLastMethodError(new HttpError("x", 500, "/", null))).toBe(false)
  })

  it("rejects non-HttpError values", () => {
    expect(isLastMethodError(new Error("boom"))).toBe(false)
    expect(isLastMethodError(undefined)).toBe(false)
  })
})
