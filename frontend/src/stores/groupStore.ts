import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import groupService from '../services/groupService'
import { useAuthStore } from './authStore'
import type { LocationGroup, GroupMembership, GroupRole } from '../types/group'

// Primary key: full LocationGroup JSON snapshot. Stored so the header's
// GroupSelector can render the current group name/icon synchronously on
// page load, before fetchGroups() resolves — otherwise there's a visible
// "Select Group" flash between mount and the first API response (#1262).
const STORAGE_KEY_CURRENT_GROUP = 'inventario_current_group'
// Legacy slug-only key from the first persistence attempt. Read on load
// for back-compat with users whose localStorage predates the snapshot;
// cleared alongside the snapshot on logout. No longer written.
const STORAGE_KEY_GROUP_SLUG_LEGACY = 'currentGroupSlug'

function readStoredSnapshot(): LocationGroup | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY_CURRENT_GROUP)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    if (parsed && typeof parsed.id === 'string' && typeof parsed.slug === 'string') {
      return parsed as LocationGroup
    }
    return null
  } catch {
    return null
  }
}

function readLegacyStoredSlug(): string | null {
  return localStorage.getItem(STORAGE_KEY_GROUP_SLUG_LEGACY)
}

function writeStoredSnapshot(group: LocationGroup | null): void {
  if (group) {
    localStorage.setItem(STORAGE_KEY_CURRENT_GROUP, JSON.stringify(group))
  } else {
    localStorage.removeItem(STORAGE_KEY_CURRENT_GROUP)
    localStorage.removeItem(STORAGE_KEY_GROUP_SLUG_LEGACY)
  }
}

export const useGroupStore = defineStore('group', () => {
  // State
  const groups = ref<LocationGroup[]>([])
  // Seed currentGroup from localStorage synchronously so the selector
  // shows the active group's name immediately on page refresh. The
  // snapshot is later reconciled against the fresh /api/v1/groups
  // response in restoreFromStorage().
  const currentGroup = ref<LocationGroup | null>(readStoredSnapshot())
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
      writeStoredSnapshot(group)
      // Load membership info for the current user
      await loadCurrentMembership(group.id)
    }
  }

  function setCurrentGroupById(groupId: string): void {
    const group = groups.value.find((g) => g.id === groupId)
    if (group) {
      currentGroup.value = group
      writeStoredSnapshot(group)
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

  // restoreFromStorage reconciles the (possibly pre-seeded) currentGroup
  // against the freshly fetched groups list. It runs after fetchGroups()
  // and is responsible for three things:
  //   1. Preferring the user's last-selected group (by id, then by slug
  //      for back-compat with the legacy slug-only storage format).
  //   2. Falling back to the first available group when the stored one
  //      is no longer accessible — e.g. the user was removed from it,
  //      the group was deleted, or they switched accounts.
  //   3. Re-writing the snapshot with the authoritative server copy so
  //      stale fields (name/icon renamed from another device) don't
  //      linger in localStorage.
  async function restoreFromStorage(): Promise<void> {
    if (groups.value.length === 0) {
      currentGroup.value = null
      writeStoredSnapshot(null)
      return
    }

    const snapshot = readStoredSnapshot()
    const legacySlug = readLegacyStoredSlug()

    let match: LocationGroup | undefined
    if (snapshot) {
      match = groups.value.find((g) => g.id === snapshot.id)
    }
    if (!match && legacySlug) {
      match = groups.value.find((g) => g.slug === legacySlug)
    }

    const resolved = match ?? groups.value[0]
    currentGroup.value = resolved
    writeStoredSnapshot(resolved)
    await loadCurrentMembership(resolved.id)
  }

  async function createGroup(name: string, icon?: string, mainCurrency?: string): Promise<LocationGroup> {
    const group = await groupService.createGroup({
      name,
      icon,
      main_currency: mainCurrency?.trim() ? mainCurrency.trim() : undefined,
    })
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
      writeStoredSnapshot(updated)
    }
    const idx = groups.value.findIndex((g) => g.id === updated.id)
    if (idx >= 0) {
      groups.value[idx] = updated
    }
    return updated
  }

  // syncGroup writes a server-returned group into both currentGroup (when it
  // matches) and groups[]. Callers that already drove the service themselves
  // use this to keep the store consistent in one place, avoiding the trap of
  // reaching for setCurrentGroupById — which re-reads from the (pre-update)
  // groups[] entry and silently clobbers the fresh data.
  function syncGroup(group: LocationGroup): void {
    if (currentGroup.value && currentGroup.value.id === group.id) {
      currentGroup.value = group
      writeStoredSnapshot(group)
    }
    const idx = groups.value.findIndex((g) => g.id === group.id)
    if (idx >= 0) {
      groups.value[idx] = group
    }
  }

  function clearCurrentGroup(): void {
    currentGroup.value = null
    currentMembership.value = null
    writeStoredSnapshot(null)
  }

  function clearAll(): void {
    groups.value = []
    currentGroup.value = null
    currentMembership.value = null
    writeStoredSnapshot(null)
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
    syncGroup,
    clearCurrentGroup,
    clearAll,
  }
})
