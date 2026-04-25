<script lang="ts" setup>
import type { LabelProps } from "reka-ui"
import type { HTMLAttributes } from "vue"
import { computed } from "vue"
import { cn } from '@design/lib/utils'
import { Label } from '@design/ui/label'
import { useFormField } from "./useFormField"

const props = defineProps<LabelProps & { class?: HTMLAttributes["class"], required?: boolean }>()

const { error, formItemId } = useFormField()

const requiredAsteriskClasses = 'data-[required]:after:ml-0.5 data-[required]:after:text-destructive data-[required]:after:content-["*"]'

const mergedClass = computed(() =>
  cn('data-[error=true]:text-destructive', requiredAsteriskClasses, props.class),
)
</script>

<template>
  <Label
    data-slot="form-label"
    :data-error="!!error"
    :data-required="required ? '' : undefined"
    :class="mergedClass"
    :for="formItemId"
  >
    <slot />
  </Label>
</template>
