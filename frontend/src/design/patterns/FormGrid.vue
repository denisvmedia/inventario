<script lang="ts">
import type { VariantProps } from "class-variance-authority"
import { cva } from "class-variance-authority"

export const formGridVariants = cva("grid", {
  variants: {
    cols: {
      "1": "grid-cols-1",
      "2": "grid-cols-1 md:grid-cols-2",
      "3": "grid-cols-1 sm:grid-cols-2 lg:grid-cols-3",
      "4": "grid-cols-1 sm:grid-cols-2 lg:grid-cols-4",
    },
    gap: {
      sm: "gap-3",
      default: "gap-4",
      lg: "gap-6",
    },
  },
  defaultVariants: {
    cols: "2",
    gap: "default",
  },
})

export type FormGridVariants = VariantProps<typeof formGridVariants>
</script>

<script setup lang="ts">
import type { HTMLAttributes } from "vue"
import { cn } from "@design/lib/utils"

interface Props {
  cols?: FormGridVariants["cols"]
  gap?: FormGridVariants["gap"]
  class?: HTMLAttributes["class"]
}

const props = defineProps<Props>()
</script>

<template>
  <div
    data-slot="form-grid"
    :class="cn(formGridVariants({ cols, gap }), props.class)"
  >
    <slot />
  </div>
</template>
