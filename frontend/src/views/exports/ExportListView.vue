<template>
  <div class="export-list">
    <div class="header">
      <div class="header-title">
        <h1>Exports</h1>
        <div v-if="!loading && exports.length > 0" class="export-count">
          {{ exports.length }} export{{ exports.length !== 1 ? 's' : '' }}
        </div>
      </div>
      <div class="header-actions">
        <div class="filter-toggle">
          <ToggleSwitch v-model="showDeleted" @change="loadExports" />
          <label class="toggle-label">Show deleted exports</label>
        </div>
        <router-link to="/exports/new" class="btn btn-primary">
          <font-awesome-icon icon="plus" /> New
        </router-link>
      </div>
    </div>

    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="exports.length === 0" class="empty">
      <div class="empty-message">
        <p>No exports found. Create your first export!</p>
        <div class="action-button">
          <router-link to="/exports/new" class="btn btn-primary">Create Export</router-link>
        </div>
      </div>
    </div>

    <div v-else class="exports-table">
      <table class="table">
        <thead>
          <tr>
            <th>Description</th>
            <th>Type</th>
            <th>Status</th>
            <th>Created</th>
            <th>Completed</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="exportItem in exports" :key="exportItem.id"
              :class="['export-row', { 'deleted': isExportDeleted(exportItem) }]"
              @click="viewExport(exportItem.id!)">
            <td class="export-description">
              <div class="description-text">{{ exportItem.description || 'No description' }}</div>
              <div v-if="exportItem.error_message" class="error-message">
                Error: {{ exportItem.error_message }}
              </div>
            </td>
            <td class="export-type">
              <span class="type-badge" :class="`type-${exportItem.type}`">
                {{ formatExportType(exportItem.type) }}
              </span>
            </td>
            <td class="export-status">
              <span class="status-badge" :class="getExportStatusClasses(exportItem)">
                {{ getExportDisplayStatus(exportItem) }}
              </span>
            </td>
            <td class="export-date">
              {{ formatDate(exportItem.created_date) }}
            </td>
            <td class="export-date">
              {{ exportItem.completed_date ? formatDate(exportItem.completed_date) : '-' }}
            </td>
            <td class="export-actions">
              <router-link :to="`/exports/${exportItem.id}`" class="btn btn-sm btn-secondary" @click.stop>
                <font-awesome-icon icon="eye" /> View
              </router-link>
              <button
                v-if="exportItem.status === 'completed' && canPerformOperations(exportItem)"
                class="btn btn-sm btn-primary"
                :disabled="downloading === exportItem.id"
                @click.stop="downloadExport(exportItem.id!)"
              >
                <font-awesome-icon :icon="downloading === exportItem.id ? 'spinner' : 'download'" :spin="downloading === exportItem.id" />
                {{ downloading === exportItem.id ? 'Downloading...' : 'Download' }}
              </button>
              <button
                v-if="canPerformOperations(exportItem)"
                class="btn btn-sm btn-danger"
                :disabled="deleting === exportItem.id"
                @click.stop="deleteExport(exportItem.id!)"
              >
                <font-awesome-icon icon="trash" />
                {{ deleting === exportItem.id ? 'Deleting...' : 'Delete' }}
              </button>
              <span v-else-if="isExportDeleted(exportItem)" class="deleted-indicator">
                <font-awesome-icon icon="trash" /> Deleted
              </span>
            </td>
          </tr>
        </tbody>
      </table>
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
      @cancel="onCancelDelete"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import Confirmation from '@/components/Confirmation.vue'
import exportService from '@/services/exportService'
import { isExportDeleted, canPerformOperations, getExportDisplayStatus, getExportStatusClasses } from '@/utils/exportUtils'
import type { Export, ResourceObject } from '@/types'

const router = useRouter()

const exports = ref<Export[]>([])
const loading = ref(true)
const error = ref('')
const deleting = ref<string | null>(null)
const downloading = ref<string | null>(null)
const showDeleteDialog = ref(false)
const exportToDelete = ref<string | null>(null)
const showDeleted = ref(false)

const loadExports = async () => {
  try {
    loading.value = true
    error.value = ''
    const response = await exportService.getExports(showDeleted.value)
    if (response.data && response.data.data) {
      const exportList = response.data.data.map((item: ResourceObject<Export>) => ({
        id: item.id,
        ...item.attributes
      }))

      exports.value = exportList
    }
  } catch (err: any) {
    error.value = err.response?.data?.errors?.[0]?.detail || 'Failed to load exports'
    console.error('Error loading exports:', err)
  } finally {
    loading.value = false
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

const formatDate = (dateString: string) => {
  if (!dateString) return '-'
  try {
    return new Date(dateString).toLocaleString()
  } catch {
    return dateString
  }
}

const viewExport = (exportId: string) => {
  router.push(`/exports/${exportId}`)
}

const deleteExport = (exportId: string) => {
  exportToDelete.value = exportId
  showDeleteDialog.value = true
}

const onConfirmDelete = async () => {
  if (!exportToDelete.value) return

  try {
    deleting.value = exportToDelete.value
    await exportService.deleteExport(exportToDelete.value)
    // Reload exports to reflect the soft delete
    await loadExports()
  } catch (err: any) {
    console.error('Error deleting export:', err)
    error.value = 'Failed to delete export: ' + (err.message || 'Unknown error')
  } finally {
    deleting.value = null
    exportToDelete.value = null
    showDeleteDialog.value = false
  }
}

const onCancelDelete = () => {
  exportToDelete.value = null
  showDeleteDialog.value = false
}

const downloadExport = async (exportId: string) => {
  try {
    downloading.value = exportId
    const response = await exportService.downloadExport(exportId)

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
    error.value = 'Failed to download export: ' + (err.message || 'Unknown error')
  } finally {
    downloading.value = null
  }
}

onMounted(() => {
  loadExports()
})
</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;

.export-list {
  max-width: $container-max-width;
  margin: 0 auto;
  padding: 20px;
}

// Header styles are now in shared _header.scss

.empty-message p {
  color: $text-secondary-color;
  margin-bottom: 20px;
}

.exports-table {
  background: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
  overflow: hidden;
}

.table {
  width: 100%;
  border-collapse: collapse;
}

.table th,
.table td {
  padding: 12px;
  text-align: left;
  border-bottom: 1px solid $border-color;
}

.table th {
  background-color: $light-bg-color;
  font-weight: 600;
  color: $text-color;
}

.export-row {
  cursor: pointer;
  transition: background-color 0.2s ease;

  &:hover {
    background-color: $light-bg-color;
  }
}

.export-description .description-text {
  font-weight: 500;
}

.export-description .error-message {
  color: $error-color;
  font-size: 0.8rem;
  margin-top: 4px;
}

.type-badge,
.status-badge {
  padding: 4px 8px;
  border-radius: $default-radius;
  font-size: 0.8rem;
  font-weight: 500;
  text-transform: uppercase;
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

.export-actions {
  display: flex;
  gap: 8px;
  white-space: nowrap;
}

.btn-sm {
  padding: 4px 8px;
  font-size: 0.75rem;
}

.deleted-indicator {
  color: #6c757d;
  font-size: 0.75rem;
  font-style: italic;
}

.export-row.deleted {
  opacity: 0.6;
  background-color: #f8f9fa;
}
</style>
