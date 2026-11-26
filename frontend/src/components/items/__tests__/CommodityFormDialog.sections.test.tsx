import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"

import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import type { Commodity } from "@/features/commodities/api"
import { GroupProvider } from "@/features/group/GroupContext"
import type { GroupSection } from "@/features/group/sections"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import {
  apiUrl,
  authHandlers,
  commodityScanHandlers,
  draftHandlers,
  groupHandlers,
  memberHandlers,
  sellerHandlers,
} from "@/test/handlers"

const SLUG = "household"

const lamp: Commodity = {
  id: "c1",
  name: "Lamp",
  short_name: "Lamp",
  type: "other",
  area_id: "a1",
  status: "in_use",
  count: 1,
}

function serveGroup(hidden: GroupSection[]) {
  server.use(
    ...groupHandlers.list([
      { id: "g1", slug: SLUG, name: "Household", group_currency: "USD", hidden_sections: hidden },
    ])
  )
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  server.use(
    ...authHandlers.signedIn(),
    ...memberHandlers.list("g1", [
      { id: "m1", attributes: { member_user_id: "u1", role: "admin" } },
    ]),
    ...sellerHandlers.list(SLUG, []),
    http.get(apiUrl(`/currencies`), () => HttpResponse.json(["USD", "EUR"]))
  )
})

function renderDialog(item: Commodity, onSubmit = vi.fn().mockResolvedValue(undefined)) {
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
              initialValues={item}
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

async function stepNames(): Promise<string[]> {
  const stepper = await screen.findByRole("list", { name: /form steps/i })
  return within(stepper)
    .getAllByRole("button")
    .map((b) => b.getAttribute("aria-label") ?? "")
}

describe("<CommodityFormDialog /> in a group that hides sections (#1607)", () => {
  it("offers the Warranty step while the group shows warranties", async () => {
    serveGroup(["loans"])
    renderDialog(lamp)
    await screen.findByTestId("commodity-form-basics-step")
    expect(await stepNames()).toEqual(["Basics", "Purchase", "Warranty", "Extras", "Files"])
  })

  it("leaves the Warranty step out while the group hides warranties", async () => {
    serveGroup(["warranties"])
    const user = userEvent.setup()
    const onSubmit = renderDialog(lamp)
    await screen.findByTestId("commodity-form-basics-step")
    // The group loads after the dialog opens; the step goes once it has.
    await waitFor(async () =>
      expect(await stepNames()).toEqual(["Basics", "Purchase", "Extras", "Files"])
    )
    expect(screen.getByText(/Step 1 of 4/)).toBeInTheDocument()

    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-form-purchase-step")
    await user.click(screen.getByTestId("commodity-form-next"))
    expect(await screen.findByTestId("commodity-form-extras-step")).toBeInTheDocument()
    expect(screen.queryByTestId("commodity-form-warranty-step")).not.toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: /^Back$/i }))
    expect(await screen.findByTestId("commodity-form-purchase-step")).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("keeps the Warranty step for an item that has a warranty", async () => {
    serveGroup(["warranties"])
    const user = userEvent.setup()
    const onSubmit = renderDialog({
      ...lamp,
      warranty_expires_at: "2030-01-31",
      warranty_notes: "Two years from the shop",
    })
    await screen.findByTestId("commodity-form-basics-step")
    expect(await stepNames()).toEqual(["Basics", "Purchase", "Warranty", "Extras", "Files"])

    const stepper = screen.getByRole("list", { name: /form steps/i })
    await user.click(within(stepper).getByRole("button", { name: "Warranty" }))
    expect(await screen.findByTestId("commodity-form-warranty-expires-at")).toHaveValue(
      "2030-01-31"
    )

    // Saving keeps what the item holds.
    await user.click(screen.getByTestId("commodity-form-submit"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalled())
    expect(onSubmit.mock.calls[0][0]).toMatchObject({
      warranty_expires_at: "2030-01-31",
      warranty_notes: "Two years from the shop",
    })
  })
})

// The Add item dialog in create mode, with server drafts, resuming draftId
// when given.
function renderCreate(draftId?: string) {
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
              mode="create"
              areas={[{ id: "a1", name: "Garage", location_id: "l1" }]}
              locations={[{ id: "l1", name: "Home" }]}
              defaultCurrency="USD"
              onSubmit={vi.fn().mockResolvedValue({ id: "new-item" })}
              draftKey={`commodity-draft:${SLUG}:create`}
              serverDrafts
              resumeDraftId={draftId}
            />
          </GroupProvider>
        }
      />
    ),
  })
}

describe("<CommodityFormDialog /> create mode in a group that hides warranties (#1607)", () => {
  beforeEach(() => {
    window.localStorage.clear()
    window.sessionStorage.clear()
  })

  it("keeps the Warranty step for a resumed draft that holds a warranty", async () => {
    serveGroup(["warranties"])
    const drafts = draftHandlers.backend(SLUG, [
      {
        id: "d1",
        revision: 1,
        values: {
          name: "Drill set",
          short_name: "Drills",
          type: "other",
          area_id: "a1",
          count: 3,
          warranty_expires_at: "2030-01-31",
        },
      },
    ])
    server.use(...drafts.handlers)
    const user = userEvent.setup()
    renderCreate("d1")

    // A bundle cannot carry a warranty, so the draft opens on Basics with
    // the quantity marked, as it does in a group that shows warranties.
    const count = await screen.findByLabelText(/^Quantity/)
    await waitFor(() => expect(count).toHaveValue(3))
    await waitFor(() => expect(count).toHaveAttribute("aria-invalid", "true"))
    expect(await stepNames()).toEqual(["Basics", "Purchase", "Warranty", "Extras", "Files"])

    await user.clear(count)
    await user.type(count, "1")
    await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-form-purchase-step")
    await user.click(screen.getByTestId("commodity-form-next"))
    expect(await screen.findByTestId("commodity-form-warranty-expires-at")).toHaveValue(
      "2030-01-31"
    )

    // Clearing the date keeps the step for the rest of this opening.
    await user.clear(screen.getByTestId("commodity-form-warranty-expires-at"))
    expect(screen.getByTestId("commodity-form-warranty-step")).toBeInTheDocument()
    expect(await stepNames()).toContain("Warranty")
  })

  it("leaves the warranty date out of a scan and keeps the step out", async () => {
    serveGroup(["warranties"])
    server.use(
      ...commodityScanHandlers.ok(SLUG, {
        fields: {
          name: { value: "Cordless drill", confidence: 0.9 },
          warranty_expires_at: { value: "2030-01-31", confidence: 0.9 },
        },
      })
    )
    const user = userEvent.setup()
    renderCreate()

    await user.upload(
      await screen.findByTestId("commodity-form-ai-file-input"),
      new File([new Uint8Array([0xff, 0xd8, 0xff])], "receipt.jpg", { type: "image/jpeg" })
    )
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    expect(await screen.findByTestId("commodity-form-ai-row-name")).toBeInTheDocument()
    expect(
      screen.queryByTestId("commodity-form-ai-row-warranty_expires_at")
    ).not.toBeInTheDocument()

    await user.click(screen.getByTestId("commodity-form-ai-use-values"))
    await screen.findByTestId("commodity-form-basics-step")
    expect(await stepNames()).toEqual(["Basics", "Purchase", "Extras", "Files"])
  })
})
