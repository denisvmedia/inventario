<script setup lang="ts">
/**
 * AreaCard — clickable card representing an area within a location.
 *
 * Replaces the legacy `<div class="area-card">` markup that lived
 * inline in `LocationDetailView` (Phase 4 / #1329 of Epic #1324).
 *
 * The legacy `area-card` class is preserved as a no-op anchor so
 * existing Playwright selectors (`.area-card`) keep working through
 * the strangler-fig migration window — see
 * devdocs/frontend/migration-conventions.md.
 *
 * The whole card is the primary "view" affordance; edit and delete
 * are secondary actions in the trailing action row that stop click
 * propagation so the card click handler only fires on the body.
 */
import type { HTMLAttributes } from 'vue'
import { Pencil, Trash2 } from 'lucide-vue-next'
import { Card } from '@design/ui/card'
import { cn } from '@design/lib/utils'
import IconButton from './IconButton.vue'

type AreaLike = {
  id: string
  attributes: {
    name: string
    [key: string]: unknown
  }
}

type Props = {
  area: AreaLike
  /** Render edit / delete action buttons. Defaults to true. */
  showActions?: boolean
  /** Optional secondary line under the title (e.g. parent location). */
  subtitle?: string
  class?: HTMLAttributes['class']
  testId?: string
}

const props = withDefaults(defineProps<Props>(), {
  showActions: true,
  subtitle: '',
})

type Emits = {
  view: [id: string]
  edit: [id: string]
  delete: [id: string]
}
const emit = defineEmits<Emits>()

function onCardClick() {
  emit('view', props.area.id)
}

function onCardKey(event: KeyboardEvent) {
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    emit('view', props.area.id)
  }
}
</script>

<template>
  <Card
    :class="
      cn(
        'area-card group relative cursor-pointer transition-all hover:-translate-y-0.5 hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        'flex-row items-start justify-between gap-4 px-5 py-4',
        props.class,
      )
    "
    :data-testid="testId ?? `area-card-${area.id}`"
    role="button"
    tabindex="0"
    @click="onCardClick"
    @keydown="onCardKey"
  >
    <div class="min-w-0 flex-1">
      <h3 class="truncate text-base font-semibold text-foreground">
        {{ area.attributes.name }}
      </h3>
      <p
        v-if="subtitle || $slots.subtitle"
        class="mt-1 truncate text-xs italic text-muted-foreground"
      >
        <slot name="subtitle">{{ subtitle }}</slot>
      </p>
    </div>

    <div v-if="showActions" class="flex shrink-0 items-center gap-1">
      <IconButton
        aria-label="Edit area"
        :test-id="`area-card-${area.id}-edit`"
        @click.stop="emit('edit', area.id)"
      >
        <Pencil class="size-4" aria-hidden="true" />
      </IconButton>
      <IconButton
        variant="ghost"
        aria-label="Delete area"
        :test-id="`area-card-${area.id}-delete`"
        class="text-destructive hover:text-destructive"
        @click.stop="emit('delete', area.id)"
      >
        <Trash2 class="size-4" aria-hidden="true" />
      </IconButton>
    </div>
  </Card>
</template>
