<script setup lang="ts">
/**
 * CommodityListView — migrated to the design system in Phase 4 of
 * Epic #1324 (issue #1329).
 *
 * Page chrome (header, grand-total card, drafts toggle, empty
 * states, commodity grid) is built from `@design/*` patterns.
 * Per-card layout / status modifiers live inside `CommodityCard`
 * (`@design/patterns/CommodityCard.vue`); this view only wires
 * data, navigation, and inline filtering.
 *
 * Legacy CSS class anchors (`commodity-list`, `total-value`,
 * `commodities-grid`, `commodities-grid-container`,
 * `new-commodity-button`) are preserved as no-op markers so
 * existing Playwright selectors keep resolving through the
 * strangler-fig migration window — see
 * devdocs/frontend/migration-conventions.md.
 */
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Plus } from 'lucide-vue-next'

import commodityService from '@/services/commodityService'
import areaService from '@/services/areaService'
import locationService from '@/services/locationService'
import valueService from '@/services/valueService'
import { useSettingsStore } from '@/stores/settingsStore'
import { useGroupStore } from '@/stores/groupStore'
import { COMMODITY_STATUS_IN_USE } from '@/constants/commodityStatuses'
import { formatPrice } from '@/services/currencyService'
import { fetchAll } from '@/utils/paginationUtils'
import { getErrorMessage } from '@/utils/errorUtils'

import { Button } from '@design/ui/button'
import { Card } from '@design/ui/card'
import { Label } from '@design/ui/label'
import { Switch } from '@design/ui/switch'
import CommodityCard from '@design/patterns/CommodityCard.vue'
import EmptyState from '@design/patterns/EmptyState.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import { useAppToast } from '@design/composables/useAppToast'
import { useConfirm } from '@design/composables/useConfirm'

import PaginationControls from '@/components/PaginationControls.vue'

type AnyRecord = Record<string, unknown>
type ApiResource = { id: string; attributes: AnyRecord }

const router = useRouter()
const route = useRoute()
const groupStore = useGroupStore()
const settingsStore = useSettingsStore()
const toast = useAppToast()
const { confirmDelete } = useConfirm()

const commodities = ref<ApiResource[]>([])
const areas = ref<ApiResource[]>([])
const locations = ref<ApiResource[]>([])
const loading = ref<boolean>(true)

const currentPage = ref(1)
const pageSize = ref(50)
const totalCommodities = ref(0)
const totalPages = computed(() => Math.ceil(totalCommodities.value / pageSize.value))

const globalTotal = ref<number>(0)
const valuesLoading = ref<boolean>(true)

const mainCurrency = computed(() => settingsStore.mainCurrency)

const highlightCommodityId = ref((route.query.highlightCommodityId as string) || '')
let highlightTimeout: number | null = null

const showInactiveItems = ref(false)

const hasLocationsAndAreas = computed(
  () => locations.value.length > 0 && areas.value.length > 0,
)

const filteredCommodities = computed(() => {
  if (showInactiveItems.value) return commodities.value
  return commodities.value.filter((commodity) => {
    const a = commodity.attributes as AnyRecord
    return !a.draft && a.status === COMMODITY_STATUS_IN_USE
  })
})

const areaMap = ref<Record<string, { name: string; locationId: string }>>({})
const locationMap = ref<Record<string, { name: string }>>({})

async function loadValues() {
  valuesLoading.value = true
  try {
    const response = await valueService.getValues()
    const data = response.data.data.attributes as AnyRecord
    if (data.global_total !== undefined && data.global_total !== null) {
      globalTotal.value =
        typeof data.global_total === 'string'
          ? parseFloat(data.global_total)
          : (data.global_total as number)
    }
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'value', 'Failed to load inventory values'))
  } finally {
    valuesLoading.value = false
  }
}

let loadSeq = 0

async function loadLookups() {
  const [allAreas, allLocations] = await Promise.all([
    fetchAll((params) => areaService.getAreas(params)),
    fetchAll((params) => locationService.getLocations(params)),
  ])

  areas.value = allAreas
  locations.value = allLocations

  areaMap.value = {}
  for (const area of allAreas) {
    const a = area.attributes as AnyRecord
    areaMap.value[area.id] = {
      name: (a.name as string) ?? '',
      locationId: (a.location_id as string) ?? '',
    }
  }

  locationMap.value = {}
  for (const location of allLocations) {
    const a = location.attributes as AnyRecord
    locationMap.value[location.id] = { name: (a.name as string) ?? '' }
  }
}

async function loadCommodities() {
  const seq = ++loadSeq
  loading.value = true
  try {
    const response = await commodityService.getCommodities({
      page: currentPage.value,
      per_page: pageSize.value,
    })

    if (seq !== loadSeq) return

    commodities.value = response.data.data
    totalCommodities.value = response.data.meta.commodities
    loading.value = false

    if (highlightCommodityId.value) {
      nextTick(() => {
        const el = document.querySelector('.commodity-card.highlighted')
        if (!el) return
        el.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
        highlightTimeout = window.setTimeout(() => {
          highlightCommodityId.value = ''
        }, 3000)
      })
    }
  } catch (err) {
    if (seq !== loadSeq) return
    loading.value = false
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to load commodities'))
  }
}

