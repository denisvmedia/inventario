import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { SellersListPage } from "@/pages/sellers/SellersListPage"
import { capture } from "@/test/capture"
import { groupHandlers, memberHandlers, sellerHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { WhenRoleKnown } from "@/test/role-known"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

const hardware = {
  id: "s1",
  name: "Hardware Store",
  website: "https://hw.example/shop",
  contact: "Ann, 555-0100",
  note: "Cash only",
  item_count: 3,
}
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

function renderPage(path = `/g/${SLUG}/sellers`) {
  return renderWithProviders({
    withAuth: true,
    initialPath: path,
    routes: (
      <Route
        path="/g/:groupSlug/sellers"
        element={
          <ConfirmProvider>
            <GroupProvider>
              <main>
                <WhenRoleKnown>
                  <SellersListPage />
                </WhenRoleKnown>
              </main>
            </GroupProvider>
          </ConfirmProvider>
        }
      />
    ),
  })
}

// The page offers write actions by the caller's role. Every test runs as
// the group's owner unless it registers another role with
// memberHandlers.asRole before rendering.
beforeEach(() => {
  server.use(...memberHandlers.asRole("g1", "owner"))
})

function seed(sellers = [market, hardware]) {
  server.use(...groupHandlers.list(groupFixture), ...sellerHandlers.list(SLUG, sellers))
}

describe("<SellersListPage />", () => {
  it("lists the sellers with their item counts and details", async () => {
    seed()
    renderPage()
    // The top bar and the browser tab name the page.
    await waitFor(() => expect(document.title).toBe("Sellers · Inventario"))
    expect(await screen.findByTestId("seller-row-s1")).toBeVisible()
    expect(screen.getByTestId("sellers-list-count")).toHaveTextContent("2 sellers")
    expect(screen.getByTestId("seller-row-s1-usage")).toHaveTextContent("3 items · Ann, 555-0100")
    expect(screen.getByTestId("seller-row-s2-usage")).toHaveTextContent("No items yet")
    expect(screen.getByTestId("seller-row-s1-website")).toHaveTextContent("hw.example")
    expect(screen.getByTestId("seller-row-s1-website")).toHaveAttribute(
      "href",
      "https://hw.example/shop"
    )
    expect(screen.getByText("Cash only")).toBeVisible()
  })

  it("links each seller to the items bought from it, inactive ones included", async () => {
    seed()
    renderPage()
    expect(await screen.findByTestId("seller-row-s1-link")).toHaveAttribute(
      "href",
      `/g/${SLUG}/commodities?seller=s1&inactive=1`
    )
  })

  it("filters by name, ignoring case and extra spaces", async () => {
    seed()
    renderPage(`/g/${SLUG}/sellers?q=HARDWARE%20%20store`)
    expect(await screen.findByTestId("seller-row-s1")).toBeVisible()
    expect(screen.queryByTestId("seller-row-s2")).not.toBeInTheDocument()
    expect(screen.getByTestId("sellers-list-count")).toHaveTextContent("1 result")
  })

  it("explains an empty group", async () => {
    seed([])
    renderPage()
    expect(await screen.findByTestId("sellers-list-empty")).toHaveTextContent(/No sellers yet/)
  })

  it("names the item count when deleting a seller with items", async () => {
    const deleted: string[] = []
    seed()
    server.use(...sellerHandlers.remove(SLUG, "s1", deleted))
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("seller-row-s1-delete"))
    const dialog = await screen.findByTestId("confirm-dialog")
    expect(dialog).toHaveTextContent(
      "3 items were bought from Hardware Store. The items stay and no longer name a seller."
    )
    await user.click(within(dialog).getByTestId("confirm-accept"))
    await waitFor(() => expect(deleted).toEqual(["s1"]))
  })

  it("deletes nothing when the confirmation is canceled", async () => {
    const deleted: string[] = []
    seed()
    server.use(...sellerHandlers.remove(SLUG, "s2", deleted))
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("seller-row-s2-delete"))
    const dialog = await screen.findByTestId("confirm-dialog")
    expect(dialog).toHaveTextContent("No item names Corner Market.")
    await user.click(within(dialog).getByTestId("confirm-cancel"))
    await waitFor(() => expect(screen.queryByTestId("confirm-dialog")).not.toBeInTheDocument())
    expect(deleted).toEqual([])
  })

  it("creates a seller from the dialog", async () => {
    const body = capture<{ data: { type: string; attributes: Record<string, string> } }>()
    seed()
    server.use(
      ...sellerHandlers.create(SLUG, { id: "s3", name: "Garden Center", item_count: 0 }, body)
    )
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("seller-row-s1")
    await user.click(screen.getByTestId("sellers-create-button"))
    const dialog = await screen.findByTestId("seller-form-dialog")
    await user.type(within(dialog).getByTestId("seller-name-input"), "Garden Center")
    await user.type(within(dialog).getByTestId("seller-website-input"), "https://garden.example")
    await user.click(within(dialog).getByTestId("seller-form-submit"))
    await waitFor(() => expect(screen.queryByTestId("seller-form-dialog")).not.toBeInTheDocument())
    expect(body.value?.data).toEqual({
      type: "sellers",
      attributes: {
        name: "Garden Center",
        website: "https://garden.example",
        contact: "",
        note: "",
      },
    })
  })

  it("rejects a website that is not an http address before sending", async () => {
    seed()
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("seller-row-s1")
    await user.click(screen.getByTestId("sellers-create-button"))
    const dialog = await screen.findByTestId("seller-form-dialog")
    await user.type(within(dialog).getByTestId("seller-name-input"), "Shop")
    await user.type(within(dialog).getByTestId("seller-website-input"), "ftp://shop.example")
    await user.click(within(dialog).getByTestId("seller-form-submit"))
    expect(await within(dialog).findByTestId("seller-website-error")).toHaveTextContent(
      /http:\/\/ or https:\/\//
    )
  })

  it("shows a taken name on the name field", async () => {
    seed()
    server.use(...sellerHandlers.nameTaken(SLUG))
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("seller-row-s1")
    await user.click(screen.getByTestId("sellers-create-button"))
    const dialog = await screen.findByTestId("seller-form-dialog")
    await user.type(within(dialog).getByTestId("seller-name-input"), "hardware store")
    await user.click(within(dialog).getByTestId("seller-form-submit"))
    expect(await within(dialog).findByTestId("seller-name-error")).toHaveTextContent(
      "Another seller of this group has this name."
    )
    expect(screen.getByTestId("seller-form-dialog")).toBeVisible()
  })

  it("edits a seller with its values filled in", async () => {
    const body = capture<{ data: { id: string; attributes: Record<string, string> } }>()
    seed()
    server.use(...sellerHandlers.update(SLUG, { ...hardware, note: "Cards too" }, body))
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("seller-row-s1-edit"))
    const dialog = await screen.findByTestId("seller-form-dialog")
    expect(within(dialog).getByTestId("seller-name-input")).toHaveValue("Hardware Store")
    const note = within(dialog).getByTestId("seller-note-input")
    await user.clear(note)
    await user.type(note, "Cards too")
    await user.click(within(dialog).getByTestId("seller-form-submit"))
    await waitFor(() => expect(body.value?.data.id).toBe("s1"))
    expect(body.value?.data.attributes.note).toBe("Cards too")
  })

  it("has no axe violations", async () => {
    seed()
    const { container } = renderPage()
    await screen.findByTestId("seller-row-s1")
    expect(await axe(container)).toHaveNoViolations()
  })
})

describe("<SellersListPage /> as a viewer (#2886)", () => {
  // The server takes seller writes from users, admins and owners only.
  it("offers no create, edit or delete", async () => {
    server.use(...memberHandlers.asRole("g1", "viewer"))
    seed()
    renderPage()
    expect(await screen.findByTestId("seller-row-s1")).toBeVisible()
    expect(screen.queryByTestId("sellers-create-button")).not.toBeInTheDocument()
    expect(screen.queryByTestId("seller-row-s1-edit")).not.toBeInTheDocument()
    expect(screen.queryByTestId("seller-row-s1-delete")).not.toBeInTheDocument()
  })
})
