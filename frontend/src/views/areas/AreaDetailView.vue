<template>
  <div class="area-detail">
    <!-- Error Notification Stack -->
    <ErrorNotificationStack
      :errors="errors"
      @dismiss="removeError"
    />

    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="!area" class="not-found">Area not found</div>
    <div v-else>
      <div class="breadcrumb-nav">
        <a href="#" class="breadcrumb-link" @click.prevent="navigateToLocations">
          <font-awesome-icon icon="arrow-left" /> Back to Locations
        </a>
      </div>
      <div class="header">
        <div class="title-section">
          <h1>
            {{ area.attributes.name }}
          </h1>
          <p class="location-info">{{ locationName || 'No location' }}{{ locationAddress ? ` - ${locationAddress}` : '' }}</p>
          <div v-if="areaTotalValue > 0" class="total-value">
            Total Value: <span class="value-amount">{{ formatPrice(areaTotalValue, getMainCurrency()) }}</span>
          </div>
        </div>
        <div class="actions">
          <button class="btn btn-danger" title="Delete" @click="confirmDelete"><font-awesome-icon icon="trash" /></button>
        </div>
      </div>

      <div v-if="commodities.length > 0" class="commodities-section">
        <div class="section-header">
          <div class="section-title">
            <h2>Commodities</h2>
            <div class="filter-toggle">
              <ToggleSwitch v-model="showInactiveItems" />
              <label class="toggle-label">Show drafts & inactive items</label>
            </div>
          </div>
          <router-link :to="`/commodities/new?area=${area.id}`" class="btn btn-primary btn-sm"><font-awesome-icon icon="plus" /> New</router-link>
        </div>
        <div class="commodities-grid">
          <CommodityListItem
            v-for="commodity in filteredCommodities"
            :key="commodity.id"
            :commodity="commodity"
            :highlight-commodity-id="highlightCommodityId"
            :show-location="false"
            @view-commodity="viewCommodity"
            @edit-commodity="editCommodity"
            @confirm-delete-commodity="confirmDeleteCommodity"
          />
        </div>
      </div>
      <div v-else class="no-commodities">
        <p>No commodities found in this area.</p>
        <router-link :to="`/commodities/new?area=${area.id}`" class="btn btn-primary">Add Commodity</router-link>
      </div>
    </div>

    <!-- Area Delete Confirmation Dialog -->
    <Confirmation
      v-model:visible="showDeleteDialog"
      title="Confirm Delete"
      message="Are you sure you want to delete this area?"
      confirm-label="Delete"
      cancel-label="Cancel"
      confirm-button-class="danger"
      confirmationIcon="exclamation-triangle"
      @confirm="onConfirmDelete"
      @cancel="onCancelDelete"
    />

    <!-- Commodity Delete Confirmation Dialog -->
    <Confirmation
      v-model:visible="showDeleteCommodityDialog"
      title="Confirm Delete"
      message="Are you sure you want to delete this commodity?"
      confirm-label="Delete"
      cancel-label="Cancel"
      confirm-button-class="danger"
      confirmationIcon="exclamation-triangle"
      @confirm="onConfirmDeleteCommodity"
      @cancel="onCancelDeleteCommodity"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, nextTick, onBeforeUnmount } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import areaService from '@/services/areaService'
import locationService from '@/services/locationService'
import commodityService from '@/services/commodityService'
import valueService from '@/services/valueService'
import { COMMODITY_STATUS_IN_USE } from '@/constants/commodityStatuses'
import { formatPrice, getDisplayPrice, getMainCurrency } from '@/services/currencyService'
import Confirmation from "@/components/Confirmation.vue"
import CommodityListItem from "@/components/CommodityListItem.vue"
import ErrorNotificationStack from '@/components/ErrorNotificationStack.vue'
import { useErrorState } from '@/utils/errorUtils'

const router = useRouter()
const route = useRoute()
const area = ref<any>(null)
const locations = ref<any[]>([])
const commodities = ref<any[]>([])
const loading = ref<boolean>(true)
const locationName = ref<string | null>(null)
const locationAddress = ref<string | null>(null)

// Error state management
const { errors, handleError, removeError, cleanup } = useErrorState()


// Area total value
const areaTotalValue = ref<number>(0)

// Highlight commodity if specified in the URL
const highlightCommodityId = ref(route.query.highlightCommodityId as string || '')
let highlightTimeout: number | null = null

// Filter toggle state
const showInactiveItems = ref(false)

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

