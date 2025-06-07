<template>
  <div class="export-create">
    <div class="breadcrumb-nav">
      <router-link to="/exports" class="breadcrumb-link">
        <font-awesome-icon icon="arrow-left" /> Back to Exports
      </router-link>
    </div>
    <h1>Create New Export</h1>

    <div v-if="error" class="error-message">{{ error }}</div>

    <form @submit.prevent="createExport" class="export-form">
      <div class="form-section">
        <h2>Export Details</h2>

        <div class="form-group">
          <label for="description">Description</label>
          <textarea
            id="description"
            v-model="exportData.description"
            placeholder="Enter a description for this export..."
            rows="3"
            maxlength="500"
            required
            :class="{ 'is-invalid': formErrors.description }"
          ></textarea>
          <div v-if="formErrors.description" class="error-message">{{ formErrors.description }}</div>
          <div class="form-help">Describe what this export contains</div>
        </div>

        <div class="form-group">
          <label for="type">Export Type</label>
          <Select
            id="type"
            v-model="exportData.type"
            :options="exportTypeOptions"
            option-label="label"
            option-value="value"
            placeholder="Select export type..."
            class="w-100"
            :class="{ 'is-invalid': formErrors.type }"
            @change="onTypeChange"
          />
          <div v-if="formErrors.type" class="error-message">{{ formErrors.type }}</div>
          <div class="form-help">Choose what data to include in the export</div>
        </div>

        <div v-if="exportData.type === 'selected_items'" class="form-group">
          <label>Selected Items</label>
          <div class="hierarchical-selector">
            <div v-if="loadingItems" class="loading">Loading items...</div>
            <div v-else class="selection-tree">
              <!-- Locations -->
              <div v-for="location in locations" :key="location.id" :data-location_id="location.id" class="tree-item location-item">
                <div class="item-header">
                  <label class="item-checkbox">
                    <input
                      type="checkbox"
                      :checked="isLocationSelected(location.id)"
                      @change="toggleLocation(location.id, ($event.target as HTMLInputElement).checked)"
                    />
                    <span class="item-name">{{ location.name }}</span>
                    <span class="item-type">Location</span>
                  </label>
                </div>

                <!-- Location expanded content -->
                <div v-if="isLocationSelected(location.id)" class="item-content" :data-location_id="location.id">
                  <div class="inclusion-toggle">
                    <label class="checkbox-label">
                      <input
                        type="checkbox"
                        :checked="getLocationInclusion(location.id)"
                        @change="setLocationInclusion(location.id, ($event.target as HTMLInputElement).checked)"
                      />
                      <span>Include all areas and commodities in this location</span>
                    </label>
                  </div>

                  <!-- Areas in this location -->
                  <div v-if="!getLocationInclusion(location.id)" class="sub-items">
                    <div v-for="area in getAreasForLocation(location.id)" :key="area.id" :data-location_id="location.id" :data-area_id="area.id" class="tree-item area-item">
                      <div class="item-header">
                        <label class="item-checkbox">
                          <input
                            type="checkbox"
                            :checked="isAreaSelected(area.id)"
                            @change="toggleArea(area.id, ($event.target as HTMLInputElement).checked)"
                          />
                          <span class="item-name">{{ area.name }}</span>
                          <span class="item-type">Area</span>
                        </label>
                      </div>

                      <!-- Area expanded content -->
                      <div v-if="isAreaSelected(area.id)" class="item-content" :data-location_id="location.id" :data-area_id="area.id">
                        <div class="inclusion-toggle">
                          <label class="checkbox-label">
                            <input
                              type="checkbox"
                              :checked="getAreaInclusion(area.id)"
                              @change="setAreaInclusion(area.id, ($event.target as HTMLInputElement).checked)"
                            />
                            <span>Include all commodities in this area</span>
                          </label>
                        </div>

                        <!-- Commodities in this area -->
                        <div v-if="!getAreaInclusion(area.id)" class="sub-items">
                          <div v-for="commodity in getCommoditiesForArea(area.id)" :key="commodity.id" :data-location_id="location.id" :data-area_id="area.id" :data-commodity_id="commodity.id" class="tree-item commodity-item">
                            <div class="item-header">
                              <label class="item-checkbox">
                                <input
                                  type="checkbox"
                                  :checked="isCommoditySelected(commodity.id)"
                                  @change="toggleCommodity(commodity.id, ($event.target as HTMLInputElement).checked)"
                                />
                                <span class="item-name">{{ commodity.name }}</span>
                                <span class="item-type">Commodity</span>
                              </label>
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div v-if="formErrors.selected_items" class="error-message">{{ formErrors.selected_items }}</div>
          <div class="form-help">
            Selected: {{ exportData.selected_items ? exportData.selected_items.length : 0 }} item(s)
          </div>
        </div>

        <div class="form-group">
          <label class="checkbox-label">
            <input
              v-model="exportData.include_file_data"
              type="checkbox"
            />
            <span>Include file data (images, invoices, manuals)</span>
          </label>
          <div class="form-help">
            When enabled, exported XML will include base64-encoded file data.
            This makes the export larger but fully self-contained.
          </div>
        </div>
      </div>

      <div class="form-actions">
        <router-link to="/exports" class="btn btn-secondary">Cancel</router-link>
        <button
          type="submit"
          class="btn btn-primary"
          :disabled="!canSubmit || creating"
        >
          <font-awesome-icon v-if="creating" icon="spinner" spin />
          <font-awesome-icon v-else icon="plus" />
          {{ creating ? 'Creating...' : 'Create Export' }}
        </button>
      </div>
    </form>

    <div v-if="formError" class="form-error">{{ formError }}</div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, reactive, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import exportService from '@/services/exportService'
