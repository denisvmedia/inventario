<template>
  <div class="area-detail">
    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="!area" class="not-found">Area not found</div>
    <div v-else>
      <div class="breadcrumb-nav">
        <a href="#" @click.prevent="navigateToLocations" class="breadcrumb-link">
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
          <button class="btn btn-danger" @click="confirmDelete" title="Delete"><font-awesome-icon icon="trash" /></button>
        </div>
      </div>

      <div class="commodities-section" v-if="commodities.length > 0">
        <div class="section-header">
          <h2>Commodities</h2>
          <router-link :to="`/commodities/new?area=${area.id}`" class="btn btn-primary btn-sm"><font-awesome-icon icon="plus" /> New</router-link>
        </div>
        <div class="commodities-grid">
          <div v-for="commodity in commodities" :key="commodity.id" class="commodity-card" :class="{
            'highlighted': commodity.id === highlightCommodityId,
            'draft': commodity.attributes.draft,
            'sold': !commodity.attributes.draft && commodity.attributes.status === 'sold',
            'lost': !commodity.attributes.draft && commodity.attributes.status === 'lost',
            'disposed': !commodity.attributes.draft && commodity.attributes.status === 'disposed',
            'written-off': !commodity.attributes.draft && commodity.attributes.status === 'written_off'
          }">
            <div class="commodity-content" @click="viewCommodity(commodity.id)">
              <h3>{{ commodity.attributes.name }}</h3>
              <div class="commodity-meta">
                <span class="type">
                  <font-awesome-icon :icon="getTypeIcon(commodity.attributes.type)" />
                  {{ getTypeName(commodity.attributes.type) }}
                </span>
                <span class="count" v-if="(commodity.attributes.count || 1) > 1">×{{ commodity.attributes.count }}</span>
              </div>
              <div class="commodity-price">
                <span class="price">{{ formatPrice(getDisplayPrice(commodity)) }}</span>
                <span class="price-per-unit" v-if="(commodity.attributes.count || 1) > 1">
                  {{ formatPrice(calculatePricePerUnit(commodity)) }} per unit
                </span>
              </div>
              <div class="commodity-status" v-if="commodity.attributes.status" :class="{ 'with-draft': commodity.attributes.draft }">
                <span class="status" :class="commodity.attributes.status">{{ getStatusName(commodity.attributes.status) }}</span>
              </div>
            </div>
            <div class="commodity-actions">
              <button class="btn btn-secondary btn-sm" @click.stop="editCommodity(commodity.id)" title="Edit">
                <font-awesome-icon icon="edit" />
              </button>
              <button class="btn btn-danger btn-sm" @click.stop="confirmDeleteCommodity(commodity.id)" title="Delete">
                <font-awesome-icon icon="trash" />
              </button>
            </div>
          </div>
        </div>
      </div>
      <div v-else class="no-commodities">
        <p>No commodities found in this area.</p>
        <router-link :to="`/commodities/new?area=${area.id}`" class="btn btn-primary">Add Commodity</router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, nextTick, onBeforeUnmount } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import areaService from '@/services/areaService'
import locationService from '@/services/locationService'
import commodityService from '@/services/commodityService'
import valueService from '@/services/valueService'
import settingsService from '@/services/settingsService'
import { COMMODITY_TYPES } from '@/constants/commodityTypes'
import { COMMODITY_STATUSES } from '@/constants/commodityStatuses'
import { formatPrice, getDisplayPrice, calculatePricePerUnit, getMainCurrency } from '@/services/currencyService'

const router = useRouter()
const route = useRoute()
const area = ref<any>(null)
const locations = ref<any[]>([])
const commodities = ref<any[]>([])
const loading = ref<boolean>(true)
const error = ref<string | null>(null)
const locationName = ref<string | null>(null)
const locationAddress = ref<string | null>(null)


// Area total value
const areaTotalValue = ref<number>(0)
const valuesLoading = ref<boolean>(true)

// Highlight commodity if specified in the URL
const highlightCommodityId = ref(route.query.highlightCommodityId as string || '')
let highlightTimeout: number | null = null

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
    error.value = 'Failed to load area: ' + (err.message || 'Unknown error')
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

