<script setup lang="ts">
/**
 * BulkActionsBar — sticky bottom bar for bulk grid operations
 * (#1330 PR 5.5).
 *
 * The bar mounts at the bottom of the viewport when at least one row
 * in a list view is selected. The selection itself is owned by the
 * parent (the list view), so this pattern stays presentational: it
 * renders the selected-count + action slots and emits a `clear` event
 * for the "deselect all" affordance.
 *
 * Test hooks:
 *   - [data-testid="bulk-actions-bar"]   — the container
 *   - [data-testid="bulk-actions-count"] — the "N selected" label
 *   - [data-testid="bulk-actions-clear"] — the deselect-all button
 *
 * Slots:
 *   - default — action buttons (Delete, Move, …). Caller passes shadcn
 *               <Button> nodes; the bar provides only layout.
 */
import { X } from 'lucide-vue-next'
import { Button } from '@design/ui/button'

defineProps<{
  count: number
  /** Singular noun for the "N <noun> selected" label, e.g. "commodity". */
  itemNoun?: string
  /** Plural noun for the same label, e.g. "commodities". */
  itemNounPlural?: string
}>()

const emit = defineEmits<{
  (_e: 'clear'): void
}>()
</script>

<template>
  <Transition
    enter-active-class="motion-safe:transition-all motion-safe:duration-150 motion-safe:ease-out"
    enter-from-class="opacity-0 translate-y-3"
    enter-to-class="opacity-100 translate-y-0"
    leave-active-class="motion-safe:transition-all motion-safe:duration-100 motion-safe:ease-in"
    leave-from-class="opacity-100 translate-y-0"
    leave-to-class="opacity-0 translate-y-3"
  >
    <div
      v-if="count > 0"
      data-testid="bulk-actions-bar"
      role="region"
      aria-label="Bulk actions"
      class="fixed inset-x-0 bottom-0 z-40 border-t border-border bg-background/95 backdrop-blur-sm shadow-lg"
    >
      <div
        class="mx-auto flex max-w-6xl flex-wrap items-center gap-3 px-4 py-3 sm:flex-nowrap"
      >
        <p
          data-testid="bulk-actions-count"
          class="text-sm font-medium"
        >
          {{ count }}
          <template v-if="count === 1">
            {{ itemNoun ?? 'item' }} selected
          </template>
          <template v-else>
            {{ itemNounPlural ?? 'items' }} selected
          </template>
        </p>

        <div class="ml-auto flex flex-wrap items-center gap-2">
          <slot />
          <Button
            data-testid="bulk-actions-clear"
            type="button"
            variant="ghost"
            size="sm"
            class="gap-1"
            @click="emit('clear')"
          >
            <X class="size-4" aria-hidden="true" />
            Clear
          </Button>
        </div>
      </div>
    </div>
  </Transition>
</template>
