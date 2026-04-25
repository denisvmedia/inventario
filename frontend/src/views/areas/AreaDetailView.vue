<script setup lang="ts">
/**
 * AreaDetailView — migrated to the design system in Phase 4 of
 * Epic #1324 (issue #1329).
 *
 * Replaces the legacy SCSS shell + ad-hoc dialogs / notifications /
 * commodity list-item with the `@design/*` patterns. The legacy
 * `.area-detail`, `.commodities-grid` and `.no-commodities` class
 * anchors are preserved so existing Playwright selectors keep
 * resolving through the strangler-fig migration window — see
 * devdocs/frontend/migration-conventions.md.
 */
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Plus, Trash2 } from 'lucide-vue-next'

import areaService from '@/services/areaService'
import locationService from '@/services/locationService'
import commodityService from '@/services/commodityService'
import valueService from '@/services/valueService'
import { COMMODITY_STATUS_IN_USE } from '@/constants/commodityStatuses'
import {
  calculatePricePerUnit,
  formatPrice,
  getDisplayPrice,
  getMainCurrency,
} from '@/services/currencyService'
import { useGroupStore } from '@/stores/groupStore'
import {
  is404Error as checkIs404Error,
  get404Message,
  get404Title,
  getErrorMessage,
} from '@/utils/errorUtils'

import { Button } from '@design/ui/button'
import { Switch } from '@design/ui/switch'
import Banner from '@design/patterns/Banner.vue'
import CommodityCard from '@design/patterns/CommodityCard.vue'
import EmptyState from '@design/patterns/EmptyState.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import PageSection from '@design/patterns/PageSection.vue'
import { useAppToast } from '@design/composables/useAppToast'
import { useConfirm } from '@design/composables/useConfirm'

type AnyRecord = Record<string, unknown>
type ApiResource = { id: string; attributes: AnyRecord }

const router = useRouter()
const route = useRoute()
const groupStore = useGroupStore()
const toast = useAppToast()
const { confirmDelete } = useConfirm()

const area = ref<ApiResource | null>(null)
const commodities = ref<ApiResource[]>([])
const loading = ref<boolean>(true)
const lastError = ref<unknown>(null)
const locationName = ref<string | null>(null)
const locationAddress = ref<string | null>(null)
const areaTotalValue = ref<number>(0)
const showInactiveItems = ref(false)
const highlightCommodityId = ref<string>((route.query.highlightCommodityId as string) || '')
let highlightTimeout: number | null = null

const is404 = computed(() => !!lastError.value && checkIs404Error(lastError.value as never))

const filteredCommodities = computed(() => {
  if (showInactiveItems.value) return commodities.value
  return commodities.value.filter((c) => {
    const a = c.attributes as AnyRecord
    return !a.draft && a.status === COMMODITY_STATUS_IN_USE
  })
})

onMounted(() => loadArea())

onBeforeUnmount(() => {
  if (highlightTimeout !== null) {
    window.clearTimeout(highlightTimeout)
    highlightTimeout = null
  }
})

async function loadArea() {
  const id = route.params.id as string
  loading.value = true
  lastError.value = null
  try {
    const [areaResponse, locationsResponse, commoditiesResponse, valuesResponse] = await Promise.all([
      areaService.getArea(id),
      locationService.getLocations(),
      commodityService.getCommodities(),
      valueService.getValues(),
    ])

    area.value = areaResponse.data.data
    const locationId = (area.value!.attributes as AnyRecord).location_id as string | undefined
    if (locationId) {
      const loc = locationsResponse.data.data.find((l: ApiResource) => l.id === locationId)
      if (loc) {
        const a = loc.attributes as AnyRecord
        locationName.value = (a.name as string) ?? null
        locationAddress.value = (a.address as string) ?? null
      }
    }

    commodities.value = commoditiesResponse.data.data.filter(
      (c: ApiResource) => (c.attributes as AnyRecord).area_id === id,
    )

    areaTotalValue.value = computeAreaTotal(valuesResponse, id, commodities.value)
    loading.value = false
    if (highlightCommodityId.value) scheduleHighlightClear()
  } catch (err) {
    lastError.value = err
    if (!checkIs404Error(err as never)) {
      toast.error(getErrorMessage(err as never, 'area', 'Failed to load area'))
    }
    loading.value = false
  }
}

function scheduleHighlightClear() {
  nextTick(() => {
    const el = document.querySelector('.commodity-card.highlighted')
    if (!el) return
    el.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
    highlightTimeout = window.setTimeout(() => {
      highlightCommodityId.value = ''
    }, 3000)
  })
}

function computeAreaTotal(valuesResponse: { data?: { data?: { attributes?: AnyRecord } } }, id: string, items: ApiResource[]): number {
  try {
    const valueAttributes = (valuesResponse?.data?.data?.attributes ?? {}) as AnyRecord
    const areaTotals = (valueAttributes.area_totals ?? []) as unknown
    let areaValue: { value: string } | null = null
    if (Array.isArray(areaTotals)) {
      areaValue = (areaTotals as Array<{ id: string; value: string }>).find((x) => x.id === id) ?? null
    } else if (areaTotals && typeof areaTotals === 'object' && (areaTotals as Record<string, string>)[id]) {
      areaValue = { value: (areaTotals as Record<string, string>)[id] }
    }
    if (areaValue) return parseFloat(areaValue.value)
  } catch {
    // fall through to per-item calculation below
  }
  return items.reduce((total, c) => {
    const a = c.attributes as AnyRecord
    if (a.status === 'in_use' && !a.draft) {
      const price = getDisplayPrice(c)
      if (!isNaN(price)) return total + price
    }
    return total
  }, 0)
}