const getTypeIcon = (typeId: string) => {
  switch(typeId) {
    case 'white_goods':
      return 'blender'
    case 'electronics':
      return 'laptop'
    case 'equipment':
      return 'tools'
    case 'furniture':
      return 'couch'
    case 'clothes':
      return 'tshirt'
    case 'other':
      return 'box'
    default:
      return 'box'
  }
}

const getTypeName = (typeId: string) => {
  const type = COMMODITY_TYPES.find(t => t.id === typeId)
  return type ? type.name : typeId
}

const getStatusName = (statusId: string) => {
  const status = COMMODITY_STATUSES.find(s => s.id === statusId)
  return status ? status.name : statusId
}

// Price utility functions are now imported from @/utils/priceUtils

// Calculate original price per unit
const calculateOriginalPricePerUnit = (commodity: any) => {
  const price = parseFloat(commodity.attributes.original_price) || 0
  const count = commodity.attributes.count || 1
  if (count <= 1) return price

  // Calculate price per unit and round to 2 decimal places
  const pricePerUnit = price / count
  return pricePerUnit.toFixed(2)
}

const updateAreaName = async (newName: string) => {
  try {
    const payload = {
      data: {
        id: area.value.id,
        type: 'areas',
        attributes: {
          name: newName,
          location_id: area.value.attributes.location_id
        }
      }
    }

    await areaService.updateArea(area.value.id, payload)
    // Update was successful, the model is already updated via v-model
  } catch (err: any) {
    error.value = 'Failed to update area name: ' + (err.message || 'Unknown error')
    // Revert the change in the UI
    area.value.attributes.name = area.value.attributes.name
  }
}

const confirmDelete = () => {
  if (confirm('Are you sure you want to delete this area?')) {
    deleteArea()
  }
}

const deleteArea = async () => {
  try {
    await areaService.deleteArea(area.value.id)
    router.push('/locations')
  } catch (err: any) {
    error.value = 'Failed to delete area: ' + (err.message || 'Unknown error')
  }
}

const viewLocation = (id: string) => {
  router.push(`/locations/${id}`)
}

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

const confirmDeleteCommodity = (id: string) => {
  if (confirm('Are you sure you want to delete this commodity?')) {
    deleteCommodity(id)
  }
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
    error.value = 'Failed to delete commodity: ' + (err.message || 'Unknown error')
  }
}
</script>

<style lang="scss" scoped>
@import '../../assets/main.scss';

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

.header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 2rem;
}

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

.total-value {
  font-size: 1rem;
  color: $text-color;
  margin-top: 0.25rem;

  .value-amount {
    font-weight: bold;
    color: $primary-color;
    font-size: 1.1rem;
  }
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

.commodities-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 1.5rem;
}

