import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"

import { VerifyEmailPage } from "@/pages/auth/VerifyEmailPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function renderVerify(path: string) {
  return renderWithProviders({
    initialPath: path,
    routes: (
      <Route
        path="/verify-email"
        element={
          <AuthProvider>
            <VerifyEmailPage />
          </AuthProvider>
        }
      />
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<VerifyEmailPage />", () => {
  it("shows the missing-token state when ?token is absent", () => {
    renderVerify("/verify-email")
    expect(screen.getByTestId("verify-missing")).toBeInTheDocument()
  })

  it("hits /verify-email and renders success on 200", async () => {
    server.use(
      msw.get(api("/verify-email"), ({ request }) => {
        expect(new URL(request.url).searchParams.get("token")).toBe("tok-1")
        return HttpResponse.json({ message: "Email verified!" })
      })
    )
    renderVerify("/verify-email?token=tok-1")
    await waitFor(() => expect(screen.getByTestId("verify-success")).toBeInTheDocument())
    expect(screen.getByTestId("verify-success")).toHaveTextContent(/email verified/i)
  })

  it("renders the expired state when the server says the link expired", async () => {
    server.use(
      msw.get(api("/verify-email"), () =>
        HttpResponse.json("verification link has expired", { status: 410 })
      )
    )
    renderVerify("/verify-email?token=tok-1")
    await waitFor(() => expect(screen.getByTestId("verify-expired")).toBeInTheDocument())
  })

  it("renders the invalid state for any other failure", async () => {
    server.use(
      msw.get(api("/verify-email"), () =>
        HttpResponse.json({ error: "bad token" }, { status: 400 })
      )
    )
    renderVerify("/verify-email?token=tok-1")
    await waitFor(() => expect(screen.getByTestId("verify-invalid")).toBeInTheDocument())
  })
})

// #2096: the backend's unknown-token message used to contain the word
// "expired", so every bad token rendered "Link expired". The two cases must
// not collide.
describe("VerifyEmailPage — error classification", () => {
  it("renders 'invalid' for an unknown token", async () => {
    server.use(
      msw.get(api("/verify-email"), () =>
        HttpResponse.json("Invalid verification token", { status: 400 })
      )
    )
    renderVerify("/verify-email?token=nope")

    await waitFor(() => expect(screen.getByTestId("verify-invalid")).toBeInTheDocument())
  })

  it("renders 'expired' for the backend's expiry message", async () => {
    server.use(
      msw.get(api("/verify-email"), () =>
        HttpResponse.json("Verification token expired. Please request a new one.", { status: 400 })
      )
    )
    renderVerify("/verify-email?token=old")

    await waitFor(() => expect(screen.getByTestId("verify-expired")).toBeInTheDocument())
  })
})
