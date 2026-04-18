import { describe, it, expect, vi, beforeEach } from 'vitest'
import groupService from '../groupService'

vi.mock('../api', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    patch: vi.fn(),
    delete: vi.fn(),
  },
}))

// Import after mocking so we get the mocked version.
import api from '../api'

const mockedApi = vi.mocked(api)

function groupResponse(overrides: Record<string, unknown> = {}) {
  return {
    data: {
      data: {
        id: 'grp-1',
        type: 'groups',
        attributes: { name: 'My Group', icon: '📦', slug: 'abc', status: 'active', ...overrides },
      },
    },
  }
}

function groupListResponse(items: Array<{ id: string; name: string }>) {
  return {
    data: {
      data: items.map((it) => ({ id: it.id, type: 'groups', attributes: { name: it.name } })),
    },
  }
}

function membershipResponse(id: string, member_user_id: string, role: 'admin' | 'user') {
  return {
    data: {
      data: { id, type: 'memberships', attributes: { group_id: 'grp-1', member_user_id, role } },
    },
  }
}

describe('groupService', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // --- Group CRUD ---

  describe('listGroups', () => {
    it('GETs /api/v1/groups and flattens JSON:API items into {id, ...attributes}', async () => {
      mockedApi.get.mockResolvedValue(groupListResponse([
        { id: 'g1', name: 'A' },
        { id: 'g2', name: 'B' },
      ]))

      const groups = await groupService.listGroups()

      expect(mockedApi.get).toHaveBeenCalledWith('/api/v1/groups')
      expect(groups).toEqual([
        { id: 'g1', name: 'A' },
        { id: 'g2', name: 'B' },
      ])
    })
  })

  describe('getGroup', () => {
    it('GETs /api/v1/groups/:id and flattens the returned resource', async () => {
      mockedApi.get.mockResolvedValue(groupResponse())

      const group = await groupService.getGroup('grp-1')

      expect(mockedApi.get).toHaveBeenCalledWith('/api/v1/groups/grp-1')
      expect(group).toEqual({ id: 'grp-1', name: 'My Group', icon: '📦', slug: 'abc', status: 'active' })
    })
  })

  describe('createGroup', () => {
    it('POSTs a JSON:API envelope with type=groups and returns the created group', async () => {
      mockedApi.post.mockResolvedValue(groupResponse({ name: 'New', icon: '🏠' }))

      const group = await groupService.createGroup({ name: 'New', icon: '🏠' })

      expect(mockedApi.post).toHaveBeenCalledWith('/api/v1/groups', {
        data: {
          type: 'groups',
          attributes: { name: 'New', icon: '🏠' },
        },
      })
      expect(group.id).toBe('grp-1')
      expect(group.name).toBe('New')
    })
  })

  describe('updateGroup', () => {
    it('PATCHes /api/v1/groups/:id with a JSON:API envelope that carries the id', async () => {
      mockedApi.patch.mockResolvedValue(groupResponse({ name: 'Renamed' }))

      const group = await groupService.updateGroup('grp-1', { name: 'Renamed', icon: '📦' })

      expect(mockedApi.patch).toHaveBeenCalledWith('/api/v1/groups/grp-1', {
        data: {
          type: 'groups',
          id: 'grp-1',
          attributes: { name: 'Renamed', icon: '📦' },
        },
      })
      expect(group.name).toBe('Renamed')
    })
  })

  describe('deleteGroup', () => {
    it('DELETEs /api/v1/groups/:id with a plain JSON body carrying confirm_word (no JSON:API envelope)', async () => {
      mockedApi.delete.mockResolvedValue({ data: null })

      await groupService.deleteGroup('grp-1', { confirm_word: 'My Group' })

      expect(mockedApi.delete).toHaveBeenCalledWith(
        '/api/v1/groups/grp-1',
        { data: { confirm_word: 'My Group' } },
      )
    })
  })

  // --- Members ---

  describe('listMembers', () => {
    it('GETs /api/v1/groups/:id/members and flattens each membership', async () => {
      mockedApi.get.mockResolvedValue({
        data: {
          data: [
            { id: 'm1', type: 'memberships', attributes: { group_id: 'grp-1', member_user_id: 'u1', role: 'admin' } },
            { id: 'm2', type: 'memberships', attributes: { group_id: 'grp-1', member_user_id: 'u2', role: 'user' } },
          ],
        },
      })

      const members = await groupService.listMembers('grp-1')

      expect(mockedApi.get).toHaveBeenCalledWith('/api/v1/groups/grp-1/members')
      expect(members).toEqual([
        { id: 'm1', group_id: 'grp-1', member_user_id: 'u1', role: 'admin' },
        { id: 'm2', group_id: 'grp-1', member_user_id: 'u2', role: 'user' },
      ])
    })
  })

  describe('removeMember', () => {
    it('DELETEs /api/v1/groups/:groupId/members/:userId', async () => {
      mockedApi.delete.mockResolvedValue({ data: null })

      await groupService.removeMember('grp-1', 'u2')

      expect(mockedApi.delete).toHaveBeenCalledWith('/api/v1/groups/grp-1/members/u2')
    })
  })

  describe('updateMemberRole', () => {
    it('PATCHes with a JSON:API envelope carrying only attributes (no top-level type/id)', async () => {
      mockedApi.patch.mockResolvedValue(membershipResponse('m2', 'u2', 'admin'))

      const m = await groupService.updateMemberRole('grp-1', 'u2', { role: 'admin' })

      expect(mockedApi.patch).toHaveBeenCalledWith('/api/v1/groups/grp-1/members/u2', {
        data: {
          attributes: { role: 'admin' },
        },
      })
      expect(m.role).toBe('admin')
    })
  })

  describe('leaveGroup', () => {
    it('POSTs to /api/v1/groups/:id/leave with no body', async () => {
      mockedApi.post.mockResolvedValue({ data: null })

      await groupService.leaveGroup('grp-1')

      expect(mockedApi.post).toHaveBeenCalledWith('/api/v1/groups/grp-1/leave')
    })
  })

  // --- Invites ---

  describe('createInvite', () => {
    it('POSTs /api/v1/groups/:id/invites with no body and flattens the returned invite', async () => {
      mockedApi.post.mockResolvedValue({
        data: {
          data: { id: 'inv-1', type: 'invites', attributes: { token: 'tk', expires_at: '2026-01-01T00:00:00Z' } },
        },
      })

      const invite = await groupService.createInvite('grp-1')

      expect(mockedApi.post).toHaveBeenCalledWith('/api/v1/groups/grp-1/invites')
      expect(invite).toEqual({ id: 'inv-1', token: 'tk', expires_at: '2026-01-01T00:00:00Z' })
    })
  })

  describe('listInvites', () => {
    it('GETs /api/v1/groups/:id/invites and flattens each invite', async () => {
      mockedApi.get.mockResolvedValue({
        data: {
          data: [
            { id: 'inv-1', type: 'invites', attributes: { token: 't1', expires_at: 'x' } },
          ],
        },
      })

      const invites = await groupService.listInvites('grp-1')

      expect(mockedApi.get).toHaveBeenCalledWith('/api/v1/groups/grp-1/invites')
      expect(invites).toEqual([{ id: 'inv-1', token: 't1', expires_at: 'x' }])
    })
  })

  describe('revokeInvite', () => {
    it('DELETEs /api/v1/groups/:groupId/invites/:inviteId', async () => {
      mockedApi.delete.mockResolvedValue({ data: null })

      await groupService.revokeInvite('grp-1', 'inv-1')

      expect(mockedApi.delete).toHaveBeenCalledWith('/api/v1/groups/grp-1/invites/inv-1')
    })
  })

  describe('getInviteInfo', () => {
    it('GETs /api/v1/invites/:token and returns attributes directly (no id wrap)', async () => {
      mockedApi.get.mockResolvedValue({
        data: {
          data: {
            type: 'invite_info',
            attributes: { group_name: 'G', group_icon: '📦', expired: false, used: false },
          },
        },
      })

      const info = await groupService.getInviteInfo('tk-123')

      expect(mockedApi.get).toHaveBeenCalledWith('/api/v1/invites/tk-123')
      expect(info).toEqual({ group_name: 'G', group_icon: '📦', expired: false, used: false })
    })
  })

  describe('acceptInvite', () => {
    it('POSTs /api/v1/invites/:token/accept and flattens the returned membership', async () => {
      mockedApi.post.mockResolvedValue(membershipResponse('m-new', 'u1', 'user'))

      const m = await groupService.acceptInvite('tk-123')

      expect(mockedApi.post).toHaveBeenCalledWith('/api/v1/invites/tk-123/accept')
      expect(m).toEqual({ id: 'm-new', group_id: 'grp-1', member_user_id: 'u1', role: 'user' })
    })
  })
})
