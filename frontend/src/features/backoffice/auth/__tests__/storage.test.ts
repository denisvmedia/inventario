import { beforeEach, describe, expect, it } from "vitest"

import {
  clearBackofficeAccessToken,
  clearBackofficeAuth,
  clearBackofficeCsrfToken,
  getBackofficeAccessToken,
  getBackofficeCsrfToken,
  setBackofficeAccessToken,
  setBackofficeCsrfToken,
} from "../storage"

beforeEach(() => {
  window.localStorage.clear()
  window.sessionStorage.clear()
  clearBackofficeAuth()
})

describe("back-office auth storage", () => {
  it("persists the access token to localStorage under a stable key", () => {
    setBackofficeAccessToken("op-tok")
    expect(getBackofficeAccessToken()).toBe("op-tok")
    expect(window.localStorage.getItem("backoffice_access_token")).toBe("op-tok")
  })

  it("does NOT cross-contaminate with the tenant access token key", () => {
    window.localStorage.setItem("inventario_token", "tenant-tok")
    setBackofficeAccessToken("op-tok")
    expect(window.localStorage.getItem("inventario_token")).toBe("tenant-tok")
    expect(window.localStorage.getItem("backoffice_access_token")).toBe("op-tok")
  })

  it("persists the CSRF token to sessionStorage and serves it from memory after", () => {
    setBackofficeCsrfToken("csrf-1")
    expect(getBackofficeCsrfToken()).toBe("csrf-1")
    // Clearing sessionStorage shouldn't flip the in-memory cache mid-tab —
    // mirrors the tenant CSRF behaviour: a writer rotates both at once.
    window.sessionStorage.clear()
    expect(getBackofficeCsrfToken()).toBe("csrf-1")
  })

  it("clearBackofficeAuth wipes both the token and the CSRF in storage and memory", () => {
    setBackofficeAccessToken("op-tok")
    setBackofficeCsrfToken("csrf-1")
    clearBackofficeAuth()
    expect(getBackofficeAccessToken()).toBeNull()
    expect(getBackofficeCsrfToken()).toBeNull()
    expect(window.localStorage.getItem("backoffice_access_token")).toBeNull()
    expect(window.sessionStorage.getItem("backoffice_csrf_token")).toBeNull()
  })

  it("clearBackofficeAccessToken and clearBackofficeCsrfToken are individually addressable", () => {
    setBackofficeAccessToken("op-tok")
    setBackofficeCsrfToken("csrf-1")
    clearBackofficeAccessToken()
    expect(getBackofficeAccessToken()).toBeNull()
    expect(getBackofficeCsrfToken()).toBe("csrf-1")
    clearBackofficeCsrfToken()
    expect(getBackofficeCsrfToken()).toBeNull()
  })
})
