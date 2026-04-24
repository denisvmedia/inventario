import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import groupService from '../services/groupService'
import { useAuthStore } from './authStore'
import type { LocationGroup, GroupMembership, GroupRole } from '../types/group'

export const useGroupStore = defineStore('group', () => {
  // State
  const groups = ref<LocationGroup[]>([])
  // currentGroup starts null: the router's /g/:groupSlug/ param is the
  // authoritative source of truth for the active group once routing
  // resolves, and restoreFromPreference() seeds the store for non-group
  // routes using the user's default_group_id preference.
  const currentGroup = ref<LocationGroup | null>(null)
  const currentMembership = ref<GroupMembership | null>(null)
  const isLoading = ref(false)
  const error = ref<string | null>(null)
  // isInitialized flips true once the first fetchGroups + restoreFromPreference
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

  // ensureLoaded runs fetchGroups + restoreFromPreference once per session and
  // then returns synchronously. Callers (router guard, App.vue bootstrap) can
  // await it on every invocation and trust that the store reflects the
  // server's current group set before they branch on hasGroups.
  async function ensureLoaded(): Promise<void> {
    if (isInitialized.value) return
    if (loadingPromise) return loadingPromise
    loadingPromise = (async () => {
      try {
        await fetchGroups()
        await restoreFromPreference()
        isInitialized.value = true
      } finally {
        loadingPromise = null
      }
    })()
    return loadingPromise
  }

  async function setCurrentGroup(slug: string): Promise<void> {
    const group = groups.value.find((g) => g.slug === slug)
    if (group) {
      currentGroup.value = group
      await loadCurrentMembership(group.id)
    }
  }

  function setCurrentGroupById(groupId: string): void {
    const group = groups.value.find((g) => g.id === groupId)
    if (group) {
      currentGroup.value = group
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

  // restoreFromPreference seeds currentGroup on a cold start when the URL
  // has no group slug (e.g. / or /profile). The priority chain is:
  //   1. user.default_group_id (#1263) — the cross-device preference.
  //   2. pickFallbackGroup — the oldest group the user created, else the
  //      oldest group they were invited to.
  //   3. groups[0] — defensive, practically unreachable when step 2 is live.
  // When the URL *does* carry a group slug, the router guard is responsible
  // for calling setCurrentGroup(slugFromURL) instead; this function doesn't
  // need to know about the URL.
  async function restoreFromPreference(): Promise<void> {
    if (groups.value.length === 0) {
      currentGroup.value = null
      return
    }

    let match: LocationGroup | undefined
    const defaultGroupID = useAuthStore().userDefaultGroupID
    if (defaultGroupID) {
      match = groups.value.find((g) => g.id === defaultGroupID)
    }

    const resolved = match ?? pickFallbackGroup(groups.value, useAuthStore().user?.id ?? null)
    currentGroup.value = resolved
    await loadCurrentMembership(resolved.id)
  }

  // pickFallbackGroup implements the "no preference" branch of #1263:
  // prefer the oldest group the user created, otherwise the oldest group
  // they were invited to. Sorting by created_at ASC keeps the choice
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
    }
    const idx = groups.value.findIndex((g) => g.id === group.id)
    if (idx >= 0) {
      groups.value[idx] = group
    }
  }

  function clearCurrentGroup(): void {
    currentGroup.value = null
    currentMembership.value = null
  }

  function clearAll(): void {
    groups.value = []
    currentGroup.value = null
    currentMembership.value = null
    isInitialized.value = false
  }

  // groupPath prefixes a data subpath with the active group's /g/<slug>/
  // scope. It is the single source of truth for building scoped URLs in
  // views, services, and router.push() sites — replaces the ad-hoc flat
  // paths that used to rely on the router's legacyFlatDataRoute rewriter.
  // When no group is active the router guard bounces the click to
  // /no-group, so returning the unscoped subpath here is only a safety
  // net for code paths that render nav links before a group is resolved.
  function groupPath(subpath: string): string {
    const slug = currentGroupSlug.value
    const normalized = subpath.startsWith('/') ? subpath : `/${subpath}`
    if (!slug) return normalized
    return `/g/${encodeURIComponent(slug)}${normalized}`
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
    restoreFromPreference,
    createGroup,
    updateCurrentGroup,
    updateGroupById,
    syncGroup,
    clearCurrentGroup,
    clearAll,
    groupPath,
  }
})

