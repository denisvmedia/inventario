import { afterEach, describe, expect, it } from "vitest"

import {
  clearPendingInvite,
  consumePendingInvite,
  peekPendingInvite,
  savePendingInvite,
} from "@/features/auth/inviteHandoff"

afterEach(() => {
  clearPendingInvite()
})

describe("inviteHandoff", () => {
  it("round-trips a saved invite via sessionStorage", () => {
    savePendingInvite({ token: "tok-1", groupName: "Household" })
    expect(peekPendingInvite()).toEqual({ token: "tok-1", groupName: "Household" })
  })

  it("consumePendingInvite returns the value and clears storage", () => {
    savePendingInvite({ token: "tok-2" })
    expect(consumePendingInvite()).toEqual({ token: "tok-2" })
    expect(peekPendingInvite()).toBeNull()
  })

  it("ignores malformed sessionStorage entries", () => {
    sessionStorage.setItem("inventario_pending_invite", "{not-json")
    expect(peekPendingInvite()).toBeNull()
  })

  it("ignores entries missing a token", () => {
    sessionStorage.setItem("inventario_pending_invite", JSON.stringify({ groupName: "Foo" }))
    expect(peekPendingInvite()).toBeNull()
  })
})
