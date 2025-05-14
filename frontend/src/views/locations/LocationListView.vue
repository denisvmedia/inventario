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
      <div class="empty-message">
        <p>No locations found. Create your first location!</p>
        <div class="action-button">
          <button class="btn btn-primary" @click="showLocationForm = true">Create Location</button>
        </div>
      </div>
    </div>

    <div v-else class="locations-list">
      <div v-for="location in locations" :key="location.id" class="location-container">
        <div class="location-card" @click="toggleLocationExpanded(location.id)">
          <div class="location-content">
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
            <div
              v-for="area in getAreasForLocation(location.id)"
              :key="area.id"
              :id="`area-${area.id}`"
              class="area-card"
              :class="{ 'area-highlight': areaToFocus === area.id }"
              @click="viewArea(area.id)"
            >
              <div class="area-content">
                <h5>{{ area.attributes.name }}</h5>
                <div v-if="!valuesLoading" class="area-value">
                  {{ getAreaValue(area.id) }}
                </div>
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
import { ref, onMounted, nextTick, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import locationService from '@/services/locationService'
import areaService from '@/services/areaService'
import valueService from '@/services/valueService'
import { useSettingsStore } from '@/stores/settingsStore'
import { formatPrice } from '@/services/currencyService'
import LocationForm from '@/components/LocationForm.vue'
import AreaForm from '@/components/AreaForm.vue'

const router = useRouter()
const route = useRoute()
const settingsStore = useSettingsStore()
const locations = ref<any[]>([])
const areas = ref<any[]>([])
const loading = ref<boolean>(true)
const error = ref<string | null>(null)

// Values data
const areaTotals = ref<any[]>([])
const valuesLoading = ref<boolean>(true)
const valuesError = ref<string | null>(null)

// Main currency from settings store
const mainCurrency = computed(() => settingsStore.mainCurrency)

// State for inline forms
const showLocationForm = ref(false)
const showAreaFormForLocation = ref<string | null>(null)

// Track expanded locations
const expandedLocations = ref<string[]>([])

// Reference to the area element to scroll to
const areaToFocus = ref<string | null>(null)

// Function to load values
async function loadValues() {
  valuesLoading.value = true
  valuesError.value = null

  try {
    const response = await valueService.getValues()
    const data = response.data.data.attributes

    // Store area totals - ensure it's an array
    if (Array.isArray(data.area_totals)) {
      areaTotals.value = data.area_totals
    } else {
      console.log('Area totals is not an array:', data.area_totals)
      // Convert to array if it's an object with key-value pairs
      if (data.area_totals && typeof data.area_totals === 'object') {
        areaTotals.value = Object.entries(data.area_totals).map(([id, value]) => ({
          id,
          value
        }))
      } else {
        areaTotals.value = []
      }
    }
  } catch (error) {
    console.error('Error loading values:', error)
    valuesError.value = 'Failed to load inventory values'
  } finally {
    valuesLoading.value = false
  }
}

// Function to get the value for a specific area
const getAreaValue = (areaId: string): string => {
  if (valuesLoading.value) return 'Loading...'

  // Check if areaTotals is an array
  if (!Array.isArray(areaTotals.value)) {
    console.error('areaTotals is not an array:', areaTotals.value)
    return '0.00 ' + mainCurrency.value
  }

  // Find the area value in the array
  const areaValue = areaTotals.value.find(area => area.id === areaId)
  if (areaValue && areaValue.value) {
    // Handle both string and number values
    const valueAsNumber = typeof areaValue.value === 'string'
      ? parseFloat(areaValue.value)
      : areaValue.value

    if (!isNaN(valueAsNumber)) {
      return formatPrice(valueAsNumber, mainCurrency.value)
    }
  }

  return '0.00 ' + mainCurrency.value
}

onMounted(async () => {
  try {
    // Make sure we have the main currency
    await settingsStore.fetchMainCurrency()

    // Load locations, areas, and values in parallel
    const [locationsResponse, areasResponse, _] = await Promise.all([
      locationService.getLocations(),
      areaService.getAreas(),
      loadValues() // Load values in parallel
    ])

    locations.value = locationsResponse.data.data
    areas.value = areasResponse.data.data
    loading.value = false

    // Check for query parameters
    const areaId = route.query.areaId as string
    const locationId = route.query.locationId as string

    if (areaId && locationId) {
      // Expand the location that contains the area
      if (!expandedLocations.value.includes(locationId)) {
        expandedLocations.value.push(locationId)
      }

      // Set the area to focus on
      areaToFocus.value = areaId

      // Wait for the DOM to update before scrolling
      await nextTick()
      scrollToArea(areaId)
    } else if (locations.value.length === 1) {
      // If there's only one location, expand it by default
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

// Function to scroll to a specific area
const scrollToArea = (areaId: string) => {
  // Find the area element by its ID
  const areaElement = document.getElementById(`area-${areaId}`)
  if (areaElement) {
    // Scroll the area into view with smooth behavior
    areaElement.scrollIntoView({ behavior: 'smooth', block: 'center' })

    // Add a temporary highlight class to make it more visible
    areaElement.classList.add('area-highlight')

    // Remove the highlight class after a delay
    setTimeout(() => {
      areaElement.classList.remove('area-highlight')
    }, 2000)
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

<style lang="scss" scoped>
@import '../../assets/main.scss';

.location-list {
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
  margin-bottom: 1.5rem;
}

.error {
  color: $danger-color;
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
  border-radius: $default-radius;
  padding: 1.5rem;
  box-shadow: $box-shadow;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  transition: box-shadow 0.2s;
  cursor: pointer;

  &:hover {
    box-shadow: 0 5px 15px rgba(0, 0, 0, 0.1);
  }
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
  color: $text-color;
}

.location-actions {
  display: flex;
  gap: 0.5rem;
  margin-left: 1rem;
}

.address {
  color: $text-color;
  margin: 0.5rem 0 0;
  font-size: 0.9rem;
  font-style: italic;
}

/* Areas styling */
.areas-container {
  margin-top: 0.5rem;
  margin-left: 2rem;
  padding: 1rem;
  background: $light-bg-color;
  border-radius: $default-radius;
  border-left: 4px solid $primary-color;
}

.areas-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;

  h4 {
    margin: 0;
    color: $text-color;
  }
}

.areas-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.area-card {
  background: white;
  border-radius: $default-radius;
  padding: 1rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  display: flex;
  justify-content: space-between;
  align-items: center;
  cursor: pointer;
  transition: background-color 0.3s ease, box-shadow 0.3s ease;

  &:hover {
    background-color: $light-bg-color;
    box-shadow: 0 2px 5px rgba(0, 0, 0, 0.15);
  }
}

.area-highlight {
  background-color: lighten($primary-color, 45%) !important;
  box-shadow: 0 0 0 2px $primary-color !important;
  animation: pulse 1s ease-in-out;
}

@keyframes pulse {
  0% { box-shadow: 0 0 0 0 rgba($primary-color, 0.7); }
  70% { box-shadow: 0 0 0 10px rgba($primary-color, 0); }
  100% { box-shadow: 0 0 0 0 rgba($primary-color, 0); }
}

.area-content {
  flex: 1;
  cursor: pointer;

  h5 {
    margin: 0;
    font-size: 1rem;
  }

  .area-value {
    font-size: 0.85rem;
    color: $primary-color;
    margin-top: 0.25rem;
    font-weight: 500;
  }
}

.area-actions {
  display: flex;
  gap: 0.5rem;
}

.no-areas {
  padding: 1rem;
  background: white;
  border-radius: $default-radius;
  text-align: center;
  color: $text-color;
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

/* Use global button styles from main.scss */

/* Button styles are inherited from main.scss */

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
}


</style>
