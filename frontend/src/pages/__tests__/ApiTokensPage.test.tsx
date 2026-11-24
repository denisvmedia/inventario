import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { ApiTokensPage } from "@/pages/ApiTokensPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

const SECRET = "inv_pat_0123456789abcdefghijABCDEFGHIJklmnopqrstu"

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

function renderPage() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: "/profile/api-tokens",
    routes: (
      <Route
        path="/profile/api-tokens"
        element={
          <AuthProvider>
            <GroupProvider>
              <ApiTokensPage />
            </GroupProvider>
          </AuthProvider>
        }
      />
    ),
  })
}

function meHandler(extra: Record<string, unknown> = {}) {
  return msw.get(api("/auth/me"), () =>
    HttpResponse.json({
      id: "u1",
      email: "alex@example.com",
      name: "Alex",
      has_password: true,
      ...extra,
    })
  )
}

function mfaHandler(state: "none" | "active") {
  return msw.get(api("/auth/mfa/status"), () =>
    HttpResponse.json({ state, backup_codes_remaining: 0 })
  )
}

const groupsHandler = msw.get(api("/groups"), () =>
  HttpResponse.json({
    data: [
      {
        id: "g-home",
        type: "groups",
        attributes: { name: "Home", slug: "home-slug", status: "active" },
      },
    ],
  })
)

const tokens = [
  {
    id: "t-ha",
    name: "Home Assistant",
    token_prefix: "inv_pat_AbCd",
    scope: "read_only",
    group_id: "g-home",
    created_at: "2026-05-13T08:00:00Z",
    last_used_at: "2026-05-14T07:55:00Z",
    expired: false,
  },
  {
    id: "t-old",
    name: "Old export job",
    token_prefix: "inv_pat_WxYz",
    scope: "read_write",
    created_at: "2026-01-10T12:00:00Z",
    expires_at: "2026-02-10T12:00:00Z",
    expired: true,
  },
]

