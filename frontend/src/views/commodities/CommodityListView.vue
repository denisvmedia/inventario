<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Plus } from 'lucide-vue-next'

import { Button } from '@design/ui/button'
import { Switch } from '@design/ui/switch'
import { Label } from '@design/ui/label'
import CommodityCard from '@design/patterns/CommodityCard.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import { useAppToast } from '@design/composables/useAppToast'

import Confirmation from '@/components/Confirmation.vue'
import PaginationControls from '@/components/PaginationControls.vue'

import areaService from '@/services/areaService'
import commodityService from '@/services/commodityService'
import locationService from '@/services/locationService'
import valueService from '@/services/valueService'
import { COMMODITY_STATUS_IN_USE } from '@/constants/commodityStatuses'
import {
  calculatePricePerUnit,
  formatPrice,
  getDisplayPrice,
} from '@/services/currencyService'
import { useGroupStore } from '@/stores/groupStore'
import { useSettingsStore } from '@/stores/settingsStore'
import { fetchAll } from '@/utils/paginationUtils'

const route = useRoute()
const router = useRouter()
const groupStore = useGroupStore()
const settingsStore = useSettingsStore()
const toast = useAppToast()

const commodities = ref<any[]>([])
const areas = ref<any[]>([])
const locations = ref<any[]>([])
const loading = ref(true)
const error = ref<string | null>(null)

const currentPage = ref(1)
const pageSize = ref(50)
const totalCommodities = ref(0)
const totalPages = computed(() =>
  Math.ceil(totalCommodities.value / pageSize.value),
)

const globalTotal = ref(0)
const valuesLoading = ref(true)

const mainCurrency = computed(() => settingsStore.mainCurrency)

const highlightCommodityId = ref(
  (route.query.highlightCommodityId as string) || '',
)
let highlightTimeout: number | null = null

const showInactiveItems = ref(false)

const hasLocationsAndAreas = computed(
  () => locations.value.length > 0 && areas.value.length > 0,
)

const filteredCommodities = computed(() => {
  if (showInactiveItems.value) return commodities.value
  return commodities.value.filter(
    (c) =>
      !c.attributes.draft && c.attributes.status === COMMODITY_STATUS_IN_USE,
  )
})

const areaMap = ref<Record<string, { name: string; locationId: string }>>({})
const locationMap = ref<
  Record<string, { name: string; address?: string }>
>({})

function getLocationName(areaId: string): string {
  const locId = areaMap.value[areaId]?.locationId
  if (!locId) return ''
  return locationMap.value[locId]?.name ?? ''
}
function getAreaName(areaId: string): string {
  return areaMap.value[areaId]?.name ?? ''
}

async function loadValues() {
  valuesLoading.value = true
  try {
    const response = await valueService.getValues()
    const data = response.data.data.attributes
    globalTotal.value =
      typeof data.global_total === 'string'
        ? parseFloat(data.global_total)
        : data.global_total ?? 0
  } catch (err: any) {
    console.error('Error loading values:', err)
  } finally {
    valuesLoading.value = false
  }
}

async function loadLookups() {
  const [allAreas, allLocations] = await Promise.all([
    fetchAll((params) => areaService.getAreas(params)),
    fetchAll((params) => locationService.getLocations(params)),
  ])
  areas.value = allAreas
  locations.value = allLocations

  const aMap: Record<string, { name: string; locationId: string }> = {}
  for (const a of allAreas) {
    aMap[a.id] = {
      name: a.attributes.name,
      locationId: a.attributes.location_id,
    }
  }
  areaMap.value = aMap

  const lMap: Record<string, { name: string; address?: string }> = {}
  for (const l of allLocations) {
    lMap[l.id] = {
      name: l.attributes.name,
      address: l.attributes.address,
    }
  }
  locationMap.value = lMap
}

