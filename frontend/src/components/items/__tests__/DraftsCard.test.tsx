import { beforeEach, describe, expect, it } from "vitest"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { delay, http, HttpResponse } from "msw"

import { DraftsCard } from "@/components/items/DraftsCard"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { apiUrl, authHandlers, draftHandlers, groupHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const groups: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

function members(role: "viewer" | "user" | "admin" | "owner") {
  return http.get(apiUrl("/groups/g1/members"), () =>
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

function Probe() {
  const loc = useLocation()
  return <div data-testid="loc">{`${loc.pathname}${loc.search}`}</div>
}

function renderCard() {
  return renderWithProviders({
    initialPath: `/g/${SLUG}/commodities`,
    routes: (
      <>
        <Route
          path="/g/:groupSlug/commodities"
          element={
            <AuthProvider>
              <ConfirmProvider>
                <GroupProvider>
                  <DraftsCard />
                  <div data-testid="list-page" />
                </GroupProvider>
              </ConfirmProvider>
            </AuthProvider>
          }
        />
        <Route path="/g/:groupSlug/commodities/new" element={<Probe />} />
      </>
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  setCurrentGroupSlug(SLUG)
})

describe("<DraftsCard />", () => {
  it("renders nothing while the group has no drafts", async () => {
    const fake = draftHandlers.backend(SLUG)
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("user"),
      ...fake.handlers
    )
    renderCard()
    await screen.findByTestId("list-page")
    // Give the list request time to land before asserting absence.
    await waitFor(() => expect(screen.queryByTestId("drafts-card")).not.toBeInTheDocument())
  })

  it("lists drafts and continues one in the item form", async () => {
    const fake = draftHandlers.backend(SLUG, [
      { id: "d1", values: { name: "Cordless drill" } },
      { id: "d2", values: {} },
    ])
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("user"),
      ...fake.handlers
    )
    const user = userEvent.setup()
    renderCard()
    const card = await screen.findByTestId("drafts-card")
    expect(within(card).getByTestId("drafts-count")).toHaveTextContent("2")
    expect(within(card).getByText("Cordless drill")).toBeInTheDocument()
    expect(within(card).getByText("Untitled draft")).toBeInTheDocument()
    await user.click(within(card).getByTestId("draft-resume-d1"))
    expect(await screen.findByTestId("loc")).toHaveTextContent(
      `/g/${SLUG}/commodities/new?draft=d1`
    )
  })

  it("discards a draft after confirmation", async () => {
    const fake = draftHandlers.backend(SLUG, [{ id: "d1", values: { name: "Lamp" } }])
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("user"),
      ...fake.handlers
    )
    const user = userEvent.setup()
    renderCard()
    await user.click(await screen.findByTestId("draft-discard-d1"))
    const dialog = await screen.findByTestId("confirm-dialog")
    expect(dialog).toHaveTextContent(/Trash/)
    expect(within(dialog).getByTestId("confirm-accept")).toHaveTextContent("Discard")
    await user.click(within(dialog).getByTestId("confirm-accept"))
    await waitFor(() => expect(fake.drafts).toHaveLength(0))
    await waitFor(() => expect(screen.queryByTestId("drafts-card")).not.toBeInTheDocument())
  })

  it("offers a viewer no actions", async () => {
    const fake = draftHandlers.backend(SLUG, [{ id: "d1", values: { name: "Lamp" } }])
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("viewer"),
      ...fake.handlers
    )
    renderCard()
    const row = await screen.findByTestId("draft-row-d1")
    await waitFor(() =>
      expect(within(row).queryByTestId("draft-resume-d1")).not.toBeInTheDocument()
    )
    expect(within(row).queryByTestId("draft-discard-d1")).not.toBeInTheDocument()
  })

  it("offers no actions while the member's role is loading", async () => {
    const fake = draftHandlers.backend(SLUG, [{ id: "d1", values: { name: "Lamp" } }])
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      http.get(apiUrl("/groups/g1/members"), async () => {
        await delay("infinite")
        return HttpResponse.json({ data: [] })
      }),
      ...fake.handlers
    )
    renderCard()
    const row = await screen.findByTestId("draft-row-d1")
    expect(within(row).queryByTestId("draft-resume-d1")).not.toBeInTheDocument()
    expect(within(row).queryByTestId("draft-discard-d1")).not.toBeInTheDocument()
  })

  it.each(["admin", "owner"] as const)("offers an %s the actions", async (role) => {
    const fake = draftHandlers.backend(SLUG, [{ id: "d1", values: { name: "Lamp" } }])
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members(role),
      ...fake.handlers
    )
    renderCard()
    expect(await screen.findByTestId("draft-resume-d1")).toBeInTheDocument()
    expect(screen.getByTestId("draft-discard-d1")).toBeInTheDocument()
  })
})
