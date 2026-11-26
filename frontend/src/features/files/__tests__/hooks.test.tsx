import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { act, renderHook, waitFor } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { type ReactNode } from "react"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { beforeEach, describe, expect, it } from "vitest"

import { commodityKeys } from "@/features/commodities/keys"
import { useBulkReclassifyFiles, useDeleteFile, useUpdateFile } from "@/features/files/hooks"
import { GroupProvider, useOptionalCurrentGroup } from "@/features/group/GroupContext"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { apiUrl, groupHandlers } from "@/test/handlers"
import { server } from "@/test/server"

const SLUG = "home"

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

function useHooks() {
  return {
    slug: useOptionalCurrentGroup()?.currentGroup?.slug,
    remove: useDeleteFile(),
    reclassify: useBulkReclassifyFiles(),
    update: useUpdateFile("f1"),
  }
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
})

// A file write can change which items have a photo, so the No photo list,
// the inbox checks and the covers are read again.
describe("file writes refresh the item queries", () => {
  async function setup() {
    server.use(
      ...groupHandlers.list([{ id: "g1", slug: SLUG, name: "Home" }]),
      http.delete(apiUrl(`/g/${SLUG}/files/:id`), () => new HttpResponse(null, { status: 204 })),
      http.put(apiUrl(`/g/${SLUG}/files/:id`), () =>
        HttpResponse.json({ id: "f1", type: "files", attributes: { title: "Receipt" } })
      )
    )
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const items = commodityKeys.group(SLUG)
    client.setQueryData([...items, "probe"], { fresh: true })
    const { result } = renderHook(useHooks, { wrapper: makeWrapper(client) })
    await waitFor(() => expect(result.current.slug).toBe(SLUG))
    const stale = () => client.getQueryState([...items, "probe"])?.isInvalidated
    const refresh = () => client.setQueryData([...items, "probe"], { fresh: true })
    return { result, stale, refresh }
  }

  it("after a file goes to Trash, is reclassified or is updated", async () => {
    const { result, stale, refresh } = await setup()
    expect(stale()).toBe(false)

    await act(() => result.current.remove.mutateAsync("f1"))
    expect(stale()).toBe(true)

    refresh()
    await act(() => result.current.reclassify.mutateAsync({ ids: ["f1"], category: "documents" }))
    expect(stale()).toBe(true)

    refresh()
    await act(() => result.current.update.mutateAsync({ title: "Receipt" }))
    expect(stale()).toBe(true)
  })
})
