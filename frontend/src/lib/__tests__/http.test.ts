import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { http, HttpError, __resetHttpForTests } from "@/lib/http"
import {
  clearAuth,
  getAccessToken,
  getCsrfToken,
  setAccessToken,
  setCsrfToken,
} from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import {
  __resetNavigationForTests,
  setNavigateToLogin,
  setNavigateToMaintenance,
} from "@/lib/navigation"
import { server } from "@/test/server"
import { http as msw, HttpResponse } from "msw"

// MSW node interceptors match against the absolute URL the browser would
// build from the relative path the wrapper produces — i.e. `${origin}/api/v1...`.
// jsdom defaults the origin to http://localhost:3000.
const api = (path: string) => `${window.location.origin}/api/v1${path}`

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetNavigationForTests()
  __resetHttpForTests()
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe("group-scoped URL rewriting", () => {
  it.each([
    "/locations",
    "/locations/abc-123",
    "/areas",
    "/commodities",
    "/commodities/values",
    "/commodities/abc/files",
    "/files",
    "/files?category=images",
    "/exports",
    "/tags",
    "/upload-slots",
    "/uploads",
    "/settings",
    "/search",
  ])("rewrites %s when a group is active", async (path) => {
    setCurrentGroupSlug("household")
    let captured: string | null = null
    server.use(
      msw.get(/.*/, ({ request }) => {
        captured = new URL(request.url).pathname + new URL(request.url).search
        return HttpResponse.json({ ok: true })
      })
    )
    await http.get(path)
    expect(captured).not.toBeNull()
    expect(captured!.startsWith("/api/v1/g/household/")).toBe(true)
  })

  it("does NOT rewrite /auth/login even when a group is active", async () => {
    setCurrentGroupSlug("household")
    let captured: string | null = null
    server.use(
      msw.post(api("/auth/login"), ({ request }) => {
        captured = new URL(request.url).pathname
        return HttpResponse.json({ access_token: "t" })
      })
    )
    await http.post("/auth/login", { email: "x", password: "y" })
    expect(captured).toBe("/api/v1/auth/login")
  })

  it.each(["/auth/me", "/groups", "/profile", "/registration"])(
    "does not rewrite non-group path %s",
    async (path) => {
      setCurrentGroupSlug("household")
      let captured: string | null = null
      server.use(
        msw.get(/.*/, ({ request }) => {
          captured = new URL(request.url).pathname
          return HttpResponse.json({ ok: true })
        })
      )
      await http.get(path)
      expect(captured).toBe(`/api/v1${path}`)
    }
  )

  it("encodes the slug to keep reserved characters safe", async () => {
    setCurrentGroupSlug("a/b c")
    let captured: string | null = null
    server.use(
      msw.get(/.*/, ({ request }) => {
        captured = new URL(request.url).pathname
        return HttpResponse.json({ ok: true })
      })
    )
    await http.get("/files")
    expect(captured).toBe("/api/v1/g/a%2Fb%20c/files")
  })

  it("does not rewrite when no group is set, even for group-scoped prefixes", async () => {
    let captured: string | null = null
    server.use(
      msw.get(/.*/, ({ request }) => {
        captured = new URL(request.url).pathname
        return HttpResponse.json({ ok: true })
      })
    )
    await http.get("/commodities")
    expect(captured).toBe("/api/v1/commodities")
  })

  it("honors skipGroupRewrite", async () => {
    setCurrentGroupSlug("household")
    let captured: string | null = null
    server.use(
      msw.get(/.*/, ({ request }) => {
        captured = new URL(request.url).pathname
        return HttpResponse.json({ ok: true })
      })
    )
    await http.request("/commodities", { skipGroupRewrite: true })
    expect(captured).toBe("/api/v1/commodities")
  })

  it("URL slug wins over a stale slot when path is /g/:slug/*", async () => {
    // Models the URL-shape-reconciliation race: the slot still holds the
    // old slug from the previous mirror effect, but window.location has
    // already replaced to the corrected slug. The http client must read
    // the URL, not the stale slot — otherwise the request 404s against
    // the wrong group (see GroupContext.tsx URL-shape reconciliation).
    const original = window.location
    Object.defineProperty(window, "location", {
      configurable: true,
      writable: true,
      value: { ...original, pathname: "/g/right-slug/dashboard" },
    })
    try {
      setCurrentGroupSlug("stale-slug")
      let captured: string | null = null
      server.use(
        msw.get(/.*/, ({ request }) => {
          captured = new URL(request.url).pathname
          return HttpResponse.json({ ok: true })
        })
      )
      await http.get("/commodities")
      expect(captured).toBe("/api/v1/g/right-slug/commodities")
    } finally {
      Object.defineProperty(window, "location", {
        configurable: true,
        writable: true,
        value: original,
      })
    }
  })

  it("reads ?g=<slug> from window.location.search on path-clean routes (#1679)", async () => {
    // Models the /profile auto-fill flow: BrowserRouter's setSearchParams
    // calls history.replaceState which updates window.location.search
    // synchronously, but the GroupProvider's slug-mirror useEffect lands
    // one tick later. The first group-scoped fetch fired in the same
    // commit (e.g. useDashboardData's `enabled` flipping true once
    // currentGroup resolves) must rewrite from the URL — otherwise it
    // 404s against `/commodities` and the profile snapshot stays as "—"
    // forever (#1679).
    const original = window.location
    Object.defineProperty(window, "location", {
      configurable: true,
      writable: true,
      value: { ...original, pathname: "/profile", search: "?g=household" },
    })
    try {
      // Slot deliberately null — simulates the pre-effect state.
      let captured: string | null = null
      server.use(
        msw.get(/.*/, ({ request }) => {
          captured = new URL(request.url).pathname
          return HttpResponse.json({ ok: true })
        })
      )
      await http.get("/commodities")
      expect(captured).toBe("/api/v1/g/household/commodities")
    } finally {
      Object.defineProperty(window, "location", {
        configurable: true,
        writable: true,
        value: original,
      })
    }
  })

  it("path /g/:slug/* still wins over ?g=<other> when both are present", async () => {
    // Sanity check: if the URL carries BOTH a path slug and a query
    // slug (e.g. a malformed redirect or hand-typed URL), the path is
    // authoritative — same precedence as before #1679.
    const original = window.location
    Object.defineProperty(window, "location", {
      configurable: true,
      writable: true,
      value: { ...original, pathname: "/g/canonical/dashboard", search: "?g=stale" },
    })
    try {
      let captured: string | null = null
      server.use(
        msw.get(/.*/, ({ request }) => {
          captured = new URL(request.url).pathname
          return HttpResponse.json({ ok: true })
        })
      )
      await http.get("/commodities")
      expect(captured).toBe("/api/v1/g/canonical/commodities")
    } finally {
      Object.defineProperty(window, "location", {
        configurable: true,
        writable: true,
        value: original,
      })
    }
  })
})

