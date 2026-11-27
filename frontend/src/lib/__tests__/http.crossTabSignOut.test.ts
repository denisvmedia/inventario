import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { http as msw, HttpResponse } from "msw"

import { server } from "@/test/server"

// A sign-out in one tab against a refresh in another (#3034). Each tab is its
// own module graph, so it has its own in-flight refresh and sign-out state as
// a browser tab does, while localStorage and the Web Locks stub below are
// shared between them as they are between real tabs. The fake server keeps
// the one cookie jar both tabs send from.

const api = (path: string) => `${window.location.origin}/api/v1${path}`

type Plane = "tenant" | "backoffice"

interface Tab {
  // A request on the plane that needs a valid access token.
  request: () => Promise<unknown>
  signOut: () => Promise<void>
  storedToken: () => string | null
}

async function openTab(plane: Plane): Promise<Tab> {
  vi.resetModules()
  const { http } = await import("@/lib/http")
  const navigation = await import("@/lib/navigation")
  navigation.setNavigateToLogin(() => {})
  navigation.setHardRedirect(() => {})
  if (plane === "tenant") {
    const { logout } = await import("@/features/auth/api")
    const storage = await import("@/lib/auth-storage")
    return {
      request: () => http.get("/groups"),
      signOut: logout,
      storedToken: storage.getAccessToken,
    }
  }
  const { backofficeLogout } = await import("@/features/backoffice/auth/api")
  const storage = await import("@/features/backoffice/auth/storage")
  return {
    request: () => http.get("/backoffice/auth/me"),
    signOut: backofficeLogout,
    storedToken: storage.getBackofficeAccessToken,
  }
}

async function storeToken(plane: Plane, token: string) {
  if (plane === "tenant") {
    const storage = await import("@/lib/auth-storage")
    storage.setAccessToken(token)
  } else {
    const storage = await import("@/features/backoffice/auth/storage")
    storage.setBackofficeAccessToken(token)
  }
}

// navigator.locks as the browser shares it between tabs: one queue per name,
// callbacks run one at a time in request order.
function installSharedWebLocks() {
  const queues = new Map<string, Promise<void>>()
  Object.defineProperty(navigator, "locks", {
    configurable: true,
    value: {
      request<T>(name: string, fn: () => Promise<T>): Promise<T> {
        const result = (queues.get(name) ?? Promise.resolve()).then(fn)
        queues.set(
          name,
          result.then(
            () => undefined,
            () => undefined
          )
        )
        return result
      },
    },
  })
}

function gate() {
  let open!: () => void
  const opened = new Promise<void>((resolve) => {
    open = resolve
  })
  return { open, opened }
}

