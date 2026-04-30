import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { SettingsPage } from "@/pages/SettingsPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ThemeProvider } from "@/components/theme-provider"
import { DensityProvider } from "@/hooks/useDensity"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname} />
}

function renderSettings() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: "/settings",
    routes: (
      <>
        <Route
          path="/settings"
          element={
            <ThemeProvider defaultTheme="system" storageKey="theme-test-1414">
              <DensityProvider defaultDensity="comfortable" storageKey="density-test-1414">
                <AuthProvider>
                  <GroupProvider>
                    <ConfirmProvider>
                      <SettingsPage />
                    </ConfirmProvider>
                  </GroupProvider>
                </AuthProvider>
              </DensityProvider>
            </ThemeProvider>
          }
        />
        <Route path="*" element={<LocationProbe />} />
      </>
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  // Wipe persistence so the test starts from defaults each time.
  localStorage.removeItem("theme-test-1414")
  localStorage.removeItem("density-test-1414")
})

const baseHandlers = [
  msw.get(api("/auth/me"), () =>
    HttpResponse.json({
      id: "u1",
      email: "alex@example.com",
      name: "Alex",
      created_at: "2024-01-15T00:00:00Z",
    })
  ),
  msw.get(api("/groups"), () => HttpResponse.json({ data: [] })),
]

describe("<SettingsPage />", () => {
  it("renders the section nav with all 6 entries", async () => {
    server.use(...baseHandlers)
    renderSettings()
    expect(await screen.findByTestId("settings-nav-account")).toBeInTheDocument()
    expect(screen.getByTestId("settings-nav-appearance")).toBeInTheDocument()
    expect(screen.getByTestId("settings-nav-notifications")).toBeInTheDocument()
    expect(screen.getByTestId("settings-nav-privacy")).toBeInTheDocument()
    expect(screen.getByTestId("settings-nav-data")).toBeInTheDocument()
    expect(screen.getByTestId("settings-nav-help")).toBeInTheDocument()
  })

  it("appearance section is the default and shows theme/density/locale controls", async () => {
    server.use(...baseHandlers)
    renderSettings()
    expect(await screen.findByTestId("section-appearance")).toBeInTheDocument()
    expect(screen.getByTestId("theme-system")).toBeInTheDocument()
    expect(screen.getByTestId("theme-light")).toBeInTheDocument()
    expect(screen.getByTestId("theme-dark")).toBeInTheDocument()
    expect(screen.getByTestId("density-select")).toBeInTheDocument()
    expect(screen.getByTestId("locale-select")).toBeInTheDocument()
  })

  it("clicking a theme card persists the choice to localStorage", async () => {
    server.use(...baseHandlers)
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("theme-dark"))
    await waitFor(() => expect(localStorage.getItem("theme-test-1414")).toBe("dark"))
  })

  it("changing density persists via the provider", async () => {
    server.use(...baseHandlers)
    const user = userEvent.setup()
    renderSettings()
    const select = await screen.findByTestId("density-select")
    await user.selectOptions(select, "compact")
    await waitFor(() => expect(localStorage.getItem("density-test-1414")).toBe("compact"))
  })

  it("privacy section renders four ComingSoonBanner stubs", async () => {
    server.use(...baseHandlers)
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-privacy"))
    expect(screen.getByTestId("coming-soon-banner-twoFactor")).toBeInTheDocument()
    expect(screen.getByTestId("coming-soon-banner-activeSessions")).toBeInTheDocument()
    expect(screen.getByTestId("coming-soon-banner-loginHistory")).toBeInTheDocument()
    expect(screen.getByTestId("coming-soon-banner-connectedAccounts")).toBeInTheDocument()
  })

  it("data section's delete button opens a confirm dialog explaining unavailability", async () => {
    server.use(...baseHandlers)
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-data"))
    await user.click(screen.getByTestId("delete-account-button"))
    expect(await screen.findByText(/account deletion is not yet available/i)).toBeInTheDocument()
  })

  it("sign out POSTs /auth/logout and navigates to /login", async () => {
    let logoutCalls = 0
    server.use(
      ...baseHandlers,
      msw.post(api("/auth/logout"), () => {
        logoutCalls++
        return new HttpResponse(null, { status: 204 })
      })
    )
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-sign-out"))
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/login")
    )
    expect(logoutCalls).toBe(1)
  })

  it("has no axe violations on the appearance section", async () => {
    server.use(...baseHandlers)
    const { container } = renderSettings()
    await waitFor(() => expect(screen.getByTestId("section-appearance")).toBeInTheDocument())
    expect(await axe(container)).toHaveNoViolations()
  })
})
