import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { LocationsListPage } from "@/pages/locations/LocationsListPage"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { areaHandlers, commodityHandlers, groupHandlers, locationHandlers } from "@/test/handlers"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

function locationResource(
  id: string,
  attrs: { name: string; address?: string; icon?: string; description?: string }
) {
  return { id, type: "locations", attributes: { ...attrs, id } }
}

function areaResource(id: string, attrs: { name: string; location_id: string; icon?: string }) {
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
      ...areaHandlers.list(SLUG, []),
      ...commodityHandlers.list(SLUG, [])
    )
    renderList()
    await waitFor(() => {
      expect(screen.queryByTestId("locations-empty")).toBeInTheDocument()
    })
  })

  it("renders each location as a click-through tile with stat chips", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.list(SLUG, [
        locationResource("loc1", { name: "Main House", address: "12 Elm St" }),
        locationResource("loc2", { name: "Garage" }),
      ]),
      ...areaHandlers.list(SLUG, [
        areaResource("a1", { name: "Kitchen", location_id: "loc1" }),
        areaResource("a2", { name: "Workshop", location_id: "loc2" }),
        areaResource("a3", { name: "Pantry", location_id: "loc1" }),
      ]),
      // Two items in loc1 (a1 + a3 collectively → 2), one in loc2 (a2).
      ...commodityHandlers.list(SLUG, [
        {
          id: "c1",
          type: "commodities",
          attributes: {
            id: "c1",
            name: "Espresso",
            area_id: "a1",
            status: "in_use",
            type: "white_goods",
          },
        },
        {
          id: "c2",
          type: "commodities",
          attributes: {
            id: "c2",
            name: "Mixer",
            area_id: "a3",
            status: "in_use",
            type: "white_goods",
          },
        },
        {
          id: "c3",
          type: "commodities",
          attributes: {
            id: "c3",
            name: "Drill",
            area_id: "a2",
            status: "in_use",
            type: "white_goods",
          },
        },
      ])
    )
    renderList()
    const cards = await screen.findAllByTestId("location-card")
    expect(cards).toHaveLength(2)
    const main = cards.find((c) => within(c).queryByText("Main House"))!
    expect(main).toBeDefined()
    expect(within(main).getByText("12 Elm St")).toBeInTheDocument()
    // Each card is wrapped by an absolute-positioned <Link> with the
    // location name as aria-label; the chevron + dropdown menu live on
    // top of it.
    expect(within(main).getByTestId("location-card-link")).toHaveAttribute(
      "href",
      `/g/${SLUG}/locations/loc1`
    )
    // Stat chips render via i18next plural keys.
    const mainAreas = within(main).getByTestId("location-card-stat-areas")
    expect(mainAreas).toHaveTextContent("2 areas")
    const mainItems = within(main).getByTestId("location-card-stat-items")
    expect(mainItems).toHaveTextContent("2 items")

    const garage = cards.find((c) => within(c).queryByText("Garage"))!
    expect(within(garage).getByTestId("location-card-stat-areas")).toHaveTextContent("1 area")
    expect(within(garage).getByTestId("location-card-stat-items")).toHaveTextContent("1 item")
  })

  it("renders the location's emoji avatar + description when set, preferring description over address", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.list(SLUG, [
        locationResource("loc1", {
          name: "Main House",
          address: "12 Elm St",
          icon: "🏡",
          description: "Primary residence",
        }),
        locationResource("loc2", { name: "Garage", address: "Out back" }),
      ]),
      ...areaHandlers.list(SLUG, []),
      ...commodityHandlers.list(SLUG, [])
    )
    renderList()
    const cards = await screen.findAllByTestId("location-card")
    const main = cards.find((c) => within(c).queryByText("Main House"))!
    // Description wins over address in the subtitle slot when both
    // exist (the mock's muted one-liner; address only surfaces on the
    // detail page).
    expect(within(main).getByTestId("location-card-description")).toHaveTextContent(
      "Primary residence"
    )
    expect(within(main).queryByText("12 Elm St")).toBeNull()
    expect(within(main).getByTestId("location-card-icon")).toHaveTextContent("🏡")

    // Location without `icon` / `description` falls back to the address
    // for the subtitle and the generic glyph for the avatar.
    const garage = cards.find((c) => within(c).queryByText("Garage"))!
    expect(within(garage).queryByTestId("location-card-description")).toBeNull()
    expect(within(garage).getByText("Out back")).toBeInTheDocument()
  })

  it("filters by query against location and area names", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.list(SLUG, [
        locationResource("loc1", { name: "Main House" }),
        locationResource("loc2", { name: "Garage" }),
      ]),
      ...areaHandlers.list(SLUG, [areaResource("a1", { name: "Kitchen", location_id: "loc1" })]),
      ...commodityHandlers.list(SLUG, [])
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
      ...areaHandlers.list(SLUG, []),
      ...commodityHandlers.list(SLUG, [])
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
      ...areaHandlers.list(SLUG, []),
      ...commodityHandlers.list(SLUG, [])
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

  it("opens the AreaFormDialog from the LocationCard dropdown's Add area item", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Main House" })]),
      ...areaHandlers.list(SLUG, []),
      ...commodityHandlers.list(SLUG, [])
    )
    renderList()
    const card = await screen.findByTestId("location-card")
    const user = userEvent.setup()
    await user.click(within(card).getByTestId("location-card-menu"))
    await user.click(await screen.findByTestId("location-card-add-area"))
    expect(await screen.findByTestId("area-form-dialog")).toBeInTheDocument()
  })

  it("has no axe violations once data has loaded", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Main House" })]),
      ...areaHandlers.list(SLUG, []),
      ...commodityHandlers.list(SLUG, [])
    )
    const { container } = renderList()
    await waitFor(() => expect(screen.getAllByTestId("location-card")).toHaveLength(1))
    expect(await axe(container)).toHaveNoViolations()
  })
})
