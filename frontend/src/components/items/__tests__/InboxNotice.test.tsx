import type { ReactNode } from "react"
import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"

import { InboxNotice } from "@/components/items/InboxNotice"
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

describe("<InboxNotice />", () => {
  it("links the drafts tab when only drafts are waiting", async () => {
    seed([inboxHandlers.draft("d1", "Drill")])
    renderAt(<InboxNotice />)
    const notice = await screen.findByTestId("items-inbox-notice")
    expect(notice).toHaveTextContent("1 draft to finish")
    expect(screen.getByTestId("items-inbox-notice-open")).toHaveAttribute(
      "href",
      `/g/${SLUG}/inbox?reason=draft`
    )
  })

  it("links the items tab when only items wait for review", async () => {
    seed([inboxHandlers.itemDraft("c1", "Bike"), inboxHandlers.itemDraft("c2", "Chair")])
    renderAt(<InboxNotice />)
    const notice = await screen.findByTestId("items-inbox-notice")
    expect(notice).toHaveTextContent("2 items to review")
    expect(screen.getByTestId("items-inbox-notice-open")).toHaveAttribute(
      "href",
      `/g/${SLUG}/inbox?reason=item_draft`
    )
  })

  it("links the whole inbox when both are waiting", async () => {
    seed([inboxHandlers.draft("d1", "Drill"), inboxHandlers.itemDraft("c1", "Bike")])
    renderAt(<InboxNotice />)
    expect(await screen.findByTestId("items-inbox-notice")).toHaveTextContent(
      "1 draft to finish · 1 item to review"
    )
    expect(screen.getByTestId("items-inbox-notice-open")).toHaveAttribute(
      "href",
      `/g/${SLUG}/inbox`
    )
  })

  it("renders nothing while nothing is waiting", async () => {
    const fake = seed([])
    renderAt(<InboxNotice />)
    await waitFor(() => expect(fake.countCalls).toBe(1))
    await screen.findByTestId("rendered")
    expect(screen.queryByTestId("items-inbox-notice")).not.toBeInTheDocument()
  })
})
