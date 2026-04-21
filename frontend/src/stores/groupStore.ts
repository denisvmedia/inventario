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
// Slug mirror of the snapshot. Kept in sync with STORAGE_KEY_CURRENT_GROUP
// because the axios request interceptor in services/api.ts reads this key
// on every request to rewrite /api/v1/<resource>/... into the group-scoped
// /api/v1/g/{slug}/... form. Dropping this key silently breaks every
// group-scoped fetch (locations, commodities, ...) the moment a group is
// selected. Also serves as a back-compat read path for users whose
// localStorage predates the snapshot format.
const STORAGE_KEY_GROUP_SLUG_LEGACY = 'currentGroupSlug'

// isStoredLocationGroupSnapshot validates the full LocationGroup shape
// before we trust a localStorage blob enough to seed currentGroup. Accepting
// a partial object (id + slug only) would let a corrupted snapshot render
// the header with missing name/icon until reconciliation catches up.
function isStoredLocationGroupSnapshot(value: unknown): value is LocationGroup {
  if (!value || typeof value !== 'object') return false
  const candidate = value as Record<string, unknown>
  return (
    typeof candidate.id === 'string' &&
    typeof candidate.slug === 'string' &&
    typeof candidate.name === 'string' &&
    typeof candidate.icon === 'string' &&
    typeof candidate.status === 'string' &&
    typeof candidate.main_currency === 'string' &&
    typeof candidate.created_by === 'string' &&
    typeof candidate.created_at === 'string' &&
    typeof candidate.updated_at === 'string'
  )
}

function readStoredSnapshot(): LocationGroup | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY_CURRENT_GROUP)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    return isStoredLocationGroupSnapshot(parsed) ? parsed : null
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
    // Mirror the slug for the api.ts interceptor — see
    // STORAGE_KEY_GROUP_SLUG_LEGACY comment.
    localStorage.setItem(STORAGE_KEY_GROUP_SLUG_LEGACY, group.slug)
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
  // isInitialized flips true once the first fetchGroups + restoreFromStorage
  // completes after login. The router guard consults this flag (via
  // ensureLoaded) before deciding whether to redirect a zero-group user to
  // /no-group — otherwise a deep-link on a fresh page load would land on the
  // target route before hasGroups has any real data to check against.
  const isInitialized = ref(false)
  // Single-flight promise for ensureLoaded — two concurrent callers (e.g.
  // App.vue's onMounted + the router guard firing on the initial navigation)
  // must share one in-flight /api/v1/groups request instead of racing.
  let loadingPromise: Promise<void> | null = null

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

  // ensureLoaded runs fetchGroups + restoreFromStorage once per session and
  // then returns synchronously. Callers (router guard, App.vue bootstrap) can
  // await it on every invocation and trust that the store reflects the
  // server's current group set before they branch on hasGroups.
  async function ensureLoaded(): Promise<void> {
    if (isInitialized.value) return
    if (loadingPromise) return loadingPromise
    loadingPromise = (async () => {
      try {
        await fetchGroups()
        await restoreFromStorage()
        isInitialized.value = true
      } finally {
        loadingPromise = null
      }
    })()
    return loadingPromise
  }

  async function setCurrentGroup(slug: string, options: { persist?: boolean } = {}): Promise<void> {
    // persist defaults to true: user-initiated switches (GroupSelector click)
    // should remember the selection for the next cold start. Router-driven
    // syncs (issue #1289 Gap C — slug from the URL) pass `persist: false`
    // so two tabs with two different /g/<slug>/... URLs don't
    // ping-pong each other's localStorage entries.
    const persist = options.persist !== false
    const group = groups.value.find((g) => g.slug === slug)
    if (group) {
      currentGroup.value = group
      if (persist) {
        writeStoredSnapshot(group)
      }
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
  // against the freshly fetched groups list. It runs after fetchGroups() and
  // implements the priority chain spelled out in #1263:
  //   1. Last-selected group from localStorage (id match, then legacy slug) —
  //      preserves session continuity across refreshes (#1262).
  //   2. The user's explicit default_group_id preference — honoured when no
  //      session snapshot exists or when it points to a group the user has
  //      since lost access to (cleared cookies, new device).
  //   3. Deterministic fallback: first group the user created (by created_at
  //      ASC), else first group they were invited to. This fires on a fresh
  //      device with no preference set.
  //   4. Last resort: groups[0] — defensive, practically unreachable because
  //      step 3 already covers a non-empty groups list.
  // Whichever branch wins, the snapshot is rewritten with the authoritative
  // server copy so a rename from another device doesn't linger in localStorage.
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

    if (!match) {
      const defaultGroupID = useAuthStore().userDefaultGroupID
      if (defaultGroupID) {
        match = groups.value.find((g) => g.id === defaultGroupID)
      }
    }

    const resolved = match ?? pickFallbackGroup(groups.value, useAuthStore().user?.id ?? null)
    currentGroup.value = resolved
    writeStoredSnapshot(resolved)
    await loadCurrentMembership(resolved.id)
  }

  // pickFallbackGroup implements the "no preference, no snapshot" branch of
  // #1263: prefer the oldest group the user created, otherwise the oldest
  // group they were invited to. Sorting by created_at ASC keeps the choice
  // deterministic so the same user lands in the same group on every fresh
  // device — which is the whole point of a fallback rule.
  function pickFallbackGroup(list: LocationGroup[], userId: string | null): LocationGroup {
    const byCreatedAtAsc = (a: LocationGroup, b: LocationGroup): number =>
      a.created_at.localeCompare(b.created_at)
    if (userId) {
      const created = list.filter((g) => g.created_by === userId).sort(byCreatedAtAsc)
      if (created.length > 0) return created[0]
      const invited = list.filter((g) => g.created_by !== userId).sort(byCreatedAtAsc)
      if (invited.length > 0) return invited[0]
    }
    return [...list].sort(byCreatedAtAsc)[0] ?? list[0]
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
    isInitialized.value = false
    writeStoredSnapshot(null)
  }

  return {
    // State
    groups,
    currentGroup,
    currentMembership,
    isLoading,
    error,
    isInitialized,

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
    ensureLoaded,
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
