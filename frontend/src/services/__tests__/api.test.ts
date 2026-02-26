import { describe, it, expect, beforeEach } from 'vitest'
import { setCsrfToken, getCsrfToken, clearCsrfToken } from '../api'

// The key used by api.ts to persist the CSRF token in sessionStorage.
const CSRF_SESSION_KEY = 'inventario_csrf_token'

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

