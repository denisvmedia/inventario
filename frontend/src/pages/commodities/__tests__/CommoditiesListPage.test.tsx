import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { CommoditiesListPage } from "@/pages/commodities/CommoditiesListPage"
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
import { pickRadixSelect } from "@/test/radix"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { formatDate } from "@/lib/intl"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

const areaFixture = [
  { id: "a1", type: "areas", attributes: { id: "a1", name: "Garage", location_id: "l1" } },
  { id: "a2", type: "areas", attributes: { id: "a2", name: "Kitchen", location_id: "l1" } },
]

function commodityRes(id: string, attrs: Record<string, unknown>) {
  return {
    id,
    type: "commodities",
    attributes: { id, count: 1, status: "in_use", type: "other", area_id: "a1", ...attrs },
  }
}

function renderList(initialPath = `/g/${SLUG}/commodities`) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path="/g/:groupSlug/commodities"
        element={
          <GroupProvider>
            <ConfirmProvider>
              <CommoditiesListPage />
            </ConfirmProvider>
          </GroupProvider>
        }
      />
    ),
  })
}

// settingsStub returns a GET /g/{slug}/settings handler with the given
// body. The CommoditiesListPage always mounts `useUserSettings()` once
// the group loads, and the suite's MSW server runs in
// `onUnhandledRequest: "error"` mode (frontend/src/test/setup.ts) — so
// every render path through this page needs a stub registered. The
// baseline below (empty SettingsObject) lets unrelated cases stay
// silent on this endpoint; the three preference-driven tests override
// it with their own bodies via `server.use(settingsStub({...}))`.
function settingsStub(body: Record<string, unknown> = {}) {
  return msw.get(apiUrl(`/g/${SLUG}/settings`), () => HttpResponse.json(body))
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  // The view-mode toggle persists to localStorage and overrides the
  // user's `appearance.default_items_view` preference. Wipe the cache
  // before each test so the default-view assertions land deterministically.
  localStorage.removeItem("commodities:viewMode")
  // Baseline /settings stub for every case. The page reads
  // `appearance.default_items_view` to compute the initial viewMode and
  // would otherwise issue an unhandled request the moment the group
  // context resolves. Tests that care about the value override this with
  // their own `server.use(settingsStub({...}))` later in the case.
  server.use(settingsStub())
})

