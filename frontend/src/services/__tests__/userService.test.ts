import { describe, it, expect, vi, beforeEach } from 'vitest'
import userService from '../userService'

// Mock the api module used by userService
vi.mock('../api', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

// Import after mocking so we get the mocked version
import api from '../api'

const mockedApi = vi.mocked(api)

describe('userService', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // -----------------------------------------------------------------------
  // buildParams (tested via listUsers URL construction)
  // -----------------------------------------------------------------------

  describe('listUsers – buildParams', () => {
    it('calls GET /api/v1/users without query string when no params', async () => {
      mockedApi.get.mockResolvedValue({ data: { users: [], total: 0, page: 1, per_page: 20, total_pages: 1 } })

      await userService.listUsers()

      expect(mockedApi.get).toHaveBeenCalledWith('/api/v1/users', expect.any(Object))
    })

    it('appends role filter', async () => {
      mockedApi.get.mockResolvedValue({ data: { users: [], total: 0, page: 1, per_page: 20, total_pages: 1 } })

      await userService.listUsers({ role: 'admin' })

      expect(mockedApi.get).toHaveBeenCalledWith(
        expect.stringContaining('role=admin'),
        expect.any(Object),
      )
    })

    it('appends active=true when active is true', async () => {
      mockedApi.get.mockResolvedValue({ data: { users: [], total: 0, page: 1, per_page: 20, total_pages: 1 } })

      await userService.listUsers({ active: true })

      expect(mockedApi.get).toHaveBeenCalledWith(
        expect.stringContaining('active=true'),
        expect.any(Object),
      )
    })

    it('appends active=false when active is false', async () => {
      mockedApi.get.mockResolvedValue({ data: { users: [], total: 0, page: 1, per_page: 20, total_pages: 1 } })

      await userService.listUsers({ active: false })

      expect(mockedApi.get).toHaveBeenCalledWith(
        expect.stringContaining('active=false'),
        expect.any(Object),
      )
    })

    it('omits active param when active is null', async () => {
      mockedApi.get.mockResolvedValue({ data: { users: [], total: 0, page: 1, per_page: 20, total_pages: 1 } })

      await userService.listUsers({ active: null })

      const url: string = mockedApi.get.mock.calls[0][0]
      expect(url).not.toContain('active')
    })

    it('appends page and per_page', async () => {
      mockedApi.get.mockResolvedValue({ data: { users: [], total: 0, page: 2, per_page: 10, total_pages: 5 } })

      await userService.listUsers({ page: 2, per_page: 10 })

      const url: string = mockedApi.get.mock.calls[0][0]
      expect(url).toContain('page=2')
      expect(url).toContain('per_page=10')
    })

    it('appends search term', async () => {
      mockedApi.get.mockResolvedValue({ data: { users: [], total: 0, page: 1, per_page: 20, total_pages: 1 } })

      await userService.listUsers({ search: 'alice' })

      expect(mockedApi.get).toHaveBeenCalledWith(
        expect.stringContaining('search=alice'),
        expect.any(Object),
      )
    })
  })

  // -----------------------------------------------------------------------
  // getUser
  // -----------------------------------------------------------------------

  describe('getUser', () => {
    it('calls GET /api/v1/users/:id', async () => {
      const mockUser = { id: 'user-1', email: 'a@b.com', name: 'Alice', role: 'user', is_active: true }
      mockedApi.get.mockResolvedValue({ data: mockUser })

      const result = await userService.getUser('user-1')

      expect(mockedApi.get).toHaveBeenCalledWith('/api/v1/users/user-1', expect.any(Object))
      expect(result).toEqual(mockUser)
    })
  })

  // -----------------------------------------------------------------------
  // createUser
  // -----------------------------------------------------------------------

  describe('createUser', () => {
    it('calls POST /api/v1/users with the correct payload', async () => {
      const payload = { email: 'new@example.com', password: 'Pass123!', name: 'New', role: 'user' as const, is_active: true }
      const created = { id: 'new-id', ...payload }
      mockedApi.post.mockResolvedValue({ data: created })

      const result = await userService.createUser(payload)

      expect(mockedApi.post).toHaveBeenCalledWith('/api/v1/users', payload, expect.any(Object))
      expect(result).toEqual(created)
    })
  })

  // -----------------------------------------------------------------------
  // updateUser
  // -----------------------------------------------------------------------

  describe('updateUser', () => {
    it('calls PUT /api/v1/users/:id with the update payload', async () => {
      const patch = { name: 'Updated Name' }
      const updated = { id: 'user-1', email: 'a@b.com', name: 'Updated Name', role: 'user', is_active: true }
      mockedApi.put.mockResolvedValue({ data: updated })

      const result = await userService.updateUser('user-1', patch)

      expect(mockedApi.put).toHaveBeenCalledWith('/api/v1/users/user-1', patch, expect.any(Object))
      expect(result).toEqual(updated)
    })
  })

  // -----------------------------------------------------------------------
  // deactivateUser
  // -----------------------------------------------------------------------

  describe('deactivateUser', () => {
    it('calls DELETE /api/v1/users/:id', async () => {
      mockedApi.delete.mockResolvedValue({ data: { message: 'User deactivated successfully' } })

      const result = await userService.deactivateUser('user-1')

      expect(mockedApi.delete).toHaveBeenCalledWith('/api/v1/users/user-1', expect.any(Object))
      expect(result).toEqual({ message: 'User deactivated successfully' })
    })
  })
})

