import "fake-indexeddb/auto"
import { Blob as NodeBlob, File as NodeFile } from "node:buffer"
import { describe, expect, it, beforeEach, onTestFinished, vi } from "vitest"
import { http, HttpResponse } from "msw"

import { server } from "@/test/server"
import { apiUrl } from "@/test/handlers"
import { getCurrentUser, login, logout, register } from "@/features/auth/api"
import { i18next } from "@/i18n"
import { getRecent, pushRecent, recentScope } from "@/features/search/recent"
import { clearAuth, getAccessToken, getCsrfToken, setAccessToken } from "@/lib/auth-storage"
import { loadPendingFiles, savePendingFiles } from "@/lib/pending-files-store"
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

describe("register", () => {
  // The server stores this as the new account's language setting, so the
  // verification and welcome emails go out in it (#3069).
  it("sends the UI language the user registers in", async () => {
    await i18next.changeLanguage("cs")
    onTestFinished(async () => {
      await i18next.changeLanguage("en")
    })
    let acceptLanguage: string | null = null
    server.use(
      http.post(apiUrl("/register"), ({ request }) => {
        acceptLanguage = request.headers.get("Accept-Language")
        return HttpResponse.json({ message: "ok" })
      })
    )

    await register({ email: "new@example.com", password: "Password123", name: "New" })

    expect(acceptLanguage).toBe("cs")
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

  // #3026: the logout blacklists the access token before it revokes the
  // refresh token. /auth/me sent with the old token in between got a 401,
  // the client refreshed, and the user was signed in again.
  it("does not refresh the session back when a request gets a 401 during the logout", async () => {
    setAccessToken("old")
    let refreshCalls = 0
    let meAnswered!: () => void
    const meDone = new Promise<void>((resolve) => {
      meAnswered = resolve
    })
    server.use(
      http.post(apiUrl("/auth/logout"), async () => {
        await meDone
        return new HttpResponse(null, { status: 204 })
      }),
      http.get(apiUrl("/auth/me"), () => {
        meAnswered()
        return HttpResponse.json({ error: "revoked" }, { status: 401 })
      }),
      http.post(apiUrl("/auth/refresh"), () => {
        refreshCalls++
        return HttpResponse.json({ access_token: "new", csrf_token: "new-csrf" })
      })
    )

    const signingOut = logout()
    await expect(getCurrentUser()).rejects.toMatchObject({ status: 401 })
    await signingOut

    expect(refreshCalls).toBe(0)
    expect(getAccessToken()).toBeNull()
    expect(getCsrfToken()).toBeNull()
  })

  it("wipes local credentials even when the server errors", async () => {
    server.use(
      http.post(apiUrl("/auth/logout"), () => HttpResponse.json({ error: "boom" }, { status: 500 }))
    )
    await expect(logout()).rejects.toBeDefined()
    expect(getAccessToken()).toBeNull()
  })

  // #2920: what the item form kept for a signed-in user must not wait in the
  // browser for the next person. The visitor's form (#1988) waits for the
  // sign-in that adds its item.
  it("removes the item form's recovery, and keeps the visitor's", async () => {
    // jsdom's File does not survive fake-indexeddb; Node's does, as a
    // browser's does with IndexedDB.
    vi.stubGlobal("File", NodeFile)
    vi.stubGlobal("Blob", NodeBlob)
    onTestFinished(() => {
      vi.unstubAllGlobals()
    })
    server.use(http.post(apiUrl("/auth/logout"), () => new HttpResponse(null, { status: 204 })))
    const slot = "commodity-draft:u1:household:create"
    const visitor = "commodity-draft:anon:create"
    const staged = [{ id: "p1", file: new File(["x"], "receipt.png"), tags: [] }]
    window.localStorage.setItem(slot, JSON.stringify({ name: "Couch" }))
    window.localStorage.setItem(`${slot}:base-revision`, "2")
    window.sessionStorage.setItem(`${slot}:client-key`, "form:1")
    window.localStorage.setItem(visitor, JSON.stringify({ name: "Lamp" }))
    window.localStorage.setItem("inventario_theme", "dark")
    await savePendingFiles(slot, staged)
    await savePendingFiles(visitor, staged)

    await logout()

    expect(window.localStorage.getItem(slot)).toBeNull()
    expect(window.localStorage.getItem(`${slot}:base-revision`)).toBeNull()
    expect(window.sessionStorage.getItem(`${slot}:client-key`)).toBeNull()
    expect(await loadPendingFiles(slot)).toEqual([])
    expect(window.localStorage.getItem(visitor)).not.toBeNull()
    expect(await loadPendingFiles(visitor)).toHaveLength(1)
    expect(window.localStorage.getItem("inventario_theme")).toBe("dark")
  })

  // #3023: the next person to sign in on this browser does not see what the
  // last one opened from search.
  it("removes the recent search results of every user", async () => {
    server.use(http.post(apiUrl("/auth/logout"), () => new HttpResponse(null, { status: 204 })))
    const entry = {
      type: "commodity",
      id: "c1",
      title: "Drill",
      url: "/g/household/commodities/c1",
    } as const
    pushRecent(recentScope("u1", "household"), entry)
    pushRecent(recentScope("u2", "office"), entry)
    window.localStorage.setItem("inventario_theme", "dark")

    await logout()

    expect(getRecent(recentScope("u1", "household"))).toEqual([])
    expect(getRecent(recentScope("u2", "office"))).toEqual([])
    expect(window.localStorage.getItem("inventario_theme")).toBe("dark")
  })
})
