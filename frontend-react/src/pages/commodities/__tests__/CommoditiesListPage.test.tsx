import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { CommoditiesListPage } from "@/pages/commodities/CommoditiesListPage"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { areaHandlers, commodityHandlers, groupHandlers } from "@/test/handlers"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", main_currency: "USD" },
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

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
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
    // The dialog mounts at the document root; the form's stepper has a
    // `Form steps` aria-label which only the dialog renders.
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
    // emoji from the design mock.
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
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.list(SLUG, []),
      ...commodityHandlers.create(SLUG, commodityRes("c-new", { name: "Sofa", type: "furniture" }))
    )
    renderList()
    await screen.findByTestId("commodities-empty")
    await user.click(screen.getByTestId("commodities-add-button"))
    await user.type(await screen.findByLabelText(/^Name$/i), "Sofa")
    await user.type(screen.getByLabelText(/^Short name$/i), "Sofa")
    await user.selectOptions(screen.getByLabelText(/^Type$/i), "furniture")
    await user.selectOptions(screen.getByLabelText(/^Area$/i), "a1")
    await user.click(screen.getByLabelText(/Save as draft/i))
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByLabelText(/Purchase date/i)
    // Step through Purchase → Warranty (stub) → Extras → Files, then submit.
    await user.click(screen.getByTestId("commodity-form-next"))
    await user.click(screen.getByTestId("commodity-form-next"))
    await user.click(screen.getByTestId("commodity-form-next"))
    await user.click(await screen.findByTestId("commodity-form-submit"))
    // Successful submit closes the dialog.
    await waitFor(() => expect(screen.queryByLabelText(/form steps/i)).not.toBeInTheDocument())
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
