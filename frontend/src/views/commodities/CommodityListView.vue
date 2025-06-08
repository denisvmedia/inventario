<template>
  <div class="commodity-list">
    <div class="header">
      <div class="header-title">
        <h1>Commodities</h1>
        <div v-if="!valuesLoading && globalTotal > 0" class="total-value">
          Total Value: <span class="value-amount">{{ formatPrice(globalTotal, mainCurrency) }}</span>
        </div>
      </div>
      <div class="header-actions">
        <div class="sort-controls">
          <label for="sortBy" class="sort-label">Sort by:</label>
          <Select
            id="sortBy"
            v-model="sortBy"
            :options="sortOptions"
            option-label="label"
            option-value="value"
            placeholder="Select sorting"
            class="sort-select"
          />
          <Button
            :title="sortOrder === 'asc' ? 'Sort Ascending' : 'Sort Descending'"
            :icon="sortOrder === 'asc' ? 'pi pi-sort-up' : 'pi pi-sort-down'"
            @click="toggleSortOrder"
            class="sort-order-btn"
            severity="secondary"
            text
          />
        </div>
        <div class="filter-toggle">
          <InputSwitch v-model="showInactiveItems" />
          <label class="toggle-label">Show drafts & inactive items</label>
        </div>
        <router-link v-if="hasLocationsAndAreas" to="/commodities/new" class="btn btn-primary"><font-awesome-icon icon="plus" /> New</router-link>
        <router-link v-else-if="locations.length === 0" to="/locations" class="btn btn-primary"><font-awesome-icon icon="plus" /> Create Location First</router-link>
        <router-link v-else-if="areas.length === 0" to="/locations" class="btn btn-primary"><font-awesome-icon icon="plus" /> Create Area First</router-link>
      </div>
    </div>

    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="commodities.length === 0" class="empty">
      <div v-if="locations.length === 0" class="empty-message">
        <p>No locations found. You need to create a location first before you can create commodities.</p>
        <div class="action-button">
          <router-link to="/locations" class="btn btn-primary">Create Location</router-link>
        </div>
      </div>
      <div v-else-if="areas.length === 0" class="empty-message">
        <p>No areas found. You need to create an area in a location before you can create commodities.</p>
        <div class="action-button">
          <router-link to="/locations" class="btn btn-primary">Create Area</router-link>
        </div>
      </div>
      <div v-else class="empty-message">
        <p>No commodities found. Create your first commodity!</p>
        <div class="action-button">
          <router-link to="/commodities/new" class="btn btn-primary">Create Commodity</router-link>
        </div>
      </div>
    </div>

    <div v-else class="commodities-grid">
      <CommodityListItem
        v-for="commodity in filteredCommodities"
        :key="commodity.id"
        :commodity="commodity"
        :highlight-commodity-id="highlightCommodityId"
        :show-location="true"
        :area-map="areaMap"
        :location-map="locationMap"
        @view-commodity="viewCommodity"
        @edit-commodity="editCommodity"
        @confirm-delete-commodity="confirmDelete"
      />
    </div>

    <!-- Commodity Delete Confirmation Dialog -->
    <Confirmation
      v-model:visible="showDeleteDialog"
      title="Confirm Delete"
      message="Are you sure you want to delete this commodity?"
      confirm-label="Delete"
      cancel-label="Cancel"
      confirm-button-class="danger"
      confirmationIcon="exclamation-triangle"
      @confirm="onConfirmDelete"
      @cancel="onCancelDelete"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, nextTick, onBeforeUnmount } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import commodityService from '@/services/commodityService'
import areaService from '@/services/areaService'
import locationService from '@/services/locationService'
import valueService from '@/services/valueService'
import { useSettingsStore } from '@/stores/settingsStore'
import { COMMODITY_STATUS_IN_USE } from '@/constants/commodityStatuses'
import { formatPrice } from '@/services/currencyService'
import Confirmation from "@/components/Confirmation.vue"
import CommodityListItem from "@/components/CommodityListItem.vue"
import Select from 'primevue/select'
import Button from 'primevue/button'
import InputSwitch from 'primevue/inputswitch'