import locationService from '@/services/locationService'
import areaService from '@/services/areaService'
import commodityService from '@/services/commodityService'
import type { Export, ExportType, Location, Area, Commodity, ResourceObject } from '@/types'

const router = useRouter()

const exportTypeOptions = ref([
  { label: 'Full Database', value: 'full_database' },
  { label: 'Locations Only', value: 'locations' },
  { label: 'Areas Only', value: 'areas' },
  { label: 'Commodities Only', value: 'commodities' },
  { label: 'Selected Items', value: 'selected_items' }
])

const exportData = ref<Partial<Export>>({
  type: '' as ExportType,
  description: '',
  include_file_data: false,
  selected_items: []
})

const locations = ref<Array<{ id: string; name: string; areas: string[] }>>([])
const areas = ref<Array<{ id: string; name: string; location_id: string }>>([])
const commodities = ref<Array<{ id: string; name: string; area_id?: string; location_id: string }>>([])

// New state for hierarchical selection
const locationInclusions = ref<Map<string, boolean>>(new Map())
const areaInclusions = ref<Map<string, boolean>>(new Map())

const creating = ref(false)
const error = ref('')
const loadingItems = ref(false)

// Form error handling
const formError = ref<string | null>(null)
const formErrors = reactive({
  description: '',
  type: '',
  selected_items: '',
  include_file_data: ''
})

const canSubmit = computed(() => {
  if (!exportData.value.type || !exportData.value.description?.trim()) {
    return false
  }

  if (exportData.value.type === 'selected_items') {
    return exportData.value.selected_items && exportData.value.selected_items.length > 0
  }

  return true
})

const onTypeChange = () => {
  exportData.value.selected_items = []
  locationInclusions.value.clear()
  areaInclusions.value.clear()
  if (exportData.value.type === 'selected_items') {
    loadItems()
  }
}

