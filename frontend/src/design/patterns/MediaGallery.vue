<script lang="ts">
import type { VariantProps } from 'class-variance-authority'
import { cva } from 'class-variance-authority'

export const mediaGalleryVariants = cva(
  'grid gap-4',
  {
    variants: {
      density: {
        compact: 'grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6',
        default: 'grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4',
        relaxed: 'grid-cols-1 sm:grid-cols-2 lg:grid-cols-3',
      },
    },
    defaultVariants: {
      density: 'default',
    },
  },
)

export type MediaGalleryVariants = VariantProps<typeof mediaGalleryVariants>
</script>

<script setup lang="ts">
/**
 * MediaGallery — pure layout helper for a responsive grid of media
 * cards (FilePreview, image thumbnails, etc.). Owns no business logic
 * and no scroll behaviour — its only job is the responsive
 * `display: grid` track recipe with sensible breakpoints.
 *
 * Three densities are exposed via the `density` prop:
 *   - compact  : thumbnail-sized cards, 2 → 6 columns.
 *   - default  : standard FilePreview cards, 1 → 4 columns.
 *   - relaxed  : roomy cards, 1 → 3 columns.
 *
 * The default slot accepts any number of children; the parent owns
 * each child's lifecycle. Pair with EmptyState for "no items" copy.
 */
import type { HTMLAttributes } from 'vue'
import { cn } from '@design/lib/utils'

interface Props {
  density?: MediaGalleryVariants['density']
  /** Optional override of the layout tag. Defaults to <div>. */
  as?: 'div' | 'ul' | 'section'
  testId?: string
  class?: HTMLAttributes['class']
}

const props = withDefaults(defineProps<Props>(), {
  density: 'default',
  as: 'div',
})

defineSlots<{
  default?: () => unknown
}>()
</script>

<template>
  <component
    :is="as"
    data-slot="media-gallery"
    :data-density="density"
    :data-testid="testId"
    :class="cn(mediaGalleryVariants({ density }), props.class)"
  >
    <slot />
  </component>
</template>