onMounted(async () => {
  await settingsStore.fetchMainCurrency()
  currentPage.value = Number(route.query.page) || 1
  await Promise.all([loadLookups(), loadCommodities(), loadValues()])
})

watch(
  () => route.query.page,
  (newPage) => {
    currentPage.value = Number(newPage) || 1
    loadCommodities()
  },
)

onBeforeUnmount(() => {
  if (highlightTimeout !== null) {
    window.clearTimeout(highlightTimeout)
    highlightTimeout = null
  }
})

function viewCommodity(id: string) {
  router.push({
    path: groupStore.groupPath(`/commodities/${id}`),
    query: { source: 'commodities' },
  })
}

function editCommodity(id: string) {
  router.push({
    path: groupStore.groupPath(`/commodities/${id}/edit`),
    query: { source: 'commodities', directEdit: 'true' },
  })
}

async function onDeleteCommodity(id: string) {
  const confirmed = await confirmDelete('commodity')
  if (!confirmed) return
  try {
    await commodityService.deleteCommodity(id)
    await loadCommodities()
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to delete commodity'))
  }
}

function goToCommodityCreate() {
  router.push(groupStore.groupPath('/commodities/new'))
}

function goToLocations() {
  router.push(groupStore.groupPath('/locations'))
}
</script>

<template>
  <PageContainer as="div" class="commodity-list">
    <PageHeader title="Commodities">
      <template #actions>
        <Button
          v-if="loading"
          disabled
          class="new-commodity-button"
        >
          <Plus class="size-4" aria-hidden="true" />
          Loading...
        </Button>
        <Button
          v-else-if="hasLocationsAndAreas"
          class="new-commodity-button"
          @click="goToCommodityCreate"
        >
          <Plus class="size-4" aria-hidden="true" />
          New
        </Button>
        <Button
          v-else-if="locations.length === 0"
          class="new-commodity-button"
          @click="goToLocations"
        >
          <Plus class="size-4" aria-hidden="true" />
          Create Location First
        </Button>
        <Button
          v-else
          class="new-commodity-button"
          @click="goToLocations"
        >
          <Plus class="size-4" aria-hidden="true" />
          Create Area First
        </Button>
      </template>
    </PageHeader>

    <Card
      v-if="!valuesLoading && globalTotal > 0"
      class="total-value mb-6 border-l-4 border-l-primary p-6"
    >
      <div class="flex items-center justify-between">
        <h3 class="m-0 text-base font-semibold text-foreground">Total Value</h3>
        <div class="value-amount text-2xl font-bold text-primary">
          {{ formatPrice(globalTotal, mainCurrency) }}
        </div>
      </div>
    </Card>

    <div class="mb-4 flex items-center gap-2">
      <Switch
        id="commodity-list-show-inactive"
        v-model="showInactiveItems"
        data-testid="commodity-list-show-inactive"
      />
      <Label
        for="commodity-list-show-inactive"
        class="cursor-pointer text-sm text-muted-foreground"
      >
        Show drafts &amp; inactive items
      </Label>
    </div>

    <div v-if="loading" class="py-12 text-center text-sm text-muted-foreground">Loading...</div>

    <EmptyState
      v-else-if="commodities.length === 0 && locations.length === 0"
      title="No locations yet"
      description="No locations found. You need to create a location first before you can create commodities."
    >
      <template #actions>
        <Button @click="goToLocations">
          <Plus class="size-4" aria-hidden="true" />
          Create Location
        </Button>
      </template>
    </EmptyState>

    <EmptyState
      v-else-if="commodities.length === 0 && areas.length === 0"
      title="No areas yet"
      description="No areas found. You need to create an area in a location before you can create commodities."
    >
      <template #actions>
        <Button @click="goToLocations">
          <Plus class="size-4" aria-hidden="true" />
          Create Area
        </Button>
      </template>
    </EmptyState>

    <EmptyState
      v-else-if="commodities.length === 0"
      title="No commodities yet"
      description="No commodities found. Create your first commodity!"
    >
      <template #actions>
        <Button @click="goToCommodityCreate">
          <Plus class="size-4" aria-hidden="true" />
          Create Commodity
        </Button>
      </template>
    </EmptyState>

    <div v-else class="commodities-grid-container flex flex-col gap-6">
      <div
        class="commodities-grid grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3"
      >
        <CommodityCard
          v-for="commodity in filteredCommodities"
          :key="commodity.id"
          :commodity="(commodity as never)"
          :highlight-commodity-id="highlightCommodityId"
          show-location
          :area-map="areaMap"
          :location-map="locationMap"
          @view="viewCommodity"
          @edit="editCommodity"
          @delete="onDeleteCommodity"
        />
      </div>

      <PaginationControls
        :current-page="currentPage"
        :total-pages="totalPages"
        :page-size="pageSize"
        :total-items="totalCommodities"
        item-label="commodities"
      />
    </div>
  </PageContainer>
</template>
