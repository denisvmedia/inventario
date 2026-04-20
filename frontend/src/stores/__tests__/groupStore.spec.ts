import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import groupService from '@/services/groupService'
import type { GroupMembership, LocationGroup } from '@/types/group'

// The store talks to groupService for API data and to authStore for the
// current user id (used by loadCurrentMembership). Mocking both keeps the
// tests focused on the persistence + reconciliation logic introduced for
// #1262 — "Current group selection lost on page refresh".

vi.mock('@/services/groupService', () => ({
  default: {
    listGroups: vi.fn(),
    listMembers: vi.fn(),
    createGroup: vi.fn(),
    updateGroup: vi.fn(),
  },
}))

// Mutable so tests for #1263 can seed user.default_group_id per case.
const authMockState: { user: { id: string; default_group_id?: string | null } | null } = {
  user: { id: 'user-1', default_group_id: null },
}

vi.mock('@/stores/authStore', () => ({
  useAuthStore: () => ({
    get user() {
      return authMockState.user
    },
    get userDefaultGroupID() {
      return authMockState.user?.default_group_id ?? null
    },
  }),
}))

const mockedGroupService = vi.mocked(groupService)

const STORAGE_KEY_CURRENT_GROUP = 'inventario_current_group'
const STORAGE_KEY_GROUP_SLUG_LEGACY = 'currentGroupSlug'