describe("auth + CSRF headers", () => {
  it("sends Authorization bearer when token is present", async () => {
    setAccessToken("token-abc")
    let auth: string | null = null
    server.use(
      msw.get(api("/auth/me"), ({ request }) => {
        auth = request.headers.get("authorization")
        return HttpResponse.json({ id: 1 })
      })
    )
    await http.get("/auth/me")
    expect(auth).toBe("Bearer token-abc")
  })

  it("attaches X-CSRF-Token to mutating requests", async () => {
    setCsrfToken("csrf-abc")
    let csrf: string | null = null
    server.use(
      msw.post(api("/auth/login"), ({ request }) => {
        csrf = request.headers.get("x-csrf-token")
        return HttpResponse.json({ access_token: "t" })
      })
    )
    await http.post("/auth/login", { email: "x", password: "y" })
    expect(csrf).toBe("csrf-abc")
  })

  it("does not attach X-CSRF-Token to GET", async () => {
    setCsrfToken("csrf-abc")
    let csrf: string | null = null
    server.use(
      msw.get(api("/auth/me"), ({ request }) => {
        csrf = request.headers.get("x-csrf-token")
        return HttpResponse.json({ id: 1 })
      })
    )
    await http.get("/auth/me")
    expect(csrf).toBeNull()
  })

  it("picks up X-CSRF-Token from response headers and persists it", async () => {
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: 1 }, { headers: { "X-CSRF-Token": "rotated-csrf" } })
      )
    )
    await http.get("/auth/me")
    expect(getCsrfToken()).toBe("rotated-csrf")
  })

  it("sends X-Auth-Check: user-initiated when requested", async () => {
    let header: string | null = null
    server.use(
      msw.get(api("/auth/me"), ({ request }) => {
        header = request.headers.get("x-auth-check")
        return HttpResponse.json({ id: 1 })
      })
    )
    await http.get("/auth/me", { authCheck: "user-initiated" })
    expect(header).toBe("user-initiated")
  })
})

