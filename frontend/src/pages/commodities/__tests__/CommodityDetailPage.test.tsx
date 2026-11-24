import { beforeEach, describe, expect, it } from "vitest"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { http, HttpResponse } from "msw"

import { CommodityDetailPage, CommodityDetailSheet } from "@/pages/commodities/CommodityDetailPage"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { capture } from "@/test/capture"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import {
  apiUrl,
  areaHandlers,
  commodityHandlers,
  fileHandlers,
  groupHandlers,
  locationHandlers,
  tagHandlers,
} from "@/test/handlers"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

const SLUG = "household"
const ID = "c1"

const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

const areaFixture = [
  { id: "a1", type: "areas", attributes: { id: "a1", name: "Garage", location_id: "l1" } },
]

const commodityFixture = {
  id: ID,
  type: "commodities",
  attributes: {
    id: ID,
    name: "MacBook Pro 16",
    short_name: "Laptop",
    type: "electronics",
    area_id: "a1",
    status: "in_use",
    count: 1,
    original_price: 2400,
    current_price: 1900,
    original_price_currency: "USD",
    serial_number: "ABC-123",
    tags: ["work", "tech"],
    purchase_date: "2024-09-12",
    draft: false,
  },
}

function renderDetail(initialPath = `/g/${SLUG}/commodities/${ID}`) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path="/g/:groupSlug/commodities/:id"
        element={
          <GroupProvider>
            <ConfirmProvider>
              <CommodityDetailPage />
            </ConfirmProvider>
          </GroupProvider>
        }
      />
    ),
  })
}

// Both item routes, as app/router.tsx mounts them, plus a probe that shows
// where navigation landed.
function LocationProbe() {
  const location = useLocation()
  return <span data-testid="location-probe">{`${location.pathname}${location.search}`}</span>
}

function renderItemRoutes(initialPath: string) {
  setAccessToken("good-token")
  const element = (
    <GroupProvider>
      <ConfirmProvider>
        <CommodityDetailPage />
        <LocationProbe />
      </ConfirmProvider>
    </GroupProvider>
  )
  return renderWithProviders({
    initialPath,
    routes: (
      <>
        <Route path="/g/:groupSlug/commodities/:id" element={element} />
        <Route path="/g/:groupSlug/commodities/:id/edit" element={element} />
      </>
    ),
  })
}

type Attrs = Record<string, unknown>

function itemRow(id: string, name: string): { id: string; type: string; attributes: Attrs } {
  return {
    id,
    type: "commodities",
    attributes: { ...commodityFixture.attributes, id, name },
  }
}

// Items behind list, detail and update, so a save shows up in what the page
// reads next. Returns the PUT bodies in order.
function serveItems(rows: ReturnType<typeof itemRow>[]) {
  const store = new Map(rows.map((r) => [r.id, { ...r, attributes: { ...r.attributes } }]))
  const puts: Array<{ id: string; attributes: Attrs }> = []
  server.use(
    ...groupHandlers.list(groupFixture),
    ...areaHandlers.list(SLUG, areaFixture),
    ...locationHandlers.list(SLUG, [
      { id: "l1", type: "locations", attributes: { id: "l1", name: "Home" } },
    ]),
    ...tagHandlers.list(SLUG, [
      { id: "t1", slug: "work", label: "Work", color: "blue" },
      { id: "t2", slug: "tech", label: "Tech", color: "green" },
      { id: "t3", slug: "home", label: "Home", color: "amber" },
    ]),
    http.get(apiUrl(`/g/${SLUG}/commodities`), () =>
      HttpResponse.json({ data: [...store.values()], meta: { commodities: store.size } })
    ),
    http.get(apiUrl(`/g/${SLUG}/commodities/:id`), ({ params }) => {
      const row = store.get(params.id as string)
      if (!row) return HttpResponse.json({ errors: [{ status: "404" }] }, { status: 404 })
      return HttpResponse.json({ data: row })
    }),
    http.put(apiUrl(`/g/${SLUG}/commodities/:id`), async ({ params, request }) => {
      const id = params.id as string
      const body = (await request.json()) as { data: { attributes: Attrs } }
      puts.push({ id, attributes: body.data.attributes })
      const row = { id, type: "commodities", attributes: { ...body.data.attributes, id } }
      store.set(id, row)
      return HttpResponse.json({ data: row })
    })
  )
  return puts
}

