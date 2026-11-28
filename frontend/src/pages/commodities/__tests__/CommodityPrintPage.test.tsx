import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route } from "react-router-dom"
import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { CommodityPrintPage } from "@/pages/commodities/CommodityPrintPage"
import { GroupProvider } from "@/features/group/GroupContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import {
  areaHandlers,
  commodityHandlers,
  customFieldHandlers,
  groupHandlers,
  sellerHandlers,
} from "@/test/handlers"
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
    original_price_currency: "USD",
    purchase_date: "2024-09-12",
    tags: ["work"],
    comments: "Daily driver",
    extra_serial_numbers: ["X-1"],
    part_numbers: ["P-1"],
  },
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

// The lists every print reads: the group, its areas and its sellers.
function listHandlers() {
  return [
    ...groupHandlers.list(groupFixture),
    ...areaHandlers.list(SLUG, areaFixture),
    ...sellerHandlers.list(SLUG, [{ id: "s1", name: "Apple Store" }]),
  ]
}

function renderPrint() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: `/g/${SLUG}/commodities/${ID}/print`,
    routes: (
      <Route
        path="/g/:groupSlug/commodities/:id/print"
        element={
          <GroupProvider>
            <CommodityPrintPage />
          </GroupProvider>
        }
      />
    ),
  })
}

describe("<CommodityPrintPage />", () => {
  it("renders the print sections with the commodity data", async () => {
    server.use(...listHandlers(), ...commodityHandlers.detail(SLUG, ID, commodityFixture))
    renderPrint()
    expect(await screen.findByTestId("page-commodity-print")).toBeInTheDocument()
    expect(screen.getByRole("heading", { name: /macbook pro 16/i })).toBeInTheDocument()
    expect(screen.getByText(/garage/i)).toBeInTheDocument()
    expect(screen.getByText(/\$2,400\.00/)).toBeInTheDocument()
    expect(screen.getByText(/daily driver/i)).toBeInTheDocument()
    expect(screen.getByTestId("commodity-print-header-date").textContent).toMatch(/\b20\d{2}\b/)
  })

  it("shows the brand, model, seller and custom fields the item has", async () => {
    server.use(
      ...listHandlers(),
      ...customFieldHandlers.list(SLUG, [
        customFieldHandlers.field({ id: "cf1", name: "Color" }),
        customFieldHandlers.field({ id: "cf2", name: "Insured", type: "boolean" }),
        customFieldHandlers.field({ id: "cf3", name: "Room" }),
      ]),
      ...commodityHandlers.detail(SLUG, ID, {
        ...commodityFixture,
        attributes: {
          ...commodityFixture.attributes,
          brand: "Apple",
          model: "A2991",
          seller_id: "s1",
          custom_fields: { cf1: "Space Black", cf2: true },
        },
      })
    )
    renderPrint()
    expect(await screen.findByTestId("commodity-print-brand-model")).toHaveTextContent(
      "Apple · A2991"
    )
    expect(screen.getByTestId("commodity-print-seller")).toHaveTextContent("Apple Store")
    expect(screen.getByTestId("commodity-print-custom-field-cf1")).toHaveTextContent(
      "ColorSpace Black"
    )
    expect(screen.getByTestId("commodity-print-custom-field-cf2")).toHaveTextContent("InsuredYes")
    // A field without a value for this item is left out.
    expect(screen.queryByTestId("commodity-print-custom-field-cf3")).toBeNull()
  })

  it("leaves out the brand, model and seller of an item without them", async () => {
    server.use(...listHandlers(), ...commodityHandlers.detail(SLUG, ID, commodityFixture))
    renderPrint()
    expect(await screen.findByTestId("page-commodity-print")).toBeInTheDocument()
    expect(screen.queryByTestId("commodity-print-brand-model")).toBeNull()
    expect(screen.queryByTestId("commodity-print-seller")).toBeNull()
    expect(screen.queryByText(/seller/i)).toBeNull()
  })

  it("calls window.print when the Print button is clicked", async () => {
    const user = userEvent.setup()
    const printSpy = vi.fn()
    Object.defineProperty(window, "print", {
      configurable: true,
      writable: true,
      value: printSpy,
    })
    server.use(...listHandlers(), ...commodityHandlers.detail(SLUG, ID, commodityFixture))
    renderPrint()
    await user.click(await screen.findByTestId("commodity-print-trigger"))
    expect(printSpy).toHaveBeenCalledTimes(1)
  })

  it("has no axe violations", async () => {
    server.use(...listHandlers(), ...commodityHandlers.detail(SLUG, ID, commodityFixture))
    const { container } = renderPrint()
    await screen.findByTestId("page-commodity-print")
    expect(await axe(container)).toHaveNoViolations()
  })
})
