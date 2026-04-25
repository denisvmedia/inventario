<script lang="ts">
import type { VariantProps } from "class-variance-authority"
import { cva } from "class-variance-authority"

export const filterBarVariants = cva(
  "flex flex-wrap items-center gap-2 rounded-md border border-border bg-card p-2",
  {
    variants: {
      density: {
        compact: "p-1.5 gap-1.5",
        default: "p-2 gap-2",
        relaxed: "p-3 gap-3",
      },
    },
    defaultVariants: {
      density: "default",
    },
  },
)

export type FilterBarVariants = VariantProps<typeof filterBarVariants>
</script>

<script setup lang="ts">
import type { HTMLAttributes } from "vue"
import { cn } from "@design/lib/utils"

interface Props {
  density?: FilterBarVariants["density"]
  class?: HTMLAttributes["class"]
}

const props = defineProps<Props>()

defineSlots<{
  /** Featured search input — flex-grows to fill the row. */
  search?: () => unknown
  /** Filter controls (selects, toggles, etc.). */
  default?: () => unknown
  /** Trailing actions (e.g. "Clear all"). Pinned to the right. */
  actions?: () => unknown
}>()
</script>

<template>
  <div
    data-slot="filter-bar"
    role="toolbar"
    :class="cn(filterBarVariants({ density }), props.class)"
  >
    <div
      v-if="$slots.search"
      data-slot="filter-bar-search"
      class="min-w-0 flex-1 basis-60"
    >
      <slot name="search" />
    </div>
    <div
      v-if="$slots.default"
      data-slot="filter-bar-filters"
      class="flex flex-wrap items-center gap-2"
    >
      <slot />
    </div>
    <div
      v-if="$slots.actions"
      data-slot="filter-bar-actions"
      class="ml-auto flex items-center gap-2"
    >
      <slot name="actions" />
    </div>
  </div>
</template>
