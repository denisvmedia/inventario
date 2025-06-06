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
          ></textarea>
          <div class="form-help">Describe what this export contains</div>
        </div>

        <div class="form-group">
          <label for="type">Export Type</label>
          <select
            id="type"
            v-model="exportData.type"
            required
            @change="onTypeChange"
          >
            <option value="">Select export type...</option>
            <option value="full_database">Full Database</option>
            <option value="locations">Locations Only</option>
            <option value="areas">Areas Only</option>
            <option value="commodities">Commodities Only</option>
            <option value="selected_items">Selected Items</option>
          </select>
          <div class="form-help">Choose what data to include in the export</div>
        </div>

        <div v-if="exportData.type === 'selected_items'" class="form-group">
          <label>Selected Items</label>
          <div class="selection-tabs">
            <button
              type="button"
              class="tab-button"
              :class="{ active: selectionType === 'locations' }"
              @click="selectionType = 'locations'"
            >
              Locations
            </button>
            <button
              type="button"
              class="tab-button"
              :class="{ active: selectionType === 'areas' }"
              @click="selectionType = 'areas'"
            >
              Areas
            </button>
            <button
              type="button"
              class="tab-button"
              :class="{ active: selectionType === 'commodities' }"
              @click="selectionType = 'commodities'"
            >
              Commodities
            </button>
          </div>

          <div class="selection-content">
            <div v-if="loadingItems" class="loading">Loading items...</div>
            <div v-else-if="selectionType === 'locations'" class="item-list">
              <div v-if="locations.length === 0" class="no-items">No locations available</div>
              <label v-for="location in locations" :key="location.id" class="item-checkbox">
                <input
                  type="checkbox"
                  v-model="exportData.selected_item_ids"
                  :value="location.id"
                />
                <span>{{ location.name }}</span>
              </label>
            </div>
            <div v-else-if="selectionType === 'areas'" class="item-list">
              <div v-if="areas.length === 0" class="no-items">No areas available</div>
              <label v-for="area in areas" :key="area.id" class="item-checkbox">
                <input
                  type="checkbox"
                  v-model="exportData.selected_item_ids"
                  :value="area.id"
                />
                <span>{{ area.name }}</span>
              </label>
            </div>
            <div v-else-if="selectionType === 'commodities'" class="item-list">
              <div v-if="commodities.length === 0" class="no-items">No commodities available</div>
              <label v-for="commodity in commodities" :key="commodity.id" class="item-checkbox">
                <input
                  type="checkbox"
                  v-model="exportData.selected_item_ids"
                  :value="commodity.id"
                />
                <span>{{ commodity.name }}</span>
              </label>
            </div>
          </div>
          <div class="form-help">
            Selected: {{ exportData.selected_item_ids.length }} item(s)
          </div>
        </div>

        <div class="form-group">
          <label class="checkbox-label">
            <input
              type="checkbox"
              v-model="exportData.include_file_data"
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
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import exportService from '@/services/exportService'
import locationService from '@/services/locationService'
import areaService from '@/services/areaService'
import commodityService from '@/services/commodityService'
import type { Export, ExportType, Location, Area, Commodity, ResourceObject } from '@/types'

const router = useRouter()

const exportData = ref<Partial<Export>>({
  type: '' as ExportType,
  description: '',
  include_file_data: false,
  selected_item_ids: []
})

const selectionType = ref<'locations' | 'areas' | 'commodities'>('locations')
const locations = ref<{ id: string; name: string }[]>([])
const areas = ref<{ id: string; name: string }[]>([])
const commodities = ref<{ id: string; name: string }[]>([])

const creating = ref(false)
const error = ref('')
const loadingItems = ref(false)

const canSubmit = computed(() => {
  if (!exportData.value.type || !exportData.value.description?.trim()) {
    return false
  }

  if (exportData.value.type === 'selected_items') {
    return exportData.value.selected_item_ids && exportData.value.selected_item_ids.length > 0
  }

  return true
})

