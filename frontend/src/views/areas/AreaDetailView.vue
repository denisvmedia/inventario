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
          <div v-for="commodity in commodities" :key="commodity.id" class="commodity-card" :class="{ 'highlighted': commodity.id === highlightCommodityId }">
            <div class="commodity-content" @click="viewCommodity(commodity.id)">
              <h3>{{ commodity.attributes.name }}</h3>
              <div class="commodity-meta">
                <span class="type">
                  <font-awesome-icon :icon="getTypeIcon(commodity.attributes.type)" />
                  {{ getTypeName(commodity.attributes.type) }}
                </span>
                <span class="count" v-if="(commodity.attributes.count || 1) > 1">×{{ commodity.attributes.count }}</span>
              </div>
              <div class="commodity-price" v-if="commodity.attributes.current_price">
                <span class="price">{{ commodity.attributes.current_price }} {{ mainCurrency }}</span>
                <span class="price-per-unit" v-if="(commodity.attributes.count || 1) > 1">
                  {{ calculatePricePerUnit(commodity) }} {{ mainCurrency }} per unit
                </span>
              </div>
              <div class="commodity-status" v-if="commodity.attributes.status">
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
import settingsService from '@/services/settingsService'
import { COMMODITY_TYPES } from '@/constants/commodityTypes'
import { COMMODITY_STATUSES } from '@/constants/commodityStatuses'

const router = useRouter()
const route = useRoute()
const area = ref<any>(null)
const locations = ref<any[]>([])
const commodities = ref<any[]>([])
const loading = ref<boolean>(true)
const error = ref<string | null>(null)
const locationName = ref<string | null>(null)
const locationAddress = ref<string | null>(null)
const mainCurrency = ref<string>('USD') // Default to USD if not set

// Highlight commodity if specified in the URL
const highlightCommodityId = ref(route.query.highlightCommodityId as string || '')
let highlightTimeout: number | null = null

onMounted(async () => {
  const id = route.params.id as string

  try {
    // Fetch main currency from settings
    try {
      const currency = await settingsService.getMainCurrency()
      if (currency) {
        mainCurrency.value = currency
      }
    } catch (settingsErr) {
      console.error('Failed to load main currency from settings:', settingsErr)
      // Continue with default currency
    }

    // Load area, locations, and commodities in parallel
    const [areaResponse, locationsResponse, commoditiesResponse] = await Promise.all([
      areaService.getArea(id),
      locationService.getLocations(),
      commodityService.getCommodities()
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

// Calculate price per unit
const calculatePricePerUnit = (commodity: any) => {
  const price = parseFloat(commodity.attributes.current_price) || 0
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
