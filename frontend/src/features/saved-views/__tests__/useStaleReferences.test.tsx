import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { act, renderHook, waitFor } from "@testing-library/react"
import { type ReactNode } from "react"
import { MemoryRouter, Route, Routes, useNavigate } from "react-router-dom"
import { http as msw, HttpResponse } from "msw"
import { beforeEach, describe, expect, it } from "vitest"

import { useAreas } from "@/features/areas/hooks"
import { DEFAULT_COMMODITY_QUERY } from "@/features/commodities/query"
import { GroupProvider } from "@/features/group/GroupContext"
import { useStaleReferences } from "@/features/saved-views/SavedViewControl"
import { useTagIndex } from "@/features/tags/hooks"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { apiUrl, areaHandlers, groupHandlers, sellerHandlers } from "@/test/handlers"
import { server } from "@/test/server"

const query = {
  ...DEFAULT_COMMODITY_QUERY,
  tags: { ...DEFAULT_COMMODITY_QUERY.tags, include: ["boat"] },
}

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return (
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={["/g/home"]}>
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

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
})

describe("useStaleReferences", () => {
  // After a group switch the tag catalogue shows the previous group's tags
  // until the new group's load; read as the new group's, they would mark
  // its tags as deleted.
  it("names no deleted tag while a group switch still shows the previous group's tags", async () => {
    let releaseLake: () => void = () => {}
    const lakeTags = new Promise<void>((resolve) => {
      releaseLake = resolve
    })
    // The first catalogue read is Home's; the one after the switch is held
    // until releaseLake. The request after the switch can still carry the
    // previous slug (the provider updates the request group in an effect),
    // so both URLs answer from the same sequence.
    let reads = 0
    const tags = async () => {
      reads++
      const slug = reads === 1 ? "garage" : "boat"
      if (reads > 1) await lakeTags
      return HttpResponse.json({
        data: [{ id: slug, slug, label: slug, color: "blue" }],
        meta: { tags: 1, total: 1 },
      })
    }
    server.use(
      ...groupHandlers.list([
        { id: "g1", slug: "home", name: "Home" },
        { id: "g2", slug: "lake", name: "Lake house" },
      ]),
      ...areaHandlers.list("home", []),
      ...areaHandlers.list("lake", []),
      ...sellerHandlers.list("home", []),
      ...sellerHandlers.list("lake", []),
      msw.get(apiUrl("/g/home/tags"), tags),
      msw.get(apiUrl("/g/lake/tags"), tags)
    )
    const { result } = renderHook(
      () => ({
        stale: useStaleReferences(query),
        tags: useTagIndex("commodity"),
        areas: useAreas(),
        navigate: useNavigate(),
      }),
      { wrapper }
    )
    await waitFor(() => expect(result.current.stale?.tags).toEqual(["boat"]))

    act(() => result.current.navigate("/g/lake"))
    await waitFor(() => expect(result.current.tags.isPlaceholderData).toBe(true))
    // The areas of the new group are in; only the tags are still Home's.
    await waitFor(() => expect(result.current.areas.isSuccess).toBe(true))
    expect(result.current.tags.tags.map((t) => t.slug)).toEqual(["garage"])
    expect(result.current.stale).toBeUndefined()

    releaseLake()
    await waitFor(() => expect(result.current.tags.isPlaceholderData).toBe(false))
    await waitFor(() => expect(result.current.tags.tags.map((t) => t.slug)).toEqual(["boat"]))
    await waitFor(() => expect(result.current.stale?.tags).toEqual([]))
  })
})
