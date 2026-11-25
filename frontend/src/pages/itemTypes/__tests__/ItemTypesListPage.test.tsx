import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { GroupProvider } from "@/features/group/GroupContext"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { ItemTypesListPage } from "@/pages/itemTypes/ItemTypesListPage"
import { capture } from "@/test/capture"
import { groupHandlers, itemTypeHandlers } from "@/test/handlers"
import { pickRadixSelect } from "@/test/radix"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

const builtins = itemTypeHandlers.builtins()
const electronics = { ...builtins[1], item_count: 4, trashed_item_count: 1 }
const vehicle = {
  id: "type-vehicle",
  slug: "vehicle",
  name: "Vehicle",
  icon: "car",
  builtin: false,
  item_count: 2,
  trashed_item_count: 0,
}
const types = [builtins[0], electronics, ...builtins.slice(2), vehicle]

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
})

function renderPage(path = `/g/${SLUG}/item-types`) {
  return renderWithProviders({
    initialPath: path,
    routes: (
      <Route
        path="/g/:groupSlug/item-types"
        element={
          <GroupProvider>
            <main>
              <ItemTypesListPage />
            </main>
          </GroupProvider>
        }
      />
    ),
  })
}

function seed(list = types) {
  server.use(...groupHandlers.list(groupFixture), ...itemTypeHandlers.list(SLUG, list))
}

