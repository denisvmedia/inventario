<script lang="ts" setup>
import type { HTMLAttributes } from "vue"
import { useId } from "reka-ui"
import { provide } from "vue"
import { cn } from '@design/lib/utils'
import { FORM_ITEM_INJECTION_KEY } from "./injectionKeys"

// `id` is an optional override that anchors the underlying control to
// a stable DOM id. Required during the Phase 4 strangler-fig
// migration where Playwright e2e helpers select inputs via
// `#name`-style selectors — see devdocs/frontend/migration-conventions.md.
const props = defineProps<{
  class?: HTMLAttributes["class"]
  id?: string
}>()

const id = useId(props.id)
provide(FORM_ITEM_INJECTION_KEY, id)
</script>

<template>
  <div
    data-slot="form-item"
    :class="cn('grid gap-2', props.class)"
  >
    <slot />
  </div>
</template>
