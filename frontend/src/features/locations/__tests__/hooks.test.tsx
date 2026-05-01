import { beforeEach, describe, expect, it } from "vitest"
import { Route, Routes } from "react-router-dom"
import { renderHook, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { MemoryRouter } from "react-router-dom"
import { type ReactNode } from "react"

import { useDeleteLocation, useLocations, useUpdateLocation } from "@/features/locations/hooks"
import { GroupProvider } from "@/features/group/GroupContext"
import { server } from "@/test/server"
import { groupHandlers, locationHandlers } from "@/test/handlers"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { locationKeys } from "@/features/locations/keys"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", main_currency: "USD" },
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

    await result.current.remove.mutateAsync("loc1").catch(() => {
      /* expected */
    })

    await waitFor(() => {
      const list = client.getQueryData<unknown[]>(locationKeys.list(SLUG))
      expect(list).toHaveLength(1)
    })
  })
})
