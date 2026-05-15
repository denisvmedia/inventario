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

function renderSettings(initialPath: string = "/settings") {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath,
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

// withGroupHandlers extends baseHandlers with a current group + a GET
// /g/{slug}/settings stub. The Notifications + new Appearance rows are
// driven by useUserSettings(), which is disabled until the user has
// at least one group, so tests that exercise those flows must opt in.
// The /groups payload follows the JSON:API envelope shape the
// GroupContext loader expects — a flat array won't be parsed and the
// group context stays null (settings query disabled).
function withGroupHandlers(settingsBody: Record<string, unknown> = {}) {
  const slug = "household"
  return [
    msw.get(api("/auth/me"), () =>
      HttpResponse.json({
        id: "u1",
        email: "alex@example.com",
        name: "Alex",
        created_at: "2024-01-15T00:00:00Z",
      })
    ),
    msw.get(api("/groups"), () =>
      HttpResponse.json({
        data: [
          {
            id: "g1",
            type: "groups",
            attributes: { id: "g1", slug, name: "Household" },
          },
        ],
      })
    ),
    msw.get(api(`/g/${slug}/settings`), () => HttpResponse.json(settingsBody)),
  ]
}

describe("<SettingsPage />", () => {
  it("renders the section nav with all 5 entries", async () => {
    server.use(...baseHandlers)
    renderSettings()
    expect(await screen.findByTestId("settings-nav-account")).toBeInTheDocument()
    expect(screen.getByTestId("settings-nav-appearance")).toBeInTheDocument()
    expect(screen.getByTestId("settings-nav-notifications")).toBeInTheDocument()
    expect(screen.getByTestId("settings-nav-privacy")).toBeInTheDocument()
    expect(screen.getByTestId("settings-nav-help")).toBeInTheDocument()
    // Storage usage and Export data moved to /groups/:id/settings → no
    // "data" nav entry on user Preferences anymore.
    expect(screen.queryByTestId("settings-nav-data")).not.toBeInTheDocument()
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

  it("privacy section: MFA row (#1645) + sessions/history links (#1644) + connected-accounts stub", async () => {
    server.use(
      ...baseHandlers,
      // #1644: the privacy section pre-fetches the sessions list to drive
      // the active-sessions row badge. The handler is wired here so the
      // useSessionsList query resolves; an empty array hides the badge.
      msw.get(api("/users/me/sessions"), () => HttpResponse.json({ sessions: [] })),
      // #1645: MFA row reads /auth/mfa/status — default to "none" so
      // the badge resolves to Inactive without exercising the dialog.
      msw.get(api("/auth/mfa/status"), () =>
        HttpResponse.json({ state: "none", backup_codes_remaining: 0 })
      )
    )
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-privacy"))
    // MFA row replaces the twoFactor stub from #1644 — assert the
    // Inactive badge resolves from /auth/mfa/status.
    expect(await screen.findByTestId("privacy-mfa-row")).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByTestId("privacy-mfa-row").getAttribute("data-mfa-state")).toBe("inactive")
    )
    const sessionsRow = await screen.findByTestId("privacy-row-activeSessions")
    expect(sessionsRow).toHaveAttribute("href", "/profile/sessions")
    const historyRow = screen.getByTestId("privacy-row-loginHistory")
    expect(historyRow).toHaveAttribute("href", "/profile/login-history")
    // Connected accounts stays a ComingSoonBanner per #1644 acceptance
    // (split into its own follow-up #1395).
    expect(screen.getByTestId("coming-soon-banner-connectedAccounts")).toBeInTheDocument()
  })

  it("account danger zone's delete button opens a confirm dialog explaining unavailability", async () => {
    server.use(...baseHandlers)
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-account"))
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

  it("appearance section adds Default view + preferred currency rows", async () => {
    server.use(...baseHandlers)
    renderSettings()
    expect(await screen.findByTestId("section-appearance")).toBeInTheDocument()
    expect(screen.getByTestId("default-view-select")).toBeInTheDocument()
    expect(screen.getByTestId("preferred-currency-row")).toBeInTheDocument()
  })

  it("notifications section renders all six toggle rows with mock-spec subgroup chrome", async () => {
    server.use(...withGroupHandlers({}))
    const user = userEvent.setup()
    renderSettings("/settings?g=household")
    await user.click(await screen.findByTestId("settings-nav-notifications"))
    expect(screen.getByTestId("notification-row-warranty-expiry")).toBeInTheDocument()
    expect(screen.getByTestId("notification-row-maintenance-reminder")).toBeInTheDocument()
    expect(screen.getByTestId("notification-row-weekly-digest")).toBeInTheDocument()
    expect(screen.getByTestId("notification-row-price-drop")).toBeInTheDocument()
    expect(screen.getByTestId("notification-row-channel-email")).toBeInTheDocument()
    expect(screen.getByTestId("notification-row-channel-push")).toBeInTheDocument()
    // Stub banners must NOT linger after the section has been wired.
    expect(
      screen.queryByTestId("coming-soon-banner-notificationPreferences")
    ).not.toBeInTheDocument()
    expect(screen.queryByTestId("coming-soon-banner-maintenanceReminders")).not.toBeInTheDocument()
  })

  it("toggling a notification row fires PATCH /settings/{field}", async () => {
    let lastPatch: { field: string; body: unknown } | null = null
    server.use(
      ...withGroupHandlers({}),
      msw.patch(api("/g/household/settings/:field"), async ({ params, request }) => {
        const body = (await request.json()) as unknown
        lastPatch = { field: String(params.field), body }
        return HttpResponse.json({ notificationsWarrantyExpiry: body })
      })
    )
    const user = userEvent.setup()
    renderSettings("/settings?g=household")
    // Wait for page chrome to mount before clicking the nav. Under
    // load (full suite) the click can otherwise outrun the initial
    // render and the section never swaps.
    await screen.findByTestId("settings-page")
    await user.click(await screen.findByTestId("settings-nav-notifications"))
    // Default for warranty_expiry is `true`; flipping it sends `false`.
    // The Switch is disabled while the GET /settings response is in
    // flight (the section avoids flicker between optimistic and
    // server-confirmed state) — wait until the row is interactive
    // before clicking, otherwise the test races the autosave.
    const row = await screen.findByTestId("notification-row-warranty-expiry", undefined, {
      timeout: 4000,
    })
    await waitFor(() => {
      const t = row.querySelector("button[role='switch']") as HTMLButtonElement | null
      expect(t).not.toBeNull()
      expect(t?.hasAttribute("disabled")).toBe(false)
    })
    const toggle = row.querySelector("button[role='switch']") as HTMLButtonElement
    await user.click(toggle)
    await waitFor(() => {
      expect(lastPatch).not.toBeNull()
      expect(lastPatch?.field).toBe("notifications.warranty_expiry")
      expect(lastPatch?.body).toBe(false)
    })
  })

  it("help section adds a Contact support row and a version badge on What's new", async () => {
    server.use(...baseHandlers)
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-help"))
    expect(screen.getByTestId("help-row-contactSupport")).toBeInTheDocument()
    expect(screen.getByTestId("help-row-whatsNew-badge")).toBeInTheDocument()
  })
})
