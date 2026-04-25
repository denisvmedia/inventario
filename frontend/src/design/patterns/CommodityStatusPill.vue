<script lang="ts">
import type { VariantProps } from "class-variance-authority"
import { cva } from "class-variance-authority"

export const COMMODITY_STATUSES = [
  "draft",
  "in_use",
  "sold",
  "lost",
  "disposed",
  "written_off",
] as const

export type CommodityStatus = (typeof COMMODITY_STATUSES)[number]

export const commodityStatusPillVariants = cva(
  "inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium w-fit whitespace-nowrap [&>svg]:size-3.5 [&>svg]:pointer-events-none",
  {
    variants: {
      status: {
        draft:
          "border-status-draft/40 bg-status-draft/10 text-status-draft",
        in_use:
          "border-status-in-use/40 bg-status-in-use/10 text-status-in-use",
        sold:
          "border-status-sold/40 bg-status-sold/10 text-status-sold",
        lost:
          "border-status-lost/40 bg-status-lost/10 text-status-lost",
        disposed:
          "border-status-disposed/40 bg-status-disposed/10 text-status-disposed",
        written_off:
          "border-status-written-off/40 bg-status-written-off/10 text-status-written-off",
      },
    },
    defaultVariants: {
      status: "in_use",
    },
  },
)

export type CommodityStatusPillVariants = VariantProps<
  typeof commodityStatusPillVariants
>

export const COMMODITY_STATUS_LABELS: Record<CommodityStatus, string> = {
  draft: "Draft",
  in_use: "In use",
  sold: "Sold",
  lost: "Lost",
  disposed: "Disposed",
  written_off: "Written off",
}
</script>

<script setup lang="ts">
import type { HTMLAttributes } from "vue"
import { computed } from "vue"
import {
  Archive,
  CheckCircle2,
  CircleDollarSign,
  FileEdit,
  Trash2,
  TriangleAlert,
} from "lucide-vue-next"

import { cn } from "@design/lib/utils"

interface Props {
  status: CommodityStatus
  /** Override the default English label, e.g. for i18n. */
  label?: string
  class?: HTMLAttributes["class"]
}

const props = defineProps<Props>()

const icons = {
  draft: FileEdit,
  in_use: CheckCircle2,
  sold: CircleDollarSign,
  lost: TriangleAlert,
  disposed: Trash2,
  written_off: Archive,
} as const

const icon = computed(() => icons[props.status])
const text = computed(
  () => props.label ?? COMMODITY_STATUS_LABELS[props.status],
)
</script>

<template>
  <span
    data-slot="commodity-status-pill"
    :data-status="status"
    :class="cn(commodityStatusPillVariants({ status }), props.class)"
  >
    <component :is="icon" aria-hidden="true" />
    {{ text }}
  </span>
</template>
