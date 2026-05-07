import { beforeAll, beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { WarrantiesListPage } from "@/pages/warranties/WarrantiesListPage"
import { GroupProvider } from "@/features/group/GroupContext"
import { initI18n } from "@/i18n"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { commodityHandlers, groupHandlers } from "@/test/handlers"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

function commodityRes(id: string, attrs: Record<string, unknown>) {
  return {
    id,
    type: "commodities",
    attributes: { id, count: 1, status: "in_use", type: "other", area_id: "a1", ...attrs },
  }
}

function renderPage(initialPath = `/g/${SLUG}/warranties`) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path="/g/:groupSlug/warranties"
        element={
          <GroupProvider>
            <main>
              <WarrantiesListPage />
            </main>
          </GroupProvider>
        }
      />
    ),
  })
}

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  // Pre-arm the slug slot — useCommodities can fire before
  // GroupProvider's useEffect mirrors `currentGroup.slug` into the
  // http client, and without arming the request would 404.
  setCurrentGroupSlug(SLUG)
})

describe("<WarrantiesListPage />", () => {
  it("defaults to the Expiring tab and shows an empty state when nothing is expiring", async () => {
    server.use(...groupHandlers.list(groupFixture), ...commodityHandlers.list(SLUG, []))
    renderPage()
    expect(await screen.findByTestId("warranties-empty")).toBeInTheDocument()
    expect(screen.queryByTestId("warranties-list")).toBeNull()
    expect(screen.getByTestId("warranties-tab-expiring")).toHaveAttribute("aria-selected", "true")
  })

  it("renders one row per commodity bucketed into the active tab + summary counts", async () => {
    // 2099-01-01 → active; <today>+30d → expiring; 1999-01-01 → expired.
    const todayPlus30 = new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10)
    server.use(
      ...groupHandlers.list(groupFixture),
      ...commodityHandlers.list(SLUG, [
        commodityRes("c-active", { name: "Fridge", warranty_expires_at: "2099-01-01" }),
        commodityRes("c-expiring", { name: "Kettle", warranty_expires_at: todayPlus30 }),
        commodityRes("c-expired", { name: "Toaster", warranty_expires_at: "1999-01-01" }),
      ])
    )
    renderPage()
    await waitFor(() => expect(screen.getByTestId("warranties-list")).toBeInTheDocument())
    // Only the Expiring bucket is on screen (default tab).
    expect(screen.getByTestId("warranties-row-c-expiring")).toHaveTextContent("Kettle")
    expect(screen.queryByTestId("warranties-row-c-active")).toBeNull()
    expect(screen.queryByTestId("warranties-row-c-expired")).toBeNull()
    // Summary cards still know about everything else.
    expect(
      within(screen.getByTestId("warranties-summary-active")).getByText("1")
    ).toBeInTheDocument()
    expect(
      within(screen.getByTestId("warranties-summary-expiring")).getByText("1")
    ).toBeInTheDocument()
    expect(
      within(screen.getByTestId("warranties-summary-expired")).getByText("1")
    ).toBeInTheDocument()
    // Per-tab counter shows for Expiring (the default + the populated bucket).
    expect(screen.getByTestId("warranties-tab-expiring-count")).toHaveTextContent("1")
  })

  it("switches buckets when a tab is clicked", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...commodityHandlers.list(SLUG, [
        commodityRes("c-active", { name: "Fridge", warranty_expires_at: "2099-01-01" }),
      ])
    )
    const user = userEvent.setup()
    renderPage()
    // Expiring is the default, so the Active row only shows after the
    // user switches tabs.
    await screen.findByTestId("warranties-empty")
    await user.click(screen.getByTestId("warranties-tab-active"))
    expect(screen.getByTestId("warranties-tab-active")).toHaveAttribute("aria-selected", "true")
    expect(await screen.findByTestId("warranties-row-c-active")).toHaveTextContent("Fridge")
  })

  it("preselects the tab matching ?tab=expired", async () => {
    server.use(...groupHandlers.list(groupFixture), ...commodityHandlers.list(SLUG, []))
    renderPage(`/g/${SLUG}/warranties?tab=expired`)
    await screen.findByTestId("warranties-empty")
    expect(screen.getByTestId("warranties-tab-expired")).toHaveAttribute("aria-selected", "true")
  })
})