.commodity-card {
  background: white;
  border-radius: $default-radius;
  padding: 1.5rem;
  box-shadow: $box-shadow;
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;

  &:hover {
    transform: translateY(-5px);
    box-shadow: 0 5px 15px rgba(0, 0, 0, 0.1);
  }

  &.highlighted {
    border-left: 4px solid $primary-color;
    box-shadow: 0 2px 10px rgba($primary-color, 0.3);
    background-color: lighten($primary-color, 45%);
  }

  &.draft {
    background: repeating-linear-gradient(45deg, #ffffff, #ffffff 5px, #eeeeee4d 5px, #eeeeee4d 7px);
    position: relative;

    h3, .commodity-meta, .commodity-price, .price-per-unit {
      color: $text-secondary-color;
    }

    .status {
      background-color: #e2e3e5 !important;
      color: #383d41 !important;
    }
  }

  &.sold {
    position: relative;
    filter: grayscale(0.8);

    &::before {
      content: 'SOLD';
      position: absolute;
      top: 50%;
      left: 50%;
      transform: translate(-50%, -50%) rotate(-45deg);
      font-size: 2.5rem;
      font-weight: bold;
      color: rgba(204, 229, 255, 0.8);
      border: 3px solid rgba(0, 64, 133, 0.5);
      padding: 0.5rem 1rem;
      border-radius: $default-radius;
      z-index: 1;
      pointer-events: none;
    }
  }

  &.lost {
    position: relative;
    filter: saturate(0.7);

    &::before {
      content: '';
      position: absolute;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      background-color: rgba(255, 243, 205, 0.3);
      z-index: 1;
      pointer-events: none;
    }

    &::after {
      content: '⚠️';
      position: absolute;
      bottom: 1rem;
      right: 1rem;
      font-size: 1.5rem;
      z-index: 2;
      pointer-events: none;
    }
  }

  &.disposed {
    position: relative;

    &::before {
      content: '';
      position: absolute;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      background-color: rgba(248, 215, 218, 0.3);
      background-image: linear-gradient(45deg, transparent, transparent 48%, rgba(114, 28, 36, 0.2) 49%, rgba(114, 28, 36, 0.2) 51%, transparent 52%, transparent);
      background-size: 20px 20px;
      z-index: 1;
      pointer-events: none;
    }

    &::after {
      content: '🗑️';
      position: absolute;
      bottom: 1rem;
      right: 1rem;
      font-size: 1.5rem;
      z-index: 2;
      pointer-events: none;
    }
  }

  &.written-off {
    position: relative;
    filter: contrast(0.8);

    &::before {
      content: '';
      position: absolute;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      background-color: rgba(226, 227, 229, 0.15);
      background-image:
        linear-gradient(45deg, transparent, transparent 45%, rgba(56, 61, 65, 0.15) 46%, rgba(56, 61, 65, 0.15) 54%, transparent 55%, transparent),
        linear-gradient(135deg, transparent, transparent 45%, rgba(56, 61, 65, 0.15) 46%, rgba(56, 61, 65, 0.15) 54%, transparent 55%, transparent);
      background-size: 30px 30px;
      z-index: 1;
      pointer-events: none;
    }
  }
}

.commodity-content {
  flex: 1;
  cursor: pointer;
}

.commodity-actions {
  display: flex;
  gap: 0.5rem;
  margin-left: 1rem;
  cursor: pointer;
}

.commodity-meta {
  display: flex;
  justify-content: space-between;
  margin-top: 0.5rem;
  font-size: 0.9rem;
  color: $text-color;
}

.type {
  font-style: italic;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.commodity-price {
  margin-top: 1rem;
  font-weight: bold;
  font-size: 1.1rem;
  display: flex;
  flex-direction: column;
}

.price-per-unit {
  font-size: 0.8rem;
  font-weight: normal;
  font-style: italic;
  color: $text-color;
  margin-top: 0.25rem;
}

.commodity-status {
  margin-top: 0.5rem;

  &.with-draft {
    display: flex;
    justify-content: space-between;
    align-items: center;

    &::after {
      content: 'Draft';
      font-size: 0.8rem;
      font-weight: 500;
      color: $text-secondary-color;
      font-style: italic;
      transform: rotate(-45deg);
      position: absolute;
      bottom: 0.5rem;
      right: 0.5rem;
    }
  }
}

.status {
  display: inline-block;
  padding: 0.25rem 0.5rem;
  border-radius: $default-radius;
  font-size: 0.8rem;
  font-weight: 500;

  &.in_use {
    background-color: #d4edda;
    color: #155724;
  }

  &.sold {
    background-color: #cce5ff;
    color: #004085;
  }

  &.lost {
    background-color: #fff3cd;
    color: #856404;
  }

  &.disposed {
    background-color: #f8d7da;
    color: $error-text-color;
  }

  &.written_off {
    background-color: #e2e3e5;
    color: #383d41;
  }
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

.btn-secondary {
  background-color: $secondary-color;
  color: white;
  border: none;
  cursor: pointer;
}

.btn-danger {
  background-color: $danger-color;
  color: white;
  border: none;
  cursor: pointer;
}

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
  margin-top: 0;
  border-radius: $default-radius;
}

.test-result, .test-error {
  margin-top: 1rem;
  padding: 1rem;
  border-radius: $default-radius;
}

.test-result {
  background-color: lighten($primary-color, 40%);
}

.test-error {
  background-color: lighten($danger-color, 40%);
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
