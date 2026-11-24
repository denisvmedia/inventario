import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route } from "react-router-dom"
import { screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { http, HttpResponse } from "msw"

import { LabelPrintPage } from "@/pages/labels/LabelPrintPage"
import { GroupProvider } from "@/features/group/GroupContext"
import { defaultSheetFormat } from "@/features/labels/formats"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import {
  apiUrl,
  areaHandlers,
  commodityHandlers,
  groupHandlers,
  locationHandlers,
} from "@/test/handlers"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

const SLUG = "household"

const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

function commodity(id: string, name: string, assetNumber?: number) {
  return {
    id,
    type: "commodities",
    attributes: {
      id,
      name,
      type: "electronics",
      status: "in_use",
      count: 1,
      area_id: "a1",
      ...(assetNumber ? { asset_number: assetNumber } : {}),
    },
  }
}

const areaFixture = {
  id: "a1",
  type: "areas",
  attributes: { id: "a1", name: "Garage", location_id: "l1" },
}
const locationFixture = [{ id: "l1", type: "locations", attributes: { id: "l1", name: "Home" } }]

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

function renderLabels(query: string) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: `/g/${SLUG}/labels${query}`,
    routes: (
      <Route
        path="/g/:groupSlug/labels"
        element={
          <GroupProvider>
            <LabelPrintPage />
          </GroupProvider>
        }
      />
    ),
  })
}

async function previewLabels() {
  const preview = await screen.findByTestId("labels-preview")
  return within(preview).getAllByTestId("label")
}

describe("<LabelPrintPage />", () => {
  it("prints one item label with its asset ID and a QR code for the item page", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...commodityHandlers.detail(SLUG, "c1", commodity("c1", "MacBook Pro 16", 42))
    )
    renderLabels("?item=c1")

    const [label] = await previewLabels()
    expect(screen.getByTestId("labels-preview")).toHaveAttribute("data-format", "single")
    expect(within(label).getByTestId("label-code")).toHaveTextContent("A-0042")
    expect(within(label).getByText("MacBook Pro 16")).toBeInTheDocument()
    expect(label).toHaveAttribute(
      "data-label-url",
      `${window.location.origin}/g/${SLUG}/commodities/c1`
    )
    expect(label.querySelector("svg")).not.toBeNull()
    expect(screen.getByRole("link", { name: /back/i })).toHaveAttribute(
      "href",
      `/g/${SLUG}/commodities/c1`
    )
  })

  it("prints the area label first, then its items in asset-ID order", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.detail(SLUG, "a1", areaFixture),
      ...locationHandlers.list(SLUG, locationFixture),
      ...commodityHandlers.list(SLUG, [
        commodity("c3", "Ladder", 3),
        commodity("c9", "Unnumbered"),
        commodity("c1", "Drill", 1),
      ])
    )
    renderLabels("?area=a1&area_items=a1")

    const preview = await screen.findByTestId("labels-preview")
    // The location name arrives with its own request.
    expect(await within(preview).findByText("Home")).toBeInTheDocument()
    const labels = await previewLabels()
    expect(labels).toHaveLength(4)
    expect(labels[0]).toHaveAttribute(
      "data-label-url",
      `${window.location.origin}/g/${SLUG}/areas/a1`
    )
    expect(within(labels[0]).getByText("Garage")).toBeInTheDocument()
    expect(within(labels[0]).queryByTestId("label-code")).toBeNull()
    expect(
      labels
        .slice(1)
        .map((l) => [
          l.getAttribute("data-label-url")?.split("/").at(-1),
          within(l).queryByTestId("label-code")?.textContent ?? null,
        ])
    ).toEqual([
      ["c1", "A-0001"],
      ["c3", "A-0003"],
      ["c9", null],
    ])
    expect(screen.getByTestId("labels-preview")).toHaveAttribute(
      "data-format",
      defaultSheetFormat(navigator.language)
    )
    expect(screen.getByRole("link", { name: /back/i })).toHaveAttribute(
      "href",
      `/g/${SLUG}/areas/a1`
    )
  })

  it("leaves skipped positions empty on a sheet", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...commodityHandlers.detail(SLUG, "c1", commodity("c1", "Drill", 1)),
      ...commodityHandlers.detail(SLUG, "c2", commodity("c2", "Ladder", 2))
    )
    renderLabels("?item=c1&item=c2&format=a4-3x7&skip=4")

    await previewLabels()
    const preview = screen.getByTestId("labels-preview")
    expect(preview).toHaveAttribute("data-format", "a4-3x7")
    const [sheet] = within(preview).getAllByTestId("label-sheet")
    expect(sheet.children).toHaveLength(6)
    expect(screen.getByTestId("labels-skip")).toHaveValue(4)
    expect(screen.getByTestId("labels-summary")).toHaveTextContent("2 labels · 1 page")
  })

  it("calls window.print from the Print button", async () => {
    const user = userEvent.setup()
    const printSpy = vi.fn()
    Object.defineProperty(window, "print", { configurable: true, writable: true, value: printSpy })
    server.use(
      ...groupHandlers.list(groupFixture),
      ...commodityHandlers.detail(SLUG, "c1", commodity("c1", "Drill", 1))
    )
    renderLabels("?item=c1")
    await previewLabels()
    await user.click(screen.getByTestId("labels-print"))
    expect(printSpy).toHaveBeenCalledTimes(1)
  })

  it("shows an empty state and disables Print when nothing is selected", async () => {
    server.use(...groupHandlers.list(groupFixture))
    renderLabels("")
    expect(await screen.findByTestId("labels-empty")).toBeInTheDocument()
    expect(screen.getByTestId("labels-print")).toBeDisabled()
  })

  it("reports labels that could not be loaded and keeps the rest", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...commodityHandlers.detail(SLUG, "c1", commodity("c1", "Drill", 1)),
      http.get(apiUrl(`/g/${SLUG}/commodities/gone`), () =>
        HttpResponse.json({ error: "not found" }, { status: 404 })
      )
    )
    renderLabels("?item=c1&item=gone")
    expect(await screen.findByTestId("labels-error")).toBeInTheDocument()
    expect(await previewLabels()).toHaveLength(1)
  })

  it("has no axe violations", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...commodityHandlers.detail(SLUG, "c1", commodity("c1", "Drill", 1))
    )
    const { container } = renderLabels("?item=c1")
    await previewLabels()
    expect(await axe(container)).toHaveNoViolations()
  })
})
