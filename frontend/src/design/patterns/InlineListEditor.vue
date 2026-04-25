<script setup lang="ts" generic="T">
import type { HTMLAttributes } from "vue"
import { computed } from "vue"
import { Plus, Trash2 } from "lucide-vue-next"

import { Button } from "@design/ui/button"
import { Input } from "@design/ui/input"
import { cn } from "@design/lib/utils"

interface Props {
  addLabel: string
  removeLabel?: string
  placeholder?: string
  newItem?: () => T
  allowEmpty?: boolean
  class?: HTMLAttributes["class"]
}

const props = withDefaults(defineProps<Props>(), {
  removeLabel: "Remove item",
  allowEmpty: true,
})

const items = defineModel<T[]>({ required: true })

defineSlots<{
  item?: (_props: {
    item: T
    index: number
    update: (_value: T) => void
  }) => unknown
  empty?: () => unknown
}>()

const list = computed(() => items.value ?? [])
const removeDisabled = computed(
  () => !props.allowEmpty && list.value.length <= 1,
)

function createNewItem(): T {
  if (!props.newItem) {
    throw new Error(
      "InlineListEditor requires a `newItem` factory to add items safely.",
    )
  }

  return props.newItem()
}

function add() {
  items.value = [...list.value, createNewItem()]
}

function remove(index: number) {
  if (removeDisabled.value) return
  items.value = list.value.filter((_, i) => i !== index)
}

function update(index: number, value: T) {
  const next = list.value.slice()
  next[index] = value
  items.value = next
}
</script>

<template>
  <div
    data-slot="inline-list-editor"
    :class="cn('flex flex-col gap-2', props.class)"
  >
    <div
      v-if="list.length === 0 && $slots.empty"
      data-slot="inline-list-editor-empty"
    >
      <slot name="empty" />
    </div>

    <div
      v-for="(item, index) in list"
      :key="index"
      data-slot="inline-list-editor-row"
      class="flex items-center gap-2"
    >
      <slot
        name="item"
        :item="item"
        :index="index"
        :update="(value: T) => update(index, value)"
      >
        <Input
          :model-value="item as string"
          :placeholder="placeholder"
          class="flex-1"
          @update:model-value="(value) => update(index, value as T)"
        />
      </slot>
      <Button
        type="button"
        variant="ghost"
        size="icon-sm"
        :aria-label="removeLabel"
        :disabled="removeDisabled"
        data-testid="inline-list-editor-remove"
        @click="remove(index)"
      >
        <Trash2 aria-hidden="true" />
      </Button>
    </div>

    <div>
      <Button
        type="button"
        variant="outline"
        size="sm"
        data-testid="inline-list-editor-add"
        @click="add"
      >
        <Plus aria-hidden="true" />
        {{ addLabel }}
      </Button>
    </div>
  </div>
</template>
