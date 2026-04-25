<script lang="ts">
import type { VariantProps } from 'class-variance-authority'
import { cva } from 'class-variance-authority'

export const statCardVariants = cva(
  'flex items-start gap-4 rounded-md border bg-card p-4 sm:p-6 shadow-sm',
  {
    variants: {
      variant: {
        default: 'border-border',
        primary: 'border-l-4 border-border border-l-primary',
        success: 'border-l-4 border-border border-l-success',
        warning: 'border-l-4 border-border border-l-warning',
        destructive: 'border-l-4 border-border border-l-destructive',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
)

export type StatCardVariants = VariantProps<typeof statCardVariants>
</script>

<script setup lang="ts">
/**
 * StatCard — single-statistic tile used inside the HomeView dashboard
 * grid (#1330 PR 5.1). Renders a big, glanceable value with a label,
 * an optional Lucide icon, and an optional description / sub-value.
 *
 * The pattern is presentation-only; parents pass already-formatted
 * strings so the same tile can show "1 234.56 USD", "42 commodities",
 * or "97% storage used" without learning anything domain-specific.
 *
 * `loading` swaps the value for a Skeleton-like placeholder so a
 * dashboard can paint the layout immediately and fill in numbers as
 * each request resolves, rather than waiting for the slowest one.
 */
import type { FunctionalComponent, HTMLAttributes } from 'vue'
import type { LucideProps } from 'lucide-vue-next'

import { cn } from '@design/lib/utils'

interface Props {
  label: string
  /** Pre-formatted value (e.g. "1 234.56 USD" or "42"). Hidden when `loading`. */
  value?: string | number
  /** Optional Lucide icon component rendered at top-left. */
  icon?: FunctionalComponent<LucideProps> | null
  /** Small descriptive text shown beneath the big value. */
  description?: string
  /** When true, the value row is replaced with a placeholder bar. */
  loading?: boolean
  variant?: StatCardVariants['variant']
  testId?: string
  class?: HTMLAttributes['class']
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
  variant: 'default',
  icon: null,
})

defineSlots<{
  /** Override the value area entirely (e.g. for a sparkline). */
  value?: () => unknown
  /** Trailing actions (link, button) aligned with the label row. */
  actions?: () => unknown
}>()
</script>

<template>
  <div
    :class="cn(statCardVariants({ variant }), props.class)"
    :data-testid="testId"
  >
    <component
      :is="icon"
      v-if="icon"
      class="size-6 shrink-0 text-muted-foreground"
      aria-hidden="true"
    />

    <div class="min-w-0 flex-1">
      <div class="flex items-start justify-between gap-2">
        <p class="text-sm font-medium text-muted-foreground">{{ label }}</p>
        <div v-if="$slots.actions" class="shrink-0">
          <slot name="actions" />
        </div>
      </div>

      <div class="mt-1">
        <slot name="value">
          <div
            v-if="loading"
            class="h-8 w-24 animate-pulse rounded bg-muted"
            aria-busy="true"
            aria-label="Loading"
          />
          <p v-else class="truncate text-2xl font-bold text-foreground sm:text-3xl">
            {{ value ?? '—' }}
          </p>
        </slot>
      </div>

      <p v-if="description" class="mt-1 text-xs text-muted-foreground">
        {{ description }}
      </p>
    </div>
  </div>
</template>