const onTypeChange = () => {
  exportData.value.selected_item_ids = []
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
        name: item.attributes.name
      }))
    }

    if (areasResponse.data && areasResponse.data.data) {
      areas.value = areasResponse.data.data.map((item: ResourceObject<Area>) => ({
        id: item.id,
        name: item.attributes.name
      }))
    }

    if (commoditiesResponse.data && commoditiesResponse.data.data) {
      commodities.value = commoditiesResponse.data.data.map((item: ResourceObject<Commodity>) => ({
        id: item.id,
        name: item.attributes.name
      }))
    }
  } catch (err) {
    console.error('Error loading items:', err)
  } finally {
    loadingItems.value = false
  }
}

const createExport = async () => {
  if (!canSubmit.value) return

  try {
    creating.value = true
    error.value = ''

    const requestData = {
      data: {
        type: 'exports',
        attributes: {
          type: exportData.value.type,
          description: exportData.value.description?.trim(),
          include_file_data: exportData.value.include_file_data,
          selected_item_ids: exportData.value.selected_item_ids || [],
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
    error.value = err.response?.data?.errors?.[0]?.detail || 'Failed to create export'
    console.error('Error creating export:', err)
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
.export-create {
  padding: 20px;
  max-width: 800px;
  margin: 0 auto;
}

.breadcrumb-nav {
  margin-bottom: 20px;
}

.breadcrumb-link {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #6c757d;
  text-decoration: none;
  font-weight: 500;
  transition: color 0.2s;
}

.breadcrumb-link:hover {
  color: #495057;
}

h1 {
  margin: 0 0 30px;
  font-size: 2rem;
}

.error-message {
  background-color: #f8d7da;
  color: #721c24;
  padding: 12px;
  border-radius: 4px;
  margin-bottom: 20px;
}

.export-form {
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgb(0 0 0 / 10%);
  padding: 30px;
}

.form-section {
  margin-bottom: 30px;
}

.form-section h2 {
  margin: 0 0 20px;
  font-size: 1.5rem;
  color: #333;
  border-bottom: 2px solid #007bff;
  padding-bottom: 10px;
}

.form-group {
  margin-bottom: 20px;
}

.form-group label {
  display: block;
  margin-bottom: 8px;
  font-weight: 600;
  color: #333;
}

.form-group input,
.form-group select,
.form-group textarea {
  width: 100%;
  padding: 10px;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
}

.form-group textarea {
  resize: vertical;
  min-height: 80px;
}

.form-help {
  font-size: 0.85rem;
  color: #666;
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

.selection-tabs {
  display: flex;
  gap: 2px;
  margin-bottom: 15px;
  border-bottom: 1px solid #ddd;
}

.tab-button {
  padding: 10px 20px;
  background: none;
  border: none;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  font-weight: 500;
  color: #666;
  transition: all 0.2s;
}

.tab-button.active {
  color: #007bff;
  border-bottom-color: #007bff;
}

.tab-button:hover {
  background-color: #f8f9fa;
}

.selection-content {
  max-height: 300px;
  overflow-y: auto;
  border: 1px solid #ddd;
  border-radius: 4px;
  padding: 15px;
}

.loading {
  text-align: center;
  padding: 20px;
  color: #666;
}

.no-items {
  text-align: center;
  color: #666;
  padding: 20px;
}

.item-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.item-checkbox {
  display: flex !important;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  padding: 8px;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.item-checkbox:hover {
  background-color: #f8f9fa;
}

.item-checkbox input[type="checkbox"] {
  width: auto;
  margin: 0;
}

.form-actions {
  display: flex;
  gap: 15px;
  justify-content: flex-end;
  padding-top: 20px;
  border-top: 1px solid #eee;
}

.btn {
  padding: 10px 20px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 1rem;
  font-weight: 500;
  transition: background-color 0.2s;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-primary {
  background-color: #007bff;
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background-color: #0056b3;
}

.btn-secondary {
  background-color: #6c757d;
  color: white;
}

.btn-secondary:hover:not(:disabled) {
  background-color: #545b62;
}
</style>
