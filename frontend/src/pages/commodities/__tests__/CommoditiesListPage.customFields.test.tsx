import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
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
  customFieldHandlers,
  groupHandlers,
  savedViewHandlers,
  sellerHandlers,
} from "@/test/handlers"
import { pickRadixSelect } from "@/test/radix"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

// The items list's custom field filter, sort and columns (#2803).

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

const plate = customFieldHandlers.field({ id: "cf-plate", name: "Plate" })
const fuel = customFieldHandlers.field({
  id: "cf-fuel",
  name: "Fuel",
  type: "select",
  options: [
    { id: "opt-diesel", label: "Diesel" },
    { id: "opt-petrol", label: "Petrol" },
  ],
})
const odometer = customFieldHandlers.field({ id: "cf-odo", name: "Odometer", type: "number" })
const charger = customFieldHandlers.field({ id: "cf-charger", name: "Charger", type: "item_link" })
const notes = customFieldHandlers.field({ id: "cf-notes", name: "Log", type: "long_text" })

function commodityRes(id: string, attrs: Record<string, unknown>) {
  return {
    id,
    type: "commodities",
    attributes: { id, count: 1, status: "in_use", type: "other", area_id: "a1", ...attrs },
  }
}

const van = commodityRes("c1", {
  name: "Van",
  custom_fields: { "cf-plate": "AB 123", "cf-fuel": "opt-diesel", "cf-charger": "c9" },
})

// listing answers GET /commodities with van and records each query string.
function listing(queries: string[]) {
  return msw.get(apiUrl(`/g/${SLUG}/commodities`), ({ request }) => {
    queries.push(new URL(request.url).search)
    return HttpResponse.json({
      data: [van],
      meta: { commodities: 1, linked_items: { c9: { name: "Wall charger", trashed: true } } },
    })
  })
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
  localStorage.clear()
  server.use(
    msw.get(apiUrl(`/g/${SLUG}/settings`), () => HttpResponse.json({})),
    ...sellerHandlers.list(SLUG, []),
    ...savedViewHandlers.none(SLUG),
    ...groupHandlers.list(groupFixture),
    ...areaHandlers.list(SLUG, [
      { id: "a1", type: "areas", attributes: { id: "a1", name: "Garage", location_id: "l1" } },
    ]),
    ...customFieldHandlers.list(SLUG, [charger, fuel, notes, odometer, plate])
  )
})

function lastQuery(queries: string[]): URLSearchParams {
  return new URLSearchParams(queries[queries.length - 1] ?? "")
}

