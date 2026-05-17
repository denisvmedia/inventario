import { beforeEach, describe, expect, it } from "vitest"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { CommodityDetailPage, CommodityDetailSheet } from "@/pages/commodities/CommodityDetailPage"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { areaHandlers, commodityHandlers, fileHandlers, groupHandlers } from "@/test/handlers"
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

  it("opens the edit dialog when the Edit button is clicked", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderDetail()
    await screen.findByTestId("commodity-detail-edit")
    await user.click(screen.getByTestId("commodity-detail-edit"))
    // Form stepper aria-label is dialog-only.
    expect(await screen.findByLabelText(/form steps/i)).toBeInTheDocument()
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

  it("auto-opens the edit dialog when /commodities/:id/edit is the entry URL", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    setAccessToken("good-token")
    renderWithProviders({
      initialPath: `/g/${SLUG}/commodities/${ID}/edit`,
      routes: (
        <Route
          path="/g/:groupSlug/commodities/:id/edit"
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
    expect(await screen.findByLabelText(/form steps/i)).toBeInTheDocument()
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
      // @ts-expect-error partial init is intentional
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
    expect(screen.getByTestId("commodity-detail-terminal-status-date")).toHaveTextContent(
      /2026/
    )
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
    let capturedBody: Record<string, unknown> | null = null
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
          capturedBody = json.data?.attributes ?? null
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
    await waitFor(() => expect(capturedBody).not.toBeNull())
    expect(capturedBody?.status).toBe("sold")
    expect(capturedBody?.sale_price).toBe(150)
    expect(String(capturedBody?.status_date ?? "")).toMatch(/^\d{4}-\d{2}-\d{2}$/)
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
      return <div data-testid="list-backdrop">{location.pathname}</div>
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

  it("supports the same `?tab=warranty` deep-link as the page variant", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderSheet({ search: "?tab=warranty" })
    // Warranty tab content is what the dashboard expiring-row +
    // warranty-list deep-links target — it must work in sheet mode.
    expect(await screen.findByTestId("commodity-detail-warranty")).toBeInTheDocument()
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
