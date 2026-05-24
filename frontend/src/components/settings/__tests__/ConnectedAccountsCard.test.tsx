import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { ConnectedAccountsCard } from "@/components/settings/ConnectedAccountsCard"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

// Capture the original window.location at module load so afterEach can
// restore it after tests that override the descriptor with a fake assign
// spy. Without this restore, a redirect-spy test leaks its mocked
// location into subsequent tests / files and breaks anything that reads
// window.location.origin etc.
const originalLocation = window.location

function renderCard() {
  return renderWithProviders({
    initialPath: "/settings",
    routes: (
      <Route
        path="/settings"
        element={
          <ConfirmProvider>
            <ConnectedAccountsCard />
          </ConfirmProvider>
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
})

afterEach(() => {
  Object.defineProperty(window, "location", {
    configurable: true,
    writable: true,
    value: originalLocation,
  })
})

describe("<ConnectedAccountsCard />", () => {
  it("hides itself when no providers are configured", async () => {
    server.use(
      msw.get(api("/auth/oauth/providers"), () => HttpResponse.json({ providers: [] })),
      msw.get(api("/auth/oauth/identities"), () => HttpResponse.json({ identities: [] }))
    )
    renderCard()
    await waitFor(() => {
      expect(screen.queryByTestId("connected-accounts-card")).not.toBeInTheDocument()
    })
  })

  it("renders one row per provider with the linked-state controls", async () => {
    server.use(
      msw.get(api("/auth/oauth/providers"), () =>
        HttpResponse.json({
          providers: [
            { name: "google", display_name: "Google" },
            { name: "github", display_name: "GitHub" },
          ],
        })
      ),
      msw.get(api("/auth/oauth/identities"), () =>
        HttpResponse.json({
          identities: [
            { provider: "google", email: "denis@example.com", linked_at: "2026-04-01T00:00:00Z" },
          ],
        })
      )
    )
    renderCard()
    expect(await screen.findByTestId("connected-accounts-card")).toBeInTheDocument()
    // Google row is the linked variant — shows the Unlink button.
    const linkedRow = screen.getByTestId("connected-account-row-google")
    expect(linkedRow.dataset.linked).toBe("true")
    expect(screen.getByTestId("connected-account-unlink-google")).toBeInTheDocument()
    expect(screen.getByTestId("connected-account-meta-google")).toHaveTextContent(/denis@example/)
    // GitHub row is the unlinked variant — shows the Link button.
    const unlinkedRow = screen.getByTestId("connected-account-row-github")
    expect(unlinkedRow.dataset.linked).toBe("false")
    expect(screen.getByTestId("connected-account-link-github")).toBeInTheDocument()
  })

  it("calls DELETE /auth/oauth/{provider} after the unlink confirmation", async () => {
    let deleteCalls = 0
    server.use(
      msw.get(api("/auth/oauth/providers"), () =>
        HttpResponse.json({ providers: [{ name: "google", display_name: "Google" }] })
      ),
      msw.get(api("/auth/oauth/identities"), () =>
        HttpResponse.json({
          identities: [
            { provider: "google", email: "denis@example.com", linked_at: "2026-04-01T00:00:00Z" },
          ],
        })
      ),
      msw.delete(api("/auth/oauth/google"), () => {
        deleteCalls++
        return new HttpResponse(null, { status: 204 })
      })
    )
    renderCard()
    const user = userEvent.setup()
    await user.click(await screen.findByTestId("connected-account-unlink-google"))
    // ConfirmProvider mounts a real dialog; click its confirm button.
    const confirmButton = await screen.findByRole("button", { name: /^Unlink$/ })
    await user.click(confirmButton)
    await waitFor(() => expect(deleteCalls).toBe(1))
  })

  it("redirects to /api/v1/auth/oauth/{provider}/link/start when linking", async () => {
    server.use(
      msw.get(api("/auth/oauth/providers"), () =>
        HttpResponse.json({ providers: [{ name: "github", display_name: "GitHub" }] })
      ),
      msw.get(api("/auth/oauth/identities"), () => HttpResponse.json({ identities: [] }))
    )
    const assignSpy = vi.fn()
    Object.defineProperty(window, "location", {
      writable: true,
      value: { ...window.location, assign: assignSpy },
    })
    renderCard()
    const linkButton = await screen.findByTestId("connected-account-link-github")
    await userEvent.setup().click(linkButton)
    expect(assignSpy).toHaveBeenCalledWith(
      "/api/v1/auth/oauth/github/link/start?redirect=%2Fsettings"
    )
  })
})
