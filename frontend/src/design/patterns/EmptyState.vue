<script setup lang="ts">
/**
 * EmptyState — placeholder shown when a list / collection has no items.
 *
 * Composition: optional illustration (slot or SVG src), title, optional
 * description, and an optional actions slot (typically a primary
 * "Create …" button).
 *
 * The pattern is intentionally layout-only: parents pass copy and
 * actions, this pattern composes them into a centred card-less block
 * sized for both narrow (mobile) and wide containers.
 */
import { type HTMLAttributes } from 'vue'
import { cn } from '@design/lib/utils'

type Props = {
  title: string
  description?: string
  /** Optional path to an SVG illustration; rendered above the title. */
  illustrationSrc?: string
  /** alt text for the illustration. Required when illustrationSrc is set. */
  illustrationAlt?: string
  class?: HTMLAttributes['class']
  testId?: string
}

const props = withDefaults(defineProps<Props>(), {
  illustrationAlt: '',
})

defineSlots<{
  /**
   * Custom illustration slot. Takes precedence over `illustrationSrc`.
   * Use this when the illustration needs to be a Vue component (icon,
   * animated SVG, etc.) instead of a static image.
   */
  illustration?: () => unknown
  /** Description override; takes precedence over the prop. */
  description?: () => unknown
  /** Trailing action buttons. */
  actions?: () => unknown
}>()
</script>

<template>
  <div
    role="status"
    :class="cn('flex flex-col items-center justify-center text-center gap-4 py-12 px-6', props.class)"
    :data-testid="testId"
  >
    <div v-if="$slots.illustration || illustrationSrc" class="mb-2">
      <slot name="illustration">
        <img
          v-if="illustrationSrc"
          :src="illustrationSrc"
          :alt="illustrationAlt"
          class="mx-auto size-32 sm:size-40 select-none"
          draggable="false"
        />
      </slot>
    </div>

    <h2 class="text-lg sm:text-xl font-semibold text-foreground">{{ title }}</h2>

    <p v-if="$slots.description || description" class="max-w-md text-sm text-muted-foreground">
      <slot name="description">{{ description }}</slot>
    </p>

    <div v-if="$slots.actions" class="mt-2 flex flex-wrap items-center justify-center gap-2">
      <slot name="actions" />
    </div>
  </div>
</template>
