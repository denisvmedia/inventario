import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { renderHook, waitFor } from "@testing-library/react"
import { type ReactNode } from "react"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { beforeEach, describe, expect, it } from "vitest"

import { GroupProvider } from "@/features/group/GroupContext"
import {
  useCreateTag,
  useDeleteTag,
  useTagAutocomplete,
  useTagStats,
  useTags,
  useUpdateTag,
} from "@/features/tags/hooks"
import { tagKeys } from "@/features/tags/keys"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { groupHandlers, tagHandlers } from "@/test/handlers"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", main_currency: "USD" },
]

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

describe("features/tags/hooks", () => {
  it("useTags returns the list keyed under the active group's slug", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...tagHandlers.list(SLUG, [
        {
          id: "t1",
          slug: "kitchen",
          label: "Kitchen",
          color: "amber",
          meta: { usage: { commodities: 2, files: 0 } },
        },
      ])
    )
    const client = newClient()
    const { result } = renderHook(() => useTags({ includeUsage: true }), {
      wrapper: makeWrapper(client),
    })
    await waitFor(() => expect(result.current.data?.tags.length).toBe(1))
    expect(result.current.data?.tags[0].usage?.commodities).toBe(2)
    // Cache is keyed under `tagKeys.list(slug, opts)`. Reading directly
    // proves the slug propagated through GroupProvider into the hook.
    const cached = client.getQueryData(tagKeys.list(SLUG, { includeUsage: true }))
    expect(cached).toBeDefined()
  })

  it("useTagStats reads /tags/stats and exposes the flat payload", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...tagHandlers.stats(SLUG, {
        tags_total: 5,
        items_tagged: 3,
        items_untagged: 1,
        files_tagged: 2,
        files_untagged: 4,
      })
    )
    const client = newClient()
    const { result } = renderHook(() => useTagStats(), { wrapper: makeWrapper(client) })
    await waitFor(() => expect(result.current.data?.tags_total).toBe(5))
    expect(result.current.data?.files_untagged).toBe(4)
  })

  it("useCreateTag returns the created tag and surfaces it in the list cache via invalidation", async () => {
    let listCalls = 0
    server.use(
      ...groupHandlers.list(groupFixture),
      ...tagHandlers.create(SLUG, {
        id: "t-new",
        slug: "kitchen",
        label: "Kitchen",
        color: "amber",
      })
    )
    // Custom list handler counts invocations so we can confirm the
    // invalidation triggered a refetch.
    const { http, HttpResponse } = await import("msw")
    const { apiUrl } = await import("@/test/handlers")
    server.use(
      http.get(apiUrl(`/g/${SLUG}/tags`), () => {
        listCalls += 1
        return HttpResponse.json({
          data: [],
          meta: { tags: 0, total: 0 },
        })
      })
    )
    const client = newClient()
    const { result } = renderHook(() => ({ list: useTags(), create: useCreateTag() }), {
      wrapper: makeWrapper(client),
    })
    await waitFor(() => expect(result.current.list.data).toBeDefined())
    const initialCalls = listCalls
    const created = await result.current.create.mutateAsync({
      slug: "kitchen",
      label: "Kitchen",
      color: "amber",
    })
    expect(created.slug).toBe("kitchen")
    // The list query is observed by the hook; invalidation refetches it.
    await waitFor(() => expect(listCalls).toBeGreaterThan(initialCalls))
  })

  it("useUpdateTag passes id + req to the patch endpoint", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...tagHandlers.update(SLUG, "t1", {
        id: "t1",
        slug: "kitchen-2",
        label: "Kitchen",
        color: "blue",
      })
    )
    const client = newClient()
    const { result } = renderHook(() => useUpdateTag(), { wrapper: makeWrapper(client) })
    const updated = await result.current.mutateAsync({
      id: "t1",
      req: { slug: "kitchen-2", color: "blue" },
    })
    expect(updated.slug).toBe("kitchen-2")
    expect(updated.color).toBe("blue")
  })

  it("useDeleteTag with force=true issues the force-delete request", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...tagHandlers.remove(SLUG, "t1", { conflict: true })
    )
    const client = newClient()
    const { result } = renderHook(() => useDeleteTag(), { wrapper: makeWrapper(client) })
    // Without force, conflict handler returns 409 → mutation rejects.
    await expect(result.current.mutateAsync({ id: "t1" })).rejects.toThrow()
    // With force, it returns 204.
    await expect(result.current.mutateAsync({ id: "t1", force: true })).resolves.toBeUndefined()
  })

  it("useTagAutocomplete returns the flat data envelope", async () => {
    server.use(...groupHandlers.list(groupFixture), ...tagHandlers.list(SLUG, []))
    const { server: srv } = await import("@/test/server")
    const { http, HttpResponse } = await import("msw")
    const { apiUrl } = await import("@/test/handlers")
    srv.use(
      http.get(apiUrl(`/g/${SLUG}/tags/autocomplete`), () =>
        HttpResponse.json({
          data: [{ id: "t1", slug: "kitchen", label: "Kitchen", color: "amber" }],
        })
      )
    )
    const client = newClient()
    const { result } = renderHook(() => useTagAutocomplete("ki"), {
      wrapper: makeWrapper(client),
    })
    await waitFor(() => expect(result.current.data?.length).toBe(1))
    expect(result.current.data?.[0].slug).toBe("kitchen")
  })
})
