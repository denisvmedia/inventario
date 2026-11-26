import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { act, renderHook, waitFor } from "@testing-library/react"
import { delay, http, HttpResponse } from "msw"
import { type ReactNode } from "react"
import { MemoryRouter, Route, Routes, useNavigate } from "react-router-dom"
import { beforeEach, describe, expect, it } from "vitest"

import { GroupProvider, useOptionalCurrentGroup } from "@/features/group/GroupContext"
import { useInboxCount } from "@/features/inbox/hooks"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { apiUrl, groupHandlers } from "@/test/handlers"
import { server } from "@/test/server"

function makeWrapper(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
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
}

function useHooks() {
  return {
    count: useInboxCount(),
    slug: useOptionalCurrentGroup()?.currentGroup?.slug,
    navigate: useNavigate(),
  }
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
})

describe("useInboxCount", () => {
  it("shows no count of the previous group while the new group's loads", async () => {
    server.use(
      ...groupHandlers.list([
        { id: "g1", slug: "home", name: "Home" },
        { id: "g2", slug: "work", name: "Work" },
      ]),
      http.get(apiUrl("/g/home/inbox/count"), () =>
        HttpResponse.json({ data: { total: 3, reasons: { draft: 3 } } })
      ),
      http.get(apiUrl("/g/work/inbox/count"), async () => {
        await delay("infinite")
        return HttpResponse.json({ data: { total: 0, reasons: {} } })
      })
    )
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const { result } = renderHook(useHooks, { wrapper: makeWrapper(client) })
    await waitFor(() => expect(result.current.count.data?.total).toBe(3))

    act(() => result.current.navigate("/g/work"))
    await waitFor(() => expect(result.current.slug).toBe("work"))
    expect(result.current.count.data).toBeUndefined()
    expect(result.current.count.isPending).toBe(true)
  })
})
