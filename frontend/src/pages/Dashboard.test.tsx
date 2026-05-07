import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import { axe } from "jest-axe"

import { DashboardPage } from "@/pages/Dashboard"
import { GroupProvider } from "@/features/group/GroupContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { commodityHandlers, groupHandlers } from "@/test/handlers"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

function commodityResource(id: string, attrs: Record<string, unknown>) {
  return { id, type: "commodities", attributes: attrs }
}

// Mounts the dashboard at /g/:groupSlug so GroupProvider's useParams()
// resolves the slug — the http client then rewrites /commodities ->
// /g/household/commodities and our MSW handlers match.
function renderDashboard() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: `/g/${SLUG}`,
    routes: (
      <Route
        path="/g/:groupSlug"
        element={
          <GroupProvider>
            <DashboardPage />
          </GroupProvider>
        }
      />
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<DashboardPage />", () => {
  it("renders the heading + tagline", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...commodityHandlers.list(SLUG, []),
      ...commodityHandlers.values(SLUG, { globalTotal: 0 })
    )
    renderDashboard()
    expect(await screen.findByRole("heading", { name: /overview/i, level: 1 })).toBeInTheDocument()
    expect(screen.getByText(/everything you own/i)).toBeInTheDocument()
  })

  it("shows zero totals + the empty 'recently added' state for a fresh group", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...commodityHandlers.list(SLUG, []),
      ...commodityHandlers.values(SLUG, { globalTotal: 0 })
    )
    renderDashboard()
    await waitFor(() =>
      expect(screen.getByTestId("dashboard-commodities-count")).toHaveTextContent("0")
    )
    expect(screen.getByTestId("dashboard-total-value")).toHaveTextContent("$0.00")
    expect(screen.getByText(/nothing here yet/i)).toBeInTheDocument()
  })

  it("renders real totals + recent additions from the API", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...commodityHandlers.list(SLUG, [
        commodityResource("c1", { name: "MacBook Pro", registered_date: "2026-04-20" }),
        commodityResource("c2", { name: "Coffee grinder", registered_date: "2026-04-25" }),
        commodityResource("c3", { name: "Office chair", registered_date: "2026-04-10" }),
      ]),
      ...commodityHandlers.values(SLUG, { globalTotal: 4250 })
    )
    renderDashboard()
    await waitFor(() =>
      expect(screen.getByTestId("dashboard-commodities-count")).toHaveTextContent("3")
    )
    expect(screen.getByTestId("dashboard-total-value")).toHaveTextContent("$4,250.00")
    // Recent addition rows are sorted newest-first.
    const rows = screen.getAllByTestId("recently-added-row")
    expect(rows).toHaveLength(3)
    expect(rows[0]).toHaveTextContent("Coffee grinder")
    expect(rows[2]).toHaveTextContent("Office chair")
  })

  it("links each stat card to /g/:slug/commodities (or /locations)", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...commodityHandlers.list(SLUG, []),
      ...commodityHandlers.values(SLUG, {})
    )
    renderDashboard()
    await waitFor(() =>
      expect(screen.getByTestId("dashboard-commodities-count").closest("a")).toHaveAttribute(
        "href",
        `/g/${SLUG}/commodities`
      )
    )
    expect(screen.getByTestId("dashboard-total-value").closest("a")).toHaveAttribute(
      "href",
      `/g/${SLUG}/commodities`
    )
    expect(screen.getByTestId("dashboard-locations-count").closest("a")).toHaveAttribute(
      "href",
      `/g/${SLUG}/locations`
    )
  })

  it("renders the Warranty Health bars + counts (#1529)", async () => {
    const todayPlus30 = new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10)
    server.use(
      ...groupHandlers.list(groupFixture),
      ...commodityHandlers.list(SLUG, [
        commodityResource("c-active", { name: "Fridge", warranty_expires_at: "2099-01-01" }),
        commodityResource("c-expiring", { name: "Kettle", warranty_expires_at: todayPlus30 }),
        commodityResource("c-expired", { name: "Toaster", warranty_expires_at: "1999-01-01" }),
        commodityResource("c-none", { name: "Lamp" }),
      ]),
      ...commodityHandlers.values(SLUG, { globalTotal: 0 })
    )
    renderDashboard()
    // The card mounts during the loading-skeleton phase too, so wait
    // for one of its inner status rows (which only render once data
    // resolves) before asserting counts.
    expect(await screen.findByTestId("dashboard-warranty-health-active")).toHaveTextContent("1")
    expect(screen.getByTestId("dashboard-warranty-health-expiring")).toHaveTextContent("1")
    expect(screen.getByTestId("dashboard-warranty-health-expired")).toHaveTextContent("1")
    expect(screen.getByTestId("dashboard-warranty-health-none")).toHaveTextContent("1")
  })

  it("renders the Expiring Warranties panel with one row per expiring item", async () => {
    const todayPlus10 = new Date(Date.now() + 10 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10)
    const todayPlus40 = new Date(Date.now() + 40 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10)
    server.use(
      ...groupHandlers.list(groupFixture),
      ...commodityHandlers.list(SLUG, [
        commodityResource("c-soon", { name: "Kettle", warranty_expires_at: todayPlus10 }),
        commodityResource("c-later", { name: "Mixer", warranty_expires_at: todayPlus40 }),
      ]),
      ...commodityHandlers.values(SLUG, { globalTotal: 0 })
    )
    renderDashboard()
    // findAllByTestId polls until at least one row is present, which
    // guarantees the loading skeleton has resolved into the real list.
    const rows = await screen.findAllByTestId("dashboard-expiring-warranty-row")
    expect(rows).toHaveLength(2)
    // Sorted by expiry ascending — Kettle (10d) ahead of Mixer (40d).
    expect(rows[0]).toHaveTextContent("Kettle")
    expect(rows[1]).toHaveTextContent("Mixer")
  })

  it("shows the Expiring Warranties empty state when nothing is expiring", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...commodityHandlers.list(SLUG, [
        commodityResource("c-active", { name: "Fridge", warranty_expires_at: "2099-01-01" }),
      ]),
      ...commodityHandlers.values(SLUG, { globalTotal: 0 })
    )
    renderDashboard()
    expect(await screen.findByTestId("dashboard-expiring-warranties-empty")).toBeInTheDocument()
  })

  it("renders an error alert when an upstream query fails", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...commodityHandlers.error(SLUG, 500),
      ...commodityHandlers.valuesError(SLUG, 500)
    )
    renderDashboard()
    expect(await screen.findByTestId("dashboard-error")).toBeInTheDocument()
    // Stat cards must NOT render alongside the error — the user
    // shouldn't see "0 items" when the load failed.
    expect(screen.queryByTestId("dashboard-commodities-count")).not.toBeInTheDocument()
  })

  it("has no axe violations once data has loaded", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...commodityHandlers.list(SLUG, [
        commodityResource("c1", { name: "MacBook Pro", registered_date: "2026-04-20" }),
      ]),
      ...commodityHandlers.values(SLUG, { globalTotal: 1500 })
    )
    const { container } = renderDashboard()
    await waitFor(() =>
      expect(screen.getByTestId("dashboard-commodities-count")).toHaveTextContent("1")
    )
    expect(await axe(container)).toHaveNoViolations()
  })
})
