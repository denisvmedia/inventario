<script setup lang="ts">
/**
 * LocationCard — list-row card showing a location's display data and
 * its per-row actions (view, edit, delete). Composed from Tailwind
 * tokens plus the IconButton pattern.
 *
 * When `expandable` is true the outermost element behaves as a button
 * (role="button" + aria-expanded + keyboard-activatable) so callers can
 * wire it to an inline panel — that is what LocationListView uses to
 * reveal a location's nested areas.
 *
 * The `.location-card` class stays on the outermost element as a legacy
 * Playwright anchor (see devdocs/frontend/migration-conventions.md).
 */
import type { HTMLAttributes } from 'vue'
import { computed } from 'vue'
import {
  ChevronDown,
  ChevronRight,
  Eye,
  MapPin,
  Pencil,
  Trash2,
} from 'lucide-vue-next'

import { cn } from '@design/lib/utils'

import IconButton from './IconButton.vue'

type Props = {
  name: string
  address?: string
  /** Pre-formatted total value (e.g. "1 234.56 USD"). Hidden if omitted. */
  valueLabel?: string
  /** When true (default) the card body toggles `toggle` on click + Enter/Space. */
  expandable?: boolean
  /** Reflects the open/closed state of the inline panel below the card. */
  expanded?: boolean
  /** Render a "Loading…" placeholder for the value row. */
  loadingValue?: boolean
  /** Stable selector hook forwarded to the outermost element. */
  testId?: string
  class?: HTMLAttributes['class']
}

const props = withDefaults(defineProps<Props>(), {
  expandable: true,
  expanded: false,
  loadingValue: false,
})

type Emits = {
  toggle: []
  view: []
  edit: []
  delete: []
}
const emit = defineEmits<Emits>()

const interactive = computed(() => props.expandable)

function onCardClick() {
  if (interactive.value) emit('toggle')
}

function onCardKeydown(event: KeyboardEvent) {
  if (!interactive.value) return
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    emit('toggle')
  }
}
</script>

<template>
  <div
    :role="interactive ? 'button' : 'article'"
    :tabindex="interactive ? 0 : undefined"
    :aria-expanded="interactive ? expanded : undefined"
    :data-testid="testId"
    :class="
      cn(
        'location-card',
        'group flex items-start justify-between gap-3 rounded-md border border-border bg-card p-4 sm:p-6 shadow-sm',
        interactive
          ? 'cursor-pointer motion-safe:transition-shadow hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2'
          : undefined,
        props.class,
      )
    "
    @click="onCardClick"
    @keydown="onCardKeydown"
  >
    <div class="min-w-0 flex-1">
      <div class="flex items-center gap-2">
        <MapPin class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
        <h3 class="truncate text-base font-semibold text-foreground">{{ name }}</h3>
        <component
          :is="expanded ? ChevronDown : ChevronRight"
          v-if="expandable"
          class="ml-auto size-4 shrink-0 text-muted-foreground"
          aria-hidden="true"
        />
      </div>
      <p v-if="address" class="mt-1 text-sm italic text-muted-foreground">
        {{ address }}
      </p>
      <p
        v-if="loadingValue || valueLabel"
        class="mt-2 text-sm font-medium text-primary"
      >
        <span class="font-normal text-muted-foreground">Total value:</span>
        {{ loadingValue ? 'Loading…' : valueLabel }}
      </p>
    </div>

    <div class="flex shrink-0 items-center gap-1" @click.stop>
      <IconButton
        aria-label="View location"
        title="View"
        size="icon-sm"
        @click="emit('view')"
      >
        <Eye />
      </IconButton>
      <IconButton
        aria-label="Edit location"
        title="Edit"
        size="icon-sm"
        @click="emit('edit')"
      >
        <Pencil />
      </IconButton>
      <IconButton
        aria-label="Delete location"
        title="Delete"
        size="icon-sm"
        class="text-destructive hover:bg-destructive/10 hover:text-destructive"
        @click="emit('delete')"
      >
        <Trash2 />
      </IconButton>
    </div>
  </div>
</template>
