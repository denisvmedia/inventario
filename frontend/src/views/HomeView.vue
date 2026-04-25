<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Box, FileBox, Files, Layers, MapPin, Wallet } from 'lucide-vue-next'

import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import StatCard from '@design/patterns/StatCard.vue'
import ValueByGroupingCard, {
  type ValueByGroupingItem,
} from '@design/patterns/ValueByGroupingCard.vue'

import areaService from '@/services/areaService'
import commodityService from '@/services/commodityService'
import fileService from '@/services/fileService'
import locationService from '@/services/locationService'
import valueService from '@/services/valueService'
import { formatPrice } from '@/services/currencyService'
import { useGroupStore } from '@/stores/groupStore'
import { useSettingsStore } from '@/stores/settingsStore'
import { fetchAll } from '@/utils/paginationUtils'

type TotalsMap = Record<string, string | number>

const settingsStore = useSettingsStore()
const groupStore = useGroupStore()

const valuesLoading = ref(true)
const countsLoading = ref(true)
const groupingsLoading = ref(true)

const globalTotal = ref<number>(0)
const locationTotals = ref<TotalsMap>({})
const areaTotals = ref<TotalsMap>({})
const locationsCount = ref<number>(0)
const areasCount = ref<number>(0)
const commoditiesCount = ref<number>(0)
const filesCount = ref<number>(0)
const locationNames = ref<Record<string, string>>({})
const areaNames = ref<Record<string, string>>({})

const mainCurrency = computed(() => settingsStore.mainCurrency)
const groupName = computed(() => groupStore.currentGroupName ?? '')

function normalizeTotals(input: unknown): TotalsMap {
  if (input && typeof input === 'object') {
    if (Array.isArray(input)) {
      const out: TotalsMap = {}
      for (const entry of input) {
        if (entry && typeof entry === 'object' && 'id' in entry) {
          out[String((entry as { id: unknown }).id)] = (entry as { value: string | number }).value
        }
      }
      return out
    }
    return input as TotalsMap
  }
  return {}
}

function toItems(map: TotalsMap, names: Record<string, string>): ValueByGroupingItem[] {
  const currency = mainCurrency.value
  return Object.entries(map)
    .map(([id, raw]) => {
      const n = typeof raw === 'string' ? parseFloat(raw) : raw
      return { id, name: names[id] ?? 'Unknown', amount: isNaN(n) ? 0 : n }
    })
    .sort((a, b) => b.amount - a.amount)
    .slice(0, 8)
    .map(({ id, name, amount }) => ({ id, name, value: formatPrice(amount, currency) }))
}

const valueByLocation = computed(() => toItems(locationTotals.value, locationNames.value))
const valueByArea = computed(() => toItems(areaTotals.value, areaNames.value))

const formattedGlobalTotal = computed(() =>
  formatPrice(globalTotal.value, mainCurrency.value),
)

async function loadValues() {
  valuesLoading.value = true
  try {
    const response = await valueService.getValues()
    const data = response.data.data.attributes
    if (data.global_total !== undefined && data.global_total !== null) {
      globalTotal.value =
        typeof data.global_total === 'string'
          ? parseFloat(data.global_total)
          : data.global_total
    }
    locationTotals.value = normalizeTotals(data.location_totals)
    areaTotals.value = normalizeTotals(data.area_totals)
  } catch (err) {
    console.error('Error loading values:', err)
  } finally {
    valuesLoading.value = false
  }
}

async function loadCounts() {
  countsLoading.value = true
  try {
    const [commResp, fileResp] = await Promise.all([
      commodityService.getCommodities({ page: 1, per_page: 1 }),
      fileService.getFiles({ page: 1, limit: 1 }),
    ])
    commoditiesCount.value = Number(commResp.data?.meta?.commodities ?? 0)
    filesCount.value = Number(fileResp.data?.meta?.total ?? 0)
  } catch (err) {
    console.error('Error loading counts:', err)
  } finally {
    countsLoading.value = false
  }
}

async function loadGroupings() {
  groupingsLoading.value = true
  try {
    const [locations, areas] = await Promise.all([
      fetchAll((params) => locationService.getLocations(params)),
      fetchAll((params) => areaService.getAreas(params)),
    ])
    locationsCount.value = locations.length
    areasCount.value = areas.length

    const lNames: Record<string, string> = {}
    for (const l of locations) lNames[l.id] = l.attributes?.name ?? 'Unknown'
    locationNames.value = lNames

    const aNames: Record<string, string> = {}
    for (const a of areas) aNames[a.id] = a.attributes?.name ?? 'Unknown'
    areaNames.value = aNames
  } catch (err) {
    console.error('Error loading groupings:', err)
  } finally {
    groupingsLoading.value = false
  }
}

onMounted(async () => {
  if (!mainCurrency.value) {
    await settingsStore.fetchMainCurrency()
  }
  // Fire all three in parallel; each card paints its own loading state.
  void loadValues()
  void loadCounts()
  void loadGroupings()
})
</script>

<template>
  <PageContainer>
    <PageHeader title="Welcome to Inventario">
      <template #description>
        <span v-if="groupName">
          You are looking at {{ groupName }}.
        </span>
        <span v-else>
          A modern inventory management system.
        </span>
      </template>
    </PageHeader>

    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
      <StatCard
        label="Total Inventory Value"
        :value="formattedGlobalTotal"
        :icon="Wallet"
        :loading="valuesLoading"
        variant="primary"
        test-id="dashboard-total-value"
        class="lg:col-span-3 xl:col-span-2"
      />
      <StatCard
        label="Locations"
        :value="locationsCount"
        :icon="MapPin"
        :loading="groupingsLoading"
        test-id="dashboard-locations-count"
      />
      <StatCard
        label="Areas"
        :value="areasCount"
        :icon="Layers"
        :loading="groupingsLoading"
        test-id="dashboard-areas-count"
      />
      <StatCard
        label="Commodities"
        :value="commoditiesCount"
        :icon="Box"
        :loading="countsLoading"
        test-id="dashboard-commodities-count"
      />
      <StatCard
        label="Files"
        :value="filesCount"
        :icon="Files"
        :loading="countsLoading"
        test-id="dashboard-files-count"
      />
      <StatCard
        label="Avg. value per commodity"
        :value="
          !valuesLoading && !countsLoading && commoditiesCount > 0
            ? formatPrice(globalTotal / commoditiesCount, mainCurrency)
            : undefined
        "
        :icon="FileBox"
        :loading="valuesLoading || countsLoading"
        description="Approximation across all commodities."
        test-id="dashboard-avg-value"
      />
    </div>

    <div class="mt-6 grid grid-cols-1 gap-4 lg:grid-cols-2">
      <ValueByGroupingCard
        title="Value by Location"
        :items="valueByLocation"
        :loading="valuesLoading || groupingsLoading"
        empty="No location values yet."
        test-id="dashboard-value-by-location"
      />
      <ValueByGroupingCard
        title="Value by Area"
        :items="valueByArea"
        :loading="valuesLoading || groupingsLoading"
        empty="No area values yet."
        test-id="dashboard-value-by-area"
      />
    </div>
  </PageContainer>
</template>
