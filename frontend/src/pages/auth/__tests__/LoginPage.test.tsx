import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { LoginPage } from "@/pages/auth/LoginPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, getAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { clearPendingInvite, savePendingInvite } from "@/features/auth/inviteHandoff"
import { clearPendingFirstItem, savePendingFirstItem } from "@/features/auth/firstItemHandoff"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname} data-search={loc.search} />
}

function renderLogin(initial = "/login") {
  return renderWithProviders({
    initialPath: initial,
    routes: (
      <>
        <Route
          path="/login"
          element={
            <AuthProvider>
              <LoginPage />
            </AuthProvider>
          }
        />
        <Route path="*" element={<LocationProbe />} />
      </>
    ),
  })
}

beforeEach(() => {
  clearAuth()
  clearPendingInvite()
  clearPendingFirstItem()
  __resetGroupContextForTests()
  __resetHttpForTests()
  // OAuthRow (#1394) probes /auth/oauth/providers on mount. Tests that
  // don't override this still need a handler so MSW doesn't surface an
  // unhandled-request error. Default to no providers — the row hides
  // itself and the login form behaves identically to the pre-#1394 layout.
  server.use(msw.get(api("/auth/oauth/providers"), () => HttpResponse.json({ providers: [] })))
  // LoginPage reads the magic-link deployment flag on mount via
  // useFeatureFlag (gating the "email me a sign-in link" affordance). Same
  // reasoning as the OAuth stub above: provide a default handler so the
  // mount-time GET /feature-flags doesn't trip MSW's
  // `onUnhandledRequest: "error"`. Default both flags off — the magic-link
  // button stays hidden and the password flow is unchanged. Tests that
  // exercise the magic-link entry point override with `magic_link_login: true`.
  server.use(
    msw.get(api("/feature-flags"), () =>
      HttpResponse.json({ currency_migration: false, magic_link_login: false })
    )
  )
})

