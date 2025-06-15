<template>
  <div class="export-detail-page" :class="{ 'deleted': exportData && isExportDeleted(exportData) }">
    <div class="breadcrumb-nav">
      <router-link to="/exports" class="breadcrumb-link">
        <font-awesome-icon icon="arrow-left" /> Back to Exports
      </router-link>
    </div>
    <div class="header">
      <h1>Export Details</h1>
      <div v-if="exportData" class="actions">
        <button
          v-if="exportData.status === 'completed' && canPerformOperations(exportData)"
          class="btn btn-primary"
          :disabled="downloading"
          @click="downloadExport"
        >
          <font-awesome-icon :icon="downloading ? 'spinner' : 'download'" :spin="downloading" />
          {{ downloading ? 'Downloading...' : 'Download' }}
        </button>

        <button
          v-if="exportData.status === 'failed'"
          class="btn btn-warning"
          :disabled="retrying"
          @click="retryExport"
        >
          <font-awesome-icon :icon="retrying ? 'spinner' : 'redo'" :spin="retrying" />
          {{ retrying ? 'Retrying...' : 'Retry' }}
        </button>

        <button
          v-if="canPerformOperations(exportData)"
          class="btn btn-danger"
          :disabled="deleting"
          @click="confirmDelete"
        >
          <font-awesome-icon :icon="deleting ? 'spinner' : 'trash'" :spin="deleting" />
          {{ deleting ? 'Deleting...' : 'Delete' }}
        </button>
        <div v-else-if="isExportDeleted(exportData)" class="deleted-status">
          <font-awesome-icon icon="trash" /> This export has been deleted
        </div>
      </div>
    </div>

    <div v-if="loading" class="loading">Loading export details...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="exportData" class="export-content">

      <div class="export-card">
        <div class="card-header">
          <h2>Export Information</h2>
          <span class="status-badge" :class="getExportStatusClasses(exportData)">
            {{ getExportDisplayStatus(exportData) }}
          </span>
        </div>

        <div class="card-body">
          <div class="info-grid">
            <div class="info-item">
              <label>Description</label>
              <div class="value">{{ exportData.description || 'No description' }}</div>
            </div>

            <div class="info-item">
              <label>Type</label>
              <div class="value">
                <span class="type-badge" :class="`type-${exportData.type}`">
                  {{ formatExportType(exportData.type) }}
                </span>
              </div>
            </div>

            <div class="info-item">
              <label>Include File Data</label>
              <div class="value">
                <span class="bool-badge" :class="exportData.include_file_data ? 'yes' : 'no'">
                  {{ exportData.include_file_data ? 'Yes' : 'No' }}
                </span>
              </div>
            </div>

            <div class="info-item">
              <label>Created</label>
              <div class="value">{{ formatDateTime(exportData.created_date) }}</div>
            </div>

            <div v-if="exportData.completed_date" class="info-item">
              <label>Completed</label>
              <div class="value">{{ formatDateTime(exportData.completed_date) }}</div>
            </div>

            <div v-if="exportData.deleted_at" class="info-item">
              <label>Deleted</label>
              <div class="value deleted-date">{{ formatDateTime(exportData.deleted_at) }}</div>
            </div>

            <div v-if="exportData.file_path" class="info-item">
              <label>File Location</label>
              <div class="value file-path">{{ exportData.file_path }}</div>
            </div>

            <div v-if="exportData.file_size" class="info-item">
              <label>File Size</label>
              <div class="value">{{ formatFileSize(exportData.file_size) }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- Export Statistics -->
      <div v-if="exportData.status === 'completed' && hasStatistics" class="export-card">
        <div class="card-header">
          <h2>Export Statistics</h2>
        </div>
        <div class="card-body">
          <div class="info-grid">
            <div v-if="exportData.location_count !== undefined" class="info-item">
              <label>Locations</label>
              <div class="value">{{ exportData.location_count.toLocaleString() }}</div>
            </div>

            <div v-if="exportData.area_count !== undefined" class="info-item">
              <label>Areas</label>
              <div class="value">{{ exportData.area_count.toLocaleString() }}</div>
            </div>

            <div v-if="exportData.commodity_count !== undefined" class="info-item">
              <label>Commodities</label>
              <div class="value">{{ exportData.commodity_count.toLocaleString() }}</div>
            </div>

            <div v-if="exportData.image_count !== undefined" class="info-item">
              <label>Images</label>
              <div class="value">{{ exportData.image_count.toLocaleString() }}</div>
            </div>

            <div v-if="exportData.invoice_count !== undefined" class="info-item">
              <label>Invoices</label>
              <div class="value">{{ exportData.invoice_count.toLocaleString() }}</div>
            </div>

            <div v-if="exportData.manual_count !== undefined" class="info-item">
              <label>Manuals</label>
              <div class="value">{{ exportData.manual_count.toLocaleString() }}</div>
            </div>

            <div v-if="exportData.binary_data_size !== undefined && exportData.binary_data_size > 0" class="info-item">
              <label>Binary Data Size</label>
              <div class="value">{{ formatFileSize(exportData.binary_data_size) }}</div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="exportData.selected_items && exportData.selected_items.length > 0" class="export-card">
        <div class="card-header">
          <h2>Selected Items</h2>
          <span class="count-badge">{{ exportData.selected_items.length }} items</span>
        </div>
        <div class="card-body">
          <div v-if="loadingItems" class="loading-items">Loading item details...</div>
          <div v-else class="selected-items-hierarchy">
            <div v-for="location in hierarchicalItems.locations" :key="location.id" class="hierarchy-item location-item">
              <div class="item-header">
                <div class="item-info">
                  <span class="item-name">{{ location.name }}</span>
                  <span class="item-type">Location</span>
                </div>
                <div v-if="location.includeAll" class="inclusion-badge">
                  includes all areas and commodities
                </div>
              </div>

              <div v-if="location.areas.length > 0" class="sub-items">
                <div v-for="area in location.areas" :key="area.id" class="hierarchy-item area-item">
                  <div class="item-header">
                    <div class="item-info">
                      <span class="item-name">{{ area.name }}</span>
                      <span class="item-type">Area</span>
                    </div>
                    <div v-if="area.includeAll" class="inclusion-badge">
                      includes all commodities
                    </div>
                  </div>

                  <div v-if="area.commodities.length > 0" class="sub-items">
                    <div v-for="commodity in area.commodities" :key="commodity.id" class="hierarchy-item commodity-item">
                      <div class="item-header">
                        <div class="item-info">
                          <span class="item-name">{{ commodity.name }}</span>
                          <span class="item-type">Commodity</span>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Standalone areas (not under any selected location) -->
            <div v-for="area in hierarchicalItems.standaloneAreas" :key="area.id" class="hierarchy-item area-item">
              <div class="item-header">
                <div class="item-info">
                  <span class="item-name">{{ area.name }}</span>
                  <span class="item-type">Area</span>
                </div>
                <div v-if="area.includeAll" class="inclusion-badge">
                  includes all commodities
                </div>
              </div>

              <div v-if="area.commodities.length > 0" class="sub-items">
                <div v-for="commodity in area.commodities" :key="commodity.id" class="hierarchy-item commodity-item">
                  <div class="item-header">
                    <div class="item-info">
                      <span class="item-name">{{ commodity.name }}</span>
                      <span class="item-type">Commodity</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Standalone commodities (not under any selected area) -->
            <div v-for="commodity in hierarchicalItems.standaloneCommodities" :key="commodity.id" class="hierarchy-item commodity-item">
              <div class="item-header">
                <div class="item-info">
                  <span class="item-name">{{ commodity.name }}</span>
                  <span class="item-type">Commodity</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="exportData.error_message" class="export-card error-card">
        <div class="card-header">
          <h2>Error Details</h2>
        </div>
        <div class="card-body">
          <div class="error-message">{{ exportData.error_message }}</div>
        </div>
      </div>

      <!-- Restore Operations -->
      <div v-if="restoreOperations.length > 0" class="export-card">
        <div class="card-header">
          <h2>Restore Operations</h2>
          <span class="count-badge">{{ restoreOperations.length }} operation{{ restoreOperations.length !== 1 ? 's' : '' }}</span>
        </div>
        <div class="card-body">
          <div class="restore-operations">
            <div v-for="(restore, index) in restoreOperations" :key="restore.id" class="restore-operation">
              <div class="restore-header" @click="toggleRestoreOperation(index)">
                <div class="restore-info">
                  <div class="restore-description">{{ restore.description }}</div>
                  <div class="restore-meta">
                    <span class="restore-date">{{ formatDateTime(restore.created_date) }}</span>
                    <span class="restore-strategy">{{ formatRestoreStrategy(restore.options?.strategy) }}</span>
                  </div>
                </div>
                <div class="restore-status">
                  <span class="status-badge" :class="getRestoreStatusClasses(restore)">
                    <font-awesome-icon
                      v-if="restore.status === 'running' || restore.status === 'pending'"
                      icon="spinner"
                      spin
                      class="status-icon"
                    />
                    {{ getRestoreDisplayStatus(restore) }}
                  </span>
                  <button class="collapse-toggle" :class="{ 'expanded': expandedRestoreOperations[index] }">
                    <font-awesome-icon :icon="expandedRestoreOperations[index] ? 'chevron-up' : 'chevron-down'" />
                  </button>
                </div>
              </div>

              <div v-if="expandedRestoreOperations[index] && restore.steps && restore.steps.length > 0" class="restore-steps">
                <div class="steps-header">
                  <h4>Restore Steps</h4>
                  <span class="steps-count">{{ restore.steps.length }} steps</span>
                </div>
                <div class="steps-list">
                  <div v-for="step in restore.steps" :key="step.id" class="restore-step">
                    <div class="step-icon">
                      <span class="step-emoji">{{ getStepEmoji(step.result) }}</span>
                    </div>
                    <div class="step-content">
                      <div class="step-name">{{ step.name }}</div>
                      <div class="step-details">
                        <span v-if="step.duration" class="step-duration">{{ formatDuration(step.duration) }}</span>
                        <span class="step-result" :class="`result-${step.result}`">{{ formatStepResult(step.result) }}</span>
                      </div>
                      <div v-if="step.reason" class="step-reason">{{ step.reason }}</div>
                    </div>
                  </div>
                </div>
              </div>

              <div v-if="expandedRestoreOperations[index] && restore.error_message" class="restore-error">
                <div class="error-message">{{ restore.error_message }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="export-card">
        <div class="card-header">
          <h2>Actions</h2>
        </div>
        <div class="card-body">
          <div class="actions right-aligned">
            <button
              v-if="exportData.status === 'completed' && canPerformOperations(exportData)"
              class="btn btn-restore"
              @click="navigateToRestore"
            >
              <font-awesome-icon icon="upload" />
              Restore from Export
            </button>

            <button
              v-if="exportData.status === 'completed'"
              class="btn btn-primary"
              :disabled="downloading"
              @click="downloadExport"
            >
              <font-awesome-icon :icon="downloading ? 'spinner' : 'download'" :spin="downloading" />
              {{ downloading ? 'Downloading...' : 'Download Export' }}
            </button>

            <button
              v-if="exportData.status === 'failed'"
              class="btn btn-warning"
              :disabled="retrying"
              @click="retryExport"
            >
              <font-awesome-icon :icon="retrying ? 'spinner' : 'redo'" :spin="retrying" />
              {{ retrying ? 'Retrying...' : 'Retry Export' }}
            </button>

            <button
              class="btn btn-danger"
              :disabled="deleting"
              @click="confirmDelete"
            >
              <font-awesome-icon :icon="deleting ? 'spinner' : 'trash'" :spin="deleting" />
              {{ deleting ? 'Deleting...' : 'Delete Export' }}
            </button>
          </div>
        </div>
      </div>

    </div>

    <!-- Export Delete Confirmation Dialog -->
    <Confirmation
      v-model:visible="showDeleteDialog"
      title="Confirm Delete"
      message="Are you sure you want to delete this export?"
      confirm-label="Delete"
      cancel-label="Cancel"
      confirm-button-class="danger"
      confirmationIcon="exclamation-triangle"
      @confirm="onConfirmDelete"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import exportService from '@/services/exportService'
import { isExportDeleted, canPerformOperations, getExportDisplayStatus, getExportStatusClasses } from '@/utils/exportUtils'
import type { Export } from '@/types'
import Confirmation from '@/components/Confirmation.vue'

const route = useRoute()
const router = useRouter()

const exportData = ref<Export | null>(null)
const loading = ref(true)
const error = ref('')
const retrying = ref(false)
const deleting = ref(false)
const downloading = ref(false)
const showDeleteDialog = ref(false)
const loadingItems = ref(false)
const selectedItemsDetails = ref<Array<{id: string, name: string, type: string}>>([])
const restoreOperations = ref<Array<any>>([])
const expandedRestoreOperations = ref<Record<number, boolean>>({})
const hierarchicalItems = ref<{
  locations: Array<{
    id: string
    name: string
    includeAll: boolean
    areas: Array<{
      id: string
      name: string
      includeAll: boolean
      commodities: Array<{id: string, name: string}>
    }>
  }>
  standaloneAreas: Array<{
    id: string
    name: string
    includeAll: boolean
    commodities: Array<{id: string, name: string}>
  }>
  standaloneCommodities: Array<{id: string, name: string}>
}>({
  locations: [],
  standaloneAreas: [],
  standaloneCommodities: []
})

// Computed property to check if export has statistics
const hasStatistics = computed(() => {
  if (!exportData.value) return false
  return exportData.value.location_count !== undefined ||
         exportData.value.area_count !== undefined ||
         exportData.value.commodity_count !== undefined ||
         exportData.value.image_count !== undefined ||
         exportData.value.invoice_count !== undefined ||
         exportData.value.manual_count !== undefined ||
         exportData.value.binary_data_size !== undefined
})

const loadExport = async () => {
  try {
    loading.value = true
    error.value = ''
    const exportId = route.params.id as string
    const response = await exportService.getExport(exportId)

    if (response.data && response.data.data) {
      exportData.value = {
        id: response.data.data.id,
        ...response.data.data.attributes
      }

      // Load selected items details if available
      if (exportData.value?.selected_items && exportData.value.selected_items.length > 0) {
        await loadSelectedItemsDetails(exportData.value.selected_items)
      }

      // Load restore operations for this export
      await loadRestoreOperations()
    }
  } catch (err: any) {
    error.value = err.response?.data?.errors?.[0]?.detail || 'Failed to load export'
    console.error('Error loading export:', err)
    throw err
  } finally {
    loading.value = false
  }
}

const loadRestoreOperations = async () => {
  try {
    if (!exportData.value?.id) return

    const response = await exportService.getRestoreOperations(exportData.value.id)
    if (response.data && response.data.data) {
      restoreOperations.value = response.data.data.map((item: any) => ({
        id: item.id,
        ...item.attributes
      }))
    } else {
      restoreOperations.value = []
    }
  } catch (err: any) {
    console.error('Error loading restore operations:', err)
    // Don't fail the whole page if restore operations can't be loaded
    restoreOperations.value = []
  }
}

const loadSelectedItemsDetails = async (items: Array<{id: string, type: string, name?: string, include_all?: boolean, location_id?: string, area_id?: string}>) => {
  try {
    loadingItems.value = true
    selectedItemsDetails.value = []
    hierarchicalItems.value = {
      locations: [],
      standaloneAreas: [],
      standaloneCommodities: []
    }

    // Create lookup maps for selected items with their include_all flag and names
    const locationItems = new Map()
    const areaItems = new Map()
    const commodityItems = new Map()

    items.forEach(item => {
      const itemData = {
        name: item.name || `[Unknown ${item.type} ${item.id}]`,
        includeAll: item.include_all || false,
        locationId: item.location_id || null,
        areaId: item.area_id || null
      }

      if (item.type === 'location') {
        locationItems.set(item.id, itemData)
      } else if (item.type === 'area') {
        areaItems.set(item.id, itemData)
      } else if (item.type === 'commodity') {
        commodityItems.set(item.id, itemData)
      }
    })

    // Build hierarchy using stored relationship data instead of fetching from database
    const processedAreaIds = new Set()

    // Process selected locations
    for (const [locationId, locationData] of locationItems) {
      let locationAreasData = []

      if (!locationData.includeAll) {
        // Find areas that belong to this location and are explicitly selected
        for (const [areaId, areaData] of areaItems) {
          if (areaData.locationId === locationId) {
            processedAreaIds.add(areaId)

            let areaCommoditiesData = []

            if (!areaData.includeAll) {
              // Find commodities that belong to this area and are explicitly selected
              for (const [commodityId, commodityData] of commodityItems) {
                if (commodityData.areaId === areaId) {
                  areaCommoditiesData.push({
                    id: commodityId,
                    name: commodityData.name
                  })
                }
              }
            }

            locationAreasData.push({
              id: areaId,
              name: areaData.name,
              includeAll: areaData.includeAll,
              commodities: areaCommoditiesData
            })
          }
        }
      }

      hierarchicalItems.value.locations.push({
        id: locationId,
        name: locationData.name,
        includeAll: locationData.includeAll,
        areas: locationAreasData
      })
    }

    // Process standalone areas (not under selected locations)
    for (const [areaId, areaData] of areaItems) {
      if (processedAreaIds.has(areaId)) continue

      // Check if parent location is selected - if so, skip this area (it's already included under location)
      const parentLocationSelected = areaData.locationId && locationItems.has(areaData.locationId)
      if (parentLocationSelected) continue

      let areaCommoditiesData = []

      if (!areaData.includeAll) {
        // Find commodities that belong to this area and are explicitly selected
        for (const [commodityId, commodityData] of commodityItems) {
          if (commodityData.areaId === areaId) {
            areaCommoditiesData.push({
              id: commodityId,
              name: commodityData.name
            })
          }
        }
      }

      hierarchicalItems.value.standaloneAreas.push({
        id: areaId,
        name: areaData.name,
        includeAll: areaData.includeAll,
        commodities: areaCommoditiesData
      })
    }

    // Process standalone commodities (not under selected areas)
    for (const [commodityId, commodityData] of commodityItems) {
      // Check if parent area is selected - if so, skip this commodity (it's already included under area)
      const parentAreaSelected = commodityData.areaId && areaItems.has(commodityData.areaId)
      if (parentAreaSelected) continue

      // Also check if parent location is selected and includes all
      let parentLocationIncludesAll = false
      if (commodityData.areaId) {
        // Find the area to get its location
        for (const [, areaData] of areaItems) {
          if (areaData.locationId && locationItems.has(areaData.locationId)) {
            const locationData = locationItems.get(areaData.locationId)
            if (locationData.includeAll) {
              parentLocationIncludesAll = true
              break
            }
          }
        }
      }
      if (parentLocationIncludesAll) continue

      hierarchicalItems.value.standaloneCommodities.push({
        id: commodityId,
        name: commodityData.name
      })
    }

  } catch (err: any) {
    console.error('Error loading selected items details:', err)
  } finally {
    loadingItems.value = false
  }
}

const formatExportType = (type: string) => {
  const typeMap = {
    'full_database': 'Full Database',
    'selected_items': 'Selected Items',
    'locations': 'Locations',
    'areas': 'Areas',
    'commodities': 'Commodities'
  }
  return typeMap[type as keyof typeof typeMap] || type
}

const formatDateTime = (dateString: string) => {
  if (!dateString) return '-'
  try {
    return new Date(dateString).toLocaleString()
  } catch {
    return dateString
  }
}

const formatFileSize = (bytes: number) => {
  if (bytes === 0) return '0 Bytes'

  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const formatRestoreStrategy = (strategy: string) => {
  const strategyMap = {
    'merge_add': 'Merge Add',
    'merge_update': 'Merge Update',
    'full_replace': 'Full Replace'
  }
  return strategyMap[strategy as keyof typeof strategyMap] || strategy
}

const getRestoreStatusClasses = (restore: any) => {
  const status = restore.status || 'pending'
  return `status-${status.replace('_', '-')}`
}

const getRestoreDisplayStatus = (restore: any) => {
  const statusMap = {
    'pending': 'Pending',
    'running': 'Running',
    'completed': 'Completed',
    'failed': 'Failed'
  }
  return statusMap[restore.status as keyof typeof statusMap] || restore.status
}

const getStepEmoji = (result: string) => {
  const emojiMap = {
    'todo': '📝',
    'in_progress': '🔄',
    'success': '✅',
    'error': '❌',
    'skipped': '⏭️'
  }
  return emojiMap[result as keyof typeof emojiMap] || '📝'
}

const formatStepResult = (result: string) => {
  const resultMap = {
    'todo': 'To Do',
    'in_progress': 'In Progress',
    'success': 'Success',
    'error': 'Error',
    'skipped': 'Skipped'
  }
  return resultMap[result as keyof typeof resultMap] || result
}

const formatDuration = (duration: number) => {
  if (duration < 1000) {
    return `${duration}ms`
  } else if (duration < 60000) {
    return `${(duration / 1000).toFixed(1)}s`
  } else {
    return `${(duration / 60000).toFixed(1)}m`
  }
}

const toggleRestoreOperation = (index: number) => {
  expandedRestoreOperations.value[index] = !expandedRestoreOperations.value[index]
}

const navigateToRestore = () => {
  if (exportData.value?.id) {
    router.push(`/exports/${exportData.value.id}/restore`)
  }
}

const retryExport = async () => {
  if (!exportData.value?.id) return

  try {
    retrying.value = true

    // Update export status to pending to retry
    const requestData = {
      data: {
        type: 'exports',
        attributes: {
          ...exportData.value,
          status: 'pending',
          error_message: '',
          completed_date: null,
          file_path: ''
        }
      }
    }

    await exportService.updateExport(exportData.value.id, requestData)
    await loadExport() // Reload to show updated status
  } catch (err: any) {
    console.error('Error retrying export:', err)
    alert('Failed to retry export')
  } finally {
    retrying.value = false
  }
}

const confirmDelete = () => {
  showDeleteDialog.value = true
}

const onConfirmDelete = () => {
  deleteExport()
  showDeleteDialog.value = false
}

const deleteExport = async () => {
  if (!exportData.value?.id) return

  try {
    deleting.value = true
    await exportService.deleteExport(exportData.value.id)
    router.push('/exports')
  } catch (err: any) {
    console.error('Error deleting export:', err)
    alert('Failed to delete export')
  } finally {
    deleting.value = false
  }
}

const downloadExport = async () => {
  if (!exportData.value?.id) return

  try {
    downloading.value = true
    const response = await exportService.downloadExport(exportData.value.id)

    // Create blob and download link
    const blob = new Blob([response.data], { type: 'application/xml' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url

    // Try to get filename from Content-Disposition header
    const contentDisposition = response.headers['content-disposition']
    let filename = 'export.xml'
    if (contentDisposition) {
      const filenameMatch = contentDisposition.match(/filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/)
      if (filenameMatch) {
        filename = filenameMatch[1].replace(/['"]/g, '')
      }
    }

    link.download = filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)
  } catch (err: any) {
    console.error('Error downloading export:', err)
    alert('Failed to download export')
  } finally {
    downloading.value = false
  }
}

onMounted(() => {
  loadExport()

  // Auto-refresh if export is in progress
  const exportInterval = setInterval(() => {
    if (exportData.value?.status === 'pending' || exportData.value?.status === 'in_progress') {
      loadExport().catch(err => {
        console.error('Error refreshing export:', err)
        clearInterval(exportInterval)
      })
    } else {
      clearInterval(exportInterval)
    }
  }, 5000)

  // Auto-refresh restore operations that are in progress
  const restoreInterval = setInterval(() => {
    if (exportData.value?.restore_operations) {
      const runningRestores = exportData.value.restore_operations.filter(
        restore => restore.status === 'pending' || restore.status === 'running'
      )

      if (runningRestores.length > 0) {
        // Refresh the entire export to get updated restore operations
        loadExport().catch(err => {
          console.error('Error refreshing restore operations:', err)
        })
      }
    }
  }, 3000) // Check restore operations more frequently

  // Cleanup intervals on component unmount
  return () => {
    clearInterval(exportInterval)
    clearInterval(restoreInterval)
  }
})
</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;

.export-detail-page {
  max-width: 800px;
  margin: 0 auto;
  padding: 20px;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.header h1 {
  margin: 0;
  font-size: 2rem;
}

.export-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.export-card {
  background: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
  overflow: hidden;
}

.error-card {
  border-left: 4px solid $error-color;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px;
  background-color: $light-bg-color;
  border-bottom: 1px solid $border-color;
}

.card-header h2 {
  margin: 0;
  font-size: 1.25rem;
}

.card-body {
  padding: 20px;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 20px;
}

.info-item label {
  display: block;
  font-weight: 600;
  color: $text-secondary-color;
  margin-bottom: 5px;
  text-transform: uppercase;
  font-size: 0.8rem;
  letter-spacing: 0.5px;
}

.info-item .value {
  font-size: 1rem;
  color: $text-color;
}

.file-path {
  word-break: break-all;
  word-wrap: break-word;
  overflow-wrap: break-word;
}

.status-badge,
.type-badge,
.bool-badge,
.count-badge {
  padding: 4px 8px;
  border-radius: $default-radius;

  .status-icon {
    margin-right: 4px;
  }

  font-size: 0.8rem;
  font-weight: 500;
  text-transform: uppercase;
}

.status-pending {
  background-color: #fff3cd;
  color: #856404;
}

.status-in_progress {
  background-color: #d4edda;
  color: #155724;
}

.status-completed {
  background-color: #d1ecf1;
  color: #0c5460;
}

.status-failed {
  background-color: #f8d7da;
  color: #721c24;
}

.export-status--deleted {
  background-color: #f5f5f5;
  color: #6c757d;
  text-decoration: line-through;
}

.type-full_database {
  background-color: #e3f2fd;
  color: #1976d2;
}

.type-selected_items {
  background-color: #f3e5f5;
  color: #7b1fa2;
}

.type-locations {
  background-color: #e8f5e8;
  color: #388e3c;
}

.type-areas {
  background-color: #fff3e0;
  color: #f57c00;
}

.type-commodities {
  background-color: #fce4ec;
  color: #c2185b;
}

.type-imported {
  background-color: #f0f4f8;
  color: #4a5568;
}

.bool-badge.yes {
  background-color: #d4edda;
  color: #155724;
}

.bool-badge.no {
  background-color: #f8d7da;
  color: #721c24;
}

.count-badge {
  background-color: #e9ecef;
  color: #495057;
}

.selected-items-hierarchy {
  display: flex;
  flex-direction: column;
  gap: 15px;
}

.hierarchy-item {
  border-left: 3px solid transparent;
  padding-left: 15px;
  position: relative;

  .item-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 15px;
    background-color: rgb(255 255 255 / 70%);
    border-radius: $default-radius;
    margin-bottom: 10px;
  }

  .item-info {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .item-name {
    font-weight: 600;
    font-size: 1rem;
    color: $text-color;
  }

  .item-type {
    font-size: 0.875rem;
    color: $text-secondary-color;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .sub-items {
    margin-top: 5px;
    padding-left: 0;
  }

  &.location-item {
    border-left-color: #1976d2;
    background-color: #f8fffe;
  }

  &.area-item {
    border-left-color: #f57c00;
    background-color: #fffef8;
    margin-left: 15px;
  }

  &.commodity-item {
    border-left-color: #c2185b;
    background-color: #fefff8;
    margin-left: 30px;
  }
}

.inclusion-badge {
  background-color: #e8f5e8;
  color: #2e7d32;
  padding: 4px 8px;
  border-radius: $default-radius;
  font-size: 0.75rem;
  font-weight: 500;
  font-style: italic;
}

.selected-items {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.item-details {
  background-color: $light-bg-color;
  padding: 12px;
  border-radius: $default-radius;
  border: 1px solid #dee2e6;
}

.loading-items {
  text-align: center;
  padding: 20px;
  color: $text-secondary-color;
  font-style: italic;
}


.error-message {
  background-color: #f8d7da;
  color: #721c24;
  padding: 15px;
  border-radius: $default-radius;
  font-family: monospace;
  white-space: pre-wrap;
}

.btn-warning {
  background-color: #ffc107;
  color: #212529;
}

.btn-warning:hover:not(:disabled) {
  background-color: #e0a800;
}

.deleted-status {
  color: #6c757d;
  font-style: italic;
  display: flex;
  align-items: center;
  gap: 8px;
}

.deleted-date {
  color: #dc3545;
  font-weight: 600;
}

.export-detail.deleted {
  opacity: 0.8;
}

.export-detail.deleted .export-card {
  background-color: #f8f9fa;
  border-left: 4px solid #6c757d;
}

// Restore operations styles
.restore-operations {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.restore-operation {
  border: 1px solid $border-color;
  border-radius: $default-radius;
  padding: 1rem;
  background-color: #fafafa;
}

.restore-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 1rem;
  cursor: pointer;
  padding: 0.5rem;
  border-radius: $default-radius;
  transition: background-color 0.2s ease;

  &:hover {
    background-color: rgba($primary-color, 0.05);
  }
}

.restore-info {
  flex: 1;
}

.restore-status {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.collapse-toggle {
  background: none;
  border: none;
  color: $text-secondary-color;
  cursor: pointer;
  padding: 0.25rem;
  border-radius: $default-radius;
  transition: all 0.2s ease;

  &:hover {
    background-color: rgba($primary-color, 0.1);
    color: $primary-color;
  }

  &.expanded {
    transform: rotate(180deg);
  }
}

.restore-description {
  font-weight: 600;
  font-size: 1rem;
  color: $text-color;
  margin-bottom: 0.5rem;
}

.restore-meta {
  display: flex;
  gap: 1rem;
  font-size: 0.875rem;
  color: $text-secondary-color;
}

.restore-strategy {
  text-transform: uppercase;
  font-weight: 500;
}

.restore-steps {
  margin-top: 1rem;
}

.steps-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.75rem;

  h4 {
    margin: 0;
    font-size: 0.9rem;
    color: $text-secondary-color;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
}

.steps-count {
  font-size: 0.8rem;
  color: $text-secondary-color;
  background-color: #e9ecef;
  padding: 2px 6px;
  border-radius: $default-radius;
}

.steps-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.restore-step {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  padding: 0.5rem;
  background-color: white;
  border-radius: $default-radius;
  border: 1px solid #e9ecef;
}

.step-icon {
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.step-emoji {
  font-size: 1rem;
}

.step-content {
  flex: 1;
  min-width: 0;
}

.step-name {
  font-weight: 500;
  color: $text-color;
  margin-bottom: 0.25rem;
}

.step-details {
  display: flex;
  gap: 1rem;
  font-size: 0.8rem;
  margin-bottom: 0.25rem;
}

.step-duration {
  color: $text-secondary-color;
}

.step-result {
  font-weight: 500;
  text-transform: uppercase;

  &.result-success {
    color: #28a745;
  }

  &.result-error {
    color: #dc3545;
  }

  &.result-in-progress {
    color: #007bff;
  }

  &.result-skipped {
    color: #6c757d;
  }

  &.result-todo {
    color: #ffc107;
  }
}

.step-reason {
  font-size: 0.8rem;
  color: $text-secondary-color;
  font-style: italic;
}

.restore-error {
  margin-top: 1rem;
  padding: 0.75rem;
  background-color: #f8d7da;
  border: 1px solid #f5c6cb;
  border-radius: $default-radius;

  .error-message {
    color: #721c24;
    font-size: 0.875rem;
    margin: 0;
  }
}

.btn-success {
  background-color: #28a745;
  color: white;
  border: 1px solid #28a745;

  &:hover:not(:disabled) {
    background-color: #218838;
    border-color: #1e7e34;
  }
}

.btn-restore {
  background-color: #1976d2;
  color: white;
  border: 1px solid #1976d2;

  &:hover:not(:disabled) {
    background-color: #1565c0;
    border-color: #1565c0;
  }
}
</style>
