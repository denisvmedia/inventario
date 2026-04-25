<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Pencil, Plus, Trash2, X } from 'lucide-vue-next'

import { Button } from '@design/ui/button'
import LocationCard from '@design/patterns/LocationCard.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import { useAppToast } from '@design/composables/useAppToast'

import LocationForm from '@/components/LocationForm.vue'
import AreaForm from '@/components/AreaForm.vue'
import Confirmation from '@/components/Confirmation.vue'
import PaginationControls from '@/components/PaginationControls.vue'

import areaService from '@/services/areaService'
import locationService from '@/services/locationService'
import valueService from '@/services/valueService'
import { formatPrice } from '@/services/currencyService'
import { useGroupStore } from '@/stores/groupStore'
import { useSettingsStore } from '@/stores/settingsStore'
import { fetchAll } from '@/utils/paginationUtils'

const route = useRoute()
const router = useRouter()
const settingsStore = useSettingsStore()
const groupStore = useGroupStore()
const toast = useAppToast()

const locations = ref<any[]>([])
const areas = ref<any[]>([])
const loading = ref(true)
const showLocationForm = ref(false)
const showAreaFormForLocation = ref<string | null>(null)
const expandedLocations = ref<string[]>([])
const areaToFocus = ref<string | null>(null)

const currentPage = ref(1)
const pageSize = ref(50)
const totalLocations = ref(0)
const totalPages = computed(() => Math.ceil(totalLocations.value / pageSize.value))

const areaTotals = ref<any[]>([])
const locationTotals = ref<any[]>([])
const globalTotal = ref(0)
const valuesLoading = ref(true)

const mainCurrency = computed(() => settingsStore.mainCurrency)

const locationToDelete = ref<string | null>(null)
const areaToDelete = ref<string | null>(null)
const showDeleteLocationDialog = ref(false)
const showDeleteAreaDialog = ref(false)

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
    areaTotals.value = Array.isArray(data.area_totals) ? data.area_totals : []
    locationTotals.value = Array.isArray(data.location_totals) ? data.location_totals : []
  } catch (err: any) {
    console.error('Error loading values:', err)
  } finally {
    valuesLoading.value = false
  }
}

function getLocationValue(locationId: string): string {
  const lv = locationTotals.value.find((l) => l.id === locationId)
  if (lv?.value !== undefined && lv.value !== null) {
    const n = typeof lv.value === 'string' ? parseFloat(lv.value) : lv.value
    if (!isNaN(n)) return formatPrice(n, mainCurrency.value)
  }
  return formatPrice(0, mainCurrency.value)
}

function getAreaValue(areaId: string): string {
  const av = areaTotals.value.find((a) => a.id === areaId)
  if (av?.value !== undefined && av.value !== null) {
    const n = typeof av.value === 'string' ? parseFloat(av.value) : av.value
    if (!isNaN(n)) return formatPrice(n, mainCurrency.value)
  }
  return formatPrice(0, mainCurrency.value)
}

async function loadLocations() {
  loading.value = true
  try {
    const [locationsResponse, allAreas] = await Promise.all([
      locationService.getLocations({
        page: currentPage.value,
        per_page: pageSize.value,
      }),
      fetchAll((params) => areaService.getAreas(params)),
      loadValues(),
    ])

    locations.value = locationsResponse.data.data
    totalLocations.value = locationsResponse.data.meta.locations
    areas.value = allAreas

    const areaId = route.query.areaId as string | undefined
    const locId = route.query.locationId as string | undefined
    if (areaId && locId) {
      if (!expandedLocations.value.includes(locId)) {
        expandedLocations.value.push(locId)
      }
      areaToFocus.value = areaId
      await nextTick()
      scrollToArea(areaId)
    } else if (locations.value.length === 1) {
      expandedLocations.value = [locations.value[0].id]
    }
  } catch (err: any) {
    toast.error(err?.message ?? 'Failed to load locations')
  } finally {
    loading.value = false
  }
}

function scrollToArea(areaId: string) {
  const el = document.getElementById(`area-${areaId}`)
  if (!el) return
  el.scrollIntoView({ behavior: 'smooth', block: 'center' })
  el.classList.add('area-highlight')
  setTimeout(() => el.classList.remove('area-highlight'), 2000)
}

