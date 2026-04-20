import { describe, it, expect, vi, beforeEach } from 'vitest'
import authService from '../authService'

// Mock the api module used by authService
vi.mock('../api', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
  setCsrfToken: vi.fn(),
  clearCsrfToken: vi.fn(),
}))

import api from '../api'

const mockedApi = vi.mocked(api)

describe('authService', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // -----------------------------------------------------------------------
  // updateProfile
  // -----------------------------------------------------------------------

  describe('updateProfile', () => {
    it('calls PUT /api/v1/auth/me with the name payload', async () => {
      const serverUser = { id: 'u1', email: 'alice@example.com', name: 'Alice Updated' }
      mockedApi.put.mockResolvedValue({ data: serverUser })

      const result = await authService.updateProfile({ name: 'Alice Updated' })

      expect(mockedApi.put).toHaveBeenCalledWith(
        '/api/v1/auth/me',
        { name: 'Alice Updated' },
        {
          headers: {
            'Content-Type': 'application/json',
            'Accept': 'application/json',
          },
        },
      )
      expect(result).toEqual({
        id: 'u1',
        email: 'alice@example.com',
        name: 'Alice Updated',
        default_group_id: null,
      })
    })

    it('maps the server response to the User interface', async () => {
      const serverUser = {
        id: 'user-42',
        email: 'bob@example.com',
        name: 'Bob Smith',
        // Extra fields the server might return (e.g. is_active) should be ignored
        is_active: true,
        tenant_id: 'tenant-1',
      }
      mockedApi.put.mockResolvedValue({ data: serverUser })

      const result = await authService.updateProfile({ name: 'Bob Smith' })

      expect(result).toEqual({
        id: 'user-42',
        email: 'bob@example.com',
        name: 'Bob Smith',
        default_group_id: null,
      })
      // is_active and tenant_id should NOT be present in the mapped result
      expect(result).not.toHaveProperty('is_active')
      expect(result).not.toHaveProperty('tenant_id')
    })

    it('preserves default_group_id from the server response (#1263)', async () => {
      const serverUser = {
        id: 'user-42',
        email: 'bob@example.com',
        name: 'Bob Smith',
        default_group_id: '11111111-1111-1111-1111-111111111111',
      }
      mockedApi.put.mockResolvedValue({ data: serverUser })

      const result = await authService.updateProfile({
        name: 'Bob Smith',
        default_group_id: '11111111-1111-1111-1111-111111111111',
      })

      expect(result.default_group_id).toBe('11111111-1111-1111-1111-111111111111')
    })

    it('propagates API errors to the caller', async () => {
      const apiError = new Error('Network Error')
      mockedApi.put.mockRejectedValue(apiError)

      await expect(authService.updateProfile({ name: 'Alice' })).rejects.toThrow('Network Error')
    })
  })
})

