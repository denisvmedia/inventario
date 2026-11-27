import "fake-indexeddb/auto"
import { Blob as NodeBlob, File as NodeFile } from "node:buffer"
import { describe, expect, it, beforeEach, onTestFinished, vi } from "vitest"
import { http, HttpResponse } from "msw"

import { server } from "@/test/server"
import { apiUrl } from "@/test/handlers"
import { login, logout } from "@/features/auth/api"
import { clearAuth, getAccessToken, getCsrfToken } from "@/lib/auth-storage"
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
})
