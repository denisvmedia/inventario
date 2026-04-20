<template>
  <div ref="pickerRoot" class="icon-picker" :class="{ 'is-open': isOpen }">
    <button
      type="button"
      class="icon-picker__trigger"
      :aria-expanded="isOpen"
      aria-haspopup="dialog"
      :aria-label="triggerAriaLabel"
      :data-testid="triggerTestId"
      @click="toggle"
    >
      <span class="icon-picker__trigger-icon">
        <template v-if="modelValue">{{ modelValue }}</template>
        <template v-else>🖼️</template>
      </span>
      <span class="icon-picker__trigger-label">{{ triggerLabel }}</span>
      <span class="icon-picker__caret" aria-hidden="true">&#9662;</span>
    </button>

    <div
      v-if="isOpen"
      class="icon-picker__panel"
      role="dialog"
      :aria-label="panelAriaLabel"
      data-testid="icon-picker-panel"
      @keydown.esc="close"
    >
      <div class="icon-picker__tabs" role="tablist">
        <button
          v-for="cat in categories"
          :key="cat.id"
          type="button"
          role="tab"
          :aria-selected="activeCategory === cat.id"
          class="icon-picker__tab"
          :class="{ 'icon-picker__tab--active': activeCategory === cat.id }"
          :data-testid="`icon-picker-tab-${cat.id}`"
          @click="activeCategory = cat.id"
        >
          {{ cat.label }}
        </button>
      </div>

      <div class="icon-picker__grid" role="tabpanel" :aria-label="activeCategoryLabel">
        <button
          v-for="icon in visibleIcons"
          :key="icon.emoji"
          type="button"
          class="icon-picker__icon"
          :class="{ 'icon-picker__icon--selected': icon.emoji === modelValue }"
          :title="icon.label"
          :aria-label="icon.label"
          :aria-pressed="icon.emoji === modelValue"
          :data-testid="`icon-picker-option-${icon.emoji}`"
          @click="select(icon.emoji)"
        >
          {{ icon.emoji }}
        </button>
      </div>

      <div class="icon-picker__footer">
        <button
          type="button"
          class="icon-picker__clear"
          :disabled="!modelValue"
          data-testid="icon-picker-clear"
          @click="select('')"
        >
          No icon
        </button>
        <button
          type="button"
          class="icon-picker__close"
          data-testid="icon-picker-close"
          @click="close"
        >
          Done
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import {
  GROUP_ICONS,
  GROUP_ICON_CATEGORIES,
  type GroupIcon,
} from '@/constants/groupIcons'

const props = defineProps<{
  modelValue: string
  triggerLabel?: string
  panelAriaLabel?: string
  triggerTestId?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const pickerRoot = ref<HTMLElement | null>(null)
const isOpen = ref(false)
const categories = GROUP_ICON_CATEGORIES
const activeCategory = ref(categories[0]?.id ?? '')

const triggerLabel = computed(() => props.triggerLabel ?? 'Choose icon')
const panelAriaLabel = computed(() => props.panelAriaLabel ?? 'Icon picker')
const triggerAriaLabel = computed(() =>
  props.modelValue
    ? `${triggerLabel.value}: ${props.modelValue}. Click to change.`
    : `${triggerLabel.value}. No icon selected.`,
)
const triggerTestId = computed(() => props.triggerTestId ?? 'icon-picker-trigger')

const activeCategoryLabel = computed(
  () => categories.find((c) => c.id === activeCategory.value)?.label ?? '',
)

const visibleIcons = computed<GroupIcon[]>(() =>
  GROUP_ICONS.filter((ic) => ic.category === activeCategory.value),
)

function toggle() {
  isOpen.value = !isOpen.value
  if (isOpen.value) {
    // Jump the tab to the category of the currently selected icon so the
    // user sees it highlighted immediately — avoids needing to hunt for it.
    if (props.modelValue) {
      const current = GROUP_ICONS.find((ic) => ic.emoji === props.modelValue)
      if (current) activeCategory.value = current.category
    }
  }
}

function close() {
  isOpen.value = false
}

function select(emoji: string) {
  emit('update:modelValue', emoji)
  if (emoji === '') {
    // Staying open on "No icon" would be confusing — nothing else to do.
    close()
  }
}

function handleClickOutside(event: MouseEvent) {
  if (!isOpen.value) return
  if (pickerRoot.value && !pickerRoot.value.contains(event.target as Node)) {
    close()
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
.icon-picker {
  position: relative;
  display: inline-block;
  width: 100%;

  &__trigger {
    display: flex;
    align-items: center;
    gap: 0.5em;
    width: 100%;
    padding: 0.5em 0.7em;
    background: white;
    border: 1px solid #ccc;
    border-radius: 6px;
    cursor: pointer;
    font-size: 1em;
    color: inherit;
    text-align: left;

    &:hover {
      border-color: #999;
    }

    &:focus-visible {
      outline: 2px solid #1a73e8;
      outline-offset: 2px;
    }
  }

  &__trigger-icon {
    font-size: 1.2em;
    line-height: 1;
    min-width: 1.5em;
    text-align: center;
  }

  &__trigger-label {
    flex: 1;
    color: #555;
  }

  &__caret {
    font-size: 0.7em;
    opacity: 0.6;
  }

  &__panel {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    z-index: 1000;
    width: 100%;
    min-width: 320px;
    max-width: 420px;
    background: white;
    border: 1px solid #ddd;
    border-radius: 8px;
    box-shadow: 0 4px 12px rgb(0 0 0 / 15%);
    padding: 0.5em;
  }

  &__tabs {
    display: flex;
    flex-wrap: wrap;
    gap: 0.2em;
    margin-bottom: 0.5em;
    border-bottom: 1px solid #eee;
    padding-bottom: 0.4em;
  }

  &__tab {
    padding: 0.25em 0.6em;
    background: transparent;
    border: none;
    border-radius: 4px;
    font-size: 0.85em;
    cursor: pointer;
    color: #555;

    &--active {
      background: #e8f0fe;
      color: #1a73e8;
      font-weight: 600;
    }

    &:hover {
      background: #f0f0f0;
    }

    &:focus-visible {
      outline: 2px solid #1a73e8;
      outline-offset: 1px;
    }
  }

  &__grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(2.4em, 1fr));
    gap: 0.2em;
    max-height: 14em;
    overflow-y: auto;
  }

  &__icon {
    width: 100%;
    aspect-ratio: 1;
    font-size: 1.4em;
    background: transparent;
    border: 1px solid transparent;
    border-radius: 6px;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    line-height: 1;
    transition: background 0.1s, border-color 0.1s;

    &--selected {
      background: #e8f0fe;
      border-color: #1a73e8;
    }

    &:hover {
      background: #f0f0f0;
    }

    &:focus-visible {
      outline: 2px solid #1a73e8;
      outline-offset: 1px;
    }
  }

  &__footer {
    display: flex;
    justify-content: space-between;
    gap: 0.5em;
    margin-top: 0.5em;
    padding-top: 0.5em;
    border-top: 1px solid #eee;
  }

  &__clear,
  &__close {
    padding: 0.35em 0.8em;
    font-size: 0.85em;
    border-radius: 4px;
    border: 1px solid #ccc;
    background: white;
    cursor: pointer;

    &:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    &:hover:not(:disabled) {
      background: #f0f0f0;
    }
  }

  &__close {
    background: #1a73e8;
    color: white;
    border-color: #1a73e8;

    &:hover {
      background: #1560c0;
    }
  }
}
</style>
