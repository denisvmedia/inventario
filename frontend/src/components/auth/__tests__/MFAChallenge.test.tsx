import { describe, expect, it, vi, beforeEach } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { MFAChallenge } from "@/components/auth/MFAChallenge"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

// Lightweight unit coverage for the in-page MFA challenge surface.
// LoginPage.test.tsx exercises the full step-1 → step-2 hand-off; this
// file pins the mode toggle, the cancel button, and the inline error
// path that LoginPage doesn't reach without staging a server failure.

const apiUrl = (path: string) => `${window.location.origin}/api/v1${path}`

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<MFAChallenge />", () => {
  it("toggles between TOTP and backup-code modes and clears the input on switch", async () => {
    const user = userEvent.setup()
    renderWithProviders({
      children: (
        <MFAChallenge
          mfaToken="challenge-jwt"
          email="alex@example.com"
          onSuccess={vi.fn()}
          onCancel={vi.fn()}
        />
      ),
    })

    const input = await screen.findByTestId("mfa-code-input")
    expect(input.getAttribute("data-mode")).toBe("totp")
    await user.type(input, "123456")
    expect((input as HTMLInputElement).value).toBe("123456")

    await user.click(screen.getByTestId("mfa-toggle-mode"))
    const swapped = screen.getByTestId("mfa-code-input")
    expect(swapped.getAttribute("data-mode")).toBe("backup")
    expect((swapped as HTMLInputElement).value).toBe("")

    await user.click(screen.getByTestId("mfa-toggle-mode"))
    expect(screen.getByTestId("mfa-code-input").getAttribute("data-mode")).toBe("totp")
  })

  it("submit is disabled until the user types something", async () => {
    const user = userEvent.setup()
    renderWithProviders({
      children: (
        <MFAChallenge
          mfaToken="challenge-jwt"
          email="alex@example.com"
          onSuccess={vi.fn()}
          onCancel={vi.fn()}
        />
      ),
    })
    const submit = await screen.findByTestId("mfa-submit")
    expect(submit).toBeDisabled()
    await user.type(screen.getByTestId("mfa-code-input"), "999999")
    expect(submit).not.toBeDisabled()
  })

  it("surfaces a server error and keeps the user on the prompt", async () => {
    server.use(
      msw.post(apiUrl("/auth/login/mfa"), () =>
        HttpResponse.json({ error: "Invalid code" }, { status: 401 })
      )
    )
    const onSuccess = vi.fn()
    const user = userEvent.setup()
    renderWithProviders({
      children: (
        <MFAChallenge
          mfaToken="challenge-jwt"
          email="alex@example.com"
          onSuccess={onSuccess}
          onCancel={vi.fn()}
        />
      ),
    })
    await user.type(await screen.findByTestId("mfa-code-input"), "000000")
    await user.click(screen.getByTestId("mfa-submit"))
    await waitFor(() => expect(screen.getByTestId("mfa-server-error")).toBeInTheDocument())
    expect(onSuccess).not.toHaveBeenCalled()
  })

  it("invokes onCancel when the cancel button is clicked", async () => {
    const onCancel = vi.fn()
    const user = userEvent.setup()
    renderWithProviders({
      children: (
        <MFAChallenge
          mfaToken="challenge-jwt"
          email="alex@example.com"
          onSuccess={vi.fn()}
          onCancel={onCancel}
        />
      ),
    })
    await user.click(await screen.findByTestId("mfa-cancel"))
    expect(onCancel).toHaveBeenCalledTimes(1)
  })
})
