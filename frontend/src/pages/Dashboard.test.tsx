import { beforeEach, describe, expect, it } from "vitest"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import i18next from "i18next"

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
    // Hero `total-value` uses `formatCurrency({ compact: true })` so a
    // narrow stat-card cell never clips a long string. Bare "$0" — no cents.
    expect(screen.getByTestId("dashboard-total-value")).toHaveTextContent("$0")
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
    expect(screen.getByTestId("dashboard-total-value")).toHaveTextContent("$4,250")
    // Recent addition rows are sorted newest-first.
    const rows = screen.getAllByTestId("recently-added-row")
    expect(rows).toHaveLength(3)
    expect(rows[0]).toHaveTextContent("Coffee grinder")
    expect(rows[2]).toHaveTextContent("Office chair")
  })

  // #1684: low-denomination currencies (HUF / IDR / VND / KRW / IRR / …)
  // hit 8–9 digit totals routinely and clip the half-screen stat-card
  // cell even with cents dropped. At ≥1e7 the hero hands off to
  // `notation: "compact"` so the total reads as K/M/B instead.
  it("switches the total-value hero to K/M/B compact notation at very large totals", async () => {
    const hufGroup: Schema<"models.LocationGroup">[] = [
      { id: "g1", slug: SLUG, name: "Household", group_currency: "HUF" },
    ]
    server.use(
      ...groupHandlers.list(hufGroup),
      ...commodityHandlers.list(SLUG, []),
      // 100M HUF — well past the 1e7 threshold. The compact form drops
      // every grouping comma and renders "HUF 100M" instead of "HUF
      // 100,000,000" (15 chars, clips on mobile).
      ...commodityHandlers.values(SLUG, { globalTotal: 1e8 })
    )
    renderDashboard()
    // Wait for the commodities query to resolve (matches the loading
    // gate the page uses) before asserting on the value cell.
    await waitFor(() =>
      expect(screen.getByTestId("dashboard-commodities-count")).toHaveTextContent("0")
    )
    // Intl uses a non-breaking space between "HUF" and "100M" — match
    // via regex so the test isn't fragile to which whitespace
    // code-point the runtime picks.
    await waitFor(() =>
      expect(screen.getByTestId("dashboard-total-value").textContent ?? "").toMatch(/HUF\s100M/)
    )
    expect(screen.getByTestId("dashboard-total-value")).not.toHaveTextContent("100,000,000")
  })

  it("links each stat card to its drill-down (commodities or warranties tab)", async () => {
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
    expect(screen.getByTestId("dashboard-active-warranties").closest("a")).toHaveAttribute(
      "href",
      `/g/${SLUG}/warranties?tab=active`
    )
    expect(screen.getByTestId("dashboard-expired-warranties").closest("a")).toHaveAttribute(
      "href",
      `/g/${SLUG}/warranties?tab=expired`
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

  // #1629: regression-protection for the mobile "Add new item" CTA.
  // The sibling sidebar test path covers the full integration; these
  // cases anchor the dashboard-specific visual contract so the navigate
  // target and the migration-lock treatment can't drift independently
  // of #1544 / PR #1621's design decisions.
  it("renders the mobile Add-item CTA pointing at the active group's create route", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...commodityHandlers.list(SLUG, []),
      ...commodityHandlers.values(SLUG, { globalTotal: 0 })
    )
    function NavSentinel() {
      const location = useLocation()
      return (
        <div>
          <span data-testid="nav-sentinel-path">{location.pathname}</span>
        </div>
      )
    }
    setAccessToken("good-token")
    renderWithProviders({
      initialPath: `/g/${SLUG}`,
      routes: (
        <>
          <Route
            path="/g/:groupSlug"
            element={
              <GroupProvider>
                <DashboardPage />
              </GroupProvider>
            }
          />
          <Route path="/g/:groupSlug/commodities/new" element={<NavSentinel />} />
        </>
      ),
    })
    const cta = await screen.findByTestId("dashboard-mobile-add-item")
    // The button defers navigation to react-router (so cursor matches
    // the design-mock — see Dashboard.tsx L100-L116). aria-disabled is
    // absent in the unlocked path, and `title` stays unset.
    expect(cta).not.toHaveAttribute("aria-disabled")
    expect(cta).not.toHaveAttribute("title")
    // Click → navigates to /g/:slug/commodities/new. The sentinel
    // resolves once react-router has replaced the URL.
    const user = userEvent.setup()
    await user.click(cta)
    expect(await screen.findByTestId("nav-sentinel-path")).toHaveTextContent(
      `/g/${SLUG}/commodities/new`
    )
  })

  it("flips the mobile Add-item CTA to aria-disabled with the migration-lock title when locked", async () => {
    const lockedGroup: Schema<"models.LocationGroup">[] = [
      {
        id: "g1",
        slug: SLUG,
        name: "Household",
        group_currency: "USD",
        currency_migration_id: "mig-42",
      },
    ]
    server.use(
      ...groupHandlers.list(lockedGroup),
      ...commodityHandlers.list(SLUG, []),
      ...commodityHandlers.values(SLUG, { globalTotal: 0 })
    )
    renderDashboard()
    const cta = await screen.findByTestId("dashboard-mobile-add-item")
    // useGroupMigrationLock() resolves after the /groups query lands,
    // so wait for the lock state to flip before asserting attributes.
    await waitFor(() => expect(cta).toHaveAttribute("aria-disabled", "true"))
    // i18n surfaces `errors:lockedDuringMigration` as the title.
    // Anchor on the key via `i18next.t(...)` (en bundle loaded by
    // `test/setup.ts`) so a copy-edit in the en JSON doesn't break
    // this test — only renaming or removing the key does.
    expect(cta).toHaveAttribute("title", i18next.t("errors:lockedDuringMigration"))
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
