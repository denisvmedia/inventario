import { beforeEach, describe, expect, it, vi } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import whatsNewMeta from "virtual:whats-new/meta"

import { SettingsPage } from "@/pages/SettingsPage"
import { pickRadixSelect } from "@/test/radix"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { KeyboardShortcutsProvider } from "@/features/shortcuts"
import { ThemeProvider } from "@/components/theme-provider"
import { DensityProvider } from "@/hooks/useDensity"
import { ConfirmProvider } from "@/hooks/useConfirm"
import {
  OnboardingTourControlsProvider,
  type OnboardingTourControls,
} from "@/hooks/useOnboardingTourControls"
import { renderWithProviders } from "@/test/render"
import { featureFlagHandlers } from "@/test/handlers"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function LocationProbe({ testId = "loc" }: { testId?: string }) {
  const loc = useLocation()
  return <div data-testid={testId} data-pathname={loc.pathname} data-search={loc.search} />
}

// Shell provides the product-tour controls in the app; tests pass their own
// to exercise the Help section's "Restart product tour" row (#2787).
const defaultTourControls: OnboardingTourControls = { restart: () => {}, blocker: null }

function renderSettings(
  initialPath: string = "/settings",
  tourControls: OnboardingTourControls = defaultTourControls
) {
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
                        <OnboardingTourControlsProvider value={tourControls}>
                          <SettingsPage />
                        </OnboardingTourControlsProvider>
                        {/* The catch-all probe below only renders when no
                            other route matches, so /settings needs its own to
                            read the query back. Under a different id, because
                            tests that assert "we did not navigate away" read
                            the absence of the catch-all one. */}
                        <LocationProbe testId="settings-loc" />
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
  // The notifications section reads the server's loan lead-time default.
  msw.get(api("/system"), () => HttpResponse.json({ loan_reminder_due_soon_days: 5 })),
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
    msw.get(api("/system"), () => HttpResponse.json({ loan_reminder_due_soon_days: 5 })),
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

  // #1389: the Free plan badge on the account card shows only when the
  // deployment has plans on; the flag is off by default.
  it("hides the plan badge on the account card when plans are off", async () => {
    server.use(...featureFlagHandlers.set({ plans: false }), ...baseHandlers)
    const { queryClient } = renderSettings()
    expect(await screen.findByTestId("section-account")).toBeInTheDocument()
    await waitFor(() =>
      expect(queryClient.getQueryState(["feature-flags"])?.status).toBe("success")
    )
    expect(screen.queryByTestId("settings-account-plan-badge")).not.toBeInTheDocument()
    expect(screen.queryByText("Free plan")).not.toBeInTheDocument()
  })

  it("shows the plan badge on the account card when plans are on", async () => {
    server.use(...featureFlagHandlers.set({ plans: true }), ...baseHandlers)
    renderSettings()
    expect(await screen.findByTestId("settings-account-plan-badge")).toHaveTextContent("Free plan")
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

  // The pages these rows open sit outside /g/:slug, so the row has to carry
  // the active group in ?g=; without it the sidebar there lost the group and
  // showed empty Inventory, Group and Reports sections.
  it("privacy rows keep the active group in ?g=", async () => {
    server.use(
      ...withGroupHandlers(),
      msw.get(api("/users/me/sessions"), () => HttpResponse.json({ sessions: [] })),
      msw.get(api("/users/me/api-tokens"), () => HttpResponse.json({ tokens: [] })),
      msw.get(api("/auth/mfa/status"), () =>
        HttpResponse.json({ state: "none", backup_codes_remaining: 0 })
      )
    )
    const user = userEvent.setup()
    renderSettings("/settings?g=household")
    await user.click(await screen.findByTestId("settings-nav-privacy"))
    await waitFor(() =>
      expect(screen.getByTestId("privacy-row-activeSessions")).toHaveAttribute(
        "href",
        "/profile/sessions?g=household"
      )
    )
    expect(screen.getByTestId("privacy-row-loginHistory")).toHaveAttribute(
      "href",
      "/profile/login-history?g=household"
    )
    expect(screen.getByTestId("privacy-row-apiTokens")).toHaveAttribute(
      "href",
      "/profile/api-tokens?g=household"
    )
  })

  // #2147 — self-service account deletion. The danger-zone button now
  // opens a typed-confirmation dialog (email + password) wired to
  // DELETE /auth/me, replacing the old "not yet available" confirm stub.
  it("account danger zone's delete button opens the delete-account dialog", async () => {
    server.use(...baseHandlers)
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-account"))
    await user.click(screen.getByTestId("delete-account-button"))
    expect(await screen.findByTestId("delete-account-dialog")).toBeInTheDocument()
    expect(screen.getByTestId("delete-confirm-email")).toBeInTheDocument()
    expect(screen.getByTestId("delete-password")).toBeInTheDocument()
  })

  // #2177 — the dialog sends the user to "Download my data" before erasing.
  it("delete-account dialog links to the personal data export", async () => {
    server.use(...baseHandlers)
    const scrollIntoView = vi.fn()
    Element.prototype.scrollIntoView = scrollIntoView
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-account"))
    expect(await screen.findByTestId("personal-data-export")).toBeInTheDocument()
    await waitFor(() => expect(screen.getByTestId("personal-data-export-request")).toBeEnabled())

    await user.click(screen.getByTestId("delete-account-button"))
    await screen.findByTestId("delete-account-dialog")
    expect(screen.getByTestId("delete-export-first")).toHaveTextContent(
      "Want to keep a copy of your data?"
    )
    await user.click(screen.getByTestId("delete-export-first-link"))

    await waitFor(() =>
      expect(screen.queryByTestId("delete-account-dialog")).not.toBeInTheDocument()
    )
    await waitFor(() => expect(scrollIntoView).toHaveBeenCalled())
    expect(screen.getByTestId("personal-data-export-request")).toHaveFocus()
  })

  it("delete-account dialog blocks submit on an email mismatch (client-side)", async () => {
    let deleteCalls = 0
    server.use(
      ...baseHandlers,
      msw.delete(api("/auth/me"), () => {
        deleteCalls++
        return new HttpResponse(null, { status: 204 })
      })
    )
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-account"))
    await user.click(screen.getByTestId("delete-account-button"))
    await screen.findByTestId("delete-account-dialog")
    await user.type(screen.getByTestId("delete-confirm-email"), "wrong@example.com")
    await user.type(screen.getByTestId("delete-password"), "secret-pw")
    await user.click(screen.getByTestId("delete-account-submit"))
    expect(await screen.findByTestId("delete-confirm-email-error")).toBeInTheDocument()
    // The mismatch is caught before the request goes out.
    expect(deleteCalls).toBe(0)
    // Dialog stays open, user stays on /settings.
    expect(screen.getByTestId("delete-account-dialog")).toBeInTheDocument()
  })

  // #2147 follow-up — OAuth-only users have no in-app password hash; the BE
  // skips re-auth for them (the access token proves identity). The dialog
  // must hide the password input + drop the password requirement, otherwise
  // the form is unsubmittable and they can never erase their account.
  it("delete-account dialog hides the password field + still deletes for OAuth-only users", async () => {
    let deleteBody: { password?: string } | null = null
    let logoutCalls = 0
    server.use(
      // /auth/me carries has_password:false for an OAuth-only account →
      // useHasPassword() resolves false once the boot probe lands.
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({
          id: "u1",
          email: "alex@example.com",
          name: "Alex",
          created_at: "2024-01-15T00:00:00Z",
          has_password: false,
        })
      ),
      msw.get(api("/groups"), () => HttpResponse.json({ data: [] })),
      msw.get(api("/auth/oauth/providers"), () => HttpResponse.json({ providers: [] })),
      msw.get(api("/auth/oauth/identities"), () => HttpResponse.json({ identities: [] })),
      msw.delete(api("/auth/me"), async ({ request }) => {
        deleteBody = (await request.json()) as typeof deleteBody
        return new HttpResponse(null, { status: 204 })
      }),
      msw.post(api("/auth/logout"), () => {
        logoutCalls++
        return new HttpResponse(null, { status: 204 })
      })
    )
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-account"))
    await user.click(screen.getByTestId("delete-account-button"))
    await screen.findByTestId("delete-account-dialog")
    // The password input must NOT exist for the OAuth-only case; the
    // social-login note stands in for it instead.
    await waitFor(() => expect(screen.queryByTestId("delete-password")).not.toBeInTheDocument())
    expect(screen.getByTestId("delete-no-password-note")).toBeInTheDocument()
    // Email confirmation still gates the destructive action.
    await user.type(screen.getByTestId("delete-confirm-email"), "alex@example.com")
    await user.click(screen.getByTestId("delete-account-submit"))
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/login")
    )
    // DELETE fired with an empty password (the BE accepts it for OAuth-only),
    // then logout + redirect ran on the 204.
    expect(deleteBody).toEqual({ password: "" })
    expect(logoutCalls).toBe(1)
  })

  it("delete-account dialog surfaces a wrong-password field error on auth.delete.invalid_password", async () => {
    server.use(
      ...baseHandlers,
      msw.delete(api("/auth/me"), () =>
        HttpResponse.json(
          { errors: [{ code: "auth.delete.invalid_password", detail: "re-auth failed" }] },
          { status: 422 }
        )
      )
    )
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-account"))
    await user.click(screen.getByTestId("delete-account-button"))
    await screen.findByTestId("delete-account-dialog")
    await user.type(screen.getByTestId("delete-confirm-email"), "alex@example.com")
    await user.type(screen.getByTestId("delete-password"), "wrong-pw")
    await user.click(screen.getByTestId("delete-account-submit"))
    expect(await screen.findByTestId("delete-password-error")).toBeInTheDocument()
    // Stays open + on /settings; no redirect to /login (the LocationProbe
    // only mounts on a non-/settings route, so its absence proves we never
    // navigated away).
    expect(screen.getByTestId("delete-account-dialog")).toBeInTheDocument()
    expect(screen.queryByTestId("loc")).not.toBeInTheDocument()
  })

  it("delete-account dialog shows a server-error banner on auth.delete.last_owner", async () => {
    server.use(
      ...baseHandlers,
      msw.delete(api("/auth/me"), () =>
        HttpResponse.json(
          { errors: [{ code: "auth.delete.last_owner", detail: "sole owner" }] },
          { status: 422 }
        )
      )
    )
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-account"))
    await user.click(screen.getByTestId("delete-account-button"))
    await screen.findByTestId("delete-account-dialog")
    await user.type(screen.getByTestId("delete-confirm-email"), "alex@example.com")
    await user.type(screen.getByTestId("delete-password"), "secret-pw")
    await user.click(screen.getByTestId("delete-account-submit"))
    expect(await screen.findByTestId("delete-server-error")).toHaveTextContent(
      /transfer ownership/i
    )
    expect(screen.getByTestId("delete-account-dialog")).toBeInTheDocument()
  })

  it("delete-account dialog logs out + navigates to /login on a 204 success", async () => {
    let deleteBody: { password?: string } | null = null
    let logoutCalls = 0
    server.use(
      ...baseHandlers,
      msw.delete(api("/auth/me"), async ({ request }) => {
        deleteBody = (await request.json()) as typeof deleteBody
        return new HttpResponse(null, { status: 204 })
      }),
      msw.post(api("/auth/logout"), () => {
        logoutCalls++
        return new HttpResponse(null, { status: 204 })
      })
    )
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-account"))
    await user.click(screen.getByTestId("delete-account-button"))
    await screen.findByTestId("delete-account-dialog")
    await user.type(screen.getByTestId("delete-confirm-email"), "alex@example.com")
    await user.type(screen.getByTestId("delete-password"), "secret-pw")
    await user.click(screen.getByTestId("delete-account-submit"))
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/login")
    )
    expect(deleteBody).toEqual({ password: "secret-pw" })
    expect(logoutCalls).toBe(1)
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

  it("saves only the default group, not the name this page loaded (#2910)", async () => {
    const puts: unknown[] = []
    server.use(...withGroupHandlers({}))
    server.use(
      msw.get(api("/groups"), () =>
        HttpResponse.json({
          data: [
            {
              id: "g1",
              type: "groups",
              attributes: { id: "g1", slug: "household", name: "Household" },
            },
            { id: "g2", type: "groups", attributes: { id: "g2", slug: "office", name: "Office" } },
          ],
        })
      ),
      msw.put(api("/auth/me"), async ({ request }) => {
        puts.push(await request.json())
        return HttpResponse.json({
          id: "u1",
          email: "alex@example.com",
          name: "Alex",
          default_group_id: "g2",
          created_at: "2024-01-15T00:00:00Z",
        })
      })
    )
    const user = userEvent.setup()
    renderSettings("/settings?g=household")
    const trigger = await screen.findByTestId("settings-default-group-select")
    await waitFor(() => expect(trigger).toHaveTextContent("Household"))
    await pickRadixSelect(user, /^Default group$/i, { optionLabel: /Office/ })
    await waitFor(() => expect(puts).toEqual([{ default_group_id: "g2" }]))
  })

  it("changing Default view saves it and drops this device's toolbar layout (#2797)", async () => {
    localStorage.setItem("commodities:viewMode", "list")
    let lastPatch: { field: string; body: unknown } | null = null
    server.use(
      ...withGroupHandlers({}),
      msw.patch(api("/g/household/settings/:field"), async ({ params, request }) => {
        const body = (await request.json()) as unknown
        lastPatch = { field: String(params.field), body }
        return HttpResponse.json({ appearanceDefaultItemsView: body })
      })
    )
    const user = userEvent.setup()
    renderSettings("/settings?g=household")
    await user.click(await screen.findByTestId("settings-nav-appearance"))
    const trigger = await screen.findByTestId("default-view-select", undefined, { timeout: 4000 })
    await waitFor(() => expect(trigger).not.toBeDisabled(), { timeout: 4000 })
    await pickRadixSelect(user, /^Default view$/i, { optionLabel: /compact cards/i })
    await waitFor(
      () => {
        expect(lastPatch?.field).toBe("appearance.default_items_view")
        expect(lastPatch?.body).toBe("compact")
      },
      { timeout: 4000 }
    )
    expect(localStorage.getItem("commodities:viewMode")).toBeNull()
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

  it("notifications section renders every toggle row with mock-spec subgroup chrome", async () => {
    server.use(...withGroupHandlers({}))
    const user = userEvent.setup()
    renderSettings("/settings?g=household")
    await user.click(await screen.findByTestId("settings-nav-notifications"))
    expect(screen.getByTestId("notification-row-warranty-expiry")).toBeInTheDocument()
    expect(screen.getByTestId("notification-row-maintenance-reminder")).toBeInTheDocument()
    expect(screen.getByTestId("notification-row-loan-reminder")).toBeInTheDocument()
    expect(screen.getByTestId("notification-row-item-reminder")).toBeInTheDocument()
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

  it("reads the stored item reminder preference and patches notifications.item_reminder", async () => {
    let lastPatch: { field: string; body: unknown } | null = null
    server.use(
      ...withGroupHandlers({ notificationsItemReminder: false }),
      msw.patch(api("/g/household/settings/:field"), async ({ params, request }) => {
        const body = (await request.json()) as unknown
        lastPatch = { field: String(params.field), body }
        return HttpResponse.json({ notificationsItemReminder: body })
      })
    )
    const user = userEvent.setup()
    renderSettings("/settings?g=household")
    await screen.findByTestId("settings-page")
    await user.click(await screen.findByTestId("settings-nav-notifications"))
    const row = await screen.findByTestId("notification-row-item-reminder", undefined, {
      timeout: 4000,
    })
    const toggle = await waitFor(() => {
      const t = row.querySelector("button[role='switch']") as HTMLButtonElement | null
      expect(t).not.toBeNull()
      expect(t?.hasAttribute("disabled")).toBe(false)
      return t as HTMLButtonElement
    })
    // The stored value wins over the default (on).
    expect(toggle).toHaveAttribute("aria-checked", "false")
    await user.click(toggle)
    await waitFor(() => {
      expect(lastPatch?.field).toBe("notifications.item_reminder")
      expect(lastPatch?.body).toBe(true)
    })
  })

  describe("reminder lead times", () => {
    async function openNotifications() {
      const user = userEvent.setup()
      renderSettings("/settings?g=household")
      await screen.findByTestId("settings-page")
      await user.click(await screen.findByTestId("settings-nav-notifications"))
      return user
    }

    function recordPatches(respond: (field: string, body: unknown) => Record<string, unknown>) {
      const patches: Array<{ field: string; body: unknown }> = []
      server.use(
        msw.patch(api("/g/household/settings/:field"), async ({ params, request }) => {
          const body = (await request.json()) as unknown
          patches.push({ field: String(params.field), body })
          return HttpResponse.json(respond(String(params.field), body))
        })
      )
      return patches
    }

    it("shows the defaults, the loan one from the server", async () => {
      server.use(...withGroupHandlers({}))
      await openNotifications()
      expect(await screen.findByTestId("lead-times-warranty-input")).toHaveValue("60, 30, 7")
      expect(screen.getByTestId("lead-times-maintenance-input")).toHaveValue("14, 7, 1")
      await waitFor(() => expect(screen.getByTestId("lead-times-loan-input")).toHaveValue("5"))
      expect(screen.queryByTestId("lead-times-warranty-reset")).not.toBeInTheDocument()
    })

    it("saves a list on blur and resets it to the default", async () => {
      server.use(...withGroupHandlers({}))
      const patches = recordPatches((_field, body) =>
        body === null ? {} : { notificationsWarrantyLeadTimes: body }
      )
      const user = await openNotifications()
      const input = await screen.findByTestId("lead-times-warranty-input")
      await user.clear(input)
      await user.type(input, "7, 90")
      await user.tab()
      await waitFor(() =>
        expect(patches).toEqual([{ field: "notifications.warranty_lead_times", body: [90, 7] }])
      )
      expect(await screen.findByTestId("lead-times-warranty-input")).toHaveValue("90, 7")

      await user.click(await screen.findByTestId("lead-times-warranty-reset"))
      await waitFor(() => expect(patches).toHaveLength(2))
      expect(patches[1]).toEqual({ field: "notifications.warranty_lead_times", body: null })
      await waitFor(() =>
        expect(screen.getByTestId("lead-times-warranty-input")).toHaveValue("60, 30, 7")
      )
    })

    it("sends the loan lead time as a number", async () => {
      server.use(...withGroupHandlers({ notificationsLoanLeadTime: 10 }))
      const patches = recordPatches((_field, body) => ({ notificationsLoanLeadTime: body }))
      const user = await openNotifications()
      const input = await screen.findByTestId("lead-times-loan-input")
      expect(input).toHaveValue("10")
      expect(screen.getByTestId("lead-times-loan-reset")).toHaveTextContent("5")
      await user.clear(input)
      await user.type(input, "21{Enter}")
      await waitFor(() =>
        expect(patches).toEqual([{ field: "notifications.loan_lead_time", body: 21 }])
      )
    })

    it("keeps keyboard focus in the field after a save and after Use default", async () => {
      server.use(...withGroupHandlers({}))
      const patches = recordPatches((_field, body) =>
        body === null ? {} : { notificationsWarrantyLeadTimes: body }
      )
      const user = await openNotifications()
      const input = await screen.findByTestId("lead-times-warranty-input")
      await user.clear(input)
      await user.type(input, "90, 7{Enter}")
      await waitFor(() => expect(patches).toHaveLength(1))
      await screen.findByTestId("lead-times-warranty-reset")
      expect(screen.getByTestId("lead-times-warranty-input")).toBe(input)
      expect(input).toHaveFocus()

      await user.tab()
      expect(screen.getByTestId("lead-times-warranty-reset")).toHaveFocus()
      await user.keyboard("{Enter}")
      await waitFor(() => expect(patches).toHaveLength(2))
      await waitFor(() =>
        expect(screen.queryByTestId("lead-times-warranty-reset")).not.toBeInTheDocument()
      )
      expect(input).toHaveValue("60, 30, 7")
      expect(input).toHaveFocus()
    })

    it("explains a value it cannot save and sends nothing", async () => {
      server.use(...withGroupHandlers({}))
      const patches = recordPatches(() => ({}))
      const user = await openNotifications()
      const input = await screen.findByTestId("lead-times-maintenance-input")
      await user.clear(input)
      await user.type(input, "7, 7")
      await user.tab()
      const field = screen.getByTestId("lead-times-maintenance")
      expect(within(field).getByRole("alert")).toHaveTextContent("only once")
      expect(input).toHaveAttribute("aria-invalid", "true")
      expect(patches).toHaveLength(0)
    })

    it("disables the field while the reminder is turned off", async () => {
      server.use(...withGroupHandlers({ notificationsWarrantyExpiry: false }))
      await openNotifications()
      expect(await screen.findByTestId("lead-times-warranty-input")).toBeDisabled()
      expect(screen.getByTestId("lead-times-maintenance-input")).toBeEnabled()
    })
  })

  it("help section shows the merged support/feedback row and a version badge on What's new", async () => {
    // The release version the build injects; the badge shows its Major.Minor.
    vi.stubGlobal("__APP_VERSION__", "v0.2.2")
    try {
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
      expect(screen.getByTestId("help-row-whatsNew-badge")).toHaveTextContent("v0.2")
    } finally {
      vi.unstubAllGlobals()
    }
  })

  it("help section shows no version badge for a build without a release number", async () => {
    // Unit tests get no injected version, so this is the "dev" build; an
    // image built for a PR or from master ("pr-123", "edge") looks the same.
    server.use(...baseHandlers)
    const user = userEvent.setup()
    renderSettings()
    await user.click(await screen.findByTestId("settings-nav-help"))
    expect(screen.getByTestId("help-row-whatsNew")).toBeInTheDocument()
    expect(screen.queryByTestId("help-row-whatsNew-badge")).not.toBeInTheDocument()
  })

  // #1386 — the row opens the in-app page instead of GitHub Releases.
  it("What's new row opens the in-app page", async () => {
    server.use(...withGroupHandlers({}))
    const user = userEvent.setup()
    renderSettings("/settings?g=household")
    await user.click(await screen.findByTestId("settings-nav-help"))
    const row = screen.getByTestId("help-row-whatsNew")
    expect(row).toHaveAttribute("href", "/whats-new?g=household")
    expect(row).not.toHaveAttribute("target")
    await user.click(row)
    expect(await screen.findByTestId("loc")).toHaveAttribute("data-pathname", "/whats-new")
  })

  it("marks the What's new row and the Help tab while a release is unread", async () => {
    server.use(...withGroupHandlers({ whatsNewLastSeenVersion: "0.0.1" }))
    const user = userEvent.setup()
    renderSettings("/settings?g=household")
    expect(await screen.findByTestId("settings-nav-help-unread")).toBeInTheDocument()
    expect(screen.getByTestId("settings-nav-help")).toHaveTextContent("new release")
    await user.click(screen.getByTestId("settings-nav-help"))
    expect(await screen.findByTestId("help-row-whatsNew-unread")).toBeInTheDocument()
  })

  it("shows no unread dot once the newest release was seen", async () => {
    let served = false
    // Listed first so it answers before the default settings handler.
    server.use(
      msw.get(api("/g/household/settings"), () => {
        served = true
        return HttpResponse.json({ whatsNewLastSeenVersion: whatsNewMeta.latestVersion })
      }),
      ...withGroupHandlers({})
    )
    const user = userEvent.setup()
    renderSettings("/settings?g=household")
    await user.click(await screen.findByTestId("settings-nav-help"))
    await waitFor(() => expect(served).toBe(true))
    await new Promise((r) => setTimeout(r, 50))
    expect(screen.queryByTestId("help-row-whatsNew-unread")).not.toBeInTheDocument()
    expect(screen.queryByTestId("settings-nav-help-unread")).not.toBeInTheDocument()
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

  // #2787 — the product tour can be restarted from Help, next to the
  // user-menu entry, through the same restart Shell wires to both.
  it("Restart product tour row restarts the tour", async () => {
    server.use(...baseHandlers)
    const user = userEvent.setup()
    const restart = vi.fn()
    renderSettings("/settings?section=help", { restart, blocker: null })
    const row = await screen.findByTestId("help-row-tour")
    expect(row.tagName).toBe("BUTTON")
    expect(row).toHaveTextContent("Restart product tour")
    expect(row).toBeEnabled()
    await user.click(row)
    expect(restart).toHaveBeenCalledOnce()
  })

  it("Restart product tour row is disabled and says why without a group", async () => {
    server.use(...baseHandlers)
    const user = userEvent.setup()
    const restart = vi.fn()
    renderSettings("/settings?section=help", { restart, blocker: "noGroup" })
    const row = await screen.findByTestId("help-row-tour")
    expect(row).toBeDisabled()
    expect(row).toHaveTextContent("Create or join a group first")
    await user.click(row)
    expect(restart).not.toHaveBeenCalled()
  })

  it("Restart product tour row is disabled and says why on a narrow screen", async () => {
    server.use(...baseHandlers)
    renderSettings("/settings?section=help", { restart: vi.fn(), blocker: "narrowScreen" })
    const row = await screen.findByTestId("help-row-tour")
    expect(row).toBeDisabled()
    expect(row).toHaveTextContent("Use a wider window")
  })
})

// #1384 — the section lives in the URL so it can be linked to: /help
// redirects here, the sidebar's Help row points straight at the help section,
// and a reload keeps the user where they were.
describe("<SettingsPage /> section routing", () => {
  it("opens the section named by ?section=", async () => {
    server.use(...baseHandlers)
    renderSettings("/settings?section=help")
    expect(await screen.findByTestId("section-help")).toBeInTheDocument()
    expect(screen.queryByTestId("section-account")).toBeNull()
  })

  it("lands on Account with no parameter", async () => {
    server.use(...baseHandlers)
    renderSettings("/settings")
    expect(await screen.findByTestId("section-account")).toBeInTheDocument()
  })

  // A stale link or a typo should not render an empty pane.
  it("falls back to Account for an unknown section", async () => {
    server.use(...baseHandlers)
    renderSettings("/settings?section=nonsense")
    expect(await screen.findByTestId("section-account")).toBeInTheDocument()
  })

  it("writes the section to the URL when a tab is clicked", async () => {
    const user = userEvent.setup()
    server.use(...baseHandlers)
    renderSettings("/settings")
    await screen.findByTestId("section-account")
    await user.click(screen.getByTestId("settings-nav-privacy"))
    expect(await screen.findByTestId("section-privacy")).toBeInTheDocument()
    expect(screen.getByTestId("settings-loc").getAttribute("data-search")).toContain(
      "section=privacy"
    )
  })

  // Other query parameters carry the active group (?g=slug); dropping them on
  // a tab click would strand the user's group context.
  it("keeps the other query parameters", async () => {
    const user = userEvent.setup()
    server.use(...baseHandlers)
    renderSettings("/settings?g=household")
    await screen.findByTestId("section-account")
    await user.click(screen.getByTestId("settings-nav-help"))
    await screen.findByTestId("section-help")
    const search = screen.getByTestId("settings-loc").getAttribute("data-search") ?? ""
    expect(search).toContain("g=household")
    expect(search).toContain("section=help")
  })
})
