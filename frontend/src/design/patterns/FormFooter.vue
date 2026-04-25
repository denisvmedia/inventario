<script lang="ts">
import type { VariantProps } from "class-variance-authority"
import { cva } from "class-variance-authority"

export const formFooterVariants = cva("flex flex-wrap items-center gap-2", {
  variants: {
    align: {
      start: "justify-start",
      between: "justify-between",
      end: "justify-end",
    },
    border: {
      none: "",
      top: "border-t border-border pt-4",
    },
  },
  defaultVariants: {
    align: "end",
    border: "top",
  },
})

export type FormFooterVariants = VariantProps<typeof formFooterVariants>
</script>

<script setup lang="ts">
import type { HTMLAttributes } from "vue"
import { cn } from "@design/lib/utils"

interface Props {
  align?: FormFooterVariants["align"]
  border?: FormFooterVariants["border"]
  class?: HTMLAttributes["class"]
}

const props = defineProps<Props>()
</script>

<template>
  <footer
    data-slot="form-footer"
    :class="cn(formFooterVariants({ align, border }), props.class)"
  >
    <slot />
  </footer>
</template>
