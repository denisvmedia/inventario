<script setup lang="ts">
/**
 * CommodityCard — clickable card representing a commodity in a list.
 *
 * Replaces `frontend/src/components/CommodityListItem.vue` (Phase 4 /
 * #1329 of Epic #1324). The legacy `commodity-card` class plus the
 * status modifier classes (`draft`, `sold`, `lost`, `disposed`,
 * `written-off`, `highlighted`) are preserved as no-op anchors so
 * existing Playwright selectors and any third-party snapshots keep
 * working through the strangler-fig window — see
 * devdocs/frontend/migration-conventions.md.
 *
 * Two presentational modes:
 *   - default: full card with type / count / purchase date / price /
 *     status pill (matches the legacy list-item shape).
 *   - compact: trimmed-down version used inside dense grids on
 *     `AreaDetailView`; hides the secondary metadata to fit narrow
 *     columns.
 */
import { computed, type FunctionalComponent, type HTMLAttributes } from 'vue'
import {
  Box,
  Calendar,
  CookingPot,
  Laptop,
  MapPin,
  Pencil,
  Shirt,
  Sofa,
  Trash2,
  Wrench,
  type LucideProps,
} from 'lucide-vue-next'
import { Card } from '@design/ui/card'
import { cn } from '@design/lib/utils'
import {
  calculatePricePerUnit,
  formatPrice,
  getDisplayPrice,
} from '@/services/currencyService'
import { COMMODITY_TYPES } from '@/constants/commodityTypes'
import IconButton from './IconButton.vue'
import CommodityStatusPill, {
  COMMODITY_STATUS_LABELS,
  type CommodityStatus,
} from './CommodityStatusPill.vue'

type CommodityLike = {
  id: string
  attributes: {
    name: string
    type?: string
    status?: string
    draft?: boolean
    count?: number
    area_id?: string
    purchase_date?: string
    [key: string]: unknown
  }
}

type Props = {
  commodity: CommodityLike
  /** Compact variant for dense grids (hides secondary metadata). */
  compact?: boolean
  /** Highlight border when this id matches the commodity id. */
  highlightCommodityId?: string
  /** Render the area / location breadcrumb under the title. */
  showLocation?: boolean
  /** id → { name, locationId } map; consulted only when showLocation is true. */
  areaMap?: Record<string, { name: string; locationId: string }>
  /** id → { name } map; consulted only when showLocation is true. */
  locationMap?: Record<string, { name: string }>
  /** Render edit / delete affordances. Defaults to true. */
  showActions?: boolean
  class?: HTMLAttributes['class']
  testId?: string
}

const props = withDefaults(defineProps<Props>(), {
  compact: false,
  highlightCommodityId: '',
  showLocation: false,
  areaMap: () => ({}),
  locationMap: () => ({}),
  showActions: true,
})

