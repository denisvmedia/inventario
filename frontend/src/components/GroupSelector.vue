<template>
  <div class="group-selector" ref="selectorRef">
    <button
      class="group-selector__trigger"
      :aria-expanded="isOpen"
      aria-haspopup="true"
      @click.stop="isOpen = !isOpen"
    >
      <span v-if="groupStore.currentGroupIcon" class="group-selector__icon">{{ groupStore.currentGroupIcon }}</span>
      <span class="group-selector__name">{{ groupStore.currentGroupName || 'Select Group' }}</span>
      <span class="group-selector__caret">&#9662;</span>
    </button>
    <div v-if="isOpen" class="group-selector__dropdown">
      <div
        v-for="group in groupStore.groups"
        :key="group.id"
        class="group-selector__item"
        :class="{ 'group-selector__item--active': group.id === groupStore.currentGroupId }"
        @click="selectGroup(group)"
      >
        <span v-if="group.icon" class="group-selector__item-icon">{{ group.icon }}</span>
        <span class="group-selector__item-name">{{ group.name }}</span>
      </div>
      <div class="group-selector__divider" />
      <div class="group-selector__item group-selector__item--action" @click="openCreateDialog">
        + Create new group
      </div>
      <div
        v-if="groupStore.currentGroup"
        class="group-selector__item group-selector__item--action"
        @click="openGroupSettings"
      >
        Group settings
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useGroupStore } from '@/stores/groupStore'
import type { LocationGroup } from '@/types/group'

const groupStore = useGroupStore()
const router = useRouter()
const isOpen = ref(false)
const selectorRef = ref<HTMLElement | null>(null)

function selectGroup(group: LocationGroup) {
  groupStore.setCurrentGroup(group.slug)
  isOpen.value = false
  // Reload current page data by navigating to the same route
  router.go(0)
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
.group-selector {
  position: relative;
  display: inline-block;

  &__trigger {
    display: flex;
    align-items: center;
    gap: 0.4em;
    background: none;
    border: 1px solid #ccc;
    border-radius: 6px;
    padding: 0.3em 0.7em;
    cursor: pointer;
    font-size: 0.9em;
    color: inherit;

    &:hover {
      border-color: #999;
      background: rgba(0, 0, 0, 0.03);
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
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
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

    &:hover {
      background: #f0f0f0;
    }

    &--active {
      background: #e8f0fe;
      font-weight: 600;
    }

    &--action {
      color: #555;
      font-size: 0.85em;
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
