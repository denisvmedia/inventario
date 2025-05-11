<template>
  <div class="location-detail">
    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="!location" class="not-found">Location not found</div>
    <div v-else>
      <div class="header">
        <div class="title-section">
          <h1>{{ location.attributes.name }}</h1>
          <p class="address">{{ location.attributes.address || 'No address provided' }}</p>
        </div>
        <div class="actions">
          <button class="btn btn-secondary" @click="editLocation">Edit</button>
          <button class="btn btn-danger" @click="confirmDelete">Delete</button>
        </div>
      </div>



      <div class="areas-section">
        <div class="section-header">
          <h2>Areas</h2>
          <button class="btn btn-primary btn-sm" @click="showAreaForm = !showAreaForm">
            {{ showAreaForm ? 'Cancel' : 'Add Area' }}
          </button>
        </div>

        <!-- Inline Area Creation Form -->
        <AreaForm
          v-if="showAreaForm"
          :locationId="location.id"
          @created="handleAreaCreated"
          @cancel="showAreaForm = false"
        />

        <div v-if="areas.length > 0" class="areas-grid">
          <div v-for="area in areas" :key="area.id" class="area-card" @click="viewArea(area.id)">
            <div class="area-content">
              <h3>{{ area.attributes.name }}</h3>
            </div>
            <div class="area-actions">
              <button class="btn btn-secondary btn-sm" @click.stop="editArea(area.id)">
                Edit
              </button>
              <button class="btn btn-danger btn-sm" @click.stop="confirmDeleteArea(area.id)">
                Delete
              </button>
            </div>
          </div>
        </div>
        <div v-else class="no-areas">
          <p>No areas found for this location. Use the button above to add your first area.</p>
        </div>
      </div>

      <!-- Test API Results Section -->
      <div v-if="testResult || testError" class="test-section">
        <h2>API Test Results</h2>
        <div v-if="testResult" class="test-result">
          <h3>Success:</h3>
          <pre>{{ testResult }}</pre>
        </div>
        <div v-if="testError" class="test-error">
          <h3>Error:</h3>
          <pre>{{ testError }}</pre>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import locationService from '@/services/locationService'
import areaService from '@/services/areaService'
import AreaForm from '@/components/AreaForm.vue'


const route = useRoute()
const router = useRouter()
const loading = ref<boolean>(true)
const error = ref<string | null>(null)
const location = ref<any>(null)
const areas = ref<any[]>([])



// Test API variables
const testResult = ref('')
const testError = ref('')

// State for inline forms
const showAreaForm = ref(false)

onMounted(async () => {
  const id = route.params.id as string

  try {
    // Load location and areas in parallel
    const [locationResponse, areasResponse] = await Promise.all([
      locationService.getLocation(id),
      areaService.getAreas()
    ])

    location.value = locationResponse.data.data

    // Filter areas that belong to this location
    areas.value = areasResponse.data.data.filter(
      (area: any) => area.attributes.location_id === id
    )

    loading.value = false


  } catch (err: any) {
    error.value = 'Failed to load location: ' + (err.message || 'Unknown error')
    loading.value = false
  }
})



const editLocation = () => {
  router.push(`/locations/${location.value.id}/edit`)
}

const confirmDelete = () => {
  if (confirm('Are you sure you want to delete this location?')) {
    deleteLocation()
  }
}

const deleteLocation = async () => {
  try {
    await locationService.deleteLocation(location.value.id)
    router.push('/locations')
  } catch (err: any) {
    error.value = 'Failed to delete location: ' + (err.message || 'Unknown error')
  }
}

const viewArea = (id: string) => {
  router.push(`/areas/${id}`)
}

const editArea = (id: string) => {
  router.push(`/areas/${id}/edit`)
}

const confirmDeleteArea = (id: string) => {
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

// Handle area creation
const handleAreaCreated = (newArea: any) => {
  areas.value.push(newArea)
  showAreaForm.value = false
}
</script>

<style scoped>
.location-detail {
  max-width: 1200px;
  margin: 0 auto;
  padding: 20px;
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
}

.title-section h1 {
  margin-bottom: 0.5rem;
}

.address {
  color: #666;
  font-style: italic;
  margin-top: 0;
}

.actions {
  display: flex;
  gap: 0.5rem;
}

.loading, .error, .not-found, .no-areas {
  text-align: center;
  padding: 2rem;
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  margin-bottom: 2rem;
}

.error {
  color: #dc3545;
}

.info-card {
  background: white;
  border-radius: 8px;
  padding: 1.5rem;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  margin-bottom: 2rem;
}

.full-width {
  width: 100%;
}

.areas-section {
  margin-bottom: 2rem;
}

.areas-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 1.5rem;
}

.area-card {
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

.area-card:hover {
  transform: translateY(-5px);
  box-shadow: 0 5px 15px rgba(0, 0, 0, 0.1);
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

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid #eee;
}

.test-section {
  margin-bottom: 2rem;
  background: white;
  border-radius: 8px;
  padding: 1.5rem;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.test-section h2 {
  margin-bottom: 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid #eee;
}

.btn-primary {
  background-color: #4CAF50;
  color: white;
  text-decoration: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  display: inline-block;
  margin-top: 1rem;
}

.btn-secondary {
  background-color: #6c757d;
  color: white;
  border: none;
  cursor: pointer;
}

.btn-danger {
  background-color: #dc3545;
  color: white;
  border: none;
  cursor: pointer;
}

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
  margin-top: 0;
  border-radius: 4px;
}

.test-result, .test-error {
  margin-top: 1rem;
  padding: 1rem;
  border-radius: 4px;
}

.test-result {
  background-color: #e6f7e6;
}

.test-error {
  background-color: #f7e6e6;
}

pre {
  white-space: pre-wrap;
  word-wrap: break-word;
  overflow-x: auto;
  background: #f8f9fa;
  padding: 0.5rem;
  border-radius: 4px;
}

.btn-info {
  background-color: #17a2b8;
  color: white;
}

.btn-info:hover {
  background-color: #138496;
}
</style>
