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
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useGroupStore } from '@/stores/groupStore'
import type { LocationGroup } from '@/types/group'

const groupStore = useGroupStore()
const router = useRouter()
const route = useRoute()
const isOpen = ref(false)
const selectorRef = ref<HTMLElement | null>(null)

// Switch to another group by navigating to its /g/<slug>/... URL, rebuilding
// the current subpath under the new slug so the user stays on the same kind
// of screen (commodities → commodities, exports → exports…). Writing the
// slug into the URL instead of mutating localStorage is what makes two tabs
// with two different groups independent (issue #1289 Gap C).
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
  const targetPath = `/g/${encodeURIComponent(group.slug)}${subpath || '/'}`
  await router.push(targetPath)
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
})
</script>

<style scoped lang="scss">
@use '@/assets/variables' as *;

.group-selector {
  position: relative;
  display: inline-block;

  &__trigger {
    display: flex;
    align-items: center;
    gap: 0.4em;
    background: none;
    border: 1px solid $header-control-border-color;
    border-radius: $header-control-radius;
    padding: $header-control-padding-y $header-control-padding-x;
    cursor: pointer;
    font-size: $header-control-font-size;
    line-height: 1.2;
    color: inherit;

    &:hover {
      border-color: $header-control-border-color-hover;
      background: $header-control-hover-bg;
    }

    &:focus-visible {
      outline: 2px solid $header-control-border-color-hover;
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
}
</style>
