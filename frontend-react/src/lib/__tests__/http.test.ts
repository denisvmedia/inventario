import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { http, HttpError, __resetHttpForTests } from "@/lib/http"
import {
  clearAuth,
  getAccessToken,
  getCsrfToken,
  setAccessToken,
  setCsrfToken,
} from "@/lib/auth-storage"
import {
  __resetGroupContextForTests,
  setCurrentGroupSlug,
} from "@/lib/group-context"
import { __resetNavigationForTests, setNavigateToLogin } from "@/lib/navigation"
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
    "/files?category=photos",
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
      }),
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
      }),
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
        }),
      )
      await http.get(path)
      expect(captured).toBe(`/api/v1${path}`)
    },
  )

  it("encodes the slug to keep reserved characters safe", async () => {
    setCurrentGroupSlug("a/b c")
    let captured: string | null = null
    server.use(
      msw.get(/.*/, ({ request }) => {
        captured = new URL(request.url).pathname
        return HttpResponse.json({ ok: true })
      }),
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
      }),
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
      }),
    )
    await http.request("/commodities", { skipGroupRewrite: true })
    expect(captured).toBe("/api/v1/commodities")
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
      }),
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
      }),
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
      }),
    )
    await http.get("/auth/me")
    expect(csrf).toBeNull()
  })

  it("picks up X-CSRF-Token from response headers and persists it", async () => {
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: 1 }, { headers: { "X-CSRF-Token": "rotated-csrf" } }),
      ),
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
      }),
    )
    await http.get("/auth/me", { authCheck: "user-initiated" })
    expect(header).toBe("user-initiated")
  })
})

describe("error handling", () => {
  it("throws HttpError with status and parsed JSON body on 4xx", async () => {
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ error: "nope" }, { status: 403 }),
      ),
    )
    await expect(http.get("/auth/me")).rejects.toMatchObject({
      name: "HttpError",
      status: 403,
      data: { error: "nope" },
    })
  })

  it("throws HttpError on 5xx (no swallowing — see #1210)", async () => {
    server.use(
      msw.get(api("/groups"), () =>
        HttpResponse.json({ error: "boom" }, { status: 503 }),
      ),
    )
    await expect(http.get("/groups")).rejects.toBeInstanceOf(HttpError)
  })

  it("forwards AbortSignal to fetch", async () => {
    server.use(
      msw.get(api("/groups"), async () => {
        await new Promise((r) => setTimeout(r, 100))
        return HttpResponse.json({ ok: true })
      }),
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
        HttpResponse.json({ error: "bad creds" }, { status: 401 }),
      ),
      msw.post(api("/auth/refresh"), () => {
        refresh()
        return HttpResponse.json({ access_token: "t" })
      }),
    )
    await expect(
      http.post("/auth/login", { email: "x", password: "y" }),
    ).rejects.toMatchObject({ status: 401 })
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
        HttpResponse.json({ access_token: "fresh", csrf_token: "fresh-csrf" }),
      ),
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
        HttpResponse.json({ error: "expired" }, { status: 401 }),
      ),
      msw.post(api("/auth/refresh"), () =>
        HttpResponse.json({ error: "refresh-bad" }, { status: 401 }),
      ),
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
    server.use(
      msw.get(api("/auth/me"), () => HttpResponse.json(null, { status: 401 })),
    )
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
      }),
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
