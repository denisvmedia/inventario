import type { ReactNode } from "react"
import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"

import { NeedsAttentionCard } from "@/components/dashboard/NeedsAttentionCard"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { apiUrl, authHandlers, groupHandlers, inboxHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

const SLUG = "household"

function renderAt(element: ReactNode) {
  return renderWithProviders({
    initialPath: `/g/${SLUG}`,
    routes: (
      <Route
        path="/g/:groupSlug"
        element={
          <AuthProvider>
            <GroupProvider>
              {element}
              <div data-testid="rendered" />
            </GroupProvider>
          </AuthProvider>
        }
      />
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

function seed(entries: inboxHandlers.FakeInboxEntry[]) {
  const fake = inboxHandlers.backend(SLUG, entries)
  server.use(...authHandlers.signedIn(), ...groupHandlers.list(), ...fake.handlers)
  return fake
}

describe("<NeedsAttentionCard />", () => {
  it("states the inbox count and what it holds, and links the inbox", async () => {
    seed([
      inboxHandlers.draft("d1", "Drill"),
      inboxHandlers.draft("d2", "Lamp"),
      inboxHandlers.itemDraft("c1", "Bike"),
    ])
    renderAt(<NeedsAttentionCard />)
    const card = await screen.findByTestId("dashboard-needs-attention")
    expect(card).toHaveAttribute("href", `/g/${SLUG}/inbox`)
    expect(card).toHaveTextContent("Needs attention: 3")
    expect(card).toHaveTextContent("2 drafts to finish · 1 item to review")
  })

  it("says when the count could not be loaded, and retries", async () => {
    let fail = true
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(),
      http.get(apiUrl(`/g/${SLUG}/inbox/count`), () =>
        fail
          ? HttpResponse.json({ errors: [{ status: "500" }] }, { status: 500 })
          : HttpResponse.json({ data: { total: 1, reasons: { draft: 1 } } })
      )
    )
    const user = userEvent.setup()
    renderAt(<NeedsAttentionCard />)
    const error = await screen.findByTestId("dashboard-needs-attention-error", undefined, {
      timeout: 10000,
    })
    expect(error).toHaveTextContent("The inbox count could not be loaded.")
    expect(within(error).getByRole("link", { name: "Open inbox" })).toHaveAttribute(
      "href",
      `/g/${SLUG}/inbox`
    )
    fail = false
    await user.click(screen.getByTestId("dashboard-needs-attention-retry"))
    expect(await screen.findByTestId("dashboard-needs-attention")).toHaveTextContent(
      "Needs attention: 1"
    )
  })

  it("renders nothing while the inbox is empty", async () => {
    const fake = seed([])
    renderAt(<NeedsAttentionCard />)
    await waitFor(() => expect(fake.countCalls).toBe(1))
    await screen.findByTestId("rendered")
    expect(screen.queryByTestId("dashboard-needs-attention")).not.toBeInTheDocument()
  })
})
