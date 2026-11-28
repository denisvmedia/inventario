import { beforeEach, describe, expect, it } from "vitest"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { http, HttpResponse } from "msw"

import { CommodityDetailPage, CommodityDetailSheet } from "@/pages/commodities/CommodityDetailPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { capture } from "@/test/capture"
import { toastSpies } from "@/test/global-mocks"
import { renderWithProviders } from "@/test/render"
import { WhenRoleKnown } from "@/test/role-known"
import { server } from "@/test/server"
import {
  apiUrl,
  areaHandlers,
  authHandlers,
  commodityHandlers,
  customFieldHandlers,
  fileHandlers,
  groupHandlers,
  inboxHandlers,
  intakeHandlers,
  loanHandlers,
  locationHandlers,
  maintenanceHandlers,
  memberHandlers,
  reminderHandlers,
  serviceHandlers,
  supplyHandlers,
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

// The page mounts once the caller's role is known (WhenRoleKnown), unless
// `waitForRole` is false.
function renderDetail(
  initialPath = `/g/${SLUG}/commodities/${ID}`,
  { waitForRole = true }: { waitForRole?: boolean } = {}
) {
  setAccessToken("good-token")
  const page = <CommodityDetailPage />
  return renderWithProviders({
    withAuth: true,
    initialPath,
    routes: (
      <Route
        path="/g/:groupSlug/commodities/:id"
        element={
          <GroupProvider>
            <ConfirmProvider>
              {waitForRole ? <WhenRoleKnown>{page}</WhenRoleKnown> : page}
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
        <WhenRoleKnown>
          <CommodityDetailPage />
        </WhenRoleKnown>
        <LocationProbe />
      </ConfirmProvider>
    </GroupProvider>
  )
  return renderWithProviders({
    withAuth: true,
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
// reads next. Returns the writes in order, a PUT as its body and a draft
// mark as the item it leaves; with `gate`, a write is recorded at once and
// answered when the gate resolves.
function serveItems(rows: ReturnType<typeof itemRow>[], opts: { gate?: Promise<void> } = {}) {
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
      await opts.gate
      const row = { id, type: "commodities", attributes: { ...body.data.attributes, id } }
      store.set(id, row)
      return HttpResponse.json({ data: row })
    }),
    // The draft mark changes on the item as stored when the write lands,
    // as the server does it; it is recorded with the item it leaves.
    http.patch(apiUrl(`/g/${SLUG}/commodities/:id/draft`), async ({ params, request }) => {
      const id = params.id as string
      const body = (await request.json()) as { data: { attributes: { draft: boolean } } }
      const draft = body.data.attributes.draft
      puts.push({ id, attributes: { ...store.get(id)?.attributes, draft } })
      await opts.gate
      const stored = store.get(id)
      if (!stored) return HttpResponse.json({ errors: [{ status: "404" }] }, { status: 404 })
      const row = { ...stored, attributes: { ...stored.attributes, draft } }
      store.set(id, row)
      return HttpResponse.json({ data: row })
    })
  )
  return puts
}

const LIST = `/g/${SLUG}/commodities?q=mac`
const FROM = `from=${encodeURIComponent(LIST)}`

function gate(): { promise: Promise<void>; release: () => void } {
  let release = () => {}
  const promise = new Promise<void>((resolve) => {
    release = resolve
  })
  return { promise, release }
}

const coverFixture = {
  file_id: "f2",
  thumbnails: {
    small: "https://files.example.test/f2/small.jpg",
    medium: "https://files.example.test/f2/medium.jpg",
    large: "https://files.example.test/f2/large.jpg",
  },
  source: "explicit" as const,
}

// Three photos linked to the item; f2 is the cover.
function photoFiles(ids: string[] = ["f1", "f2", "f3"]) {
  const photos = ids.map((id) => ({
    id,
    attributes: {
      title: `Photo ${id}`,
      path: `${id}.jpg`,
      category: "images",
      type: "image",
      mime_type: "image/jpeg",
      links: [{ entity_type: "commodity", entity_id: ID }],
    },
  }))
  const signed = Object.fromEntries(
    photos.map((p) => [p.id, { url: `https://files.example.test/${p.id}/raw` }])
  )
  return fileHandlers.list(SLUG, photos, { signed_urls: signed })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

// The page offers write actions by the caller's role. Every test runs as
// the group's owner unless it registers another role before rendering.
beforeEach(() => {
  server.use(...memberHandlers.asRole("g1", "owner"))
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

  it("names the other system the item's document came from (#2809)", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture),
      ...intakeHandlers.documents(SLUG, [
        {
          id: "doc-1",
          external_system: "paperless-ngx",
          external_document_id: "117",
          source_name: "Paperless-ngx",
          source_url: "https://paperless.example/documents/117/",
          draft_id: "d1",
          commodity_id: ID,
          state: "item",
        },
      ])
    )
    renderDetail()
    const row = await screen.findByTestId("commodity-detail-source")
    expect(row).toHaveTextContent("Source")
    const link = within(row).getByRole("link", { name: /Paperless-ngx/ })
    expect(link).toHaveAttribute("href", "https://paperless.example/documents/117/")
    expect(link).toHaveAttribute("target", "_blank")
  })

  it("has no source row for an item made in the app", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderDetail()
    await screen.findByTestId("commodity-detail-seller")
    expect(screen.queryByTestId("commodity-detail-source")).toBeNull()
  })

  it("shows brand · model under the name (#2804)", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, {
        ...commodityFixture,
        attributes: { ...commodityFixture.attributes, brand: "Apple", model: "A2780" },
      })
    )
    renderDetail()
    expect(await screen.findByTestId("commodity-detail-brand-model")).toHaveTextContent(
      /^Apple · A2780$/
    )
  })

  it("leaves the brand line out for an item without a brand or model (#2804)", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderDetail()
    await screen.findByTestId("commodity-detail-name")
    expect(screen.queryByTestId("commodity-detail-brand-model")).toBeNull()
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

  it("links tags, the area and the type to the filtered items list", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...tagHandlers.list(SLUG, [{ id: "t1", slug: "work", label: "Work", color: "green" }]),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderDetail()
    const tag = await screen.findByRole("link", { name: "Work" })
    // Each opens the list filtered by that alone, inactive items included.
    expect(tag).toHaveAttribute("href", `/g/${SLUG}/commodities?inactive=1&tags=work`)
    expect(tag.className).toContain("text-tag-green")
    expect(screen.getByRole("link", { name: "tech" })).toHaveAttribute(
      "href",
      `/g/${SLUG}/commodities?inactive=1&tags=tech`
    )
    expect(screen.getByTestId("commodity-detail-area-link")).toHaveAttribute(
      "href",
      `/g/${SLUG}/commodities?area=a1&inactive=1`
    )
    expect(screen.getByTestId("commodity-detail-type-link")).toHaveAttribute(
      "href",
      `/g/${SLUG}/commodities?type=electronics&inactive=1`
    )
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

  it("locks the edit form while its save is in flight (#2795)", async () => {
    const user = userEvent.setup()
    const held = gate()
    const puts = serveItems([itemRow(ID, "MacBook Pro 16")], { gate: held.promise })
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}/edit`)
    const name = await screen.findByLabelText(/^name/i)
    await user.clear(name)
    await user.type(name, "MacBook Air")
    await user.click(screen.getByTestId("commodity-edit-save"))
    await waitFor(() => expect(puts).toHaveLength(1))
    // An edit typed now would miss the request and be dropped after it.
    expect(name).toBeDisabled()
    expect(screen.getByTestId("commodity-edit-tags-option-home")).toBeDisabled()
    expect(screen.getByTestId("commodity-edit-all-fields")).toBeDisabled()
    expect(screen.getByTestId("commodity-edit-save")).toBeDisabled()

    held.release()
    expect(await screen.findByTestId("commodity-detail-tabs")).toBeInTheDocument()
    expect(puts[0].attributes.name).toBe("MacBook Air")
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

  it("keeps the status date, note and sale price when All fields saves (#2915)", async () => {
    const user = userEvent.setup()
    const row = itemRow(ID, "MacBook Pro 16")
    row.attributes = {
      ...row.attributes,
      status: "sold",
      status_date: "2026-05-17",
      status_note: "Sold to Bob",
      sale_price: 99.5,
    }
    const puts = serveItems([row])
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}/edit`)
    await screen.findByLabelText(/^name/i)
    await user.click(screen.getByTestId("commodity-edit-all-fields"))
    const dialog = await screen.findByRole("dialog")
    const name = within(dialog).getByRole("textbox", { name: /^name/i })
    await user.clear(name)
    await user.type(name, "MacBook Air")
    await user.click(within(dialog).getByTestId("commodity-form-submit"))
    await waitFor(() => expect(puts).toHaveLength(1))
    expect(puts[0].attributes).toMatchObject({
      name: "MacBook Air",
      status: "sold",
      status_date: "2026-05-17",
      status_note: "Sold to Bob",
      sale_price: 99.5,
    })
  })

  // The item changed after the edit form opened: a save from All fields is
  // refused, and the form under the dialog offers the reload. A form opened
  // again afterwards is fresh and says nothing about it (#2981).
  it("refuses an All fields save on a changed item, and forgets it when the form closes", async () => {
    const user = userEvent.setup()
    serveItems([itemRow(ID, "MacBook Pro 16")])
    server.use(
      http.put(apiUrl(`/g/${SLUG}/commodities/:id`), () =>
        HttpResponse.json(
          {
            errors: [
              {
                status: "Conflict",
                code: "commodity.revision_conflict",
                error: { msg: "the item changed since it was read" },
              },
            ],
          },
          { status: 409 }
        )
      )
    )
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}`)
    await user.click(await screen.findByTestId("commodity-detail-edit"))
    await user.click(await screen.findByTestId("commodity-edit-all-fields"))
    const dialog = await screen.findByRole("dialog")
    await user.click(within(dialog).getByTestId("commodity-form-submit"))

    expect(
      await within(dialog).findByText(/This item changed since you opened it, so nothing was saved/)
    ).toBeInTheDocument()
    await user.click(within(dialog).getByTestId("commodity-form-cancel"))
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull())
    expect(screen.getByTestId("commodity-edit-stale")).toBeInTheDocument()

    await user.click(screen.getByTestId("commodity-edit-cancel"))
    await user.click(await screen.findByTestId("commodity-detail-edit"))
    expect(await screen.findByTestId("commodity-edit-form")).toBeInTheDocument()
    expect(screen.queryByTestId("commodity-edit-stale")).toBeNull()
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
    // The red outline trigger of a detail surface stays red on hover (#2914).
    expect(screen.getByTestId("commodity-detail-delete")).toHaveClass(
      "text-destructive",
      "hover:bg-destructive/10",
      "hover:text-destructive"
    )
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

  it("shows the server's reason when the delete is refused", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture),
      http.delete(apiUrl(`/g/${SLUG}/commodities/${ID}`), () =>
        HttpResponse.json(
          { errors: [{ status: "Conflict", detail: "the item is lent out" }] },
          { status: 409 }
        )
      )
    )
    renderDetail()
    await user.click(await screen.findByTestId("commodity-detail-delete"))
    await waitFor(() => expect(document.body).toHaveAttribute("data-scroll-locked"))
    const buttons = screen.getAllByRole("button", { name: /^Delete$/i })
    await user.click(buttons[buttons.length - 1])
    await waitFor(() => expect(toastSpies.error).toHaveBeenCalledWith("the item is lent out"))
    expect(toastSpies.error).toHaveBeenCalledTimes(1)
    expect(screen.getByTestId("commodity-detail-delete")).toBeInTheDocument()
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

  it("closes a quick edit when Next opens an item that is already loaded (#2795)", async () => {
    const user = userEvent.setup()
    const puts = serveItems([
      itemRow("c0", "Mac mini"),
      itemRow(ID, "MacBook Pro 16"),
      itemRow("c2", "iMac"),
    ])
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}?${FROM}`)
    await waitFor(() =>
      expect(screen.getByTestId("commodity-detail-position")).toHaveTextContent("2 of 3")
    )
    // Visit the next item once, so it is in the cache when Next opens it again.
    await user.click(screen.getByTestId("commodity-detail-next"))
    await waitFor(() =>
      expect(screen.getByTestId("commodity-detail-position")).toHaveTextContent("3 of 3")
    )
    await user.click(screen.getByTestId("commodity-detail-previous"))
    await waitFor(() =>
      expect(screen.getByTestId("commodity-detail-position")).toHaveTextContent("2 of 3")
    )

    await user.click(await screen.findByTestId("commodity-quick-edit-notes"))
    await user.type(screen.getByTestId("commodity-quick-notes-input"), "For the laptop")
    await user.click(screen.getByTestId("commodity-detail-next"))

    await waitFor(() =>
      expect(screen.getByTestId("commodity-detail-position")).toHaveTextContent("3 of 3")
    )
    expect(screen.queryByTestId("commodity-quick-notes")).toBeNull()
    expect(screen.getByTestId("commodity-quick-edit-notes")).toBeInTheDocument()
    expect(puts).toHaveLength(0)
  })

  it("keeps a notes editor opened while a tags save finishes (#2795)", async () => {
    const user = userEvent.setup()
    const held = gate()
    const puts = serveItems([itemRow(ID, "MacBook Pro 16")], { gate: held.promise })
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}`)
    await user.click(await screen.findByTestId("commodity-quick-edit-tags"))
    await user.click(await screen.findByTestId("commodity-quick-tags-palette-option-home"))
    await user.click(screen.getByTestId("commodity-quick-tags-save"))
    await waitFor(() => expect(puts).toHaveLength(1))

    await user.click(screen.getByTestId("commodity-quick-edit-notes"))
    await user.type(screen.getByTestId("commodity-quick-notes-input"), "Draft")
    held.release()

    await waitFor(() =>
      expect(screen.getByTestId("commodity-detail-tags")).toHaveTextContent("Home")
    )
    expect(screen.getByTestId("commodity-quick-notes-input")).toHaveValue("Draft")
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
    // The card shows while no group is resolved for the page, which is
    // before any role is known.
    renderDetail(undefined, { waitForRole: false })
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

  it("offers Donated and Stolen among the status changes of an item in use", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderDetail()
    const bar = await screen.findByTestId("commodity-detail-change-status")
    expect(
      within(bar)
        .getAllByRole("button")
        .map((b) => b.textContent)
    ).toEqual(["Sold", "Lost", "Disposed", "Written Off", "Donated", "Stolen"])
    expect(within(bar).getByTestId("commodity-detail-transition-donated")).toHaveClass(
      "text-item-status-donated"
    )
    expect(within(bar).getByTestId("commodity-detail-transition-stolen")).toHaveClass(
      "text-item-status-stolen"
    )
  })

  it.each([
    ["donated", "Donated", "text-item-status-donated"],
    ["stolen", "Stolen", "text-item-status-stolen"],
  ])("shows a %s item's status card in its own tone", async (status, label, tone) => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, {
        ...commodityFixture,
        attributes: {
          ...commodityFixture.attributes,
          status,
          status_date: "2026-11-26",
          status_note: "A note",
        },
      })
    )
    renderDetail()
    const card = await screen.findByTestId("commodity-detail-terminal-status")
    expect(within(card).getByTestId("commodity-detail-terminal-status-label")).toHaveTextContent(
      label
    )
    expect(card).toHaveClass(tone)
    expect(screen.queryByTestId("commodity-detail-terminal-status-sale-price")).toBeNull()
    expect(screen.getByTestId("commodity-detail-revert-status")).toBeInTheDocument()
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

  it("shows the cover photo beside the details and opens the viewer on it (#2797)", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture, coverFixture),
      ...photoFiles()
    )
    const { container } = renderDetail()
    const cover = await screen.findByTestId("commodity-detail-cover")
    const img = within(cover).getByRole("img")
    expect(img.getAttribute("srcset")).toContain("https://files.example.test/f2/large.jpg 600w")
    expect(await within(cover).findByTestId("commodity-detail-photo-count")).toHaveTextContent(
      "3 photos"
    )
    expect(cover).toHaveAccessibleName(/view 3 photos of macbook pro 16/i)
    expect(await axe(container)).toHaveNoViolations()

    // Enter on the focused cover opens the viewer on the cover photo.
    cover.focus()
    await user.keyboard("{Enter}")
    const viewer = await screen.findByTestId("file-image-viewer")
    expect(within(viewer).getByTestId("image-viewer-position")).toHaveTextContent("2 / 3")
    await user.click(within(viewer).getByTestId("image-viewer-next"))
    expect(within(viewer).getByTestId("image-viewer-position")).toHaveTextContent("3 / 3")
    await user.keyboard("{Escape}")
    await waitFor(() => expect(screen.queryByTestId("file-image-viewer")).toBeNull())
  })

  it("leads the viewer with the cover when the photo list does not hold it (#2797)", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture, coverFixture),
      // The list holds the newest photos; the cover is older than all of them.
      ...photoFiles(["f3", "f4", "f5"])
    )
    renderDetail()
    const cover = await screen.findByTestId("commodity-detail-cover")
    await within(cover).findByTestId("commodity-detail-photo-count")
    await user.click(cover)
    const viewer = await screen.findByTestId("file-image-viewer")
    expect(within(viewer).getByTestId("image-viewer-position")).toHaveTextContent("1 / 4")
    expect(within(viewer).getByRole("img")).toHaveAttribute(
      "src",
      "https://files.example.test/f2/large.jpg"
    )
    await user.click(within(viewer).getByTestId("image-viewer-next"))
    expect(within(viewer).getByTestId("image-viewer-position")).toHaveTextContent("2 / 4")
  })

  it("keeps the viewer on the cover when the photo list arrives while it is open (#2797)", async () => {
    const user = userEvent.setup()
    const held = gate()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture, coverFixture),
      // Answers nothing until released, then falls through to the list.
      http.get(apiUrl(`/g/${SLUG}/files`), async () => {
        await held.promise
      }),
      ...photoFiles()
    )
    renderDetail()
    await user.click(await screen.findByTestId("commodity-detail-cover"))
    const viewer = await screen.findByTestId("file-image-viewer")
    expect(within(viewer).getByRole("img")).toHaveAttribute(
      "src",
      "https://files.example.test/f2/large.jpg"
    )
    expect(within(viewer).queryByTestId("image-viewer-position")).toBeNull()

    held.release()
    await waitFor(() =>
      expect(within(viewer).getByTestId("image-viewer-position")).toHaveTextContent("2 / 3")
    )
    expect(within(viewer).getByRole("img")).toHaveAttribute(
      "src",
      "https://files.example.test/f2/raw"
    )
  })

  it("keeps the small type tile when the item has no photo (#2797)", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderDetail()
    const thumb = await screen.findByTestId("commodity-detail-thumb")
    expect(thumb).toHaveAttribute("data-state", "fallback")
    expect(thumb.style.width).toBe("48px")
    expect(screen.queryByTestId("commodity-detail-cover")).toBeNull()
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

  it("shows the cover at 96px in the sheet, without the viewer (#2797)", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture, coverFixture)
    )
    renderSheet()
    const thumb = await screen.findByTestId("commodity-detail-thumb")
    expect(thumb).toHaveAttribute("data-state", "image")
    expect(thumb.style.width).toBe("96px")
    expect(screen.queryByTestId("commodity-detail-cover")).toBeNull()
  })

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

// Opened from the inbox, an item marked as draft walks the Items to review
// list: Mark as reviewed clears the mark and opens the next one, Skip opens
// the next one as it is (#2794).
describe("<CommodityDetailPage /> review from the inbox", () => {
  const REVIEW = `/g/${SLUG}/inbox?reason=item_draft`
  const FROM_INBOX = `from=${encodeURIComponent(REVIEW)}`

  function draftRow(id: string, name: string) {
    const row = itemRow(id, name)
    row.attributes.draft = true
    return row
  }

  // The items behind serveItems, with the inbox listing those still marked
  // as draft page by page, and the member's role.
  function serveReview(
    rows: ReturnType<typeof itemRow>[],
    role = "user",
    opts: { gate?: Promise<void> } = {}
  ) {
    const puts = serveItems(rows, opts)
    const stillDraft = () =>
      rows.filter((r) => {
        const last = puts.filter((p) => p.id === r.id).at(-1)
        return last ? last.attributes.draft === true : r.attributes.draft === true
      })
    server.use(
      ...authHandlers.signedIn(),
      http.get(apiUrl("/groups/g1/members"), () =>
        HttpResponse.json({
          data: [
            {
              id: "m1",
              type: "memberships",
              attributes: { group_id: "g1", member_user_id: "u1", role },
            },
          ],
        })
      ),
      http.get(apiUrl(`/g/${SLUG}/inbox`), ({ request }) => {
        const url = new URL(request.url)
        const page = Number(url.searchParams.get("page") ?? "1")
        const perPage = Number(url.searchParams.get("per_page") ?? "50")
        const entries = stillDraft().map((r) =>
          inboxHandlers.itemDraft(r.id, String(r.attributes.name))
        )
        return HttpResponse.json({
          data: entries.slice((page - 1) * perPage, page * perPage),
          meta: {
            entries: entries.length,
            counts: { total: entries.length, reasons: { item_draft: entries.length } },
          },
        })
      })
    )
    return puts
  }

  function renderReview(initialPath: string) {
    setAccessToken("good-token")
    const element = (
      <AuthProvider>
        <GroupProvider>
          <ConfirmProvider>
            <CommodityDetailPage />
            <LocationProbe />
          </ConfirmProvider>
        </GroupProvider>
      </AuthProvider>
    )
    return renderWithProviders({
      initialPath,
      routes: (
        <>
          <Route path="/g/:groupSlug/commodities/:id" element={element} />
          <Route path="/g/:groupSlug/inbox" element={<LocationProbe />} />
        </>
      ),
    })
  }

  it("marks the item as reviewed and opens the next one", async () => {
    const user = userEvent.setup()
    const puts = serveReview([draftRow("c1", "Bike"), draftRow("c2", "Lamp")])
    renderReview(`/g/${SLUG}/commodities/c1?${FROM_INBOX}`)
    expect(await screen.findByTestId("commodity-review-position")).toHaveTextContent(
      "Item 1 of 2 to review"
    )
    // Back returns to the inbox list it came from.
    expect(screen.getByTestId("commodity-detail-back")).toHaveAttribute("href", REVIEW)
    expect(screen.getByTestId("commodity-detail-back")).toHaveTextContent("Back to inbox")
    await user.click(screen.getByTestId("commodity-review-mark"))
    await waitFor(() => expect(puts).toHaveLength(1))
    expect(puts[0]).toMatchObject({ id: "c1", attributes: { name: "Bike", draft: false } })
    await waitFor(() =>
      expect(screen.getByTestId("location-probe")).toHaveTextContent(
        `/g/${SLUG}/commodities/c2?${FROM_INBOX}`
      )
    )
    expect(toastSpies.success).toHaveBeenCalledWith(
      "“Bike” marked as reviewed",
      expect.objectContaining({ action: expect.objectContaining({ label: "Undo" }) })
    )
    // The reviewed item left the list: the next one is now the only one.
    expect(await screen.findByTestId("commodity-review-position")).toHaveTextContent(
      "Item 1 of 1 to review"
    )
  })

  it("opens the item that now follows when the next one left the list meanwhile", async () => {
    const user = userEvent.setup()
    const puts = serveReview([
      draftRow("c1", "Bike"),
      draftRow("c2", "Lamp"),
      draftRow("c3", "Desk"),
    ])
    renderReview(`/g/${SLUG}/commodities/c1?${FROM_INBOX}`)
    await waitFor(() =>
      expect(screen.getByTestId("commodity-review-position")).toHaveTextContent(
        "Item 1 of 3 to review"
      )
    )
    // Another member reviews the next item while this one is on screen.
    puts.push({ id: "c2", attributes: { ...itemRow("c2", "Lamp").attributes, draft: false } })
    await user.click(screen.getByTestId("commodity-review-mark"))
    await waitFor(() =>
      expect(screen.getByTestId("location-probe")).toHaveTextContent(
        `/g/${SLUG}/commodities/c3?${FROM_INBOX}`
      )
    )
    expect(await screen.findByTestId("commodity-review-position")).toHaveTextContent(
      "Item 1 of 1 to review"
    )
  })

  it("opens the next item when an earlier one left the list meanwhile", async () => {
    const user = userEvent.setup()
    const puts = serveReview([
      draftRow("c1", "Bike"),
      draftRow("c2", "Lamp"),
      draftRow("c3", "Desk"),
    ])
    renderReview(`/g/${SLUG}/commodities/c2?${FROM_INBOX}`)
    await waitFor(() =>
      expect(screen.getByTestId("commodity-review-position")).toHaveTextContent(
        "Item 2 of 3 to review"
      )
    )
    // Another member reviews the first item while this one is on screen:
    // after the save, the next item is the only one left, before this
    // item's old place.
    puts.push({ id: "c1", attributes: { ...itemRow("c1", "Bike").attributes, draft: false } })
    await user.click(screen.getByTestId("commodity-review-mark"))
    await waitFor(() =>
      expect(screen.getByTestId("location-probe")).toHaveTextContent(
        `/g/${SLUG}/commodities/c3?${FROM_INBOX}`
      )
    )
  })

  it("opens the next item across a page edge", async () => {
    const user = userEvent.setup()
    const rows = Array.from({ length: 52 }, (_, i) => draftRow(`c${i + 1}`, `Item ${i + 1}`))
    serveReview(rows)
    renderReview(`/g/${SLUG}/commodities/c50?${FROM_INBOX}`)
    await waitFor(() =>
      expect(screen.getByTestId("commodity-review-position")).toHaveTextContent(
        "Item 50 of 52 to review"
      )
    )
    // The position comes from page 1, but the next item is on page 2, and
    // both review actions stay disabled until that page has loaded. A click
    // before then lands on a disabled button and does nothing.
    const mark = screen.getByTestId("commodity-review-mark")
    await waitFor(() => expect(mark).toBeEnabled())
    await user.click(mark)
    await waitFor(() =>
      expect(screen.getByTestId("location-probe")).toHaveTextContent(`/g/${SLUG}/commodities/c51?`)
    )
    await waitFor(() =>
      expect(screen.getByTestId("commodity-review-position")).toHaveTextContent(
        "Item 50 of 51 to review"
      )
    )
  })

  it("returns to the inbox after the last item", async () => {
    const user = userEvent.setup()
    serveReview([draftRow("c1", "Bike")])
    renderReview(`/g/${SLUG}/commodities/c1?${FROM_INBOX}`)
    await screen.findByTestId("commodity-review-position")
    // Nothing to skip to.
    expect(screen.queryByTestId("commodity-review-skip")).toBeNull()
    await user.click(screen.getByTestId("commodity-review-mark"))
    await waitFor(() => expect(screen.getByTestId("location-probe")).toHaveTextContent(REVIEW))
  })

  it("skips to the next item without changing the current one", async () => {
    const user = userEvent.setup()
    const puts = serveReview([draftRow("c1", "Bike"), draftRow("c2", "Lamp")])
    renderReview(`/g/${SLUG}/commodities/c1?${FROM_INBOX}`)
    await screen.findByTestId("commodity-review-position")
    await user.click(screen.getByTestId("commodity-review-skip"))
    await waitFor(() =>
      expect(screen.getByTestId("commodity-review-position")).toHaveTextContent(
        "Item 2 of 2 to review"
      )
    )
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      `/g/${SLUG}/commodities/c2?${FROM_INBOX}`
    )
    expect(puts).toHaveLength(0)
  })

  it("stays where the user went while the review saved", async () => {
    const user = userEvent.setup()
    const put = gate()
    serveReview([draftRow("c1", "Bike"), draftRow("c2", "Lamp"), draftRow("c3", "Desk")], "user", {
      gate: put.promise,
    })
    renderReview(`/g/${SLUG}/commodities/c2?${FROM_INBOX}`)
    await waitFor(() =>
      expect(screen.getByTestId("commodity-review-position")).toHaveTextContent(
        "Item 2 of 3 to review"
      )
    )
    await user.click(screen.getByTestId("commodity-review-mark"))
    // Nothing else to start while the review saves.
    expect(screen.getByTestId("commodity-review-skip")).toBeDisabled()
    expect(screen.getByTestId("commodity-detail-edit")).toBeDisabled()
    await user.click(screen.getByTestId("commodity-detail-previous"))
    await waitFor(() =>
      expect(screen.getByTestId("location-probe")).toHaveTextContent(`/g/${SLUG}/commodities/c1?`)
    )
    put.release()
    await waitFor(() => expect(toastSpies.success).toHaveBeenCalled())
    expect(screen.getByTestId("location-probe")).toHaveTextContent(`/g/${SLUG}/commodities/c1?`)
  })

  it("stays on the item when a quick edit was opened while the next item was looked up", async () => {
    const user = userEvent.setup()
    serveReview([draftRow("c1", "Bike"), draftRow("c2", "Lamp")])
    renderReview(`/g/${SLUG}/commodities/c1?${FROM_INBOX}`)
    const mark = await screen.findByTestId("commodity-review-mark")
    await waitFor(() => expect(mark).toBeEnabled())
    // The save refreshes the list once; the lookup of the next item is the
    // read after that, held until the edit below is open.
    const lookup = gate()
    let reads = 0
    server.use(
      http.get(apiUrl(`/g/${SLUG}/inbox`), async () => {
        reads += 1
        if (reads >= 2) await lookup.promise
        return undefined
      })
    )
    await user.click(mark)
    await waitFor(() => expect(reads).toBe(2))
    await user.click(await screen.findByTestId("commodity-quick-edit-notes"))
    await user.type(screen.getByTestId("commodity-quick-notes-input"), "Check the tires")
    lookup.release()
    await new Promise((resolve) => setTimeout(resolve, 50))
    expect(screen.getByTestId("location-probe")).toHaveTextContent(`/g/${SLUG}/commodities/c1?`)
    expect(screen.getByTestId("commodity-quick-notes-input")).toHaveValue("Check the tires")
  })

  it("stays on the item when its status dialog was opened while the next item was looked up", async () => {
    const user = userEvent.setup()
    const puts = serveReview([draftRow("c1", "Bike"), draftRow("c2", "Lamp")])
    renderReview(`/g/${SLUG}/commodities/c1?${FROM_INBOX}`)
    const mark = await screen.findByTestId("commodity-review-mark")
    await waitFor(() => expect(mark).toBeEnabled())
    const lookup = gate()
    let reads = 0
    server.use(
      http.get(apiUrl(`/g/${SLUG}/inbox`), async () => {
        reads += 1
        if (reads >= 2) await lookup.promise
        return undefined
      })
    )
    await user.click(mark)
    await waitFor(() => expect(reads).toBe(2))
    await user.click(await screen.findByTestId("commodity-detail-transition-lost"))
    const dialog = await screen.findByTestId("status-transition-dialog")
    lookup.release()
    await new Promise((resolve) => setTimeout(resolve, 50))
    expect(screen.getByTestId("location-probe")).toHaveTextContent(`/g/${SLUG}/commodities/c1?`)
    // The status lands on the item the dialog was opened on.
    await user.click(within(dialog).getByTestId("status-transition-confirm"))
    await waitFor(() => expect(puts.at(-1)?.attributes.status).toBe("lost"))
    expect(puts.at(-1)?.id).toBe("c1")
    expect(puts.filter((p) => p.id === "c2")).toHaveLength(0)
  })

  // The tabs keep their dialogs in their own state; the reminder dialog on
  // the Details tab stands for all of them (hooks/useItemHold).
  it("stays on the item when its reminder dialog was opened while the next item was looked up", async () => {
    const user = userEvent.setup()
    serveReview([draftRow("c1", "Bike"), draftRow("c2", "Lamp")])
    server.use(...reminderHandlers.listForCommodity(SLUG, "c1"))
    renderReview(`/g/${SLUG}/commodities/c1?${FROM_INBOX}`)
    const mark = await screen.findByTestId("commodity-review-mark")
    await waitFor(() => expect(mark).toBeEnabled())
    const lookup = gate()
    let reads = 0
    server.use(
      http.get(apiUrl(`/g/${SLUG}/inbox`), async () => {
        reads += 1
        if (reads >= 2) await lookup.promise
        return undefined
      })
    )
    await user.click(mark)
    await waitFor(() => expect(reads).toBe(2))
    await user.click(await screen.findByTestId("reminder-add"))
    await user.type(await screen.findByTestId("reminder-title-input"), "Check the tires")
    lookup.release()
    await new Promise((resolve) => setTimeout(resolve, 50))
    expect(screen.getByTestId("location-probe")).toHaveTextContent(`/g/${SLUG}/commodities/c1?`)
    expect(screen.getByTestId("reminder-title-input")).toHaveValue("Check the tires")
  })

  it("keeps the tab and the Files chip on the next item", async () => {
    const user = userEvent.setup()
    serveReview([draftRow("c1", "Bike"), draftRow("c2", "Lamp")])
    server.use(...fileHandlers.list(SLUG), ...fileHandlers.countsFromFiles(SLUG, []))
    renderReview(`/g/${SLUG}/commodities/c1?tab=files&files=images&${FROM_INBOX}`)
    await screen.findByTestId("commodity-review-position")
    await user.click(screen.getByTestId("commodity-review-skip"))
    await waitFor(() =>
      expect(screen.getByTestId("commodity-review-position")).toHaveTextContent(
        "Item 2 of 2 to review"
      )
    )
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      `/g/${SLUG}/commodities/c2?tab=files&files=images&${FROM_INBOX}`
    )
    expect(await screen.findByTestId("commodity-files-chip-images")).toHaveAttribute(
      "aria-selected",
      "true"
    )
  })

  it("waits for a quick edit to save before a review", async () => {
    const user = userEvent.setup()
    const put = gate()
    const puts = serveReview([draftRow("c1", "Bike")], "user", { gate: put.promise })
    renderReview(`/g/${SLUG}/commodities/c1?${FROM_INBOX}`)
    await screen.findByTestId("commodity-review-position")
    await user.click(await screen.findByTestId("commodity-quick-edit-notes"))
    await user.type(screen.getByTestId("commodity-quick-notes-input"), "Check the tires")
    await user.click(screen.getByTestId("commodity-quick-notes-save"))
    await waitFor(() => expect(puts).toHaveLength(1))
    expect(screen.getByTestId("commodity-review-mark")).toBeDisabled()
    put.release()
    await waitFor(() => expect(screen.getByTestId("commodity-review-mark")).toBeEnabled())
  })

  it("keeps Mark as reviewed off while a quick edit is open", async () => {
    const user = userEvent.setup()
    serveReview([draftRow("c1", "Bike"), draftRow("c2", "Lamp")])
    renderReview(`/g/${SLUG}/commodities/c1?${FROM_INBOX}`)
    const mark = await screen.findByTestId("commodity-review-mark")
    await waitFor(() => expect(mark).toBeEnabled())
    await user.click(await screen.findByTestId("commodity-quick-edit-notes"))
    expect(mark).toBeDisabled()
    expect(mark).toHaveAttribute("title", "Save or cancel the open edit first.")
  })

  it("stays on the item when the list to review could not be read", async () => {
    const user = userEvent.setup()
    const puts = serveReview([draftRow("c1", "Bike")])
    renderReview(`/g/${SLUG}/commodities/c1?${FROM_INBOX}`)
    const mark = await screen.findByTestId("commodity-review-mark")
    await waitFor(() => expect(mark).toBeEnabled())
    // The list was read; reading it again after the save fails.
    server.use(
      http.get(apiUrl(`/g/${SLUG}/inbox`), () =>
        HttpResponse.json({ errors: [{ status: "500" }] }, { status: 500 })
      )
    )
    await user.click(mark)
    await waitFor(() => expect(puts).toHaveLength(1))
    await waitFor(() => expect(toastSpies.success).toHaveBeenCalled())
    // Not the end of the list, just an unknown one: no trip to the inbox.
    expect(screen.getByTestId("location-probe")).toHaveTextContent(`/g/${SLUG}/commodities/c1?`)
  })

  it("shows no review bar to a viewer or on a reviewed item", async () => {
    serveReview([itemRow("c1", "Bike"), draftRow("c2", "Lamp")], "viewer")
    renderReview(`/g/${SLUG}/commodities/c2?${FROM_INBOX}`)
    expect(await screen.findByRole("heading", { name: /^lamp$/i, level: 1 })).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByTestId("commodity-detail-position")).toHaveTextContent("1 of 1")
    )
    expect(screen.queryByTestId("commodity-review-bar")).toBeNull()
  })
})

describe("CommodityDetailPage custom fields", () => {
  beforeEach(() => {
    clearAuth()
    __resetGroupContextForTests()
    __resetHttpForTests()
  })

  it("shows the values the item has, a link by name and a linked item in Trash", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...customFieldHandlers.list(SLUG, [
        customFieldHandlers.field({ id: "cf-charger", name: "Charger", type: "item_link" }),
        customFieldHandlers.field({ id: "cf-dock", name: "Dock", type: "item_link" }),
        customFieldHandlers.field({ id: "cf-keyboard", name: "Keyboard", type: "boolean" }),
        customFieldHandlers.field({
          id: "cf-grade",
          name: "Grade",
          type: "select",
          options: [{ id: "opt-a", label: "A" }],
        }),
        customFieldHandlers.field({ id: "cf-unused", name: "Unused" }),
      ]),
      http.get(apiUrl(`/g/${SLUG}/commodities/${ID}`), () =>
        HttpResponse.json({
          data: {
            ...commodityFixture,
            attributes: {
              ...commodityFixture.attributes,
              custom_fields: {
                "cf-charger": "c2",
                "cf-dock": "c3",
                "cf-keyboard": false,
                "cf-grade": "opt-a",
              },
            },
          },
          meta: {
            linked_items: {
              c2: { name: "USB-C charger", trashed: false },
              c3: { name: "Old dock", trashed: true },
            },
          },
        })
      )
    )
    renderDetail()
    const charger = await screen.findByTestId("commodity-detail-custom-field-cf-charger")
    expect(within(charger).getByRole("link", { name: "USB-C charger" })).toHaveAttribute(
      "href",
      `/g/${SLUG}/commodities/c2`
    )
    const dock = screen.getByTestId("commodity-detail-custom-field-cf-dock")
    expect(dock).toHaveTextContent("Old dock")
    expect(dock).toHaveTextContent("in Trash")
    expect(within(dock).queryByRole("link")).not.toBeInTheDocument()
    expect(screen.getByTestId("commodity-detail-custom-field-cf-keyboard")).toHaveTextContent("No")
    expect(screen.getByTestId("commodity-detail-custom-field-cf-grade")).toHaveTextContent("A")
    expect(screen.queryByTestId("commodity-detail-custom-field-cf-unused")).not.toBeInTheDocument()
  })
})

describe("<CommodityDetailPage /> by role (#2886)", () => {
  // Items and their files take writes from users, admins and owners; a
  // viewer only reads.
  function expectNoItemWrites() {
    expect(screen.queryByTestId("commodity-detail-edit")).not.toBeInTheDocument()
    expect(screen.queryByTestId("commodity-detail-delete")).not.toBeInTheDocument()
    expect(screen.queryByTestId("commodity-detail-change-status")).not.toBeInTheDocument()
    expect(screen.queryByTestId("commodity-quick-edit-tags")).not.toBeInTheDocument()
    expect(screen.queryByTestId("commodity-quick-edit-notes")).not.toBeInTheDocument()
  }

  it("offers a viewer no edit, delete, status change or quick edit", async () => {
    server.use(...memberHandlers.asRole("g1", "viewer"))
    serveItems([itemRow(ID, "MacBook Pro 16")])
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}`)
    expect(await screen.findByTestId("commodity-detail-insurance")).toBeInTheDocument()
    expectNoItemWrites()
  })

  it("shows a viewer who follows an edit link the item, not the form", async () => {
    server.use(...memberHandlers.asRole("g1", "viewer"))
    serveItems([itemRow(ID, "MacBook Pro 16")])
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}/edit`)
    expect(await screen.findByTestId("commodity-detail-insurance")).toBeInTheDocument()
    expect(screen.queryByTestId("commodity-edit-form")).not.toBeInTheDocument()
    expectNoItemWrites()
  })

  it("offers a viewer no upload on the Warranty and Files tabs", async () => {
    const user = userEvent.setup()
    server.use(
      ...memberHandlers.asRole("g1", "viewer"),
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture),
      ...fileHandlers.list(SLUG, []),
      ...fileHandlers.counts(SLUG, {})
    )
    renderDetail(`/g/${SLUG}/commodities/${ID}?tab=warranty`)
    expect(await screen.findByTestId("commodity-detail-warranty-status")).toBeInTheDocument()
    expect(screen.queryByTestId("commodity-detail-warranty-upload-receipt")).not.toBeInTheDocument()
    await user.click(screen.getByTestId("commodity-detail-tab-files"))
    expect(await screen.findByTestId("commodity-files-chip-bar")).toBeInTheDocument()
    expect(screen.queryByTestId("commodity-files-upload-zone")).not.toBeInTheDocument()
    expect(screen.queryByTestId("commodity-files-attach-existing")).not.toBeInTheDocument()
  })

  // #3038: the tabs that record loans, services, supplies and maintenance,
  // and the reminders on the Details tab, are writes the server refuses a
  // viewer too. Each has a record that would carry actions for a user.
  it("offers a viewer no writes on the Lend, Service, Supplies and Maintenance tabs or reminders", async () => {
    const user = userEvent.setup()
    server.use(
      ...memberHandlers.asRole("g1", "viewer"),
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...tagHandlers.list(SLUG, []),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture),
      ...fileHandlers.list(SLUG, []),
      ...fileHandlers.counts(SLUG, {}),
      ...reminderHandlers.listForCommodity(SLUG, ID, [
        {
          id: "r1",
          commodity_id: ID,
          title: "Insurance renewal",
          due_on: "2027-01-31",
          remind_days_before: [7],
          repeat_unit: "none",
          repeat_interval: 1,
        },
      ]),
      ...loanHandlers.listForCommodity(SLUG, ID, [
        {
          id: "loan-1",
          commodity_id: ID,
          borrower_name: "Bob",
          lent_at: "2026-03-01",
          returned_at: "2026-03-10",
        },
      ]),
      ...serviceHandlers.listForCommodity(SLUG, ID, []),
      ...supplyHandlers.listForCommodity(SLUG, ID, [
        {
          id: "supply-1",
          commodity_id: ID,
          label: "Charger",
          url: "https://example.com/charger",
          notes: "",
          sort_order: 0,
        },
      ]),
      ...maintenanceHandlers.listForCommodity(SLUG, ID, [
        {
          id: "ms-1",
          commodity_id: ID,
          title: "Clean the fans",
          interval_days: 90,
          next_due_at: "2027-01-01",
          enabled: true,
        },
      ])
    )
    renderDetail()

    expect(await screen.findByTestId("reminder-row-r1")).toHaveTextContent("Insurance renewal")
    expect(screen.queryByTestId("reminder-add")).not.toBeInTheDocument()
    expect(screen.queryByTestId("reminder-row-r1-edit")).not.toBeInTheDocument()
    expect(screen.queryByTestId("reminder-row-r1-delete")).not.toBeInTheDocument()

    await user.click(screen.getByTestId("commodity-detail-tab-lend"))
    expect(await screen.findByTestId("lend-history-row-loan-1")).toHaveTextContent("Bob")
    expect(screen.queryByTestId("commodity-detail-lend-button")).not.toBeInTheDocument()
    expect(screen.queryByTestId("lend-history-row-loan-1-edit")).not.toBeInTheDocument()

    await user.click(screen.getByTestId("commodity-detail-tab-service"))
    expect(await screen.findByTestId("service-empty-state")).toBeInTheDocument()
    expect(screen.queryByTestId("commodity-detail-service-button")).not.toBeInTheDocument()

    await user.click(screen.getByTestId("commodity-detail-tab-supplies"))
    expect(await screen.findByTestId("supplies-row-open")).toBeInTheDocument()
    expect(screen.queryByTestId("supplies-add")).not.toBeInTheDocument()
    expect(screen.queryByTestId("supplies-edit")).not.toBeInTheDocument()
    expect(screen.queryByTestId("supplies-delete")).not.toBeInTheDocument()

    await user.click(screen.getByTestId("commodity-detail-tab-maintenance"))
    expect(await screen.findByTestId("schedule-ms-1-next-due")).toBeInTheDocument()
    expect(screen.queryByTestId("maintenance-add")).not.toBeInTheDocument()
    expect(screen.queryByTestId("schedule-ms-1-done")).not.toBeInTheDocument()
    expect(screen.queryByTestId("schedule-ms-1-delete")).not.toBeInTheDocument()
    expect(screen.queryByRole("switch", { name: "Enabled" })).not.toBeInTheDocument()
  })

  it("offers a viewer neither Revert to In Use nor the prompt to assign a location", async () => {
    server.use(
      ...memberHandlers.asRole("g1", "viewer"),
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...locationHandlers.list(SLUG, [
        { id: "l1", type: "locations", attributes: { id: "l1", name: "Home" } },
      ]),
      ...commodityHandlers.detail(SLUG, ID, {
        ...commodityFixture,
        attributes: { ...commodityFixture.attributes, status: "sold", area_id: null },
      })
    )
    renderDetail()
    expect(await screen.findByTestId("commodity-detail-terminal-status")).toHaveTextContent(/Sold/i)
    expect(screen.queryByTestId("commodity-detail-revert-status")).not.toBeInTheDocument()
    expect(screen.queryByTestId("assign-location-banner")).not.toBeInTheDocument()
  })

  it("offers a user edit, delete, status change and quick edit", async () => {
    server.use(...memberHandlers.asRole("g1", "user"))
    serveItems([itemRow(ID, "MacBook Pro 16")])
    renderItemRoutes(`/g/${SLUG}/commodities/${ID}`)
    expect(await screen.findByTestId("commodity-detail-edit")).toBeInTheDocument()
    expect(screen.getByTestId("commodity-detail-delete")).toBeInTheDocument()
    expect(screen.getByTestId("commodity-detail-change-status")).toBeInTheDocument()
    expect(screen.getByTestId("commodity-quick-edit-notes")).toBeInTheDocument()
  })
})
