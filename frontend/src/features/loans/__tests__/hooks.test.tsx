import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { act, renderHook, waitFor } from "@testing-library/react"
import { type ReactNode } from "react"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { beforeEach, describe, expect, it } from "vitest"

import { GroupProvider, useCurrentGroup } from "@/features/group/GroupContext"
import { useDeleteLoan, useReturnLoan, useStartLoan } from "@/features/loans/hooks"
import { savedViewKeys } from "@/features/saved-views/keys"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { groupHandlers, loanHandlers } from "@/test/handlers"
import { server } from "@/test/server"

const SLUG = "household"
const LOAN = {
  id: "loan-1",
  commodity_id: "c1",
  borrower_name: "Neighbor",
  lent_at: "2026-01-01",
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

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  server.use(
    ...groupHandlers.list([{ id: "g1", slug: SLUG, name: "Household" }]),
    ...loanHandlers.startLoan(SLUG, "c1", LOAN),
    ...loanHandlers.returnLoan(SLUG, "c1", "loan-1", { ...LOAN, returned_at: "2026-02-01" }),
    ...loanHandlers.deleteLoan(SLUG, "c1", "loan-1")
  )
})

// The item counts of saved views that filter on lent_out move when a loan
// starts, ends or is deleted, so each of those mutations marks them stale.
describe("features/loans/hooks", () => {
  it.each([
    {
      name: "starting a loan",
      run: (r: ReturnType<typeof useHooks>) =>
        r.start.mutateAsync({
          commodity_id: "c1",
          borrower_name: "Neighbor",
          lent_at: "2026-01-01",
        }),
    },
    {
      name: "returning a loan",
      run: (r: ReturnType<typeof useHooks>) =>
        r.ret.mutateAsync({ commodityID: "c1", loanID: "loan-1" }),
    },
    {
      name: "deleting a loan",
      run: (r: ReturnType<typeof useHooks>) =>
        r.del.mutateAsync({ commodityID: "c1", loanID: "loan-1" }),
    },
  ])("$name invalidates the saved view counts", async ({ run }) => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    client.setQueryData(savedViewKeys.counts(SLUG), { sv1: 0 })
    const { result } = renderHook(useHooks, { wrapper: makeWrapper(client) })
    await waitFor(() => expect(result.current.group?.slug).toBe(SLUG))
    expect(client.getQueryState(savedViewKeys.counts(SLUG))?.isInvalidated).toBe(false)

    await act(async () => {
      await run(result.current)
    })

    expect(client.getQueryState(savedViewKeys.counts(SLUG))?.isInvalidated).toBe(true)
  })
})

function useHooks() {
  const { currentGroup } = useCurrentGroup()
  return {
    group: currentGroup,
    start: useStartLoan(),
    ret: useReturnLoan(),
    del: useDeleteLoan(),
  }
}
