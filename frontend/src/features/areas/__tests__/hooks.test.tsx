import { beforeEach, describe, expect, it } from "vitest"
import { Route, Routes } from "react-router-dom"
import { renderHook, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { MemoryRouter } from "react-router-dom"
import { type ReactNode } from "react"

import { useAreas, useDeleteArea, useUpdateArea } from "@/features/areas/hooks"
import { GroupProvider } from "@/features/group/GroupContext"
import { server } from "@/test/server"
import { areaHandlers, groupHandlers } from "@/test/handlers"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { areaKeys } from "@/features/areas/keys"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", main_currency: "USD" },
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

    await result.current.remove.mutateAsync("a1").catch(() => {
      /* expected */
    })

    await waitFor(() => {
      const list = client.getQueryData<unknown[]>(areaKeys.list(SLUG))
      expect(list).toHaveLength(1)
    })
  })
})
