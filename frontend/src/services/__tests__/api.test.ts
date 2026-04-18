import { describe, it, expect, beforeEach, afterEach, afterAll, vi } from 'vitest'
import api, { setCsrfToken, getCsrfToken, clearCsrfToken } from '../api'

// The key used by api.ts to persist the CSRF token in sessionStorage.
const CSRF_SESSION_KEY = 'inventario_csrf_token'
// The key used by api.ts to persist the currently-selected group slug.
const GROUP_SLUG_KEY = 'currentGroupSlug'

describe('CSRF token management', () => {
  beforeEach(() => {
    // Reset both the in-memory variable and sessionStorage between tests.
    // clearCsrfToken() is the only way to zero-out the module-level variable
    // from outside the module, which is exactly how the production code clears
    // it on logout.
    clearCsrfToken()
  })

  describe('setCsrfToken()', () => {
    it('writes the token to sessionStorage', () => {
      setCsrfToken('token-abc')

      expect(sessionStorage.getItem(CSRF_SESSION_KEY)).toBe('token-abc')
    })

    it('makes the token immediately available via getCsrfToken()', () => {
      setCsrfToken('token-abc')

      expect(getCsrfToken()).toBe('token-abc')
    })

    it('overwrites a previously stored token', () => {
      setCsrfToken('old-token')
      setCsrfToken('new-token')

      expect(sessionStorage.getItem(CSRF_SESSION_KEY)).toBe('new-token')
      expect(getCsrfToken()).toBe('new-token')
    })
  })

  describe('getCsrfToken()', () => {
    it('returns null when no token has been set', () => {
      expect(getCsrfToken()).toBeNull()
    })

    it('restores the token from sessionStorage after a simulated page reload', () => {
      // Simulate what the browser does across a page reload:
      //   - sessionStorage survives (written here directly, bypassing setCsrfToken)
      //   - the module-level in-memory variable starts as null (cleared by beforeEach)
      sessionStorage.setItem(CSRF_SESSION_KEY, 'persisted-token')

      // getCsrfToken() must fall back to sessionStorage and return the value.
      expect(getCsrfToken()).toBe('persisted-token')
    })

    it('caches the sessionStorage value in memory on first read', () => {
      sessionStorage.setItem(CSRF_SESSION_KEY, 'cached-token')

      // First call reads from sessionStorage and caches in memory.
      expect(getCsrfToken()).toBe('cached-token')

      // Removing from sessionStorage after the first read must not affect
      // subsequent in-memory reads (token is already cached).
      sessionStorage.removeItem(CSRF_SESSION_KEY)
      expect(getCsrfToken()).toBe('cached-token')
    })
  })

  describe('clearCsrfToken()', () => {
    it('removes the token from sessionStorage', () => {
      setCsrfToken('token-to-clear')
      clearCsrfToken()

      expect(sessionStorage.getItem(CSRF_SESSION_KEY)).toBeNull()
    })

    it('makes getCsrfToken() return null after clearing', () => {
      setCsrfToken('token-to-clear')
      clearCsrfToken()

      expect(getCsrfToken()).toBeNull()
    })

    it('is idempotent — calling it twice does not throw', () => {
      setCsrfToken('token')
      clearCsrfToken()

      expect(() => clearCsrfToken()).not.toThrow()
      expect(getCsrfToken()).toBeNull()
      expect(sessionStorage.getItem(CSRF_SESSION_KEY)).toBeNull()
    })
  })
})

// -----------------------------------------------------------------------
// Group-scoped URL rewrite in the axios request interceptor.
//
// The interceptor reads currentGroupSlug from localStorage and rewrites
// /api/v1/<prefix>/... to /api/v1/g/{slug}/<prefix>/... for data endpoints.
// We drive this by replacing the axios adapter with a vi.fn(); the adapter
// is the last stop before the HTTP call and receives the post-interceptor
// config, so its captured `url` reflects exactly what the interceptor
// produced. We restore the original adapter after each test.
// -----------------------------------------------------------------------
describe('group-scoped URL rewrite interceptor', () => {
  const originalAdapter = api.defaults.adapter
  let captured: string | undefined

  const fakeAdapter = vi.fn((config: { url?: string }) => {
    captured = config.url
    return Promise.resolve({
      data: { ok: true },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    })
  })

  beforeEach(() => {
    captured = undefined
    fakeAdapter.mockClear()
    localStorage.removeItem(GROUP_SLUG_KEY)
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    api.defaults.adapter = fakeAdapter as any
  })

  // Restore the real adapter after each test so a thrown assertion, a
  // filtered run, or a reorder can't leak the fake into unrelated tests.
  afterEach(() => {
    api.defaults.adapter = originalAdapter
    localStorage.removeItem(GROUP_SLUG_KEY)
  })

  // Defense in depth: if something in the module scope ever left the fake
  // adapter set (e.g. beforeEach throws before afterEach exists), make sure
  // the final state of the file is the original adapter.
  afterAll(() => {
    api.defaults.adapter = originalAdapter
  })

  it('leaves URLs untouched when no group slug is set', async () => {
    await api.get('/api/v1/locations')

    expect(captured).toBe('/api/v1/locations')
  })

  it.each([
    ['/api/v1/locations', '/api/v1/g/my-slug/locations'],
    ['/api/v1/areas', '/api/v1/g/my-slug/areas'],
    ['/api/v1/commodities', '/api/v1/g/my-slug/commodities'],
    ['/api/v1/files', '/api/v1/g/my-slug/files'],
    ['/api/v1/exports', '/api/v1/g/my-slug/exports'],
    ['/api/v1/upload-slots', '/api/v1/g/my-slug/upload-slots'],
    ['/api/v1/uploads', '/api/v1/g/my-slug/uploads'],
    ['/api/v1/settings', '/api/v1/g/my-slug/settings'],
    ['/api/v1/search', '/api/v1/g/my-slug/search'],
  ])('rewrites group-scoped endpoint %s to %s', async (input, expected) => {
    localStorage.setItem(GROUP_SLUG_KEY, 'my-slug')

    await api.get(input)

    expect(captured).toBe(expected)
  })

  it('preserves path segments and query strings when rewriting', async () => {
    localStorage.setItem(GROUP_SLUG_KEY, 'my-slug')

    await api.get('/api/v1/commodities/abc-123?include=images')

    expect(captured).toBe('/api/v1/g/my-slug/commodities/abc-123?include=images')
  })

  it('does NOT rewrite endpoints outside the group-scoped prefix list', async () => {
    localStorage.setItem(GROUP_SLUG_KEY, 'my-slug')

    await api.get('/api/v1/groups')
    expect(captured).toBe('/api/v1/groups')

    await api.get('/api/v1/auth/me')
    expect(captured).toBe('/api/v1/auth/me')

    await api.get('/api/v1/invites/some-token')
    expect(captured).toBe('/api/v1/invites/some-token')
  })
})
