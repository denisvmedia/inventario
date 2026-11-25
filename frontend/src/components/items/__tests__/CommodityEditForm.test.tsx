import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { http, HttpResponse } from "msw"

import { CommodityEditForm } from "@/components/items/CommodityEditForm"
import {
  updateCommodity,
  type Commodity,
  type UpdateCommodityRequest,
} from "@/features/commodities/api"
import { CUSTOM_FIELD_INVALID_VALUE } from "@/features/customFields/api"
import { GroupProvider } from "@/features/group/GroupContext"
import type { ListNavigation } from "@/hooks/useListNavigation"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { HttpError, __resetHttpForTests } from "@/lib/http"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { pickRadixSelect } from "@/test/radix"
import {
  apiUrl,
  authHandlers,
  commodityHandlers,
  customFieldHandlers,
  groupHandlers,
  itemTypeHandlers,
  memberHandlers,
  tagHandlers,
} from "@/test/handlers"

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

const commodity: Commodity = {
  id: "c1",
  name: "Desk lamp",
  short_name: "Lamp",
  type: "furniture",
  area_id: "a1",
  status: "in_use",
  count: 1,
  draft: false,
  purchase_date: "2024-03-01",
  original_price: 40,
  original_price_currency: "USD",
  // An array at runtime; the generated type says string (see draft.ts).
  urls: ["https://example.com/lamp"] as unknown as string,
}

const noList: ListNavigation = {
  active: false,
  loading: false,
  error: false,
  inList: false,
  position: null,
  total: null,
  page: 1,
  previous: null,
  next: null,
}

function renderForm(
  overrides: Partial<Commodity> = {},
  // `role` signs a member in with that role in the group.
  props: { onSubmit?: (req: unknown) => Promise<unknown>; role?: string } = {}
) {
  setAccessToken("good-token")
  server.use(
    ...groupHandlers.list([{ id: "g1", slug: SLUG, name: "Household", group_currency: "USD" }]),
    ...tagHandlers.list(SLUG, []),
    http.get(apiUrl("/currencies"), () => HttpResponse.json(["USD", "EUR"]))
  )
  if (props.role) {
    server.use(
      ...authHandlers.signedIn(),
      ...memberHandlers.list("g1", [
        { id: "m1", attributes: { member_user_id: "u1", role: props.role } },
      ])
    )
  }
  const handlers = {
    onSubmit: vi.fn(props.onSubmit ?? (async () => ({}))),
    onExit: vi.fn(),
    onGoTo: vi.fn(),
    onAllFields: vi.fn(),
  }
  const utils = renderWithProviders({
    withAuth: !!props.role,
    initialPath: `/g/${SLUG}/x`,
    routes: (
      <Route
        path="/g/:groupSlug/x"
        element={
          <GroupProvider>
            <CommodityEditForm
              commodity={{ ...commodity, ...overrides }}
              areas={[
                { id: "a1", name: "Study", location_id: "l1" },
                { id: "a2", name: "Hallway", location_id: "l1" },
              ]}
              locations={[{ id: "l1", name: "Home" }]}
              groupCurrency="USD"
              nav={noList}
              hrefFor={() => "#"}
              {...handlers}
            />
          </GroupProvider>
        }
      />
    ),
  })
  return { ...utils, ...handlers }
}

function validationError(attributes: Record<string, string>) {
  return new HttpError("invalid", 422, "/commodities/c1", {
    errors: [
      {
        status: "Unprocessable Entity",
        error: { type: "validation.Errors", error: { data: { attributes } } },
      },
    ],
  })
}