describe("<ApiTokensPage />", () => {
  it("lists tokens with scope, group, last use and expiry", async () => {
    server.use(
      meHandler(),
      groupsHandler,
      msw.get(api("/users/me/api-tokens"), () => HttpResponse.json({ tokens }))
    )
    renderPage()
    const list = await screen.findByTestId("api-tokens-list")
    const cards = within(list).getAllByTestId("api-token-card")
    expect(cards).toHaveLength(2)

    expect(within(cards[0]).getByText("Home Assistant")).toBeInTheDocument()
    expect(within(cards[0]).getByTestId("api-token-scope-badge")).toHaveTextContent("Read only")
    // The group name arrives with the group list.
    await waitFor(() =>
      expect(
        within(cards[0])
          .getByText(/inv_pat_AbCd/)
          .closest("p")
      ).toHaveTextContent("inv_pat_AbCd… · Home")
    )
    expect(within(cards[0]).getByTestId("api-token-usage")).toHaveTextContent("No expiry")
    expect(within(cards[0]).queryByTestId("api-token-expired-badge")).not.toBeInTheDocument()

    expect(within(cards[1]).getByTestId("api-token-scope-badge")).toHaveTextContent(
      "Read and write"
    )
    expect(within(cards[1]).getByTestId("api-token-expired-badge")).toBeInTheDocument()
    expect(within(cards[1]).getByTestId("api-token-usage")).toHaveTextContent("Never used")
    expect(within(cards[1]).getByText(/All your groups/)).toBeInTheDocument()
  })

  it("shows the empty state", async () => {
    server.use(
      meHandler(),
      groupsHandler,
      msw.get(api("/users/me/api-tokens"), () => HttpResponse.json({ tokens: [] }))
    )
    renderPage()
    expect(await screen.findByTestId("api-tokens-empty")).toBeInTheDocument()
  })

  it("creates a token, shows it once, and forgets it when the dialog closes", async () => {
    let body: Record<string, unknown> | null = null
    server.use(
      meHandler(),
      mfaHandler("none"),
      groupsHandler,
      msw.get(api("/users/me/api-tokens"), () => HttpResponse.json({ tokens: [] })),
      msw.post(api("/users/me/api-tokens"), async ({ request }) => {
        body = (await request.json()) as Record<string, unknown>
        return HttpResponse.json(
          {
            token: {
              id: "t-new",
              name: "Cron export",
              token_prefix: SECRET.slice(0, 12),
              scope: "read_write",
            },
            secret: SECRET,
          },
          { status: 201 }
        )
      })
    )
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("api-tokens-empty")

    await user.click(screen.getByTestId("api-tokens-create-btn"))
    const dialog = await screen.findByTestId("api-token-create-dialog")
    await user.type(within(dialog).getByTestId("api-token-name"), "Cron export")
    await user.click(within(dialog).getByTestId("api-token-scope-read_write"))
    await user.type(within(dialog).getByTestId("api-token-password"), "Password123")
    await user.click(within(dialog).getByTestId("api-token-create-submit"))

    const secret = await screen.findByTestId("api-token-secret")
    expect(secret).toHaveValue(SECRET)
    expect(body).toMatchObject({
      name: "Cron export",
      scope: "read_write",
      current_password: "Password123",
    })
    // The default expiry preset is 90 days, sent as an absolute time.
    const expiresAt = Date.parse(String(body!["expires_at"]))
    const days = (expiresAt - Date.now()) / (24 * 60 * 60 * 1000)
    expect(days).toBeGreaterThan(89)
    expect(days).toBeLessThan(91)
    expect(body!["group_id"]).toBeUndefined()

    await user.click(screen.getByTestId("api-token-reveal-done"))
    await waitFor(() =>
      expect(screen.queryByTestId("api-token-create-dialog")).not.toBeInTheDocument()
    )

    // Reopening starts from an empty form; the secret is gone.
    await user.click(screen.getByTestId("api-tokens-create-btn"))
    await screen.findByTestId("api-token-name")
    expect(screen.queryByTestId("api-token-secret")).not.toBeInTheDocument()
    expect(screen.queryByDisplayValue(SECRET)).not.toBeInTheDocument()
  })

  it("validates the name and password before calling the server", async () => {
    let posted = false
    server.use(
      meHandler(),
      mfaHandler("none"),
      groupsHandler,
      msw.get(api("/users/me/api-tokens"), () => HttpResponse.json({ tokens: [] })),
      msw.post(api("/users/me/api-tokens"), () => {
        posted = true
        return HttpResponse.json({}, { status: 201 })
      })
    )
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("api-tokens-empty")
    await user.click(screen.getByTestId("api-tokens-create-btn"))
    await user.click(await screen.findByTestId("api-token-create-submit"))
    expect(await screen.findByText("Give the token a name.")).toBeInTheDocument()
    expect(screen.getByText("Enter your current password.")).toBeInTheDocument()
    expect(posted).toBe(false)
  })

  it("maps a wrong password to its own message", async () => {
    server.use(
      meHandler(),
      mfaHandler("none"),
      groupsHandler,
      msw.get(api("/users/me/api-tokens"), () => HttpResponse.json({ tokens: [] })),
      msw.post(api("/users/me/api-tokens"), () =>
        HttpResponse.json(
          { errors: [{ status: "Unprocessable Entity", code: "api_token.invalid_password" }] },
          { status: 422 }
        )
      )
    )
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("api-tokens-empty")
    await user.click(screen.getByTestId("api-tokens-create-btn"))
    const dialog = await screen.findByTestId("api-token-create-dialog")
    await user.type(within(dialog).getByTestId("api-token-name"), "Script")
    await user.type(within(dialog).getByTestId("api-token-password"), "wrong")
    await user.click(within(dialog).getByTestId("api-token-create-submit"))
    expect(await screen.findByTestId("api-token-create-error")).toHaveTextContent(
      "The password is incorrect."
    )
  })

  it("asks for a second factor when MFA is on and skips the password for OAuth-only accounts", async () => {
    let body: Record<string, unknown> | null = null
    server.use(
      meHandler({ has_password: false }),
      mfaHandler("active"),
      groupsHandler,
      msw.get(api("/users/me/api-tokens"), () => HttpResponse.json({ tokens: [] })),
      msw.post(api("/users/me/api-tokens"), async ({ request }) => {
        body = (await request.json()) as Record<string, unknown>
        return HttpResponse.json(
          { token: { id: "t", name: "Script" }, secret: SECRET },
          { status: 201 }
        )
      })
    )
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("api-tokens-empty")
    await user.click(screen.getByTestId("api-tokens-create-btn"))
    const dialog = await screen.findByTestId("api-token-create-dialog")
    expect(within(dialog).queryByTestId("api-token-password")).not.toBeInTheDocument()
    await user.type(within(dialog).getByTestId("api-token-name"), "Script")
    await user.type(await within(dialog).findByTestId("api-token-code"), "123456")
    await user.click(within(dialog).getByTestId("api-token-create-submit"))
    await screen.findByTestId("api-token-secret")
    expect(body).toMatchObject({ totp_code: "123456" })
    expect(body!["current_password"]).toBeUndefined()
  })

  it("revokes a token after confirmation", async () => {
    let revoked = ""
    let listCalls = 0
    server.use(
      meHandler(),
      groupsHandler,
      msw.get(api("/users/me/api-tokens"), () => {
        listCalls += 1
        return HttpResponse.json({ tokens: listCalls === 1 ? tokens : [tokens[1]] })
      }),
      msw.delete(api("/users/me/api-tokens/:id"), ({ params }) => {
        revoked = String(params.id)
        return new HttpResponse(null, { status: 204 })
      })
    )
    const user = userEvent.setup()
    renderPage()
    const list = await screen.findByTestId("api-tokens-list")
    await user.click(within(list).getAllByTestId("api-token-revoke-btn")[0])
    await user.click(await screen.findByTestId("api-tokens-confirm-revoke-btn"))
    await waitFor(() => expect(revoked).toBe("t-ha"))
    await waitFor(() =>
      expect(
        within(screen.getByTestId("api-tokens-list")).getAllByTestId("api-token-card")
      ).toHaveLength(1)
    )
  })
})
