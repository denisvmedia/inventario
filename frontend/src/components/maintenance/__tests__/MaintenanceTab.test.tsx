import { screen } from "@testing-library/react"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { MaintenanceTab } from "@/components/maintenance/MaintenanceTab"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { groupHandlers, maintenanceHandlers } from "@/test/handlers"
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
                <MaintenanceTab commodityId={COMMODITY_ID} commodityCount={commodityCount} />
              </main>
            </GroupProvider>
          </ConfirmProvider>
        }
      />
    ),
  })
}

describe("<MaintenanceTab />", () => {
  it("renders the empty state when no schedules exist", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...maintenanceHandlers.listForCommodity(SLUG, COMMODITY_ID, [])
    )
    renderTab()
    expect(await screen.findByTestId("maintenance-add")).toBeInTheDocument()
    expect(await screen.findByText(/No maintenance schedules yet/i)).toBeInTheDocument()
  })

  it("renders a row with the per-schedule Mark Done CTA", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...maintenanceHandlers.listForCommodity(SLUG, COMMODITY_ID, [
        {
          id: "ms-1",
          commodity_id: COMMODITY_ID,
          title: "Descale espresso machine",
          interval_days: 60,
          next_due_at: "2027-01-01",
          enabled: true,
        },
      ])
    )
    renderTab()
    expect(await screen.findByTestId("maintenance-list")).toBeInTheDocument()
    expect(screen.getByTestId("schedule-ms-1-done")).toBeInTheDocument()
    expect(screen.getByTestId("schedule-ms-1-next-due")).toBeInTheDocument()
  })

  it("renders the bundle hint instead of the form for count > 1 rows", () => {
    server.use(...groupHandlers.list(groupFixture))
    renderTab(/* commodityCount */ 5)
    expect(screen.getByText(/can't be set on a bundle/i, { selector: "*" })).toBeInTheDocument()
    expect(screen.queryByTestId("maintenance-add")).toBeNull()
  })
})
