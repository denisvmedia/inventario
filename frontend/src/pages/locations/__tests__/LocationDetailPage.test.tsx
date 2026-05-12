import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { LocationDetailPage } from "@/pages/locations/LocationDetailPage"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import {
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
  warranty_expires_at?: string
}

function commodityResource(id: string, attrs: CommodityAttrs) {
  return { id, type: "commodities", attributes: { ...attrs, id } }
}

function renderDetail(initialPath: string, props: { initialMode?: "edit" } = {}) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path="/g/:groupSlug/locations/:id"
        element={
          <ConfirmProvider>
            <GroupProvider>
              <LocationDetailPage {...props} />
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

describe("<LocationDetailPage />", () => {
  it("renders the breadcrumb + metadata + area tile grid with per-area item counts", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.detail(
        SLUG,
        "loc1",
        locationResource("loc1", {
          name: "Main House",
          address: "12 Elm St",
        })
      ),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Main House" })]),
      ...areaHandlers.list(SLUG, [
        areaResource("a1", { name: "Kitchen", location_id: "loc1" }),
        areaResource("a2", { name: "Workshop", location_id: "loc1" }),
        areaResource("a3", { name: "Other-Group", location_id: "loc-other" }),
      ]),
      ...commodityHandlers.list(SLUG, [
        commodityResource("c1", { name: "Espresso", area_id: "a1", status: "in_use" }),
        commodityResource("c2", { name: "Toaster", area_id: "a1", status: "in_use" }),
        commodityResource("c3", { name: "Drill", area_id: "a2", status: "in_use" }),
      ])
    )
    renderDetail(`/g/${SLUG}/locations/loc1`)
    await waitFor(() =>
      expect(screen.getByRole("heading", { name: "Main House" })).toBeInTheDocument()
    )
    expect(screen.getByText("12 Elm St")).toBeInTheDocument()
    // Breadcrumb shows Locations / Main House.
    const crumbs = screen.getByTestId("location-detail-breadcrumb")
    expect(within(crumbs).getByTestId("breadcrumb-locations")).toHaveAttribute(
      "href",
      `/g/${SLUG}/locations`
    )
    expect(within(crumbs).getByTestId("breadcrumb-current")).toHaveTextContent("Main House")
    // Only the two areas whose location_id === loc1 are surfaced.
    const tiles = await screen.findAllByTestId("location-detail-area")
    expect(tiles).toHaveLength(2)
    const kitchen = tiles.find((t) => within(t).queryByText("Kitchen"))!
    expect(within(kitchen).getByText("2 items")).toBeInTheDocument()
    const workshop = tiles.find((t) => within(t).queryByText("Workshop"))!
    expect(within(workshop).getByText("1 item")).toBeInTheDocument()
  })

  it("surfaces the warranty-expiring pill when an area carries expiring items", async () => {
    const soon = new Date()
    soon.setDate(soon.getDate() + 30)
    const expiresAt = soon.toISOString().slice(0, 10)
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.detail(SLUG, "loc1", locationResource("loc1", { name: "Main House" })),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Main House" })]),
      ...areaHandlers.list(SLUG, [areaResource("a1", { name: "Kitchen", location_id: "loc1" })]),
      ...commodityHandlers.list(SLUG, [
        commodityResource("c1", {
          name: "Espresso",
          area_id: "a1",
          status: "in_use",
          warranty_expires_at: expiresAt,
        }),
        commodityResource("c2", { name: "Toaster", area_id: "a1", status: "in_use" }),
      ])
    )
    renderDetail(`/g/${SLUG}/locations/loc1`)
    const tile = await screen.findByTestId("location-detail-area")
    expect(within(tile).getByTestId("location-detail-area-expiring")).toHaveTextContent(
      "1 warranty expiring"
    )
  })

  it("auto-opens the edit dialog when initialMode='edit'", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.detail(SLUG, "loc1", locationResource("loc1", { name: "Garage" })),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Garage" })]),
      ...areaHandlers.list(SLUG, []),
      ...commodityHandlers.list(SLUG, [])
    )
    renderDetail(`/g/${SLUG}/locations/loc1`, { initialMode: "edit" })
    expect(await screen.findByTestId("location-form-dialog")).toBeInTheDocument()
  })

  it("renders the empty-areas card when the location has no areas", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.detail(SLUG, "loc2", locationResource("loc2", { name: "Cottage" })),
      ...locationHandlers.list(SLUG, [locationResource("loc2", { name: "Cottage" })]),
      ...areaHandlers.list(SLUG, []),
      ...commodityHandlers.list(SLUG, [])
    )
    renderDetail(`/g/${SLUG}/locations/loc2`)
    await waitFor(() =>
      expect(screen.getByRole("heading", { name: "Cottage" })).toBeInTheDocument()
    )
    expect(screen.getByTestId("location-detail-areas-empty")).toBeInTheDocument()
  })

  it("opens the upload dialog with the location name in the title from the EntityFilesPanel Attach button (#1448)", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.detail(SLUG, "loc1", locationResource("loc1", { name: "Garage" })),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Garage" })]),
      ...areaHandlers.list(SLUG, []),
      ...fileHandlers.list(SLUG, []),
      ...commodityHandlers.list(SLUG, [])
    )
    renderDetail(`/g/${SLUG}/locations/loc1`)
    const attach = await screen.findByTestId("entity-files-panel-attach")
    await user.click(attach)
    expect(
      await screen.findByRole("heading", { name: /attach files to garage/i })
    ).toBeInTheDocument()
  })

  it("shows the drop overlay while files are dragged over the location detail page (#1448)", async () => {
    const { fireEvent } = await import("@testing-library/react")
    server.use(
      ...groupHandlers.list(groupFixture),
      ...locationHandlers.detail(SLUG, "loc1", locationResource("loc1", { name: "Garage" })),
      ...locationHandlers.list(SLUG, [locationResource("loc1", { name: "Garage" })]),
      ...areaHandlers.list(SLUG, []),
      // EntityFilesPanel mounts unconditionally on the location
      // detail page once location.data resolves. Without this stub
      // its background GET /files would hit MSW's onUnhandledRequest:
      // "error" hook (set in src/test/setup.ts) and surface as a
      // post-test warning even if the assertions complete first.
      ...fileHandlers.list(SLUG, []),
      ...commodityHandlers.list(SLUG, [])
    )
    renderDetail(`/g/${SLUG}/locations/loc1`)
    const page = await screen.findByTestId("page-location-detail")
    const init = {
      bubbles: true,
      cancelable: true,
      // @ts-expect-error partial init is intentional
      dataTransfer: { types: ["Files"], files: [], dropEffect: "none" },
    }
    fireEvent.dragEnter(page, init)
    expect(await screen.findByTestId("entity-drop-overlay")).toBeInTheDocument()
    fireEvent.dragLeave(page, init)
    await waitFor(() => expect(screen.queryByTestId("entity-drop-overlay")).not.toBeInTheDocument())
  })
})
