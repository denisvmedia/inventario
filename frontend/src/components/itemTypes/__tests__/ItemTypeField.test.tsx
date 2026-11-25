import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"
import { useState } from "react"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { ItemTypeField } from "@/components/itemTypes/ItemTypeField"
import { GroupProvider } from "@/features/group/GroupContext"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { capture } from "@/test/capture"
import { apiUrl, groupHandlers, itemTypeHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

const SLUG = "household"
const vehicle = {
  id: "type-vehicle",
  slug: "vehicle",
  name: "Vehicle",
  icon: "car",
  builtin: false,
  item_count: 1,
  trashed_item_count: 0,
}

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
})

// Harness keeps the picked slug in state, as the item forms do, and shows
// it. The "fail" button stands in for a save the server refused with a 422
// on the type.
function Harness({ initial = "" }: { initial?: string }) {
  const [value, setValue] = useState(initial)
  const [error, setError] = useState("")
  return (
    <>
      <label htmlFor="type">Type</label>
      <ItemTypeField id="type" value={value} onChange={setValue} errorMessage={error} />
      <output data-testid="picked">{value}</output>
      <button type="button" onClick={() => setError("no item type with this slug in the group")}>
        fail
      </button>
    </>
  )
}

function renderField(initial = "", withGroup = true) {
  if (!withGroup) {
    return renderWithProviders({
      initialPath: "/x",
      routes: <Route path="/x" element={<Harness initial={initial} />} />,
    })
  }
  return renderWithProviders({
    initialPath: `/g/${SLUG}/x`,
    routes: (
      <Route
        path="/g/:groupSlug/x"
        element={
          <GroupProvider>
            <Harness initial={initial} />
          </GroupProvider>
        }
      />
    ),
  })
}

function seedGroup() {
  server.use(
    ...groupHandlers.list([{ id: "g1", slug: SLUG, name: "Household", group_currency: "USD" }])
  )
}

// serveTypes answers GET /item-types from a list the test can change.
function serveTypes(types: ReturnType<typeof itemTypeHandlers.builtins>) {
  const state = { types }
  server.use(
    http.get(apiUrl(`/g/${SLUG}/item-types`), () =>
      HttpResponse.json({
        data: state.types.map((t) => ({ id: t.id, type: "item_types", attributes: t })),
        meta: { item_types: state.types.length },
      })
    )
  )
  return state
}

describe("<ItemTypeField />", () => {
  it("offers the group's types, built-in ones by their translated names", async () => {
    seedGroup()
    server.use(...itemTypeHandlers.list(SLUG, [...itemTypeHandlers.builtins(), vehicle]))
    const user = userEvent.setup()
    renderField()
    await user.click(await screen.findByRole("combobox", { name: "Type" }))
    const listbox = await screen.findByRole("listbox")
    const options = within(listbox)
      .getAllByRole("option")
      .map((o) => o.textContent)
    expect(options).toEqual([
      "Appliance",
      "Electronics",
      "Equipment",
      "Furniture",
      "Clothes",
      "Other",
      "Vehicle",
      "New type",
    ])
    await user.click(within(listbox).getByRole("option", { name: "Vehicle" }))
    expect(screen.getByTestId("picked")).toHaveTextContent("vehicle")
  })

  it("creates a type from the list and selects it", async () => {
    const body = capture<{ data: { attributes: { name: string; icon: string } } }>()
    seedGroup()
    const served = serveTypes(itemTypeHandlers.builtins())
    server.use(
      http.post(apiUrl(`/g/${SLUG}/item-types`), async ({ request }) => {
        body.value = (await request.json()) as typeof body.value
        served.types = [...served.types, vehicle]
        return HttpResponse.json(
          { data: { id: vehicle.id, type: "item_types", attributes: vehicle } },
          { status: 201 }
        )
      })
    )
    const user = userEvent.setup()
    renderField()
    await user.click(await screen.findByRole("combobox", { name: "Type" }))
    await user.click(await screen.findByTestId("type-new"))
    const dialog = await screen.findByTestId("item-type-form-dialog")
    await user.type(within(dialog).getByTestId("item-type-name-input"), "Vehicle")
    await user.click(within(dialog).getByTestId("item-type-icon-car"))
    await user.click(within(dialog).getByTestId("item-type-form-submit"))
    await waitFor(() =>
      expect(screen.queryByTestId("item-type-form-dialog")).not.toBeInTheDocument()
    )
    expect(body.value?.data.attributes).toEqual({ name: "Vehicle", icon: "car" })
    expect(screen.getByTestId("picked")).toHaveTextContent("vehicle")
    expect(screen.getByRole("combobox", { name: "Type" })).toHaveTextContent("Vehicle")
  })

  it("offers the type that already has a name instead of a duplicate", async () => {
    seedGroup()
    server.use(...itemTypeHandlers.list(SLUG, [...itemTypeHandlers.builtins(), vehicle]))
    const user = userEvent.setup()
    renderField()
    await user.click(await screen.findByRole("combobox", { name: "Type" }))
    await user.click(await screen.findByTestId("type-new"))
    const dialog = await screen.findByTestId("item-type-form-dialog")
    await user.type(within(dialog).getByTestId("item-type-name-input"), "VEHICLE")
    await user.click(within(dialog).getByTestId("item-type-form-submit"))
    await user.click(await within(dialog).findByTestId("item-type-name-taken-use"))
    expect(screen.getByTestId("picked")).toHaveTextContent("vehicle")
  })

  it("says so when the selected type was deleted meanwhile", async () => {
    seedGroup()
    const served = serveTypes([...itemTypeHandlers.builtins(), vehicle])
    const user = userEvent.setup()
    renderField("vehicle")
    await waitFor(() =>
      expect(screen.getByRole("combobox", { name: "Type" })).toHaveTextContent("Vehicle")
    )
    expect(screen.queryByTestId("type-gone")).not.toBeInTheDocument()

    // Another member deletes the type; the save is refused with a 422 on
    // the field, and the field refetches the list.
    served.types = itemTypeHandlers.builtins()
    await user.click(screen.getByRole("button", { name: "fail" }))
    expect(await screen.findByTestId("type-gone")).toHaveTextContent(
      "This type was deleted meanwhile. Pick another one."
    )
    expect(screen.getByRole("combobox", { name: "Type" })).toHaveAttribute("aria-invalid", "true")
  })

  it("offers the built-in types and no new type without a group", async () => {
    const user = userEvent.setup()
    renderField("", false)
    await user.click(await screen.findByRole("combobox", { name: "Type" }))
    const listbox = await screen.findByRole("listbox")
    expect(within(listbox).getAllByRole("option")).toHaveLength(6)
    expect(within(listbox).queryByText("New type")).not.toBeInTheDocument()
    await user.click(within(listbox).getByRole("option", { name: "Furniture" }))
    expect(screen.getByTestId("picked")).toHaveTextContent("furniture")
  })
})
