<template>
  <div class="area-list">
    <div class="header">
      <h1>Areas</h1>
      <router-link to="/areas/new" class="btn btn-primary"><font-awesome-icon icon="plus" /> New</router-link>
    </div>

    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="areas.length === 0" class="empty">No areas found. Create your first area!</div>

    <div v-else class="areas-grid">
      <div v-for="area in areas" :key="area.id" class="area-card" @click="viewArea(area.id)">
        <div class="area-content">
          <h3>{{ area.attributes.name }}</h3>
          <div class="area-meta" v-if="area.attributes.location_id">
            <span class="location">Location: {{ getLocationName(area.attributes.location_id) }}</span>
          </div>
        </div>
        <div class="area-actions">
          <button class="btn btn-secondary btn-sm" @click.stop="editArea(area.id)" title="Edit">
            <font-awesome-icon icon="edit" />
          </button>
          <button class="btn btn-danger btn-sm" @click.stop="confirmDelete(area.id)" title="Delete">
            <font-awesome-icon icon="trash" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import areaService from '@/services/areaService'
import locationService from '@/services/locationService'

const router = useRouter()
const areas = ref<any[]>([])
const locations = ref<any[]>([])
const loading = ref<boolean>(true)
const error = ref<string | null>(null)

onMounted(async () => {
  try {
    // Load areas and locations in parallel
    const [areasResponse, locationsResponse] = await Promise.all([
      areaService.getAreas(),
      locationService.getLocations()
    ])

    areas.value = areasResponse.data.data
    locations.value = locationsResponse.data.data
    loading.value = false
  } catch (err: any) {
    error.value = 'Failed to load areas: ' + (err.message || 'Unknown error')
    loading.value = false
  }
})

const getLocationName = (locationId: string) => {
  const location = locations.value.find(l => l.id === locationId)
  return location ? location.attributes.name : 'Unknown Location'
}

const viewArea = (id: string) => {
  router.push(`/areas/${id}`)
}

const editArea = (id: string) => {
  router.push(`/areas/${id}/edit`)
}

const confirmDelete = (id: string) => {
  if (confirm('Are you sure you want to delete this area?')) {
    deleteArea(id)
  }
}

const deleteArea = async (id: string) => {
  try {
    await areaService.deleteArea(id)
    // Remove the deleted area from the list
    areas.value = areas.value.filter(area => area.id !== id)
  } catch (err: any) {
    error.value = 'Failed to delete area: ' + (err.message || 'Unknown error')
  }
}
</script>

<style lang="scss" scoped>
@import '../../assets/main.scss';

.area-list {
  max-width: $container-max-width;
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
  border-radius: $default-radius;
  box-shadow: $box-shadow;
}

.error {
  color: $danger-color;
}

.areas-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 1.5rem;
}

.area-card {
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
}

.area-content {
  flex: 1;
  cursor: pointer;
}

.area-actions {
  display: flex;
  gap: 0.5rem;
  margin-left: 1rem;
  cursor: pointer;
}

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
}

.area-meta {
  margin-top: 0.5rem;
  font-size: 0.9rem;
  color: $text-color;
}

.location {
  font-style: italic;
}

.btn {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: $default-radius;
  cursor: pointer;
  font-weight: 500;
  text-decoration: none;
  display: inline-block;
}

.btn-primary {
  background-color: $primary-color;
  color: white;
}

.btn-secondary {
  background-color: $secondary-color;
  color: white;
}
</style>
