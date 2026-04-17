import { useGroupStore } from '../stores/groupStore'

const BASE_API_URL = '/api/v1'

/**
 * Returns the API base URL for group-scoped data endpoints.
 * When a group is active: "/api/v1/g/{slug}"
 * When no group is active: "/api/v1" (fallback for transition period)
 */
export function getGroupApiUrl(): string {
  const groupStore = useGroupStore()
  if (groupStore.currentGroupSlug) {
    return `${BASE_API_URL}/g/${groupStore.currentGroupSlug}`
  }
  return BASE_API_URL
}

/**
 * Returns the non-group-scoped API base URL.
 * Always "/api/v1" — for endpoints that are not group-scoped
 * (auth, groups management, invites, etc.)
 */
export function getApiUrl(): string {
  return BASE_API_URL
}