let loadSeq = 0
async function loadCommodities() {
  const seq = ++loadSeq
  loading.value = true
  error.value = null
  try {
    const resp = await commodityService.getCommodities({
      page: currentPage.value,
      per_page: pageSize.value,
    })
    if (seq !== loadSeq) return
    commodities.value = resp.data.data
    totalCommodities.value = resp.data.meta.commodities

    if (highlightCommodityId.value) {
      nextTick(() => {
        const el = document.querySelector(
          `[data-commodity-id="${highlightCommodityId.value}"]`,
        )
        if (el) {
          el.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
          highlightTimeout = window.setTimeout(() => {
            highlightCommodityId.value = ''
          }, 3000)
        }
      })
    }
  } catch (err: any) {
    if (seq !== loadSeq) return
    error.value = err?.message ?? 'Failed to load commodities'
    toast.error(error.value)
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

onMounted(async () => {
  await settingsStore.fetchMainCurrency()
  currentPage.value = Number(route.query.page) || 1
  await Promise.all([loadLookups(), loadCommodities(), loadValues()])
})

watch(
  () => route.query.page,
  (np) => {
    currentPage.value = Number(np) || 1
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

const commodityToDelete = ref<string | null>(null)
const showDeleteDialog = ref(false)

function confirmDelete(id: string) {
  commodityToDelete.value = id
  showDeleteDialog.value = true
}

async function onConfirmDelete() {
  const id = commodityToDelete.value
  showDeleteDialog.value = false
  commodityToDelete.value = null
  if (!id) return
  try {
    await commodityService.deleteCommodity(id)
    await loadCommodities()
  } catch (err: any) {
    toast.error(err?.message ?? 'Failed to delete commodity')
  }
}

function onCancelDelete() {
  showDeleteDialog.value = false
  commodityToDelete.value = null
}

function priceForCommodity(c: any): string | undefined {
  const price = getDisplayPrice(c)
  if (isNaN(price)) return undefined
  return formatPrice(price)
}

function pricePerUnitFor(c: any): string | undefined {
  const count = c.attributes.count || 1
  if (count <= 1) return undefined
  const ppu = calculatePricePerUnit(c)
  if (isNaN(ppu)) return undefined
  return formatPrice(ppu)
}

const newCommodityHref = computed(() => groupStore.groupPath('/commodities/new'))
const goCreateLocationHref = computed(() => groupStore.groupPath('/locations'))
</script>

<template>
  <PageContainer>
    <PageHeader title="Commodities">
      <template #description>
        <span
          v-if="!valuesLoading && globalTotal > 0"
          class="inline-flex items-center gap-1"
        >
          Total Value:
          <span class="font-semibold text-foreground">
            {{ formatPrice(globalTotal, mainCurrency) }}
          </span>
        </span>
      </template>
      <template #actions>
        <div class="filter-toggle flex items-center gap-2">
          <Switch
            id="show-inactive-items"
            v-model="showInactiveItems"
            aria-label="Show drafts and inactive items"
          />
          <Label
            for="show-inactive-items"
            class="cursor-pointer text-sm text-muted-foreground"
          >
            Show drafts &amp; inactive items
          </Label>
        </div>

        <Button v-if="loading" disabled>
          <Plus class="size-4" aria-hidden="true" />
          Loading...
        </Button>
        <Button
          v-else-if="hasLocationsAndAreas"
          as-child
        >
          <router-link :to="newCommodityHref">
            <Plus class="size-4" aria-hidden="true" />
            New
          </router-link>
        </Button>
        <Button v-else-if="locations.length === 0" as-child>
          <router-link :to="goCreateLocationHref">
            <Plus class="size-4" aria-hidden="true" />
            Create Location First
          </router-link>
        </Button>
        <Button v-else-if="areas.length === 0" as-child>
          <router-link :to="goCreateLocationHref">
            <Plus class="size-4" aria-hidden="true" />
            Create Area First
          </router-link>
        </Button>
      </template>
    </PageHeader>

    <div
      v-if="loading"
      class="rounded-md border border-border bg-card p-12 text-center text-muted-foreground shadow-sm"
    >
      Loading...
    </div>

    <div
      v-else-if="error"
      class="rounded-md border border-destructive/50 bg-destructive/10 p-12 text-center text-destructive shadow-sm"
    >
      {{ error }}
    </div>

    <div
      v-else-if="commodities.length === 0"
      class="flex flex-col items-center gap-4 rounded-md border border-border bg-card p-12 text-center shadow-sm"
    >
      <template v-if="locations.length === 0">
        <p class="text-base">
          No locations found. You need to create a location first before you
          can create commodities.
        </p>
        <Button as-child>
          <router-link :to="goCreateLocationHref">Create Location</router-link>
        </Button>
      </template>
      <template v-else-if="areas.length === 0">
        <p class="text-base">
          No areas found. You need to create an area in a location before you
          can create commodities.
        </p>
        <Button as-child>
          <router-link :to="goCreateLocationHref">Create Area</router-link>
        </Button>
      </template>
      <template v-else>
        <p class="text-base">No commodities found. Create your first commodity!</p>
        <Button as-child>
          <router-link :to="newCommodityHref">Create Commodity</router-link>
        </Button>
      </template>
    </div>

    <div v-else class="flex flex-col gap-6">
      <div
        class="grid gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4"
      >
        <CommodityCard
          v-for="commodity in filteredCommodities"
          :key="commodity.id"
          :name="commodity.attributes.name"
          :type="commodity.attributes.type"
          :status="commodity.attributes.status"
          :draft="commodity.attributes.draft"
          :count="commodity.attributes.count"
          :purchase-date="commodity.attributes.purchase_date"
          :display-price="priceForCommodity(commodity)"
          :price-per-unit="pricePerUnitFor(commodity)"
          :location-name="getLocationName(commodity.attributes.area_id)"
          :area-name="getAreaName(commodity.attributes.area_id)"
          :highlighted="commodity.id === highlightCommodityId"
          :data-commodity-id="commodity.id"
          @view="viewCommodity(commodity.id)"
          @edit="editCommodity(commodity.id)"
          @delete="confirmDelete(commodity.id)"
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

    <Confirmation
      v-model:visible="showDeleteDialog"
      title="Confirm Delete"
      message="Are you sure you want to delete this commodity?"
      confirm-label="Delete"
      cancel-label="Cancel"
      confirm-button-class="danger"
      confirmation-icon="exclamation-triangle"
      @confirm="onConfirmDelete"
      @cancel="onCancelDelete"
    />
  </PageContainer>
</template>