const LIST = `/g/${SLUG}/commodities?q=mac`
const FROM = `from=${encodeURIComponent(LIST)}`

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<CommodityDetailPage />", () => {
  it("renders the commodity name + key fields", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderDetail()
    await waitFor(() => expect(screen.getByTestId("page-commodity-detail")).toBeInTheDocument())
    expect(screen.getByRole("heading", { name: /macbook pro 16/i })).toBeInTheDocument()
    expect(screen.getByTestId("commodity-detail-short-name")).toHaveTextContent("Laptop")
    expect(screen.getByText(/garage/i)).toBeInTheDocument()
    // Currency-formatted prices show up in the Details tab.
    expect(screen.getByText(/\$1,900\.00/)).toBeInTheDocument()
    // No acquisition-frozen line on a fresh commodity (the live
    // OriginalPrice already represents the purchase amount).
    expect(screen.queryByTestId("commodity-detail-acquisition")).toBeNull()
  })

  it("shows the asset ID and links to its label (#2807)", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, {
        ...commodityFixture,
        attributes: { ...commodityFixture.attributes, asset_number: 42 },
      })
    )
    renderDetail()
    expect(await screen.findByTestId("commodity-detail-asset-id")).toHaveTextContent("A-0042")
    expect(screen.getByTestId("commodity-detail-print-label")).toHaveAttribute(
      "href",
      `/g/${SLUG}/labels?item=${ID}`
    )
  })

  it("hides the asset ID until the item has one", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderDetail()
    await screen.findByTestId("commodity-detail-name")
    expect(screen.queryByTestId("commodity-detail-asset-id")).toBeNull()
  })

  it("shows tag labels instead of slugs", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...tagHandlers.list(SLUG, [{ id: "t1", slug: "work", label: "Работа", color: "blue" }]),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderDetail()
    expect(await screen.findByText("Работа")).toBeInTheDocument()
    // "tech" has no tag row, so the slug is the best name available.
    expect(screen.getByText("tech")).toBeInTheDocument()
    expect(screen.queryByText("work")).toBeNull()
  })

  it("renders the acquisition-frozen line after a currency migration", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, {
        ...commodityFixture,
        attributes: {
          ...commodityFixture.attributes,
          // Group has been migrated USD → EUR; the BE froze the
          // pre-migration purchase amount in `acquisition_*`.
          acquisition_price: 2400,
          acquisition_currency: "USD",
          original_price_currency: "EUR",
          original_price: 2160,
        },
      })
    )
    renderDetail()
    const acquisition = await screen.findByTestId("commodity-detail-acquisition")
    expect(acquisition).toHaveTextContent(/Originally purchased for/i)
    expect(acquisition).toHaveTextContent(/2,400/)
  })

  it("hides the acquisition line when only one of price / currency is present", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, {
        ...commodityFixture,
        attributes: {
          ...commodityFixture.attributes,
          acquisition_price: 2400,
          // No acquisition_currency — guard against partially-frozen
          // rows from a buggy migration.
        },
      })
    )
    renderDetail()
    await waitFor(() => expect(screen.getByTestId("page-commodity-detail")).toBeInTheDocument())
    expect(screen.queryByTestId("commodity-detail-acquisition")).toBeNull()
  })

  it("renders an error alert when the detail endpoint 5xx", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture)
      // Hijack the same path with a 500 — mirrors the legacy error
      // handler factory pattern. We do this inline since
      // commodityHandlers.error covers list, not detail.
    )
    server.use(...commodityHandlers.detail(SLUG, ID, { errors: [{ status: "500" }] }))
    // The fixture above returns 200 with an unexpected envelope; force
    // a network error instead by overriding with a 500-response handler.
    const { http, HttpResponse } = await import("msw")
    server.use(
      http.get(`${window.location.origin}/api/v1/g/${SLUG}/commodities/${ID}`, () =>
        HttpResponse.json({ error: "boom" }, { status: 500 })
      )
    )
    renderDetail()
    expect(await screen.findByTestId("commodity-detail-error")).toBeInTheDocument()
  })

  it("switches between Details / Warranty / Files tabs", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderDetail()
    await screen.findByTestId("commodity-detail-details")
    await user.click(screen.getByTestId("commodity-detail-tab-warranty"))
    expect(await screen.findByTestId("commodity-detail-warranty")).toBeInTheDocument()
    await user.click(screen.getByTestId("commodity-detail-tab-files"))
    expect(await screen.findByTestId("commodity-detail-files")).toBeInTheDocument()
  })

  it("opens the upload dialog directly from the Warranty tab's Upload Receipt CTA (#2012)", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture),
      ...fileHandlers.list(SLUG, []),
      ...fileHandlers.counts(SLUG, {})
    )
    renderDetail(`/g/${SLUG}/commodities/${ID}?tab=warranty`)
    const uploadReceipt = await screen.findByTestId("commodity-detail-warranty-upload-receipt")
    // Regression guard: before #2012 this button only switched to the
    // Files tab and the user had to hunt for the dropzone. The dialog
    // must not be open until the CTA is clicked.
    expect(screen.queryByTestId("files-upload-dialog")).toBeNull()
    await user.click(uploadReceipt)
    // The CTA now opens the page-level UploadFilesDialog directly — the
    // same dialog the Files-tab dropzone opens — with this commodity
    // preselected (the title carries the commodity name).
    const dialog = await screen.findByTestId("files-upload-dialog")
    expect(dialog).toBeInTheDocument()
    expect(dialog).toHaveTextContent(/MacBook Pro 16/)
    // …and it switches the underlying tab to Files (#2012) so the newly
    // uploaded receipt is visible once the dialog closes — the Files tab
    // content mounts behind the modal.
    expect(await screen.findByTestId("commodity-detail-files")).toBeInTheDocument()
  })

  it("preselects the Warranty tab when the URL has ?tab=warranty (#1529)", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderDetail(`/g/${SLUG}/commodities/${ID}?tab=warranty`)
    // Warranty tab should already be active without any user click —
    // the warranty list / dashboard expiring panel both deep-link in
    // through this query string.
    expect(await screen.findByTestId("commodity-detail-warranty")).toBeInTheDocument()
  })

  it("opens the edit form in place when the Edit button is clicked (#2795)", async () => {
    const user = userEvent.setup()
    serveItems([itemRow(ID, "MacBook Pro 16")])
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}`)
    await user.click(await screen.findByTestId("commodity-detail-edit"))
    expect(await screen.findByTestId("commodity-edit-form")).toBeInTheDocument()
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      `/g/${SLUG}/commodities/${ID}/edit`
    )
    // In place of the read view, not in a dialog.
    expect(screen.queryByTestId("commodity-detail-tabs")).toBeNull()
    expect(screen.queryByLabelText(/form steps/i)).toBeNull()
    expect(screen.getByLabelText(/^name/i)).toHaveValue("MacBook Pro 16")
    // Cancel steps back to the read view it came from.
    await user.click(screen.getByTestId("commodity-edit-cancel"))
    expect(await screen.findByTestId("commodity-detail-tabs")).toBeInTheDocument()
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      new RegExp(`/commodities/${ID}$`)
    )
  })

  it("saves the in-place form and returns to the read view (#2795)", async () => {
    const user = userEvent.setup()
    const puts = serveItems([itemRow(ID, "MacBook Pro 16")])
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}/edit`)
    const name = await screen.findByLabelText(/^name/i)
    await user.clear(name)
    await user.type(name, "MacBook Air")
    await user.click(screen.getByTestId("commodity-edit-save"))
    await waitFor(() => expect(puts).toHaveLength(1))
    expect(puts[0].attributes.name).toBe("MacBook Air")
    // What the form does not show goes back unchanged.
    expect(puts[0].attributes.status).toBe("in_use")
    expect(puts[0].attributes.tags).toEqual(["work", "tech"])
    await waitFor(() =>
      expect(screen.getByTestId("location-probe")).toHaveTextContent(
        new RegExp(`/commodities/${ID}$`)
      )
    )
    expect(await screen.findByTestId("commodity-detail-tabs")).toBeInTheDocument()
  })

  it("toggles a tag from the edit form's palette (#2795)", async () => {
    const user = userEvent.setup()
    const puts = serveItems([itemRow(ID, "MacBook Pro 16")])
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}/edit`)
    const work = await screen.findByTestId("commodity-edit-tags-option-work")
    expect(work).toHaveAttribute("aria-pressed", "true")
    await user.click(work)
    await user.click(screen.getByTestId("commodity-edit-tags-option-home"))
    await user.click(screen.getByTestId("commodity-edit-save"))
    await waitFor(() => expect(puts).toHaveLength(1))
    expect(puts[0].attributes.tags).toEqual(["tech", "home"])
  })

  it("opens every field in the step dialog with the values entered so far (#2795)", async () => {
    const user = userEvent.setup()
    serveItems([itemRow(ID, "MacBook Pro 16")])
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}/edit`)
    const name = await screen.findByLabelText(/^name/i)
    await user.clear(name)
    await user.type(name, "Work laptop")
    await user.click(screen.getByTestId("commodity-edit-all-fields"))
    const dialog = await screen.findByRole("dialog")
    expect(within(dialog).getByLabelText(/form steps/i)).toBeInTheDocument()
    expect(within(dialog).getByRole("textbox", { name: /^name/i })).toHaveValue("Work laptop")
    // Save is on the first step already; edit mode needs no walk to the end.
    expect(within(dialog).getByTestId("commodity-form-submit")).toBeInTheDocument()
  })

  it("deletes the commodity from the detail page", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture),
      ...commodityHandlers.remove(SLUG, ID)
    )
    renderDetail()
    await screen.findByTestId("commodity-detail-delete")
    await user.click(screen.getByTestId("commodity-detail-delete"))
    // Wait for the confirm dialog body-lock, then click its Delete CTA.
    await waitFor(() => expect(document.body).toHaveAttribute("data-scroll-locked"))
    const buttons = screen.getAllByRole("button", { name: /^Delete$/i })
    await user.click(buttons[buttons.length - 1])
    // The page navigates back to the list on success — the detail
    // surface unmounts, so the not-found card should NOT appear (the
    // route is gone). Easiest assertion: header buttons disappear.
    await waitFor(() =>
      expect(screen.queryByTestId("commodity-detail-delete")).not.toBeInTheDocument()
    )
  })

  it("renders the audit timeline with kind-aware copy (#1450)", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture),
      ...commodityHandlers.events(SLUG, ID, [
        {
          id: "ev-status",
          kind: "status_changed",
          occurred_at: "2026-04-25T10:30:00Z",
          before: { status: "in_use" },
          after: { status: "sold" },
          meta: { actor: { id: "u1", name: "Denis", email: "d@example.com" } },
        },
        {
          id: "ev-created",
          kind: "created",
          occurred_at: "2026-04-01T08:00:00Z",
          after: { name: "TV", area_id: "a1", status: "in_use" },
          meta: { actor: { id: "u1", name: "Denis", email: "d@example.com" } },
        },
      ])
    )
    renderDetail()
    expect(await screen.findByTestId("commodity-detail-history")).toBeInTheDocument()
    // Both rows are visible (timeline is below the 10-row collapse threshold).
    expect(await screen.findByTestId("history-row-status_changed")).toHaveTextContent(/Sold/i)
    expect(screen.getByTestId("history-row-created")).toHaveTextContent(/Added this item/i)
    // Actor renders alongside the absolute timestamp.
    expect(screen.getByTestId("history-row-status_changed")).toHaveTextContent(/Denis/)
  })

  it("shows an empty-state when the timeline is empty", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture),
      ...commodityHandlers.events(SLUG, ID, [])
    )
    renderDetail()
    expect(await screen.findByTestId("history-empty")).toHaveTextContent(/No activity yet/i)
  })

  it("shows the edit form when /commodities/:id/edit is the entry URL", async () => {
    serveItems([itemRow(ID, "MacBook Pro 16")])
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}/edit`)
    expect(await screen.findByTestId("commodity-edit-form")).toBeInTheDocument()
    // Opened without a list, there is nothing to walk.
    expect(screen.queryByTestId("commodity-edit-pager")).toBeNull()
    expect(screen.queryByTestId("commodity-edit-save-next")).toBeNull()
  })

  it("walks the list the item was opened from (#2795)", async () => {
    serveItems([itemRow("c0", "Mac mini"), itemRow(ID, "MacBook Pro 16"), itemRow("c2", "iMac")])
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}?${FROM}`)
    await waitFor(() =>
      expect(screen.getByTestId("commodity-detail-position")).toHaveTextContent("2 of 3")
    )
    expect(screen.getByTestId("commodity-detail-previous")).toHaveAttribute(
      "href",
      `/g/${SLUG}/commodities/c0?${FROM}`
    )
    expect(screen.getByTestId("commodity-detail-next")).toHaveAttribute(
      "href",
      `/g/${SLUG}/commodities/c2?${FROM}`
    )
    // Back returns to the list as the user left it.
    expect(screen.getByTestId("commodity-detail-back")).toHaveAttribute("href", LIST)
  })

  it("saves and moves to the next item's edit form (#2795)", async () => {
    const user = userEvent.setup()
    const puts = serveItems([
      itemRow("c0", "Mac mini"),
      itemRow(ID, "MacBook Pro 16"),
      itemRow("c2", "iMac"),
    ])
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}/edit?${FROM}`)
    await waitFor(() =>
      expect(screen.getByTestId("commodity-edit-position")).toHaveTextContent("2 of 3")
    )
    const serial = screen.getByLabelText(/serial number/i)
    await user.clear(serial)
    await user.type(serial, "XYZ-9")
    await user.click(screen.getByTestId("commodity-edit-save-next"))
    await waitFor(() => expect(puts).toHaveLength(1))
    expect(puts[0]).toMatchObject({ id: ID, attributes: { serial_number: "XYZ-9" } })
    await waitFor(() =>
      expect(screen.getByTestId("location-probe")).toHaveTextContent(
        `/g/${SLUG}/commodities/c2/edit?${FROM}`
      )
    )
    expect(await screen.findByDisplayValue("iMac")).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByTestId("commodity-edit-position")).toHaveTextContent("3 of 3")
    )
    expect(screen.getByTestId("commodity-edit-save-next")).toBeDisabled()
  })

  it("adds a tag from the Details tab without opening the edit form (#2795)", async () => {
    const user = userEvent.setup()
    const puts = serveItems([itemRow(ID, "MacBook Pro 16")])
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}`)
    await user.click(await screen.findByTestId("commodity-quick-edit-tags"))
    await user.click(await screen.findByTestId("commodity-quick-tags-palette-option-home"))
    await user.click(screen.getByTestId("commodity-quick-tags-save"))
    await waitFor(() => expect(puts).toHaveLength(1))
    expect(puts[0].attributes.tags).toEqual(["work", "tech", "home"])
    expect(puts[0].attributes.name).toBe("MacBook Pro 16")
    await waitFor(() => expect(screen.queryByTestId("commodity-quick-tags")).toBeNull())
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      new RegExp(`/commodities/${ID}$`)
    )
  })

  it("edits the notes from the Details tab (#2795)", async () => {
    const user = userEvent.setup()
    const puts = serveItems([itemRow(ID, "MacBook Pro 16")])
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}`)
    await user.click(await screen.findByTestId("commodity-quick-edit-notes"))
    const save = screen.getByTestId("commodity-quick-notes-save")
    // Nothing to save until the text changes.
    expect(save).toBeDisabled()
    await user.type(screen.getByTestId("commodity-quick-notes-input"), "Keep the box")
    await user.click(save)
    await waitFor(() => expect(puts).toHaveLength(1))
    expect(puts[0].attributes.comments).toBe("Keep the box")
    expect(await screen.findByText("Keep the box")).toBeInTheDocument()
  })

  it("clears the location from the Details tab (#2795)", async () => {
    const user = userEvent.setup()
    const puts = serveItems([itemRow(ID, "MacBook Pro 16")])
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}`)
    await user.click(await screen.findByTestId("commodity-quick-edit-location"))
    await user.click(await screen.findByTestId("quick-location-clear"))
    await user.click(screen.getByTestId("commodity-quick-location-save"))
    await waitFor(() => expect(puts).toHaveLength(1))
    expect(puts[0].attributes).not.toHaveProperty("area_id")
  })

  it("cancels a quick edit without saving (#2795)", async () => {
    const user = userEvent.setup()
    const puts = serveItems([itemRow(ID, "MacBook Pro 16")])
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}`)
    await user.click(await screen.findByTestId("commodity-quick-edit-notes"))
    await user.type(screen.getByTestId("commodity-quick-notes-input"), "Draft")
    await user.click(screen.getByTestId("commodity-quick-notes-cancel"))
    expect(screen.queryByTestId("commodity-quick-notes")).toBeNull()
    expect(screen.getByTestId("commodity-quick-edit-notes")).toBeInTheDocument()
    expect(puts).toHaveLength(0)
  })

  it("opens the upload dialog with the commodity name in the title from the Files tab upload zone (#1448, #1530)", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture),
      ...fileHandlers.list(SLUG, [])
    )
    renderDetail()
    await screen.findByTestId("commodity-detail-details")
    await user.click(screen.getByTestId("commodity-detail-tab-files"))
    // #1530 swapped the Attach button for a full-width upload zone
    // so the chip-bar dictates the dropped category. The click target
    // is the zone itself; the testid moves alongside.
    const upload = await screen.findByTestId("commodity-files-upload-zone")
    await user.click(upload)
    expect(
      await screen.findByRole("heading", { name: /attach files to macbook pro 16/i })
    ).toBeInTheDocument()
  })

  it("shows the drop overlay while files are dragged over the detail page (#1448)", async () => {
    const { fireEvent } = await import("@testing-library/react")
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderDetail()
    const page = await screen.findByTestId("page-commodity-detail")
    expect(screen.queryByTestId("entity-drop-overlay")).not.toBeInTheDocument()
    const init = {
      bubbles: true,
      cancelable: true,
      // jsdom-friendly partial DataTransfer — the hook only reads
      // `types`, `files`, and `dropEffect`.
      dataTransfer: { types: ["Files"], files: [], dropEffect: "none" },
    }
    fireEvent.dragEnter(page, init)
    expect(await screen.findByTestId("entity-drop-overlay")).toBeInTheDocument()
    fireEvent.dragLeave(page, init)
    await waitFor(() => expect(screen.queryByTestId("entity-drop-overlay")).not.toBeInTheDocument())
  })

  it("renders the not-found card when the commodity is missing", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, { id: ID, type: "commodities", attributes: null })
    )
    renderDetail()
    expect(await screen.findByTestId("commodity-detail-not-found")).toBeInTheDocument()
  })

  // #1530 item 1 — terminal-status info card + Revert to In Use.
  it("renders the terminal-status info card with the status name when status !== in_use", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, {
        ...commodityFixture,
        attributes: { ...commodityFixture.attributes, status: "sold" },
      })
    )
    renderDetail()
    const card = await screen.findByTestId("commodity-detail-terminal-status")
    expect(card).toHaveTextContent(/Sold/i)
    expect(screen.getByTestId("commodity-detail-revert-status")).toBeInTheDocument()
    // The forward CHANGE STATUS bar is mutually exclusive — the
    // mock surfaces only one of the two at a time.
    expect(screen.queryByTestId("commodity-detail-change-status")).toBeNull()
  })

  it("hides the terminal-status info card while the commodity is in_use", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderDetail()
    // CHANGE STATUS bar is the in_use side of the same conditional.
    await screen.findByTestId("commodity-detail-change-status")
    expect(screen.queryByTestId("commodity-detail-terminal-status")).toBeNull()
  })

  it("reverts to in_use when the user confirms the Revert button (#1530)", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, {
        ...commodityFixture,
        attributes: { ...commodityFixture.attributes, status: "sold" },
      }),
      ...commodityHandlers.update(SLUG, ID, {
        id: ID,
        type: "commodities",
        attributes: { ...commodityFixture.attributes, status: "in_use" },
      })
    )
    renderDetail()
    const revert = await screen.findByTestId("commodity-detail-revert-status")
    await user.click(revert)
    // useConfirm pops up a modal — find its primary CTA and click.
    await waitFor(() => expect(document.body).toHaveAttribute("data-scroll-locked"))
    const buttons = screen.getAllByRole("button", { name: /confirm/i })
    await user.click(buttons[buttons.length - 1])
    // The query refetch isn't asserted here — the success path is
    // surfaced through the toast plus the modal teardown. Once the
    // modal unmounts, body-scroll lock is released.
    await waitFor(() => expect(document.body).not.toHaveAttribute("data-scroll-locked"))
  })

  // #1611 — terminal-status info card surfaces the captured
  // status_date / status_note / sale_price triple alongside the Revert
  // CTA. Each row gates on its column being set so pre-#1611 rows with
  // NULL metadata still render cleanly.
  it("renders status_date / status_note / sale_price rows when the BE persists the metadata", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, {
        ...commodityFixture,
        attributes: {
          ...commodityFixture.attributes,
          status: "sold",
          status_date: "2026-05-17",
          status_note: "Sold to Bob",
          sale_price: 99.5,
        },
      })
    )
    renderDetail()
    await screen.findByTestId("commodity-detail-terminal-status")
    expect(screen.getByTestId("commodity-detail-terminal-status-date")).toHaveTextContent(/2026/)
    expect(screen.getByTestId("commodity-detail-terminal-status-note")).toHaveTextContent(
      "Sold to Bob"
    )
    expect(screen.getByTestId("commodity-detail-terminal-status-sale-price")).toHaveTextContent(
      /\$99\.50/
    )
  })

  it("hides the status_date / status_note / sale_price rows when the BE has no metadata", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, {
        ...commodityFixture,
        attributes: { ...commodityFixture.attributes, status: "sold" },
      })
    )
    renderDetail()
    await screen.findByTestId("commodity-detail-terminal-status")
    expect(screen.queryByTestId("commodity-detail-terminal-status-date")).toBeNull()
    expect(screen.queryByTestId("commodity-detail-terminal-status-note")).toBeNull()
    expect(screen.queryByTestId("commodity-detail-terminal-status-sale-price")).toBeNull()
  })

  // #1611 — forward transitions open the StatusTransitionDialog
  // instead of the legacy useConfirm modal, then thread the captured
  // status_date / sale_price into the PATCH payload.
  it("threads sale_price + status_date into the PATCH when marking as sold", async () => {
    const user = userEvent.setup()
    const capturedBody = capture<Record<string, unknown>>()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture),
      // Inline override to capture the request body — the shared
      // commodityHandlers.update fixture doesn't expose it.
      (await import("msw")).http.put(
        `*/api/v1/g/${encodeURIComponent(SLUG)}/commodities/${encodeURIComponent(ID)}`,
        async ({ request }) => {
          const json = (await request.json()) as { data?: { attributes?: Record<string, unknown> } }
          capturedBody.value = json.data?.attributes
          return (await import("msw")).HttpResponse.json({
            data: {
              ...commodityFixture,
              attributes: { ...commodityFixture.attributes, status: "sold" },
            },
          })
        }
      )
    )
    renderDetail()
    await user.click(await screen.findByTestId("commodity-detail-transition-sold"))
    const priceInput = await screen.findByTestId("status-transition-sale-price")
    await user.type(priceInput, "150")
    await user.click(screen.getByTestId("status-transition-confirm"))
    await waitFor(() => expect(capturedBody.value).toBeDefined())
    expect(capturedBody.value?.status).toBe("sold")
    expect(capturedBody.value?.sale_price).toBe(150)
    expect(String(capturedBody.value?.status_date ?? "")).toMatch(/^\d{4}-\d{2}-\d{2}$/)
  })

  it("has no axe violations on a populated detail", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    const { container } = renderDetail()
    await screen.findByTestId("commodity-detail-details")
    expect(await axe(container)).toHaveNoViolations()
  })
})

describe("<CommodityDetailSheet /> (#1546 modal-routes overlay)", () => {
  // The sheet variant ships the same content as the page variant
  // (header, action buttons, tabs, drag-drop, dialogs); the only
  // difference is the outer chrome. These tests cover the wiring —
  // the rendered Sheet primitive shows up, the inner content is
  // present and labelled with the sheet-specific testid (so the
  // page-mode tests can keep using `page-commodity-detail`), and
  // closing the sheet navigates to `state.background`.
  function renderSheet(opts: { pathname?: string; search?: string } = {}) {
    const pathname = opts.pathname ?? `/g/${SLUG}/commodities/${ID}`
    const search = opts.search ?? ""
    setAccessToken("good-token")
    // Mount BOTH the list backdrop route and the sheet route. We
    // can't actually render <CommoditiesListPage /> without its
    // upstream queries; an empty sentinel is enough to assert the
    // backdrop URL stayed mounted while the sheet was on screen.
    function ListBackdrop() {
      const location = useLocation()
      return <div data-testid="list-backdrop">{`${location.pathname}${location.search}`}</div>
    }
    return renderWithProviders({
      // Pass the full Location-shape so `state.background` carries
      // through and `?tab=...` arrives in `location.search` (the
      // detail page's `useSearchParams` reads it from there, not
      // from the pathname).
      initialEntries: [
        {
          pathname,
          search,
          state: { background: { pathname: `/g/${SLUG}/commodities` } },
        },
      ],
      routes: (
        <>
          <Route path="/g/:groupSlug/commodities" element={<ListBackdrop />} />
          <Route
            path="/g/:groupSlug/commodities/:id"
            element={
              <GroupProvider>
                <ConfirmProvider>
                  <CommodityDetailSheet />
                </ConfirmProvider>
              </GroupProvider>
            }
          />
        </>
      ),
    })
  }

  it("renders the detail surface inside a Sheet panel", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderSheet()
    // Sheet wrapper testid identifies the overlay variant; inside,
    // the same content (`sheet-commodity-detail`) renders without
    // the page-mode `back-to-list` link.
    expect(await screen.findByTestId("commodity-detail-sheet")).toBeInTheDocument()
    expect(await screen.findByTestId("sheet-commodity-detail")).toBeInTheDocument()
    expect(screen.queryByTestId("page-commodity-detail")).toBeNull()
    expect(screen.getByRole("heading", { name: /macbook pro 16/i, level: 1 })).toBeInTheDocument()
  })

  it("renders a slim Details quick-peek (no tab strip) plus a View-full-details CTA (#1965)", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    // The 7-tab strip overflows the narrow Sheet, so #1965 drops it
    // in sheet mode: the sheet now shows only the Details fields plus
    // a "View full details" CTA into the full-page tabbed view. A
    // `?tab=warranty` deep-link no longer surfaces Warranty content
    // inside the sheet (that path resolves on the full page).
    renderSheet({ search: "?tab=warranty" })
    // Details quick-peek is present…
    expect(await screen.findByTestId("commodity-detail-details")).toBeInTheDocument()
    // …the View-full-details CTA is visible…
    expect(screen.getByTestId("commodity-detail-view-full")).toBeInTheDocument()
    // …and the tab strip + the (clipped) Warranty tab body are gone.
    expect(screen.queryByTestId("commodity-detail-tabs")).toBeNull()
    expect(screen.queryByTestId("commodity-detail-warranty")).toBeNull()
  })

  it("View full details navigates to the bare detail URL (no background → full page mounts) (#1965)", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    // Mount a single route that mirrors app/router.tsx's selection:
    // render the Sheet while `state.background` is set, else the full
    // Page. Clicking "View full details" navigates to the bare detail
    // href with NO background, so the page variant mounts.
    function DetailRouteSwitch() {
      const location = useLocation()
      const background = (location.state as { background?: unknown } | null)?.background
      return (
        <GroupProvider>
          <ConfirmProvider>
            {background ? <CommodityDetailSheet /> : <CommodityDetailPage />}
          </ConfirmProvider>
        </GroupProvider>
      )
    }
    renderWithProviders({
      initialEntries: [
        {
          pathname: `/g/${SLUG}/commodities/${ID}`,
          state: { background: { pathname: `/g/${SLUG}/commodities` } },
        },
      ],
      routes: <Route path="/g/:groupSlug/commodities/:id" element={<DetailRouteSwitch />} />,
    })
    // Starts in the sheet overlay.
    expect(await screen.findByTestId("sheet-commodity-detail")).toBeInTheDocument()
    await user.click(screen.getByTestId("commodity-detail-view-full"))
    // After navigation the background is gone, so the full-page
    // variant mounts (all 7 tabs, with the tab strip).
    expect(await screen.findByTestId("page-commodity-detail")).toBeInTheDocument()
    expect(screen.getByTestId("commodity-detail-tabs")).toBeInTheDocument()
    expect(screen.queryByTestId("sheet-commodity-detail")).toBeNull()
  })

  it("walks the list inside the sheet and keeps the list behind it in step (#2795)", async () => {
    const user = userEvent.setup()
    serveItems([itemRow("c0", "Mac mini"), itemRow(ID, "MacBook Pro 16"), itemRow("c2", "iMac")])
    renderSheet({ search: `?${FROM}` })
    await waitFor(() =>
      expect(screen.getByTestId("commodity-detail-position")).toHaveTextContent("2 of 3")
    )
    await user.click(screen.getByTestId("commodity-detail-next"))
    expect(await screen.findByRole("heading", { name: /^imac$/i, level: 1 })).toBeInTheDocument()
    expect(screen.getByTestId("commodity-detail-position")).toHaveTextContent("3 of 3")
    // Closing goes back to the list the item came from, filters included.
    await user.keyboard("{Escape}")
    await waitFor(() =>
      expect(screen.getByTestId("list-backdrop")).toHaveTextContent(`/g/${SLUG}/commodities?q=mac`)
    )
  })

  it("navigates back to the backdrop URL when the user closes the sheet (Escape)", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderSheet()
    expect(await screen.findByTestId("commodity-detail-sheet")).toBeInTheDocument()
    await user.keyboard("{Escape}")
    // The Sheet primitive's onOpenChange fires `handleClose`, which
    // navigates back to `state.background.pathname`. The list
    // backdrop sentinel renders the new URL.
    await waitFor(() =>
      expect(screen.getByTestId("list-backdrop")).toHaveTextContent(`/g/${SLUG}/commodities`)
    )
    expect(screen.queryByTestId("commodity-detail-sheet")).toBeNull()
  })
})
