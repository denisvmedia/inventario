<script setup lang="ts">
/**
 * PageSection — titled subdivision within a page.
 *
 * Used to break a long view into labelled regions (e.g. "General",
 * "Permissions", "Danger zone"). Renders an <section> with an
 * <h2>-equivalent heading by default and optional description and trailing
 * actions. The body comes through the default slot.
 *
 * Heading level is configurable so deeply nested sections can keep the
 * document outline correct.
 */
import { computed, type HTMLAttributes } from 'vue'
import { cn } from '@design/lib/utils'

type HeadingLevel = 'h2' | 'h3' | 'h4'

type Props = {
  title?: string
  description?: string
  as?: HeadingLevel
  class?: HTMLAttributes['class']
  testId?: string
}

const props = withDefaults(defineProps<Props>(), {
  as: 'h2',
})

defineSlots<{
  default?: () => unknown
  /** Action buttons aligned with the section heading. */
  actions?: () => unknown
  /** Override for the description area. */
  description?: () => unknown
}>()

const titleClasses = computed(() => {
  const base = 'font-semibold tracking-tight text-foreground'
  const sizeByLevel: Record<HeadingLevel, string> = {
    h2: 'text-lg sm:text-xl',
    h3: 'text-base sm:text-lg',
    h4: 'text-sm sm:text-base',
  }
  return cn(base, sizeByLevel[props.as])
})

const hasHeader = computed(() => Boolean(props.title))
</script>

<template>
  <section :class="cn('flex flex-col gap-4', props.class)" :data-testid="testId">
    <div v-if="hasHeader" class="flex flex-wrap items-start justify-between gap-3">
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
    <div>
      <slot />
    </div>
  </section>
</template>
