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
import { CustomFieldsListPage } from "@/pages/customFields/CustomFieldsListPage"
import { capture } from "@/test/capture"
import { customFieldHandlers, groupHandlers } from "@/test/handlers"
import { pickRadixSelect } from "@/test/radix"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

const plate = customFieldHandlers.field({
  id: "cf-plate",
  name: "Plate",
  item_count: 3,
  trashed_item_count: 1,
})
const fuel = customFieldHandlers.field({
  id: "cf-fuel",
  name: "Fuel",
  type: "select",
  options: [
    { id: "opt-petrol", label: "Petrol", item_count: 2 },
    { id: "opt-diesel", label: "Diesel", item_count: 1 },
  ],
})
const fields = [fuel, plate]

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
})

function renderPage(path = `/g/${SLUG}/custom-fields`) {
  return renderWithProviders({
    initialPath: path,
    routes: (
      <Route
        path="/g/:groupSlug/custom-fields"
        element={
          <GroupProvider>
            <main>
              <CustomFieldsListPage />
            </main>
          </GroupProvider>
        }
      />
    ),
  })
}

function seed(list = fields) {
  server.use(...groupHandlers.list(groupFixture), ...customFieldHandlers.list(SLUG, list))
}

describe("<CustomFieldsListPage />", () => {
  it("lists the fields with their types and usage", async () => {
    seed()
    renderPage()
    expect(await screen.findByTestId("custom-field-row-cf-plate")).toHaveTextContent("Plate")
    expect(screen.getByTestId("custom-field-row-cf-plate")).toHaveTextContent("Text")
    expect(screen.getByTestId("custom-field-row-cf-plate-usage")).toHaveTextContent(
      "3 items · 1 in Trash"
    )
    expect(screen.getByTestId("custom-field-row-cf-fuel-usage")).toHaveTextContent(
      "2 options · No values yet"
    )
    expect(screen.getByTestId("custom-fields-list-count")).toHaveTextContent("2 fields")
  })

  it("explains an empty group", async () => {
    seed([])
    renderPage()
    expect(await screen.findByTestId("custom-fields-list-empty")).toHaveTextContent(
      "No custom fields yet."
    )
  })

  it("filters by name", async () => {
    seed()
    renderPage(`/g/${SLUG}/custom-fields?q=fu`)
    expect(await screen.findByTestId("custom-field-row-cf-fuel")).toBeVisible()
    expect(screen.queryByTestId("custom-field-row-cf-plate")).not.toBeInTheDocument()
  })

  it("creates a select field with its options", async () => {
    const body = capture<{ data: { type: string; attributes: Record<string, unknown> } }>()
    seed()
    server.use(
      ...customFieldHandlers.create(
        SLUG,
        customFieldHandlers.field({ id: "cf-mount", name: "Mount", type: "select" }),
        body
      )
    )
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("custom-field-row-cf-plate")
    await user.click(screen.getByTestId("custom-fields-create-button"))
    const dialog = await screen.findByTestId("custom-field-form-dialog")
    await user.type(within(dialog).getByTestId("custom-field-name-input"), "Mount")
    await pickRadixSelect(user, /^Type$/, { optionLabel: /^Select$/ })
    // A select field needs an option.
    await user.click(within(dialog).getByTestId("custom-field-form-submit"))
    expect(await within(dialog).findByTestId("custom-field-options-error")).toHaveTextContent(
      "Add at least one option."
    )
    await user.click(within(dialog).getByTestId("custom-field-option-add"))
    await user.type(within(dialog).getByTestId("custom-field-option-input-0"), "E")
    await user.click(within(dialog).getByTestId("custom-field-option-add"))
    await user.type(within(dialog).getByTestId("custom-field-option-input-1"), "e")
    await user.click(within(dialog).getByTestId("custom-field-form-submit"))
    expect(await within(dialog).findByText("Each option needs its own label.")).toBeVisible()
    await user.clear(within(dialog).getByTestId("custom-field-option-input-1"))
    await user.type(within(dialog).getByTestId("custom-field-option-input-1"), "Z")
    await user.click(within(dialog).getByTestId("custom-field-form-submit"))
    await waitFor(() =>
      expect(screen.queryByTestId("custom-field-form-dialog")).not.toBeInTheDocument()
    )
    expect(body.value?.data).toEqual({
      type: "custom_fields",
      attributes: {
        name: "Mount",
        type: "select",
        options: [{ label: "E" }, { label: "Z" }],
        templates: [],
      },
    })
  })

  it("refuses a name the group already has before sending it", async () => {
    seed()
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("custom-field-row-cf-plate")
    await user.click(screen.getByTestId("custom-fields-create-button"))
    const dialog = await screen.findByTestId("custom-field-form-dialog")
    await user.type(within(dialog).getByTestId("custom-field-name-input"), " PLATE ")
    await user.click(within(dialog).getByTestId("custom-field-form-submit"))
    expect(await within(dialog).findByTestId("custom-field-name-error")).toHaveTextContent(
      "Another field of the group has this name."
    )
  })

  it("renames a field, keeps its type and says what removing an option clears", async () => {
    const body = capture<{ data: { attributes: Record<string, unknown> } }>()
    seed()
    server.use(...customFieldHandlers.update(SLUG, { ...fuel, name: "Fuel type" }, body))
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("custom-field-row-cf-fuel-edit"))
    const dialog = await screen.findByTestId("custom-field-form-dialog")
    expect(within(dialog).getByTestId("custom-field-type-select")).toBeDisabled()
    expect(within(dialog).getByText("2 items")).toBeVisible()
    await user.click(within(dialog).getByTestId("custom-field-option-remove-0"))
    expect(within(dialog).getByTestId("custom-field-options-cleared")).toHaveTextContent(
      "Saving clears the removed options on 2 items."
    )
    const name = within(dialog).getByTestId("custom-field-name-input")
    await user.clear(name)
    await user.type(name, "Fuel type")
    await user.click(within(dialog).getByTestId("custom-field-form-submit"))
    await waitFor(() =>
      expect(screen.queryByTestId("custom-field-form-dialog")).not.toBeInTheDocument()
    )
    expect(body.value?.data.attributes).toEqual({
      name: "Fuel type",
      options: [{ id: "opt-diesel", label: "Diesel" }],
      templates: [],
    })
  })

  it("offers a field on item types, with a default for each", async () => {
    const body = capture<{ data: { attributes: Record<string, unknown> } }>()
    seed([
      fuel,
      { ...plate, templates: [{ item_type_id: "type-electronics", default_value: "X" }] },
    ])
    server.use(...customFieldHandlers.update(SLUG, plate, body))
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("custom-field-row-cf-plate-edit"))
    const dialog = await screen.findByTestId("custom-field-form-dialog")
    const templates = within(dialog).getByTestId("custom-field-templates")
    expect(within(templates).getByTestId("custom-field-template-electronics")).toBeChecked()
    expect(
      within(templates).getByTestId("custom-field-template-default-type-electronics-input")
    ).toHaveValue("X")

    // Untick electronics, tick equipment with a default and furniture without.
    await user.click(within(templates).getByTestId("custom-field-template-electronics"))
    await user.click(within(templates).getByTestId("custom-field-template-equipment"))
    await user.type(
      within(templates).getByTestId("custom-field-template-default-type-equipment-input"),
      "N/A"
    )
    await user.click(within(templates).getByTestId("custom-field-template-furniture"))
    await user.click(within(dialog).getByTestId("custom-field-form-submit"))
    await waitFor(() =>
      expect(screen.queryByTestId("custom-field-form-dialog")).not.toBeInTheDocument()
    )
    expect(body.value?.data.attributes.templates).toEqual([
      { item_type_id: "type-equipment", default_value: "N/A" },
      { item_type_id: "type-furniture", default_value: null },
    ])
  })

  it("checks a template default against the field's type", async () => {
    const body = capture<{ data: { attributes: Record<string, unknown> } }>()
    seed()
    server.use(
      ...customFieldHandlers.create(
        SLUG,
        customFieldHandlers.field({ id: "cf-odo", name: "Odometer", type: "number" }),
        body
      )
    )
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("custom-field-row-cf-plate")
    await user.click(screen.getByTestId("custom-fields-create-button"))
    const dialog = await screen.findByTestId("custom-field-form-dialog")
    await user.type(within(dialog).getByTestId("custom-field-name-input"), "Odo")
    await pickRadixSelect(user, /^Type$/, { optionLabel: /^Number$/ })
    await user.click(within(dialog).getByTestId("custom-field-template-other"))
    const input = within(dialog).getByTestId("custom-field-template-default-type-other-input")
    await user.type(input, "lots")
    await user.click(within(dialog).getByTestId("custom-field-form-submit"))
    expect(await within(dialog).findByText("Enter a number.")).toBeVisible()
    expect(body.value).toBeUndefined()

    await user.clear(input)
    await user.type(input, "12")
    await user.click(within(dialog).getByTestId("custom-field-form-submit"))
    await waitFor(() =>
      expect(screen.queryByTestId("custom-field-form-dialog")).not.toBeInTheDocument()
    )
    expect(body.value?.data.attributes.templates).toEqual([
      { item_type_id: "type-other", default_value: "12" },
    ])
  })

  it("deletes a field in use only once the member confirms its values go too", async () => {
    const calls: string[] = []
    seed()
    server.use(...customFieldHandlers.remove(SLUG, "cf-plate", calls, { live: 3, trashed: 1 }))
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("custom-field-row-cf-plate-delete"))
    const dialog = await screen.findByTestId("custom-field-delete-dialog")
    expect(within(dialog).getByTestId("custom-field-delete-description")).toHaveTextContent(
      "4 items have a value for this field. Deleting the field deletes their values. This includes 1 item in Trash."
    )
    expect(within(dialog).getByTestId("custom-field-delete-confirm")).toBeDisabled()
    await user.click(within(dialog).getByTestId("custom-field-delete-values"))
    await user.click(within(dialog).getByTestId("custom-field-delete-confirm"))
    await waitFor(() =>
      expect(screen.queryByTestId("custom-field-delete-dialog")).not.toBeInTheDocument()
    )
    expect(calls).toEqual(["delete_values=true&expected_values=4"])
  })

  it("asks again when the count changed after the member confirmed", async () => {
    const calls: string[] = []
    seed()
    server.use(...customFieldHandlers.remove(SLUG, "cf-plate", calls, { live: 5, trashed: 1 }))
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("custom-field-row-cf-plate-delete"))
    const dialog = await screen.findByTestId("custom-field-delete-dialog")
    await user.click(within(dialog).getByTestId("custom-field-delete-values"))
    await user.click(within(dialog).getByTestId("custom-field-delete-confirm"))
    expect(await within(dialog).findByTestId("custom-field-delete-refused")).toBeVisible()
    expect(within(dialog).getByTestId("custom-field-delete-description")).toHaveTextContent(
      "6 items have a value for this field."
    )
    expect(within(dialog).getByTestId("custom-field-delete-confirm")).toBeDisabled()
    expect(calls).toEqual(["delete_values=true&expected_values=4"])
  })

  it("asks again when values were written after the page loaded", async () => {
    const calls: string[] = []
    seed([{ ...plate, item_count: 0, trashed_item_count: 0 }])
    server.use(...customFieldHandlers.remove(SLUG, "cf-plate", calls, { live: 1, trashed: 0 }))
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("custom-field-row-cf-plate-delete"))
    const dialog = await screen.findByTestId("custom-field-delete-dialog")
    await user.click(within(dialog).getByTestId("custom-field-delete-confirm"))
    expect(await within(dialog).findByTestId("custom-field-delete-refused")).toBeVisible()
    expect(within(dialog).getByTestId("custom-field-delete-description")).toHaveTextContent(
      "1 item has a value for this field. Deleting the field deletes that value."
    )
    expect(within(dialog).getByTestId("custom-field-delete-confirm")).toBeDisabled()
    expect(calls).toEqual([""])
  })

  it("has no axe violations", async () => {
    seed()
    const { container } = renderPage()
    await screen.findByTestId("custom-field-row-cf-plate")
    expect(await axe(container)).toHaveNoViolations()
  })
})
