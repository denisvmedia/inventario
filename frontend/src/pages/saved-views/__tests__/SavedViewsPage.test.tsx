import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { SavedViewsPage } from "@/pages/saved-views/SavedViewsPage"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import {
  apiUrl,
  areaHandlers,
  groupHandlers,
  itemTypeHandlers,
  savedViewHandlers,
  sellerHandlers,
  tagHandlers,
} from "@/test/handlers"
import type { SavedViewFixture } from "@/test/handlers/savedViews"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const SLUG = "household"

const MINE: SavedViewFixture = {
  id: "sv1",
  name: "Garage tools",
  icon: "wrench",
  query: "area_id=a1&include_inactive=false&sort=-purchase_date&tags=red&type=equipment",
}
const SHARED: SavedViewFixture = {
  id: "sv2",
  name: "Family shared",
  query: "lent_out=true",
  shared: true,
  owned: false,
  can_edit: false,
  can_delete: false,
}
const ALSO_MINE: SavedViewFixture = {
  id: "sv3",
  name: "Gone things",
  query: "area_id=deleted&include_inactive=false&sort=name&tags_exclude=vanished",
}

function renderPage() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: `/g/${SLUG}/views`,
    routes: (
      <Route
        path="/g/:groupSlug/views"
        element={
          <GroupProvider>
            <ConfirmProvider>
              <SavedViewsPage />
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
  server.use(
    ...groupHandlers.list([{ id: "g1", slug: SLUG, name: "Household" }]),
    ...areaHandlers.list(SLUG, [
      { id: "a1", type: "areas", attributes: { id: "a1", name: "Garage", location_id: "l1" } },
    ]),
    ...tagHandlers.list(SLUG, [{ id: "t1", slug: "red", label: "Red", color: "red" }]),
    ...sellerHandlers.list(SLUG, [{ id: "s1", name: "Hardware shop" }])
  )
})

