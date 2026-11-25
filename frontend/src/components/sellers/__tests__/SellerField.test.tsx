import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { useState } from "react"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { SellerField } from "@/components/sellers/SellerField"
import { GroupProvider } from "@/features/group/GroupContext"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { capture } from "@/test/capture"
import { groupHandlers, sellerHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

const SLUG = "household"
const hardware = { id: "s1", name: "Hardware Store", item_count: 2 }
const market = { id: "s2", name: "Corner Market", item_count: 0 }

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
})

// Harness keeps the picked id in state, as the item form does, and shows it.
function Harness({ initial = "", suggestion = "" }: { initial?: string; suggestion?: string }) {
  const [value, setValue] = useState(initial)
  const [scanned, setScanned] = useState(suggestion)
  return (
    <>
      <SellerField
        id="seller"
        value={value}
        onChange={setValue}
        suggestion={scanned}
        onSuggestionDone={() => setScanned("")}
      />
      <output data-testid="picked">{value}</output>
    </>
  )
}

function renderField(props: { initial?: string; suggestion?: string } = {}) {
  return renderWithProviders({
    initialPath: `/g/${SLUG}/x`,
    routes: (
      <Route
        path="/g/:groupSlug/x"
        element={
          <GroupProvider>
            <Harness {...props} />
          </GroupProvider>
        }
      />
    ),
  })
}

function seed(sellers = [hardware, market]) {
  server.use(
    ...groupHandlers.list([{ id: "g1", slug: SLUG, name: "Household", group_currency: "USD" }]),
    ...sellerHandlers.list(SLUG, sellers)
  )
}

describe("<SellerField />", () => {
  it("picks a seller of the group and clears it", async () => {
    seed()
    const user = userEvent.setup()
    renderField()
    const trigger = await screen.findByRole("combobox", { name: "Seller" })
    await user.click(trigger)
    await user.click(await screen.findByRole("option", { name: "Corner Market" }))
    expect(screen.getByTestId("picked")).toHaveTextContent("s2")
    expect(trigger).toHaveTextContent("Corner Market")

    await user.click(screen.getByTestId("seller-clear"))
    expect(screen.getByTestId("picked")).toBeEmptyDOMElement()
    expect(trigger).toHaveTextContent("Pick a seller")
  })

  it("creates a seller from the list and selects it", async () => {
    const body = capture<{ data: { attributes: { name: string } } }>()
    seed()
    server.use(
      ...sellerHandlers.create(SLUG, { id: "s3", name: "Garden Center", item_count: 0 }, body)
    )
    const user = userEvent.setup()
    renderField()
    await user.click(await screen.findByRole("combobox", { name: "Seller" }))
    await user.click(await screen.findByTestId("seller-new"))
    const dialog = await screen.findByTestId("seller-form-dialog")
    await user.type(within(dialog).getByTestId("seller-name-input"), "Garden Center{Enter}")

    await waitFor(() => expect(screen.queryByTestId("seller-form-dialog")).toBeNull())
    expect(body.value?.data.attributes.name).toBe("Garden Center")
    expect(screen.getByTestId("picked")).toHaveTextContent("s3")
    // The new seller shows before the list is fetched again.
    expect(screen.getByRole("combobox", { name: "Seller" })).toHaveTextContent("Garden Center")
  })

  it("offers to create the seller the scan read, with its name filled in", async () => {
    seed()
    server.use(...sellerHandlers.create(SLUG, { id: "s3", name: "Mega Mart", item_count: 0 }))
    const user = userEvent.setup()
    renderField({ suggestion: "Mega Mart" })
    const box = await screen.findByTestId("seller-suggestion")
    expect(box).toHaveTextContent("The scan read the seller “Mega Mart”.")
    await user.click(within(box).getByTestId("seller-suggestion-create"))
    const dialog = await screen.findByTestId("seller-form-dialog")
    expect(within(dialog).getByTestId("seller-name-input")).toHaveValue("Mega Mart")
    await user.click(within(dialog).getByTestId("seller-form-submit"))

    await waitFor(() => expect(screen.getByTestId("picked")).toHaveTextContent("s3"))
    expect(screen.queryByTestId("seller-suggestion")).toBeNull()
  })

  it("offers the existing seller when the scanned name matches one", async () => {
    seed()
    const user = userEvent.setup()
    renderField({ suggestion: "hardware  STORE" })
    const use = await screen.findByTestId("seller-suggestion-use")
    expect(use).toHaveTextContent("Use Hardware Store")
    expect(screen.queryByTestId("seller-suggestion-create")).toBeNull()
    await user.click(use)
    expect(screen.getByTestId("picked")).toHaveTextContent("s1")
    expect(screen.queryByTestId("seller-suggestion")).toBeNull()
  })

  it("creates nothing when the dialog is canceled", async () => {
    seed([])
    const user = userEvent.setup()
    renderField({ suggestion: "Mega Mart" })
    await user.click(await screen.findByTestId("seller-suggestion-create"))
    const dialog = await screen.findByTestId("seller-form-dialog")
    await user.click(within(dialog).getByTestId("seller-form-cancel"))
    await waitFor(() => expect(screen.queryByTestId("seller-form-dialog")).toBeNull())
    expect(screen.getByTestId("picked")).toBeEmptyDOMElement()
    expect(screen.getByTestId("seller-suggestion")).toBeVisible()
  })
})
