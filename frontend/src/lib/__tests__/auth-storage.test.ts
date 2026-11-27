import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { clearAuth, getAccessToken, onAuthIdentityChange, setAccessToken } from "@/lib/auth-storage"
import { fakeAccessToken } from "@/test/handlers/auth"

const TOKEN_KEY = "inventario_token"

// Another tab wrote the token: the browser fires a storage event in this one.
function otherTabWrites(oldValue: string | null, newValue: string | null) {
  if (newValue === null) window.localStorage.removeItem(TOKEN_KEY)
  else window.localStorage.setItem(TOKEN_KEY, newValue)
  window.dispatchEvent(
    new StorageEvent("storage", {
      key: TOKEN_KEY,
      oldValue,
      newValue,
      storageArea: window.localStorage,
    })
  )
}

let unsubscribe: (() => void) | undefined

beforeEach(() => {
  clearAuth()
})

afterEach(() => {
  unsubscribe?.()
  unsubscribe = undefined
})

function subscribe() {
  const listener = vi.fn()
  unsubscribe = onAuthIdentityChange(listener)
  return listener
}

describe("onAuthIdentityChange", () => {
  it("reports a sign-in, a switch to another user and a sign-out in this tab", () => {
    const listener = subscribe()

    setAccessToken(fakeAccessToken("user-a"))
    expect(listener).toHaveBeenCalledTimes(1)

    setAccessToken(fakeAccessToken("user-b"))
    expect(listener).toHaveBeenCalledTimes(2)

    clearAuth()
    expect(listener).toHaveBeenCalledTimes(3)
    expect(getAccessToken()).toBeNull()
  })

  it("does not report a refreshed token for the same user", () => {
    setAccessToken(fakeAccessToken("user-a", "first"))
    const listener = subscribe()

    setAccessToken(fakeAccessToken("user-a", "second"))

    expect(listener).not.toHaveBeenCalled()
  })

  it("does not report clearing storage that holds no token", () => {
    const listener = subscribe()

    clearAuth()

    expect(listener).not.toHaveBeenCalled()
  })

  it("treats tokens without a readable user id as one user", () => {
    setAccessToken("opaque-token")
    const listener = subscribe()

    setAccessToken("another-opaque-token")
    expect(listener).not.toHaveBeenCalled()

    clearAuth()
    expect(listener).toHaveBeenCalledTimes(1)
  })

  it("reports a sign-out and a switch of user made in another tab", () => {
    const tokenA = fakeAccessToken("user-a")
    const tokenB = fakeAccessToken("user-b")
    setAccessToken(tokenA)
    const listener = subscribe()

    otherTabWrites(tokenA, tokenB)
    expect(listener).toHaveBeenCalledTimes(1)

    otherTabWrites(tokenB, null)
    expect(listener).toHaveBeenCalledTimes(2)
  })

  it("ignores a token refresh in another tab and writes to other keys", () => {
    const first = fakeAccessToken("user-a", "first")
    setAccessToken(first)
    const listener = subscribe()

    otherTabWrites(first, fakeAccessToken("user-a", "second"))
    window.dispatchEvent(
      new StorageEvent("storage", {
        key: "inventario-theme",
        oldValue: "light",
        newValue: "dark",
        storageArea: window.localStorage,
      })
    )

    expect(listener).not.toHaveBeenCalled()
  })

  it("reports another tab clearing the whole storage area", () => {
    setAccessToken(fakeAccessToken("user-a"))
    const listener = subscribe()

    window.localStorage.clear()
    window.dispatchEvent(
      new StorageEvent("storage", { key: null, storageArea: window.localStorage })
    )

    expect(listener).toHaveBeenCalledTimes(1)
  })

  it("stops calling a listener after it unsubscribes", () => {
    const listener = vi.fn()
    onAuthIdentityChange(listener)()

    setAccessToken(fakeAccessToken("user-a"))

    expect(listener).not.toHaveBeenCalled()
  })
})
