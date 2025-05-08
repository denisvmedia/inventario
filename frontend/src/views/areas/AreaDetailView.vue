<template>
  <div class="area-detail">
    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="!area" class="not-found">Area not found</div>
    <div v-else>
      <div class="header">
        <h1>{{ area.attributes.name }}</h1>
        <div class="actions">
          <button class="btn btn-secondary" @click="editArea">Edit</button>
          <button class="btn btn-danger" @click="confirmDelete">Delete</button>
        </div>
      </div>

      <div class="area-info">
        <div class="info-card">
          <h2>Details</h2>
          <!-- Add area details here -->
        </div>

        <div class="info-card" v-if="locations.length > 0">
          <h2>Locations</h2>
          <ul class="locations-list">
            <li v-for="location in locations" :key="location.id" @click="viewLocation(location.id)">
              {{ location.attributes.name }}
            </li>
          </ul>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import areaService from '@/services/areaService'
import locationService from '@/services/locationService'

const router = useRouter()
const route = useRoute()
const area = ref<any>(null)
const locations = ref<any[]>([])
const loading = ref<boolean>(true)
const error = ref<string | null>(null)

onMounted(async () => {
  const id = route.params.id as string

  try {
    const response = await areaService.getArea(id)
    area.value = response.data.data

    // Load locations for this area
    try {
      const locationsResponse = await locationService.getLocations()
      // Filter locations that belong to this area
      locations.value = locationsResponse.data.data.filter(
        (location: any) =>
          location.relationships &&
          location.relationships.area &&
          location.relationships.area.data.id === id
      )
    } catch (err) {
      console.error('Failed to load locations:', err)
    }

    loading.value = false
  } catch (err: any) {
    error.value = 'Failed to load area: ' + (err.message || 'Unknown error')
    loading.value = false
  }
})

const editArea = () => {
  router.push(`/areas/${area.value.id}/edit`)
}

const confirmDelete = () => {
  if (confirm('Are you sure you want to delete this area?')) {
    deleteArea()
  }
}

const deleteArea = async () => {
  try {
    await areaService.deleteArea(area.value.id)
    alert('Area deleted successfully')
    router.push('/areas')
  } catch (err: any) {
    error.value = 'Failed to delete area: ' + (err.message || 'Unknown error')
  }
}

const viewLocation = (id: string) => {
  router.push(`/locations/${id}`)
}
</script>

<style scoped>
.area-detail {
  max-width: 1200px;
  margin: 0 auto;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2rem;
}

.actions {
  display: flex;
  gap: 0.5rem;
}

.loading, .error, .not-found {
  text-align: center;
  padding: 2rem;
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.error {
  color: #dc3545;
}

.area-info {
  display: grid;
  grid-template-columns: 1fr;
  gap: 1.5rem;
}

.info-card {
  background: white;
  border-radius: 8px;
  padding: 1.5rem;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.info-card h2 {
  margin-bottom: 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid #eee;
}

.locations-list {
  list-style: none;
  padding: 0;
  margin: 0;
}

.locations-list li {
  padding: 0.75rem 0;
  border-bottom: 1px solid #eee;
  cursor: pointer;
}

.locations-list li:last-child {
  border-bottom: none;
}

.locations-list li:hover {
  color: #4CAF50;
}

@media (min-width: 768px) {
  .area-info {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
