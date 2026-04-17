import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import groupService from '../services/groupService'
import type { LocationGroup, GroupMembership, GroupRole } from '../types/group'

const STORAGE_KEY_GROUP_SLUG = 'currentGroupSlug'

export const useGroupStore = defineStore('group', () => {
  // State
  const groups = ref<LocationGroup[]>([])
  const currentGroup = ref<LocationGroup | null>(null)
  const currentMembership = ref<GroupMembership | null>(null)
  const isLoading = ref(false)
  const error = ref<string | null>(null)

  // Getters
  const hasGroups = computed(() => groups.value.length > 0)
  const currentGroupSlug = computed(() => currentGroup.value?.slug || null)
  const currentGroupId = computed(() => currentGroup.value?.id || null)
  const currentGroupName = computed(() => currentGroup.value?.name || null)
  const currentGroupIcon = computed(() => currentGroup.value?.icon || null)

  const currentRole = computed<GroupRole | null>(() => currentMembership.value?.role || null)
  const isGroupAdmin = computed(() => currentMembership.value?.role === 'admin')
  const isGroupUser = computed(() => currentMembership.value?.role === 'user')

  /**
   * Returns the API base URL for group-scoped data endpoints.
   * E.g. "/api/v1/g/abc123xyz" — append resource path after this.
   */
  const groupApiBaseUrl = computed(() => {
    if (!currentGroup.value) return null
    return `/api/v1/g/${currentGroup.value.slug}`
  })

  // Actions

  async function fetchGroups(): Promise<void> {
    isLoading.value = true
    error.value = null
    try {
      groups.value = await groupService.listGroups()
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to load groups'
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function setCurrentGroup(slug: string): Promise<void> {
    const group = groups.value.find((g) => g.slug === slug)
    if (group) {
      currentGroup.value = group
      localStorage.setItem(STORAGE_KEY_GROUP_SLUG, slug)
      // Load membership info for the current user
      await loadCurrentMembership(group.id)
    }
  }

  function setCurrentGroupById(groupId: string): void {
    const group = groups.value.find((g) => g.id === groupId)
    if (group) {
      currentGroup.value = group
      localStorage.setItem(STORAGE_KEY_GROUP_SLUG, group.slug)
    }
  }

  async function loadCurrentMembership(groupId: string): Promise<void> {
    try {
      const members = await groupService.listMembers(groupId)
      // The current user's membership — the backend only returns members
      // visible to the current tenant, so we pick the one matching our user ID.
      // For now, since we don't have the user ID here, we store all members
      // and rely on the component to filter. Alternatively, we could add
      // a dedicated endpoint.
      // Workaround: the store consumer can call setCurrentMembership directly.
      if (members.length > 0) {
        // We'll set it from the component layer where we have user context
        currentMembership.value = null
      }
    } catch {
      currentMembership.value = null
    }
  }

  function setCurrentMembership(membership: GroupMembership | null): void {
    currentMembership.value = membership
  }

  async function restoreFromStorage(): Promise<void> {
    const storedSlug = localStorage.getItem(STORAGE_KEY_GROUP_SLUG)
    if (storedSlug && groups.value.length > 0) {
      const group = groups.value.find((g) => g.slug === storedSlug)
      if (group) {
        currentGroup.value = group
        return
      }
    }
    // If stored slug not found, select the first group
    if (groups.value.length > 0) {
      currentGroup.value = groups.value[0]
      localStorage.setItem(STORAGE_KEY_GROUP_SLUG, groups.value[0].slug)
    }
  }

  async function createGroup(name: string, icon?: string): Promise<LocationGroup> {
    const group = await groupService.createGroup({ name, icon })
    groups.value.push(group)
    return group
  }

  async function updateCurrentGroup(name: string, icon?: string): Promise<void> {
    if (!currentGroup.value) return
    const updated = await groupService.updateGroup(currentGroup.value.id, { name, icon })
    currentGroup.value = updated
    const idx = groups.value.findIndex((g) => g.id === updated.id)
    if (idx >= 0) {
      groups.value[idx] = updated
    }
  }

  function clearCurrentGroup(): void {
    currentGroup.value = null
    currentMembership.value = null
    localStorage.removeItem(STORAGE_KEY_GROUP_SLUG)
  }

  function clearAll(): void {
    groups.value = []
    currentGroup.value = null
    currentMembership.value = null
    localStorage.removeItem(STORAGE_KEY_GROUP_SLUG)
  }

  return {
    // State
    groups,
    currentGroup,
    currentMembership,
    isLoading,
    error,

    // Getters
    hasGroups,
    currentGroupSlug,
    currentGroupId,
    currentGroupName,
    currentGroupIcon,
    currentRole,
    isGroupAdmin,
    isGroupUser,
    groupApiBaseUrl,

    // Actions
    fetchGroups,
    setCurrentGroup,
    setCurrentGroupById,
    setCurrentMembership,
    restoreFromStorage,
    createGroup,
    updateCurrentGroup,
    clearCurrentGroup,
    clearAll,
  }
})