onMounted(async () => {
  const id = route.params.id as string

  try {
    // Main currency is now handled by the currency service

    // Load area, locations, commodities, and values in parallel
    const [areaResponse, locationsResponse, commoditiesResponse, valuesResponse] = await Promise.all([
      areaService.getArea(id),
      locationService.getLocations(),
      commodityService.getCommodities(),
      valueService.getValues()
    ])

    area.value = areaResponse.data.data

    // Get the location ID from the area
    const locationId = area.value.attributes.location_id

    if (locationId) {
      // Find the location in the locations list
      const location = locationsResponse.data.data.find(
        (loc: any) => loc.id === locationId
      )

      if (location) {
        locationName.value = location.attributes.name
        locationAddress.value = location.attributes.address
      }
    }

    // Filter locations that belong to this area
    locations.value = locationsResponse.data.data.filter(
      (location: any) =>
        location.relationships &&
        location.relationships.area &&
        location.relationships.area.data.id === id
    )

    // Filter commodities that belong to this area
    commodities.value = commoditiesResponse.data.data.filter(
      (commodity: any) => commodity.attributes.area_id === id
    )

    // Get the area total value from the values response
    try {
      // Ensure we have a valid data structure
      const valueAttributes = valuesResponse?.data?.data?.attributes || {}
      const areaTotals = valueAttributes.area_totals || []

      // Handle both array and object formats for area_totals
      let areaValue = null
      if (Array.isArray(areaTotals)) {
        // If it's an array, use find
        areaValue = areaTotals.find((areaValue: any) => areaValue.id === id)
      } else if (areaTotals && typeof areaTotals === 'object') {
        // If it's an object with key-value pairs, check if our ID exists as a key
        if (areaTotals[id]) {
          areaValue = {
            id: id,
            value: areaTotals[id]
          }
        }
      }

      if (areaValue) {
        areaTotalValue.value = parseFloat(areaValue.value)
      } else {
        // If no value found in the API response, calculate it from the commodities
        areaTotalValue.value = commodities.value.reduce((total: number, commodity: any) => {
          // Only include commodities that are in use and not drafts
          if (commodity.attributes.status === 'in_use' && !commodity.attributes.draft) {
            const price = getDisplayPrice(commodity)
            if (!isNaN(price)) {
              return total + price
            }
          }
          return total
        }, 0)
      }
    } catch (err) {
      console.error('Error processing area values:', err)
      // Fallback to calculating from commodities
      areaTotalValue.value = commodities.value.reduce((total: number, commodity: any) => {
        // Only include commodities that are in use and not drafts
        if (commodity.attributes.status === 'in_use' && !commodity.attributes.draft) {
          const price = getDisplayPrice(commodity)
          if (!isNaN(price)) {
            return total + price
          }
        }
        return total
      }, 0)
    }

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
    handleError(err, 'area', 'Failed to load area')
    loading.value = false
  }
})

// Clean up timeout when component is unmounted
onBeforeUnmount(() => {
  if (highlightTimeout !== null) {
    window.clearTimeout(highlightTimeout)
    highlightTimeout = null
  }
  cleanup()
})

// These functions are now handled by the CommodityListItem component

// Price utility functions are now imported from @/utils/priceUtils

// Note: We're using the imported calculatePricePerUnit function instead

const showDeleteDialog = ref(false)

const confirmDelete = () => {
  showDeleteDialog.value = true
}

const onConfirmDelete = () => {
  deleteArea()
  showDeleteDialog.value = false
}

const onCancelDelete = () => {
  showDeleteDialog.value = false
}

const deleteArea = async () => {
  try {
    await areaService.deleteArea(area.value.id)
    router.push('/locations')
  } catch (err: any) {
    handleError(err, 'area', 'Failed to delete area')
  }
}

// Navigation to location is handled by navigateToLocations function

const viewCommodity = (id: string) => {
  router.push({
    path: `/commodities/${id}`,
    query: {
      source: 'area',
      areaId: area.value.id
    }
  })
}

const editCommodity = (id: string) => {
  router.push({
    path: `/commodities/${id}/edit`,
    query: {
      source: 'area',
      areaId: area.value.id,
      directEdit: 'true'
    }
  })
}

const commodityToDelete = ref<string | null>(null)
const showDeleteCommodityDialog = ref(false)

const confirmDeleteCommodity = (id: string) => {
  commodityToDelete.value = id
  showDeleteCommodityDialog.value = true
}

const onConfirmDeleteCommodity = () => {
  if (commodityToDelete.value) {
    deleteCommodity(commodityToDelete.value)
    showDeleteCommodityDialog.value = false
    commodityToDelete.value = null
  }
}

const onCancelDeleteCommodity = () => {
  showDeleteCommodityDialog.value = false
  commodityToDelete.value = null
}

const navigateToLocations = () => {
  // Navigate to locations list with area and location context
  router.push({
    path: '/locations',
    query: {
      areaId: area.value.id,
      locationId: area.value.attributes.location_id
    }
  })
}

const deleteCommodity = async (id: string) => {
  try {
    await commodityService.deleteCommodity(id)
    // Remove the deleted commodity from the list
    commodities.value = commodities.value.filter(commodity => commodity.id !== id)
  } catch (err: any) {
    handleError(err, 'commodity', 'Failed to delete commodity')
  }
}
</script>

<style lang="scss" scoped>
@use 'sass:color';
@use '@/assets/main.scss' as *;

.area-detail {
  max-width: $container-max-width;
  margin: 0 auto;
  padding: 20px;
}

.breadcrumb-nav {
  margin-bottom: 1rem;
}

.breadcrumb-link {
  color: $secondary-color;
  font-size: 0.9rem;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  transition: color 0.2s;

  &:hover {
    color: $primary-color;
    text-decoration: none;
  }
}

// Header styles are now in shared _header.scss

.title-section {
  display: flex;
  flex-direction: column;

  h1 {
    margin-bottom: 0.5rem;
  }
}

.location-info {
  color: $text-color;
  font-style: italic;
  margin-top: 0;
  margin-bottom: 0.5rem;
}

.actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.6rem;
}

.loading, .error, .not-found, .no-commodities {
  text-align: center;
  padding: 2rem;
  background: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
  margin-bottom: 2rem;
}

.error {
  color: $danger-color;
}

.commodities-section {
  margin-bottom: 2rem;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid $border-color;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 1rem;
}

// Filter toggle styles are now in shared _filter-toggle.scss

.commodities-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 1.5rem;
}

.btn-primary {
  background-color: $primary-color;
  color: white;
  text-decoration: none;
  padding: 0.5rem 1rem;
  border-radius: $default-radius;
  display: inline-block;
  margin-top: 1rem;
}

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
  margin-top: 0;
  border-radius: $default-radius;
}

pre {
  white-space: pre-wrap;
  word-wrap: break-word;
  overflow-x: auto;
  background: $light-bg-color;
  padding: 0.5rem;
  border-radius: $default-radius;
}


</style>