const loadItems = async () => {
  loadingItems.value = true
  try {
    const [locationsResponse, areasResponse, commoditiesResponse] = await Promise.all([
      locationService.getLocations(),
      areaService.getAreas(),
      commodityService.getCommodities()
    ])

    if (locationsResponse.data && locationsResponse.data.data) {
      locations.value = locationsResponse.data.data.map((item: ResourceObject<Location>) => ({
        id: item.id,
        name: item.attributes.name,
        areas: item.attributes.areas || []
      }))
    }

    if (areasResponse.data && areasResponse.data.data) {
      areas.value = areasResponse.data.data.map((item: ResourceObject<Area>) => ({
        id: item.id,
        name: item.attributes.name,
        location_id: item.attributes.location_id
      }))
    }

    if (commoditiesResponse.data && commoditiesResponse.data.data) {
      commodities.value = commoditiesResponse.data.data.map((item: ResourceObject<Commodity>) => ({
        id: item.id,
        name: item.attributes.name,
        area_id: item.attributes.area_id,
        location_id: item.attributes.location_id
      }))
    }
  } catch (err) {
    console.error('Error loading items:', err)
  } finally {
    loadingItems.value = false
  }
}

// Selection helper functions
const isLocationSelected = (locationId: string): boolean => {
  return exportData.value.selected_items?.some(item => item.id === locationId && item.type === 'location') || false
}

const isAreaSelected = (areaId: string): boolean => {
  return exportData.value.selected_items?.some(item => item.id === areaId && item.type === 'area') || false
}

const isCommoditySelected = (commodityId: string): boolean => {
  return exportData.value.selected_items?.some(item => item.id === commodityId && item.type === 'commodity') || false
}

const getLocationInclusion = (locationId: string): boolean => {
  return locationInclusions.value.get(locationId) ?? true
}

const getAreaInclusion = (areaId: string): boolean => {
  return areaInclusions.value.get(areaId) ?? true
}

const setLocationInclusion = (locationId: string, include: boolean) => {
  locationInclusions.value.set(locationId, include)
}

const setAreaInclusion = (areaId: string, include: boolean) => {
  areaInclusions.value.set(areaId, include)
}

const getAreasForLocation = (locationId: string) => {
  return areas.value.filter(area => area.location_id === locationId)
}

const getCommoditiesForArea = (areaId: string) => {
  return commodities.value.filter(commodity => commodity.area_id === areaId)
}

const toggleLocation = (locationId: string, selected: boolean) => {
  if (!exportData.value.selected_items) {
    exportData.value.selected_items = []
  }

  if (selected) {
    // Add location if not already selected
    if (!isLocationSelected(locationId)) {
      exportData.value.selected_items.push({ id: locationId, type: 'location' })
      locationInclusions.value.set(locationId, true) // Default to include all
    }
  } else {
    // Remove location and all its children
    exportData.value.selected_items = exportData.value.selected_items.filter(item =>
      !(item.id === locationId && item.type === 'location')
    )
    locationInclusions.value.delete(locationId)

    // Also remove any areas and commodities in this location
    const areasInLocation = getAreasForLocation(locationId)
    areasInLocation.forEach(area => {
      exportData.value.selected_items = exportData.value.selected_items!.filter(item =>
        !(item.id === area.id && item.type === 'area')
      )
      areaInclusions.value.delete(area.id)

      // Remove commodities in this area
      const commoditiesInArea = getCommoditiesForArea(area.id)
      commoditiesInArea.forEach(commodity => {
        exportData.value.selected_items = exportData.value.selected_items!.filter(item =>
          !(item.id === commodity.id && item.type === 'commodity')
        )
      })
    })
  }
}

const toggleArea = (areaId: string, selected: boolean) => {
  if (!exportData.value.selected_items) {
    exportData.value.selected_items = []
  }

  if (selected) {
    // Add area if not already selected
    if (!isAreaSelected(areaId)) {
      exportData.value.selected_items.push({ id: areaId, type: 'area' })
      areaInclusions.value.set(areaId, true) // Default to include all
    }
  } else {
    // Remove area and all its commodities
    exportData.value.selected_items = exportData.value.selected_items.filter(item =>
      !(item.id === areaId && item.type === 'area')
    )
    areaInclusions.value.delete(areaId)

    // Also remove any commodities in this area
    const commoditiesInArea = getCommoditiesForArea(areaId)
    commoditiesInArea.forEach(commodity => {
      exportData.value.selected_items = exportData.value.selected_items!.filter(item =>
        !(item.id === commodity.id && item.type === 'commodity')
      )
    })
  }
}