function toggleExpand(locationId: string) {
  expandedLocations.value = expandedLocations.value.includes(locationId)
    ? expandedLocations.value.filter((id) => id !== locationId)
    : [...expandedLocations.value, locationId]
}

function toggleAreaForm(locationId: string) {
  showAreaFormForLocation.value =
    showAreaFormForLocation.value === locationId ? null : locationId
}

function getAreasForLocation(locationId: string) {
  return areas.value.filter((a) => a.attributes.location_id === locationId)
}

async function handleLocationCreated(newLocation: any) {
  showLocationForm.value = false
  expandedLocations.value.push(newLocation.id)
  await loadLocations()
}

async function handleAreaCreated() {
  showAreaFormForLocation.value = null
  await loadLocations()
}

function viewLocation(id: string) {
  router.push(groupStore.groupPath(`/locations/${id}`))
}
function editLocation(id: string) {
  router.push(groupStore.groupPath(`/locations/${id}/edit`))
}
function viewArea(id: string) {
  router.push(groupStore.groupPath(`/areas/${id}`))
}
function editArea(id: string) {
  router.push(groupStore.groupPath(`/areas/${id}/edit`))
}

function confirmDeleteLocation(id: string) {
  locationToDelete.value = id
  showDeleteLocationDialog.value = true
}

async function onConfirmDeleteLocation() {
  const id = locationToDelete.value
  showDeleteLocationDialog.value = false
  locationToDelete.value = null
  if (!id) return
  try {
    await locationService.deleteLocation(id)
    expandedLocations.value = expandedLocations.value.filter((x) => x !== id)
    await loadLocations()
  } catch (err: any) {
    toast.error(err?.message ?? 'Failed to delete location')
  }
}

function onCancelDeleteLocation() {
  showDeleteLocationDialog.value = false
  locationToDelete.value = null
}

function confirmDeleteArea(id: string) {
  areaToDelete.value = id
  showDeleteAreaDialog.value = true
}

async function onConfirmDeleteArea() {
  const id = areaToDelete.value
  showDeleteAreaDialog.value = false
  areaToDelete.value = null
  if (!id) return
  try {
    await areaService.deleteArea(id)
    await loadLocations()
  } catch (err: any) {
    toast.error(err?.message ?? 'Failed to delete area')
  }
}

function onCancelDeleteArea() {
  showDeleteAreaDialog.value = false
  areaToDelete.value = null
}

onMounted(async () => {
  await settingsStore.fetchMainCurrency()
  currentPage.value = Number(route.query.page) || 1
  await loadLocations()
})

watch(
  () => route.query.page,
  (np) => {
    currentPage.value = Number(np) || 1
    loadLocations()
  },
)
</script>

