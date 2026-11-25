import type { ReactNode } from "react"
import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"

import { NeedsAttentionCard } from "@/components/dashboard/NeedsAttentionCard"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { authHandlers, groupHandlers, inboxHandlers } from "@/test/handlers"
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

  it("renders nothing while the inbox is empty", async () => {
    const fake = seed([])
    renderAt(<NeedsAttentionCard />)
    await waitFor(() => expect(fake.countCalls).toBe(1))
    await screen.findByTestId("rendered")
    expect(screen.queryByTestId("dashboard-needs-attention")).not.toBeInTheDocument()
  })
})
