import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route } from "react-router-dom"
import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { CommodityPrintPage } from "@/pages/commodities/CommodityPrintPage"
import { GroupProvider } from "@/features/group/GroupContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { areaHandlers, commodityHandlers, groupHandlers } from "@/test/handlers"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

const SLUG = "household"
const ID = "c1"

const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", main_currency: "USD" },
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
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderPrint()
    expect(await screen.findByTestId("page-commodity-print")).toBeInTheDocument()
    expect(screen.getByRole("heading", { name: /macbook pro 16/i })).toBeInTheDocument()
    expect(screen.getByText(/garage/i)).toBeInTheDocument()
    expect(screen.getByText(/\$2,400\.00/)).toBeInTheDocument()
    expect(screen.getByText(/daily driver/i)).toBeInTheDocument()
  })

  it("calls window.print when the Print button is clicked", async () => {
    const user = userEvent.setup()
    const printSpy = vi.fn()
    Object.defineProperty(window, "print", {
      configurable: true,
      writable: true,
      value: printSpy,
    })
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderPrint()
    await user.click(await screen.findByTestId("commodity-print-trigger"))
    expect(printSpy).toHaveBeenCalledTimes(1)
  })

  it("has no axe violations", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    const { container } = renderPrint()
    await screen.findByTestId("page-commodity-print")
    expect(await axe(container)).toHaveNoViolations()
  })
})
