import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { SuppliesTab } from "@/components/supplies/SuppliesTab"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { groupHandlers, supplyHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const COMMODITY_ID = "commodity-1"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  // Pre-arm the http client's slug slot so the very first list request
  // hits the right /g/{slug}/... path before GroupProvider's mirror
  // effect runs. Same dance as LendTab.test.tsx.
  setCurrentGroupSlug(SLUG)
})

function renderTab() {
  return renderWithProviders({
    initialPath: `/g/${SLUG}/commodities/${COMMODITY_ID}`,
    routes: (
      <Route
        path="/g/:groupSlug/commodities/:id"
        element={
          <ConfirmProvider>
            <GroupProvider>
              <main>
                <SuppliesTab commodityId={COMMODITY_ID} />
              </main>
            </GroupProvider>
          </ConfirmProvider>
        }
      />
    ),
  })
}

const linkFixture = (
  overrides: Partial<{ id: string; label: string; url: string; sort_order: number }> = {}
) => ({
  id: overrides.id ?? "supply-1",
  commodity_id: COMMODITY_ID,
  label: overrides.label ?? "Water filter",
  url: overrides.url ?? "https://example.com/water-filter",
  notes: "",
  sort_order: overrides.sort_order ?? 0,
})

describe("<SuppliesTab />", () => {
  it("renders the empty state when there are no supply links", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...supplyHandlers.listForCommodity(SLUG, COMMODITY_ID, [])
    )
    renderTab()
    expect(await screen.findByTestId("supplies-empty")).toBeInTheDocument()
    expect(screen.getByTestId("supplies-add")).toBeInTheDocument()
    expect(screen.queryByTestId("supplies-list")).not.toBeInTheDocument()
  })

  it("renders the list of supply links in sort order", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...supplyHandlers.listForCommodity(SLUG, COMMODITY_ID, [
        linkFixture({ id: "a", label: "Filter A", sort_order: 0 }),
        linkFixture({ id: "b", label: "Filter B", sort_order: 1 }),
      ])
    )
    renderTab()
    const rows = await screen.findAllByTestId("supplies-row")
    expect(rows).toHaveLength(2)
    const labels = await screen.findAllByTestId("supplies-row-label")
    expect(labels.map((el) => el.textContent)).toEqual(["Filter A", "Filter B"])
  })

  it("opens the add dialog when the add button is clicked", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...supplyHandlers.listForCommodity(SLUG, COMMODITY_ID, [])
    )
    const user = userEvent.setup()
    renderTab()
    await user.click(await screen.findByTestId("supplies-add"))
    expect(await screen.findByTestId("supply-link-dialog")).toBeVisible()
  })

  it("submits a new supply link and refreshes the list", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...supplyHandlers.listForCommodity(SLUG, COMMODITY_ID, []),
      ...supplyHandlers.create(
        SLUG,
        COMMODITY_ID,
        linkFixture({ id: "supply-new", label: "Espresso beans", url: "https://example.com/beans" })
      )
    )
    const user = userEvent.setup()
    renderTab()

    await user.click(await screen.findByTestId("supplies-add"))
    const dialog = await screen.findByTestId("supply-link-dialog")
    await user.type(
      dialog.querySelector('[data-testid="supply-link-label-input"]')!,
      "Espresso beans"
    )
    await user.type(
      dialog.querySelector('[data-testid="supply-link-url-input"]')!,
      "https://example.com/beans"
    )

    // Swap the list handler so the refetch after a successful create
    // returns the new row. The first listForCommodity above already
    // resolved the empty page; once the mutation invalidates the
    // query the second handler wins.
    server.use(
      ...supplyHandlers.listForCommodity(SLUG, COMMODITY_ID, [
        linkFixture({ id: "supply-new", label: "Espresso beans", sort_order: 0 }),
      ])
    )

    await user.click(dialog.querySelector('[data-testid="supply-link-submit"]')!)
    await waitFor(() => expect(screen.queryByTestId("supply-link-dialog")).toBeNull())
    expect(await screen.findByText("Espresso beans")).toBeInTheDocument()
  })

  it("renders the external open link with target=_blank and rel=noopener", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...supplyHandlers.listForCommodity(SLUG, COMMODITY_ID, [
        linkFixture({ id: "supply-1", label: "Water filter", url: "https://example.com/refill" }),
      ])
    )
    renderTab()
    const openAnchor = await screen.findByTestId("supplies-row-open")
    expect(openAnchor).toHaveAttribute("href", "https://example.com/refill")
    expect(openAnchor).toHaveAttribute("target", "_blank")
    expect(openAnchor).toHaveAttribute("rel", "noopener noreferrer")
  })

  it("disables the move-up button on the first row and move-down on the last", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...supplyHandlers.listForCommodity(SLUG, COMMODITY_ID, [
        linkFixture({ id: "a", label: "First", sort_order: 0 }),
        linkFixture({ id: "b", label: "Last", sort_order: 1 }),
      ])
    )
    renderTab()
    const rows = await screen.findAllByTestId("supplies-row")
    const firstUp = rows[0].querySelector('[data-testid="supplies-move-up"]') as HTMLButtonElement
    const lastDown = rows[1].querySelector(
      '[data-testid="supplies-move-down"]'
    ) as HTMLButtonElement
    expect(firstUp).toBeDisabled()
    expect(lastDown).toBeDisabled()
  })
})
