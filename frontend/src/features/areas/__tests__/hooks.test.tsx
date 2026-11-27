import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route, Routes } from "react-router-dom"
import { renderHook, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { MemoryRouter } from "react-router-dom"
import { type ReactNode } from "react"
import { http, HttpResponse } from "msw"

import { useAreas, useDeleteArea, useUpdateArea } from "@/features/areas/hooks"
import { GroupProvider } from "@/features/group/GroupContext"
import { server } from "@/test/server"
import { apiUrl, areaHandlers, groupHandlers } from "@/test/handlers"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { areaKeys } from "@/features/areas/keys"
import { commodityKeys } from "@/features/commodities/keys"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

function areaResource(id: string, attrs: { name: string; location_id: string }) {
  return { id, type: "areas", attributes: { ...attrs, id } }
}

function makeWrapper(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={[`/g/${SLUG}`]}>
          <Routes>
            <Route
              path="/g/:groupSlug"
              element={<GroupProvider>{children as React.ReactElement}</GroupProvider>}
            />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    )
  }
}

function newClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: 0 },
      mutations: { retry: false },
    },
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
})

// The list endpoint pages its result (50 by default, at most 100). The hook
// feeds pickers and name lookups, so it must return every area.
describe("useAreas paging", () => {
  function manyAreas(count: number) {
    return Array.from({ length: count }, (_, i) =>
      areaResource(`a${i + 1}`, { name: `Area ${i + 1}`, location_id: "loc1" })
    )
  }

  it("returns every area when the group has more than one page", async () => {
    const requests: string[] = []
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.paged(SLUG, manyAreas(230), requests)
    )
    const { result } = renderHook(() => useAreas(), { wrapper: makeWrapper(newClient()) })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(result.current.data).toHaveLength(230)
    expect(result.current.data?.at(-1)).toMatchObject({ id: "a230", name: "Area 230" })
    const asked = requests.map((url) => new URL(url).searchParams)
    expect(asked.map((q) => q.get("page"))).toEqual(["1", "2", "3"])
    expect(asked.every((q) => q.get("per_page") === "100")).toBe(true)
  })

  it("stops once it has as many areas as the server reports", async () => {
    const requests: string[] = []
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.paged(SLUG, manyAreas(100), requests)
    )
    const { result } = renderHook(() => useAreas(), { wrapper: makeWrapper(newClient()) })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(result.current.data).toHaveLength(100)
    expect(requests).toHaveLength(1)
  })
})

describe("useAreas + mutations", () => {
  it("rolls the list back when an optimistic update fails", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, [areaResource("a1", { name: "Original", location_id: "loc1" })]),
      ...areaHandlers.updateError(SLUG, "a1", 500)
    )
    const client = newClient()
    const { result } = renderHook(() => ({ list: useAreas(), update: useUpdateArea("a1") }), {
      wrapper: makeWrapper(client),
    })

    await waitFor(() => expect(result.current.list.data?.length).toBe(1))
    expect(result.current.list.data?.[0].name).toBe("Original")

    await result.current.update.mutateAsync({ name: "Renamed" }).catch(() => {
      /* expected */
    })

    await waitFor(() => {
      const list = client.getQueryData<{ name?: string }[]>(areaKeys.list(SLUG))
      expect(list?.[0].name).toBe("Original")
    })
  })

  it("rolls the list back when an optimistic delete fails", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, [areaResource("a1", { name: "Workshop", location_id: "loc1" })]),
      ...areaHandlers.removeError(SLUG, "a1", 500)
    )
    const client = newClient()
    const { result } = renderHook(() => ({ list: useAreas(), remove: useDeleteArea() }), {
      wrapper: makeWrapper(client),
    })

    await waitFor(() => expect(result.current.list.data?.length).toBe(1))

    await result.current.remove.mutateAsync({ id: "a1" }).catch(() => {
      /* expected */
    })

    await waitFor(() => {
      const list = client.getQueryData<unknown[]>(areaKeys.list(SLUG))
      expect(list).toHaveLength(1)
    })
  })

  it("threads the chosen strategy into the DELETE url", async () => {
    let capturedUrl = ""
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, [areaResource("a1", { name: "Workshop", location_id: "loc1" })]),
      http.delete(apiUrl(`/g/${SLUG}/areas/a1`), ({ request }) => {
        capturedUrl = request.url
        return new HttpResponse(null, { status: 204 })
      })
    )
    const client = newClient()
    const { result } = renderHook(() => ({ list: useAreas(), remove: useDeleteArea() }), {
      wrapper: makeWrapper(client),
    })

    await waitFor(() => expect(result.current.list.data?.length).toBe(1))

    await result.current.remove.mutateAsync({ id: "a1", strategy: "unlink" })

    expect(new URL(capturedUrl).searchParams.get("strategy")).toBe("unlink")
  })

  it("invalidates the commodity group + area list caches on a strategy delete", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, [areaResource("a1", { name: "Workshop", location_id: "loc1" })]),
      ...areaHandlers.remove(SLUG, "a1")
    )
    const client = newClient()
    const invalidateSpy = vi.spyOn(client, "invalidateQueries")
    const { result } = renderHook(() => ({ list: useAreas(), remove: useDeleteArea() }), {
      wrapper: makeWrapper(client),
    })

    await waitFor(() => expect(result.current.list.data?.length).toBe(1))
    invalidateSpy.mockClear()

    await result.current.remove.mutateAsync({ id: "a1", strategy: "unlink" })

    await waitFor(() => {
      const keys = invalidateSpy.mock.calls.map((c) => JSON.stringify(c[0]?.queryKey))
      expect(keys).toContain(JSON.stringify(commodityKeys.group(SLUG)))
      expect(keys).toContain(JSON.stringify(areaKeys.list(SLUG)))
    })
  })

  it("does NOT touch the commodity cache on a default (no-strategy) delete", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, [areaResource("a1", { name: "Workshop", location_id: "loc1" })]),
      ...areaHandlers.remove(SLUG, "a1")
    )
    const client = newClient()
    const invalidateSpy = vi.spyOn(client, "invalidateQueries")
    const { result } = renderHook(() => ({ list: useAreas(), remove: useDeleteArea() }), {
      wrapper: makeWrapper(client),
    })

    await waitFor(() => expect(result.current.list.data?.length).toBe(1))
    invalidateSpy.mockClear()

    await result.current.remove.mutateAsync({ id: "a1" })

    await waitFor(() => {
      const keys = invalidateSpy.mock.calls.map((c) => JSON.stringify(c[0]?.queryKey))
      expect(keys).toContain(JSON.stringify(areaKeys.list(SLUG)))
      expect(keys).not.toContain(JSON.stringify(commodityKeys.group(SLUG)))
    })
  })
})