const toggleCommodity = (commodityId: string, selected: boolean) => {
  if (!exportData.value.selected_items) {
    exportData.value.selected_items = []
  }

  if (selected) {
    // Add commodity if not already selected
    if (!isCommoditySelected(commodityId)) {
      exportData.value.selected_items.push({ id: commodityId, type: 'commodity' })
    }
  } else {
    // Remove commodity
    exportData.value.selected_items = exportData.value.selected_items.filter(item =>
      !(item.id === commodityId && item.type === 'commodity')
    )
  }
}

// Function to scroll to the first error in the form
const scrollToFirstError = () => {
  // Use nextTick to ensure the DOM has updated with error messages
  nextTick(() => {
    // Find the first element with an error message
    const firstErrorElement = document.querySelector('.error-message')
    if (firstErrorElement) {
      // Find the parent form group to scroll to
      const formGroup = firstErrorElement.closest('.form-group')
      if (formGroup) {
        // Scroll the form group into view with some padding at the top
        formGroup.scrollIntoView({ behavior: 'smooth', block: 'center' })
      }
    }
  })
}

// Form validation function
const validateForm = () => {
  let isValid = true

  // Reset errors
  Object.keys(formErrors).forEach(key => {
    formErrors[key] = ''
  })

  // Validate description
  if (!exportData.value.description?.trim()) {
    formErrors.description = 'Description is required'
    isValid = false
  }

  // Validate type
  if (!exportData.value.type) {
    formErrors.type = 'Export type is required'
    isValid = false
  }

  // Validate selected items if type is 'selected_items'
  if (exportData.value.type === 'selected_items') {
    if (!exportData.value.selected_items || exportData.value.selected_items.length === 0) {
      formErrors.selected_items = 'At least one item must be selected'
      isValid = false
    }
  }

  // If validation failed, scroll to the first error
  if (!isValid) {
    scrollToFirstError()
  }

  return isValid
}

const createExport = async () => {
  if (!canSubmit.value) return

  // Validate form first
  if (!validateForm()) {
    return
  }

  try {
    creating.value = true
    error.value = ''
    formError.value = null

    // Build selected items with include_all information
    const selectedItemsWithInclusion = (exportData.value.selected_items || []).map(item => {
      const enrichedItem = { ...item }

      // Add include_all flag for locations and areas
      if (item.type === 'location') {
        enrichedItem.include_all = getLocationInclusion(item.id)
      } else if (item.type === 'area') {
        enrichedItem.include_all = getAreaInclusion(item.id)
      }

      return enrichedItem
    })

    const requestData = {
      data: {
        type: 'exports',
        attributes: {
          type: exportData.value.type,
          description: exportData.value.description?.trim(),
          include_file_data: exportData.value.include_file_data,
          selected_items: selectedItemsWithInclusion,
          status: 'pending'
        }
      }
    }

    const response = await exportService.createExport(requestData)

    if (response.data && response.data.data) {
      router.push(`/exports/${response.data.data.id}`)
    } else {
      router.push('/exports')
    }
  } catch (err: any) {
    console.error('Error creating export:', err)

    if (err.response) {
      console.error('Response status:', err.response.status)
      console.error('Response data:', err.response.data)

      // Extract validation errors if present
      const apiErrors = err.response.data.errors?.[0]?.error?.error?.data?.attributes || {}

      // Map backend errors to form errors
      if (apiErrors && Object.keys(apiErrors).length > 0) {
        const unknownErrors = {}

        Object.keys(apiErrors).forEach(key => {
          // Convert snake_case to camelCase
          const camelKey = key.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase())
          if (formErrors.hasOwnProperty(camelKey)) {
            formErrors[camelKey] = apiErrors[key]
            delete(unknownErrors[key])
          } else {
            unknownErrors[key] = apiErrors[key]
          }
        })

        // If there are field errors, show a general message and scroll to first error
        if (Object.keys(unknownErrors).length === 0) {
          formError.value = 'Please correct the errors above (x).'
          scrollToFirstError()
        } else {
          formError.value = 'Please correct the errors above. Additional errors: ' + JSON.stringify(unknownErrors)
          scrollToFirstError()
        }
      } else {
        // No field-specific errors, show general error
        formError.value = `Failed to create export: ${err.response.status} - ${JSON.stringify(err.response.data)}`
      }
    } else {
      formError.value = 'Failed to create export: ' + (err.message || 'Unknown error')
    }
  } finally {
    creating.value = false
  }
}

