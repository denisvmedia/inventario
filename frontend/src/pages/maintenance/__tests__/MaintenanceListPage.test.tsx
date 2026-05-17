import { screen } from "@testing-library/react"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { MaintenanceListPage } from "@/pages/maintenance/MaintenanceListPage"
import { groupHandlers, maintenanceHandlers } from "@/test/handlers"
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

function renderPage(initialPath = `/g/${SLUG}/maintenance`) {
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path="/g/:groupSlug/maintenance"
        element={
          <ConfirmProvider>
            <GroupProvider>
              <main>
                <MaintenanceListPage />
              </main>
            </GroupProvider>
          </ConfirmProvider>
        }
      />
    ),
  })
}

describe("<MaintenanceListPage />", () => {
  it("renders the empty state when there are no schedules", async () => {
    server.use(...groupHandlers.list(groupFixture), ...maintenanceHandlers.listGroup(SLUG, []))
    renderPage()
    expect(await screen.findByTestId("maintenance-empty")).toBeInTheDocument()
    expect(screen.queryByTestId("maintenance-table")).toBeNull()
  })

  it("renders a table with one row per schedule + commodity link", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...maintenanceHandlers.listGroup(SLUG, [
        {
          id: "ms-1",
          commodity_id: "c1",
          title: "Replace water filter",
          interval_days: 180,
          next_due_at: "2027-01-01",
          enabled: true,
          commodity: { id: "c1", name: "Fridge" },
        },
      ])
    )
    renderPage()
    const row = await screen.findByTestId("maintenance-row-ms-1")
    expect(row).toBeInTheDocument()
    // Commodity name surfaces as a link to the per-commodity detail
    // with the maintenance tab pre-selected.
    const link = row.querySelector("a")
    expect(link).toBeTruthy()
    expect(link?.getAttribute("href")).toContain("/commodities/c1")
    expect(link?.getAttribute("href")).toContain("tab=maintenance")
  })

  it("flags overdue schedules with the Overdue badge", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...maintenanceHandlers.listGroup(SLUG, [
        {
          id: "ms-2",
          commodity_id: "c2",
          title: "Descale espresso",
          interval_days: 60,
          // 2000-01-01 — guaranteed in the past.
          next_due_at: "2000-01-01",
          enabled: true,
          commodity: { id: "c2", name: "Espresso machine" },
        },
      ])
    )
    renderPage()
    const row = await screen.findByTestId("maintenance-row-ms-2")
    expect(row).toBeInTheDocument()
    // The overdue badge text comes from maintenance:row.overdue.
    expect(row.textContent ?? "").toMatch(/Overdue/i)
  })
})
