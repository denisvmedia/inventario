<template>
  <div class="commodity-list">
    <div class="header">
      <div class="header-title">
        <h1>Commodities</h1>
        <div v-if="!valuesLoading && globalTotal > 0" class="total-value">
          Total Value: <span class="value-amount">{{ formatPrice(globalTotal, mainCurrency) }}</span>
        </div>
      </div>
      <router-link v-if="hasLocationsAndAreas" to="/commodities/new" class="btn btn-primary"><font-awesome-icon icon="plus" /> New</router-link>
      <router-link v-else-if="locations.length === 0" to="/locations" class="btn btn-primary"><font-awesome-icon icon="plus" /> Create Location First</router-link>
      <router-link v-else-if="areas.length === 0" to="/locations" class="btn btn-primary"><font-awesome-icon icon="plus" /> Create Area First</router-link>
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
      <div v-for="commodity in commodities" :key="commodity.id" class="commodity-card" :class="{
        'highlighted': commodity.id === highlightCommodityId,
        'draft': commodity.attributes.draft,
        'sold': !commodity.attributes.draft && commodity.attributes.status === 'sold',
        'lost': !commodity.attributes.draft && commodity.attributes.status === 'lost',
        'disposed': !commodity.attributes.draft && commodity.attributes.status === 'disposed',
        'written-off': !commodity.attributes.draft && commodity.attributes.status === 'written_off'
      }" @click="viewCommodity(commodity.id)">
        <div class="commodity-content">
          <h3>{{ commodity.attributes.name }}</h3>
          <div class="commodity-location" v-if="commodity.attributes.area_id">
            <span class="location-info">
              <font-awesome-icon icon="map-marker-alt" />
              {{ getLocationName(commodity.attributes.area_id) }} / {{ getAreaName(commodity.attributes.area_id) }}
            </span>
          </div>
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
          <div class="commodity-status" :class="{ 'with-draft': commodity.attributes.draft }">
            <span class="status" :class="commodity.attributes.status">{{ getStatusName(commodity.attributes.status) }}</span>
          </div>
        </div>
        <div class="commodity-actions">
          <button class="btn btn-secondary btn-sm" @click.stop="editCommodity(commodity.id)" title="Edit">
            <font-awesome-icon icon="edit" />
          </button>
          <button class="btn btn-danger btn-sm" @click.stop="confirmDelete(commodity.id)" title="Delete">
            <font-awesome-icon icon="trash" />
          </button>
        </div>
      </div>
    </div>
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
import { COMMODITY_TYPES } from '@/constants/commodityTypes'
import { COMMODITY_STATUSES } from '@/constants/commodityStatuses'
import { formatPrice, calculatePricePerUnit, getDisplayPrice } from '@/services/currencyService'

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

// Computed property to check if there are locations and areas
const hasLocationsAndAreas = computed(() => {
  return locations.value.length > 0 && areas.value.length > 0
})

// Map to store area and location information
const areaMap = ref<Record<string, any>>({})
const locationMap = ref<Record<string, any>>({})

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

// Get area name for a commodity
const getAreaName = (areaId: string) => {
  return areaMap.value[areaId]?.name || ''
}

// Get location name for an area
const getLocationName = (areaId: string) => {
  const locationId = areaMap.value[areaId]?.locationId
  return locationId ? locationMap.value[locationId]?.name || '' : ''
}

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
    const [commoditiesResponse, areasResponse, locationsResponse, _] = await Promise.all([
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

const confirmDelete = (id: string) => {
  if (confirm('Are you sure you want to delete this commodity?')) {
    deleteCommodity(id)
  }
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

.commodity-list {
  max-width: $container-max-width;
  margin: 0 auto;
  padding: 20px;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 2rem;
}

.header-title {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
}

.total-value {
  font-size: 1rem;
  color: $text-color;
  margin-top: 0.5rem;

  .value-amount {
    font-weight: bold;
    color: $primary-color;
    font-size: 1.1rem;
  }
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
    background-color: #f9fff9;
  }

  &.draft {
    background: repeating-linear-gradient(45deg, #ffffff, #ffffff 5px, #eeeeee4d 5px, #eeeeee4d 7px);
    position: relative;
    filter: grayscale(0.8);

    h3, .commodity-location, .commodity-meta, .commodity-price, .price-per-unit {
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
    filter: contrast(0.95);

    &::before {
      content: '';
      position: absolute;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      background-color: rgba(226, 227, 229, 0.0375);
      background-image:
        linear-gradient(45deg, transparent, transparent 45%, rgba(56, 61, 65, 0.0375) 46%, rgba(56, 61, 65, 0.0375) 54%, transparent 55%, transparent),
        linear-gradient(135deg, transparent, transparent 45%, rgba(56, 61, 65, 0.0375) 46%, rgba(56, 61, 65, 0.0375) 54%, transparent 55%, transparent);
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

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
}

.commodity-location {
  margin-top: 0.5rem;
  font-size: 0.85rem;
  color: $text-color;
}

.location-info {
  display: flex;
  align-items: center;
  gap: 0.25rem;
}

.commodity-meta {
  display: flex;
  justify-content: space-between;
  margin-top: 0.5rem;
  font-size: 0.9rem;
  color: $text-color;
}

.type {
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
    color: #721c24;
  }

  &.written_off {
    background-color: #e2e3e5;
    color: #383d41;
  }
}

/* Use global button styles from main.scss */
</style>
