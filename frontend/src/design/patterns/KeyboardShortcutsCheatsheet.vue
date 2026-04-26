<script setup lang="ts">
/**
 * KeyboardShortcutsCheatsheet — modal listing the global keyboard
 * shortcuts. Bound to `?` in App.vue and reachable from the user menu.
 * The shortcut definitions are mirrored here from the App.vue bindings
 * so the cheatsheet stays the source of truth for *which* shortcuts the
 * user can press; updating App.vue's bindings without updating this
 * table will surface as drift in code review.
 */
import { computed } from 'vue'

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@design/ui/dialog'

interface Props {
  modelValue: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()

const open = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

interface ShortcutRow {
  keys: string[]
  description: string
}

interface ShortcutGroup {
  title: string
  rows: ShortcutRow[]
}

const groups: ShortcutGroup[] = [
  {
    title: 'Global',
    rows: [
      { keys: ['?'], description: 'Show this cheatsheet' },
      { keys: ['Mod', 'K'], description: 'Open command palette / search' },
      { keys: ['/'], description: 'Open command palette' },
      { keys: ['Esc'], description: 'Close dialog or palette' },
    ],
  },
  {
    title: 'Navigation',
    rows: [
      { keys: ['g', 'h'], description: 'Go to dashboard (home)' },
      { keys: ['g', 'l'], description: 'Go to locations' },
      { keys: ['g', 'c'], description: 'Go to commodities' },
      { keys: ['g', 'f'], description: 'Go to files' },
    ],
  },
]

function isMacLike(): boolean {
  if (typeof navigator === 'undefined') return false
  return /Mac|iPhone|iPad|iPod/i.test(navigator.platform || '')
    || /Mac/i.test(navigator.userAgent || '')
}

function renderKey(key: string): string {
  if (key !== 'Mod') return key
  return isMacLike() ? '⌘' : 'Ctrl'
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="max-w-md">
      <DialogHeader>
        <DialogTitle>Keyboard shortcuts</DialogTitle>
        <DialogDescription>
          Press these anywhere in the app. Two-key sequences (like
          <kbd class="font-mono text-xs">g h</kbd>) need to be pressed within a
          short window.
        </DialogDescription>
      </DialogHeader>
      <div class="flex flex-col gap-6">
        <section v-for="group in groups" :key="group.title" class="flex flex-col gap-2">
          <h3 class="text-sm font-semibold text-foreground">{{ group.title }}</h3>
          <ul class="flex flex-col gap-1.5">
            <li
              v-for="row in group.rows"
              :key="row.description"
              class="flex items-center justify-between gap-3 text-sm"
            >
              <span class="text-muted-foreground">{{ row.description }}</span>
              <span class="flex items-center gap-1">
                <kbd
                  v-for="(key, index) in row.keys"
                  :key="`${row.description}-${index}`"
                  class="rounded border border-border bg-muted px-1.5 py-0.5 font-mono text-xs text-foreground"
                >
                  {{ renderKey(key) }}
                </kbd>
              </span>
            </li>
          </ul>
        </section>
      </div>
    </DialogContent>
  </Dialog>
</template>
