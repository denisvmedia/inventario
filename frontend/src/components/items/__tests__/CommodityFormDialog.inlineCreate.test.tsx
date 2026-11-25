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
import { apiUrl, authHandlers, groupHandlers, memberHandlers } from "@/test/handlers"

const SLUG = "household"

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

// The step-by-step dialog in edit mode hosts the same Location / Area
// picker as the edit form (#2796). The create dialog opens over it, and
// Enter there must neither move the wizard on nor submit the item.
describe("<CommodityFormDialog /> creating an area from the picker", () => {
  it("creates and selects the area and stays on Basics", async () => {
    const user = userEvent.setup()
    setAccessToken("good-token")
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list([{ id: "g1", slug: SLUG, name: "Household", group_currency: "USD" }]),
      ...memberHandlers.list("g1", [
        { id: "m1", attributes: { member_user_id: "u1", role: "admin" } },
      ]),
      http.post(apiUrl(`/g/${SLUG}/areas`), () =>
        HttpResponse.json(
          { data: { id: "a3", type: "areas", attributes: { name: "Pantry", location_id: "l1" } } },
          { status: 201 }
        )
      )
    )
    const onSubmit = vi.fn().mockResolvedValue(undefined)
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

    const areaTrigger = await screen.findByRole("combobox", { name: /^Area$/i })
    await waitFor(() => expect(areaTrigger).toHaveTextContent("Garage"))
    await user.click(areaTrigger)
    const listbox = await screen.findByRole("listbox")
    await user.click(await within(listbox).findByRole("option", { name: /new area/i }))
    const dialog = await screen.findByTestId("area-form-dialog")
    await user.type(within(dialog).getByTestId("area-name-input"), "Pantry{Enter}")

    await waitFor(() => expect(screen.queryByTestId("area-form-dialog")).toBeNull())
    expect(screen.getByRole("combobox", { name: /^Area$/i })).toHaveTextContent("Pantry")
    expect(screen.getByTestId("commodity-form-basics-step")).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })
})
