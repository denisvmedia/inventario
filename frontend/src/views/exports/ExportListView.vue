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
          <tr v-for="exportItem in exports" :key="exportItem.id" class="export-row">
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
              <span class="status-badge" :class="`status-${exportItem.status}`">
                {{ formatExportStatus(exportItem.status) }}
              </span>
            </td>
            <td class="export-date">
              {{ formatDate(exportItem.created_date) }}
            </td>
            <td class="export-date">
              {{ exportItem.completed_date ? formatDate(exportItem.completed_date) : '-' }}
            </td>
            <td class="export-actions">
              <router-link :to="`/exports/${exportItem.id}`" class="btn btn-sm btn-secondary">
                <font-awesome-icon icon="eye" /> View
              </router-link>
              <button
                v-if="exportItem.status === 'completed'"
                class="btn btn-sm btn-primary"
                :disabled="downloading === exportItem.id"
                @click="downloadExport(exportItem.id!)"
              >
                <font-awesome-icon :icon="downloading === exportItem.id ? 'spinner' : 'download'" :spin="downloading === exportItem.id" />
                {{ downloading === exportItem.id ? 'Downloading...' : 'Download' }}
              </button>
              <button
                class="btn btn-sm btn-danger"
                :disabled="deleting === exportItem.id"
                @click="deleteExport(exportItem.id!)"
              >
                <font-awesome-icon icon="trash" />
                {{ deleting === exportItem.id ? 'Deleting...' : 'Delete' }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import exportService from '@/services/exportService'
import type { Export, ResourceObject } from '@/types'

const exports = ref<Export[]>([])
const loading = ref(true)
const error = ref('')
const deleting = ref<string | null>(null)
const downloading = ref<string | null>(null)

const loadExports = async () => {
  try {
    loading.value = true
    error.value = ''
    const response = await exportService.getExports()
    if (response.data && response.data.data) {
      exports.value = response.data.data.map((item: ResourceObject<Export>) => ({
        id: item.id,
        ...item.attributes
      }))
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

const deleteExport = async (exportId: string) => {
  if (!confirm('Are you sure you want to delete this export?')) {
    return
  }

  try {
    deleting.value = exportId
    await exportService.deleteExport(exportId)
    exports.value = exports.value.filter(exp => exp.id !== exportId)
  } catch (err: any) {
    console.error('Error deleting export:', err)
    alert('Failed to delete export')
  } finally {
    deleting.value = null
  }
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
    alert('Failed to download export')
  } finally {
    downloading.value = null
  }
}

onMounted(() => {
  loadExports()
})
</script>

<style lang="scss" scoped>
.export-list {
  padding: 20px;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

.header-title h1 {
  margin: 0 0 5px;
  font-size: 2rem;
}

.export-count {
  color: #666;
  font-size: 0.9rem;
}

.header-actions {
  display: flex;
  gap: 10px;
  align-items: center;
}

.loading, .error, .empty {
  text-align: center;
  padding: 40px 20px;
}

.error {
  color: #dc3545;
}

.empty-message p {
  color: #666;
  margin-bottom: 20px;
}

.exports-table {
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgb(0 0 0 / 10%);
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
  border-bottom: 1px solid #eee;
}

.table th {
  background-color: #f8f9fa;
  font-weight: 600;
  color: #333;
}

.export-row:hover {
  background-color: #f8f9fa;
}

.export-description .description-text {
  font-weight: 500;
}

.export-description .error-message {
  color: #dc3545;
  font-size: 0.8rem;
  margin-top: 4px;
}

.type-badge,
.status-badge {
  padding: 4px 8px;
  border-radius: 4px;
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

.export-actions {
  display: flex;
  gap: 8px;
  white-space: nowrap;
}

.btn {
  padding: 6px 12px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 0.8rem;
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

.btn-danger {
  background-color: #dc3545;
  color: white;
}

.btn-danger:hover:not(:disabled) {
  background-color: #c82333;
}

.btn-sm {
  padding: 4px 8px;
  font-size: 0.75rem;
}
</style>