<template>
  <PageContainer>
    <PageHeader title="Locations">
      <template #actions>
        <Button @click="showLocationForm = !showLocationForm">
          <component
            :is="showLocationForm ? X : Plus"
            class="size-4"
            aria-hidden="true"
          />
          {{ showLocationForm ? 'Cancel' : 'New' }}
        </Button>
      </template>
    </PageHeader>

    <div
      v-if="!valuesLoading && globalTotal > 0"
      class="mb-6 flex flex-wrap items-center justify-between gap-3 rounded-md border border-l-4 border-border border-l-primary bg-card p-6 shadow-sm"
    >
      <h3 class="text-base font-semibold text-foreground">
        Total Inventory Value
      </h3>
      <div class="text-2xl font-bold text-primary">
        {{ formatPrice(globalTotal, mainCurrency) }}
      </div>
    </div>

    <LocationForm
      v-if="showLocationForm"
      class="mb-6"
      @created="handleLocationCreated"
      @cancel="showLocationForm = false"
    />

    <div
      v-if="loading"
      class="rounded-md border border-border bg-card p-12 text-center text-muted-foreground shadow-sm"
    >
      Loading...
    </div>

    <div
      v-else-if="locations.length === 0"
      class="flex flex-col items-center gap-4 rounded-md border border-border bg-card p-12 text-center shadow-sm"
    >
      <p class="text-base">No locations found. Create your first location!</p>
      <Button @click="showLocationForm = true">Create Location</Button>
    </div>

    <div v-else class="flex flex-col gap-6">
      <div v-for="location in locations" :key="location.id" class="flex flex-col">
        <LocationCard
          :name="location.attributes.name"
          :address="location.attributes.address"
          :value-label="getLocationValue(location.id)"
          :loading-value="valuesLoading"
          :expanded="expandedLocations.includes(location.id)"
          :data-location-id="location.id"
          @toggle="toggleExpand(location.id)"
          @view="viewLocation(location.id)"
          @edit="editLocation(location.id)"
          @delete="confirmDeleteLocation(location.id)"
        />

        <div
          v-if="expandedLocations.includes(location.id)"
          class="ml-4 mt-2 rounded-md border-l-4 border-l-primary bg-muted/40 p-4 sm:ml-8 sm:p-6"
        >
          <div class="areas-header mb-4 flex items-center justify-between">
            <h4 class="text-base font-semibold text-foreground">Areas</h4>
            <Button size="sm" @click="toggleAreaForm(location.id)">
              {{ showAreaFormForLocation === location.id ? 'Cancel' : 'Add Area' }}
            </Button>
          </div>

          <AreaForm
            v-if="showAreaFormForLocation === location.id"
            :location-id="location.id"
            class="mb-4"
            @created="handleAreaCreated"
            @cancel="showAreaFormForLocation = null"
          />

          <div
            v-if="getAreasForLocation(location.id).length > 0"
            class="flex flex-col gap-3"
          >
            <div
              v-for="area in getAreasForLocation(location.id)"
              :id="`area-${area.id}`"
              :key="area.id"
              role="button"
              tabindex="0"
              :class="[
                'area-card group flex cursor-pointer items-center justify-between gap-3 rounded-md border border-border bg-card p-4 shadow-sm motion-safe:transition-shadow hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                areaToFocus === area.id && 'ring-2 ring-primary motion-safe:animate-pulse',
              ]"
              @click="viewArea(area.id)"
              @keydown.enter.prevent="viewArea(area.id)"
              @keydown.space.prevent="viewArea(area.id)"
            >
              <div class="min-w-0 flex-1">
                <h5 class="truncate text-sm font-semibold text-foreground">
                  {{ area.attributes.name }}
                </h5>
                <p class="mt-1 text-sm font-medium text-primary">
                  <span class="font-normal text-muted-foreground">Total value:</span>
                  {{ valuesLoading ? 'Loading…' : getAreaValue(area.id) }}
                </p>
              </div>
              <div class="area-actions flex shrink-0 items-center gap-1" @click.stop>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  title="Edit"
                  aria-label="Edit area"
                  @click="editArea(area.id)"
                >
                  <Pencil class="size-4" aria-hidden="true" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  title="Delete"
                  aria-label="Delete area"
                  class="text-destructive hover:bg-destructive/10 hover:text-destructive"
                  @click="confirmDeleteArea(area.id)"
                >
                  <Trash2 class="size-4" aria-hidden="true" />
                </Button>
              </div>
            </div>
          </div>
          <div
            v-else
            class="rounded-md bg-card p-4 text-center text-sm text-muted-foreground"
          >
            No areas found for this location. Add your first area using the
            button above.
          </div>
        </div>
      </div>
    </div>

    <PaginationControls
      v-if="!loading"
      :current-page="currentPage"
      :total-pages="totalPages"
      :page-size="pageSize"
      :total-items="totalLocations"
      item-label="locations"
    />

    <Confirmation
      v-model:visible="showDeleteLocationDialog"
      title="Confirm Delete"
      message="Are you sure you want to delete this location?"
      confirm-label="Delete"
      cancel-label="Cancel"
      confirm-button-class="danger"
      confirmation-icon="exclamation-triangle"
      @confirm="onConfirmDeleteLocation"
      @cancel="onCancelDeleteLocation"
    />

    <Confirmation
      v-model:visible="showDeleteAreaDialog"
      title="Confirm Delete"
      message="Are you sure you want to delete this area?"
      confirm-label="Delete"
      cancel-label="Cancel"
      confirm-button-class="danger"
      confirmation-icon="exclamation-triangle"
      @confirm="onConfirmDeleteArea"
      @cancel="onCancelDeleteArea"
    />
  </PageContainer>
</template>
