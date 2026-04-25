<script lang="ts">
import type { VariantProps } from "class-variance-authority"
import { cva } from "class-variance-authority"

export const EXPORT_STATUSES = [
  "pending",
  "in_progress",
  "completed",
  "failed",
  "deleted",
] as const

export type ExportStatus = (typeof EXPORT_STATUSES)[number]

export const exportStatusPillVariants = cva(
  "inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium w-fit whitespace-nowrap [&>svg]:size-3.5 [&>svg]:pointer-events-none",
  {
    variants: {
      status: {
        pending:
          "border-muted-foreground/30 bg-muted text-muted-foreground",
        in_progress:
          "border-primary/40 bg-primary/10 text-primary",
        completed:
          "border-success/40 bg-success/10 text-success",
        failed:
          "border-destructive/40 bg-destructive/10 text-destructive",
        deleted:
          "border-muted-foreground/20 bg-muted/40 text-muted-foreground line-through decoration-muted-foreground/60",
      },
    },
    defaultVariants: {
      status: "pending",
    },
  },
)

export type ExportStatusPillVariants = VariantProps<
  typeof exportStatusPillVariants
>

export const EXPORT_STATUS_LABELS: Record<ExportStatus, string> = {
  pending: "Pending",
  in_progress: "In progress",
  completed: "Completed",
  failed: "Failed",
  deleted: "Deleted",
}
</script>

<script setup lang="ts">
import type { HTMLAttributes } from "vue"
import { computed } from "vue"
import {
  CheckCircle2,
  Clock,
  Loader2,
  Trash2,
  XCircle,
} from "lucide-vue-next"

import { cn } from "@design/lib/utils"

interface Props {
  status: ExportStatus
  /** Override the default English label, e.g. for i18n. */
  label?: string
  class?: HTMLAttributes["class"]
}

const props = defineProps<Props>()

const icons = {
  pending: Clock,
  in_progress: Loader2,
  completed: CheckCircle2,
  failed: XCircle,
  deleted: Trash2,
} as const

const icon = computed(() => icons[props.status])
const text = computed(
  () => props.label ?? EXPORT_STATUS_LABELS[props.status],
)
const iconClass = computed(() =>
  props.status === "in_progress"
    ? "motion-safe:animate-spin"
    : undefined,
)
</script>

<template>
  <span
    data-slot="export-status-pill"
    :data-status="status"
    :class="cn(exportStatusPillVariants({ status }), props.class)"
  >
    <component :is="icon" :class="iconClass" aria-hidden="true" />
    {{ text }}
  </span>
</template>
