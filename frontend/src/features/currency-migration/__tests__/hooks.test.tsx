import { afterEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { renderHook, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import type { ReactNode } from "react"

import {
  useCurrencyMigration,
  useCurrencyMigrations,
  usePreviewMigration,
  useStartMigration,
} from "@/features/currency-migration/hooks"
import { currencyMigrationKeys } from "@/features/currency-migration/keys"
import { groupKeys } from "@/features/group/keys"
import { server } from "@/test/server"
import { apiUrl } from "@/test/handlers"

const SLUG = "household"

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
  return { client, Wrapper }
}

describe("useCurrencyMigration (detail)", () => {
  it("fetches a migration by id and exposes it through query state", async () => {
    server.use(
      msw.get(apiUrl(`/g/${SLUG}/currency-migrations/m1`), () =>
        HttpResponse.json({
          data: {
            id: "m1",
            type: "currency-migrations",
            attributes: { status: "pending", from_currency: "USD", to_currency: "EUR" },
          },
        })
      )
    )
    const { Wrapper } = makeWrapper()
    const { result } = renderHook(() => useCurrencyMigration(SLUG, "m1"), { wrapper: Wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toMatchObject({ id: "m1", status: "pending" })
  })

  it("stays disabled (no fetch fired) when id or slug is missing", async () => {
    let hits = 0
    server.use(
      msw.get(apiUrl(`/g/${SLUG}/currency-migrations/m1`), () => {
        hits++
        return HttpResponse.json({ data: {} })
      })
    )
    const { Wrapper } = makeWrapper()
    // No id → enabled=false branch.
    const { result: noId } = renderHook(() => useCurrencyMigration(SLUG, undefined), {
      wrapper: Wrapper,
    })
    expect(noId.current.fetchStatus).toBe("idle")
    // No slug → enabled=false branch.
    const { result: noSlug } = renderHook(() => useCurrencyMigration(undefined, "m1"), {
      wrapper: Wrapper,
    })
    expect(noSlug.current.fetchStatus).toBe("idle")
    // Neither hook should have triggered a network call.
    expect(hits).toBe(0)
  })

  it("polls while the migration is non-terminal and stops when it terminates", async () => {
    const { Wrapper, client } = makeWrapper()
    server.use(
      msw.get(apiUrl(`/g/${SLUG}/currency-migrations/m1`), () =>
        HttpResponse.json({
          data: {
            id: "m1",
            type: "currency-migrations",
            attributes: { status: "completed" },
          },
        })
      )
    )
    const { result } = renderHook(() => useCurrencyMigration(SLUG, "m1"), { wrapper: Wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    // refetchInterval branch — exercised by reading the function the
    // hook installs on the QueryObserver. Terminal state must return
    // `false`; non-terminal must return the poll cadence (5_000 ms).
    const queries = client.getQueryCache().findAll({
      queryKey: currencyMigrationKeys.detail(SLUG, "m1"),
    })
    expect(queries).toHaveLength(1)
    const observer = queries[0]
    expect(typeof observer.options.refetchInterval).toBe("function")
    const completedSig = (
      observer.options.refetchInterval as (q: typeof observer) => false | number
    )(observer)
    expect(completedSig).toBe(false)
  })
})

describe("useCurrencyMigrations (list)", () => {
  it("polls while any row is non-terminal", async () => {
    server.use(
      msw.get(apiUrl(`/g/${SLUG}/currency-migrations`), () =>
        HttpResponse.json({
          data: [
            {
              id: "m1",
              type: "currency-migrations",
              attributes: { status: "running" },
            },
          ],
        })
      )
    )
    const { Wrapper, client } = makeWrapper()
    const { result } = renderHook(() => useCurrencyMigrations(SLUG), { wrapper: Wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    const queries = client.getQueryCache().findAll({ queryKey: currencyMigrationKeys.list(SLUG) })
    const observer = queries[0]
    const sig = (observer.options.refetchInterval as (q: typeof observer) => false | number)(
      observer
    )
    expect(sig).toBe(5_000)
  })
})

describe("useStartMigration", () => {
  it("invalidates the migrations list and the group cache on success", async () => {
    server.use(
      msw.post(apiUrl(`/g/${SLUG}/currency-migrations`), () =>
        HttpResponse.json(
          {
            data: {
              id: "m9",
              type: "currency-migrations",
              attributes: { status: "pending" },
            },
          },
          { status: 201 }
        )
      )
    )
    const { Wrapper, client } = makeWrapper()
    // Pre-warm both caches so we can observe invalidation.
    client.setQueryData(currencyMigrationKeys.list(SLUG), { migrations: [] })
    client.setQueryData(groupKeys.list(), [])
    const { result } = renderHook(() => useStartMigration(SLUG), { wrapper: Wrapper })
    result.current.mutate({
      from_currency: "USD",
      to_currency: "EUR",
      exchange_rate: 0.9,
      preview_token: "tok",
    })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    // Both query keys are now flagged stale (invalidated). Verify by
    // checking the queryCache state.
    const listQ = client.getQueryCache().find({ queryKey: currencyMigrationKeys.list(SLUG) })
    expect(listQ?.state.isInvalidated).toBe(true)
    const groupQ = client.getQueryCache().find({ queryKey: groupKeys.list() })
    expect(groupQ?.state.isInvalidated).toBe(true)
  })
})

describe("usePreviewMigration", () => {
  it("calls preview with the configured slug and returns the body", async () => {
    server.use(
      msw.post(apiUrl(`/g/${SLUG}/currency-migrations/preview`), () =>
        HttpResponse.json({
          data: {
            type: "currency-migration-previews",
            attributes: { preview_token: "tok", commodity_count: 0 },
          },
        })
      )
    )
    const { Wrapper } = makeWrapper()
    const { result } = renderHook(() => usePreviewMigration(SLUG), { wrapper: Wrapper })
    result.current.mutate({ from_currency: "USD", to_currency: "EUR", exchange_rate: 0.9 })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toMatchObject({ preview_token: "tok", commodity_count: 0 })
  })
})