describe("<CommoditiesListPage /> custom fields (#2803)", () => {
  it("adds and removes a custom field condition", async () => {
    const queries: string[] = []
    server.use(listing(queries))
    const user = userEvent.setup()
    renderList()
    await screen.findByTestId("commodity-card")

    await user.click(screen.getByTestId("commodities-filter-custom-fields"))
    await pickRadixSelect(user, /^Field$/, { optionLabel: /^Plate$/ })
    await user.type(screen.getByTestId("commodities-filter-cf-value"), "AB")
    await user.click(screen.getByTestId("commodities-filter-cf-add"))

    await waitFor(() => expect(lastQuery(queries).getAll("cf")).toEqual(["cf-plate:contains:AB"]))
    expect(screen.getByTestId("commodities-filter-cf-condition-0")).toHaveTextContent(
      "Plate contains “AB”"
    )

    await user.click(screen.getByTestId("commodities-filter-cf-remove-0"))
    await waitFor(() => expect(lastQuery(queries).getAll("cf")).toEqual([]))
  })

  it("builds a select condition from the options picked", async () => {
    const queries: string[] = []
    server.use(listing(queries))
    const user = userEvent.setup()
    renderList()
    await screen.findByTestId("commodity-card")

    await user.click(screen.getByTestId("commodities-filter-custom-fields"))
    await pickRadixSelect(user, /^Field$/, { optionLabel: /^Fuel$/ })
    const add = screen.getByTestId("commodities-filter-cf-add")
    expect(add).toBeDisabled()
    await user.click(screen.getByTestId("commodities-filter-cf-option-opt-petrol"))
    await user.click(screen.getByTestId("commodities-filter-cf-option-opt-diesel"))
    await user.click(add)

    await waitFor(() =>
      expect(lastQuery(queries).getAll("cf")).toEqual(["cf-fuel:in:opt-petrol,opt-diesel"])
    )
    expect(screen.getByTestId("commodities-filter-cf-condition-0")).toHaveTextContent(
      "Fuel: Petrol, Diesel"
    )
  })

  it("marks a condition on a deleted field so it can be removed", async () => {
    const queries: string[] = []
    server.use(listing(queries))
    const user = userEvent.setup()
    renderList(`/g/${SLUG}/commodities?cf=cf-gone%3Aset`)
    await screen.findByTestId("commodity-card")

    await user.click(screen.getByTestId("commodities-filter-custom-fields"))
    const condition = await screen.findByTestId("commodities-filter-cf-condition-0")
    expect(condition).toHaveTextContent("Deleted field")
    expect(condition).toHaveAttribute("data-stale", "true")
    await user.click(screen.getByTestId("commodities-filter-cf-remove-0"))
    await waitFor(() => expect(lastQuery(queries).getAll("cf")).toEqual([]))
  })

  it("sorts by a custom field and reverses on a second pick", async () => {
    const queries: string[] = []
    server.use(listing(queries))
    const user = userEvent.setup()
    renderList()
    await screen.findByTestId("commodity-card")

    await user.click(screen.getByTestId("commodities-sort"))
    // Long text and links do not sort.
    expect(screen.queryByTestId("commodities-sort-cf-cf-notes")).not.toBeInTheDocument()
    expect(screen.queryByTestId("commodities-sort-cf-cf-charger")).not.toBeInTheDocument()
    await user.click(await screen.findByTestId("commodities-sort-cf-cf-odo"))
    await waitFor(() => expect(lastQuery(queries).get("sort")).toBe("-cf.cf-odo"))
    expect(screen.getByTestId("commodities-sort")).toHaveTextContent("Odometer")

    await user.click(screen.getByTestId("commodities-sort"))
    await user.click(await screen.findByTestId("commodities-sort-cf-cf-odo"))
    await waitFor(() => expect(lastQuery(queries).get("sort")).toBe("cf.cf-odo"))

    // A built-in sort replaces it.
    await user.click(screen.getByTestId("commodities-sort"))
    await user.click(await screen.findByRole("menuitemcheckbox", { name: /Date added/i }))
    await waitFor(() => expect(lastQuery(queries).get("sort")).toBe("-registered_date"))
  })

  it("shows the custom fields picked as columns in the list layout", async () => {
    const queries: string[] = []
    server.use(listing(queries))
    const user = userEvent.setup()
    renderList(`/g/${SLUG}/commodities?view=list`)
    await screen.findByTestId("commodity-row")

    await user.click(screen.getByTestId("commodities-custom-field-columns"))
    await user.click(await screen.findByTestId("commodities-custom-field-column-cf-plate"))
    await user.click(screen.getByTestId("commodities-custom-field-column-cf-fuel"))
    await user.click(screen.getByTestId("commodities-custom-field-column-cf-charger"))
    await user.click(screen.getByTestId("commodities-custom-field-column-cf-odo"))
    await user.keyboard("{Escape}")

    expect(screen.getByTestId("commodities-column-header-cf-plate")).toHaveTextContent("Plate")
    const row = screen.getByTestId("commodity-row")
    expect(within(row).getByTestId("commodity-row-cf-cf-plate")).toHaveTextContent("AB 123")
    expect(within(row).getByTestId("commodity-row-cf-cf-fuel")).toHaveTextContent("Diesel")
    expect(within(row).getByTestId("commodity-row-cf-cf-charger")).toHaveTextContent("Wall charger")
    expect(within(row).getByTestId("commodity-row-cf-cf-odo")).toHaveTextContent("—")
    expect(
      JSON.parse(localStorage.getItem(`commodities:customFieldColumns:${SLUG}`) ?? "")
    ).toEqual(["cf-plate", "cf-fuel", "cf-charger", "cf-odo"])
  })

  it("names the custom field a search matched", async () => {
    server.use(
      msw.get(apiUrl(`/g/${SLUG}/commodities`), () =>
        HttpResponse.json({
          data: [van],
          meta: {
            commodities: 1,
            matches: {
              c1: [{ field: "custom_fields", custom_field_id: "cf-plate", value: "AB 123" }],
            },
          },
        })
      )
    )
    renderList(`/g/${SLUG}/commodities?q=AB`)
    expect(await screen.findByTestId("commodity-search-match")).toHaveTextContent("Plate: AB 123")
  })

  it("keeps at most four columns and drops a deleted field", async () => {
    localStorage.setItem(
      `commodities:customFieldColumns:${SLUG}`,
      JSON.stringify(["cf-gone", "cf-plate"])
    )
    server.use(listing([]))
    const user = userEvent.setup()
    renderList(`/g/${SLUG}/commodities?view=list`)
    await screen.findByTestId("commodity-row")
    expect(screen.getByTestId("commodities-column-header-cf-plate")).toBeInTheDocument()
    expect(screen.queryByTestId("commodities-column-header-cf-gone")).not.toBeInTheDocument()

    await user.click(screen.getByTestId("commodities-custom-field-columns"))
    await user.click(await screen.findByTestId("commodities-custom-field-column-cf-fuel"))
    await user.click(screen.getByTestId("commodities-custom-field-column-cf-odo"))
    await user.click(screen.getByTestId("commodities-custom-field-column-cf-notes"))
    expect(screen.getByTestId("commodities-custom-field-column-cf-charger")).toHaveAttribute(
      "data-disabled"
    )
  })
})
