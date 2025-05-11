<template>
  <div class="commodity-list">
    <div class="header">
      <h1>Commodities</h1>
      <router-link to="/commodities/new" class="btn btn-primary">Create New Commodity</router-link>
    </div>

    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="commodities.length === 0" class="empty">No commodities found. Create your first commodity!</div>

    <div v-else class="commodities-grid">
      <div v-for="commodity in commodities" :key="commodity.id" class="commodity-card" @click="viewCommodity(commodity.id)">
        <div class="commodity-content">
          <h3>{{ commodity.attributes.name }}</h3>
          <div class="commodity-location" v-if="commodity.attributes.area_id">
            <span class="location-info">
              <i class="fas fa-map-marker-alt"></i>
              {{ getLocationName(commodity.attributes.area_id) }} / {{ getAreaName(commodity.attributes.area_id) }}
            </span>
          </div>
          <div class="commodity-meta">
            <span class="type">
              <i :class="getTypeIcon(commodity.attributes.type)"></i>
              {{ getTypeName(commodity.attributes.type) }}
            </span>
            <span class="count" v-if="(commodity.attributes.count || 1) > 1">Count: {{ commodity.attributes.count }}</span>
          </div>
          <div class="commodity-price">
            <span class="price">{{ commodity.attributes.current_price }} {{ commodity.attributes.original_price_currency }}</span>
          </div>
          <div class="commodity-status">
            <span class="status" :class="commodity.attributes.status">{{ getStatusName(commodity.attributes.status) }}</span>
          </div>
        </div>
        <div class="commodity-actions">
          <button class="btn btn-secondary btn-sm" @click.stop="editCommodity(commodity.id)">
            Edit
          </button>
          <button class="btn btn-danger btn-sm" @click.stop="confirmDelete(commodity.id)">
            Delete
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import commodityService from '@/services/commodityService'
import areaService from '@/services/areaService'
import locationService from '@/services/locationService'
import { COMMODITY_TYPES } from '@/constants/commodityTypes'
import { COMMODITY_STATUSES } from '@/constants/commodityStatuses'

const router = useRouter()
const commodities = ref<any[]>([])
const areas = ref<any[]>([])
const locations = ref<any[]>([])
const loading = ref<boolean>(true)
const error = ref<string | null>(null)

// Map to store area and location information
const areaMap = ref<Record<string, any>>({})
const locationMap = ref<Record<string, any>>({})

const getTypeIcon = (typeId: string) => {
  switch(typeId) {
    case 'white_goods':
      return 'fas fa-blender'
    case 'electronics':
      return 'fas fa-laptop'
    case 'equipment':
      return 'fas fa-tools'
    case 'furniture':
      return 'fas fa-couch'
    case 'clothes':
      return 'fas fa-tshirt'
    case 'other':
      return 'fas fa-box'
    default:
      return 'fas fa-box'
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

onMounted(async () => {
  try {
    // Load commodities, areas, and locations in parallel
    const [commoditiesResponse, areasResponse, locationsResponse] = await Promise.all([
      commodityService.getCommodities(),
      areaService.getAreas(),
      locationService.getLocations()
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
  } catch (err: any) {
    error.value = 'Failed to load data: ' + (err.message || 'Unknown error')
    loading.value = false
  }
})

const viewCommodity = (id: string) => {
  router.push(`/commodities/${id}`)
}

const editCommodity = (id: string) => {
  router.push(`/commodities/${id}/edit`)
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

<style scoped>
.commodity-list {
  max-width: 1200px;
  margin: 0 auto;
  padding: 20px;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2rem;
}

.loading, .error, .empty {
  text-align: center;
  padding: 2rem;
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.error {
  color: #dc3545;
}

.commodities-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 1.5rem;
}

.commodity-card {
  background: white;
  border-radius: 8px;
  padding: 1.5rem;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.commodity-card:hover {
  transform: translateY(-5px);
  box-shadow: 0 5px 15px rgba(0, 0, 0, 0.1);
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
  color: #666;
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
  color: #555;
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
}

.commodity-status {
  margin-top: 0.5rem;
}

.status {
  display: inline-block;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.8rem;
  font-weight: 500;
}

.status.in_use {
  background-color: #d4edda;
  color: #155724;
}

.status.sold {
  background-color: #cce5ff;
  color: #004085;
}

.status.lost {
  background-color: #fff3cd;
  color: #856404;
}

.status.disposed {
  background-color: #f8d7da;
  color: #721c24;
}

.status.written_off {
  background-color: #e2e3e5;
  color: #383d41;
}

.btn {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-weight: 500;
  text-decoration: none;
  display: inline-block;
}

.btn-primary {
  background-color: #4CAF50;
  color: white;
}

.btn-secondary {
  background-color: #6c757d;
  color: white;
}

/* Add Font Awesome icons */
@import url('https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0-beta3/css/all.min.css');
</style>