describe("error handling", () => {
  it("throws HttpError with status and parsed JSON body on 4xx", async () => {
    server.use(
      msw.get(api("/auth/me"), () => HttpResponse.json({ error: "nope" }, { status: 403 }))
    )
    await expect(http.get("/auth/me")).rejects.toMatchObject({
      name: "HttpError",
      status: 403,
      data: { error: "nope" },
    })
  })

  it("throws HttpError on 5xx (no swallowing — see #1210)", async () => {
    server.use(msw.get(api("/groups"), () => HttpResponse.json({ error: "boom" }, { status: 503 })))
    await expect(http.get("/groups")).rejects.toBeInstanceOf(HttpError)
  })

  it("503 bounces through navigateToMaintenance with Retry-After + X-Maintenance-Status headers (#1542)", async () => {
    server.use(
      msw.get(api("/groups"), () =>
        HttpResponse.json(null, {
          status: 503,
          headers: {
            "Retry-After": "900",
            "X-Maintenance-Status": "api=degraded,database=maintenance,storage=operational",
          },
        })
      )
    )
    const navigate = vi.fn()
    setNavigateToMaintenance(navigate)
    await expect(http.get("/groups")).rejects.toBeInstanceOf(HttpError)
    expect(navigate).toHaveBeenCalledOnce()
    expect(navigate).toHaveBeenCalledWith({
      retryAfter: "900",
      componentStatus: "api=degraded,database=maintenance,storage=operational",
    })
  })

  it("503 does NOT re-bounce when the user is already on /maintenance (#1542 — avoids reload loop)", async () => {
    server.use(msw.get(api("/groups"), () => HttpResponse.json(null, { status: 503 })))
    const navigate = vi.fn()
    setNavigateToMaintenance(navigate)
    const originalLocation = window.location
    // jsdom's `location` is a getter — override just the pathname for the
    // duration of this test so the early-return in the http client fires.
    Object.defineProperty(window, "location", {
      configurable: true,
      value: { ...originalLocation, pathname: "/maintenance" },
    })
    try {
      await expect(http.get("/groups")).rejects.toBeInstanceOf(HttpError)
      expect(navigate).not.toHaveBeenCalled()
    } finally {
      Object.defineProperty(window, "location", {
        configurable: true,
        value: originalLocation,
      })
    }
  })

  it("forwards AbortSignal to fetch", async () => {
    server.use(
      msw.get(api("/groups"), async () => {
        await new Promise((r) => setTimeout(r, 100))
        return HttpResponse.json({ ok: true })
      })
    )
    const ctl = new AbortController()
    const promise = http.get("/groups", { signal: ctl.signal })
    ctl.abort()
    await expect(promise).rejects.toThrow()
  })
})

