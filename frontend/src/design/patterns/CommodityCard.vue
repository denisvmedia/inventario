<script setup lang="ts">
/**
 * CommodityCard — list-row card for a commodity inside the
 * CommodityListView grid. Composes from Tailwind status tokens, the
 * CommodityStatusPill pill, and IconButton for the trailing actions.
 *
 * Replaces the legacy `CommodityListItem` (#1328 PR 3.2). The legacy
 * "SOLD" watermark, "draft" diagonal hatch, ⚠️ / 🗑️ emoji markers and
 * heavy `filter: grayscale/saturate` are gone — modern visual signals
 * are: a `border-l-4 border-l-status-*` accent, a desaturation modifier
 * for non-active items, and the existing CommodityStatusPill (icon +
 * text + color) which is fully accessible and dark-mode aware. The
 * `.commodity-card` class stays on the outermost element as a legacy
 * Playwright anchor (devdocs/frontend/migration-conventions.md).
 */
import type { FunctionalComponent, HTMLAttributes } from 'vue'
import { computed } from 'vue'
import {
  Box,
  Calendar,
  CookingPot,
  Laptop,
  type LucideProps,
  MapPin,
  Pencil,
  Shirt,
  Sofa,
  Trash2,
  Wrench,
} from 'lucide-vue-next'

import { cn } from '@design/lib/utils'

import CommodityStatusPill, {
  type CommodityStatus,
} from './CommodityStatusPill.vue'
import IconButton from './IconButton.vue'

type CommodityType =
  | 'white_goods'
  | 'electronics'
  | 'equipment'
  | 'furniture'
  | 'clothes'
  | 'other'

type Props = {
  name: string
  /** Commodity type id (free-form to tolerate unknown values from older data). */
  type: CommodityType | string
  status: CommodityStatus
  /** When true, the card renders the draft visual treatment regardless of status. */
  draft?: boolean
  count?: number
  /** ISO date string; rendered as a localized "Mar 5, 2026" line when present. */
  purchaseDate?: string
  /** Pre-formatted main price string (e.g. "100.00 USD"). */
  displayPrice?: string
  /** Pre-formatted per-unit price; rendered when count > 1. */
  pricePerUnit?: string
  /** Optional location/area breadcrumb shown above the meta row. */
  locationName?: string
  areaName?: string
  /** Highlight ring shown when arriving from a deep-link / detail back-nav. */
  highlighted?: boolean
  testId?: string
  class?: HTMLAttributes['class']
}

const props = withDefaults(defineProps<Props>(), {
  draft: false,
  count: 1,
  highlighted: false,
})

type Emits = {
  view: []
  edit: []
  delete: []
}
const emit = defineEmits<Emits>()

const typeIcons: Record<string, FunctionalComponent<LucideProps>> = {
  white_goods: CookingPot,
  electronics: Laptop,
  equipment: Wrench,
  furniture: Sofa,
  clothes: Shirt,
  other: Box,
}

const typeLabels: Record<string, string> = {
  white_goods: 'White Goods',
  electronics: 'Electronics',
  equipment: 'Equipment',
  furniture: 'Furniture',
  clothes: 'Clothes',
  other: 'Other',
}

const typeIcon = computed<FunctionalComponent<LucideProps>>(
  () => typeIcons[props.type] ?? Box,
)
const typeLabel = computed(() => typeLabels[props.type] ?? props.type)

const effectiveStatus = computed<CommodityStatus>(() =>
  props.draft ? 'draft' : props.status,
)

const muted = computed(() => {
  if (props.draft) return true
  return (
    props.status === 'sold' ||
    props.status === 'lost' ||
    props.status === 'disposed' ||
    props.status === 'written_off'
  )
})

// Full literal classes so Tailwind's source scanner can detect them.
const borderClass = computed(() => {
  if (props.draft) return 'border-l-status-draft'
  switch (props.status) {
    case 'in_use':
      return 'border-l-status-in-use'
    case 'sold':
      return 'border-l-status-sold'
    case 'lost':
      return 'border-l-status-lost'
    case 'disposed':
      return 'border-l-status-disposed'
    case 'written_off':
      return 'border-l-status-written-off'
    default:
      return 'border-l-status-draft'
  }
})

function formatDate(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  return d.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

function onClick() {
  emit('view')
}
function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    emit('view')
  }
}
</script>

<template>
  <div
    role="button"
    tabindex="0"
    :data-testid="testId"
    :data-status="effectiveStatus"
    :class="
      cn(
        'commodity-card group relative flex items-start justify-between gap-3',
        'cursor-pointer rounded-md border border-l-4 border-border bg-card p-4 sm:p-6 shadow-sm',
        borderClass,
        muted && 'opacity-80 saturate-[.75]',
        highlighted && 'ring-2 ring-primary ring-offset-2',
        'motion-safe:transition-shadow hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
        props.class,
      )
    "
    @click="onClick"
    @keydown="onKeydown"
  >
    <div class="min-w-0 flex-1">
      <h3 class="truncate text-base font-semibold text-foreground">{{ name }}</h3>

      <div
        v-if="locationName || areaName"
        class="mt-1 flex items-center gap-1 text-sm text-muted-foreground"
      >
        <MapPin class="size-3.5 shrink-0" aria-hidden="true" />
        <span class="truncate">
          {{ locationName }}{{ locationName && areaName ? ' / ' : '' }}{{ areaName }}
        </span>
      </div>

      <div class="mt-2 flex items-center justify-between gap-2 text-sm text-muted-foreground">
        <span class="inline-flex items-center gap-1">
          <component :is="typeIcon" class="size-4" aria-hidden="true" />
          {{ typeLabel }}
        </span>
        <span v-if="count > 1" class="font-medium">×{{ count }}</span>
      </div>

      <div
        v-if="purchaseDate"
        class="mt-2 flex items-center gap-1 text-sm text-muted-foreground"
      >
        <Calendar class="size-3.5" aria-hidden="true" />
        {{ formatDate(purchaseDate) }}
      </div>

      <div v-if="displayPrice" class="mt-3 flex flex-col gap-0.5">
        <span class="text-base font-semibold text-foreground">{{ displayPrice }}</span>
        <span
          v-if="count > 1 && pricePerUnit"
          class="text-xs italic text-muted-foreground"
        >
          {{ pricePerUnit }} per unit
        </span>
      </div>

      <div class="mt-3">
        <CommodityStatusPill :status="effectiveStatus" />
      </div>
    </div>

    <div class="flex shrink-0 items-center gap-1" @click.stop>
      <IconButton
        aria-label="Edit commodity"
        title="Edit"
        size="icon-sm"
        @click="emit('edit')"
      >
        <Pencil />
      </IconButton>
      <IconButton
        aria-label="Delete commodity"
        title="Delete"
        size="icon-sm"
        class="text-destructive hover:bg-destructive/10 hover:text-destructive"
        @click="emit('delete')"
      >
        <Trash2 />
      </IconButton>
    </div>
  </div>
</template>
