import { screen, waitFor, within } from "@testing-library/react"
import { http, HttpResponse } from "msw"
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
import { apiUrl, groupHandlers, sellerHandlers } from "@/test/handlers"
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

interface HarnessProps {
  initial?: string
  suggestion?: string
}

// Harness keeps the picked id in state, as the item form does, and shows it.
// The "fail" button stands in for a save the server refused with a 422 on
// the seller.
function Harness({ initial = "", suggestion = "" }: HarnessProps) {
  const [value, setValue] = useState(initial)
  const [scanned, setScanned] = useState(suggestion)
  const [error, setError] = useState("")
  return (
    <>
      <SellerField
        id="seller"
        value={value}
        onChange={setValue}
        suggestion={scanned}
        onSuggestionDone={() => setScanned("")}
        error={error ? { message: error } : undefined}
      />
      <output data-testid="picked">{value}</output>
      <button type="button" onClick={() => setError("no seller with this ID in the group")}>
        fail
      </button>
    </>
  )
}

function renderField(props: HarnessProps = {}) {
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

// serveSellers answers GET /sellers from a list the test can change, and
// counts the requests.
function serveSellers(sellers: { id: string; name: string; item_count: number }[]) {
  const state = { sellers, gets: 0 }
  server.use(
    http.get(apiUrl(`/g/${SLUG}/sellers`), () => {
      state.gets++
      return HttpResponse.json({
        data: state.sellers.map((s) => ({ id: s.id, type: "sellers", attributes: s })),
        meta: { sellers: state.sellers.length },
      })
    })
  )
  return state
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
    const garden = { id: "s3", name: "Garden Center", item_count: 0 }
    seed()
    const served = serveSellers([hardware, market])
    server.use(
      http.post(apiUrl(`/g/${SLUG}/sellers`), async ({ request }) => {
        body.value = (await request.json()) as typeof body.value
        served.sellers = [...served.sellers, garden]
        return HttpResponse.json(
          { data: { id: garden.id, type: "sellers", attributes: garden } },
          { status: 201 }
        )
      })
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
    expect(screen.getByRole("combobox", { name: "Seller" })).toHaveTextContent("Garden Center")
    expect(screen.queryByTestId("seller-gone")).toBeNull()
  })

  it("keeps showing a created seller when the list cannot be fetched again", async () => {
    seed()
    const user = userEvent.setup()
    renderField()
    await user.click(await screen.findByRole("combobox", { name: "Seller" }))
    server.use(
      http.get(apiUrl(`/g/${SLUG}/sellers`), () => new HttpResponse(null, { status: 500 })),
      ...sellerHandlers.create(SLUG, { id: "s3", name: "Garden Center", item_count: 0 })
    )
    await user.click(await screen.findByTestId("seller-new"))
    const dialog = await screen.findByTestId("seller-form-dialog")
    await user.type(within(dialog).getByTestId("seller-name-input"), "Garden Center{Enter}")

    await waitFor(() => expect(screen.queryByTestId("seller-form-dialog")).toBeNull())
    expect(screen.getByTestId("picked")).toHaveTextContent("s3")
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

  it("offers the seller that has a taken name, once the list knows it", async () => {
    seed()
    // Another member created the seller after this list was loaded.
    const served = serveSellers([hardware, market])
    server.use(...sellerHandlers.nameTaken(SLUG))
    const user = userEvent.setup()
    renderField()
    await user.click(await screen.findByRole("combobox", { name: "Seller" }))
    await user.click(await screen.findByTestId("seller-new"))
    const dialog = await screen.findByTestId("seller-form-dialog")
    served.sellers = [hardware, market, { id: "s3", name: "Garden Center", item_count: 4 }]
    await user.type(within(dialog).getByTestId("seller-name-input"), "garden  center{Enter}")

    expect(await within(dialog).findByTestId("seller-name-error")).toHaveTextContent(
      "Another seller of this group has this name."
    )
    const use = await within(dialog).findByTestId("seller-name-taken-use")
    expect(use).toHaveTextContent("Use Garden Center")
    await user.click(use)

    await waitFor(() => expect(screen.queryByTestId("seller-form-dialog")).toBeNull())
    expect(screen.getByTestId("picked")).toHaveTextContent("s3")
    expect(screen.getByRole("combobox", { name: "Seller" })).toHaveTextContent("Garden Center")
  })

  it("drops a created seller deleted elsewhere once the list is fetched again", async () => {
    seed()
    // The list never includes the new seller: it was deleted right away.
    serveSellers([hardware, market])
    server.use(...sellerHandlers.create(SLUG, { id: "s3", name: "Garden Center", item_count: 0 }))
    const user = userEvent.setup()
    renderField()
    await user.click(await screen.findByRole("combobox", { name: "Seller" }))
    await user.click(await screen.findByTestId("seller-new"))
    const dialog = await screen.findByTestId("seller-form-dialog")
    await user.type(within(dialog).getByTestId("seller-name-input"), "Garden Center{Enter}")

    await waitFor(() => expect(screen.queryByTestId("seller-form-dialog")).toBeNull())
    expect(await screen.findByTestId("seller-gone")).toHaveTextContent(
      "This seller was deleted. Pick another one or clear the seller."
    )
  })

  it("says so when the picked seller is not in the group any more", async () => {
    seed()
    const served = serveSellers([hardware, market])
    const user = userEvent.setup()
    renderField({ initial: "s9" })
    expect(await screen.findByTestId("seller-gone")).toBeVisible()
    // The list was fetched once more before the seller was called gone.
    expect(served.gets).toBe(2)
    await user.click(screen.getByTestId("seller-clear"))
    expect(screen.queryByTestId("seller-gone")).toBeNull()
  })

  it("trusts a fresh list over an old one that lacks the seller", async () => {
    seed()
    // The first list predates the seller; the next one has it.
    const garden = { id: "s3", name: "Garden Center", item_count: 1 }
    const served = { gets: 0 }
    server.use(
      http.get(apiUrl(`/g/${SLUG}/sellers`), () => {
        served.gets++
        const sellers = served.gets === 1 ? [hardware] : [hardware, garden]
        return HttpResponse.json({
          data: sellers.map((x) => ({ id: x.id, type: "sellers", attributes: x })),
          meta: { sellers: sellers.length },
        })
      })
    )
    renderField({ initial: "s3" })
    const trigger = await screen.findByRole("combobox", { name: "Seller" })
    await waitFor(() => expect(trigger).toHaveTextContent("Garden Center"))
    expect(served.gets).toBe(2)
    expect(screen.queryByTestId("seller-gone")).toBeNull()
  })

  it("does not call the seller gone when the fresh fetch fails", async () => {
    seed()
    const served = { gets: 0 }
    server.use(
      http.get(apiUrl(`/g/${SLUG}/sellers`), () => {
        served.gets++
        if (served.gets > 1) return new HttpResponse(null, { status: 500 })
        return HttpResponse.json({
          data: [{ id: hardware.id, type: "sellers", attributes: hardware }],
          meta: { sellers: 1 },
        })
      })
    )
    renderField({ initial: "s9" })
    await screen.findByRole("combobox", { name: "Seller" })
    await waitFor(() => expect(served.gets).toBe(2))
    // Give the failed fetch time to settle.
    await new Promise((resolve) => setTimeout(resolve, 50))
    expect(screen.queryByTestId("seller-gone")).toBeNull()
  })

  it("fetches the list again when the server refuses the seller", async () => {
    seed()
    const served = serveSellers([hardware, market])
    const user = userEvent.setup()
    renderField({ initial: "s2" })
    const trigger = await screen.findByRole("combobox", { name: "Seller" })
    await waitFor(() => expect(trigger).toHaveTextContent("Corner Market"))
    const before = served.gets

    // Another member deleted the seller; the save is refused with a 422.
    served.sellers = [hardware]
    await user.click(screen.getByRole("button", { name: "fail" }))

    await waitFor(() => expect(served.gets).toBe(before + 1))
    expect(await screen.findByTestId("seller-gone")).toBeVisible()
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
