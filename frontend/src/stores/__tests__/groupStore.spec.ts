import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import groupService from '@/services/groupService'
import type { GroupMembership, LocationGroup } from '@/types/group'

// The store talks to groupService for API data and to authStore for the
// current user id and default_group_id preference. Mocking both keeps the
// tests focused on the priority chain in restoreFromPreference() and the
// legacy-localStorage migration path (#1300 cleanup of #1262).

vi.mock('@/services/groupService', () => ({
  default: {
    listGroups: vi.fn(),
    listMembers: vi.fn(),
    createGroup: vi.fn(),
    updateGroup: vi.fn(),
  },
}))

// authStore is both queried (user / userDefaultGroupID getters) and mutated
// (updateProfile called by the legacy-storage migration). Expose updateProfile
// as a spy so individual tests can assert it was called with the right id.
const authUpdateProfile = vi.fn().mockResolvedValue(undefined)
const authMockState: { user: { id: string; name?: string; default_group_id?: string | null } | null } = {
  user: { id: 'user-1', name: 'Alice', default_group_id: null },
}

vi.mock('@/stores/authStore', () => ({
  useAuthStore: () => ({
    get user() {
      return authMockState.user
    },
    get userDefaultGroupID() {
      return authMockState.user?.default_group_id ?? null
    },
    updateProfile: authUpdateProfile,
  }),
}))

const mockedGroupService = vi.mocked(groupService)

// Legacy keys that the one-shot migration looks for on boot. Tests seed
// these to exercise the migration path and assert it wipes them afterwards.
const LEGACY_STORAGE_KEY_CURRENT_GROUP = 'inventario_current_group'
const LEGACY_STORAGE_KEY_GROUP_SLUG = 'currentGroupSlug'

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

async function freshStore(): Promise<ReturnType<typeof import('@/stores/groupStore').useGroupStore>> {
  vi.resetModules()
  const { useGroupStore } = await import('@/stores/groupStore')
  setActivePinia(createPinia())
  return useGroupStore()
}

describe('groupStore — initial state (#1300)', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    authMockState.user = { id: 'user-1', name: 'Alice', default_group_id: null }
  })

  afterEach(() => {
    localStorage.clear()
    authMockState.user = { id: 'user-1', name: 'Alice', default_group_id: null }
  })

  it('starts with null currentGroup regardless of localStorage state', async () => {
    // Legacy snapshots must not pre-seed currentGroup synchronously anymore —
    // the router's /g/:groupSlug/ param is the authoritative source of truth
    // and reading localStorage at construction time would re-introduce the
    // cross-tab coupling that #1289 Gap C and #1300 removed.
    const legacy = makeGroup({ id: 'grp-7', slug: 'office', name: 'Office' })
    localStorage.setItem(LEGACY_STORAGE_KEY_CURRENT_GROUP, JSON.stringify(legacy))

    const store = await freshStore()

    expect(store.currentGroup).toBeNull()
    expect(store.currentGroupId).toBeNull()
    expect(store.currentGroupSlug).toBeNull()
    expect(store.hasGroups).toBe(false)
  })

  it('starts with null currentGroup when nothing is stored', async () => {
    const store = await freshStore()
    expect(store.currentGroup).toBeNull()
  })
})

