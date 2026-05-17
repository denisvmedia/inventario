import { afterEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { renderHook, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import type { ReactNode } from "react"

import { useFeatureFlag, useFeatureFlags } from "@/features/feature-flags/hooks"
import { server } from "@/test/server"
import { apiUrl } from "@/test/handlers"

afterEach(() => {
  server.resetHandlers()
})

function makeWrapper() {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: 0 },
      mutations: { retry: false },
    },
  })
  function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>
  }
  return { Wrapper }
}

describe("useFeatureFlags", () => {
  it("fetches the deployment feature flags and exposes them through query state", async () => {
    server.use(
      msw.get(apiUrl("/feature-flags"), () =>
        HttpResponse.json({ currency_migration: true })
      )
    )
    const { Wrapper } = makeWrapper()
    const { result } = renderHook(() => useFeatureFlags(), { wrapper: Wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toEqual({ currency_migration: true })
  })
})

describe("useFeatureFlag", () => {
  it("returns the resolved flag value once the query lands", async () => {
    server.use(
      msw.get(apiUrl("/feature-flags"), () =>
        HttpResponse.json({ currency_migration: true })
      )
    )
    const { Wrapper } = makeWrapper()
    const { result } = renderHook(() => useFeatureFlag("currency_migration"), { wrapper: Wrapper })
    await waitFor(() => expect(result.current).toBe(true))
  })

  it("falls back to the off state while the request is in flight", () => {
    // No handler registered — request stays pending. The fail-closed
    // default (#1616 design) is exactly what gated entry points rely on:
    // hide the CTA until proven that the feature is on.
    server.use(msw.get(apiUrl("/feature-flags"), async () => new Promise(() => {})))
    const { Wrapper } = makeWrapper()
    const { result } = renderHook(() => useFeatureFlag("currency_migration"), { wrapper: Wrapper })
    expect(result.current).toBe(false)
  })

  it("falls back to the off state on network failure", async () => {
    server.use(msw.get(apiUrl("/feature-flags"), () => HttpResponse.error()))
    const { Wrapper } = makeWrapper()
    const { result } = renderHook(() => useFeatureFlag("currency_migration"), { wrapper: Wrapper })
    // Wait until the query has reached an error state, then assert the
    // selector still returns false (fail-closed).
    await waitFor(() => expect(result.current).toBe(false))
  })
})
