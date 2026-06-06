import { beforeEach, describe, expect, it, vi } from "vitest"
import { http as msw, HttpResponse, delay } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { FeedbackDialog } from "@/components/feedback/FeedbackDialog"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { __resetNavigationForTests, setNavigateToMaintenance } from "@/lib/navigation"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

const baseUserHandlers = [
  msw.get(api("/auth/me"), () =>
    HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
  ),
  msw.get(api("/groups"), () => HttpResponse.json({ data: [] })),
]

function renderDialog() {
  setAccessToken("good-token")
  const onOpenChange = vi.fn()
  const utils = renderWithProviders({
    initialPath: "/settings",
    routes: (
      <Route
        path="/settings"
        element={
          <AuthProvider>
            <GroupProvider>
              <ConfirmProvider>
                <FeedbackDialog open onOpenChange={onOpenChange} />
              </ConfirmProvider>
            </GroupProvider>
          </AuthProvider>
        }
      />
    ),
  })
  return { ...utils, onOpenChange }
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  __resetNavigationForTests()
})

describe("<FeedbackDialog />", () => {
  it("submits the form and closes on success", async () => {
    let received: Record<string, unknown> | null = null
    server.use(
      ...baseUserHandlers,
      msw.post(api("/feedback"), async ({ request }) => {
        received = (await request.json()) as Record<string, unknown>
        return HttpResponse.json({ status: "accepted" }, { status: 202 })
      })
    )
    const user = userEvent.setup()
    const { onOpenChange } = renderDialog()

    // Wait for the auth probe to resolve so the reply-to field is
    // populated with the user's email.
    await waitFor(() =>
      expect(screen.getByTestId("feedback-reply-to")).toHaveValue("alex@example.com")
    )

    await user.click(screen.getByTestId("feedback-type-bug"))
    await user.type(
      screen.getByTestId("feedback-message"),
      "Login bounces me back after 2FA verification."
    )
    await user.click(screen.getByTestId("feedback-submit"))

    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(received).not.toBeNull()
    expect(received?.type).toBe("bug")
    expect(received?.message).toBe("Login bounces me back after 2FA verification.")
    expect(received?.reply_to_email).toBe("alex@example.com")
    expect(received?.diagnostics).toMatchObject({ app_version: expect.any(String) })
  })

  it("omits the diagnostics payload when the checkbox is cleared", async () => {
    let received: Record<string, unknown> | null = null
    server.use(
      ...baseUserHandlers,
      msw.post(api("/feedback"), async ({ request }) => {
        received = (await request.json()) as Record<string, unknown>
        return HttpResponse.json({ status: "accepted" }, { status: 202 })
      })
    )
    const user = userEvent.setup()
    renderDialog()

    await waitFor(() => expect(screen.getByTestId("feedback-include-diagnostics")).toBeChecked())
    await user.click(screen.getByTestId("feedback-include-diagnostics"))
    await user.type(screen.getByTestId("feedback-message"), "hello there")
    await user.click(screen.getByTestId("feedback-submit"))

    await waitFor(() => expect(received).not.toBeNull())
    expect(received?.diagnostics).toBeUndefined()
  })

  it("surfaces a validation error when the message is empty", async () => {
    server.use(...baseUserHandlers)
    const user = userEvent.setup()
    const { onOpenChange } = renderDialog()

    await waitFor(() => expect(screen.getByTestId("feedback-message")).toBeInTheDocument())
    await user.click(screen.getByTestId("feedback-submit"))

    expect(await screen.findByTestId("feedback-message-error")).toBeInTheDocument()
    expect(onOpenChange).not.toHaveBeenCalled()
  })

  it("does not close the dialog on a 429 response", async () => {
    server.use(
      ...baseUserHandlers,
      msw.post(api("/feedback"), () =>
        HttpResponse.text("Rate limit exceeded. Please try again later.", {
          status: 429,
          headers: { "Retry-After": "60" },
        })
      )
    )
    const user = userEvent.setup()
    const { onOpenChange } = renderDialog()

    await waitFor(() => expect(screen.getByTestId("feedback-message")).toBeInTheDocument())
    await user.type(screen.getByTestId("feedback-message"), "spammy")
    await user.click(screen.getByTestId("feedback-submit"))

    // Dialog stays open so the user can try again later without
    // retyping the message.
    await waitFor(() => {
      expect(onOpenChange).not.toHaveBeenCalledWith(false)
    })
  })

  it("keeps the dialog open (no /maintenance bounce) on a typed 503 not-configured error", async () => {
    // Regression: on deployments without SUPPORT_EMAIL the BE returns 503.
    // It used to be an untyped text/plain 503, which tripped the global
    // 503 → /maintenance bounce in lib/http.ts and unmounted the whole
    // shell instead of letting the dialog show its toast. The BE now
    // returns a *typed* `feedback.not_configured` code (mirrors the
    // commodity_scan #1720 contract); the dialog must stay put.
    const navigate = vi.fn()
    setNavigateToMaintenance(navigate)
    server.use(
      ...baseUserHandlers,
      msw.post(api("/feedback"), async () => {
        // Small delay so the in-flight (disabled) → settled (enabled)
        // transition on the submit button is observable below.
        await delay(30)
        // Mirror the real jsonapi.Errors payload the BE emits for the
        // errx sentinel (see apiserver/feedback.go): `status` is a text
        // label, `error` is an errormarshal object, and `code` carries
        // the dotted product code lib/http.ts keys off.
        return HttpResponse.json(
          {
            errors: [
              {
                status: "Service Unavailable",
                error: {
                  error: {
                    message: "feedback is not configured on this deployment",
                    sentinels: ["feedback is not configured on this deployment"],
                  },
                  type: "*errx.sentinel",
                },
                code: "feedback.not_configured",
              },
            ],
          },
          { status: 503 }
        )
      })
    )
    const user = userEvent.setup()
    const { onOpenChange } = renderDialog()

    await waitFor(() => expect(screen.getByTestId("feedback-message")).toBeInTheDocument())
    await user.type(screen.getByTestId("feedback-message"), "anyone home?")
    const submit = screen.getByTestId("feedback-submit")
    await user.click(submit)

    // Drive the full submit cycle so the negative assertion isn't vacuous:
    // the button disables while the request is in flight, then re-enables
    // once the 503 has been received AND the global bounce decision (in
    // lib/http.ts) has run. Only then is "navigate not called" meaningful.
    await waitFor(() => expect(submit).toBeDisabled())
    await waitFor(() => expect(submit).not.toBeDisabled())

    // The typed product 503 must NOT bounce the shell to /maintenance…
    expect(navigate).not.toHaveBeenCalled()
    // …and the dialog stays open so the user keeps their typed message.
    expect(screen.getByTestId("feedback-dialog")).toBeInTheDocument()
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
  })
})