// The parts of the server the race runs through. The logout puts the access
// token on the blacklist, revokes the refresh-token row the request carried,
// and deletes the cookie in its response. `logoutPause` is where the logout
// waits: between the blacklist and the revoke, or between the revoke and the
// response. A refresh with a revoked row is reuse detection, which on the real
// server revokes every session of the user.
function fakeServer(plane: Plane, logoutPause: "before-revoke" | "after-revoke" | "none") {
  const prefix = plane === "tenant" ? "/auth" : "/backoffice/auth"
  const state = {
    cookieJar: "rt-1" as string | null,
    rows: new Map<string, "active" | "revoked">([["rt-1", "active"]]),
    blacklist: new Set<string>(),
    logoutStarted: false,
    sessionsIssuedAfterLogoutStarted: 0,
    reuseDetected: 0,
  }
  const logoutPaused = gate()
  const logoutResume = gate()
  const refreshStarted = gate()
  const refreshResume = gate()
  let holdRefresh = false
  let rotation = 1
  const bearer = (request: Request) =>
    (request.headers.get("authorization") ?? "").replace(/^Bearer /, "")

  const handlers = [
    msw.post(api(`${prefix}/logout`), async ({ request }) => {
      state.logoutStarted = true
      const cookie = state.cookieJar
      state.blacklist.add(bearer(request))
      if (logoutPause === "before-revoke") {
        logoutPaused.open()
        await logoutResume.opened
      }
      if (cookie) state.rows.set(cookie, "revoked")
      if (logoutPause === "after-revoke") {
        logoutPaused.open()
        await logoutResume.opened
      }
      state.cookieJar = null
      return new HttpResponse(null, { status: 204 })
    }),
    msw.post(api(`${prefix}/refresh`), async () => {
      const cookie = state.cookieJar
      refreshStarted.open()
      if (holdRefresh) await refreshResume.opened
      if (!cookie) return HttpResponse.json(null, { status: 401 })
      if (state.rows.get(cookie) !== "active") {
        state.reuseDetected++
        return HttpResponse.json(null, { status: 401 })
      }
      state.rows.set(cookie, "revoked")
      rotation++
      state.rows.set(`rt-${rotation}`, "active")
      state.cookieJar = `rt-${rotation}`
      if (state.logoutStarted) state.sessionsIssuedAfterLogoutStarted++
      return HttpResponse.json({ access_token: `token-${rotation}`, csrf_token: "csrf" })
    }),
  ]
  const guarded = async ({ request }: { request: Request }) =>
    state.blacklist.has(bearer(request)) || bearer(request) === "token-1-expired"
      ? HttpResponse.json(null, { status: 401 })
      : HttpResponse.json({ ok: true })
  handlers.push(
    plane === "tenant"
      ? msw.get(api("/groups"), guarded)
      : msw.get(api("/backoffice/auth/me"), guarded)
  )
  server.use(...handlers)

  return {
    state,
    logoutPaused: logoutPaused.opened,
    resumeLogout: logoutResume.open,
    refreshStarted: refreshStarted.opened,
    holdRefresh: () => {
      holdRefresh = true
    },
    resumeRefresh: refreshResume.open,
    activeRows: () => [...state.rows.values()].filter((row) => row === "active").length,
  }
}

// Long enough for a request that is not held back to reach the fake server.
const letRequestsLand = () => new Promise((resolve) => setTimeout(resolve, 50))

beforeEach(() => {
  window.localStorage.clear()
  window.sessionStorage.clear()
  installSharedWebLocks()
})

afterEach(() => {
  Reflect.deleteProperty(navigator as object, "locks")
  window.localStorage.clear()
  window.sessionStorage.clear()
})

describe.each<Plane>(["tenant", "backoffice"])("%s sign-out across tabs (#3034)", (plane) => {
  it.each(["before-revoke", "after-revoke"] as const)(
    "a peer's 401 while the logout runs (%s) brings no session back and trips no reuse detection",
    async (logoutPause) => {
      const fake = fakeServer(plane, logoutPause)
      await storeToken(plane, "token-1")
      const tabA = await openTab(plane)
      const tabB = await openTab(plane)

      const signOut = tabA.signOut()
      await fake.logoutPaused
      const peer = tabB.request().then(
        () => "ok",
        (error: { status?: number }) => error.status
      )
      await letRequestsLand()
      fake.resumeLogout()
      await signOut

      expect(await peer).toBe(401)
      expect(fake.state.sessionsIssuedAfterLogoutStarted).toBe(0)
      expect(fake.state.reuseDetected).toBe(0)
      expect(fake.activeRows()).toBe(0)
      expect(tabA.storedToken()).toBeNull()
      expect(tabB.storedToken()).toBeNull()
    }
  )

  it("a sign-out waits for a peer's refresh and revokes the session it rotated to", async () => {
    const fake = fakeServer(plane, "none")
    fake.holdRefresh()
    await storeToken(plane, "token-1-expired")
    const tabA = await openTab(plane)
    const tabB = await openTab(plane)

    const peer = tabB.request()
    await fake.refreshStarted
    const signOut = tabA.signOut()
    await letRequestsLand()
    expect(fake.state.logoutStarted).toBe(false)
    fake.resumeRefresh()
    // Tab B's retry goes out after the logout put its new token on the
    // blacklist, so it may fail; the user signed out.
    await peer.catch(() => undefined)
    await signOut

    expect(fake.state.reuseDetected).toBe(0)
    expect(fake.activeRows()).toBe(0)
    expect(tabA.storedToken()).toBeNull()
  })
})
