import { http, HttpResponse } from "msw"
import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { AreaDetailPage } from "@/pages/areas/AreaDetailPage"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import {
  apiUrl,
  areaHandlers,
  commodityHandlers,
  fileHandlers,
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
      }),
      ...fileHandlers.list(SLUG, [])
    )
    renderDetail(`/g/${SLUG}/areas/a1`)
    await waitFor(() =>
      expect(screen.getByRole("heading", { name: "Kitchen" })).toBeInTheDocument()
    )
    // Parent-location resolves on a chained fetch (area → location_id
    // → /locations/:id), so wait again for the breadcrumb to fill in.
    const crumbs = await screen.findByTestId("area-detail-breadcrumb")
    await waitFor(() => {
      expect(within(crumbs).getByTestId("breadcrumb-location")).toHaveTextContent("Main House")
    })
    expect(within(crumbs).getByTestId("breadcrumb-locations")).toHaveAttribute(
      "href",
      `/g/${SLUG}/locations`
    )
    expect(within(crumbs).getByTestId("breadcrumb-location")).toHaveAttribute(
      "href",
      `/g/${SLUG}/locations/loc1`
    )
    expect(within(crumbs).getByTestId("breadcrumb-current")).toHaveTextContent("Kitchen")
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
      ...commodityHandlers.values(SLUG, { areaTotals: [] }),
      ...fileHandlers.list(SLUG, [])
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
      }),
      ...fileHandlers.list(SLUG, [])
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
      ...commodityHandlers.values(SLUG, { areaTotals: [] }),
      ...fileHandlers.list(SLUG, [])
    )
    renderDetail(`/g/${SLUG}/areas/a1`)
    expect(await screen.findByTestId("area-detail-items-empty")).toBeInTheDocument()
    expect(screen.queryByTestId("area-detail-items-list")).not.toBeInTheDocument()
  })

  it("renders the toolbar (search + filters + sort + view-mode toggles) and the active-warranties stat", async () => {
    const items = [
      commodityResource("c1", {
        name: "Espresso",
        area_id: "a1",
        status: "in_use",
        type: "white_goods",
        current_price: 100,
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
      http.get(apiUrl(`/g/${SLUG}/commodities`), () => HttpResponse.json(commodityListBody(items))),
      ...commodityHandlers.values(SLUG, {
        areaTotals: [{ id: "a1", name: "Kitchen", value: 100 }],
      }),
      ...fileHandlers.list(SLUG, [])
    )
    renderDetail(`/g/${SLUG}/areas/a1`)
    // Toolbar surfaces — search, the three filter dropdowns, sort, the
    // two view-mode buttons. The full keyboard contract is exercised
    // through the underlying primitives' own tests; here we just assert
    // composition.
    await screen.findByTestId("area-detail-items-toolbar")
    expect(screen.getByTestId("area-detail-items-search")).toBeInTheDocument()
    expect(screen.getByTestId("area-detail-items-filter-type")).toBeInTheDocument()
    expect(screen.getByTestId("area-detail-items-filter-status")).toBeInTheDocument()
    expect(screen.getByTestId("area-detail-items-filter-warranty")).toBeInTheDocument()
    expect(screen.getByTestId("area-detail-items-sort")).toBeInTheDocument()
    expect(screen.getByTestId("area-detail-items-view-grid")).toBeInTheDocument()
    expect(screen.getByTestId("area-detail-items-view-list")).toBeInTheDocument()
    // Active-warranties stat — third stat cell, mock-parity Level 3.
    expect(screen.getByTestId("area-detail-items-active-warranties")).toBeInTheDocument()
  })

  it("typing in the toolbar search debounces a refetch with the q= filter applied", async () => {
    const commodityRequestQueries: string[] = []
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.detail(
        SLUG,
        "a1",
        areaResource("a1", { name: "Kitchen", location_id: "loc1" })
      ),
      ...locationHandlers.detail(SLUG, "loc1", locationResource("loc1", { name: "Main House" })),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Main House" })]),
      // Capture every commodities request so we can assert the debounced
      // search round-tripped into the BE query. MemoryRouter doesn't
      // mutate window.location, so the URL itself isn't observable — the
      // refetch IS, and that's what users actually feel.
      http.get(apiUrl(`/g/${SLUG}/commodities`), ({ request }) => {
        commodityRequestQueries.push(new URL(request.url).search)
        return HttpResponse.json(commodityListBody([]))
      }),
      ...commodityHandlers.values(SLUG, { areaTotals: [] }),
      ...fileHandlers.list(SLUG, [])
    )
    renderDetail(`/g/${SLUG}/areas/a1`)
    const search = await screen.findByTestId("area-detail-items-search")
    const user = userEvent.setup()
    await user.type(search, "drill")
    await waitFor(
      () => {
        expect(commodityRequestQueries.some((q) => /[?&]q=drill\b/.test(q))).toBe(true)
      },
      { timeout: 2000 }
    )
  })

  it("renders the Area Files panel under the items section (linked_entity_type=area)", async () => {
    let filesUrl: URL | undefined
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.detail(
        SLUG,
        "a1",
        areaResource("a1", { name: "Kitchen", location_id: "loc1" })
      ),
      ...locationHandlers.detail(SLUG, "loc1", locationResource("loc1", { name: "Main House" })),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Main House" })]),
      ...commodityHandlers.list(SLUG, []),
      ...commodityHandlers.values(SLUG, { areaTotals: [] }),
      // Capture the query string so we can verify the panel scoped its
      // GET to this area via linked_entity_type=area. The handler still
      // returns an empty list so the panel renders its empty state.
      http.get(apiUrl(`/g/${SLUG}/files`), ({ request }) => {
        filesUrl = new URL(request.url)
        return HttpResponse.json({ data: [], meta: { files: 0, total: 0 } })
      })
    )
    renderDetail(`/g/${SLUG}/areas/a1`)
    // The Files panel mounts unconditionally on area-detail once the
    // area resolves. Wait for its empty state then verify the GET that
    // backed it carried the right linkage query params.
    await screen.findByText(/no files attached/i)
    await waitFor(() => {
      expect(filesUrl?.searchParams.get("linked_entity_type")).toBe("area")
      expect(filesUrl?.searchParams.get("linked_entity_id")).toBe("a1")
    })
  })

  it("swaps the items into a grid when the view-grid toggle is clicked", async () => {
    const items = [
      commodityResource("c1", {
        name: "Espresso",
        area_id: "a1",
        status: "in_use",
        type: "white_goods",
        current_price: 100,
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
      http.get(apiUrl(`/g/${SLUG}/commodities`), () => HttpResponse.json(commodityListBody(items))),
      ...commodityHandlers.values(SLUG, {
        areaTotals: [{ id: "a1", name: "Kitchen", value: 125 }],
      }),
      ...fileHandlers.list(SLUG, [])
    )
    renderDetail(`/g/${SLUG}/areas/a1`)
    // Default view is list; rows are rendered, grid is not yet.
    await screen.findByTestId("area-detail-items-list")
    expect(screen.queryByTestId("area-detail-items-grid")).not.toBeInTheDocument()
    // Click the grid toggle; renderer flips. The same backing rows are
    // re-rendered as cards, so the count must match.
    const user = userEvent.setup()
    await user.click(screen.getByTestId("area-detail-items-view-grid"))
    const grid = await screen.findByTestId("area-detail-items-grid")
    expect(within(grid).getAllByTestId("area-detail-items-card")).toHaveLength(2)
    expect(screen.queryByTestId("area-detail-items-list")).not.toBeInTheDocument()
  })

  it("renders pagination + bumps page= on Next when total > the page size", async () => {
    // Server-side pagination: BE reports total=50 (≥ PER_PAGE of 24), so
    // the panel ships 2 pages of controls and the Next button rolls the
    // URL forward, which fires a refetch with page=2.
    const items = Array.from({ length: 24 }).map((_, i) =>
      commodityResource(`c${i}`, {
        name: `Item ${i}`,
        area_id: "a1",
        status: "in_use",
        type: "other",
      })
    )
    const commodityRequestQueries: string[] = []
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.detail(
        SLUG,
        "a1",
        areaResource("a1", { name: "Workshop", location_id: "loc1" })
      ),
      ...locationHandlers.detail(SLUG, "loc1", locationResource("loc1", { name: "Garage" })),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Garage" })]),
      http.get(apiUrl(`/g/${SLUG}/commodities`), ({ request }) => {
        commodityRequestQueries.push(new URL(request.url).search)
        // total > items.length forces the pagination block to render.
        // The FE list-shape reads `meta.commodities` as the total
        // (`features/commodities/api.ts`), so that's the value that
        // matters here.
        return HttpResponse.json({
          data: items,
          meta: { commodities: 50, page: 1, per_page: 24, total_pages: 3 },
        })
      }),
      ...commodityHandlers.values(SLUG, { areaTotals: [] }),
      ...fileHandlers.list(SLUG, [])
    )
    renderDetail(`/g/${SLUG}/areas/a1`)
    await screen.findByTestId("area-detail-items-pagination")
    const user = userEvent.setup()
    await user.click(screen.getByTestId("area-detail-items-pagination-next"))
    await waitFor(
      () => {
        expect(commodityRequestQueries.some((q) => /[?&]page=2\b/.test(q))).toBe(true)
      },
      { timeout: 2000 }
    )
  })

  it("ignores an unknown ?sort= field and falls back to name ascending (no surprise -name desc)", async () => {
    // Regression guard for the sort fallback: `?sort=-bogus` used to
    // leak `desc=true` into the request because the panel split the
    // raw string before validating the field. The fallback now also
    // resets the direction so the user never sees a surprise Z→A sort.
    const commodityRequestQueries: string[] = []
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.detail(
        SLUG,
        "a1",
        areaResource("a1", { name: "Kitchen", location_id: "loc1" })
      ),
      ...locationHandlers.detail(SLUG, "loc1", locationResource("loc1", { name: "Main House" })),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Main House" })]),
      http.get(apiUrl(`/g/${SLUG}/commodities`), ({ request }) => {
        commodityRequestQueries.push(new URL(request.url).search)
        return HttpResponse.json(commodityListBody([]))
      }),
      ...commodityHandlers.values(SLUG, { areaTotals: [] }),
      ...fileHandlers.list(SLUG, [])
    )
    renderDetail(`/g/${SLUG}/areas/a1?sort=-bogus`)
    await waitFor(() => {
      expect(commodityRequestQueries.length).toBeGreaterThan(0)
    })
    // The request must carry sort=name (ascending — the `-` prefix
    // emits when sortDesc is true). It must NOT carry sort=-name.
    const last = commodityRequestQueries[commodityRequestQueries.length - 1]!
    expect(last).toMatch(/[?&]sort=name\b/)
    expect(last).not.toMatch(/[?&]sort=-/)
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
