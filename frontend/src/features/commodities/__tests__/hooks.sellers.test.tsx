import { beforeEach, describe, expect, it, vi } from "vitest"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { renderHook, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { type ReactNode } from "react"

import {
  useBulkDeleteCommodities,
  useCreateCommodity,
  useDeleteCommodity,
  useUpdateCommodity,
} from "@/features/commodities/hooks"
import { GroupProvider, useCurrentGroup } from "@/features/group/GroupContext"
import { sellerKeys } from "@/features/sellers/keys"
import { server } from "@/test/server"
import { commodityHandlers, groupHandlers } from "@/test/handlers"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

// Every item write that can change how many items a seller has refreshes the
// group's seller queries, so the Sellers page counts do not lag (#2804).

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]
const item = {
  id: "c1",
  type: "commodities",
  attributes: { id: "c1", name: "Kettle", type: "other", status: "in_use", count: 1 },
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
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
}

function setup() {
  server.use(
    ...groupHandlers.list(groupFixture),
    ...commodityHandlers.create(SLUG, item),
    ...commodityHandlers.update(SLUG, "c1", item),
    ...commodityHandlers.remove(SLUG, "c1"),
    ...commodityHandlers.bulkDelete(SLUG)
  )
  const client = newClient()
  const invalidateSpy = vi.spyOn(client, "invalidateQueries")
  const { result } = renderHook(
    () => ({
      group: useCurrentGroup(),
      create: useCreateCommodity(),
      update: useUpdateCommodity("c1"),
      remove: useDeleteCommodity(),
      bulkDelete: useBulkDeleteCommodities(),
    }),
    { wrapper: makeWrapper(client) }
  )
  return { result, invalidateSpy }
}

function invalidatedKeys(spy: ReturnType<typeof setup>["invalidateSpy"]) {
  return spy.mock.calls.map((c) => JSON.stringify(c[0]?.queryKey))
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
})

describe("commodity writes refresh the seller counts", () => {
  const sellersKey = JSON.stringify(sellerKeys.group(SLUG))

  it.each([
    [
      "create",
      (r: ReturnType<typeof setup>["result"]) =>
        r.current.create.mutateAsync({
          name: "Kettle",
          type: "other",
          status: "in_use",
          count: 1,
          seller_id: "s1",
        }),
    ],
    [
      "update",
      (r: ReturnType<typeof setup>["result"]) => r.current.update.mutateAsync({ seller_id: "s2" }),
    ],
    ["delete", (r: ReturnType<typeof setup>["result"]) => r.current.remove.mutateAsync("c1")],
    [
      "bulk delete",
      (r: ReturnType<typeof setup>["result"]) => r.current.bulkDelete.mutateAsync(["c1"]),
    ],
  ])("after %s", async (_name, run) => {
    const { result, invalidateSpy } = setup()
    await waitFor(() => expect(result.current.group.currentGroup?.slug).toBe(SLUG))
    invalidateSpy.mockClear()

    await run(result)

    await waitFor(() => expect(invalidatedKeys(invalidateSpy)).toContain(sellersKey))
  })
})
