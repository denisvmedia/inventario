<script setup lang="ts">
/**
 * PageHeader — title block at the top of a routed view.
 *
 * Composes a heading, an optional supporting description, an optional
 * breadcrumb slot rendered above the title, and an optional actions slot
 * rendered to the right of the title (wraps below on narrow viewports).
 *
 * The heading level is configurable via `as` so views can keep document
 * outline correct when nested under a parent heading. Defaults to <h1>
 * which is the right choice for the top of a page.
 */
import { computed, type HTMLAttributes } from 'vue'
import { cn } from '@design/lib/utils'

type HeadingLevel = 'h1' | 'h2' | 'h3'

type Props = {
  title: string
  description?: string
  as?: HeadingLevel
  class?: HTMLAttributes['class']
  testId?: string
}

const props = withDefaults(defineProps<Props>(), {
  as: 'h1',
})

defineSlots<{
  /** Breadcrumb / contextual navigation rendered above the title. */
  breadcrumbs?: () => unknown
  /** Action buttons (typically <Button>s) on the trailing edge. */
  actions?: () => unknown
  /** Optional override for the description area. */
  description?: () => unknown
}>()

const titleClasses = computed(() => {
  const base = 'font-semibold tracking-tight text-foreground'
  const sizeByLevel: Record<HeadingLevel, string> = {
    h1: 'text-2xl sm:text-3xl',
    h2: 'text-xl sm:text-2xl',
    h3: 'text-lg sm:text-xl',
  }
  return cn(base, sizeByLevel[props.as])
})
</script>

<template>
  <header
    :class="cn('flex flex-col gap-3 pb-4 mb-6 border-b border-border', props.class)"
    :data-testid="testId ?? 'page-header'"
  >
    <div v-if="$slots.breadcrumbs" class="text-sm text-muted-foreground">
      <slot name="breadcrumbs" />
    </div>

    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0 flex-1">
        <component :is="as" :class="titleClasses">{{ title }}</component>
        <p v-if="$slots.description || description" class="mt-1 text-sm text-muted-foreground">
          <slot name="description">{{ description }}</slot>
        </p>
      </div>

      <div v-if="$slots.actions" class="flex flex-wrap items-center gap-2">
        <slot name="actions" />
      </div>
    </div>
  </header>
</template>
