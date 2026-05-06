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
import { ServicesListPage } from "@/pages/services/ServicesListPage"
import { groupHandlers, serviceHandlers } from "@/test/handlers"
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
  setCurrentGroupSlug(SLUG)
})

function renderPage(initialPath = `/g/${SLUG}/in-service`) {
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path="/g/:groupSlug/in-service"
        element={
          <ConfirmProvider>
            <GroupProvider>
              <main>
                <ServicesListPage />
              </main>
            </GroupProvider>
          </ConfirmProvider>
        }
      />
    ),
  })
}

describe("<ServicesListPage />", () => {
  it("renders the empty state when there are no service rows", async () => {
    server.use(...groupHandlers.list(groupFixture), ...serviceHandlers.listGroup(SLUG, []))
    renderPage()
    expect(await screen.findByTestId("in-service-empty")).toBeInTheDocument()
    expect(screen.queryByTestId("in-service-table")).toBeNull()
  })

  it("renders the table with one row per service + commodity link + cost", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...serviceHandlers.listGroup(SLUG, [
        {
          id: "svc-1",
          commodity_id: "c1",
          provider_name: "Apple Service",
          provider_contact: "+1 800-275-2273",
          reason: "screen replacement",
          sent_at: "2026-04-01",
          expected_return_at: "2026-04-15",
          returned_at: null,
          commodity: { id: "c1", name: "MacBook Pro" },
        },
        {
          id: "svc-2",
          commodity_id: "c2",
          provider_name: "Bob's Repair Shop",
          reason: "battery swap",
          sent_at: "2026-03-01",
          returned_at: "2026-03-10",
          cost_amount: "245.00",
          cost_currency: "EUR",
          commodity: { id: "c2", name: "Cordless Drill" },
        },
      ])
    )
    renderPage()
    // useGroupServices is gated on `currentGroup?.slug` (#1517 review),
    // so on cold mount the query is disabled until GroupProvider's
    // useGroups fetch resolves and the URL slug populates the context.
    // findByTestId polls, getByTestId does not — the row lookup must
    // poll too, otherwise we read the table before its rows arrive.
    const row1 = await screen.findByTestId("in-service-row-svc-1")
    expect(row1).toHaveTextContent("MacBook Pro")
    expect(row1).toHaveTextContent("Apple Service")
    expect(row1).toHaveTextContent("screen replacement")
    const row2 = screen.getByTestId("in-service-row-svc-2")
    expect(row2).toHaveTextContent("Cordless Drill")
    expect(row2).toHaveTextContent("Bob's Repair Shop")
    // Cost column rendered when both fields set.
    expect(row2).toHaveTextContent("245.00")
    expect(row2).toHaveTextContent("EUR")
  })

  it("renders an em-dash placeholder when no commodity ref is attached", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...serviceHandlers.listGroup(SLUG, [
        {
          id: "svc-orphan",
          commodity_id: "missing",
          provider_name: "Apple Service",
          sent_at: "2026-04-01",
          returned_at: null,
        },
      ])
    )
    renderPage()
    const row = await screen.findByTestId("in-service-row-svc-orphan")
    // First cell is the commodity link or "—" when commodity ref is absent.
    expect(row).toHaveTextContent("—")
  })

  it("updates the URL state when a tab is clicked", async () => {
    server.use(...groupHandlers.list(groupFixture), ...serviceHandlers.listGroup(SLUG, []))
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("in-service-empty")
    await user.click(screen.getByTestId("in-service-state-overdue"))
    expect(screen.getByTestId("in-service-state-overdue")).toHaveAttribute("aria-selected", "true")
  })

  it("preselects the tab matching ?state=completed", async () => {
    server.use(...groupHandlers.list(groupFixture), ...serviceHandlers.listGroup(SLUG, []))
    renderPage(`/g/${SLUG}/in-service?state=completed`)
    await screen.findByTestId("in-service-empty")
    expect(screen.getByTestId("in-service-state-completed")).toHaveAttribute(
      "aria-selected",
      "true"
    )
  })

  it("falls back to 'all' for an unknown state param", async () => {
    server.use(...groupHandlers.list(groupFixture), ...serviceHandlers.listGroup(SLUG, []))
    renderPage(`/g/${SLUG}/in-service?state=bogus`)
    await screen.findByTestId("in-service-empty")
    expect(screen.getByTestId("in-service-state-all")).toHaveAttribute("aria-selected", "true")
  })

  it("clicking 'all' clears the ?state= param from the URL", async () => {
    server.use(...groupHandlers.list(groupFixture), ...serviceHandlers.listGroup(SLUG, []))
    const user = userEvent.setup()
    renderPage(`/g/${SLUG}/in-service?state=open`)
    await screen.findByTestId("in-service-empty")
    await user.click(screen.getByTestId("in-service-state-all"))
    expect(screen.getByTestId("in-service-state-all")).toHaveAttribute("aria-selected", "true")
  })

  it("renders the appropriate badge variant per row state", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...serviceHandlers.listGroup(SLUG, [
        // Returned row.
        {
          id: "svc-returned",
          commodity_id: "c1",
          provider_name: "Apple Service",
          sent_at: "2026-03-01",
          returned_at: "2026-03-10",
          commodity: { id: "c1", name: "Item A" },
        },
        // Overdue row (expected return in the past, no returned_at).
        {
          id: "svc-overdue",
          commodity_id: "c2",
          provider_name: "Bob's Shop",
          sent_at: "2026-03-01",
          expected_return_at: "2026-03-05",
          returned_at: null,
          commodity: { id: "c2", name: "Item B" },
        },
        // Open, not yet overdue.
        {
          id: "svc-open",
          commodity_id: "c3",
          provider_name: "Generic Workshop",
          sent_at: new Date().toISOString().slice(0, 10),
          expected_return_at: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000)
            .toISOString()
            .slice(0, 10),
          returned_at: null,
          commodity: { id: "c3", name: "Item C" },
        },
      ])
    )
    renderPage()
    // findByTestId on the first row polls past the gated-on-slug
    // initial paint; the rest are siblings and resolve synchronously.
    expect(await screen.findByTestId("in-service-row-svc-returned")).toHaveTextContent(/Back/i)
    expect(screen.getByTestId("in-service-row-svc-overdue")).toHaveTextContent(/Overdue/i)
    expect(screen.getByTestId("in-service-row-svc-open")).toHaveTextContent(/At provider/i)
  })

  it("is axe-clean once data has loaded", async () => {
    server.use(...groupHandlers.list(groupFixture), ...serviceHandlers.listGroup(SLUG, []))
    const { baseElement } = renderPage()
    await screen.findByTestId("in-service-empty")
    const results = await axe(baseElement)
    expect(results).toHaveNoViolations()
  })
})
