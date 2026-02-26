import api from './api'
import type {
  AdminUser,
  AdminUserCreateRequest,
  AdminUserUpdateRequest,
  AdminUserListResponse,
  AdminUserListParams,
} from '../types'

const BASE_URL = '/api/v1/users'

/**
 * Build a URLSearchParams object from the given list params, omitting undefined/null values.
 */
function buildParams(params?: AdminUserListParams): URLSearchParams {
  const q = new URLSearchParams()
  if (!params) return q
  if (params.role) q.set('role', params.role)
  if (params.active !== undefined && params.active !== null) {
    q.set('active', params.active ? 'true' : 'false')
  }
  if (params.search) q.set('search', params.search)
  if (params.page) q.set('page', String(params.page))
  if (params.per_page) q.set('per_page', String(params.per_page))
  return q
}

const userService = {
  /**
   * List users in the current admin's tenant.
   * Requires the requesting user to have the admin role.
   */
  async listUsers(params?: AdminUserListParams): Promise<AdminUserListResponse> {
    const q = buildParams(params)
    const url = q.toString() ? `${BASE_URL}?${q}` : BASE_URL
    const response = await api.get(url, {
      headers: { 'Accept': 'application/json' },
    })
    return response.data
  },

  /**
   * Retrieve a single user by ID.
   * Requires the requesting user to have the admin role.
   */
  async getUser(id: string): Promise<AdminUser> {
    const response = await api.get(`${BASE_URL}/${id}`, {
      headers: { 'Accept': 'application/json' },
    })
    return response.data
  },

  /**
   * Create a new user in the current admin's tenant.
   * Requires the requesting user to have the admin role.
   */
  async createUser(data: AdminUserCreateRequest): Promise<AdminUser> {
    const response = await api.post(BASE_URL, data, {
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
      },
    })
    return response.data
  },

  /**
   * Update an existing user.
   * Requires the requesting user to have the admin role.
   */
  async updateUser(id: string, data: AdminUserUpdateRequest): Promise<AdminUser> {
    const response = await api.put(`${BASE_URL}/${id}`, data, {
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
      },
    })
    return response.data
  },

  /**
   * Deactivate a user (set is_active = false).
   * Admins cannot deactivate their own account.
   * Requires the requesting user to have the admin role.
   */
  async deactivateUser(id: string): Promise<{ message: string }> {
    const response = await api.delete(`${BASE_URL}/${id}`, {
      headers: { 'Accept': 'application/json' },
    })
    return response.data
  },
}

export default userService

