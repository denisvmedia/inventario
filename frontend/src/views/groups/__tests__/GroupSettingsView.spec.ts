import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createRouter, createWebHistory, type Router } from 'vue-router'
import GroupSettingsView from '../GroupSettingsView.vue'
import groupService from '@/services/groupService'
import type { GroupMembership, LocationGroup } from '@/types/group'

// The component talks to groupService directly for data, to groupStore for
// side-effects after mutations, and to authStore for "who am I". All three
// are mocked so the tests can drive membership shape deterministically and
// exercise only the UI logic (isAdmin / isLastAdmin / hasPromotableMembers).

vi.mock('@/services/groupService', () => ({
  default: {
    getGroup: vi.fn(),
    listMembers: vi.fn(),
    listInvites: vi.fn(),
    leaveGroup: vi.fn(),
    deleteGroup: vi.fn(),
    updateMemberRole: vi.fn(),
    removeMember: vi.fn(),
    createInvite: vi.fn(),
    revokeInvite: vi.fn(),
  },
}))

const mockAuthStore = {
  user: { id: 'user-1', name: 'Alice', email: 'alice@example.com' },
}

vi.mock('@/stores/authStore', () => ({
  useAuthStore: () => mockAuthStore,
}))

const mockGroupStore = {
  currentGroup: null,
  hasGroups: false,
  updateGroupById: vi.fn(),
  clearCurrentGroup: vi.fn(),
  fetchGroups: vi.fn().mockResolvedValue(undefined),
  restoreFromPreference: vi.fn().mockResolvedValue(undefined),
}

vi.mock('@/stores/groupStore', () => ({
  useGroupStore: () => mockGroupStore,
}))

const mockedGroupService = vi.mocked(groupService)

function makeGroup(overrides: Partial<LocationGroup> = {}): LocationGroup {
  return {
    id: 'grp-1',
    slug: 'test-group',
    name: 'Test Group',
    icon: '📦',
    status: 'active',
    main_currency: 'USD',
    created_by: 'user-1',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

function makeMembership(
  userId: string,
  role: 'admin' | 'user',
  id = `mem-${userId}`,
): GroupMembership {
  return {
    id,
    group_id: 'grp-1',
    member_user_id: userId,
    role,
    joined_at: '2026-01-01T00:00:00Z',
  }
}

function createTestRouter(): Router {
  return createRouter({
    history: createWebHistory(),
    routes: [
      { path: '/', component: { template: '<div>home</div>' } },
      { path: '/no-group', name: 'no-group', component: { template: '<div>no-group</div>' } },
      {
        path: '/groups/:groupId/settings',
        name: 'group-settings',
        component: GroupSettingsView,
      },
    ],
  })
}

async function mountView(members: GroupMembership[]): Promise<VueWrapper> {
  mockedGroupService.getGroup.mockResolvedValueOnce(makeGroup())
  mockedGroupService.listMembers.mockResolvedValueOnce(members)
  // listInvites is only called for admins — return empty to keep tests focused.
  mockedGroupService.listInvites.mockResolvedValueOnce([])

  const router = createTestRouter()
  await router.push('/groups/grp-1/settings')
  await router.isReady()

  const wrapper = mount(GroupSettingsView, {
    global: { plugins: [router] },
  })

  // loadData() is async; wait for both getGroup and listMembers to settle
  // before assertions so computed props reflect the loaded membership.
  await flushPromises()
  return wrapper
}

describe('GroupSettingsView — Leave Group action', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    mockAuthStore.user = { id: 'user-1', name: 'Alice', email: 'alice@example.com' }
  })

  describe('when the current user is the sole admin (last admin)', () => {
    it('disables the Leave Group button', async () => {
      const wrapper = await mountView([
        makeMembership('user-1', 'admin'),
        makeMembership('user-2', 'user'),
      ])

      const leaveBtn = wrapper.get('[data-testid="leave-group-btn"]')
      expect(leaveBtn.attributes('disabled')).toBeDefined()
      expect(leaveBtn.attributes('aria-disabled')).toBe('true')
    })

    it('exposes the tooltip via the title attribute', async () => {
      const wrapper = await mountView([
        makeMembership('user-1', 'admin'),
        makeMembership('user-2', 'user'),
      ])

      const leaveBtn = wrapper.get('[data-testid="leave-group-btn"]')
      expect(leaveBtn.attributes('title')).toBe(
        'You are the last admin. Promote another member first, or delete the group.',
      )
    })

    it('suggests promoting a member when one is available', async () => {
      const wrapper = await mountView([
        makeMembership('user-1', 'admin'),
        makeMembership('user-2', 'user'),
      ])

      const notice = wrapper.get('[data-testid="last-admin-notice"]')
      expect(notice.text()).toContain('You are the last admin of this group')
      expect(notice.text()).toContain('Promote another member to admin before leaving')
      expect(notice.text()).toContain('delete the group below')
    })

    it('suggests only deletion when no promotable members exist', async () => {
      // Sole member is also sole admin — nobody to promote.
      const wrapper = await mountView([makeMembership('user-1', 'admin')])

      const notice = wrapper.get('[data-testid="last-admin-notice"]')
      expect(notice.text()).not.toContain('Promote another member')
      expect(notice.text()).toContain('To remove your access, delete the group below')
    })

    it('does not call leaveGroup when the disabled button is clicked', async () => {
      const wrapper = await mountView([
        makeMembership('user-1', 'admin'),
        makeMembership('user-2', 'user'),
      ])

      // A native disabled button swallows click events, but we assert the
      // service call explicitly — if someone later replaces the disabled
      // attribute with a :class/:aria-disabled-only pattern (common mistake
      // in a11y-focused rewrites), this test will flag the regression.
      await wrapper.get('[data-testid="leave-group-btn"]').trigger('click')
      expect(mockedGroupService.leaveGroup).not.toHaveBeenCalled()
    })
  })

  describe('when another admin exists', () => {
    it('keeps the Leave Group button enabled and shows no warning', async () => {
      const wrapper = await mountView([
        makeMembership('user-1', 'admin'),
        makeMembership('user-2', 'admin'),
      ])

      const leaveBtn = wrapper.get('[data-testid="leave-group-btn"]')
      expect(leaveBtn.attributes('disabled')).toBeUndefined()
      expect(leaveBtn.attributes('aria-disabled')).toBeUndefined()
      expect(wrapper.find('[data-testid="last-admin-notice"]').exists()).toBe(false)
    })
  })

  describe('when the current user is not an admin', () => {
    it('keeps the Leave Group button enabled even if there is only one admin total', async () => {
      // user-1 is a regular member; user-2 is the sole admin. The last-admin
      // check must be scoped by role — a non-admin leaving never endangers
      // the ≥1-admin invariant.
      const wrapper = await mountView([
        makeMembership('user-1', 'user'),
        makeMembership('user-2', 'admin'),
      ])

      const leaveBtn = wrapper.get('[data-testid="leave-group-btn"]')
      expect(leaveBtn.attributes('disabled')).toBeUndefined()
      expect(wrapper.find('[data-testid="last-admin-notice"]').exists()).toBe(false)
    })
  })
})