describe("<SavedViewsPage />", () => {
  it("lists each view with its conditions, sharing and count", async () => {
    server.use(
      ...savedViewHandlers.list(SLUG, [MINE, SHARED]),
      ...savedViewHandlers.counts(SLUG, { sv1: 4, sv2: 1 })
    )
    const { container } = renderPage()

    const rows = await screen.findAllByTestId("saved-view-row")
    expect(rows).toHaveLength(2)
    const conditions = within(rows[0]).getByTestId("saved-view-conditions")
    await waitFor(() => expect(conditions).toHaveTextContent("Garage"))
    expect(conditions).toHaveTextContent("Equipment")
    expect(conditions).toHaveTextContent("Tag Red")
    expect(conditions).toHaveTextContent("Sorted by Purchase date, descending")
    expect(within(rows[0]).getByText("Personal")).toBeInTheDocument()
    await waitFor(() =>
      expect(within(rows[0]).getByTestId("saved-view-count")).toHaveTextContent("4 items")
    )
    expect(within(rows[0]).getByTestId("saved-view-open").getAttribute("href")).toContain(
      "saved_view=sv1"
    )

    // A shared view of someone else: visible and placeable, not changeable.
    expect(within(rows[1]).getByText("Shared")).toBeInTheDocument()
    expect(within(rows[1]).queryByTestId("saved-view-actions")).not.toBeInTheDocument()
    expect(within(rows[1]).getByTestId("saved-view-toggle-sidebar")).toBeEnabled()
    expect(within(rows[1]).getByTestId("saved-view-toggle-dashboard")).toBeEnabled()

    expect(await axe(container)).toHaveNoViolations()
  })

  it("marks the conditions that name something deleted", async () => {
    server.use(...savedViewHandlers.list(SLUG, [ALSO_MINE]), ...savedViewHandlers.counts(SLUG))
    renderPage()

    const row = await screen.findByTestId("saved-view-row")
    await waitFor(() =>
      expect(within(row).getByText("Deleted area")).toHaveAttribute("data-stale", "true")
    )
    // A deleted tag says so in words, not only in color.
    expect(within(row).getByText("Not vanished (deleted)")).toHaveAttribute("data-stale", "true")
  })

  it("names each seller of a view and marks a deleted one in words", async () => {
    server.use(
      ...savedViewHandlers.list(SLUG, [
        { id: "sv5", name: "Shops", query: "seller_id=s1&seller_id=s9" },
      ]),
      ...savedViewHandlers.counts(SLUG)
    )
    renderPage()

    const row = await screen.findByTestId("saved-view-row")
    await waitFor(() =>
      expect(within(row).getByText("Deleted seller")).toHaveAttribute("data-stale", "true")
    )
    expect(within(row).getByText("Seller Hardware shop")).not.toHaveAttribute("data-stale")
  })

  it("names each item type of a view by the group's name and marks a deleted one in words", async () => {
    server.use(
      ...itemTypeHandlers.list(SLUG, [
        ...itemTypeHandlers.builtins(),
        {
          id: "type-vehicle",
          slug: "vehicle",
          name: "Vehicle",
          icon: "car",
          builtin: false,
          item_count: 0,
          trashed_item_count: 0,
        },
      ]),
      ...savedViewHandlers.list(SLUG, [
        { id: "sv6", name: "Kinds", query: "type=vehicle&type=boat&type=electronics" },
      ]),
      ...savedViewHandlers.counts(SLUG)
    )
    renderPage()

    const row = await screen.findByTestId("saved-view-row")
    await waitFor(() =>
      expect(within(row).getByText("boat (deleted)")).toHaveAttribute("data-stale", "true")
    )
    expect(within(row).getByText("Vehicle")).not.toHaveAttribute("data-stale")
    expect(within(row).getByText("Electronics")).not.toHaveAttribute("data-stale")
    expect(within(row).getByText(/refer to something that was deleted/)).toBeInTheDocument()
  })

  it("marks no item type as deleted while the types fail to load", async () => {
    let typeRequests = 0
    server.use(
      ...savedViewHandlers.list(SLUG, [{ id: "sv6", name: "Kinds", query: "type=boat" }]),
      ...savedViewHandlers.counts(SLUG),
      msw.get(apiUrl(`/g/${SLUG}/item-types`), () => {
        typeRequests++
        return HttpResponse.json({ errors: [{ status: "Internal Server Error" }] }, { status: 500 })
      })
    )
    renderPage()

    const row = await screen.findByTestId("saved-view-row")
    await waitFor(() => expect(typeRequests).toBeGreaterThan(0))
    await waitFor(() => expect(within(row).getByText("boat")).toBeInTheDocument())
    expect(within(row).getByText("boat")).not.toHaveAttribute("data-stale")
    expect(within(row).queryByText(/\(deleted\)/)).not.toBeInTheDocument()
  })

  it("marks no seller as deleted while the sellers fail to load", async () => {
    let sellerRequests = 0
    server.use(
      ...savedViewHandlers.list(SLUG, [{ id: "sv5", name: "Shops", query: "seller_id=s9" }]),
      ...savedViewHandlers.counts(SLUG),
      msw.get(apiUrl(`/g/${SLUG}/sellers`), () => {
        sellerRequests++
        return HttpResponse.json({ errors: [{ status: "Internal Server Error" }] }, { status: 500 })
      })
    )
    renderPage()

    const row = await screen.findByTestId("saved-view-row")
    await waitFor(() => expect(sellerRequests).toBeGreaterThan(0))
    await waitFor(() => expect(within(row).getByText("Seller")).toBeInTheDocument())
    expect(within(row).getByText("Seller")).not.toHaveAttribute("data-stale")
    expect(within(row).queryByText("Deleted seller")).not.toBeInTheDocument()
  })

  it("marks no tag as deleted while the tag catalogue fails to load", async () => {
    let tagRequests = 0
    server.use(
      ...savedViewHandlers.list(SLUG, [ALSO_MINE]),
      ...savedViewHandlers.counts(SLUG),
      msw.get(apiUrl(`/g/${SLUG}/tags`), () => {
        tagRequests++
        return HttpResponse.json({ errors: [{ status: "Internal Server Error" }] }, { status: 500 })
      })
    )
    renderPage()

    const row = await screen.findByTestId("saved-view-row")
    await waitFor(() => expect(tagRequests).toBeGreaterThan(0))
    await waitFor(() => expect(within(row).getByText("Not vanished")).toBeInTheDocument())
    expect(within(row).getByText("Not vanished")).not.toHaveAttribute("data-stale")
    expect(within(row).queryByText(/\(deleted\)/)).not.toBeInTheDocument()
  })

  it("explains a view whose filter is no longer supported", async () => {
    server.use(
      ...savedViewHandlers.list(SLUG, [
        {
          id: "sv4",
          name: "Old",
          query: "warranty_expires_before=2020-01-01",
          query_error: { parameter: "warranty_expires_before", reason: "is not available" },
        },
      ]),
      ...savedViewHandlers.counts(SLUG)
    )
    renderPage()

    const row = await screen.findByTestId("saved-view-row")
    expect(within(row).getByTestId("saved-view-query-error")).toHaveTextContent(
      "warranty_expires_before"
    )
    expect(within(row).queryByTestId("saved-view-open")).not.toBeInTheDocument()
  })

  it("pins a view to the sidebar", async () => {
    const user = userEvent.setup()
    let sent: Record<string, unknown> | undefined
    server.use(
      ...savedViewHandlers.list(SLUG, [{ ...MINE, show_in_sidebar: false }]),
      ...savedViewHandlers.counts(SLUG),
      ...savedViewHandlers.update(SLUG, MINE, (attrs) => {
        sent = attrs
      })
    )
    renderPage()

    await user.click(await screen.findByTestId("saved-view-toggle-sidebar"))
    await waitFor(() => expect(sent).toEqual({ show_in_sidebar: true }))
  })

  it("puts a shared view of someone else on the member's own dashboard", async () => {
    const user = userEvent.setup()
    let sent: Record<string, unknown> | undefined
    server.use(
      ...savedViewHandlers.list(SLUG, [{ ...SHARED, show_on_dashboard: false }]),
      ...savedViewHandlers.counts(SLUG),
      ...savedViewHandlers.update(SLUG, SHARED, (attrs) => {
        sent = attrs
      })
    )
    renderPage()

    await user.click(await screen.findByTestId("saved-view-toggle-dashboard"))
    await waitFor(() => expect(sent).toEqual({ show_on_dashboard: true }))
  })

  it("moves a view below the next one it may change", async () => {
    const user = userEvent.setup()
    let ids: string[] | undefined
    server.use(
      ...savedViewHandlers.list(SLUG, [MINE, SHARED, ALSO_MINE]),
      ...savedViewHandlers.counts(SLUG),
      ...savedViewHandlers.reorder(SLUG, (got) => {
        ids = got
      })
    )
    renderPage()

    const rows = await screen.findAllByTestId("saved-view-row")
    await user.click(within(rows[0]).getByTestId("saved-view-actions"))
    expect(await screen.findByTestId("saved-view-move-up")).toHaveAttribute("aria-disabled", "true")
    await user.click(screen.getByTestId("saved-view-move-down"))
    // The shared view in between is not the member's to move, so the two
    // views of their own swap places.
    await waitFor(() => expect(ids).toEqual(["sv3", "sv1"]))
  })

  it("renames a view in the edit dialog", async () => {
    const user = userEvent.setup()
    let sent: Record<string, unknown> | undefined
    server.use(
      ...savedViewHandlers.list(SLUG, [MINE]),
      ...savedViewHandlers.counts(SLUG),
      ...savedViewHandlers.update(SLUG, MINE, (attrs) => {
        sent = attrs
      })
    )
    renderPage()

    await user.click(await screen.findByTestId("saved-view-actions"))
    await user.click(await screen.findByTestId("saved-view-edit"))
    const dialog = await screen.findByTestId("saved-view-dialog")
    const name = within(dialog).getByTestId("saved-view-name")
    await user.clear(name)
    await user.type(name, "Workshop")
    await user.click(within(dialog).getByTestId("saved-view-submit"))

    await waitFor(() => expect(sent).toBeDefined())
    expect(sent).toMatchObject({ name: "Workshop", icon: "wrench", shared: false })
    expect(sent).not.toHaveProperty("query")
  })

  it("deletes a view after confirmation", async () => {
    const user = userEvent.setup()
    let deleted = false
    server.use(
      ...savedViewHandlers.list(SLUG, [MINE]),
      ...savedViewHandlers.counts(SLUG),
      ...savedViewHandlers.remove(SLUG, "sv1", () => {
        deleted = true
      })
    )
    renderPage()

    await user.click(await screen.findByTestId("saved-view-actions"))
    await user.click(await screen.findByTestId("saved-view-delete"))
    await user.click(await screen.findByTestId("confirm-accept"))
    await waitFor(() => expect(deleted).toBe(true))
  })

  it("points to the items list while there are no views", async () => {
    server.use(...savedViewHandlers.none(SLUG))
    renderPage()
    expect(await screen.findByTestId("saved-views-empty")).toHaveTextContent("No saved views yet")
  })
})
