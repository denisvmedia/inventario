import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route, Routes } from "react-router-dom"
import { renderHook, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { MemoryRouter } from "react-router-dom"
import { type ReactNode } from "react"
import { http, HttpResponse } from "msw"

import { useDeleteLocation, useLocations, useUpdateLocation } from "@/features/locations/hooks"
import { GroupProvider } from "@/features/group/GroupContext"
import { server } from "@/test/server"
import { apiUrl, groupHandlers, locationHandlers } from "@/test/handlers"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { locationKeys } from "@/features/locations/keys"
import { areaKeys } from "@/features/areas/keys"
import { commodityKeys } from "@/features/commodities/keys"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

function locationResource(id: string, attrs: { name: string; address?: string }) {
  return { id, type: "locations", attributes: { ...attrs, id } }
}

// Wrap hooks in a fresh QueryClient + MemoryRouter pointing at /g/:slug
// so GroupProvider's useParams resolves the slug. retry:false keeps
// rejected mutations deterministic.
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

describe("useLocations + mutations", () => {
  it("rolls the list back when an optimistic update fails", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Original" })]),
      ...locationHandlers.updateError(SLUG, "loc1", 500)
    )
    const client = newClient()
    const { result } = renderHook(
      () => ({
        list: useLocations(),
        update: useUpdateLocation("loc1"),
      }),
      { wrapper: makeWrapper(client) }
    )

    await waitFor(() => expect(result.current.list.data?.length).toBe(1))
    expect(result.current.list.data?.[0].name).toBe("Original")

    // Optimistic update flips the name immediately, then the server
    // returns 500 and the snapshot restores the pre-mutation list.
    await result.current.update.mutateAsync({ name: "Renamed" }).catch(() => {
      /* expected */
    })

    await waitFor(() => {
      const list = client.getQueryData<{ name?: string }[]>(locationKeys.list(SLUG))
      expect(list?.[0].name).toBe("Original")
    })
  })

  it("rolls the list back when an optimistic delete fails", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Garage" })]),
      ...locationHandlers.removeError(SLUG, "loc1", 500)
    )
    const client = newClient()
    const { result } = renderHook(() => ({ list: useLocations(), remove: useDeleteLocation() }), {
      wrapper: makeWrapper(client),
    })

    await waitFor(() => expect(result.current.list.data?.length).toBe(1))

    await result.current.remove.mutateAsync({ id: "loc1" }).catch(() => {
      /* expected */
    })

    await waitFor(() => {
      const list = client.getQueryData<unknown[]>(locationKeys.list(SLUG))
      expect(list).toHaveLength(1)
    })
  })

  it("threads the chosen strategy into the DELETE url", async () => {
    let capturedUrl = ""
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Garage" })]),
      http.delete(apiUrl(`/g/${SLUG}/locations/loc1`), ({ request }) => {
        capturedUrl = request.url
        return new HttpResponse(null, { status: 204 })
      })
    )
    const client = newClient()
    const { result } = renderHook(() => ({ list: useLocations(), remove: useDeleteLocation() }), {
      wrapper: makeWrapper(client),
    })

    await waitFor(() => expect(result.current.list.data?.length).toBe(1))

    await result.current.remove.mutateAsync({ id: "loc1", strategy: "cascade" })

    expect(new URL(capturedUrl).searchParams.get("strategy")).toBe("cascade")
  })

  it("invalidates the commodity group + area list caches on a strategy delete", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Garage" })]),
      ...locationHandlers.remove(SLUG, "loc1")
    )
    const client = newClient()
    const invalidateSpy = vi.spyOn(client, "invalidateQueries")
    const { result } = renderHook(() => ({ list: useLocations(), remove: useDeleteLocation() }), {
      wrapper: makeWrapper(client),
    })

    await waitFor(() => expect(result.current.list.data?.length).toBe(1))
    invalidateSpy.mockClear()

    await result.current.remove.mutateAsync({ id: "loc1", strategy: "unlink" })

    await waitFor(() => {
      const keys = invalidateSpy.mock.calls.map((c) => JSON.stringify(c[0]?.queryKey))
      expect(keys).toContain(JSON.stringify(commodityKeys.group(SLUG)))
      expect(keys).toContain(JSON.stringify(areaKeys.list(SLUG)))
    })
  })

  it("does NOT touch the commodity/area caches on a default (no-strategy) delete", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Garage" })]),
      ...locationHandlers.remove(SLUG, "loc1")
    )
    const client = newClient()
    const invalidateSpy = vi.spyOn(client, "invalidateQueries")
    const { result } = renderHook(() => ({ list: useLocations(), remove: useDeleteLocation() }), {
      wrapper: makeWrapper(client),
    })

    await waitFor(() => expect(result.current.list.data?.length).toBe(1))
    invalidateSpy.mockClear()

    await result.current.remove.mutateAsync({ id: "loc1" })

    await waitFor(() => {
      const keys = invalidateSpy.mock.calls.map((c) => JSON.stringify(c[0]?.queryKey))
      expect(keys).toContain(JSON.stringify(locationKeys.list(SLUG)))
      expect(keys).not.toContain(JSON.stringify(commodityKeys.group(SLUG)))
      expect(keys).not.toContain(JSON.stringify(areaKeys.list(SLUG)))
    })
  })
})