describe('GroupSettingsView — Remove Member action', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    // The viewer is user-1, an admin, so the member-actions block renders
    // for every row. The protection being tested here is about the *target*
    // of the Remove action — orthogonal to who is clicking.
    mockAuthStore.user = { id: 'user-1', name: 'Alice', email: 'alice@example.com' }
  })

  it('disables Remove for the sole admin (same user as the viewer)', async () => {
    const wrapper = await mountView([
      makeMembership('user-1', 'admin'),
      makeMembership('user-2', 'user'),
    ])

    const btn = wrapper.get('[data-testid="remove-member-btn-user-1"]')
    expect(btn.attributes('disabled')).toBeDefined()
    expect(btn.attributes('aria-disabled')).toBe('true')
    expect(btn.attributes('title')).toBe(
      'Cannot remove the last admin — promote another member first or delete the group.',
    )
  })

  it('enables Remove for non-admin members even when only one admin exists', async () => {
    const wrapper = await mountView([
      makeMembership('user-1', 'admin'),
      makeMembership('user-2', 'user'),
    ])

    const btn = wrapper.get('[data-testid="remove-member-btn-user-2"]')
    expect(btn.attributes('disabled')).toBeUndefined()
    expect(btn.attributes('aria-disabled')).toBeUndefined()
  })

  it('enables Remove for admins once a second admin exists', async () => {
    // Both admins are removable — dropping either keeps the admin count ≥ 1.
    const wrapper = await mountView([
      makeMembership('user-1', 'admin'),
      makeMembership('user-2', 'admin'),
    ])

    const btn1 = wrapper.get('[data-testid="remove-member-btn-user-1"]')
    const btn2 = wrapper.get('[data-testid="remove-member-btn-user-2"]')
    expect(btn1.attributes('disabled')).toBeUndefined()
    expect(btn2.attributes('disabled')).toBeUndefined()
  })

  it('does not call removeMember when the disabled Remove button is clicked', async () => {
    const wrapper = await mountView([
      makeMembership('user-1', 'admin'),
      makeMembership('user-2', 'user'),
    ])

    // Regression guard: if a future a11y refactor drops the native `disabled`
    // attribute in favor of class-only styling, the click handler would fire
    // and issue a doomed request that the backend rejects with 422. This
    // assertion locks the UI gate in place.
    await wrapper.get('[data-testid="remove-member-btn-user-1"]').trigger('click')
    expect(mockedGroupService.removeMember).not.toHaveBeenCalled()
  })
})
