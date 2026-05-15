import { describe, expect, it, beforeEach } from "vitest"
import { http, HttpResponse } from "msw"

import { server } from "@/test/server"
import { apiUrl } from "@/test/handlers"
import { login, logout } from "@/features/auth/api"
import { clearAuth, getAccessToken, getCsrfToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("login", () => {
  it("persists access + CSRF tokens and returns the user payload", async () => {
    server.use(
      http.post(apiUrl("/auth/login"), () =>
        HttpResponse.json({
          access_token: "tok-1",
          csrf_token: "csrf-1",
          user: { id: "u1", email: "denis@example.com" },
        })
      )
    )
    const outcome = await login("denis@example.com", "secret")
    expect(outcome.kind).toBe("ok")
    if (outcome.kind === "ok") {
      expect(outcome.user).toMatchObject({ id: "u1", email: "denis@example.com" })
    }
    expect(getAccessToken()).toBe("tok-1")
    expect(getCsrfToken()).toBe("csrf-1")
  })

  it("does not throw when the server omits user/tokens (the bare 200 case)", async () => {
    server.use(http.post(apiUrl("/auth/login"), () => HttpResponse.json({})))
    await expect(login("a@b.c", "x")).resolves.toMatchObject({ kind: "ok" })
  })

  // #1645: after the MFA gate landed, /auth/login can short-circuit with
  // mfa_required + an mfa_token instead of access/refresh tokens. login()
  // must surface that without storing any tokens, so the page can hand off
  // to the code-entry surface.
  it("returns an mfa_required outcome when the backend asks for a code", async () => {
    server.use(
      http.post(apiUrl("/auth/login"), () =>
        HttpResponse.json({
          mfa_required: true,
          mfa_token: "challenge-jwt",
          expires_in: 300,
          email: "denis@example.com",
        })
      )
    )
    const outcome = await login("denis@example.com", "secret")
    expect(outcome.kind).toBe("mfa_required")
    if (outcome.kind === "mfa_required") {
      expect(outcome.mfaToken).toBe("challenge-jwt")
      expect(outcome.email).toBe("denis@example.com")
    }
    expect(getAccessToken()).toBeNull()
    expect(getCsrfToken()).toBeNull()
  })
})

describe("logout", () => {
  it("calls /auth/logout and wipes local credentials on success", async () => {
    let logoutCalls = 0
    server.use(
      http.post(apiUrl("/auth/logout"), () => {
        logoutCalls++
        return new HttpResponse(null, { status: 204 })
      })
    )
    await logout()
    expect(logoutCalls).toBe(1)
    expect(getAccessToken()).toBeNull()
  })

  it("wipes local credentials even when the server errors", async () => {
    server.use(
      http.post(apiUrl("/auth/logout"), () => HttpResponse.json({ error: "boom" }, { status: 500 }))
    )
    await expect(logout()).rejects.toBeDefined()
    expect(getAccessToken()).toBeNull()
  })
})
