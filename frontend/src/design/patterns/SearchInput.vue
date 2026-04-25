<script setup lang="ts">
import type { HTMLAttributes } from "vue"
import { computed } from "vue"
import { Search, X } from "lucide-vue-next"

import { Input } from "@design/ui/input"
import { cn } from "@design/lib/utils"

interface Props {
  placeholder?: string
  ariaLabel?: string
  clearLabel?: string
  class?: HTMLAttributes["class"]
}

const props = withDefaults(defineProps<Props>(), {
  ariaLabel: "Search",
  clearLabel: "Clear search",
})

const emit = defineEmits<{
  (_e: "clear"): void
}>()

const value = defineModel<string>({ default: "" })

const showClear = computed(() => !!value.value)

function clear() {
  value.value = ""
  emit("clear")
}
</script>

<template>
  <div
    data-slot="search-input"
    :class="cn('relative w-full', props.class)"
  >
    <Search
      class="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
      aria-hidden="true"
    />
    <Input
      v-model="value"
      type="search"
      role="searchbox"
      :placeholder="placeholder"
      :aria-label="ariaLabel"
      class="pl-9 pr-9"
    />
    <button
      v-if="showClear"
      type="button"
      :aria-label="clearLabel"
      data-testid="search-input-clear"
      class="absolute right-2 top-1/2 inline-flex size-7 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      @click="clear"
    >
      <X class="size-4" aria-hidden="true" />
    </button>
  </div>
</template>
