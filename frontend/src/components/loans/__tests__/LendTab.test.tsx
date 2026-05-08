import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { LendTab } from "@/components/loans/LendTab"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { groupHandlers, loanHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const COMMODITY_ID = "commodity-1"
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
  // Pre-arm the http client's slug slot. GroupProvider mirrors the
  // URL slug via a useEffect that runs *after* first paint, but the
  // very first refetch in a TanStack Query mutation can fire before
  // that effect has resolved — leading to an un-rewritten request and
  // a cached 404 that the assertion-on-disappear paths depend on.
  // Setting the slug directly mirrors what the live GroupProvider's
  // effect will (re-)stamp anyway.
  setCurrentGroupSlug(SLUG)
})

function renderTab(commodityCount?: number) {
  return renderWithProviders({
    initialPath: `/g/${SLUG}/commodities/${COMMODITY_ID}`,
    routes: (
      <Route
        path="/g/:groupSlug/commodities/:id"
        element={
          <ConfirmProvider>
            <GroupProvider>
              <main>
                <LendTab commodityId={COMMODITY_ID} commodityCount={commodityCount} />
              </main>
            </GroupProvider>
          </ConfirmProvider>
        }
      />
    ),
  })
}

describe("<LendTab />", () => {
  it("renders the empty state + Lend button when there are no loans", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...loanHandlers.listForCommodity(SLUG, COMMODITY_ID, [])
    )
    renderTab()
    expect(await screen.findByTestId("lend-empty-state")).toBeInTheDocument()
    expect(screen.getByTestId("commodity-detail-lend-button")).toBeInTheDocument()
    expect(screen.queryByTestId("lend-current")).toBeNull()
    expect(screen.queryByTestId("lend-history")).toBeNull()
  })

  it("renders the current-loan card when an open loan exists", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...loanHandlers.listForCommodity(SLUG, COMMODITY_ID, [
        {
          id: "loan-1",
          commodity_id: COMMODITY_ID,
          borrower_name: "Alice",
          borrower_contact: "alice@example.com",
          lent_at: "2026-04-01",
          due_back_at: "2026-04-15",
          returned_at: null,
        },
      ])
    )
    renderTab()
    const current = await screen.findByTestId("lend-current")
    expect(current).toHaveTextContent("Alice")
    expect(current).toHaveTextContent("alice@example.com")
    // Past due — overdue badge should be present.
    expect(screen.getByTestId("lend-overdue-badge")).toBeInTheDocument()
    // No "Lend out" button while a loan is open.
    expect(screen.queryByTestId("commodity-detail-lend-button")).toBeNull()
  })

  it("renders the history list for closed loans", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...loanHandlers.listForCommodity(SLUG, COMMODITY_ID, [
        {
          id: "loan-2",
          commodity_id: COMMODITY_ID,
          borrower_name: "Bob",
          lent_at: "2026-03-01",
          returned_at: "2026-03-10",
        },
      ])
    )
    renderTab()
    expect(await screen.findByTestId("lend-history")).toBeInTheDocument()
    expect(screen.getByTestId("lend-history-row-loan-2")).toHaveTextContent("Bob")
    // No open loan → empty state below the card header.
    expect(screen.getByTestId("lend-empty-state")).toBeInTheDocument()
  })

  it("opens the LendDialog when the Lend button is clicked", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...loanHandlers.listForCommodity(SLUG, COMMODITY_ID, [])
    )
    const user = userEvent.setup()
    renderTab()
    await user.click(await screen.findByTestId("commodity-detail-lend-button"))
    expect(await screen.findByTestId("lend-dialog")).toBeVisible()
  })

  it("submits the LendDialog and fires the start-loan request", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...loanHandlers.listForCommodity(SLUG, COMMODITY_ID, []),
      ...loanHandlers.startLoan(SLUG, COMMODITY_ID, {
        id: "loan-new",
        commodity_id: COMMODITY_ID,
        borrower_name: "Alice",
        lent_at: "2026-05-05",
        returned_at: null,
      })
    )
    const user = userEvent.setup()
    renderTab()
    await user.click(await screen.findByTestId("commodity-detail-lend-button"))
    await user.type(screen.getByTestId("lend-borrower-name"), "Alice")
    await user.click(screen.getByTestId("lend-submit"))
    // Dialog closes on success — wait for it to unmount.
    await waitFor(() => expect(screen.queryByTestId("lend-dialog")).toBeNull())
  })

  it("marks the open loan as returned via the confirm dialog", async () => {
    const openLoan = {
      id: "loan-open",
      commodity_id: COMMODITY_ID,
      borrower_name: "Alice",
      lent_at: "2026-04-01",
      returned_at: null,
    }
    let returnHit = false
    server.use(
      ...groupHandlers.list(groupFixture),
      ...loanHandlers.listForCommodity(SLUG, COMMODITY_ID, [openLoan])
    )
    // Override the return handler with a one-shot probe so the test can
    // assert that the click actually reached the BE — driving coverage
    // of handleReturn without depending on the post-state refresh
    // (which would need a stateful list handler we don't have).
    server.use(
      ...loanHandlers.returnLoan(SLUG, COMMODITY_ID, "loan-open", {
        ...openLoan,
        returned_at: "2026-05-05",
      })
    )
    const realFetch = window.fetch
    window.fetch = (...args: Parameters<typeof fetch>) => {
      const url = String(args[0])
      const init = args[1] as RequestInit | undefined
      if (init?.method === "POST" && url.includes("/loans/loan-open/return")) {
        returnHit = true
      }
      return realFetch(...args)
    }
    try {
      const user = userEvent.setup()
      renderTab()
      await user.click(await screen.findByTestId("lend-mark-returned"))
      await user.click(await screen.findByTestId("confirm-accept"))
      await waitFor(() => expect(returnHit).toBe(true))
    } finally {
      window.fetch = realFetch
    }
  })

  it("removes the loan record via the confirm dialog", async () => {
    const openLoan = {
      id: "loan-open",
      commodity_id: COMMODITY_ID,
      borrower_name: "Alice",
      lent_at: "2026-04-01",
      returned_at: null,
    }
    let deleteHit = false
    server.use(
      ...groupHandlers.list(groupFixture),
      ...loanHandlers.listForCommodity(SLUG, COMMODITY_ID, [openLoan]),
      ...loanHandlers.deleteLoan(SLUG, COMMODITY_ID, "loan-open")
    )
    const realFetch = window.fetch
    window.fetch = (...args: Parameters<typeof fetch>) => {
      const url = String(args[0])
      const init = args[1] as RequestInit | undefined
      if (init?.method === "DELETE" && url.includes("/loans/loan-open")) {
        deleteHit = true
      }
      return realFetch(...args)
    }
    try {
      const user = userEvent.setup()
      renderTab()
      await user.click(await screen.findByTestId("lend-delete"))
      await user.click(await screen.findByTestId("confirm-accept"))
      await waitFor(() => expect(deleteHit).toBe(true))
    } finally {
      window.fetch = realFetch
    }
  })

  // Issue #1554: bundle commodities (count > 1) cannot be lent out —
  // the row models a bag of interchangeable units, not a single
  // tracked instance. The tab swaps its body for the empty-state hint
  // and hides the Lend CTA so the user can't even open the dialog.
  it("renders the bundle empty-state and hides the Lend button when count > 1", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...loanHandlers.listForCommodity(SLUG, COMMODITY_ID, [])
    )
    renderTab(12)
    expect(await screen.findByTestId("lend-bundle-empty-state")).toBeInTheDocument()
    expect(screen.queryByTestId("commodity-detail-lend-button")).toBeNull()
    expect(screen.queryByTestId("lend-empty-state")).toBeNull()
  })

  it("is axe-clean once data has loaded", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...loanHandlers.listForCommodity(SLUG, COMMODITY_ID, [])
    )
    const { baseElement } = renderTab()
    await screen.findByTestId("lend-empty-state")
    await waitFor(async () => {
      const results = await axe(baseElement)
      expect(results).toHaveNoViolations()
    })
  })
})