describe('groupStore — restoreFromPreference (#1263 / #1300)', () => {
  // After #1300, the priority chain for seeding currentGroup on a cold start
  // (i.e. a route with no /g/:groupSlug/ param) is:
  //   1. user.default_group_id (#1263).
  //   2. pickFallbackGroup: oldest group the user created, else oldest they
  //      were invited to.
  //   3. groups[0] — defensive last resort.
  // localStorage is no longer part of the chain.

  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    authMockState.user = { id: 'user-1', name: 'Alice', default_group_id: null }
  })

  afterEach(() => {
    localStorage.clear()
    authMockState.user = { id: 'user-1', name: 'Alice', default_group_id: null }
  })

  it('honours user.default_group_id when set', async () => {
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
      // would ignore this group; only the preference can make it win.
      created_by: 'user-99',
      created_at: '2026-03-01T00:00:00Z',
    })
    mockedGroupService.listGroups.mockResolvedValueOnce([primary, preferred])
    mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])
    authMockState.user = { id: 'user-1', name: 'Alice', default_group_id: 'grp-preferred' }

    const store = await freshStore()
    await store.fetchGroups()
    await store.restoreFromPreference()

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
    authMockState.user = { id: 'user-1', name: 'Alice', default_group_id: 'grp-that-does-not-exist' }

    const store = await freshStore()
    await store.fetchGroups()
    await store.restoreFromPreference()

    // Oldest "created by me" wins over any invited group, regardless of age.
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
    authMockState.user = { id: 'user-1', name: 'Alice', default_group_id: null }

    const store = await freshStore()
    await store.fetchGroups()
    await store.restoreFromPreference()

    expect(store.currentGroup?.id).toBe('grp-invited-older')
  })

  it('clears currentGroup when the fresh groups list is empty', async () => {
    mockedGroupService.listGroups.mockResolvedValueOnce([])

    const store = await freshStore()
    await store.fetchGroups()
    await store.restoreFromPreference()

    expect(store.currentGroup).toBeNull()
  })
})

describe('groupStore — setters do not touch localStorage (#1300)', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    authMockState.user = { id: 'user-1', name: 'Alice', default_group_id: null }
  })

  afterEach(() => {
    localStorage.clear()
    authMockState.user = { id: 'user-1', name: 'Alice', default_group_id: null }
  })

  it('setCurrentGroup only mutates in-memory state', async () => {
    const a = makeGroup({ id: 'grp-a', slug: 'a', name: 'A' })
    const b = makeGroup({ id: 'grp-b', slug: 'b', name: 'B' })
    mockedGroupService.listGroups.mockResolvedValueOnce([a, b])
    mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])

    const store = await freshStore()
    await store.fetchGroups()
    await store.setCurrentGroup('b')

    expect(store.currentGroupId).toBe('grp-b')
    expect(localStorage.getItem(LEGACY_STORAGE_KEY_CURRENT_GROUP)).toBeNull()
    expect(localStorage.getItem(LEGACY_STORAGE_KEY_GROUP_SLUG)).toBeNull()
  })

  it('clearAll leaves localStorage keys alone (migration wipes them once)', async () => {
    // clearAll is called on logout; it zeroes in-memory state. The legacy
    // keys are gone by the time logout fires (migration runs on bootstrap),
    // so clearAll has no business poking at them anymore.
    const store = await freshStore()
    store.clearAll()

    expect(store.groups).toEqual([])
    expect(store.currentGroup).toBeNull()
    expect(store.isInitialized).toBe(false)
  })

  it('updateGroupById refreshes in-memory state only', async () => {
    const original = makeGroup({ id: 'grp-1', name: 'Home' })
    const renamed = makeGroup({ id: 'grp-1', name: 'Home Office' })
    mockedGroupService.listGroups.mockResolvedValueOnce([original])
    mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])
    mockedGroupService.updateGroup.mockResolvedValueOnce(renamed)

    const store = await freshStore()
    await store.fetchGroups()
    await store.setCurrentGroup('home')
    await store.updateGroupById('grp-1', 'Home Office')

    expect(store.currentGroupName).toBe('Home Office')
    expect(localStorage.getItem(LEGACY_STORAGE_KEY_CURRENT_GROUP)).toBeNull()
  })
})