// customFieldValueError is the 422 the server sends for a refused custom
// field value: the reason and its code nested under the field id.
function customFieldValueError(fieldId: string, message: string, code: string) {
  return new HttpError("invalid", 422, "/commodities/c1", {
    errors: [
      {
        status: "Unprocessable Entity",
        code: CUSTOM_FIELD_INVALID_VALUE,
        error: {
          type: "validation.Errors",
          error: { data: { attributes: { custom_fields: { [fieldId]: message } } } },
          errorCodes: { data: { attributes: { custom_fields: { [fieldId]: { code } } } } },
        },
      },
    ],
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<CommodityEditForm />", () => {
  it("lays out the mock's sections with the item's values", async () => {
    renderForm()
    expect(await screen.findByLabelText(/^name/i)).toHaveValue("Desk lamp")
    for (const section of ["Identity", "Location", "Purchase", "Warranty", "Notes & Tags"]) {
      expect(screen.getByText(section, { selector: "p" })).toBeInTheDocument()
    }
    expect(screen.getByLabelText(/purchase date/i)).toHaveValue("2024-03-01")
    expect(screen.getByRole("combobox", { name: /^location/i })).toHaveTextContent("Home")
    expect(screen.getByRole("combobox", { name: /^area/i })).toHaveTextContent("Study")
  })

  it("suggests the group's brands and saves the brand and model", async () => {
    const user = userEvent.setup()
    server.use(
      ...commodityHandlers.brands(SLUG, [
        { brand: "Philips", count: 3 },
        { brand: "Philips Hue", count: 1 },
        { brand: "IKEA", count: 2 },
      ])
    )
    const { onSubmit } = renderForm({ brand: "", model: "" })
    const brand = await screen.findByRole("combobox", { name: /^brand/i })
    await user.type(brand, "phil")
    const list = await screen.findByTestId("brand-suggestions")
    expect(
      within(list)
        .getAllByRole("option")
        .map((o) => o.textContent)
    ).toEqual(["Philips", "Philips Hue"])
    // The arrow keys and Enter pick a suggestion without submitting.
    await user.keyboard("{ArrowDown}{Enter}")
    expect(brand).toHaveValue("Philips")
    expect(onSubmit).not.toHaveBeenCalled()
    await user.type(screen.getByLabelText(/^model/i), "  Hue Go  ")
    await user.click(screen.getByTestId("commodity-edit-save"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    expect(onSubmit.mock.calls[0][0]).toMatchObject({ brand: "Philips", model: "Hue Go" })
  })

  it("leaves at once when nothing changed", async () => {
    const user = userEvent.setup()
    const { onExit, onSubmit } = renderForm()
    await user.click(await screen.findByTestId("commodity-edit-cancel"))
    expect(onExit).toHaveBeenCalledTimes(1)
    await user.click(screen.getByTestId("commodity-edit-save"))
    // Save with no changes leaves too, without a request.
    expect(onExit).toHaveBeenCalledTimes(2)
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("asks before dropping unsaved edits", async () => {
    const user = userEvent.setup()
    const { onExit, onSubmit } = renderForm()
    await user.type(await screen.findByLabelText(/^name/i), " (old)")
    await user.click(screen.getByTestId("commodity-edit-back"))
    expect(await screen.findByTestId("unsaved-changes-dialog")).toBeInTheDocument()
    expect(onExit).not.toHaveBeenCalled()
    await user.click(screen.getByTestId("unsaved-changes-discard"))
    expect(onExit).toHaveBeenCalledTimes(1)
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("says the type was deleted when the server refuses it", async () => {
    const user = userEvent.setup()
    const served = { types: [...itemTypeHandlers.builtins(), vehicle] }
    server.use(
      http.get(apiUrl(`/g/${SLUG}/item-types`), () =>
        HttpResponse.json({
          data: served.types.map((t) => ({ id: t.id, type: "item_types", attributes: t })),
          meta: { item_types: served.types.length },
        })
      ),
      ...itemTypeHandlers.unknownOnUpdate(SLUG, "c1")
    )
    renderForm(
      { type: "vehicle" },
      { onSubmit: (req) => updateCommodity("c1", req as UpdateCommodityRequest) }
    )
    await waitFor(() => expect(screen.getByTestId("edit-category")).toHaveTextContent("Vehicle"))

    // Another member deletes the type before the save.
    served.types = itemTypeHandlers.builtins()
    await user.type(screen.getByLabelText(/^name/i), " 2")
    await user.click(screen.getByTestId("commodity-edit-save"))

    expect(await screen.findByText("This type is not in the group.")).toBeInTheDocument()
    expect(await screen.findByTestId("edit-category-gone")).toHaveTextContent(
      "This type was deleted meanwhile. Pick another one."
    )
    expect(screen.getByTestId("edit-category")).toHaveAttribute("aria-invalid", "true")
    expect(screen.queryByTestId("commodity-edit-error")).not.toBeInTheDocument()
  })

  it("keeps the fields it does not show when saving", async () => {
    const user = userEvent.setup()
    const { onSubmit, onExit } = renderForm()
    await user.type(await screen.findByLabelText(/^notes/i), "Bulb is E27")
    await user.click(screen.getByTestId("commodity-edit-save"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    expect(onSubmit.mock.calls[0][0]).toMatchObject({
      comments: "Bulb is E27",
      urls: ["https://example.com/lamp"],
      status: "in_use",
    })
    await waitFor(() => expect(onExit).toHaveBeenCalledTimes(1))
  })

  it("points to all fields when the server rejects one this form does not show", async () => {
    const user = userEvent.setup()
    const { onAllFields, onExit } = renderForm(
      {},
      {
        onSubmit: async () => {
          throw validationError({ urls: "must be a valid URL" })
        },
      }
    )
    const name = await screen.findByLabelText(/^name/i)
    await user.clear(name)
    await user.type(name, "Reading lamp")
    await user.click(screen.getByTestId("commodity-edit-save"))
    expect(await screen.findByTestId("commodity-edit-hidden-error")).toBeInTheDocument()
    expect(screen.queryByTestId("commodity-edit-error")).toBeNull()
    expect(onExit).not.toHaveBeenCalled()
    await user.click(screen.getByTestId("commodity-edit-all-fields"))
    // The dialog opens with what was typed here.
    expect(onAllFields).toHaveBeenCalledWith(expect.objectContaining({ name: "Reading lamp" }))
  })

  it("shows a server message under the field it belongs to", async () => {
    const user = userEvent.setup()
    renderForm(
      {},
      {
        onSubmit: async () => {
          throw validationError({ serial_number: "is already taken" })
        },
      }
    )
    await user.type(await screen.findByLabelText(/serial number/i), "SN-1")
    await user.click(screen.getByTestId("commodity-edit-save"))
    expect(await screen.findByText("is already taken")).toBeInTheDocument()
    expect(screen.queryByTestId("commodity-edit-hidden-error")).toBeNull()
  })

  it("drops a server message on the area once another area is picked", async () => {
    const user = userEvent.setup()
    renderForm(
      {},
      {
        onSubmit: async () => {
          throw validationError({ area_id: "area not found" })
        },
      }
    )
    await user.type(await screen.findByLabelText(/serial number/i), "SN-1")
    await user.click(screen.getByTestId("commodity-edit-save"))
    expect(await screen.findByText("area not found")).toBeInTheDocument()

    await user.click(screen.getByRole("combobox", { name: /^area/i }))
    await user.click(await screen.findByRole("option", { name: "Hallway" }))
    await waitFor(() => expect(screen.queryByText("area not found")).toBeNull())
  })

  it("creates a location from the picker without saving or dropping edits", async () => {
    const user = userEvent.setup()
    server.use(
      http.post(apiUrl(`/g/${SLUG}/locations`), () =>
        HttpResponse.json(
          { data: { id: "l2", type: "locations", attributes: { name: "Cottage" } } },
          { status: 201 }
        )
      )
    )
    const { onSubmit, onExit } = renderForm({}, { role: "owner" })
    await user.type(await screen.findByLabelText(/serial number/i), "SN-7")

    await user.click(screen.getByRole("combobox", { name: /^location/i }))
    const listbox = await screen.findByRole("listbox")
    await user.click(await within(listbox).findByRole("option", { name: /new location/i }))
    const dialog = await screen.findByTestId("location-form-dialog")
    await user.type(within(dialog).getByTestId("location-name-input"), "Cottage")
    await user.click(within(dialog).getByTestId("location-form-submit"))
    await waitFor(() => expect(screen.queryByTestId("location-form-dialog")).toBeNull())

    expect(screen.getByRole("combobox", { name: /^location/i })).toHaveTextContent("Cottage")
    expect(screen.getByLabelText(/serial number/i)).toHaveValue("SN-7")
    // The dialog's submit stays out of the item form.
    expect(onSubmit).not.toHaveBeenCalled()
    expect(onExit).not.toHaveBeenCalled()
    expect(screen.getByTestId("commodity-edit-form")).toBeInTheDocument()
  })

  it("disables an empty warranty for a bundle", async () => {
    renderForm({ count: 6 })
    expect(await screen.findByLabelText(/expiry date/i)).toBeDisabled()
    expect(screen.getByLabelText(/warranty notes/i)).toBeDisabled()
  })

  it("has no axe violations", async () => {
    const { container } = renderForm()
    await screen.findByLabelText(/^name/i)
    expect(await axe(container)).toHaveNoViolations()
  })
})

describe("<CommodityEditForm /> custom fields", () => {
  const fields = [
    customFieldHandlers.field({
      id: "cf-fuel",
      name: "Fuel",
      type: "select",
      options: [
        { id: "opt-petrol", label: "Petrol" },
        { id: "opt-diesel", label: "Diesel" },
      ],
    }),
    customFieldHandlers.field({ id: "cf-plate", name: "Plate" }),
    customFieldHandlers.field({ id: "cf-weight", name: "Weight", type: "number" }),
  ]
  const stored = {
    "cf-plate": "AB 123",
    "cf-weight": "12.5",
    // A field this client does not know yet.
    "cf-new": "kept",
  } as unknown as Commodity["custom_fields"]

  it("edits, adds and clears values and sends only the fields it knows", async () => {
    server.use(...customFieldHandlers.list(SLUG, fields))
    const user = userEvent.setup()
    const { onSubmit } = renderForm({ custom_fields: stored })
    const plate = await screen.findByLabelText("Plate")
    expect(plate).toHaveValue("AB 123")
    expect(screen.getByLabelText("Weight")).toHaveValue("12.5")
    expect(screen.queryByLabelText("Fuel")).not.toBeInTheDocument()

    await user.clear(plate)
    await user.type(plate, "XY 9")
    await user.click(screen.getByTestId("custom-field-cf-weight-remove"))
    await user.click(screen.getByTestId("custom-field-add"))
    await user.click(await screen.findByTestId("custom-field-add-cf-fuel"))
    await pickRadixSelect(user, /^Fuel$/, { optionLabel: /^Diesel$/ })
    await user.click(screen.getByTestId("commodity-edit-save"))

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    const req = onSubmit.mock.calls[0][0] as UpdateCommodityRequest
    expect(req.custom_fields).toEqual({
      "cf-plate": "XY 9",
      "cf-weight": null,
      "cf-fuel": "opt-diesel",
    })
  })

  it("leaves the values out of a save that does not touch them", async () => {
    server.use(...customFieldHandlers.list(SLUG, fields))
    const user = userEvent.setup()
    const { onSubmit } = renderForm()
    await screen.findByTestId("custom-field-add")
    await user.type(screen.getByLabelText(/^name/i), "!")
    await user.click(screen.getByTestId("commodity-edit-save"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    const req = onSubmit.mock.calls[0][0] as UpdateCommodityRequest
    expect(req.custom_fields).toBeUndefined()
  })

  it("refuses a value its field would refuse before saving", async () => {
    server.use(...customFieldHandlers.list(SLUG, fields))
    const user = userEvent.setup()
    const { onSubmit } = renderForm({ custom_fields: stored })
    const weight = await screen.findByLabelText("Weight")
    await user.clear(weight)
    await user.type(weight, "heavy")
    await user.click(screen.getByTestId("commodity-edit-save"))
    expect(await screen.findByTestId("custom-field-cf-weight-error")).toHaveTextContent(
      "Enter a number."
    )
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("applies the template of the type picked without overwriting a value", async () => {
    server.use(
      ...customFieldHandlers.list(SLUG, [
        { ...fields[0], templates: [{ item_type_id: "type-equipment" }] },
        { ...fields[1], templates: [{ item_type_id: "type-equipment", default_value: "ZZ" }] },
        { ...fields[2], templates: [{ item_type_id: "type-equipment", default_value: "3" }] },
      ])
    )
    const user = userEvent.setup()
    const { onSubmit } = renderForm({ custom_fields: { "cf-plate": "AB 123" } as never })
    expect(await screen.findByLabelText("Plate")).toHaveValue("AB 123")
    // Opening the form applies nothing.
    expect(screen.queryByLabelText("Weight")).not.toBeInTheDocument()

    await pickRadixSelect(user, /^Type/, { optionLabel: /^Equipment$/ })
    expect(screen.getByLabelText("Plate")).toHaveValue("AB 123")
    expect(await screen.findByLabelText("Weight")).toHaveValue("3")
    expect(screen.getByRole("combobox", { name: "Fuel" })).toBeInTheDocument()

    await user.click(screen.getByTestId("commodity-edit-save"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    const req = onSubmit.mock.calls[0][0] as UpdateCommodityRequest
    expect(req.type).toBe("equipment")
    expect(req.custom_fields).toEqual({ "cf-plate": "AB 123", "cf-weight": "3" })
  })

  it("shows a value the server refused under its field", async () => {
    server.use(...customFieldHandlers.list(SLUG, fields))
    const user = userEvent.setup()
    renderForm(
      { custom_fields: stored },
      {
        onSubmit: async () => {
          throw customFieldValueError(
            "cf-plate",
            "must be at most 200 characters",
            "custom_field_too_long"
          )
        },
      }
    )
    const plate = await screen.findByLabelText("Plate")
    await user.type(plate, "!")
    await user.click(screen.getByTestId("commodity-edit-save"))
    expect(await screen.findByTestId("custom-field-cf-plate-error")).toHaveTextContent(
      "must be at most 200 characters"
    )
  })

  it("names the currency picker of a money field and flags it with the amount", async () => {
    const insured = customFieldHandlers.field({
      id: "cf-insured",
      name: "Insured value",
      type: "money",
    })
    server.use(...customFieldHandlers.list(SLUG, [insured]))
    const user = userEvent.setup()
    const { onSubmit } = renderForm({
      custom_fields: {
        "cf-insured": { amount: 12, currency: "EUR" },
      } as unknown as Commodity["custom_fields"],
    })
    const picker = await screen.findByRole("combobox", { name: "Currency of Insured value" })
    expect(picker).toHaveTextContent("EUR")
    expect(picker).not.toHaveAttribute("aria-invalid")
    const amount = screen.getByLabelText("Insured value")
    await user.clear(amount)
    await user.type(amount, "a lot")
    await user.click(screen.getByTestId("commodity-edit-save"))
    expect(await screen.findByTestId("custom-field-cf-insured-error")).toBeVisible()
    expect(picker).toHaveAttribute("aria-invalid", "true")
    expect(onSubmit).not.toHaveBeenCalled()
  })

  // The API sends a number as a decimal string, and the form sends it back
  // with every digit.
  it("keeps every digit of a number", async () => {
    server.use(...customFieldHandlers.list(SLUG, fields))
    const user = userEvent.setup()
    const { onSubmit } = renderForm({
      custom_fields: { "cf-weight": "0.0000005" } as unknown as Commodity["custom_fields"],
    })
    const weight = await screen.findByLabelText("Weight")
    expect(weight).toHaveValue("0.0000005")
    await user.clear(weight)
    await user.type(weight, "1234567890.1234567891")
    await user.click(screen.getByTestId("commodity-edit-save"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    const req = onSubmit.mock.calls[0][0] as UpdateCommodityRequest
    expect(req.custom_fields).toEqual({ "cf-weight": "1234567890.1234567891" })
  })
})