async function onDeleteArea() {
  if (!area.value) return
  const confirmed = await confirmDelete('area')
  if (!confirmed) return
  try {
    await areaService.deleteArea(area.value.id)
    router.push(groupStore.groupPath('/locations'))
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'area', 'Failed to delete area'))
  }
}

function priceForCommodity(c: ApiResource): string | undefined {
  const price = getDisplayPrice(c as never)
  if (isNaN(price)) return undefined
  return formatPrice(price)
}

function pricePerUnitFor(c: ApiResource): string | undefined {
  const count = ((c.attributes as AnyRecord).count as number) || 1
  if (count <= 1) return undefined
  const ppu = calculatePricePerUnit(c as never)
  if (isNaN(ppu)) return undefined
  return formatPrice(ppu)
}

function viewCommodity(id: string) {
  if (!area.value) return
  router.push({
    path: groupStore.groupPath(`/commodities/${id}`),
    query: { source: 'area', areaId: area.value.id },
  })
}

function editCommodity(id: string) {
  if (!area.value) return
  router.push({
    path: groupStore.groupPath(`/commodities/${id}/edit`),
    query: { source: 'area', areaId: area.value.id, directEdit: 'true' },
  })
}

async function onDeleteCommodity(id: string) {
  const confirmed = await confirmDelete('commodity')
  if (!confirmed) return
  try {
    await commodityService.deleteCommodity(id)
    commodities.value = commodities.value.filter((c) => c.id !== id)
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to delete commodity'))
  }
}

function navigateToLocations() {
  if (!area.value) return
  router.push({
    path: groupStore.groupPath('/locations'),
    query: {
      areaId: area.value.id,
      locationId: (area.value.attributes as AnyRecord).location_id as string,
    },
  })
}

function goBackToList() {
  router.push(groupStore.groupPath('/locations'))
}

function goToNewCommodity() {
  if (!area.value) return
  router.push(groupStore.groupPath(`/commodities/new?area=${area.value.id}`))
}

const locationLine = computed(() => {
  const name = locationName.value || 'No location'
  return locationAddress.value ? `${name} — ${locationAddress.value}` : name
})

const totalValueLabel = computed(() =>
  areaTotalValue.value > 0 ? formatPrice(areaTotalValue.value, getMainCurrency()) : '',
)
</script>

<template>
  <PageContainer as="div" class="area-detail">
    <div v-if="loading" class="py-12 text-center text-sm text-muted-foreground">Loading...</div>

    <EmptyState
      v-else-if="is404"
      :title="get404Title('area')"
      :description="get404Message('area')"
    >
      <template #actions>
        <Button variant="outline" @click="goBackToList">
          <ArrowLeft class="size-4" aria-hidden="true" />
          Back to Locations
        </Button>
        <Button @click="loadArea">Try Again</Button>
      </template>
    </EmptyState>

    <Banner v-else-if="!area" variant="warning">Area not found</Banner>

    <template v-else>
      <PageHeader :title="(area.attributes.name as string)" :description="locationLine">
        <template #breadcrumbs>
          <a
            href="#"
            class="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
            @click.prevent="navigateToLocations"
          >
            <ArrowLeft class="size-4" aria-hidden="true" />
            Back to Locations
          </a>
        </template>
        <template #description>
          <span>{{ locationLine }}</span>
          <span v-if="totalValueLabel" class="ml-2 font-medium text-foreground">
            · Total Value: {{ totalValueLabel }}
          </span>
        </template>
        <template #actions>
          <Button variant="destructive" @click="onDeleteArea">
            <Trash2 class="size-4" aria-hidden="true" />
            Delete
          </Button>
        </template>
      </PageHeader>

      <PageSection title="Commodities">
        <template #actions>
          <label class="flex items-center gap-2 text-sm text-muted-foreground">
            <Switch v-model:checked="showInactiveItems" />
            <span>Show drafts &amp; inactive items</span>
          </label>
          <Button size="sm" @click="goToNewCommodity">
            <Plus class="size-4" aria-hidden="true" />
            New
          </Button>
        </template>

        <div
          v-if="filteredCommodities.length > 0"
          class="commodities-grid grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3"
        >
          <CommodityCard
            v-for="commodity in filteredCommodities"
            :key="commodity.id"
            :name="(commodity.attributes as AnyRecord).name as string"
            :type="(commodity.attributes as AnyRecord).type as string"
            :status="(commodity.attributes as AnyRecord).status as never"
            :draft="(commodity.attributes as AnyRecord).draft as boolean"
            :count="(commodity.attributes as AnyRecord).count as number"
            :purchase-date="(commodity.attributes as AnyRecord).purchase_date as string"
            :display-price="priceForCommodity(commodity)"
            :price-per-unit="pricePerUnitFor(commodity)"
            :highlighted="commodity.id === highlightCommodityId"
            :data-commodity-id="commodity.id"
            @view="viewCommodity(commodity.id)"
            @edit="editCommodity(commodity.id)"
            @delete="onDeleteCommodity(commodity.id)"
          />
        </div>
        <div v-else class="no-commodities">
          <EmptyState
            title="No commodities yet"
            description="No commodities found in this area. Add your first one to get started."
          >
            <template #actions>
              <Button @click="goToNewCommodity">
                <Plus class="size-4" aria-hidden="true" />
                Add Commodity
              </Button>
            </template>
          </EmptyState>
        </div>
      </PageSection>
    </template>
  </PageContainer>
</template>

