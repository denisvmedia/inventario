<template>
  <div class="location-list">
    <div class="header">
      <h1>Locations</h1>
      <button class="btn btn-primary" @click="showLocationForm = !showLocationForm">
        <font-awesome-icon :icon="showLocationForm ? 'times' : 'plus'" /> {{ showLocationForm ? 'Cancel' : 'New' }}
      </button>
    </div>

    <!-- Inline Location Creation Form -->
    <LocationForm
      v-if="showLocationForm"
      @created="handleLocationCreated"
      @cancel="showLocationForm = false"
    />

    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="locations.length === 0" class="empty">
      No locations found. Create your first location using the button above!
    </div>

    <div v-else class="locations-list">
      <div v-for="location in locations" :key="location.id" class="location-container">
        <div class="location-card">
          <div class="location-content" @click="toggleLocationExpanded(location.id)">
            <div class="location-header">
              <h3>{{ location.attributes.name }}</h3>
              <div class="location-expand-icon">
                <font-awesome-icon :icon="expandedLocations.includes(location.id) ? 'chevron-down' : 'chevron-right'" />
              </div>
            </div>
            <p v-if="location.attributes.address" class="address">{{ location.attributes.address }}</p>
          </div>
          <div class="location-actions">
            <button class="btn btn-secondary btn-sm" @click.stop="editLocation(location.id)" title="Edit">
              <font-awesome-icon icon="edit" />
            </button>
            <button class="btn btn-danger btn-sm" @click.stop="confirmDeleteLocation(location.id)" title="Delete">
              <font-awesome-icon icon="trash" />
            </button>
          </div>
        </div>

        <!-- Areas for this location (shown when expanded) -->
        <div v-if="expandedLocations.includes(location.id)" class="areas-container">
          <div class="areas-header">
            <h4>Areas</h4>
            <button class="btn btn-primary btn-sm" @click="toggleAreaForm(location.id)">
              {{ showAreaFormForLocation === location.id ? 'Cancel' : 'Add Area' }}
            </button>
          </div>

          <!-- Inline Area Creation Form -->
          <AreaForm
            v-if="showAreaFormForLocation === location.id"
            :locationId="location.id"
            @created="handleAreaCreated"
            @cancel="showAreaFormForLocation = null"
          />

          <!-- Areas List -->
          <div v-if="getAreasForLocation(location.id).length > 0" class="areas-list">
            <div v-for="area in getAreasForLocation(location.id)" :key="area.id" class="area-card">
              <div class="area-content" @click="viewArea(area.id)">
                <h5>{{ area.attributes.name }}</h5>
              </div>
              <div class="area-actions">
                <button class="btn btn-secondary btn-sm" @click.stop="editArea(area.id)" title="Edit">
                  <font-awesome-icon icon="edit" />
                </button>
                <button class="btn btn-danger btn-sm" @click.stop="confirmDeleteArea(area.id)" title="Delete">
                  <font-awesome-icon icon="trash" />
                </button>
              </div>
            </div>
          </div>
          <div v-else class="no-areas">
            <p>No areas found for this location. Add your first area using the button above.</p>
          </div>
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
import LocationForm from '@/components/LocationForm.vue'
import AreaForm from '@/components/AreaForm.vue'

const router = useRouter()
const locations = ref<any[]>([])
const areas = ref<any[]>([])
const loading = ref<boolean>(true)
const error = ref<string | null>(null)

// State for inline forms
const showLocationForm = ref(false)
const showAreaFormForLocation = ref<string | null>(null)

// Track expanded locations
const expandedLocations = ref<string[]>([])

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

    // If there's only one location, expand it by default
    if (locations.value.length === 1) {
      expandedLocations.value = [locations.value[0].id]
    }
  } catch (err: any) {
    error.value = 'Failed to load locations: ' + (err.message || 'Unknown error')
    loading.value = false
  }
})

// Toggle location expanded state
const toggleLocationExpanded = (locationId: string) => {
  if (expandedLocations.value.includes(locationId)) {
    expandedLocations.value = expandedLocations.value.filter(id => id !== locationId)
  } else {
    expandedLocations.value.push(locationId)
  }
}

// Toggle area form visibility
const toggleAreaForm = (locationId: string) => {
  showAreaFormForLocation.value = showAreaFormForLocation.value === locationId ? null : locationId
}

// Get areas for a specific location
const getAreasForLocation = (locationId: string) => {
  return areas.value.filter(area => area.attributes.location_id === locationId)
}

// Handle location creation
const handleLocationCreated = (newLocation: any) => {
  locations.value.push(newLocation)
  showLocationForm.value = false
  // Expand the newly created location
  expandedLocations.value.push(newLocation.id)
}

// Handle area creation
const handleAreaCreated = (newArea: any) => {
  areas.value.push(newArea)
  showAreaFormForLocation.value = null
}

// Location actions
const editLocation = (id: string) => {
  router.push(`/locations/${id}/edit`)
}

const confirmDeleteLocation = (id: string) => {
  if (confirm('Are you sure you want to delete this location? This will also delete all areas within this location.')) {
    deleteLocation(id)
  }
}

const deleteLocation = async (id: string) => {
  try {
    await locationService.deleteLocation(id)
    // Remove the deleted location from the list
    locations.value = locations.value.filter(location => location.id !== id)
    // Also remove any areas that belonged to this location
    areas.value = areas.value.filter(area => area.attributes.location_id !== id)
    // Remove from expanded locations if present
    expandedLocations.value = expandedLocations.value.filter(locationId => locationId !== id)
  } catch (err: any) {
    error.value = 'Failed to delete location: ' + (err.message || 'Unknown error')
  }
}

// Area actions
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
</script>

<style scoped>
.location-list {
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
  margin-bottom: 1.5rem;
}

.error {
  color: #dc3545;
}

.locations-list {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.location-container {
  display: flex;
  flex-direction: column;
}

.location-card {
  background: white;
  border-radius: 8px;
  padding: 1.5rem;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  transition: box-shadow 0.2s;
}

.location-card:hover {
  box-shadow: 0 5px 15px rgba(0, 0, 0, 0.1);
}

.location-content {
  flex: 1;
  cursor: pointer;
}

.location-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.location-expand-icon {
  margin-left: 1rem;
  color: #666;
}

.location-actions {
  display: flex;
  gap: 0.5rem;
  margin-left: 1rem;
}

.address {
  color: #666;
  margin: 0.5rem 0 0;
  font-size: 0.9rem;
  font-style: italic;
}

/* Areas styling */
.areas-container {
  margin-top: 0.5rem;
  margin-left: 2rem;
  padding: 1rem;
  background: #f9f9f9;
  border-radius: 8px;
  border-left: 4px solid #4CAF50;
}

.areas-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.areas-header h4 {
  margin: 0;
  color: #333;
}

.areas-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.area-card {
  background: white;
  border-radius: 6px;
  padding: 1rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.area-content {
  flex: 1;
  cursor: pointer;
}

.area-content h5 {
  margin: 0;
  font-size: 1rem;
}

.area-actions {
  display: flex;
  gap: 0.5rem;
}

.no-areas {
  padding: 1rem;
  background: white;
  border-radius: 6px;
  text-align: center;
  color: #666;
}

/* Button styling */
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

.btn-danger {
  background-color: #dc3545;
  color: white;
}

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
}


</style>