describe("<CommoditiesListPage />", () => {
  it("renders the empty state when the group has no items", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [])
    )
    renderList()
    expect(await screen.findByTestId("commodities-empty")).toBeInTheDocument()
  })

  it("lists commodities returned from the BE", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [
        commodityRes("c1", { name: "MacBook Pro", short_name: "Laptop", current_price: 2400 }),
        commodityRes("c2", { name: "Coffee grinder", current_price: 200 }),
      ])
    )
    renderList()
    await waitFor(() => expect(screen.getAllByTestId("commodity-card").length).toBe(2))
    expect(screen.getByText("MacBook Pro")).toBeInTheDocument()
    expect(screen.getByText("Coffee grinder")).toBeInTheDocument()
  })

  it("shows the purchase date chip on grid cards and omits it when missing", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [
        commodityRes("c1", { name: "MacBook Pro", purchase_date: "2024-09-12" }),
        commodityRes("c2", { name: "Coffee grinder" }),
      ])
    )
    renderList()
    const cards = await screen.findAllByTestId("commodity-card")
    const withDate = cards.find((c) => c.getAttribute("data-commodity-id") === "c1")
    const withoutDate = cards.find((c) => c.getAttribute("data-commodity-id") === "c2")
    expect(withDate).toBeDefined()
    expect(withoutDate).toBeDefined()
    // Compute the expected rendered text via the same helper the
    // component uses, so the assertion isn't pinned to one runtime's
    // Intl/ICU output (en short-date varies across Node versions).
    const expectedDate = formatDate("2024-09-12", { style: "short" })
    const chip = within(withDate!).getByTestId("commodity-card-purchase-date")
    expect(chip).toHaveTextContent(expectedDate)
    expect(chip.getAttribute("aria-label")).toBe(`Purchased ${expectedDate}`)
    expect(within(withoutDate!).queryByTestId("commodity-card-purchase-date")).toBeNull()
  })

  it("renders the cover thumbnail when meta.covers is present and falls back to type icon otherwise", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(
        SLUG,
        [
          commodityRes("c1", { name: "MacBook Pro", type: "electronics" }),
          commodityRes("c2", { name: "Coffee grinder", type: "other" }),
        ],
        {
          c1: {
            file_id: "f1",
            thumbnails: { small: "https://example.test/c1.jpg" },
            source: "first_photo",
          },
        }
      )
    )
    renderList()
    await waitFor(() => expect(screen.getAllByTestId("commodity-card").length).toBe(2))
    // c1 has a cover → image renders with the alt text.
    const withCover = screen
      .getAllByTestId("commodity-card-thumb")
      .find((el) => el.getAttribute("data-state") === "image")
    expect(withCover).toBeTruthy()
    expect(
      within(withCover as HTMLElement)
        .getByRole("img")
        .getAttribute("alt")
    ).toBe("MacBook Pro")
    // c2 has no cover → fallback Lucide icon renders (#1392). The
    // slot exposes the type via `data-commodity-type` so the test does
    // not have to peek at the exact SVG glyph.
    const withoutCover = screen
      .getAllByTestId("commodity-card-thumb")
      .find((el) => el.getAttribute("data-state") === "fallback")
    expect(withoutCover).toBeTruthy()
    expect(withoutCover!.getAttribute("data-commodity-type")).toBe("other")
    expect((withoutCover as HTMLElement).querySelector("svg")).not.toBeNull()
  })

  it("toggles between grid and list view", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [commodityRes("c1", { name: "Item" })])
    )
    renderList()
    await screen.findByTestId("commodity-card")
    await user.click(screen.getByTestId("commodities-view-list"))
    await waitFor(() => expect(screen.getByTestId("commodities-table")).toBeInTheDocument())
    expect(screen.queryByTestId("commodities-grid")).not.toBeInTheDocument()
  })

  // #1643 acceptance: with no URL ?view= override and an empty
  // localStorage, the initial viewMode falls back to the user's
  // `appearance.default_items_view` preference. The Settings → Appearance
  // → Default view selector flips this; the Items list must respect it.
  it("uses preferences default_items_view='list' as the initial view mode", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [commodityRes("c1", { name: "Item" })]),
      // Overrides the empty-body baseline stub registered in beforeEach.
      settingsStub({ appearanceDefaultItemsView: "list" })
    )
    renderList()
    // The table renders without a manual toolbar click — the preference
    // pushed it past the grid default.
    await waitFor(() => expect(screen.getByTestId("commodities-table")).toBeInTheDocument())
    expect(screen.queryByTestId("commodities-grid")).not.toBeInTheDocument()
  })

  it("ignores an unrecognised default_items_view and stays on grid", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [commodityRes("c1", { name: "Item" })]),
      settingsStub({ appearanceDefaultItemsView: "garbage" })
    )
    renderList()
    // Validator rejects unknown values; the page should land on grid
    // rather than crash or render an empty table.
    await screen.findByTestId("commodity-card")
    expect(screen.getByTestId("commodities-grid")).toBeInTheDocument()
    expect(screen.queryByTestId("commodities-table")).not.toBeInTheDocument()
  })

  it("localStorage override wins over preferences default_items_view", async () => {
    // Per-device toolbar flips are cached under `commodities:viewMode`;
    // an explicit local choice must beat the synced server preference,
    // matching the comment in CommoditiesListPage about scoping flips to
    // one device.
    localStorage.setItem("commodities:viewMode", "grid")
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [commodityRes("c1", { name: "Item" })]),
      settingsStub({ appearanceDefaultItemsView: "list" })
    )
    renderList()
    await screen.findByTestId("commodity-card")
    expect(screen.getByTestId("commodities-grid")).toBeInTheDocument()
    expect(screen.queryByTestId("commodities-table")).not.toBeInTheDocument()
  })

  it("opens the bulk action bar when at least one row is selected", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [
        commodityRes("c1", { name: "First" }),
        commodityRes("c2", { name: "Second" }),
      ])
    )
    renderList()
    const cards = await screen.findAllByTestId("commodity-card")
    const checkbox = within(cards[0]).getByTestId("commodity-select")
    await user.click(checkbox)
    expect(await screen.findByTestId("commodities-bulk-bar")).toBeInTheDocument()
    expect(screen.getByText(/1 item selected/i)).toBeInTheDocument()
  })

  it("renders an error alert when the list endpoint 5xx", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.error(SLUG, 500)
    )
    renderList()
    expect(await screen.findByTestId("commodities-error")).toBeInTheDocument()
  })

  it("opens the create dialog when the Add Item button is clicked", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [])
    )
    renderList()
    await screen.findByTestId("commodities-empty")
    await user.click(screen.getByTestId("commodities-add-button"))
    // Create mode opens on the AI step first (see CommodityFormDialog
    // `initialStep`). Walk past it via "Fill manually" — the testid is
    // the same Next-button id we use on the form steps. Once we're on
    // Basics the numbered stepper renders with the `Form steps`
    // aria-label.
    await user.click(await screen.findByTestId("commodity-form-next"))
    expect(await screen.findByLabelText(/form steps/i)).toBeInTheDocument()
  })

  it("toggles include-inactive via the toolbar button", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [commodityRes("c1", { name: "Active" })])
    )
    renderList()
    await screen.findByTestId("commodity-card")
    const toggle = screen.getByTestId("commodities-toggle-inactive")
    expect(toggle).toHaveTextContent(/Hide inactive/i)
    await user.click(toggle)
    await waitFor(() => expect(toggle).toHaveTextContent(/Showing inactive/i))
  })

  it("opens the bulk-move dialog and lists target areas", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [commodityRes("c1", { name: "Item" })])
    )
    renderList()
    const card = await screen.findByTestId("commodity-card")
    await user.click(within(card).getByTestId("commodity-select"))
    await user.click(await screen.findByTestId("commodities-bulk-move"))
    // The dialog renders a <select> populated from the areas fixture.
    const select = await screen.findByTestId("bulk-move-area")
    expect(within(select as HTMLSelectElement).getByText(/Garage/i)).toBeInTheDocument()
    expect(within(select as HTMLSelectElement).getByText(/Kitchen/i)).toBeInTheDocument()
  })

  it("toggles a Type filter via the dropdown", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [commodityRes("c1", { name: "Chair", type: "furniture" })])
    )
    renderList()
    await screen.findByTestId("commodity-card")
    await user.click(screen.getByTestId("commodities-filter-type"))
    // Furniture is one of the static type options rendered with its
    // Lucide icon from COMMODITY_TYPE_ICONS (#1392).
    await user.click(await screen.findByRole("menuitemcheckbox", { name: /Furniture/i }))
    // After toggling, the badge counter on the trigger flips to 1.
    await waitFor(() => {
      expect(screen.getByTestId("commodities-filter-type")).toHaveTextContent("1")
    })
  })

  it("flips the sort field via the sort dropdown", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [commodityRes("c1", { name: "Chair" })])
    )
    renderList()
    await screen.findByTestId("commodity-card")
    await user.click(screen.getByTestId("commodities-sort"))
    await user.click(await screen.findByRole("menuitemcheckbox", { name: /Date added/i }))
    await waitFor(() => {
      expect(screen.getByTestId("commodities-sort")).toHaveTextContent(/Date added/i)
    })
  })

  it("toggles the Lent out chip and rounds-trips lent_out=true to the BE (#1510)", async () => {
    const user = userEvent.setup()
    const queries: string[] = []
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      msw.get(apiUrl(`/g/${SLUG}/commodities`), ({ request }) => {
        queries.push(new URL(request.url).search)
        return HttpResponse.json({ data: [commodityRes("c1", { name: "Item" })] })
      })
    )
    renderList()
    await screen.findByTestId("commodity-card")
    const chip = screen.getByTestId("commodities-filter-lent-out")
    expect(chip).toHaveAttribute("aria-pressed", "false")

    await user.click(chip)
    await waitFor(() => {
      expect(chip).toHaveAttribute("aria-pressed", "true")
    })
    await waitFor(() => {
      expect(queries.some((q) => /[?&]lent_out=true\b/.test(q))).toBe(true)
    })

    // The Clear-filters button surfaces because `lent_out` flips
    // hasFilters on. Click it and verify the chip un-presses AND the
    // next BE request omits the lent_out param entirely.
    await user.click(screen.getByTestId("commodities-clear-filters"))
    await waitFor(() => {
      expect(chip).toHaveAttribute("aria-pressed", "false")
    })
    await waitFor(() => {
      // At least one request after the clear must NOT include lent_out.
      const tail = queries[queries.length - 1] ?? ""
      expect(/[?&]lent_out=/.test(tail)).toBe(false)
    })
  })

  it("clears all active filters via Clear filters", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [commodityRes("c1", { name: "Chair", type: "furniture" })])
    )
    renderList()
    await screen.findByTestId("commodity-card")
    await user.click(screen.getByTestId("commodities-filter-type"))
    await user.click(await screen.findByRole("menuitemcheckbox", { name: /Furniture/i }))
    expect(await screen.findByTestId("commodities-clear-filters")).toBeInTheDocument()
    await user.click(screen.getByTestId("commodities-clear-filters"))
    await waitFor(() => {
      expect(screen.queryByTestId("commodities-clear-filters")).not.toBeInTheDocument()
    })
  })

  it("confirms a bulk delete and clears the selection", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [
        commodityRes("c1", { name: "First" }),
        commodityRes("c2", { name: "Second" }),
      ]),
      ...commodityHandlers.bulkDelete(SLUG)
    )
    renderList()
    const cards = await screen.findAllByTestId("commodity-card")
    await user.click(within(cards[0]).getByTestId("commodity-select"))
    await user.click(within(cards[1]).getByTestId("commodity-select"))
    await user.click(await screen.findByTestId("commodities-bulk-delete"))
    // ConfirmProvider mounts a generic Dialog (role="dialog"). Use its
    // body-scrolllock signature to wait for it before clicking Delete.
    await waitFor(() => expect(document.body).toHaveAttribute("data-scroll-locked"))
    const buttons = screen.getAllByRole("button", { name: /^Delete$/i })
    // Pick the dialog's primary action: it's the only button with that
    // exact name inside the confirm dialog (the bulk-bar button has a
    // different label rendered).
    await user.click(buttons[buttons.length - 1])
    await waitFor(() =>
      expect(screen.queryByTestId("commodities-bulk-bar")).not.toBeInTheDocument()
    )
    await waitFor(() =>
      expect(screen.queryByTestId("commodities-bulk-bar")).not.toBeInTheDocument()
    )
  })

  it("submits a new item via the create dialog and clears the form", async () => {
    // Replays the regression that motivated #1629: walk past the
    // initial AI placeholder step, fill Basics (Name + Short name +
    // Type + Location → Area), skip Save-as-draft so the schema's
    // whenNotDraft fields stay optional, walk Purchase → Warranty →
    // Extras → Files, then submit. After 201 the dialog closes and
    // the page navigates to /commodities/:newId.
    const user = userEvent.setup()
    const requests: Record<string, unknown>[] = []
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...locationHandlers.list(SLUG, [
        { id: "l1", type: "locations", attributes: { id: "l1", name: "Home" } },
      ]),
      ...commodityHandlers.list(SLUG, []),
      // Intercept the POST manually so the test can assert the
      // mapped-value payload Radix Select drives through the form.
      msw.post(apiUrl(`/g/${SLUG}/commodities`), async ({ request }) => {
        requests.push((await request.json()) as Record<string, unknown>)
        return HttpResponse.json(
          {
            data: {
              id: "new-1",
              type: "commodities",
              attributes: {
                id: "new-1",
                name: "Couch",
                short_name: "Couch",
                type: "furniture",
                area_id: "a1",
                status: "in_use",
                count: 1,
                draft: true,
              },
            },
          },
          { status: 201 }
        )
      })
    )
    function NavSentinel() {
      const location = useLocation()
      return <span data-testid="nav-sentinel-path">{location.pathname}</span>
    }
    setAccessToken("good-token")
    renderWithProviders({
      initialPath: `/g/${SLUG}/commodities`,
      routes: (
        <>
          <Route
            path="/g/:groupSlug/commodities"
            element={
              <GroupProvider>
                <ConfirmProvider>
                  <CommoditiesListPage />
                </ConfirmProvider>
              </GroupProvider>
            }
          />
          <Route path="/g/:groupSlug/commodities/:id" element={<NavSentinel />} />
        </>
      ),
    })
    await screen.findByTestId("commodities-empty")
    await user.click(screen.getByTestId("commodities-add-button"))
    // Past the AI placeholder step → Basics.
    await user.click(await screen.findByTestId("commodity-form-next"))
    await user.type(await screen.findByLabelText(/^Name$/i), "Couch")
    await user.type(screen.getByLabelText(/^Short name$/i), "Couch")
    await pickRadixSelect(user, /^Type$/i, { optionLabel: /^Furniture$/i })
    await pickRadixSelect(user, /^Location$/i, { optionLabel: /^Home$/i })
    await pickRadixSelect(user, /^Area$/i, { optionLabel: /^Garage$/i })
    // Save-as-draft so purchase_date + price triad stay optional —
    // the test asserts the navigation contract, not the schema.
    await user.click(screen.getByLabelText(/Save as draft/i))
    await user.click(screen.getByTestId("commodity-form-next"))
    // Walk Purchase → Warranty → Extras → Files.
    await screen.findByLabelText(/Purchase date/i)
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-form-warranty-step")
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-tags-input")
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-form-submit")
    await user.click(screen.getByTestId("commodity-form-submit"))
    // Page navigates to the new item's detail route — the sentinel
    // resolves once react-router has replaced the URL.
    expect(await screen.findByTestId("nav-sentinel-path")).toHaveTextContent(
      `/g/${SLUG}/commodities/new-1`
    )
    // The mapped payload landed on the BE — anchor on the schema
    // fields the dialog test pins (#1629 success criterion). The
    // hook wraps the payload in a JSON:API envelope.
    expect(requests).toHaveLength(1)
    expect(requests[0]).toMatchObject({
      data: {
        type: "commodities",
        attributes: {
          name: "Couch",
          short_name: "Couch",
          type: "furniture",
          area_id: "a1",
          status: "in_use",
          draft: true,
        },
      },
    })
  })

  it("filters rows by warranty status (derived from warranty_expires_at)", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [
        commodityRes("c1", { name: "Chair", warranty_expires_at: "2099-12-31" }),
        commodityRes("c2", { name: "Couch" }),
      ])
    )
    renderList()
    await waitFor(() => expect(screen.getAllByTestId("commodity-card").length).toBe(2))
    await user.click(screen.getByTestId("commodities-filter-warranty"))
    await user.click(await screen.findByRole("menuitemcheckbox", { name: /Active/i }))
    // Only the row with a tracked warranty remains.
    await waitFor(() => {
      const rows = screen.queryAllByTestId("commodity-card")
      expect(rows).toHaveLength(1)
      expect(rows[0]).toHaveTextContent(/Chair/)
    })
  })

  it("renders WarrantyBadge with status derived from warranty_expires_at on grid cards (#1657)", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [
        commodityRes("c1", { name: "Active", warranty_expires_at: "2099-12-31" }),
        commodityRes("c2", { name: "None" }),
      ])
    )
    renderList()
    const cards = await screen.findAllByTestId("commodity-card")
    const active = cards.find((c) => c.getAttribute("data-commodity-id") === "c1")
    const none = cards.find((c) => c.getAttribute("data-commodity-id") === "c2")
    expect(within(active!).getByTestId("commodity-card-warranty")).toHaveAttribute(
      "data-status",
      "active"
    )
    expect(within(none!).getByTestId("commodity-card-warranty")).toHaveAttribute(
      "data-status",
      "none"
    )
  })

  it("renders the purchase date in the secondary metadata strip (#1657)", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [
        commodityRes("c1", { name: "MacBook Pro", purchase_date: "2024-09-12" }),
      ])
    )
    renderList()
    const card = await screen.findByTestId("commodity-card")
    // The chip lives in the secondary slot below the title — it is
    // structurally inside the same card as (and a sibling of) the
    // currency. Asserting both render inside the card is the closest
    // we can get to "below the title" without coupling to layout
    // class names.
    const dateChip = within(card).getByTestId("commodity-card-purchase-date")
    expect(dateChip).toBeInTheDocument()
    // The warranty pill should sit in the top-right cluster — *not*
    // the same node that hosts the purchase date.
    expect(within(card).getByTestId("commodity-card-warranty")).toBeInTheDocument()
    expect(dateChip).not.toContainElement(within(card).getByTestId("commodity-card-warranty"))
  })

  it("toggling the row checkbox does not trigger navigation (#1657)", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [commodityRes("c1", { name: "Item" })])
    )
    // Mount a sentinel under /commodities/:id so any rogue navigation
    // surfaces as the sentinel testid. The interactive Checkbox sits
    // ABOVE the absolute overlay <Link> by being explicitly marked
    // `pointer-events-auto` — clicking it must not bubble to the
    // overlay or change the URL.
    setAccessToken("good-token")
    function NavSentinel() {
      const location = useLocation()
      return <span data-testid="nav-sentinel-path">{location.pathname}</span>
    }
    renderWithProviders({
      initialPath: `/g/${SLUG}/commodities`,
      routes: (
        <>
          <Route
            path="/g/:groupSlug/commodities"
            element={
              <GroupProvider>
                <ConfirmProvider>
                  <CommoditiesListPage />
                </ConfirmProvider>
              </GroupProvider>
            }
          />
          <Route path="/g/:groupSlug/commodities/:id" element={<NavSentinel />} />
        </>
      ),
    })
    const card = await screen.findByTestId("commodity-card")
    await user.click(within(card).getByTestId("commodity-select"))
    // Bulk action bar appears (proof the click registered on the
    // checkbox) and we're still on the list, not the sentinel.
    expect(await screen.findByTestId("commodities-bulk-bar")).toBeInTheDocument()
    expect(screen.queryByTestId("nav-sentinel-path")).not.toBeInTheDocument()
  })

  it("navigates to /commodities/:id with state.background on a bare card click (#1546)", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [
        commodityRes("c1", {
          name: "MacBook Pro",
          short_name: "Laptop",
          current_price: 1900,
          original_price: 2400,
          tags: ["work"],
        }),
      ])
    )
    // Mount the list under its real path, plus a sentinel route at
    // /commodities/:id so the navigation lands somewhere we can
    // inspect. The sentinel reads useLocation and emits the
    // pathname + the captured `state.background.pathname` so the
    // test can assert both the URL flip and the modal-routes
    // backdrop wiring without spinning up the full <AppRoutes>.
    setAccessToken("good-token")
    function NavSentinel() {
      const location = useLocation()
      const background = (location.state as { background?: { pathname: string } } | null)
        ?.background
      return (
        <div>
          <span data-testid="nav-sentinel-path">{location.pathname}</span>
          <span data-testid="nav-sentinel-background">{background?.pathname ?? ""}</span>
        </div>
      )
    }
    renderWithProviders({
      initialPath: `/g/${SLUG}/commodities`,
      routes: (
        <>
          <Route
            path="/g/:groupSlug/commodities"
            element={
              <GroupProvider>
                <ConfirmProvider>
                  <CommoditiesListPage />
                </ConfirmProvider>
              </GroupProvider>
            }
          />
          <Route path="/g/:groupSlug/commodities/:id" element={<NavSentinel />} />
        </>
      ),
    })
    const card = await screen.findByTestId("commodity-card")
    await user.click(within(card).getByTestId("commodity-card-link"))
    expect(await screen.findByTestId("nav-sentinel-path")).toHaveTextContent(
      `/g/${SLUG}/commodities/c1`
    )
    // The list URL is stamped on `state.background` so the modal
    // routes tree in app/router.tsx can render the list backdrop
    // behind the sheet.
    expect(screen.getByTestId("nav-sentinel-background")).toHaveTextContent(
      `/g/${SLUG}/commodities`
    )
  })

  it("has no axe violations on the empty state", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, [])
    )
    const { container } = renderList()
    await screen.findByTestId("commodities-empty")
    expect(await axe(container)).toHaveNoViolations()
  })
})
