import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { LocationsListPage } from "@/pages/locations/LocationsListPage"
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

function renderList(initialPath = `/g/${SLUG}/locations`) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path="/g/:groupSlug/locations"
        element={
          <ConfirmProvider>
            <GroupProvider>
              <LocationsListPage />
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

describe("<LocationsListPage />", () => {
  it("renders heading + empty state for a group with no locations", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.list(SLUG, []),
      ...areaHandlers.list(SLUG, [])
    )
    renderList()
    await waitFor(() => {
      expect(screen.queryByTestId("locations-empty")).toBeInTheDocument()
    })
  })

  it("renders locations + their nested areas", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.list(SLUG, [
        locationResource("loc1", { name: "Main House", address: "12 Elm St" }),
        locationResource("loc2", { name: "Garage" }),
      ]),
      ...areaHandlers.list(SLUG, [
        areaResource("a1", { name: "Kitchen", location_id: "loc1" }),
        areaResource("a2", { name: "Workshop", location_id: "loc2" }),
      ])
    )
    renderList()
    await waitFor(() => expect(screen.getAllByTestId("location-card")).toHaveLength(2))
    expect(screen.getByText("Main House")).toBeInTheDocument()
    expect(screen.getByText("12 Elm St")).toBeInTheDocument()
    expect(screen.getByText("Kitchen")).toBeInTheDocument()
    expect(screen.getByText("Workshop")).toBeInTheDocument()
  })

  it("filters by query against location and area names", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.list(SLUG, [
        locationResource("loc1", { name: "Main House" }),
        locationResource("loc2", { name: "Garage" }),
      ]),
      ...areaHandlers.list(SLUG, [areaResource("a1", { name: "Kitchen", location_id: "loc1" })])
    )
    renderList()
    await waitFor(() => expect(screen.getAllByTestId("location-card")).toHaveLength(2))
    const search = screen.getByTestId("locations-search")
    const user = userEvent.setup()
    await user.type(search, "kitchen")
    // Searching by area name surfaces the parent location only.
    await waitFor(() => {
      expect(screen.getAllByTestId("location-card")).toHaveLength(1)
      expect(screen.getByText("Main House")).toBeInTheDocument()
    })
  })

  it("opens the create dialog from the header CTA", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.list(SLUG, []),
      ...areaHandlers.list(SLUG, [])
    )
    renderList()
    const user = userEvent.setup()
    // Wait for the page's data fetch to settle before clicking — Radix
    // Dialog can swallow the open signal if the click lands during
    // React's first commit.
    await screen.findByTestId("locations-empty")
    await user.click(screen.getByTestId("locations-add-button"))
    expect(await screen.findByTestId("location-form-dialog")).toBeInTheDocument()
  })

  it("auto-opens the create dialog when mounted at /locations/new", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.list(SLUG, []),
      ...areaHandlers.list(SLUG, [])
    )
    setAccessToken("good-token")
    renderWithProviders({
      initialPath: `/g/${SLUG}/locations/new`,
      routes: (
        <>
          <Route
            path="/g/:groupSlug/locations"
            element={
              <ConfirmProvider>
                <GroupProvider>
                  <LocationsListPage />
                </GroupProvider>
              </ConfirmProvider>
            }
          />
          <Route
            path="/g/:groupSlug/locations/new"
            element={
              <ConfirmProvider>
                <GroupProvider>
                  <LocationsListPage initialMode="create" />
                </GroupProvider>
              </ConfirmProvider>
            }
          />
        </>
      ),
    })
    expect(await screen.findByTestId("location-form-dialog")).toBeInTheDocument()
  })

  it("has no axe violations once data has loaded", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Main House" })]),
      ...areaHandlers.list(SLUG, [])
    )
    const { container } = renderList()
    await waitFor(() => expect(screen.getAllByTestId("location-card")).toHaveLength(1))
    expect(await axe(container)).toHaveNoViolations()
  })
})
