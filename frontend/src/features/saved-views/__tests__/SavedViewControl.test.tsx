import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

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
  savedViewHandlers,
  sellerHandlers,
  tagHandlers,
} from "@/test/handlers"
import type { SavedViewFixture } from "@/test/handlers/savedViews"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const SLUG = "household"

const areaFixture = [
  { id: "a1", type: "areas", attributes: { id: "a1", name: "Garage", location_id: "l1" } },
]

const SOLD: SavedViewFixture = {
  id: "sv1",
  name: "Sold things",
  query: "include_inactive=false&sort=name&status=sold",
}

function LocationProbe() {
  const location = useLocation()
  return <span data-testid="location-search">{location.search}</span>
}

function renderList(search: string) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: `/g/${SLUG}/commodities${search}`,
    routes: (
      <Route
        path="/g/:groupSlug/commodities"
        element={
          <GroupProvider>
            <ConfirmProvider>
              <CommoditiesListPage />
              <LocationProbe />
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
  localStorage.removeItem("commodities:viewMode")
  server.use(
    msw.get(apiUrl(`/g/${SLUG}/settings`), () => HttpResponse.json({})),
    ...groupHandlers.list([{ id: "g1", slug: SLUG, name: "Household" }]),
    ...areaHandlers.list(SLUG, areaFixture),
    ...tagHandlers.list(SLUG, [{ id: "t1", slug: "garage", label: "Garage", color: "blue" }]),
    ...sellerHandlers.list(SLUG, [{ id: "s1", name: "Hardware shop" }]),
    ...commodityHandlers.list(SLUG, [])
  )
})