describe("<ItemTypesListPage />", () => {
  it("lists the built-in types by their translated names, then custom types", async () => {
    seed()
    renderPage()
    expect(await screen.findByTestId("item-type-row-white_goods")).toHaveTextContent("Appliance")
    expect(screen.getByTestId("item-types-list-count")).toHaveTextContent("7 types")
    expect(screen.getByTestId("item-type-row-white_goods")).toHaveTextContent("Built-in")
    expect(screen.getByTestId("item-type-row-vehicle")).toHaveTextContent("Vehicle")
    expect(screen.getByTestId("item-type-row-vehicle")).not.toHaveTextContent("Built-in")
    expect(screen.getByTestId("item-type-row-electronics-usage")).toHaveTextContent(
      "4 items · 1 in Trash"
    )
    expect(screen.getByTestId("item-type-row-other-usage")).toHaveTextContent("No items yet")
  })

  it("links each type to its items, inactive ones included", async () => {
    seed()
    renderPage()
    expect(await screen.findByTestId("item-type-row-vehicle-link")).toHaveAttribute(
      "href",
      `/g/${SLUG}/commodities?type=vehicle&inactive=1`
    )
  })

  it("filters by the name shown, ignoring case", async () => {
    seed()
    renderPage(`/g/${SLUG}/item-types?q=APPLI`)
    expect(await screen.findByTestId("item-type-row-white_goods")).toBeVisible()
    expect(screen.queryByTestId("item-type-row-vehicle")).not.toBeInTheDocument()
    expect(screen.getByTestId("item-types-list-count")).toHaveTextContent("1 result")
  })

  it("explains a search without matches", async () => {
    seed()
    renderPage(`/g/${SLUG}/item-types?q=boat`)
    expect(await screen.findByTestId("item-types-list-empty")).toHaveTextContent(
      "No item types match “boat”."
    )
  })

  it("creates a type with a name and an icon", async () => {
    const body = capture<{ data: { type: string; attributes: Record<string, string> } }>()
    seed()
    server.use(
      ...itemTypeHandlers.create(
        SLUG,
        { ...vehicle, id: "type-boat", slug: "boat", name: "Boat", icon: "sailboat" },
        body
      )
    )
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("item-type-row-vehicle")
    await user.click(screen.getByTestId("item-types-create-button"))
    const dialog = await screen.findByTestId("item-type-form-dialog")
    await user.type(within(dialog).getByTestId("item-type-name-input"), "Boat")
    await user.click(within(dialog).getByTestId("item-type-icon-sailboat"))
    await user.click(within(dialog).getByTestId("item-type-form-submit"))
    await waitFor(() =>
      expect(screen.queryByTestId("item-type-form-dialog")).not.toBeInTheDocument()
    )
    expect(body.value?.data).toEqual({
      type: "item_types",
      attributes: { name: "Boat", icon: "sailboat" },
    })
  })

  it("refuses a name the group already has before sending it", async () => {
    seed()
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("item-type-row-vehicle")
    await user.click(screen.getByTestId("item-types-create-button"))
    const dialog = await screen.findByTestId("item-type-form-dialog")
    // The built-in Appliance keeps its English name on the server.
    await user.type(within(dialog).getByTestId("item-type-name-input"), "  appliance ")
    await user.click(within(dialog).getByTestId("item-type-form-submit"))
    expect(await within(dialog).findByTestId("item-type-name-error")).toHaveTextContent(
      "Another item type of this group has this name."
    )
  })

  it("shows the server's taken name on the name field", async () => {
    seed()
    server.use(...itemTypeHandlers.nameTaken(SLUG))
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("item-type-row-vehicle")
    await user.click(screen.getByTestId("item-types-create-button"))
    const dialog = await screen.findByTestId("item-type-form-dialog")
    await user.type(within(dialog).getByTestId("item-type-name-input"), "Boat")
    await user.click(within(dialog).getByTestId("item-type-form-submit"))
    expect(await within(dialog).findByTestId("item-type-name-error")).toHaveTextContent(
      "Another item type of this group has this name."
    )
    expect(screen.getByTestId("item-type-form-dialog")).toBeVisible()
  })

  it("says when the group has the most types it keeps", async () => {
    seed()
    server.use(...itemTypeHandlers.limit(SLUG))
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("item-type-row-vehicle")
    await user.click(screen.getByTestId("item-types-create-button"))
    const dialog = await screen.findByTestId("item-type-form-dialog")
    await user.type(within(dialog).getByTestId("item-type-name-input"), "Boat")
    await user.click(within(dialog).getByTestId("item-type-form-submit"))
    expect(await within(dialog).findByTestId("item-type-form-server-error")).toHaveTextContent(
      "A group keeps at most 100 item types. Delete or merge types before you add another."
    )
  })

  it("renames a built-in type and keeps its default name when left empty", async () => {
    const body = capture<{ data: { id: string; attributes: Record<string, string> } }>()
    seed()
    server.use(...itemTypeHandlers.update(SLUG, { ...electronics, icon: "smartphone" }, body))
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("item-type-row-electronics-edit"))
    const dialog = await screen.findByTestId("item-type-form-dialog")
    const input = within(dialog).getByTestId("item-type-name-input")
    expect(input).toHaveValue("")
    expect(input).toHaveAttribute("placeholder", "Electronics")
    await user.click(within(dialog).getByTestId("item-type-icon-smartphone"))
    await user.click(within(dialog).getByTestId("item-type-form-submit"))
    await waitFor(() => expect(body.value?.data.id).toBe(electronics.id))
    expect(body.value?.data.attributes).toEqual({ name: "", icon: "smartphone" })
  })

  it("requires a name for a custom type", async () => {
    seed()
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("item-type-row-vehicle-edit"))
    const dialog = await screen.findByTestId("item-type-form-dialog")
    await user.clear(within(dialog).getByTestId("item-type-name-input"))
    await user.click(within(dialog).getByTestId("item-type-form-submit"))
    expect(await within(dialog).findByTestId("item-type-name-error")).toHaveTextContent(
      "Enter a name."
    )
  })

  it("deletes a type without items after a confirmation", async () => {
    const calls: string[] = []
    seed()
    server.use(...itemTypeHandlers.remove(SLUG, "type-other", 0, calls))
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("item-type-row-other-delete"))
    const dialog = await screen.findByTestId("item-type-delete-dialog")
    expect(within(dialog).getByTestId("item-type-delete-description")).toHaveTextContent(
      "No item has the type Other."
    )
    expect(within(dialog).queryByTestId("item-type-delete-target")).not.toBeInTheDocument()
    await user.click(within(dialog).getByTestId("item-type-delete-confirm"))
    await waitFor(() => expect(calls).toEqual([""]))
  })

  it("moves the items of a type to the type picked, Trash included, then deletes it", async () => {
    const calls: string[] = []
    seed()
    server.use(...itemTypeHandlers.remove(SLUG, electronics.id, 5, calls))
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("item-type-row-electronics-delete"))
    const dialog = await screen.findByTestId("item-type-delete-dialog")
    expect(within(dialog).getByTestId("item-type-delete-description")).toHaveTextContent(
      "5 items have the type Electronics, 1 of them in Trash."
    )
    const confirm = within(dialog).getByTestId("item-type-delete-confirm")
    expect(confirm).toBeDisabled()
    await pickRadixSelect(user, /^Move the items to$/, { optionLabel: /^Vehicle$/ })
    expect(confirm).toBeEnabled()
    await user.click(confirm)
    await waitFor(() => expect(calls).toEqual([vehicle.id]))
  })

  it("asks for a target when items got the type after the page loaded", async () => {
    const calls: string[] = []
    seed()
    server.use(...itemTypeHandlers.inUse(SLUG, "type-other", { live: 3, trashed: 0 }, calls))
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("item-type-row-other-delete"))
    const dialog = await screen.findByTestId("item-type-delete-dialog")
    await user.click(within(dialog).getByTestId("item-type-delete-confirm"))
    expect(await within(dialog).findByTestId("item-type-delete-refused")).toBeVisible()
    expect(within(dialog).getByTestId("item-type-delete-description")).toHaveTextContent(
      "3 items have the type Other."
    )
    await pickRadixSelect(user, /^Move the items to$/, { optionLabel: /^Furniture$/ })
    await user.click(within(dialog).getByTestId("item-type-delete-confirm"))
    await waitFor(() => expect(calls).toEqual(["", "type-furniture"]))
  })

  it("does not offer to delete the group's last type", async () => {
    seed([vehicle])
    renderPage()
    expect(await screen.findByTestId("item-type-row-vehicle-delete")).toBeDisabled()
  })

  it("has no axe violations", async () => {
    seed()
    const { container } = renderPage()
    await screen.findByTestId("item-type-row-vehicle")
    expect(await axe(container)).toHaveNoViolations()
  })
})