type Emits = {
  view: [id: string]
  edit: [id: string]
  delete: [id: string]
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

const typeLabel = computed(() => {
  const t = COMMODITY_TYPES.find((x) => x.id === props.commodity.attributes.type)
  return t?.name ?? props.commodity.attributes.type ?? ''
})
const typeIcon = computed(() => typeIcons[props.commodity.attributes.type ?? 'other'] ?? Box)

const status = computed<CommodityStatus | null>(() => {
  const a = props.commodity.attributes
  if (a.draft) return 'draft'
  const s = a.status
  if (!s) return null
  return (s as CommodityStatus) in COMMODITY_STATUS_LABELS
    ? (s as CommodityStatus)
    : null
})

const isHighlighted = computed(
  () => props.commodity.id === props.highlightCommodityId,
)

const count = computed(() => props.commodity.attributes.count ?? 1)
const showCount = computed(() => count.value > 1)

const displayPrice = computed(() => formatPrice(getDisplayPrice(props.commodity)))
const pricePerUnit = computed(() =>
  formatPrice(calculatePricePerUnit(props.commodity)),
)

const purchaseDateLabel = computed(() => {
  const d = props.commodity.attributes.purchase_date
  if (!d) return ''
  return new Date(d).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
})

const locationLabel = computed(() => {
  if (!props.showLocation) return ''
  const areaId = props.commodity.attributes.area_id
  if (!areaId) return ''
  const area = props.areaMap[areaId]
  if (!area) return 'Unknown Location / Unknown Area'
  const location = props.locationMap[area.locationId]
  return `${location?.name ?? 'Unknown Location'} / ${area.name}`
})

const statusModifierClass = computed(() => {
  const a = props.commodity.attributes
  if (a.draft) return 'draft opacity-70'
  if (a.status === 'sold') return 'sold opacity-70'
  if (a.status === 'lost') return 'lost'
  if (a.status === 'disposed') return 'disposed'
  if (a.status === 'written_off') return 'written-off opacity-80'
  return ''
})

function onCardClick() {
  emit('view', props.commodity.id)
}
function onCardKey(event: KeyboardEvent) {
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    emit('view', props.commodity.id)
  }
}
</script>

<template>
  <Card
    :class="
      cn(
        'commodity-card group relative cursor-pointer transition-all hover:-translate-y-0.5 hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        'flex-row items-start justify-between gap-4 px-5 py-4',
        isHighlighted && 'highlighted border-l-4 border-l-primary',
        statusModifierClass,
        props.class,
      )
    "
    :data-testid="testId ?? `commodity-card-${commodity.id}`"
    role="button"
    tabindex="0"
    @click="onCardClick"
    @keydown="onCardKey"
  >
    <div class="commodity-content min-w-0 flex-1">
      <h3 class="truncate text-base font-semibold text-foreground">
        {{ commodity.attributes.name }}
      </h3>

      <p
        v-if="showLocation && locationLabel"
        class="commodity-location mt-1 flex items-center gap-1 text-xs text-muted-foreground"
      >
        <MapPin class="size-3.5" aria-hidden="true" />
        <span class="truncate">{{ locationLabel }}</span>
      </p>

      <div
        v-if="!compact"
        class="commodity-meta mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground"
      >
        <span class="type flex items-center gap-1">
          <component :is="typeIcon" class="size-3.5" aria-hidden="true" />
          {{ typeLabel }}
        </span>
        <span v-if="showCount" class="count tabular-nums">×{{ count }}</span>
      </div>

      <p
        v-if="!compact && purchaseDateLabel"
        class="commodity-purchase-date mt-1 flex items-center gap-1 text-xs text-muted-foreground"
      >
        <Calendar class="size-3.5" aria-hidden="true" />
        {{ purchaseDateLabel }}
      </p>

      <div
        v-if="!compact"
        class="commodity-price mt-3 flex flex-col text-base font-semibold text-foreground"
      >
        <span class="price tabular-nums">{{ displayPrice }}</span>
        <span
          v-if="showCount"
          class="price-per-unit text-xs font-normal italic text-muted-foreground"
        >
          {{ pricePerUnit }} per unit
        </span>
      </div>

      <div v-if="status" class="commodity-status mt-2">
        <CommodityStatusPill :status="status" />
      </div>
    </div>

    <div v-if="showActions" class="commodity-actions flex shrink-0 items-center gap-1">
      <IconButton
        aria-label="Edit commodity"
        :test-id="`commodity-card-${commodity.id}-edit`"
        @click.stop="emit('edit', commodity.id)"
      >
        <Pencil class="size-4" aria-hidden="true" />
      </IconButton>
      <IconButton
        aria-label="Delete commodity"
        :test-id="`commodity-card-${commodity.id}-delete`"
        class="text-destructive hover:text-destructive"
        @click.stop="emit('delete', commodity.id)"
      >
        <Trash2 class="size-4" aria-hidden="true" />
      </IconButton>
    </div>
  </Card>
</template>
