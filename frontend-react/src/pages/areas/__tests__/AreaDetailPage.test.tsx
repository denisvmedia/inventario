import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"

import { AreaDetailPage } from "@/pages/areas/AreaDetailPage"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { areaHandlers, groupHandlers, locationHandlers } from "@/test/handlers"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", main_currency: "USD" },
]

function locationResource(id: string, attrs: { name: string; address?: string }) {
  return { id, type: "locations", attributes: { ...attrs, id } }
}

function areaResource(id: string, attrs: { name: string; location_id: string }) {
  return { id, type: "areas", attributes: { ...attrs, id } }
}

function renderDetail(initialPath: string, props: { initialMode?: "edit" } = {}) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path="/g/:groupSlug/areas/:id"
        element={
          <ConfirmProvider>
            <GroupProvider>
              <AreaDetailPage {...props} />
            </GroupProvider>
          </ConfirmProvider>
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

describe("<AreaDetailPage />", () => {
  it("renders the area's name + parent-location breadcrumb", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.detail(
        SLUG,
        "a1",
        areaResource("a1", {
          name: "Kitchen",
          location_id: "loc1",
        })
      ),
      ...locationHandlers.detail(
        SLUG,
        "loc1",
        locationResource("loc1", {
          name: "Main House",
        })
      ),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Main House" })])
    )
    renderDetail(`/g/${SLUG}/areas/a1`)
    await waitFor(() => expect(screen.getByText("Kitchen")).toBeInTheDocument())
    // Parent-location resolves on a chained fetch (area → location_id
    // → /locations/:id), so wait again for the breadcrumb.
    await waitFor(() => {
      expect(screen.getAllByText("Main House").length).toBeGreaterThan(0)
    })
    // The "items coming soon" alert references #1410.
    expect(screen.getByTestId("area-detail-items-soon")).toBeInTheDocument()
  })

  it("auto-opens the edit dialog when initialMode='edit'", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.detail(
        SLUG,
        "a1",
        areaResource("a1", {
          name: "Workshop",
          location_id: "loc1",
        })
      ),
      ...locationHandlers.detail(SLUG, "loc1", locationResource("loc1", { name: "Garage" })),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Garage" })])
    )
    renderDetail(`/g/${SLUG}/areas/a1`, { initialMode: "edit" })
    expect(await screen.findByTestId("area-form-dialog")).toBeInTheDocument()
  })
})
