import { http, HttpResponse } from "msw"
import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"

import { AreaDetailPage } from "@/pages/areas/AreaDetailPage"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import {
  apiUrl,
  areaHandlers,
  commodityHandlers,
  groupHandlers,
  locationHandlers,
} from "@/test/handlers"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

function locationResource(id: string, attrs: { name: string; address?: string }) {
  return { id, type: "locations", attributes: { ...attrs, id } }
}

function areaResource(id: string, attrs: { name: string; location_id: string }) {
  return { id, type: "areas", attributes: { ...attrs, id } }
}

interface CommodityAttrs {
  name: string
  area_id: string
  status?: string
  type?: string
  short_name?: string
  current_price?: number
  draft?: boolean
}

function commodityResource(id: string, attrs: CommodityAttrs) {
  return { id, type: "commodities", attributes: { ...attrs, id } }
}

function commodityListBody(items: ReturnType<typeof commodityResource>[]) {
  return { data: items, meta: { commodities: items.length } }
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
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Main House" })]),
      ...commodityHandlers.list(SLUG, []),
      ...commodityHandlers.values(SLUG, {
        areaTotals: [{ name: "Kitchen", total: 0 }],
      })
    )
    renderDetail(`/g/${SLUG}/areas/a1`)
    await waitFor(() => expect(screen.getByText("Kitchen")).toBeInTheDocument())
    // Parent-location resolves on a chained fetch (area → location_id
    // → /locations/:id), so wait again for the breadcrumb.
    await waitFor(() => {
      expect(screen.getAllByText("Main House").length).toBeGreaterThan(0)
    })
    // Stats strip + empty-state list.
    expect(await screen.findByTestId("area-detail-items-stats")).toBeInTheDocument()
    expect(screen.getByTestId("area-detail-items-empty")).toBeInTheDocument()
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
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Garage" })]),
      ...commodityHandlers.list(SLUG, []),
      ...commodityHandlers.values(SLUG, { areaTotals: [] })
    )
    renderDetail(`/g/${SLUG}/areas/a1`, { initialMode: "edit" })
    expect(await screen.findByTestId("area-form-dialog")).toBeInTheDocument()
  })

  it("renders the per-area items list with stats, rows, and detail links", async () => {
    // The page calls useCommodities with includeInactive: false, so the
    // BE would filter to in_use rows; the values endpoint sums only
    // non-draft in_use commodities. Both fixtures stay in_use so the
    // numbers line up with real behaviour.
    const items = [
      commodityResource("c1", {
        name: "Espresso machine",
        area_id: "a1",
        status: "in_use",
        type: "white_goods",
        short_name: "Coffee bar",
        current_price: 350,
      }),
      commodityResource("c2", {
        name: "Toaster",
        area_id: "a1",
        status: "in_use",
        type: "white_goods",
        current_price: 25,
      }),
    ]
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.detail(
        SLUG,
        "a1",
        areaResource("a1", { name: "Kitchen", location_id: "loc1" })
      ),
      ...locationHandlers.detail(SLUG, "loc1", locationResource("loc1", { name: "Main House" })),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Main House" })]),
      // commodityHandlers.list reports total=data.length when no meta is
      // passed; the "Items" stat reads from total, so we want it to match.
      http.get(apiUrl(`/g/${SLUG}/commodities`), () => HttpResponse.json(commodityListBody(items))),
      // Match by id (BE NamedTotal shape after #1632 deserializer fix).
      ...commodityHandlers.values(SLUG, {
        areaTotals: [{ id: "a1", name: "Kitchen", value: 375 }],
      })
    )
    renderDetail(`/g/${SLUG}/areas/a1`)

    const list = await screen.findByTestId("area-detail-items-list")
    const rows = within(list).getAllByTestId("area-detail-items-row")
    expect(rows).toHaveLength(2)
    expect(rows[0]).toHaveAttribute("href", `/g/${SLUG}/commodities/c1`)
    expect(within(rows[0]!).getByText("Espresso machine")).toBeInTheDocument()
    expect(within(rows[0]!).getByText("Coffee bar")).toBeInTheDocument()
    // Stats: count = 2 in_use, total = $375.00.
    const stats = screen.getByTestId("area-detail-items-stats")
    expect(within(stats).getByText("2")).toBeInTheDocument()
    expect(within(stats).getByText("$375.00")).toBeInTheDocument()
  })

  it("shows the empty state when the area has no items", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.detail(
        SLUG,
        "a1",
        areaResource("a1", { name: "Attic", location_id: "loc1" })
      ),
      ...locationHandlers.detail(SLUG, "loc1", locationResource("loc1", { name: "Main House" })),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Main House" })]),
      ...commodityHandlers.list(SLUG, []),
      ...commodityHandlers.values(SLUG, { areaTotals: [] })
    )
    renderDetail(`/g/${SLUG}/areas/a1`)
    expect(await screen.findByTestId("area-detail-items-empty")).toBeInTheDocument()
    expect(screen.queryByTestId("area-detail-items-list")).not.toBeInTheDocument()
  })

  it("falls through to the area-error Alert when the area request fails", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      http.get(apiUrl(`/g/${SLUG}/areas/a1`), () =>
        HttpResponse.json({ error: "boom" }, { status: 500 })
      )
    )
    renderDetail(`/g/${SLUG}/areas/a1`)
    expect(await screen.findByTestId("area-detail-error")).toBeInTheDocument()
  })
})
