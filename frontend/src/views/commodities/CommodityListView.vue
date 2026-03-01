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
        <div class="filter-toggle">
          <ToggleSwitch v-model="showInactiveItems" />
          <label class="toggle-label">Show drafts & inactive items</label>
        </div>
        <router-link v-if="loading" to="#" class="btn btn-primary"><font-awesome-icon icon="plus" /> Loading...</router-link>
        <router-link v-else-if="hasLocationsAndAreas" to="/commodities/new" class="btn btn-primary new-commodity-button"><font-awesome-icon icon="plus" /> New</router-link>
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

    <div v-else class="commodities-grid-container">
      <div class="commodities-grid">
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

      <!-- Pagination -->
      <div v-if="totalPages > 1" class="pagination-card">
        <div class="pagination-info">
          Showing {{ (currentPage - 1) * pageSize + 1 }} to {{ Math.min(currentPage * pageSize, totalCommodities) }} of {{ totalCommodities }} commodities
        </div>
        <div class="pagination-controls">
          <router-link
            v-if="currentPage > 1"
            :to="getPaginationUrl(currentPage - 1)"
            class="btn btn-secondary pagination-link"
          >
            <font-awesome-icon icon="chevron-left" />
            Previous
          </router-link>
          <span v-else class="btn btn-secondary pagination-link disabled">
            <font-awesome-icon icon="chevron-left" />
            Previous
          </span>

          <div class="page-numbers">
            <router-link
              v-for="page in visiblePages"
              :key="page"
              :to="getPaginationUrl(page)"
              class="btn pagination-link"
              :class="{ 'btn-primary': page === currentPage, 'btn-secondary': page !== currentPage }"
            >
              {{ page }}
            </router-link>
          </div>

          <router-link
            v-if="currentPage < totalPages"
            :to="getPaginationUrl(currentPage + 1)"
            class="btn btn-secondary pagination-link"
          >
            Next
            <font-awesome-icon icon="chevron-right" />
          </router-link>
          <span v-else class="btn btn-secondary pagination-link disabled">
            Next
            <font-awesome-icon icon="chevron-right" />
          </span>
        </div>
      </div>
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
import { ref, onMounted, computed, nextTick, onBeforeUnmount, watch } from 'vue'
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

const router = useRouter()
const route = useRoute()
const settingsStore = useSettingsStore()
const commodities = ref<any[]>([])
const areas = ref<any[]>([])
const locations = ref<any[]>([])
const loading = ref<boolean>(true)
const error = ref<string | null>(null)

// Pagination state
const currentPage = ref(1)
const pageSize = ref(50)
const totalCommodities = ref(0)
const totalPages = computed(() => Math.ceil(totalCommodities.value / pageSize.value))
const visiblePages = computed(() => {
  const pages: number[] = []
  const start = Math.max(1, currentPage.value - 2)
  const end = Math.min(totalPages.value, currentPage.value + 2)
  for (let i = start; i <= end; i++) pages.push(i)
  return pages
})

const getPaginationUrl = (page: number) => {
  const query = { ...route.query }
  if (page > 1) {
    query.page = page.toString()
  } else {
    delete query.page
  }
  return { path: route.path, query }
}

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

// Computed property to check if there are locations and areas
const hasLocationsAndAreas = computed(() => {
  return locations.value.length > 0 && areas.value.length > 0
})

// Filtered commodities based on toggle state
const filteredCommodities = computed(() => {
  if (showInactiveItems.value) {
    return commodities.value
  }

  return commodities.value.filter(commodity => {
    // Show only non-draft items with status 'in_use'
    return !commodity.attributes.draft && commodity.attributes.status === COMMODITY_STATUS_IN_USE
  })
})

// Map to store area and location information
const areaMap = ref<Record<string, any>>({})
const locationMap = ref<Record<string, any>>({})

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

const loadCommodities = async () => {
  loading.value = true
  error.value = null
  try {
    // Load commodities with pagination; load areas/locations fully for lookup maps
    const [commoditiesResponse, areasResponse, locationsResponse] = await Promise.all([
      commodityService.getCommodities({ page: currentPage.value, per_page: pageSize.value }),
      areaService.getAreas({ per_page: 1000 }),
      locationService.getLocations({ per_page: 1000 }),
    ])

    commodities.value = commoditiesResponse.data.data
    totalCommodities.value = commoditiesResponse.data.meta.commodities
    areas.value = areasResponse.data.data
    locations.value = locationsResponse.data.data

    // Create maps for quick lookups
    areaMap.value = {}
    areas.value.forEach(area => {
      areaMap.value[area.id] = {
        name: area.attributes.name,
        locationId: area.attributes.location_id
      }
    })

    locationMap.value = {}
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
}

onMounted(async () => {
  await settingsStore.fetchMainCurrency()
  currentPage.value = Number(route.query.page) || 1
  await Promise.all([loadCommodities(), loadValues()])
})

watch(() => route.query.page, (newPage) => {
  currentPage.value = Number(newPage) || 1
  loadCommodities()
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
    // Reload the current page to reflect deletion with accurate pagination
    await loadCommodities()
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

.commodities-grid-container {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.commodities-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 1.5rem;
}

.pagination-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1rem;
  padding: 1rem;
  background: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
}

.pagination-info {
  font-size: 0.9rem;
  color: $text-color;
}

.pagination-controls {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
  justify-content: center;
}

.page-numbers {
  display: flex;
  gap: 0.25rem;
}

.pagination-link {
  min-width: 2.5rem;
  text-align: center;

  &.disabled {
    opacity: 0.5;
    pointer-events: none;
  }
}
</style>