const router = useRouter()
const route = useRoute()
const settingsStore = useSettingsStore()
const commodities = ref<any[]>([])
const areas = ref<any[]>([])
const locations = ref<any[]>([])
const loading = ref<boolean>(true)
const error = ref<string | null>(null)

// Values data
const globalTotal = ref<number>(0)
const valuesLoading = ref<boolean>(true)
const valuesError = ref<string | null>(null)

// Main currency from settings store
const mainCurrency = computed(() => settingsStore.mainCurrency)

// Highlight commodity if specified in the URL
const highlightCommodityId = ref(route.query.highlightCommodityId as string || '')
let highlightTimeout: number | null = null

// Filter toggle state
const showInactiveItems = ref(false)

// Sorting state
const sortBy = ref('name')
const sortOrder = ref<'asc' | 'desc'>('asc')

// Sort options
const sortOptions = ref([
  { label: 'Name', value: 'name' },
  { label: 'Purchase Date', value: 'purchaseDate' },
  { label: 'Location', value: 'location' },
  { label: 'Area', value: 'area' },
  { label: 'Type', value: 'type' },
  { label: 'Status', value: 'status' }
])

// Computed property to check if there are locations and areas
const hasLocationsAndAreas = computed(() => {
  return locations.value.length > 0 && areas.value.length > 0
})

// Filtered and sorted commodities
const filteredCommodities = computed(() => {
  let filtered = commodities.value
  
  if (!showInactiveItems.value) {
    filtered = filtered.filter(commodity => {
      // Show only non-draft items with status 'in_use'
      return !commodity.attributes.draft && commodity.attributes.status === COMMODITY_STATUS_IN_USE
    })
  }
  
  // Sort the filtered commodities
  const sorted = [...filtered].sort((a, b) => {
    let aValue: any, bValue: any
    
    switch (sortBy.value) {
      case 'name':
        aValue = a.attributes.name.toLowerCase()
        bValue = b.attributes.name.toLowerCase()
        break
      case 'purchaseDate':
        aValue = new Date(a.attributes.purchase_date || '1900-01-01')
        bValue = new Date(b.attributes.purchase_date || '1900-01-01')
        break
      case 'location':
        aValue = getLocationName(a.attributes.area_id).toLowerCase()
        bValue = getLocationName(b.attributes.area_id).toLowerCase()
        break
      case 'area':
        aValue = getAreaName(a.attributes.area_id).toLowerCase()
        bValue = getAreaName(b.attributes.area_id).toLowerCase()
        break
      case 'type':
        aValue = a.attributes.type.toLowerCase()
        bValue = b.attributes.type.toLowerCase()
        break
      case 'status':
        aValue = a.attributes.status.toLowerCase()
        bValue = b.attributes.status.toLowerCase()
        break
      default:
        aValue = a.attributes.name.toLowerCase()
        bValue = b.attributes.name.toLowerCase()
    }
    
    if (aValue < bValue) {
      return sortOrder.value === 'asc' ? -1 : 1
    }
    if (aValue > bValue) {
      return sortOrder.value === 'asc' ? 1 : -1
    }
    return 0
  })
  
  return sorted
})

// Map to store area and location information
const areaMap = ref<Record<string, any>>({})
const locationMap = ref<Record<string, any>>({})

// Helper functions for sorting
const getAreaName = (areaId: string) => {
  return areaMap.value[areaId]?.name || 'Unknown Area'
}

const getLocationName = (areaId: string) => {
  const locationId = areaMap.value[areaId]?.locationId
  return locationMap.value[locationId]?.name || 'Unknown Location'
}

const toggleSortOrder = () => {
  sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
}

// These functions are now handled by the CommodityListItem component

// Function to load total values
async function loadValues() {
  valuesLoading.value = true
  valuesError.value = null

  try {
    const response = await valueService.getValues()
    const data = response.data.data.attributes

    // Parse the decimal string to a number
    globalTotal.value = parseFloat(data.global_total)
  } catch (error) {
    console.error('Error loading values:', error)
    valuesError.value = 'Failed to load inventory values'
  } finally {
    valuesLoading.value = false
  }
}