describe("<SavedViewControl /> on the items list", () => {
  it("saves the current filter, sort and layout as a view and opens it", async () => {
    const user = userEvent.setup()
    let sent: Record<string, unknown> | undefined
    server.use(
      ...savedViewHandlers.none(SLUG),
      ...savedViewHandlers.create(SLUG, "sv9", (attrs) => {
        sent = attrs
      })
    )
    renderList("?status=sold&view=list&sort=-purchase_date")

    await user.click(await screen.findByTestId("saved-views-menu"))
    await user.click(await screen.findByTestId("saved-views-save-new"))
    const dialog = await screen.findByTestId("saved-view-dialog")
    await user.type(within(dialog).getByTestId("saved-view-name"), "  Sold, newest first ")
    await user.click(within(dialog).getByTestId("saved-view-dashboard"))
    await user.click(within(dialog).getByTestId("saved-view-submit"))

    await waitFor(() => expect(sent).toBeDefined())
    expect(sent).toMatchObject({
      name: "Sold, newest first",
      query: "status=sold&include_inactive=false&sort=-purchase_date",
      layout: "list",
      shared: false,
      show_in_sidebar: true,
      show_on_dashboard: true,
    })
    await waitFor(() =>
      expect(screen.getByTestId("location-search")).toHaveTextContent("saved_view=sv9")
    )
  })

  it("asks for a name before saving", async () => {
    const user = userEvent.setup()
    server.use(...savedViewHandlers.none(SLUG))
    renderList("")

    await user.click(await screen.findByTestId("saved-views-menu"))
    await user.click(await screen.findByTestId("saved-views-save-new"))
    const dialog = await screen.findByTestId("saved-view-dialog")
    await user.click(within(dialog).getByTestId("saved-view-submit"))
    const error = await within(dialog).findByTestId("saved-view-name-error")
    expect(error).toHaveTextContent("Enter a name.")
    // The input names the message, so a screen reader reads it with the field.
    expect(within(dialog).getByTestId("saved-view-name")).toHaveAttribute(
      "aria-describedby",
      error.id
    )
  })

  it("chooses the icon with the arrow keys and takes one tab stop for it", async () => {
    const user = userEvent.setup()
    let sent: Record<string, unknown> | undefined
    server.use(
      ...savedViewHandlers.none(SLUG),
      ...savedViewHandlers.create(SLUG, "sv9", (attrs) => {
        sent = attrs
      })
    )
    renderList("")

    await user.click(await screen.findByTestId("saved-views-menu"))
    await user.click(await screen.findByTestId("saved-views-save-new"))
    const dialog = await screen.findByTestId("saved-view-dialog")
    await user.type(within(dialog).getByTestId("saved-view-name"), "Starred")
    await user.tab()
    const bookmark = within(dialog).getByRole("radio", { name: "Bookmark" })
    expect(bookmark).toHaveFocus()
    // Held down as a real key press is: the radio group moves the focus
    // after the keydown and checks the radio while the arrow key is down.
    await user.keyboard("{ArrowRight>}")
    const star = within(dialog).getByRole("radio", { name: "Star" })
    await waitFor(() => expect(star).toHaveAttribute("aria-checked", "true"))
    await user.keyboard("{/ArrowRight}")
    expect(star).toHaveFocus()
    expect(bookmark).toHaveAttribute("aria-checked", "false")
    // Tab leaves the group: the next stop is the first placement box.
    await user.tab()
    expect(within(dialog).getByTestId("saved-view-sidebar")).toHaveFocus()

    await user.click(within(dialog).getByTestId("saved-view-submit"))
    await waitFor(() => expect(sent).toBeDefined())
    expect(sent).toMatchObject({ name: "Starred", icon: "star" })
  })

  it("shows the server's reason when a filter cannot be kept", async () => {
    const user = userEvent.setup()
    server.use(
      ...savedViewHandlers.none(SLUG),
      msw.post(apiUrl(`/g/${SLUG}/saved-views`), () =>
        HttpResponse.json(
          {
            errors: [
              {
                status: "Unprocessable Entity",
                code: "query.invalid_parameter",
                meta: { parameter: "lent_out", reason: "is not a boolean" },
              },
            ],
          },
          { status: 422 }
        )
      )
    )
    renderList("?lent_out=maybe")

    await user.click(await screen.findByTestId("saved-views-menu"))
    await user.click(await screen.findByTestId("saved-views-save-new"))
    const dialog = await screen.findByTestId("saved-view-dialog")
    await user.type(within(dialog).getByTestId("saved-view-name"), "Odd")
    await user.click(within(dialog).getByTestId("saved-view-submit"))
    expect(await within(dialog).findByTestId("saved-view-error")).toHaveTextContent("lent_out")
  })

  it("names the open view and shows no change while the list matches it", async () => {
    server.use(...savedViewHandlers.list(SLUG, [SOLD]), ...savedViewHandlers.counts(SLUG))
    renderList("?status=sold&saved_view=sv1")

    await waitFor(() =>
      expect(screen.getByTestId("saved-views-menu-label")).toHaveTextContent("Sold things")
    )
    expect(screen.queryByTestId("saved-views-changed")).not.toBeInTheDocument()
  })

  it("marks a changed view and saves the change into it", async () => {
    const user = userEvent.setup()
    let sent: Record<string, unknown> | undefined
    server.use(
      ...savedViewHandlers.list(SLUG, [SOLD]),
      ...savedViewHandlers.update(SLUG, SOLD, (attrs) => {
        sent = attrs
      })
    )
    renderList("?status=sold&status=lost&saved_view=sv1")

    expect(await screen.findByTestId("saved-views-changed")).toBeInTheDocument()
    await user.click(screen.getByTestId("saved-views-menu"))
    await user.click(await screen.findByTestId("saved-views-save-changes"))

    await waitFor(() => expect(sent).toBeDefined())
    expect(sent).toEqual({
      query: "status=lost&status=sold&include_inactive=false&sort=name",
      layout: "grid",
    })
  })

  // The stored query may have been rewritten since the views loaded (a tag
  // rename); a save that changes only the layout leaves it alone.
  it("saves a layout-only change without the query", async () => {
    const user = userEvent.setup()
    let sent: Record<string, unknown> | undefined
    const gridView: SavedViewFixture = { ...SOLD, layout: "grid" }
    server.use(
      ...savedViewHandlers.list(SLUG, [gridView]),
      ...savedViewHandlers.update(SLUG, gridView, (attrs) => {
        sent = attrs
      })
    )
    renderList("?status=sold&view=list&saved_view=sv1")

    expect(await screen.findByTestId("saved-views-changed")).toBeInTheDocument()
    await user.click(screen.getByTestId("saved-views-menu"))
    await user.click(await screen.findByTestId("saved-views-save-changes"))

    await waitFor(() => expect(sent).toBeDefined())
    expect(sent).toEqual({ layout: "list" })
  })

  it("reverts a changed view to what it keeps", async () => {
    const user = userEvent.setup()
    server.use(...savedViewHandlers.list(SLUG, [SOLD]))
    renderList("?status=lost&saved_view=sv1")

    await user.click(await screen.findByTestId("saved-views-menu"))
    await user.click(await screen.findByTestId("saved-views-revert"))
    await waitFor(() =>
      expect(screen.getByTestId("location-search")).toHaveTextContent("status=sold")
    )
    expect(screen.getByTestId("location-search")).not.toHaveTextContent("status=lost")
    expect(screen.queryByTestId("saved-views-changed")).not.toBeInTheDocument()
  })

  it("offers no save on a view the member may not change", async () => {
    const user = userEvent.setup()
    server.use(...savedViewHandlers.list(SLUG, [{ ...SOLD, owned: false, can_edit: false }]))
    renderList("?status=lost&saved_view=sv1")

    await user.click(await screen.findByTestId("saved-views-menu"))
    expect(await screen.findByTestId("saved-views-revert")).toBeInTheDocument()
    expect(screen.queryByTestId("saved-views-save-changes")).not.toBeInTheDocument()
    expect(screen.getByTestId("saved-views-save-new")).toHaveTextContent("Save as new view")
  })

  it("names the deleted area and tag an open view still filters by", async () => {
    const user = userEvent.setup()
    const stale: SavedViewFixture = {
      id: "sv2",
      name: "Gone",
      query: "area_id=deleted-area&include_inactive=false&sort=name&tags=garage,gone-tag",
    }
    server.use(...savedViewHandlers.list(SLUG, [stale]))
    renderList("?area=deleted-area&tags=garage,gone-tag&saved_view=sv2")

    await user.click(await screen.findByTestId("saved-views-menu"))
    const notice = await screen.findByTestId("saved-views-stale")
    expect(notice).toHaveTextContent("The area of this view was deleted.")
    expect(notice).toHaveTextContent("Tag gone-tag no longer exists.")
    expect(notice).not.toHaveTextContent("garage")
  })

  it("names a deleted seller an open view still filters by, and only once the sellers loaded", async () => {
    const user = userEvent.setup()
    const shops: SavedViewFixture = {
      id: "sv6",
      name: "Shops",
      query: "include_inactive=false&seller_id=s1&seller_id=s9&sort=name",
    }
    server.use(...savedViewHandlers.list(SLUG, [shops]))
    renderList("?seller=s1&seller=s9&saved_view=sv6")

    await user.click(await screen.findByTestId("saved-views-menu"))
    const notice = await screen.findByTestId("saved-views-stale")
    expect(notice).toHaveTextContent("A seller of this view was deleted.")
    // The view's own filter is not a change.
    expect(screen.queryByTestId("saved-views-changed")).not.toBeInTheDocument()
  })

  it("names an item type an open view still filters by that the group no longer has", async () => {
    const user = userEvent.setup()
    const kinds: SavedViewFixture = {
      id: "sv7",
      name: "Boats",
      query: "include_inactive=false&sort=name&type=boat",
    }
    server.use(...savedViewHandlers.list(SLUG, [kinds]))
    renderList("?type=boat&saved_view=sv7")

    await user.click(await screen.findByTestId("saved-views-menu"))
    const notice = await screen.findByTestId("saved-views-stale")
    expect(notice).toHaveTextContent("An item type of this view was deleted.")
    expect(screen.queryByTestId("saved-views-changed")).not.toBeInTheDocument()
  })

  it("names no deleted item type while the types fail to load", async () => {
    const user = userEvent.setup()
    let typeRequests = 0
    server.use(
      ...savedViewHandlers.list(SLUG, [
        { id: "sv7", name: "Boats", query: "include_inactive=false&sort=name&type=boat" },
      ]),
      msw.get(apiUrl(`/g/${SLUG}/item-types`), () => {
        typeRequests++
        return HttpResponse.json({ errors: [{ status: "Internal Server Error" }] }, { status: 500 })
      })
    )
    renderList("?type=boat&saved_view=sv7")

    await waitFor(() =>
      expect(screen.getByTestId("saved-views-menu-label")).toHaveTextContent("Boats")
    )
    await waitFor(() => expect(typeRequests).toBeGreaterThan(0))
    await user.click(screen.getByTestId("saved-views-menu"))
    await screen.findByTestId("saved-views-close")
    expect(screen.queryByTestId("saved-views-stale")).not.toBeInTheDocument()
  })

  it("names no deleted seller while the sellers fail to load", async () => {
    const user = userEvent.setup()
    let sellerRequests = 0
    server.use(
      ...savedViewHandlers.list(SLUG, [
        { id: "sv6", name: "Shops", query: "include_inactive=false&seller_id=s9&sort=name" },
      ]),
      msw.get(apiUrl(`/g/${SLUG}/sellers`), () => {
        sellerRequests++
        return HttpResponse.json({ errors: [{ status: "Internal Server Error" }] }, { status: 500 })
      })
    )
    renderList("?seller=s9&saved_view=sv6")

    await waitFor(() =>
      expect(screen.getByTestId("saved-views-menu-label")).toHaveTextContent("Shops")
    )
    await waitFor(() => expect(sellerRequests).toBeGreaterThan(0))
    await user.click(screen.getByTestId("saved-views-menu"))
    await screen.findByTestId("saved-views-close")
    expect(screen.queryByTestId("saved-views-stale")).not.toBeInTheDocument()
  })

  it("keeps a view with an unsupported filter out of reach", async () => {
    const user = userEvent.setup()
    server.use(
      ...savedViewHandlers.list(SLUG, [
        {
          id: "sv3",
          name: "Old",
          query: "warranty_expires_before=2020-01-01",
          query_error: { parameter: "warranty_expires_before", reason: "is not available" },
        },
      ])
    )
    renderList("")

    await user.click(await screen.findByTestId("saved-views-menu"))
    const item = await screen.findByTestId("saved-views-item")
    expect(item).toHaveAttribute("aria-disabled", "true")
  })

  // The URL of a view whose query cannot run holds only the part the list
  // can read (an old bookmark, a view that went bad later), so running it
  // would show more items than the view does.
  it("does not run the list of a view whose stored query cannot run", async () => {
    const user = userEvent.setup()
    let listRequests = 0
    server.use(
      ...savedViewHandlers.list(SLUG, [
        {
          id: "sv3",
          name: "Old",
          query: "type=electronics&warranty_expires_before=2026-12-01",
          query_error: { parameter: "warranty_expires_before", reason: "is not available" },
        },
      ]),
      msw.get(apiUrl(`/g/${SLUG}/commodities`), () => {
        listRequests++
        return HttpResponse.json({ data: [] })
      })
    )
    renderList("?type=electronics&status=lost&saved_view=sv3")

    const alert = await screen.findByTestId("saved-view-query-error-alert")
    expect(alert).toHaveTextContent("This view cannot be opened")
    expect(alert).toHaveTextContent("warranty_expires_before")
    expect(listRequests).toBe(0)

    // The menu explains it too and offers neither Revert nor Save changes,
    // only Close view.
    await user.click(screen.getByTestId("saved-views-menu"))
    expect(await screen.findByTestId("saved-views-query-error")).toHaveTextContent(
      "warranty_expires_before"
    )
    expect(screen.queryByTestId("saved-views-revert")).not.toBeInTheDocument()
    expect(screen.queryByTestId("saved-views-save-changes")).not.toBeInTheDocument()
    expect(screen.getByTestId("saved-views-close")).toBeInTheDocument()
    await user.keyboard("{Escape}")

    // Closed, the list shows its own filters as an ordinary list.
    await user.click(within(alert).getByRole("button", { name: "Close view" }))
    await waitFor(() =>
      expect(screen.getByTestId("location-search")).not.toHaveTextContent("saved_view")
    )
    await waitFor(() => expect(listRequests).toBeGreaterThan(0))
    expect(screen.queryByTestId("saved-view-query-error-alert")).not.toBeInTheDocument()
  })

  // A filter this client does not know yet (one a newer server added)
  // would be dropped on the way into the list, which would then show more
  // than the view.
  it("does not open a view whose stored query the list cannot read in full", async () => {
    const user = userEvent.setup()
    let listRequests = 0
    server.use(
      ...savedViewHandlers.list(SLUG, [
        { id: "sv5", name: "Newer", query: "include_inactive=false&item_type=abc&sort=name" },
      ]),
      msw.get(apiUrl(`/g/${SLUG}/commodities`), () => {
        listRequests++
        return HttpResponse.json({ data: [] })
      })
    )
    renderList("?saved_view=sv5")

    const alert = await screen.findByTestId("saved-view-query-error-alert")
    expect(alert).toHaveTextContent("item_type")
    expect(listRequests).toBe(0)
    await user.click(screen.getByTestId("saved-views-menu"))
    const item = await screen.findByTestId("saved-views-item")
    expect(item).toHaveAttribute("aria-disabled", "true")
  })

  // Without the saved views the list cannot tell whether the view it was
  // opened from can run, so it waits rather than run the URL's filters.
  it("does not run the list while the saved views fail to load", async () => {
    const user = userEvent.setup()
    let listRequests = 0
    let failing = true
    server.use(
      msw.get(apiUrl(`/g/${SLUG}/saved-views`), () =>
        failing
          ? HttpResponse.json({ errors: [{ status: "Internal Server Error" }] }, { status: 500 })
          : HttpResponse.json({
              data: [
                {
                  id: "sv1",
                  type: "saved_views",
                  attributes: { name: "Sold things", query: SOLD.query, query_version: 1 },
                },
              ],
              meta: { total: 1, limits: {} },
            })
      ),
      msw.get(apiUrl(`/g/${SLUG}/commodities`), () => {
        listRequests++
        return HttpResponse.json({ data: [] })
      })
    )
    renderList("?status=sold&saved_view=sv1")

    const alert = await screen.findByTestId("saved-view-unavailable-alert")
    expect(alert).toHaveTextContent("This view could not be loaded")
    expect(listRequests).toBe(0)

    failing = false
    await user.click(within(alert).getByRole("button", { name: "Try again" }))
    await waitFor(() =>
      expect(screen.queryByTestId("saved-view-unavailable-alert")).not.toBeInTheDocument()
    )
    await waitFor(() => expect(listRequests).toBeGreaterThan(0))
    expect(screen.getByTestId("saved-views-menu-label")).toHaveTextContent("Sold things")
  })

  it("closes a view whose saved views fail to load and shows the list", async () => {
    const user = userEvent.setup()
    let listRequests = 0
    server.use(
      msw.get(apiUrl(`/g/${SLUG}/saved-views`), () =>
        HttpResponse.json({ errors: [{ status: "Internal Server Error" }] }, { status: 500 })
      ),
      msw.get(apiUrl(`/g/${SLUG}/commodities`), () => {
        listRequests++
        return HttpResponse.json({ data: [] })
      })
    )
    renderList("?status=sold&saved_view=sv1")

    const alert = await screen.findByTestId("saved-view-unavailable-alert")
    await user.click(within(alert).getByRole("button", { name: "Close view" }))
    await waitFor(() =>
      expect(screen.getByTestId("location-search")).not.toHaveTextContent("saved_view")
    )
    expect(screen.getByTestId("location-search")).toHaveTextContent("status=sold")
    await waitFor(() => expect(listRequests).toBeGreaterThan(0))
  })

  it("names no deleted tag while the tag catalogue fails to load", async () => {
    const user = userEvent.setup()
    let tagRequests = 0
    server.use(
      ...savedViewHandlers.list(SLUG, [
        { id: "sv2", name: "Garage", query: "include_inactive=false&sort=name&tags=garage" },
      ]),
      msw.get(apiUrl(`/g/${SLUG}/tags`), () => {
        tagRequests++
        return HttpResponse.json({ errors: [{ status: "Internal Server Error" }] }, { status: 500 })
      })
    )
    renderList("?tags=garage&saved_view=sv2")

    await waitFor(() =>
      expect(screen.getByTestId("saved-views-menu-label")).toHaveTextContent("Garage")
    )
    await waitFor(() => expect(tagRequests).toBeGreaterThan(0))
    await user.click(screen.getByTestId("saved-views-menu"))
    await screen.findByTestId("saved-views-close")
    expect(screen.queryByTestId("saved-views-stale")).not.toBeInTheDocument()
  })

  it("shows no change for a view whose tags differ from the list's only in case", async () => {
    server.use(
      ...savedViewHandlers.list(SLUG, [
        { id: "sv4", name: "Garage", query: "include_inactive=false&sort=name&tags=garage" },
      ])
    )
    renderList("?tags=Garage&saved_view=sv4")

    await waitFor(() =>
      expect(screen.getByTestId("saved-views-menu-label")).toHaveTextContent("Garage")
    )
    expect(screen.queryByTestId("saved-views-changed")).not.toBeInTheDocument()
  })
})