describe("401 flow", () => {
  it("on /auth/login: does NOT call refresh, does NOT navigate, surfaces 401", async () => {
    const refresh = vi.fn()
    const navigate = vi.fn()
    setNavigateToLogin(navigate)
    server.use(
      msw.post(api("/auth/login"), () =>
        HttpResponse.json({ error: "bad creds" }, { status: 401 })
      ),
      msw.post(api("/auth/refresh"), () => {
        refresh()
        return HttpResponse.json({ access_token: "t" })
      })
    )
    await expect(http.post("/auth/login", { email: "x", password: "y" })).rejects.toMatchObject({
      status: 401,
    })
    expect(refresh).not.toHaveBeenCalled()
    expect(navigate).not.toHaveBeenCalled()
  })

  it("on a normal request: refreshes, retries, returns success", async () => {
    setAccessToken("expired")
    setCurrentGroupSlug("household")
    let attempt = 0
    server.use(
      msw.get(api("/g/household/commodities"), ({ request }) => {
        attempt++
        if (attempt === 1) {
          return HttpResponse.json({ error: "expired" }, { status: 401 })
        }
        return HttpResponse.json({ data: [], auth: request.headers.get("authorization") })
      }),
      msw.post(api("/auth/refresh"), () =>
        HttpResponse.json({ access_token: "fresh", csrf_token: "fresh-csrf" })
      )
    )
    const result = await http.get<{ data: unknown[]; auth: string }>("/commodities")
    expect(attempt).toBe(2)
    expect(getAccessToken()).toBe("fresh")
    expect(getCsrfToken()).toBe("fresh-csrf")
    expect(result.auth).toBe("Bearer fresh")
  })

  it("on refresh failure: clears auth and calls navigateToLogin", async () => {
    setAccessToken("expired")
    setCsrfToken("old-csrf")
    setCurrentGroupSlug("household")
    const navigate = vi.fn()
    setNavigateToLogin(navigate)
    server.use(
      msw.get(api("/g/household/commodities"), () =>
        HttpResponse.json({ error: "expired" }, { status: 401 })
      ),
      msw.post(api("/auth/refresh"), () =>
        HttpResponse.json({ error: "refresh-bad" }, { status: 401 })
      )
    )
    await expect(http.get("/commodities")).rejects.toBeInstanceOf(HttpError)
    expect(getAccessToken()).toBeNull()
    expect(getCsrfToken()).toBeNull()
    expect(navigate).toHaveBeenCalledOnce()
    expect(navigate.mock.calls[0][1]).toBe("session_expired")
  })

  it("background /auth/me 401: does NOT clear auth or navigate", async () => {
    setAccessToken("possibly-still-good")
    const navigate = vi.fn()
    setNavigateToLogin(navigate)
    server.use(msw.get(api("/auth/me"), () => HttpResponse.json(null, { status: 401 })))
    await expect(http.get("/auth/me", { authCheck: "background" })).rejects.toMatchObject({
      status: 401,
    })
    expect(getAccessToken()).toBe("possibly-still-good")
    expect(navigate).not.toHaveBeenCalled()
  })

  it("single-flight refresh: concurrent 401s share one /auth/refresh call", async () => {
    setAccessToken("expired")
    setCurrentGroupSlug("household")
    let refreshCalls = 0
    let firstAttempts = 0
    let secondAttempts = 0
    server.use(
      msw.get(api("/g/household/commodities"), () => {
        firstAttempts++
        if (firstAttempts === 1) return HttpResponse.json(null, { status: 401 })
        return HttpResponse.json({ ok: "first" })
      }),
      msw.get(api("/g/household/locations"), () => {
        secondAttempts++
        if (secondAttempts === 1) return HttpResponse.json(null, { status: 401 })
        return HttpResponse.json({ ok: "second" })
      }),
      msw.post(api("/auth/refresh"), async () => {
        refreshCalls++
        await new Promise((r) => setTimeout(r, 20))
        return HttpResponse.json({ access_token: "fresh" })
      })
    )
    const [r1, r2] = await Promise.all([
      http.get<{ ok: string }>("/commodities"),
      http.get<{ ok: string }>("/locations"),
    ])
    expect(refreshCalls).toBe(1)
    expect(r1.ok).toBe("first")
    expect(r2.ok).toBe("second")
  })
})
