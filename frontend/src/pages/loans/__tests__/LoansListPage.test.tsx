import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { LoansListPage } from "@/pages/loans/LoansListPage"
import { groupHandlers, loanHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  // Pre-arm the http client's slug slot — GroupProvider mirrors it via
  // useEffect (post-first-paint), but the first useGroupLoans fetch
  // can fire before that effect resolves; without this arming, the
  // request leaves without /g/{slug}/ and MSW returns 404.
  setCurrentGroupSlug(SLUG)
})

function renderPage(initialPath = `/g/${SLUG}/lent`) {
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path="/g/:groupSlug/lent"
        element={
          <ConfirmProvider>
            <GroupProvider>
              <main>
                <LoansListPage />
              </main>
            </GroupProvider>
          </ConfirmProvider>
        }
      />
    ),
  })
}

describe("<LoansListPage />", () => {
  it("renders the empty state when there are no loans", async () => {
    server.use(...groupHandlers.list(groupFixture), ...loanHandlers.listGroup(SLUG, []))
    renderPage()
    expect(await screen.findByTestId("lent-empty")).toBeInTheDocument()
    expect(screen.queryByTestId("lent-table")).toBeNull()
  })

  it("renders the table with one row per loan + commodity link", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...loanHandlers.listGroup(SLUG, [
        {
          id: "loan-1",
          commodity_id: "c1",
          borrower_name: "Alice",
          lent_at: "2026-04-01",
          due_back_at: "2026-04-15",
          returned_at: null,
          commodity: { id: "c1", name: "Cordless Drill" },
        },
        {
          id: "loan-2",
          commodity_id: "c2",
          borrower_name: "Bob",
          lent_at: "2026-03-01",
          returned_at: "2026-03-10",
          commodity: { id: "c2", name: "Camping Tent" },
        },
      ])
    )
    renderPage()
    expect(await screen.findByTestId("lent-table")).toBeInTheDocument()
    const row1 = screen.getByTestId("lent-row-loan-1")
    expect(row1).toHaveTextContent("Cordless Drill")
    expect(row1).toHaveTextContent("Alice")
    const row2 = screen.getByTestId("lent-row-loan-2")
    expect(row2).toHaveTextContent("Camping Tent")
    expect(row2).toHaveTextContent("Bob")
  })

  it("updates the URL state when a tab is clicked", async () => {
    server.use(...groupHandlers.list(groupFixture), ...loanHandlers.listGroup(SLUG, []))
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("lent-empty")
    await user.click(screen.getByTestId("lent-state-overdue"))
    // The "overdue" tab is now selected (aria-selected=true).
    expect(screen.getByTestId("lent-state-overdue")).toHaveAttribute("aria-selected", "true")
  })

  it("preselects the tab matching ?state=open", async () => {
    server.use(...groupHandlers.list(groupFixture), ...loanHandlers.listGroup(SLUG, []))
    renderPage(`/g/${SLUG}/lent?state=open`)
    await screen.findByTestId("lent-empty")
    expect(screen.getByTestId("lent-state-open")).toHaveAttribute("aria-selected", "true")
  })

  it("is axe-clean once data has loaded", async () => {
    server.use(...groupHandlers.list(groupFixture), ...loanHandlers.listGroup(SLUG, []))
    const { baseElement } = renderPage()
    await screen.findByTestId("lent-empty")
    const results = await axe(baseElement)
    expect(results).toHaveNoViolations()
  })
})
