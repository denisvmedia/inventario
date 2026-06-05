import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { SettingsPage } from "@/pages/SettingsPage"
import { pickRadixSelect } from "@/test/radix"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { KeyboardShortcutsProvider } from "@/features/shortcuts"
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
                      <KeyboardShortcutsProvider>
                        <SettingsPage />
                      </KeyboardShortcutsProvider>
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
  // ConnectedAccountsCard (#1394) queries this on every settings render;
  // default to no providers so the card hides itself and the privacy
  // section stays compact unless a test explicitly overrides.
  msw.get(api("/auth/oauth/providers"), () => HttpResponse.json({ providers: [] })),
  msw.get(api("/auth/oauth/identities"), () => HttpResponse.json({ identities: [] })),
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

  // #1888 — Account is the landing tab (was Appearance). Most users open
  // Preferences for email/password/MFA/profile, not theme/density.
  it("account section is the default landing (#1888)", async () => {
    server.use(...baseHandlers)
    renderSettings()
    expect(await screen.findByTestId("section-account")).toBeInTheDocument()
    expect(screen.queryByTestId("section-appearance")).not.toBeInTheDocument()
  })

  it("appearance section shows theme/density/locale controls", async () => {
    server.use(...baseHandlers)
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-appearance"))
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
    await user.click(await screen.findByTestId("settings-nav-appearance"))
    await user.click(await screen.findByTestId("theme-dark"))
    await waitFor(() => expect(localStorage.getItem("theme-test-1414")).toBe("dark"))
  })

  it("changing density persists via the provider", async () => {
    server.use(...baseHandlers)
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-appearance"))
    await screen.findByTestId("density-select")
    // Density is now a shadcn/Radix Select (blessed form dropdown), so drive
    // it via the listbox rather than the native-select-only selectOptions.
    await pickRadixSelect(user, /^Density$/i, { optionLabel: /^Compact$/i })
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
    // Connected accounts (#1394) — when the BE reports no enabled
    // providers (the baseHandlers default in this test), the card hides
    // itself entirely so the section stays compact for deployments
    // without OAuth.
    expect(screen.queryByTestId("connected-accounts-card")).not.toBeInTheDocument()
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
    const user = userEvent.setup()
    const { container } = renderSettings()
    await user.click(await screen.findByTestId("settings-nav-appearance"))
    await waitFor(() => expect(screen.getByTestId("section-appearance")).toBeInTheDocument())
    expect(await axe(container)).toHaveNoViolations()
  })

  it("appearance section adds Default view + preferred currency rows", async () => {
    server.use(...baseHandlers)
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-appearance"))
    expect(await screen.findByTestId("section-appearance")).toBeInTheDocument()
    expect(screen.getByTestId("default-view-select")).toBeInTheDocument()
    expect(screen.getByTestId("preferred-currency-row")).toBeInTheDocument()
  })

  // #1683 — Region & formatting dropdown decouples Intl.* locale from
  // the UI translation language. Lives in the same Appearance section
  // and follows the same autosave path (PATCH /settings/{field}).
  it("appearance section adds a Region & formatting dropdown (#1683)", async () => {
    server.use(...withGroupHandlers({}))
    const user = userEvent.setup()
    renderSettings("/settings?g=household")
    await user.click(await screen.findByTestId("settings-nav-appearance"))
    expect(await screen.findByTestId("section-appearance")).toBeInTheDocument()
    const trigger = await screen.findByTestId("number-format-locale-select")
    expect(trigger).toBeInTheDocument()
    // The Region & formatting control is now a shadcn/Radix Select; its
    // options live in a portalled listbox that only mounts on open.
    await waitFor(() => expect(trigger).not.toBeDisabled())
    await user.click(trigger)
    const listbox = await screen.findByRole("listbox")
    // Auto-detect (the "" sentinel, surfaced as "auto") + one explicit
    // BCP-47 locale both render.
    expect(within(listbox).getByRole("option", { name: /auto-detect/i })).toBeInTheDocument()
    expect(within(listbox).getByRole("option", { name: /czech \(czechia\)/i })).toBeInTheDocument()
  })

  it("changing Region & formatting fires PATCH /settings/{field} (#1683)", async () => {
    let lastPatch: { field: string; body: unknown } | null = null
    server.use(
      ...withGroupHandlers({}),
      msw.patch(api("/g/household/settings/:field"), async ({ params, request }) => {
        const body = (await request.json()) as unknown
        lastPatch = { field: String(params.field), body }
        return HttpResponse.json({ appearanceNumberFormatLocale: body })
      })
    )
    const user = userEvent.setup()
    renderSettings("/settings?g=household")
    await screen.findByTestId("settings-page")
    await user.click(await screen.findByTestId("settings-nav-appearance"))
    const trigger = await screen.findByTestId("number-format-locale-select", undefined, {
      timeout: 4000,
    })
    // The trigger is disabled while the GET /settings response is in
    // flight (settings === undefined). Wait until it's interactive so the
    // pick isn't a no-op — same pattern as the notification row toggle.
    await waitFor(() => expect(trigger).not.toBeDisabled(), { timeout: 4000 })
    await pickRadixSelect(user, /^Region & formatting$/i, {
      optionLabel: /czech \(czechia\)/i,
    })
    await waitFor(
      () => {
        expect(lastPatch).not.toBeNull()
        expect(lastPatch?.field).toBe("appearance.number_format_locale")
        expect(lastPatch?.body).toBe("cs-CZ")
      },
      { timeout: 4000 }
    )
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

  it("help section shows the merged support/feedback row and a version badge on What's new", async () => {
    server.use(...baseHandlers)
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-help"))
    // #1387 folded the static "Contact support" mailto row into the
    // feedback entry point — the dialog's "Question" type doubles as
    // the support channel, so there's a single row now.
    expect(screen.queryByTestId("help-row-contactSupport")).not.toBeInTheDocument()
    const feedbackRow = screen.getByTestId("help-row-feedback")
    expect(feedbackRow).toHaveTextContent("Contact support / share feedback")
    expect(screen.getByTestId("help-row-whatsNew-badge")).toBeInTheDocument()
  })

  it("merged support/feedback row opens the FeedbackDialog in place (#1387)", async () => {
    server.use(...baseHandlers)
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-help"))
    const row = screen.getByTestId("help-row-feedback")
    // It's a click-to-open trigger, not a navigation link.
    expect(row.tagName).toBe("BUTTON")
    expect(screen.queryByTestId("feedback-dialog")).not.toBeInTheDocument()
    await user.click(row)
    expect(await screen.findByTestId("feedback-dialog")).toBeVisible()
  })

  it("Keyboard shortcuts row opens the cheat-sheet dialog in place (#1385)", async () => {
    server.use(...baseHandlers)
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-help"))
    const row = screen.getByTestId("help-row-shortcuts")
    // The row is a click-to-open trigger, not a navigation link — the
    // dialog handles its own routing-free lifecycle.
    expect(row.tagName).toBe("BUTTON")
    expect(screen.queryByTestId("keyboard-shortcuts-dialog")).not.toBeInTheDocument()
    await user.click(row)
    expect(await screen.findByTestId("keyboard-shortcuts-dialog")).toBeVisible()
  })
})