describe('groupStore — legacy localStorage migration (#1300)', () => {
  // One-shot cleanup on app boot: if localStorage has the pre-#1300 keys,
  // read them once, call PUT /auth/me with the matching default_group_id,
  // then removeItem. Runs inside ensureLoaded() after fetchGroups().

  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    authMockState.user = { id: 'user-1', name: 'Alice', default_group_id: null }
  })

  afterEach(() => {
    localStorage.clear()
    authMockState.user = { id: 'user-1', name: 'Alice', default_group_id: null }
  })

  it('promotes a legacy snapshot to default_group_id when no preference is set', async () => {
    const other = makeGroup({ id: 'grp-other', slug: 'other', created_by: 'user-1', created_at: '2026-02-01T00:00:00Z' })
    const legacy = makeGroup({ id: 'grp-legacy', slug: 'legacy', name: 'Legacy', created_by: 'user-1', created_at: '2026-01-01T00:00:00Z' })
    localStorage.setItem(LEGACY_STORAGE_KEY_CURRENT_GROUP, JSON.stringify(legacy))
    mockedGroupService.listGroups.mockResolvedValueOnce([other, legacy])
    mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])

    const store = await freshStore()
    await store.ensureLoaded()

    expect(authUpdateProfile).toHaveBeenCalledWith({ name: 'Alice', default_group_id: 'grp-legacy' })
    expect(localStorage.getItem(LEGACY_STORAGE_KEY_CURRENT_GROUP)).toBeNull()
    expect(localStorage.getItem(LEGACY_STORAGE_KEY_GROUP_SLUG)).toBeNull()
  })

  it('promotes the legacy slug key when the snapshot is absent', async () => {
    const beta = makeGroup({ id: 'grp-beta', slug: 'beta', name: 'Beta', created_by: 'user-1' })
    const alpha = makeGroup({ id: 'grp-alpha', slug: 'alpha', name: 'Alpha', created_by: 'user-1', created_at: '2025-12-01T00:00:00Z' })
    localStorage.setItem(LEGACY_STORAGE_KEY_GROUP_SLUG, 'beta')
    mockedGroupService.listGroups.mockResolvedValueOnce([alpha, beta])
    mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])

    const store = await freshStore()
    await store.ensureLoaded()

    expect(authUpdateProfile).toHaveBeenCalledWith({ name: 'Alice', default_group_id: 'grp-beta' })
    expect(localStorage.getItem(LEGACY_STORAGE_KEY_GROUP_SLUG)).toBeNull()
  })

  it('does not overwrite an existing default_group_id preference', async () => {
    const legacy = makeGroup({ id: 'grp-legacy', slug: 'legacy', created_by: 'user-1' })
    const preferred = makeGroup({ id: 'grp-preferred', slug: 'preferred', created_by: 'user-1' })
    localStorage.setItem(LEGACY_STORAGE_KEY_CURRENT_GROUP, JSON.stringify(legacy))
    mockedGroupService.listGroups.mockResolvedValueOnce([legacy, preferred])
    mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])
    authMockState.user = { id: 'user-1', name: 'Alice', default_group_id: 'grp-preferred' }

    const store = await freshStore()
    await store.ensureLoaded()

    expect(authUpdateProfile).not.toHaveBeenCalled()
    // Legacy keys are still dropped — the preference is the future source of
    // truth, and the old keys are dead weight either way.
    expect(localStorage.getItem(LEGACY_STORAGE_KEY_CURRENT_GROUP)).toBeNull()
  })

  it('drops unresolvable legacy keys without calling PUT /auth/me', async () => {
    // Legacy snapshot points at a group the user no longer has access to.
    // The migration must not call updateProfile with a value the server
    // would reject — just wipe the key and move on.
    const stale = makeGroup({ id: 'grp-gone', slug: 'gone' })
    const available = makeGroup({ id: 'grp-available', slug: 'available', created_by: 'user-1' })
    localStorage.setItem(LEGACY_STORAGE_KEY_CURRENT_GROUP, JSON.stringify(stale))
    mockedGroupService.listGroups.mockResolvedValueOnce([available])
    mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])

    const store = await freshStore()
    await store.ensureLoaded()

    expect(authUpdateProfile).not.toHaveBeenCalled()
    expect(localStorage.getItem(LEGACY_STORAGE_KEY_CURRENT_GROUP)).toBeNull()
  })

  it('does not crash on a malformed legacy snapshot', async () => {
    localStorage.setItem(LEGACY_STORAGE_KEY_CURRENT_GROUP, 'not-valid-json')
    const available = makeGroup({ id: 'grp-available', slug: 'available', created_by: 'user-1' })
    mockedGroupService.listGroups.mockResolvedValueOnce([available])
    mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])

    const store = await freshStore()
    await store.ensureLoaded()

    expect(authUpdateProfile).not.toHaveBeenCalled()
    expect(localStorage.getItem(LEGACY_STORAGE_KEY_CURRENT_GROUP)).toBeNull()
    expect(store.isInitialized).toBe(true)
  })
})
