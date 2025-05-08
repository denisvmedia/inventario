<template>
  <div class="location-list">
    <div class="header">
      <h1>Locations</h1>
      <router-link to="/locations/new" class="btn btn-primary">Create New Location</router-link>
    </div>

    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="locations.length === 0" class="empty">No locations found. Create your first location!</div>

    <div v-else class="locations-grid">
      <div v-for="location in locations" :key="location.id" class="location-card">
        <div class="location-content" @click="viewLocation(location.id)">
          <h3>{{ location.attributes.name }}</h3>
          <p v-if="location.attributes.description" class="description">{{ location.attributes.description }}</p>
          <div class="location-meta" v-if="location.relationships && location.relationships.area">
            <span class="area">Area: {{ getAreaName(location.relationships.area.data.id) }}</span>
          </div>
        </div>
        <div class="location-actions">
          <button class="btn btn-danger btn-sm" @click.stop="confirmDelete(location.id)">
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
import locationService from '@/services/locationService'
import areaService from '@/services/areaService'

const router = useRouter()
const locations = ref<any[]>([])
const areas = ref<any[]>([])
const loading = ref<boolean>(true)
const error = ref<string | null>(null)

onMounted(async () => {
  try {
    // Load locations and areas in parallel
    const [locationsResponse, areasResponse] = await Promise.all([
      locationService.getLocations(),
      areaService.getAreas()
    ])

    locations.value = locationsResponse.data.data
    areas.value = areasResponse.data.data
    loading.value = false
  } catch (err: any) {
    error.value = 'Failed to load locations: ' + (err.message || 'Unknown error')
    loading.value = false
  }
})

const getAreaName = (areaId: string) => {
  const area = areas.value.find(a => a.id === areaId)
  return area ? area.attributes.name : 'Unknown Area'
}

const viewLocation = (id: string) => {
  router.push(`/locations/${id}`)
}

const confirmDelete = (id: string) => {
  if (confirm('Are you sure you want to delete this location?')) {
    deleteLocation(id)
  }
}

const deleteLocation = async (id: string) => {
  try {
    await locationService.deleteLocation(id)
    // Remove the deleted location from the list
    locations.value = locations.value.filter(location => location.id !== id)
  } catch (err: any) {
    error.value = 'Failed to delete location: ' + (err.message || 'Unknown error')
  }
}
</script>

<style scoped>
.location-list {
  max-width: 1200px;
  margin: 0 auto;
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

.locations-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 1.5rem;
}

.location-card {
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

.location-card:hover {
  transform: translateY(-5px);
  box-shadow: 0 5px 15px rgba(0, 0, 0, 0.1);
}

.location-content {
  flex: 1;
  cursor: pointer;
}

.location-actions {
  display: flex;
  gap: 0.5rem;
  margin-left: 1rem;
}

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
}

.description {
  color: #666;
  margin: 0.5rem 0;
  font-size: 0.9rem;
}

.location-meta {
  margin-top: 1rem;
  font-size: 0.9rem;
  color: #555;
}
</style>
