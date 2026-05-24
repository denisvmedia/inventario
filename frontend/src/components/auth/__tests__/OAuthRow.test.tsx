import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { OAuthRow } from "@/components/auth/OAuthRow"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { clearAuth } from "@/lib/auth-storage"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

// Capture the original window.location at module load so afterEach can
// restore it after tests that override the descriptor with a fake assign
// spy. Without this restore, a redirect-spy test leaks its mocked
// location into subsequent tests / files and breaks anything that reads
// window.location.origin etc.
const originalLocation = window.location

function renderRow(initial = "/login") {
  return renderWithProviders({
    initialPath: initial,
    routes: <Route path="/login" element={<OAuthRow />} />,
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

afterEach(() => {
  Object.defineProperty(window, "location", {
    configurable: true,
    writable: true,
    value: originalLocation,
  })
})

describe("<OAuthRow />", () => {
  it("renders nothing when the BE returns an empty providers list", async () => {
    server.use(msw.get(api("/auth/oauth/providers"), () => HttpResponse.json({ providers: [] })))
    renderRow()
    // The query resolves in a microtask; waitFor catches the transition
    // from the no-data → empty-data render so we don't assert on the
    // pre-fetch "still loading, still hidden" frame.
    await waitFor(() => {
      expect(screen.queryByTestId("oauth-row")).not.toBeInTheDocument()
    })
  })

  it("renders one button per enabled provider", async () => {
    server.use(
      msw.get(api("/auth/oauth/providers"), () =>
        HttpResponse.json({
          providers: [
            { name: "google", display_name: "Google" },
            { name: "github", display_name: "GitHub" },
          ],
        })
      )
    )
    renderRow()
    expect(await screen.findByTestId("oauth-row")).toBeInTheDocument()
    expect(screen.getByTestId("oauth-google-button")).toBeInTheDocument()
    expect(screen.getByTestId("oauth-github-button")).toBeInTheDocument()
  })

  it("redirects the browser to the BE start endpoint with the sanitised ?redirect param on click", async () => {
    server.use(
      msw.get(api("/auth/oauth/providers"), () =>
        HttpResponse.json({ providers: [{ name: "google", display_name: "Google" }] })
      )
    )
    const assignSpy = vi.fn()
    // jsdom's `window.location.assign` is a no-op that prints a noisy
    // navigation warning; replacing the descriptor with a spy lets us
    // assert the URL the row would send the browser to without jsdom
    // attempting an actual navigation.
    Object.defineProperty(window, "location", {
      writable: true,
      value: { ...window.location, assign: assignSpy },
    })

    renderRow("/login?redirect=/g/household")
    const btn = await screen.findByTestId("oauth-google-button")
    await userEvent.setup().click(btn)
    expect(assignSpy).toHaveBeenCalledWith(
      "/api/v1/auth/oauth/google/start?redirect=%2Fg%2Fhousehold"
    )
  })

  it("falls back to / when the ?redirect param looks like an open-redirect attempt", async () => {
    server.use(
      msw.get(api("/auth/oauth/providers"), () =>
        HttpResponse.json({ providers: [{ name: "github", display_name: "GitHub" }] })
      )
    )
    const assignSpy = vi.fn()
    Object.defineProperty(window, "location", {
      writable: true,
      value: { ...window.location, assign: assignSpy },
    })
    renderRow("/login?redirect=https://evil.example/")
    const btn = await screen.findByTestId("oauth-github-button")
    await userEvent.setup().click(btn)
    // sanitizeRedirectPath collapses `https://…` to `/`.
    expect(assignSpy).toHaveBeenCalledWith("/api/v1/auth/oauth/github/start?redirect=%2F")
  })
})