onMounted(() => {
  // Pre-load items in case user selects "selected_items"
  loadItems()
})
</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;

.export-create {
  max-width: 800px;
  margin: 0 auto;
  padding: 20px;
}

h1 {
  margin: 0 0 30px;
  font-size: 2rem;
}

.export-form {
  background: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
  padding: 30px;
}

.form-section {
  margin-bottom: 30px;
}

.form-section h2 {
  margin: 0 0 20px;
  font-size: 1.5rem;
  color: $text-color;
  border-bottom: 2px solid $primary-color;
  padding-bottom: 10px;
}

.form-group {
  margin-bottom: 20px;
}

.form-group label {
  display: block;
  margin-bottom: 8px;
  font-weight: 600;
  color: $text-color;
}

.form-group input,
.form-group select,
.form-group textarea {
  width: 100%;
  padding: 10px;
  border: 1px solid $border-color;
  border-radius: $default-radius;
  font-size: 1rem;
}

.w-100 {
  width: 100%;
}

.form-group textarea {
  resize: vertical;
  min-height: 80px;
}

.form-help {
  font-size: 0.85rem;
  color: $text-secondary-color;
  margin-top: 5px;
}

.checkbox-label {
  display: flex !important;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.checkbox-label input[type="checkbox"] {
  width: auto;
  margin: 0;
}

.hierarchical-selector {
  border: 1px solid $border-color;
  border-radius: $default-radius;
  max-height: 400px;
  overflow-y: auto;
}

.selection-tree {
  padding: 15px;
}

.tree-item {
  margin-bottom: 10px;
}

.tree-item.location-item {
  border-left: 3px solid #2196f3;
  padding-left: 10px;
}

.tree-item.area-item {
  border-left: 3px solid #ff9800;
  padding-left: 10px;
  margin-left: 20px;
}

.tree-item.commodity-item {
  border-left: 3px solid #4caf50;
  padding-left: 10px;
  margin-left: 40px;
}

.item-header {
  margin-bottom: 8px;
}

.item-checkbox {
  display: flex !important;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  padding: 8px;
  border-radius: $default-radius;
  transition: background-color 0.2s;
}

.item-checkbox:hover {
  background-color: $light-bg-color;
}

.item-checkbox input[type="checkbox"] {
  width: auto;
  margin: 0;
}

.item-name {
  font-weight: 600;
  color: $text-color;
}

.item-type {
  font-size: 0.8rem;
  color: $text-secondary-color;
  text-transform: uppercase;
  background-color: $light-bg-color;
  padding: 2px 6px;
  border-radius: 3px;
  margin-left: auto;
}

.item-content {
  margin-top: 10px;
  padding-left: 20px;
}

.inclusion-toggle {
  margin-bottom: 15px;
  padding: 10px;
  background-color: $light-bg-color;
  border-radius: $default-radius;
}

.sub-items {
  margin-top: 10px;
}

/* Error handling styles */
.error-message {
  color: $error-color;
  font-size: 0.875rem;
  margin-top: 5px;
  display: block;
}

.form-error {
  background-color: #fee;
  border: 1px solid $error-color;
  border-radius: $default-radius;
  color: $error-color;
  padding: 15px;
  margin-top: 20px;
  font-weight: 500;
}

.is-invalid {
  border-color: $error-color !important;
  box-shadow: 0 0 0 0.2rem rgba(220, 53, 69, 0.25) !important;
}

/* PrimeVue Select error styling */
:deep(.p-select.is-invalid) {
  border-color: $error-color !important;

  .p-select-label {
    border-color: $error-color !important;
  }
}
</style>
