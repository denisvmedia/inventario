<script setup lang="ts">
/**
 * PageContainer — outermost layout wrapper for a routed view.
 *
 * Replaces the legacy `<main class="container">` shell from `App.vue` so
 * views can compose their own width without depending on global SCSS in
 * `_components.scss`. Caller-supplied `class` always wins thanks to
 * `cn()` + `tailwind-merge`.
 *
 * Variants:
 *   width="default" → max-w-screen-xl (mirrors the legacy 1200px cap)
 *   width="narrow"  → max-w-2xl       (forms, auth, single-column flows)
 *   width="full"    → no max-width    (full-bleed views like print)
 */
import { computed, type HTMLAttributes } from 'vue'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@design/lib/utils'

const containerVariants = cva('mx-auto w-full px-4 sm:px-6', {
  variants: {
    width: {
      default: 'max-w-screen-xl',
      narrow: 'max-w-2xl',
      full: 'max-w-none px-0 sm:px-0',
    },
    padded: {
      true: 'py-6 sm:py-8',
      false: '',
    },
  },
  defaultVariants: {
    width: 'default',
    padded: true,
  },
})

type ContainerVariants = VariantProps<typeof containerVariants>

type Props = {
  width?: ContainerVariants['width']
  padded?: ContainerVariants['padded']
  as?: 'main' | 'div' | 'section' | 'article'
  class?: HTMLAttributes['class']
  testId?: string
}

const props = withDefaults(defineProps<Props>(), {
  width: 'default',
  padded: true,
  as: 'main',
})

const classes = computed(() =>
  cn(containerVariants({ width: props.width, padded: props.padded }), props.class),
)
</script>

<template>
  <component :is="as" :class="classes" :data-testid="testId">
    <slot />
  </component>
</template>
