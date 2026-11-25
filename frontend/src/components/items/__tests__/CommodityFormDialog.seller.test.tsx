import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"

import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import { GroupProvider } from "@/features/group/GroupContext"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import {
  apiUrl,
  authHandlers,
  commodityScanHandlers,
  groupHandlers,
  memberHandlers,
  sellerHandlers,
} from "@/test/handlers"

const SLUG = "household"

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  server.use(
    ...authHandlers.signedIn(),
    ...groupHandlers.list([{ id: "g1", slug: SLUG, name: "Household", group_currency: "USD" }]),
    ...memberHandlers.list("g1", [
      { id: "m1", attributes: { member_user_id: "u1", role: "admin" } },
    ]),
    ...sellerHandlers.list(SLUG, [{ id: "s1", name: "Hardware Store", item_count: 1 }]),
    http.get(apiUrl(`/currencies`), () => HttpResponse.json(["USD", "EUR"]))
  )
})

function renderDialog(onSubmit = vi.fn().mockResolvedValue(undefined)) {
  renderWithProviders({
    withAuth: true,
    initialPath: `/g/${SLUG}/x`,
    routes: (
      <Route
        path="/g/:groupSlug/x"
        element={
          <GroupProvider>
            <CommodityFormDialog
              open
              onOpenChange={() => {}}
              mode="edit"
              initialValues={{
                id: "c1",
                name: "Lamp",
                short_name: "Lamp",
                type: "other",
                area_id: "a1",
                status: "in_use",
                count: 1,
                draft: true,
              }}
              areas={[{ id: "a1", name: "Garage", location_id: "l1" }]}
              locations={[{ id: "l1", name: "Home" }]}
              defaultCurrency="USD"
              onSubmit={onSubmit}
            />
          </GroupProvider>
        }
      />
    ),
  })
  return onSubmit
}

// scanSeller rewinds to the AI step, scans a receipt that names seller,
// accepts the values and walks on to the Purchase step.
async function scanSeller(user: ReturnType<typeof userEvent.setup>, seller: string) {
  server.use(
    ...commodityScanHandlers.ok(SLUG, {
      fields: { seller: { value: seller, confidence: 0.9 } },
    })
  )
  await screen.findByTestId("commodity-form-basics-step")
  await user.click(screen.getByRole("button", { name: /^Back$/i }))
  await user.upload(
    await screen.findByTestId("commodity-form-ai-file-input"),
    new File([new Uint8Array([0xff, 0xd8, 0xff])], "receipt.jpg", { type: "image/jpeg" })
  )
  await user.click(screen.getByTestId("commodity-form-ai-scan"))
  expect(await screen.findByTestId("commodity-form-ai-row-seller")).toHaveTextContent("Seller")
  await user.click(screen.getByTestId("commodity-form-ai-use-values"))
  await screen.findByTestId("commodity-form-basics-step")
  await user.click(screen.getByTestId("commodity-form-next"))
  return screen.findByTestId("commodity-form-purchase-step")
}

describe("<CommodityFormDialog /> seller from the AI scan", () => {
  it("picks the group's seller whose name the scan read", async () => {
    const user = userEvent.setup()
    renderDialog()
    const step = await scanSeller(user, "  HARDWARE store ")
    await waitFor(() =>
      expect(within(step).getByRole("combobox", { name: "Seller" })).toHaveTextContent(
        "Hardware Store"
      )
    )
    expect(within(step).queryByTestId("commodity-seller-suggestion")).toBeNull()
  })

  it("creates no seller on its own for a name the group does not have", async () => {
    const posted = vi.fn()
    server.use(
      http.post(apiUrl(`/g/${SLUG}/sellers`), () => {
        posted()
        return HttpResponse.json({}, { status: 500 })
      })
    )
    const user = userEvent.setup()
    renderDialog()
    const step = await scanSeller(user, "Mega Mart")
    const box = within(step).getByTestId("commodity-seller-suggestion")
    expect(box).toHaveTextContent("The scan read the seller “Mega Mart”.")
    expect(within(box).getByTestId("commodity-seller-suggestion-create")).toBeVisible()
    expect(within(step).getByRole("combobox", { name: "Seller" })).toHaveTextContent(
      "Pick a seller"
    )
    expect(posted).not.toHaveBeenCalled()
  })
})