describe("<LoginPage />", () => {
  it("validates required fields before submitting", async () => {
    const user = userEvent.setup()
    renderLogin()
    await user.click(screen.getByTestId("login-button"))
    expect(await screen.findByTestId("email-error")).toBeInTheDocument()
    expect(screen.getByTestId("password-error")).toBeInTheDocument()
  })

  it("renders the session message when ?reason=session_expired is present", () => {
    renderLogin("/login?reason=session_expired")
    expect(screen.getByTestId("session-message")).toHaveTextContent(/session has expired/i)
  })

  it("submits credentials, stores the token, and navigates on success", async () => {
    server.use(
      msw.post(api("/auth/login"), async ({ request }) => {
        const body = (await request.json()) as { email: string; password: string }
        expect(body).toEqual({ email: "alex@example.com", password: "secret-pw" })
        return HttpResponse.json({
          access_token: "tok",
          csrf_token: "csrf",
          user: { id: "u1", email: "alex@example.com", name: "Alex" },
        })
      })
    )
    const user = userEvent.setup()
    renderLogin("/login?redirect=/g/household")
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.type(screen.getByTestId("password"), "secret-pw")
    await user.click(screen.getByTestId("login-button"))

    await waitFor(() => expect(getAccessToken()).toBe("tok"))
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/g/household")
    )
  })

  it("routes a fresh login to /welcome when a first-item draft is pending, even without ?redirect (#2015)", async () => {
    // The #1988 anonymous hand-off now goes via /register → verify-email →
    // /login, which drops the ?redirect query. finalizeLogin still routes to
    // /welcome off the pending marker, and the isAuthenticated guard must NOT
    // override that to "/" on the post-login re-render (the race that
    // interrupted FirstItemResolver mid-replay — #2015).
    savePendingFirstItem({ draftKey: "commodity-draft:anon:create", currency: "USD", savedAt: 1 })
    server.use(
      msw.post(api("/auth/login"), () =>
        HttpResponse.json({
          access_token: "tok",
          csrf_token: "csrf",
          user: { id: "u1", email: "alex@example.com", name: "Alex" },
        })
      )
    )
    const user = userEvent.setup()
    renderLogin("/login")
    // Dismiss the reassurance drawer so its overlay can't intercept the form.
    await user.click(await screen.findByTestId("pending-first-item-drawer-ok"))
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.type(screen.getByTestId("password"), "secret-pw")
    await user.click(screen.getByTestId("login-button"))

    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/welcome")
    )
  })

  it("surfaces the server error inline when the API returns 401", async () => {
    server.use(
      msw.post(api("/auth/login"), () =>
        HttpResponse.json({ error: "Invalid credentials" }, { status: 401 })
      )
    )
    const user = userEvent.setup()
    renderLogin()
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.type(screen.getByTestId("password"), "wrong")
    await user.click(screen.getByTestId("login-button"))
    expect(await screen.findByTestId("server-error")).toHaveTextContent(/invalid credentials/i)
  })

  // Unhappy paths beyond 401 (#1038). Every login failure funnels through
  // parseServerError into the same destructive banner and the page stays on
  // /login — none of these trigger a refresh-and-retry (/auth/login is a
  // NON_REFRESHABLE_AUTH_PATHS entry) or a navigation.
  it("surfaces a 422 validation error from the JSON:API envelope", async () => {
    server.use(
      msw.post(api("/auth/login"), () =>
        HttpResponse.json(
          { errors: [{ detail: "Email and password are required" }] },
          { status: 422 }
        )
      )
    )
    const user = userEvent.setup()
    renderLogin()
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.type(screen.getByTestId("password"), "secret-pw")
    await user.click(screen.getByTestId("login-button"))
    expect(await screen.findByTestId("server-error")).toHaveTextContent(
      /email and password are required/i
    )
    // Still on the login page — no token stored, no navigation away.
    expect(screen.getByTestId("login-page")).toBeInTheDocument()
    expect(getAccessToken()).toBeNull()
  })

  it("surfaces a 429 rate-limit error inline", async () => {
    server.use(
      msw.post(api("/auth/login"), () =>
        HttpResponse.json(
          { error: "Too many attempts. Please wait and try again." },
          { status: 429 }
        )
      )
    )
    const user = userEvent.setup()
    renderLogin()
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.type(screen.getByTestId("password"), "secret-pw")
    await user.click(screen.getByTestId("login-button"))
    expect(await screen.findByTestId("server-error")).toHaveTextContent(/too many attempts/i)
    expect(screen.getByTestId("login-page")).toBeInTheDocument()
  })

  it("falls back to generic copy when a 5xx returns an opaque body", async () => {
    // 500 (not 503 — a 503 bounces the shell to /maintenance) with no useful
    // body: parseServerError can extract nothing, so the page shows the
    // generic auth:login.errorGeneric copy rather than an empty banner.
    server.use(msw.post(api("/auth/login"), () => new HttpResponse(null, { status: 500 })))
    const user = userEvent.setup()
    renderLogin()
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.type(screen.getByTestId("password"), "secret-pw")
    await user.click(screen.getByTestId("login-button"))
    expect(await screen.findByTestId("server-error")).toHaveTextContent(/sign-in failed/i)
    expect(screen.getByTestId("login-page")).toBeInTheDocument()
  })

  it("auto-accepts a pending invite after successful login", async () => {
    savePendingInvite({ token: "inv-tok", groupName: "Household" })
    let acceptCalls = 0
    server.use(
      msw.post(api("/auth/login"), () =>
        HttpResponse.json({
          access_token: "tok",
          csrf_token: "csrf",
          user: { id: "u1", email: "alex@example.com", name: "Alex" },
        })
      ),
      msw.post(api("/invites/inv-tok/accept"), () => {
        acceptCalls++
        return HttpResponse.json({ data: { id: "m1", attributes: { group_id: "g1" } } })
      })
    )
    const user = userEvent.setup()
    renderLogin()
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.type(screen.getByTestId("password"), "secret")
    await user.click(screen.getByTestId("login-button"))
    await waitFor(() => expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/"))
    expect(acceptCalls).toBe(1)
  })

  it("opens the first-item reassurance drawer when a draft is pending (#1988)", async () => {
    savePendingFirstItem({ draftKey: "commodity-draft:anon:create", currency: "USD", savedAt: 1 })
    renderLogin()
    const drawer = await screen.findByTestId("pending-first-item-drawer")
    expect(drawer).toBeInTheDocument()
    expect(drawer).toHaveTextContent("Your item is saved")
    expect(screen.getByTestId("pending-first-item-drawer-ok")).toBeInTheDocument()
  })

  it("omits the first-item drawer when no draft is pending", async () => {
    renderLogin()
    await screen.findByTestId("login-page")
    expect(screen.queryByTestId("pending-first-item-drawer")).not.toBeInTheDocument()
  })

  it("offers a resume pill that routes back to the drafted item (#1988)", async () => {
    savePendingFirstItem({ draftKey: "commodity-draft:anon:create", currency: "USD", savedAt: 1 })
    // The reassurance drawer is modal (vaul sets pointer-events:none on the
    // rest of the page), so disable the pointer-events guard — we're asserting
    // the pill's navigation wiring, not the modal's dismiss interaction.
    const user = userEvent.setup({ pointerEventsCheck: 0 })
    renderLogin()
    const pill = await screen.findByTestId("resume-first-item-pill")
    await user.click(pill)
    await waitFor(() => {
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/")
    })
    expect(screen.getByTestId("loc").getAttribute("data-search")).toBe("?addFirstItem=1")
  })

  it("omits the resume pill when no draft is pending", async () => {
    renderLogin()
    await screen.findByTestId("login-page")
    expect(screen.queryByTestId("resume-first-item-pill")).not.toBeInTheDocument()
  })

  it("has no axe violations on the form", async () => {
    const { container } = renderLogin()
    expect(await axe(container)).toHaveNoViolations()
  })

  it("falls back to / when ?redirect= points off the app (open-redirect guard)", async () => {
    server.use(
      msw.post(api("/auth/login"), () =>
        HttpResponse.json({
          access_token: "tok",
          csrf_token: "csrf",
          user: { id: "u1", email: "alex@example.com", name: "Alex" },
        })
      )
    )
    const user = userEvent.setup()
    renderLogin("/login?redirect=//evil.example/foo")
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.type(screen.getByTestId("password"), "secret")
    await user.click(screen.getByTestId("login-button"))
    await waitFor(() => expect(getAccessToken()).toBe("tok"))
    await waitFor(() => expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/"))
  })

  // #1645 — when the backend short-circuits with mfa_required, the page
  // swaps the password form for the code-entry surface and waits for
  // step-2 before storing tokens or navigating.
  it("renders the MFA challenge surface when the server requires a code", async () => {
    server.use(
      msw.post(api("/auth/login"), () =>
        HttpResponse.json({
          mfa_required: true,
          mfa_token: "challenge-jwt",
          expires_in: 300,
          email: "alex@example.com",
        })
      )
    )
    const user = userEvent.setup()
    renderLogin("/login?redirect=/g/household")
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.type(screen.getByTestId("password"), "secret-pw")
    await user.click(screen.getByTestId("login-button"))

    expect(await screen.findByTestId("mfa-challenge")).toBeInTheDocument()
    expect(screen.queryByTestId("login-page")).not.toBeInTheDocument()
    // Step-1 must NOT have stored credentials yet — we're between
    // password-accepted and code-verified.
    expect(getAccessToken()).toBeNull()
  })

  it("completes login after a valid TOTP code is submitted", async () => {
    server.use(
      msw.post(api("/auth/login"), () =>
        HttpResponse.json({
          mfa_required: true,
          mfa_token: "challenge-jwt",
          expires_in: 300,
          email: "alex@example.com",
        })
      ),
      msw.post(api("/auth/login/mfa"), async ({ request }) => {
        const body = (await request.json()) as Record<string, string>
        expect(body.mfa_token).toBe("challenge-jwt")
        expect(body.totp_code).toBe("123456")
        return HttpResponse.json({
          access_token: "tok-mfa",
          csrf_token: "csrf-mfa",
          user: { id: "u1", email: "alex@example.com", name: "Alex" },
        })
      })
    )
    const user = userEvent.setup()
    renderLogin("/login?redirect=/g/household")
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.type(screen.getByTestId("password"), "secret-pw")
    await user.click(screen.getByTestId("login-button"))
    await user.type(await screen.findByTestId("mfa-code-input"), "123456")
    await user.click(screen.getByTestId("mfa-submit"))
    await waitFor(() => expect(getAccessToken()).toBe("tok-mfa"))
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/g/household")
    )
  })

  // #1394 — when the OAuth callback redirects here with
  // `?oauth_link_required=1&email=…&provider=…`, the page shows a
  // dedicated banner prompting the user to sign in with their password
  // before linking the provider from Settings.
  it("renders the OAuth link-required banner with the provider name when ?provider= is set", () => {
    renderLogin("/login?oauth_link_required=1&email=denis%40example.com&provider=google")
    const banner = screen.getByTestId("oauth-link-required-banner")
    expect(banner).toBeInTheDocument()
    expect(banner).toHaveTextContent(/google/i)
    expect(banner).toHaveTextContent(/denis@example\.com/i)
  })

  // #1394 — defensive fallback: if a redirect ever lands here without
  // `?provider=`, the banner falls back to a generic "your provider"
  // string rather than rendering an empty hole. The BE fix-up batch is
  // closing the gap by always emitting `provider=`, but the FE keeps the
  // fallback for forward-compat.
  it("falls back to 'your provider' when ?provider= is missing from the link-required redirect", () => {
    renderLogin("/login?oauth_link_required=1&email=denis%40example.com")
    const banner = screen.getByTestId("oauth-link-required-banner")
    expect(banner).toBeInTheDocument()
    expect(banner).toHaveTextContent(/your provider/i)
    expect(banner).toHaveTextContent(/denis@example\.com/i)
  })

  it("omits the OAuth link-required banner when the query param is absent", () => {
    renderLogin("/login")
    expect(screen.queryByTestId("oauth-link-required-banner")).not.toBeInTheDocument()
  })

  // #magic-link — the passwordless "email me a sign-in link" entry point is
  // gated on the `magic_link_login` feature flag read at mount. The beforeEach
  // above defaults the flag OFF; these cases assert both gate states and the
  // request → neutral-confirmation flow.
  it("hides the magic-link button when the feature flag is off (default)", async () => {
    renderLogin()
    // The password form is unchanged — both the form and submit button render.
    expect(await screen.findByTestId("login-page")).toBeInTheDocument()
    expect(screen.getByTestId("login-button")).toBeInTheDocument()
    expect(screen.queryByTestId("magic-link-button")).not.toBeInTheDocument()
  })

  it("shows the magic-link button when the feature flag is on", async () => {
    server.use(
      msw.get(api("/feature-flags"), () =>
        HttpResponse.json({ currency_migration: false, magic_link_login: true })
      )
    )
    renderLogin()
    expect(await screen.findByTestId("magic-link-button")).toBeInTheDocument()
  })

  it("blocks the magic-link request and shows the inline email error when the email is empty", async () => {
    let requestCalls = 0
    server.use(
      msw.get(api("/feature-flags"), () =>
        HttpResponse.json({ currency_migration: false, magic_link_login: true })
      ),
      msw.post(api("/auth/magic-link/request"), () => {
        requestCalls++
        return HttpResponse.json({ message: "If that email exists, we sent a link." })
      })
    )
    const user = userEvent.setup()
    renderLogin()
    await user.click(await screen.findByTestId("magic-link-button"))
    // RHF validates just the email field — the empty value surfaces the same
    // inline error a normal submit would, and the request never fires.
    expect(await screen.findByTestId("email-error")).toBeInTheDocument()
    expect(requestCalls).toBe(0)
    // No confirmation swap — still on the form.
    expect(screen.getByTestId("login-page")).toBeInTheDocument()
    expect(screen.queryByTestId("magic-link-sent")).not.toBeInTheDocument()
  })

  it("sends the magic-link request and shows the neutral confirmation on success", async () => {
    server.use(
      msw.get(api("/feature-flags"), () =>
        HttpResponse.json({ currency_migration: false, magic_link_login: true })
      ),
      msw.post(api("/auth/magic-link/request"), async ({ request }) => {
        const body = (await request.json()) as { email: string }
        expect(body.email).toBe("alex@example.com")
        return HttpResponse.json({ message: "If that email exists, we sent a link." })
      })
    )
    const user = userEvent.setup()
    renderLogin()
    await user.type(screen.getByTestId("email"), "alex@example.com")
    await user.click(await screen.findByTestId("magic-link-button"))
    expect(await screen.findByTestId("magic-link-sent")).toBeInTheDocument()
    // The password form is replaced by the confirmation surface.
    expect(screen.queryByTestId("login-page")).not.toBeInTheDocument()
    // The "back to sign in" affordance returns to the form.
    expect(screen.getByTestId("magic-link-back")).toBeInTheDocument()
  })
})
