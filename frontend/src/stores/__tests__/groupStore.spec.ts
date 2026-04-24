import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import groupService from '@/services/groupService'
import type { GroupMembership, LocationGroup } from '@/types/group'

// The store talks to groupService for API data and to authStore for the
// current user id and default_group_id preference. Mocking both keeps the
// tests focused on the priority chain in restoreFromPreference().

vi.mock('@/services/groupService', () => ({
  default: {
    listGroups: vi.fn(),
    listMembers: vi.fn(),
    createGroup: vi.fn(),
    updateGroup: vi.fn(),
  },
}))

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
  }),
}))

const mockedGroupService = vi.mocked(groupService)

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

describe('groupStore — initial state', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    authMockState.user = { id: 'user-1', name: 'Alice', default_group_id: null }
  })

  afterEach(() => {
    authMockState.user = { id: 'user-1', name: 'Alice', default_group_id: null }
  })

  it('starts with null currentGroup on a cold store', async () => {
    // The router's /g/:groupSlug/ param is the authoritative source of truth
    // for the active group; the store must not synthesise one at construction
    // time.
    const store = await freshStore()

    expect(store.currentGroup).toBeNull()
    expect(store.currentGroupId).toBeNull()
    expect(store.currentGroupSlug).toBeNull()
    expect(store.hasGroups).toBe(false)
  })
})

describe('groupStore — restoreFromPreference (#1263)', () => {
  // The priority chain for seeding currentGroup on a cold start (i.e. a
  // route with no /g/:groupSlug/ param) is:
  //   1. user.default_group_id (#1263).
  //   2. pickFallbackGroup: oldest group the user created, else oldest they
  //      were invited to.
  //   3. groups[0] — defensive last resort.

  beforeEach(() => {
    vi.clearAllMocks()
    authMockState.user = { id: 'user-1', name: 'Alice', default_group_id: null }
  })

  afterEach(() => {
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

describe('groupStore — setters', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    authMockState.user = { id: 'user-1', name: 'Alice', default_group_id: null }
  })

  afterEach(() => {
    authMockState.user = { id: 'user-1', name: 'Alice', default_group_id: null }
  })

  it('setCurrentGroup mutates in-memory state', async () => {
    const a = makeGroup({ id: 'grp-a', slug: 'a', name: 'A' })
    const b = makeGroup({ id: 'grp-b', slug: 'b', name: 'B' })
    mockedGroupService.listGroups.mockResolvedValueOnce([a, b])
    mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])

    const store = await freshStore()
    await store.fetchGroups()
    await store.setCurrentGroup('b')

    expect(store.currentGroupId).toBe('grp-b')
  })

  it('clearAll zeroes in-memory state', async () => {
    const store = await freshStore()
    store.clearAll()

    expect(store.groups).toEqual([])
    expect(store.currentGroup).toBeNull()
    expect(store.isInitialized).toBe(false)
  })

  it('updateGroupById refreshes in-memory state', async () => {
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
  })
})

describe('groupStore — groupPath (#1321)', () => {
  // groupPath is the single source of truth for building /g/<slug>/ scoped
  // URLs in views, services, and router.push() sites. It replaces the
  // ad-hoc flat paths that used to rely on the router's legacyFlatDataRoute
  // rewriter.

  beforeEach(() => {
    vi.clearAllMocks()
    authMockState.user = { id: 'user-1', name: 'Alice', default_group_id: null }
  })

  it('prefixes the subpath with /g/<slug> when a group is active', async () => {
    const home = makeGroup({ id: 'grp-1', slug: 'home', name: 'Home' })
    mockedGroupService.listGroups.mockResolvedValueOnce([home])
    mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])

    const store = await freshStore()
    await store.fetchGroups()
    await store.setCurrentGroup('home')

    expect(store.groupPath('/locations')).toBe('/g/home/locations')
    expect(store.groupPath('areas/area-1')).toBe('/g/home/areas/area-1')
  })

  it('returns the unscoped subpath when no group is active', async () => {
    const store = await freshStore()

    expect(store.groupPath('/locations')).toBe('/locations')
    expect(store.groupPath('areas')).toBe('/areas')
  })

  it('URL-encodes slugs that contain reserved characters', async () => {
    const odd = makeGroup({ id: 'grp-odd', slug: 'a b/c', name: 'Odd' })
    mockedGroupService.listGroups.mockResolvedValueOnce([odd])
    mockedGroupService.listMembers.mockResolvedValueOnce([makeMembership('user-1')])

    const store = await freshStore()
    await store.fetchGroups()
    await store.setCurrentGroup('a b/c')

    expect(store.groupPath('/locations')).toBe('/g/a%20b%2Fc/locations')
  })
})
