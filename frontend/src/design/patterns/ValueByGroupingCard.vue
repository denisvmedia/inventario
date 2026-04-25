<script setup lang="ts">
/**
 * ValueByGroupingCard — a card that shows a labelled list of "name →
 * value" pairs (e.g. "Value by Location", "Items by Status"). Used by
 * the HomeView dashboard (#1330 PR 5.1) and ready for any other
 * grouped-totals widget that lands later.
 *
 * The pattern is presentation-only: items arrive with `name` + a
 * pre-formatted `value` string, so the same card can show currency
 * totals, integer counts, percentages, or any other domain figure
 * without knowing the formatting rules.
 *
 * `loading` paints a row of skeleton bars so the grid can lay out
 * immediately. `empty` is shown when the parent has loaded but has
 * nothing to display — keeps the empty-state copy out of the parent.
 */
import type { HTMLAttributes } from 'vue'

import { cn } from '@design/lib/utils'

export interface ValueByGroupingItem {
  /** Stable key for v-for (defaults to `name` if omitted). */
  id?: string
  name: string
  /** Pre-formatted display value (currency, count, percentage, …). */
  value: string
}

interface Props {
  title: string
  items: ValueByGroupingItem[]
  loading?: boolean
  /** Copy shown when items.length === 0 and not loading. */
  empty?: string
  /** How many skeleton rows to render while `loading` is true. */
  skeletonRows?: number
  testId?: string
  class?: HTMLAttributes['class']
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
  empty: 'No data yet.',
  skeletonRows: 3,
})

defineSlots<{
  /** Trailing action(s) on the header row (e.g. a "View all" link). */
  actions?: () => unknown
}>()
</script>

<template>
  <section
    :class="
      cn(
        'flex flex-col gap-4 rounded-md border border-border bg-card p-4 sm:p-6 shadow-sm',
        props.class,
      )
    "
    :data-testid="testId"
  >
    <header class="flex items-center justify-between gap-3">
      <h3 class="text-base font-semibold text-foreground">{{ title }}</h3>
      <div v-if="$slots.actions" class="shrink-0">
        <slot name="actions" />
      </div>
    </header>

    <ul v-if="loading" class="flex flex-col gap-2" aria-busy="true">
      <li
        v-for="n in skeletonRows"
        :key="n"
        class="flex items-center justify-between gap-3"
      >
        <div class="h-4 w-32 animate-pulse rounded bg-muted" />
        <div class="h-4 w-20 animate-pulse rounded bg-muted" />
      </li>
    </ul>

    <p
      v-else-if="items.length === 0"
      class="py-2 text-sm text-muted-foreground"
    >
      {{ empty }}
    </p>

    <ul v-else class="flex flex-col divide-y divide-border">
      <li
        v-for="item in items"
        :key="item.id ?? item.name"
        class="flex items-center justify-between gap-3 py-2 first:pt-0 last:pb-0"
      >
        <span class="truncate text-sm text-foreground">{{ item.name }}</span>
        <span class="shrink-0 text-sm font-medium text-primary">{{ item.value }}</span>
      </li>
    </ul>
  </section>
</template>
