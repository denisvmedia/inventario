import { beforeEach, describe, expect, it } from "vitest"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { http as msw, HttpResponse } from "msw"
import { renderHook, waitFor } from "@testing-library/react"

import { server } from "@/test/server"
import { renderWithProviders } from "@/test/render"
import { __resetHttpForTests } from "@/lib/http"

import { BackofficeAuthProvider, useBackofficeAuth } from "../context"
import { useBackofficeLogin } from "../hooks"
import { clearBackofficeAuth, getBackofficeAccessToken, setBackofficeAccessToken } from "../storage"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function makeWrapper() {
  // Per-test QueryClient with retries disabled keeps assertion-on-error
  // tests deterministic — mirrors the test/render.tsx pattern.
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: 0 },
      mutations: { retry: false },
    },
  })
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={client}>
        <BackofficeAuthProvider>{children}</BackofficeAuthProvider>
      </QueryClientProvider>
    )
  }
}

beforeEach(() => {
  clearBackofficeAuth()
  __resetHttpForTests()
})

describe("useBackofficeAuth", () => {
  it("returns isAuthenticated=false with no token (boot probe skipped)", async () => {
    let resolvedHook: ReturnType<typeof useBackofficeAuth> | undefined
    renderWithProviders({
      children: (
        <BackofficeAuthProvider>
          <Probe onValue={(v) => (resolvedHook = v)} />
        </BackofficeAuthProvider>
      ),
    })
    await waitFor(() => expect(resolvedHook?.isInitialized).toBe(true))
    expect(resolvedHook?.isAuthenticated).toBe(false)
    expect(resolvedHook?.user).toBeNull()
  })

  it("resolves an authenticated operator when /backoffice/auth/me returns a profile", async () => {
    setBackofficeAccessToken("op-tok")
    server.use(
      msw.get(api("/backoffice/auth/me"), () =>
        HttpResponse.json({
          id: "op-1",
          email: "operator@example.com",
          name: "Operator",
          role: "platform_admin",
          mfa_enforced: true,
        })
      )
    )
    let resolvedHook: ReturnType<typeof useBackofficeAuth> | undefined
    renderWithProviders({
      children: (
        <BackofficeAuthProvider>
          <Probe onValue={(v) => (resolvedHook = v)} />
        </BackofficeAuthProvider>
      ),
    })
    await waitFor(() => expect(resolvedHook?.isAuthenticated).toBe(true))
    expect(resolvedHook?.user?.email).toBe("operator@example.com")
    expect(resolvedHook?.user?.role).toBe("platform_admin")
  })
})

describe("useBackofficeLogin", () => {
  it("on success: persists the access token and seeds the operator cache", async () => {
    server.use(
      msw.post(api("/backoffice/auth/login"), async ({ request }) => {
        const body = (await request.json()) as { email: string; password: string }
        expect(body).toEqual({ email: "operator@example.com", password: "secret" })
        return HttpResponse.json({
          access_token: "fresh-tok",
          token_type: "Bearer",
          expires_in: 600,
          user: { id: "op-1", email: "operator@example.com", name: "Operator" },
        })
      })
    )
    const { result } = renderHook(() => useBackofficeLogin(), { wrapper: makeWrapper() })
    const outcome = await result.current.mutateAsync({
      email: "operator@example.com",
      password: "secret",
    })
    expect(outcome.kind).toBe("ok")
    expect(getBackofficeAccessToken()).toBe("fresh-tok")
  })

  it("returns mfaRequired without storing tokens when the BE asks for a code", async () => {
    server.use(
      msw.post(api("/backoffice/auth/login"), () =>
        HttpResponse.json({
          mfa_required: true,
          mfa_token: "challenge-jwt",
          expires_in: 300,
          email: "operator@example.com",
        })
      )
    )
    const { result } = renderHook(() => useBackofficeLogin(), { wrapper: makeWrapper() })
    const outcome = await result.current.mutateAsync({
      email: "operator@example.com",
      password: "secret",
    })
    expect(outcome.kind).toBe("mfaRequired")
    if (outcome.kind === "mfaRequired") {
      expect(outcome.mfaToken).toBe("challenge-jwt")
      expect(outcome.email).toBe("operator@example.com")
    }
    // Step-1 must NOT have stored credentials yet — that happens at step-2.
    expect(getBackofficeAccessToken()).toBeNull()
  })

  it("returns mfaNotEnrolled on a 501 with the not_implemented code", async () => {
    server.use(
      msw.post(api("/backoffice/auth/login"), () =>
        HttpResponse.json(
          {
            mfa_required: true,
            code: "backoffice.mfa_not_implemented",
            email: "operator@example.com",
            mfa_token: "",
            expires_in: 0,
          },
          { status: 501 }
        )
      )
    )
    const { result } = renderHook(() => useBackofficeLogin(), { wrapper: makeWrapper() })
    const outcome = await result.current.mutateAsync({
      email: "operator@example.com",
      password: "secret",
    })
    expect(outcome.kind).toBe("mfaNotEnrolled")
    if (outcome.kind === "mfaNotEnrolled") {
      expect(outcome.email).toBe("operator@example.com")
    }
    expect(getBackofficeAccessToken()).toBeNull()
  })
})

// Probe surfaces the hook value to the test scope so we can assert
// without driving every case through findByText.
function Probe({ onValue }: { onValue: (v: ReturnType<typeof useBackofficeAuth>) => void }) {
  const v = useBackofficeAuth()
  onValue(v)
  return null
}
