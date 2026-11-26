import { beforeEach, describe, expect, it } from "vitest"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"

import { IntakeCard } from "@/components/items/IntakeCard"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { apiUrl, authHandlers, groupHandlers, intakeHandlers } from "@/test/handlers"
import { receiptSession } from "@/test/handlers/intake"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const groups: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

function members(role: "viewer" | "user") {
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
  return <div data-testid="loc">{loc.pathname}</div>
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
                  <IntakeCard />
                  <div data-testid="list-page" />
                </GroupProvider>
              </ConfirmProvider>
            </AuthProvider>
          }
        />
        <Route path="/g/:groupSlug/intake/:id" element={<Probe />} />
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

describe("<IntakeCard />", () => {
  it("renders nothing while the group has no batches", async () => {
    const fake = intakeHandlers.backend(SLUG)
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("user"),
      ...fake.handlers
    )
    renderCard()
    await screen.findByTestId("list-page")
    await waitFor(() => expect(screen.queryByTestId("intake-card")).not.toBeInTheDocument())
  })

  it("summarizes a batch and opens its review", async () => {
    const session = receiptSession()
    const fake = intakeHandlers.backend(SLUG, [session])
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("user"),
      ...fake.handlers
    )
    const user = userEvent.setup()
    renderCard()
    const row = await screen.findByTestId(`intake-row-${session.id}`)
    expect(within(row).getByText("2 files")).toBeInTheDocument()
    expect(within(row).getByTestId(`intake-summary-${session.id}`)).toHaveTextContent(
      "3 products to review, 1 file unreadable"
    )
    await user.click(await within(row).findByText("Review"))
    expect(await screen.findByTestId("loc")).toHaveTextContent(`/g/${SLUG}/intake/${session.id}`)
  })

  it("removes a finished batch after confirmation", async () => {
    const session = receiptSession()
    for (const c of session.candidates) c.outcome = "created"
    const fake = intakeHandlers.backend(SLUG, [session])
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("user"),
      ...fake.handlers
    )
    const user = userEvent.setup()
    renderCard()
    await user.click(await screen.findByTestId(`intake-remove-${session.id}`))
    const dialog = await screen.findByTestId("confirm-dialog")
    expect(dialog).toHaveTextContent(/Drafts made from it stay/)
    await user.click(within(dialog).getByTestId("confirm-accept"))
    await waitFor(() => expect(fake.sessions).toHaveLength(0))
    await waitFor(() => expect(screen.queryByTestId("intake-card")).not.toBeInTheDocument())
  })
})
