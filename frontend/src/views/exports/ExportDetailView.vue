<template>
  <div class="export-detail">
    <div class="breadcrumb-nav">
      <router-link to="/exports" class="breadcrumb-link">
        <font-awesome-icon icon="arrow-left" /> Back to Exports
      </router-link>
    </div>
    <div class="header">
      <h1>Export Details</h1>
      <div v-if="exportData" class="actions">
        <button
          v-if="exportData.status === 'completed'"
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
          class="btn btn-danger"
          :disabled="deleting"
          @click="deleteExport"
        >
          <font-awesome-icon :icon="deleting ? 'spinner' : 'trash'" :spin="deleting" />
          {{ deleting ? 'Deleting...' : 'Delete' }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="loading">Loading export details...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="exportData" class="export-content">

      <div class="export-card">
        <div class="card-header">
          <h2>Export Information</h2>
          <span class="status-badge" :class="`status-${exportData.status}`">
            {{ formatExportStatus(exportData.status) }}
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
              <div class="value">{{ formatDate(exportData.created_date) }}</div>
            </div>

            <div v-if="exportData.completed_date" class="info-item">
              <label>Completed</label>
              <div class="value">{{ formatDate(exportData.completed_date) }}</div>
            </div>

            <div v-if="exportData.file_path" class="info-item">
              <label>File Location</label>
              <div class="value file-path">{{ exportData.file_path }}</div>
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
          <div v-else class="selected-items">
            <div v-for="item in selectedItemsDetails" :key="`${item.type}-${item.id}`" class="item-details">
              <div class="item-name">{{ item.name }}</div>
              <div class="item-type">{{ item.type }}</div>
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

      <div class="export-card">
        <div class="card-header">
          <h2>Actions</h2>
        </div>
        <div class="card-body">
          <div class="action-buttons right-aligned">
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
              @click="deleteExport"
            >
              <font-awesome-icon :icon="deleting ? 'spinner' : 'trash'" :spin="deleting" />
              {{ deleting ? 'Deleting...' : 'Delete Export' }}
            </button>
          </div>
        </div>
      </div>

    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import exportService from '@/services/exportService'
import commodityService from '@/services/commodityService'
import locationService from '@/services/locationService'
import areaService from '@/services/areaService'
import type { Export } from '@/types'

const route = useRoute()
const router = useRouter()

const exportData = ref<Export | null>(null)
const loading = ref(true)
const error = ref('')
const retrying = ref(false)
const deleting = ref(false)
const downloading = ref(false)
const loadingItems = ref(false)
const selectedItemsDetails = ref<Array<{id: string, name: string, type: string}>>([])

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
      if (exportData.value.selected_items && exportData.value.selected_items.length > 0) {
        await loadSelectedItemsDetails(exportData.value.selected_items)
      }
    }
  } catch (err: any) {
    error.value = err.response?.data?.errors?.[0]?.detail || 'Failed to load export'
    console.error('Error loading export:', err)
  } finally {
    loading.value = false
  }
}

const loadSelectedItemsDetails = async (items: Array<{id: string, type: string}>) => {
  try {
    loadingItems.value = true
    selectedItemsDetails.value = []
    
    // Fetch details for each item based on its type
    const itemPromises = items.map(async (item) => {
      try {
        let response
        let itemType = ''
        
        switch (item.type) {
          case 'location':
            response = await locationService.getLocation(item.id)
            itemType = 'Location'
            break
          case 'area':
            response = await areaService.getArea(item.id)
            itemType = 'Area'
            break
          case 'commodity':
            response = await commodityService.getCommodity(item.id)
            itemType = 'Commodity'
            break
          default:
            return {
              id: item.id,
              name: `Unknown Item (${item.type})`,
              type: 'Unknown'
            }
        }
        
        if (response.data && response.data.data) {
          return {
            id: item.id,
            name: response.data.data.attributes.name,
            type: itemType
          }
        }
        
        return {
          id: item.id,
          name: 'Unknown Item',
          type: itemType || 'Unknown'
        }
      } catch (err) {
        console.error(`Error fetching details for ${item.type} ${item.id}:`, err)
        return {
          id: item.id,
          name: `${item.type} ${item.id}`,
          type: item.type.charAt(0).toUpperCase() + item.type.slice(1)
        }
      }
    })
    
    selectedItemsDetails.value = await Promise.all(itemPromises)
  } catch (err) {
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

const formatExportStatus = (status: string) => {
  const statusMap = {
    'pending': 'Pending',
    'in_progress': 'In Progress',
    'completed': 'Completed',
    'failed': 'Failed'
  }
  return statusMap[status as keyof typeof statusMap] || status
}

const formatDate = (dateString: string) => {
  if (!dateString) return '-'
  try {
    return new Date(dateString).toLocaleString()
  } catch {
    return dateString
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

const deleteExport = async () => {
  if (!exportData.value?.id) return

  if (!confirm('Are you sure you want to delete this export?')) {
    return
  }

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
  const interval = setInterval(() => {
    if (exportData.value?.status === 'pending' || exportData.value?.status === 'in_progress') {
      loadExport()
    } else {
      clearInterval(interval)
    }
  }, 5000)

  // Cleanup interval on component unmount
  return () => clearInterval(interval)
})
</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;

.export-detail {
  max-width: 1000px;
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

.header .actions {
  display: flex;
  gap: 10px;
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

.item-name {
  font-weight: 600;
  font-size: 1rem;
  color: $text-color;
  margin-bottom: 4px;
}

.item-type {
  font-size: 0.875rem;
  color: $text-secondary-color;
  text-transform: uppercase;
  letter-spacing: 0.5px;
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

.action-buttons {
  display: flex;
  gap: 15px;
}

.action-buttons.right-aligned {
  justify-content: flex-end;
}

.btn-warning {
  background-color: #ffc107;
  color: #212529;
}

.btn-warning:hover:not(:disabled) {
  background-color: #e0a800;
}
</style>
