import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import groupService from '../services/groupService'
import { useAuthStore } from './authStore'
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

  // currentGroupMainCurrency is the valuation currency of the active group.
  // Moved here (from the user-scoped settingsStore) in #1248 — valuation is a
  // group-level property so a user who toggles between a CZK group and a USD
  // group sees the right currency on each.
  const currentGroupMainCurrency = computed(() => currentGroup.value?.main_currency || '')

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
    const userId = useAuthStore().user?.id
    if (!userId) {
      currentMembership.value = null
      return
    }
    try {
      const members = await groupService.listMembers(groupId)
      currentMembership.value = members.find((m) => m.member_user_id === userId) ?? null
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
        // Refresh the user's membership for this group so currentRole /
        // isGroupAdmin / isGroupUser getters aren't stale after a reload.
        await loadCurrentMembership(group.id)
        return
      }
    }
    // If stored slug not found, select the first group
    if (groups.value.length > 0) {
      const group = groups.value[0]
      currentGroup.value = group
      localStorage.setItem(STORAGE_KEY_GROUP_SLUG, group.slug)
      await loadCurrentMembership(group.id)
    }
  }

  async function createGroup(name: string, icon?: string): Promise<LocationGroup> {
    const group = await groupService.createGroup({ name, icon })
    groups.value.push(group)
    return group
  }

  async function updateCurrentGroup(name: string, icon?: string): Promise<void> {
    if (!currentGroup.value) return
    await updateGroupById(currentGroup.value.id, name, icon)
  }

  // updateGroupById centralizes the "save + sync local store" workflow so that
  // views (e.g. GroupSettingsView) don't need to poke at groupStore.currentGroup
  // and groupStore.groups[] directly after calling the service.
  async function updateGroupById(groupId: string, name: string, icon?: string): Promise<LocationGroup> {
    const updated = await groupService.updateGroup(groupId, { name, icon })
    if (currentGroup.value && currentGroup.value.id === updated.id) {
      currentGroup.value = updated
    }
    const idx = groups.value.findIndex((g) => g.id === updated.id)
    if (idx >= 0) {
      groups.value[idx] = updated
    }
    return updated
  }

  // updateCurrentGroupMainCurrency changes the valuation currency of the
  // active group. The backend reprices the group's commodities — exchange
  // rate is optional (falls back to the built-in rate table server-side).
  async function updateCurrentGroupMainCurrency(currency: string, exchangeRate?: string): Promise<void> {
    if (!currentGroup.value) {
      throw new Error('No active group to update')
    }
    const group = currentGroup.value
    const updated = await groupService.updateGroup(group.id, {
      name: group.name,
      icon: group.icon,
      main_currency: currency,
      exchange_rate: exchangeRate?.trim() ? exchangeRate.trim() : undefined,
    })
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
    currentGroupMainCurrency,
    groupApiBaseUrl,

    // Actions
    fetchGroups,
    setCurrentGroup,
    setCurrentGroupById,
    setCurrentMembership,
    restoreFromStorage,
    createGroup,
    updateCurrentGroup,
    updateGroupById,
    updateCurrentGroupMainCurrency,
    clearCurrentGroup,
    clearAll,
  }
})