onMounted(async () => {
  try {
    // Fetch main currency from the store
    await settingsStore.fetchMainCurrency()

    // Load commodities, areas, locations, and values in parallel
    const [commoditiesResponse, areasResponse, locationsResponse] = await Promise.all([
      commodityService.getCommodities(),
      areaService.getAreas(),
      locationService.getLocations(),
      loadValues() // Load values in parallel
    ])

    commodities.value = commoditiesResponse.data.data
    areas.value = areasResponse.data.data
    locations.value = locationsResponse.data.data

    // Create maps for quick lookups
    areas.value.forEach(area => {
      areaMap.value[area.id] = {
        name: area.attributes.name,
        locationId: area.attributes.location_id
      }
    })

    locations.value.forEach(location => {
      locationMap.value[location.id] = {
        name: location.attributes.name,
        address: location.attributes.address
      }
    })

    loading.value = false

    // Scroll to highlighted commodity if specified
    if (highlightCommodityId.value) {
      nextTick(() => {
        const highlightedElement = document.querySelector(`.commodity-card.highlighted`)
        if (highlightedElement) {
          highlightedElement.scrollIntoView({ behavior: 'smooth', block: 'nearest' })

          // Clear the highlight after 3 seconds
          highlightTimeout = window.setTimeout(() => {
            highlightCommodityId.value = ''
          }, 3000)
        }
      })
    }
  } catch (err: any) {
    error.value = 'Failed to load data: ' + (err.message || 'Unknown error')
    loading.value = false
  }
})

// Clean up timeout when component is unmounted
onBeforeUnmount(() => {
  if (highlightTimeout !== null) {
    window.clearTimeout(highlightTimeout)
    highlightTimeout = null
  }
})

const viewCommodity = (id: string) => {
  router.push({
    path: `/commodities/${id}`,
    query: {
      source: 'commodities'
    }
  })
}

const editCommodity = (id: string) => {
  router.push({
    path: `/commodities/${id}/edit`,
    query: {
      source: 'commodities',
      directEdit: 'true'
    }
  })
}

const commodityToDelete = ref<string | null>(null)
const showDeleteDialog = ref(false)

const confirmDelete = (id: string) => {
  commodityToDelete.value = id
  showDeleteDialog.value = true
}

const onConfirmDelete = () => {
  if (commodityToDelete.value) {
    deleteCommodity(commodityToDelete.value)
    showDeleteDialog.value = false
    commodityToDelete.value = null
  }
}

const onCancelDelete = () => {
  showDeleteDialog.value = false
  commodityToDelete.value = null
}

const deleteCommodity = async (id: string) => {
  try {
    await commodityService.deleteCommodity(id)
    // Remove the deleted commodity from the list
    commodities.value = commodities.value.filter(commodity => commodity.id !== id)
  } catch (err: any) {
    error.value = 'Failed to delete commodity: ' + (err.message || 'Unknown error')
  }
}
</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;

.commodity-list {
  max-width: $container-max-width;
  margin: 0 auto;
  padding: 20px;
}

// Header styles are now in shared _header.scss

// Sort controls specific to this component
.sort-controls {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  background-color: #f8f9fa;
  padding: 0.5rem 0.75rem;
  border-radius: $default-radius;
  border: 1px solid #e9ecef;
}

.sort-label {
  font-size: 0.9rem;
  margin: 0;
  white-space: nowrap;
  color: $text-color;
}

.sort-select {
  min-width: 140px;
}

.sort-order-btn {
  padding: 0.25rem;
  height: 32px;
  width: 32px;
}

.loading, .error, .empty {
  text-align: center;
  padding: 2rem;
  background: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
}

.error {
  color: $danger-color;
}

.empty-message {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1.5rem;
}

.empty-message p {
  margin-bottom: 0;
  font-size: 1.1rem;
}

.action-button {
  margin-top: 0.5rem;
}

.commodities-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 1.5rem;
}
</style>
