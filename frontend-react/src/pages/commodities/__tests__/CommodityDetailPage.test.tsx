import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { CommodityDetailPage } from "@/pages/commodities/CommodityDetailPage"
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
    current_price: 1900,
    original_price_currency: "USD",
    serial_number: "ABC-123",
    tags: ["work", "tech"],
    purchase_date: "2024-09-12",
    draft: false,
  },
}

function renderDetail(initialPath = `/g/${SLUG}/commodities/${ID}`) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path="/g/:groupSlug/commodities/:id"
        element={
          <GroupProvider>
            <ConfirmProvider>
              <CommodityDetailPage />
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

describe("<CommodityDetailPage />", () => {
  it("renders the commodity name + key fields", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderDetail()
    await waitFor(() =>
      expect(screen.getByTestId("page-commodity-detail")).toBeInTheDocument()
    )
    expect(screen.getByRole("heading", { name: /macbook pro 16/i })).toBeInTheDocument()
    expect(screen.getByTestId("commodity-detail-short-name")).toHaveTextContent("Laptop")
    expect(screen.getByText(/garage/i)).toBeInTheDocument()
    // Currency-formatted prices show up in the Details tab.
    expect(screen.getByText(/\$1,900\.00/)).toBeInTheDocument()
  })

  it("renders an error alert when the detail endpoint 5xx", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      // Hijack the same path with a 500 — mirrors the legacy error
      // handler factory pattern. We do this inline since
      // commodityHandlers.error covers list, not detail.
    )
    server.use(
      ...commodityHandlers.detail(SLUG, ID, { errors: [{ status: "500" }] })
    )
    // The fixture above returns 200 with an unexpected envelope; force
    // a network error instead by overriding with a 500-response handler.
    const { http, HttpResponse } = await import("msw")
    server.use(
      http.get(
        `${window.location.origin}/api/v1/g/${SLUG}/commodities/${ID}`,
        () => HttpResponse.json({ error: "boom" }, { status: 500 })
      )
    )
    renderDetail()
    expect(await screen.findByTestId("commodity-detail-error")).toBeInTheDocument()
  })

  it("switches between Details / Warranty / Files tabs", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderDetail()
    await screen.findByTestId("commodity-detail-details")
    await user.click(screen.getByTestId("commodity-detail-tab-warranty"))
    expect(await screen.findByTestId("commodity-detail-warranty")).toBeInTheDocument()
    await user.click(screen.getByTestId("commodity-detail-tab-files"))
    expect(await screen.findByTestId("commodity-detail-files")).toBeInTheDocument()
  })

  it("opens the edit dialog when the Edit button is clicked", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    renderDetail()
    await screen.findByTestId("commodity-detail-edit")
    await user.click(screen.getByTestId("commodity-detail-edit"))
    // Form stepper aria-label is dialog-only.
    expect(await screen.findByLabelText(/form steps/i)).toBeInTheDocument()
  })

  it("deletes the commodity from the detail page", async () => {
    const user = userEvent.setup()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture),
      ...commodityHandlers.remove(SLUG, ID)
    )
    renderDetail()
    await screen.findByTestId("commodity-detail-delete")
    await user.click(screen.getByTestId("commodity-detail-delete"))
    // Wait for the confirm dialog body-lock, then click its Delete CTA.
    await waitFor(() => expect(document.body).toHaveAttribute("data-scroll-locked"))
    const buttons = screen.getAllByRole("button", { name: /^Delete$/i })
    await user.click(buttons[buttons.length - 1])
    // The page navigates back to the list on success — the detail
    // surface unmounts, so the not-found card should NOT appear (the
    // route is gone). Easiest assertion: header buttons disappear.
    await waitFor(() =>
      expect(screen.queryByTestId("commodity-detail-delete")).not.toBeInTheDocument()
    )
  })

  it("renders the not-found card when the commodity is missing", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, { id: ID, type: "commodities", attributes: null })
    )
    renderDetail()
    expect(await screen.findByTestId("commodity-detail-not-found")).toBeInTheDocument()
  })

  it("has no axe violations on a populated detail", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.detail(SLUG, ID, commodityFixture)
    )
    const { container } = renderDetail()
    await screen.findByTestId("commodity-detail-details")
    expect(await axe(container)).toHaveNoViolations()
  })
})
