<script lang="ts">
import type { VariantProps } from "class-variance-authority"
import { cva } from "class-variance-authority"

export const formSectionVariants = cva("flex flex-col", {
  variants: {
    spacing: {
      compact: "gap-3",
      default: "gap-4",
      relaxed: "gap-6",
    },
  },
  defaultVariants: {
    spacing: "default",
  },
})

export type FormSectionVariants = VariantProps<typeof formSectionVariants>
</script>

<script setup lang="ts">
import type { HTMLAttributes } from "vue"
import { computed, useId } from "vue"
import { cn } from "@design/lib/utils"

interface Props {
  title?: string
  description?: string
  spacing?: FormSectionVariants["spacing"]
  class?: HTMLAttributes["class"]
}

const props = defineProps<Props>()

defineSlots<{
  title?: () => unknown
  description?: () => unknown
  default?: () => unknown
}>()

const titleId = useId()
const descriptionId = useId()

const hasTitle = computed(() => !!props.title)
const hasDescription = computed(() => !!props.description)
</script>

<template>
  <section
    data-slot="form-section"
    :aria-labelledby="hasTitle || $slots.title ? titleId : undefined"
    :aria-describedby="hasDescription || $slots.description ? descriptionId : undefined"
    :class="cn(formSectionVariants({ spacing }), props.class)"
  >
    <header
      v-if="hasTitle || $slots.title || hasDescription || $slots.description"
      class="flex flex-col gap-1"
    >
      <h3
        v-if="hasTitle || $slots.title"
        :id="titleId"
        class="text-base font-semibold text-foreground"
      >
        <slot name="title">{{ title }}</slot>
      </h3>
      <p
        v-if="hasDescription || $slots.description"
        :id="descriptionId"
        class="text-sm text-muted-foreground"
      >
        <slot name="description">{{ description }}</slot>
      </p>
    </header>
    <div data-slot="form-section-body" class="flex flex-col gap-4">
      <slot />
    </div>
  </section>
</template>