function makeGroup(overrides: Partial<LocationGroup> = {}): LocationGroup {
  return {
    id: 'grp-1',
    slug: 'home',
    name: 'Home',
    icon: '🏠',
    status: 'active',
    main_currency: 'USD',
    created_by: 'user-1',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

function makeMembership(userId: string, role: 'admin' | 'user' = 'admin'): GroupMembership {
  return {
    id: `mem-${userId}`,
    group_id: 'grp-1',
    member_user_id: userId,
    role,
    joined_at: '2026-01-01T00:00:00Z',
  }
}

// The groupStore reads localStorage synchronously at state-initialization
// time (to pre-seed currentGroup before the first render). That means the
// localStorage value set inside a test is only picked up if the module is
// re-imported fresh — hence resetModules() + dynamic import in each test
// that depends on the initial snapshot behavior.
async function freshStore(): Promise<ReturnType<typeof import('@/stores/groupStore').useGroupStore>> {
  vi.resetModules()
  const { useGroupStore } = await import('@/stores/groupStore')
  setActivePinia(createPinia())
  return useGroupStore()
}

describe('groupStore — localStorage persistence (#1262)', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    authMockState.user = { id: 'user-1', default_group_id: null }
  })

  afterEach(() => {
    localStorage.clear()
    authMockState.user = { id: 'user-1', default_group_id: null }
  })

  describe('initial state rehydration from snapshot', () => {
    it('seeds currentGroup from a stored snapshot before any API call', async () => {
      // Simulates a page refresh for a user who previously selected a group.
      // The header's GroupSelector must render the group name immediately on
      // mount — not wait for /api/v1/groups to resolve — otherwise the user
      // sees a "Select Group" flash (the symptom reported in #1262).
      const stored = makeGroup({ id: 'grp-7', slug: 'office', name: 'Office', icon: '🏢' })
      localStorage.setItem(STORAGE_KEY_CURRENT_GROUP, JSON.stringify(stored))

      const store = await freshStore()

      expect(store.currentGroup).toEqual(stored)
      expect(store.currentGroupId).toBe('grp-7')
      expect(store.currentGroupSlug).toBe('office')
      expect(store.currentGroupName).toBe('Office')
      expect(store.currentGroupIcon).toBe('🏢')
      // The full groups list hasn't loaded yet, so the selector's dropdown is
      // still hidden (gated by hasGroups). Reconciliation happens in
      // restoreFromStorage() once fetchGroups() resolves.
      expect(store.hasGroups).toBe(false)
    })

    it('starts with null currentGroup when no snapshot exists', async () => {
      const store = await freshStore()
      expect(store.currentGroup).toBeNull()
    })

    it('ignores a malformed snapshot instead of crashing', async () => {
      // A corrupted or partial JSON blob must not prevent the app from
      // booting — return null and fall through to the normal fetch flow.
      localStorage.setItem(STORAGE_KEY_CURRENT_GROUP, 'not-valid-json')
      const store = await freshStore()
      expect(store.currentGroup).toBeNull()
    })

    it('ignores a snapshot missing required fields', async () => {
      localStorage.setItem(STORAGE_KEY_CURRENT_GROUP, JSON.stringify({ foo: 'bar' }))
      const store = await freshStore()
      expect(store.currentGroup).toBeNull()
    })

    it('rejects a partial snapshot that has only id and slug', async () => {
      // A corrupted or truncated snapshot would otherwise pre-seed currentGroup
      // with undefined name/icon, so the header briefly renders placeholders
      // until reconciliation arrives. Full-shape validation is the cheaper fix.
      localStorage.setItem(
        STORAGE_KEY_CURRENT_GROUP,
        JSON.stringify({ id: 'grp-1', slug: 'home' }),
      )
      const store = await freshStore()
      expect(store.currentGroup).toBeNull()
    })
  })

  describe('restoreFromStorage reconciliation', () => {
    it('keeps the stored group when it is still in the fresh list', async () => {
      const a = makeGroup({ id: 'grp-a', slug: 'a', name: 'Alpha' })
      const b = makeGroup({ id: 'grp-b', slug: 'b', name: 'Beta' })
      localStorage.setItem(STORAGE_KEY_CURRENT_GROUP, JSON.stringify(b))

      mockedGroupService.listGroups.mockResolvedValueOnce([a, b])
      mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])

      const store = await freshStore()
      await store.fetchGroups()
      await store.restoreFromStorage()

      expect(store.currentGroupId).toBe('grp-b')
      expect(store.currentGroupName).toBe('Beta')
    })

    it('refreshes the snapshot with server-authoritative data after reconcile', async () => {
      // User renamed the group on another device — the local snapshot has a
      // stale name/icon. After reconciliation, the server copy wins and the
      // localStorage snapshot must be rewritten; otherwise a future refresh
      // flashes the stale name before reconciliation runs.
      const stale = makeGroup({ id: 'grp-1', slug: 'home', name: 'Home', icon: '🏠' })
      const fresh = makeGroup({ id: 'grp-1', slug: 'home', name: 'Home Sweet Home', icon: '🏡' })
      localStorage.setItem(STORAGE_KEY_CURRENT_GROUP, JSON.stringify(stale))

      mockedGroupService.listGroups.mockResolvedValueOnce([fresh])
      mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])

      const store = await freshStore()
      await store.fetchGroups()
      await store.restoreFromStorage()

      expect(store.currentGroupName).toBe('Home Sweet Home')
      expect(store.currentGroupIcon).toBe('🏡')
      const persisted = JSON.parse(localStorage.getItem(STORAGE_KEY_CURRENT_GROUP) || 'null')
      expect(persisted).toEqual(fresh)
    })

    it('falls back to the first group when the stored group is no longer accessible', async () => {
      // Covers the acceptance criterion "Graceful fallback when the saved
      // group is no longer accessible" — e.g. user was removed, or logged
      // in as someone else with a different group set.
      const stale = makeGroup({ id: 'grp-gone', slug: 'gone', name: 'Gone' })
      const first = makeGroup({ id: 'grp-new', slug: 'new', name: 'New' })
      localStorage.setItem(STORAGE_KEY_CURRENT_GROUP, JSON.stringify(stale))

      mockedGroupService.listGroups.mockResolvedValueOnce([first])
      mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])

      const store = await freshStore()
      await store.fetchGroups()
      await store.restoreFromStorage()

      expect(store.currentGroupId).toBe('grp-new')
      const persisted = JSON.parse(localStorage.getItem(STORAGE_KEY_CURRENT_GROUP) || 'null')
      expect(persisted.id).toBe('grp-new')
    })

    it('falls back to the legacy slug key when the snapshot key is absent', async () => {
      // Users upgrading from the pre-#1262 deployment only have the slug
      // stored. Reconciliation must still honor their prior selection; a
      // silent revert to "first group" would flip their active group on
      // the first refresh after the update.
      const first = makeGroup({ id: 'grp-a', slug: 'alpha', name: 'Alpha' })
      const second = makeGroup({ id: 'grp-b', slug: 'beta', name: 'Beta' })
      localStorage.setItem(STORAGE_KEY_GROUP_SLUG_LEGACY, 'beta')

      mockedGroupService.listGroups.mockResolvedValueOnce([first, second])
      mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])

      const store = await freshStore()
      await store.fetchGroups()
      await store.restoreFromStorage()

      expect(store.currentGroupId).toBe('grp-b')
    })

    it('clears currentGroup when the fresh groups list is empty', async () => {
      // A user who was removed from every group should not retain a stale
      // currentGroup — the app needs to send them to /no-group instead.
      const stale = makeGroup()
      localStorage.setItem(STORAGE_KEY_CURRENT_GROUP, JSON.stringify(stale))

      mockedGroupService.listGroups.mockResolvedValueOnce([])

      const store = await freshStore()
      await store.fetchGroups()
      await store.restoreFromStorage()

      expect(store.currentGroup).toBeNull()
      expect(localStorage.getItem(STORAGE_KEY_CURRENT_GROUP)).toBeNull()
    })
  })

  describe('setters persist to localStorage', () => {
    it('setCurrentGroup writes the full snapshot', async () => {
      const a = makeGroup({ id: 'grp-a', slug: 'a', name: 'A' })
      const b = makeGroup({ id: 'grp-b', slug: 'b', name: 'B' })
      mockedGroupService.listGroups.mockResolvedValueOnce([a, b])
      mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])

      const store = await freshStore()
      await store.fetchGroups()
      await store.setCurrentGroup('b')

      const persisted = JSON.parse(localStorage.getItem(STORAGE_KEY_CURRENT_GROUP) || 'null')
      expect(persisted).toEqual(b)
    })

    it('setCurrentGroup mirrors the slug for the api.ts interceptor', async () => {
      // services/api.ts reads STORAGE_KEY_GROUP_SLUG_LEGACY on every request
      // to rewrite /api/v1/<resource> into /api/v1/g/{slug}/<resource>. If the
      // snapshot writer drops this mirror, every group-scoped fetch silently
      // stops rewriting — the CI-breaking bug that surfaced after the first
      // pass at #1262 and was flagged by review.
      const a = makeGroup({ id: 'grp-a', slug: 'a', name: 'A' })
      const b = makeGroup({ id: 'grp-b', slug: 'beta-slug', name: 'B' })
      mockedGroupService.listGroups.mockResolvedValueOnce([a, b])
      mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])

      const store = await freshStore()
      await store.fetchGroups()
      await store.setCurrentGroup('beta-slug')

      expect(localStorage.getItem(STORAGE_KEY_GROUP_SLUG_LEGACY)).toBe('beta-slug')
    })

    it('clearAll removes both the snapshot and the legacy slug', async () => {
      localStorage.setItem(STORAGE_KEY_CURRENT_GROUP, JSON.stringify(makeGroup()))
      localStorage.setItem(STORAGE_KEY_GROUP_SLUG_LEGACY, 'home')

      const store = await freshStore()
      store.clearAll()

      expect(localStorage.getItem(STORAGE_KEY_CURRENT_GROUP)).toBeNull()
      expect(localStorage.getItem(STORAGE_KEY_GROUP_SLUG_LEGACY)).toBeNull()
    })

    it('updateGroupById refreshes the snapshot when editing the current group', async () => {
      const original = makeGroup({ id: 'grp-1', name: 'Home' })
      const renamed = makeGroup({ id: 'grp-1', name: 'Home Office' })
      mockedGroupService.listGroups.mockResolvedValueOnce([original])
      mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])
      mockedGroupService.updateGroup.mockResolvedValueOnce(renamed)

      const store = await freshStore()
      await store.fetchGroups()
      await store.setCurrentGroup('home')
      await store.updateGroupById('grp-1', 'Home Office')

      const persisted = JSON.parse(localStorage.getItem(STORAGE_KEY_CURRENT_GROUP) || 'null')
      expect(persisted.name).toBe('Home Office')
    })
  })

  // -----------------------------------------------------------------------
  // #1263: user-level default group preference + deterministic fallback.
  // The ordering tested here mirrors the priority chain documented in
  // restoreFromStorage(): snapshot → legacy slug → user preference →
  // first-created → first-invited.
  // -----------------------------------------------------------------------
  describe('default group preference and fallback (#1263)', () => {
    it('honours user.default_group_id on a fresh device with no snapshot', async () => {
      const primary = makeGroup({
        id: 'grp-primary',
        slug: 'primary',
        name: 'Primary',
        created_by: 'user-1',
        created_at: '2026-01-01T00:00:00Z',
      })
      const preferred = makeGroup({
        id: 'grp-preferred',
        slug: 'preferred',
        name: 'Preferred',
        // Different creator so the fallback "first-created-by-user" branch
        // would ignore this group; the only way it wins is the preference.
        created_by: 'user-99',
        created_at: '2026-03-01T00:00:00Z',
      })
      mockedGroupService.listGroups.mockResolvedValueOnce([primary, preferred])
      mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])
      authMockState.user = { id: 'user-1', default_group_id: 'grp-preferred' }

      const store = await freshStore()
      await store.fetchGroups()
      await store.restoreFromStorage()

      expect(store.currentGroup?.id).toBe('grp-preferred')
    })

    it('falls back from a stale preference to first-created group', async () => {
      const firstCreated = makeGroup({
        id: 'grp-created-1',
        slug: 'created-first',
        name: 'Created First',
        created_by: 'user-1',
        created_at: '2026-01-01T00:00:00Z',
      })
      const laterCreated = makeGroup({
        id: 'grp-created-2',
        slug: 'created-second',
        name: 'Created Second',
        created_by: 'user-1',
        created_at: '2026-02-01T00:00:00Z',
      })
      const invited = makeGroup({
        id: 'grp-invited',
        slug: 'invited',
        name: 'Invited',
        created_by: 'user-99',
        created_at: '2025-12-01T00:00:00Z',
      })
      mockedGroupService.listGroups.mockResolvedValueOnce([laterCreated, invited, firstCreated])
      mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])
      // Preference points at a group the user no longer has access to — should
      // be skipped and trigger the fallback path.
      authMockState.user = { id: 'user-1', default_group_id: 'grp-that-does-not-exist' }

      const store = await freshStore()
      await store.fetchGroups()
      await store.restoreFromStorage()

      // Invited is older than both created-by-me, but the #1263 rule prefers
      // the oldest "created by me" group over any invited one regardless of age.
      expect(store.currentGroup?.id).toBe('grp-created-1')
    })

    it('falls back to first-invited group when the user created none', async () => {
      const invitedNewer = makeGroup({
        id: 'grp-invited-newer',
        slug: 'invited-newer',
        name: 'Invited Newer',
        created_by: 'user-99',
        created_at: '2026-05-01T00:00:00Z',
      })
      const invitedOlder = makeGroup({
        id: 'grp-invited-older',
        slug: 'invited-older',
        name: 'Invited Older',
        created_by: 'user-99',
        created_at: '2026-01-01T00:00:00Z',
      })
      mockedGroupService.listGroups.mockResolvedValueOnce([invitedNewer, invitedOlder])
      mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])
      authMockState.user = { id: 'user-1', default_group_id: null }

      const store = await freshStore()
      await store.fetchGroups()
      await store.restoreFromStorage()

      expect(store.currentGroup?.id).toBe('grp-invited-older')
    })

    it('session snapshot beats default_group_id (session continuity wins on refresh)', async () => {
      const snapshotGroup = makeGroup({
        id: 'grp-session',
        slug: 'session',
        name: 'Session',
        created_by: 'user-1',
      })
      const preferredGroup = makeGroup({
        id: 'grp-preferred',
        slug: 'preferred',
        name: 'Preferred',
        created_by: 'user-1',
      })
      // Pre-seed localStorage with the last-selected group — this is the
      // #1262 refresh-continuity behaviour; #1263 must not regress it.
      localStorage.setItem(STORAGE_KEY_CURRENT_GROUP, JSON.stringify(snapshotGroup))
      localStorage.setItem(STORAGE_KEY_GROUP_SLUG_LEGACY, snapshotGroup.slug)
      mockedGroupService.listGroups.mockResolvedValueOnce([snapshotGroup, preferredGroup])
      mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])
      authMockState.user = { id: 'user-1', default_group_id: 'grp-preferred' }

      const store = await freshStore()
      await store.fetchGroups()
      await store.restoreFromStorage()

      expect(store.currentGroup?.id).toBe('grp-session')
    })
  })
})
