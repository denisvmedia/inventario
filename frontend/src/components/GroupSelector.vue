<template>
  <div ref="selectorRef" class="group-selector">
    <button
      type="button"
      class="group-selector__trigger"
      :aria-expanded="isOpen"
      aria-haspopup="true"
      @click.stop="isOpen = !isOpen"
    >
      <span v-if="groupStore.currentGroupIcon" class="group-selector__icon">{{ groupStore.currentGroupIcon }}</span>
      <span class="group-selector__name">{{ groupStore.currentGroupName || 'Select Group' }}</span>
      <span class="group-selector__caret">&#9662;</span>
    </button>
    <div v-if="isOpen" class="group-selector__dropdown" role="menu" @keydown.esc="isOpen = false">
      <button
        v-for="group in groupStore.groups"
        :key="group.id"
        type="button"
        class="group-selector__item"
        :class="{ 'group-selector__item--active': group.id === groupStore.currentGroupId }"
        role="menuitem"
        @click="selectGroup(group)"
      >
        <span v-if="group.icon" class="group-selector__item-icon">{{ group.icon }}</span>
        <span class="group-selector__item-name">{{ group.name }}</span>
      </button>
      <div class="group-selector__divider" />
      <button type="button" class="group-selector__item group-selector__item--action" role="menuitem" @click="openCreateDialog">
        + Create new group
      </button>
      <button
        v-if="groupStore.currentGroup"
        type="button"
        class="group-selector__item group-selector__item--action"
        role="menuitem"
        @click="openGroupSettings"
      >
        Group settings
      </button>
    </div>
    <p v-if="preferenceError" class="group-selector__error" role="alert" data-testid="group-selector-error">
      {{ preferenceError }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useGroupStore } from '@/stores/groupStore'
import { useAuthStore } from '@/stores/authStore'
import type { LocationGroup } from '@/types/group'

const groupStore = useGroupStore()
const authStore = useAuthStore()
const router = useRouter()
const route = useRoute()
const isOpen = ref(false)
const selectorRef = ref<HTMLElement | null>(null)
const preferenceError = ref<string | null>(null)

// Debounce the PUT /auth/me that persists the user's "remember this group"
// preference. Rapid clicks through the dropdown collapse into one round
// trip, and the final click is always the one that wins.
let preferenceTimer: ReturnType<typeof setTimeout> | null = null
const PREFERENCE_DEBOUNCE_MS = 400

// Switch to another group by navigating to its /g/<slug>/... URL, rebuilding
// the current subpath under the new slug so the user stays on the same kind
// of screen (commodities → commodities, exports → exports…). Writing the
// slug into the URL makes two tabs with two different groups independent
// (#1289 Gap C), and the "remember this group for next time" follows the
// user across devices via user.default_group_id (#1300).
async function selectGroup(group: LocationGroup) {
  isOpen.value = false
  if (groupStore.currentGroupSlug === group.slug) {
    return
  }

  // Preserve the current subpath after the /g/<slug> prefix when possible,
  // so the user stays on the same view kind. Fall back to the group's root
  // path when the current route isn't group-scoped (profile, no-group, …).
  const currentSlug = typeof route.params.groupSlug === 'string' ? route.params.groupSlug : ''
  let subpath = ''
  if (currentSlug) {
    const marker = `/g/${encodeURIComponent(currentSlug)}`
    if (route.path.startsWith(marker)) {
      subpath = route.path.slice(marker.length)
    }
  }
  preferenceError.value = null
  const targetPath = `/g/${encodeURIComponent(group.slug)}${subpath || '/'}`
  await router.push(targetPath)
  schedulePreferenceUpdate(group.id)
}

function schedulePreferenceUpdate(groupId: string): void {
  if (preferenceTimer) {
    clearTimeout(preferenceTimer)
  }
  preferenceTimer = setTimeout(() => {
    preferenceTimer = null
    void persistPreference(groupId)
  }, PREFERENCE_DEBOUNCE_MS)
}

async function persistPreference(groupId: string): Promise<void> {
  const name = authStore.user?.name
  if (!name) return
  try {
    await authStore.updateProfile({ name, default_group_id: groupId })
  } catch (err: unknown) {
    const e = err as { response?: { status?: number; data?: { errors?: Array<{ detail?: string }> } } }
    const detail = e.response?.data?.errors?.[0]?.detail
    preferenceError.value = detail ?? 'Could not save group preference. Your selection is active for this session only.'
  }
}

function openCreateDialog() {
  isOpen.value = false
  router.push({ name: 'group-create' })
}

function openGroupSettings() {
  isOpen.value = false
  if (groupStore.currentGroupId) {
    router.push({ name: 'group-settings', params: { groupId: groupStore.currentGroupId } })
  }
}

function handleClickOutside(event: MouseEvent) {
  if (selectorRef.value && !selectorRef.value.contains(event.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  // A still-pending debounced PUT /auth/me would otherwise fire after the
  // component (and likely the whole route) is gone — the user-visible
  // error state couldn't be surfaced anymore and the write may race a
  // logout that's about to invalidate the token.
  if (preferenceTimer) {
    clearTimeout(preferenceTimer)
    preferenceTimer = null
  }
})
</script>

<style scoped lang="scss">
.group-selector {
  position: relative;
  display: inline-block;

  &__trigger {
    display: flex;
    align-items: center;
    gap: 0.4em;
    background: none;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 0.3em 0.7em;
    cursor: pointer;
    font-size: 0.9em;
    line-height: 1.2;
    color: inherit;

    &:hover {
      border-color: var(--color-ring);
      background: var(--color-accent);
    }

    &:focus-visible {
      outline: 2px solid var(--color-ring);
      outline-offset: 1px;
    }
  }

  &__icon {
    font-size: 1.1em;
  }

  &__name {
    max-width: 150px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  &__caret {
    font-size: 0.7em;
    opacity: 0.6;
  }

  &__dropdown {
    position: absolute;
    top: 100%;
    left: 0;
    z-index: 1000;
    min-width: 200px;
    max-width: 300px;
    background: white;
    border: 1px solid #ddd;
    border-radius: 8px;
    box-shadow: 0 4px 12px rgb(0 0 0 / 15%);
    margin-top: 4px;
    padding: 4px 0;
  }

  &__item {
    display: flex;
    align-items: center;
    gap: 0.5em;
    padding: 0.5em 0.8em;
    cursor: pointer;
    font-size: 0.9em;
    width: 100%;
    background: none;
    border: none;
    text-align: left;
    color: inherit;
    font-family: inherit;

    &--active {
      background: #e8f0fe;
      font-weight: 600;
    }

    &--action {
      color: #555;
      font-size: 0.85em;
    }

    &:hover {
      background: #f0f0f0;
    }
  }

  &__item-icon {
    font-size: 1.1em;
  }

  &__item-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  &__divider {
    height: 1px;
    background: #eee;
    margin: 4px 0;
  }

  &__error {
    margin: 0.3em 0 0;
    font-size: 0.8em;
    color: #c62828;
    max-width: 250px;
  }
}
</style>
