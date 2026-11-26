import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { fireEvent, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { CommoditiesListPage } from "@/pages/commodities/CommoditiesListPage"
import {
  apiUrl,
  areaHandlers,
  authHandlers,
  commodityHandlers,
  draftHandlers,
  groupHandlers,
  intakeHandlers,
  savedViewHandlers,
  sellerHandlers,
} from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const groups: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

function members(role: "viewer" | "user") {
  return msw.get(apiUrl("/groups/g1/members"), () =>
    HttpResponse.json({
      data: [
        {
          id: "m1",
          type: "memberships",
          attributes: { group_id: "g1", member_user_id: "u1", role },
        },
      ],
    })
  )
}

function renderList() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: `/g/${SLUG}/commodities`,
    routes: (
      <Route
        path="/g/:groupSlug/commodities"
        element={
          <AuthProvider>
            <GroupProvider>
              <ConfirmProvider>
                <CommoditiesListPage />
              </ConfirmProvider>
            </GroupProvider>
          </AuthProvider>
        }
      />
    ),
  })
}

// files builds the dataTransfer of a drag of files from the desktop.
function files(list: File[]) {
  return { dataTransfer: { types: ["Files"], files: list, dropEffect: "none" } }
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  server.use(
    msw.get(apiUrl(`/g/${SLUG}/settings`), () => HttpResponse.json({})),
    ...authHandlers.signedIn(),
    ...groupHandlers.list(groups),
    ...areaHandlers.list(SLUG, []),
    ...commodityHandlers.list(SLUG, []),
    ...sellerHandlers.list(SLUG, []),
    ...savedViewHandlers.none(SLUG),
    ...draftHandlers.backend(SLUG).handlers
  )
})

describe("<CommoditiesListPage /> batch intake", () => {
  it("opens the intake dialog with the files dropped on the page", async () => {
    const fake = intakeHandlers.backend(SLUG)
    server.use(members("user"), ...fake.handlers)
    renderList()
    const page = await screen.findByTestId("page-commodities")
    await screen.findByTestId("commodities-add-from-files")
    const dropped = [
      new File(["a"], "receipt.jpg", { type: "image/jpeg" }),
      new File(["b"], "invoice.pdf", { type: "application/pdf" }),
    ]
    fireEvent.dragEnter(page, files(dropped))
    expect(screen.getByTestId("intake-drop-overlay")).toBeInTheDocument()
    fireEvent.drop(page, files(dropped))
    expect(screen.queryByTestId("intake-drop-overlay")).not.toBeInTheDocument()
    const dialog = await screen.findByTestId("intake-upload-dialog")
    expect(within(dialog).getAllByTestId("intake-upload-file")).toHaveLength(2)
    expect(within(dialog).getByTestId("intake-upload-submit")).toHaveTextContent("Scan 2 files")
  })

  it("opens the review once the batch is queued", async () => {
    const fake = intakeHandlers.backend(SLUG)
    server.use(members("user"), ...fake.handlers)
    const user = userEvent.setup()
    renderList()
    const page = await screen.findByTestId("page-commodities")
    await screen.findByTestId("commodities-add-from-files")
    const dropped = [new File(["a"], "receipt.jpg", { type: "image/jpeg" })]
    fireEvent.drop(page, files(dropped))
    await user.click(await screen.findByTestId("intake-upload-submit"))
    await waitFor(() => expect(fake.sessions).toHaveLength(1))
    await waitFor(() =>
      expect(screen.queryByTestId("intake-upload-dialog")).not.toBeInTheDocument()
    )
  })

  it("offers no intake without an AI provider", async () => {
    const fake = intakeHandlers.backend(SLUG, [], { enabled: false })
    server.use(members("user"), ...fake.handlers)
    renderList()
    const page = await screen.findByTestId("page-commodities")
    await screen.findByTestId("commodities-add-button")
    await waitFor(() =>
      expect(screen.queryByTestId("commodities-add-from-files")).not.toBeInTheDocument()
    )
    fireEvent.dragEnter(page, files([new File(["a"], "a.jpg", { type: "image/jpeg" })]))
    expect(screen.queryByTestId("intake-drop-overlay")).not.toBeInTheDocument()
  })
})
