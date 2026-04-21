import { describe, it, expect, beforeEach } from 'vitest'

import {
  savePendingInvite,
  peekPendingInvite,
  consumePendingInvite,
  clearPendingInvite,
} from '../inviteHandoff'

const STORAGE_KEY = 'inventario_pending_invite'

describe('inviteHandoff', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  describe('savePendingInvite', () => {
    it('persists the token and group name to sessionStorage', () => {
      savePendingInvite({ token: 'abc', groupName: 'Lab' })
      const raw = sessionStorage.getItem(STORAGE_KEY)
      expect(raw).not.toBeNull()
      expect(JSON.parse(raw as string)).toEqual({ token: 'abc', groupName: 'Lab' })
    })

    it('persists the token alone when groupName is omitted', () => {
      savePendingInvite({ token: 'abc' })
      expect(JSON.parse(sessionStorage.getItem(STORAGE_KEY) as string)).toEqual({ token: 'abc' })
    })
  })

  describe('peekPendingInvite', () => {
    it('returns null when no invite is stored', () => {
      expect(peekPendingInvite()).toBeNull()
    })

    it('reads a previously saved invite', () => {
      savePendingInvite({ token: 'abc', groupName: 'Lab' })
      expect(peekPendingInvite()).toEqual({ token: 'abc', groupName: 'Lab' })
    })

    it('does not remove the invite on peek (repeat reads return the same value)', () => {
      savePendingInvite({ token: 'abc' })
      peekPendingInvite()
      expect(peekPendingInvite()).toEqual({ token: 'abc' })
    })

    it('returns null on malformed JSON payload', () => {
      sessionStorage.setItem(STORAGE_KEY, '{not json')
      expect(peekPendingInvite()).toBeNull()
    })

    it('returns null when the payload is missing a token', () => {
      sessionStorage.setItem(STORAGE_KEY, JSON.stringify({ groupName: 'orphan' }))
      expect(peekPendingInvite()).toBeNull()
    })
  })

  describe('consumePendingInvite', () => {
    it('returns the stored invite and clears it atomically', () => {
      savePendingInvite({ token: 'abc', groupName: 'Lab' })
      const consumed = consumePendingInvite()
      expect(consumed).toEqual({ token: 'abc', groupName: 'Lab' })
      expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
    })

    it('returns null and leaves storage untouched when empty', () => {
      expect(consumePendingInvite()).toBeNull()
      expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
    })
  })

  describe('clearPendingInvite', () => {
    it('removes the stored invite', () => {
      savePendingInvite({ token: 'abc' })
      clearPendingInvite()
      expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
    })

    it('is a no-op when nothing is stored', () => {
      expect(() => clearPendingInvite()).not.toThrow()
    })
  })
})
