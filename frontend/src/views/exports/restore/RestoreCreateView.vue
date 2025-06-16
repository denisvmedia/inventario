<template>
  <div class="export-detail-page">
    <div class="breadcrumb-nav">
      <router-link :to="`/exports/${exportId}`" class="breadcrumb-link">
        <font-awesome-icon icon="arrow-left" /> Back to Export Details
      </router-link>
    </div>
    <div class="header">
      <h1>Restore from Export</h1>
    </div>
    <div v-if="exportData" class="export-card">
      <div class="card-header">
        <h2>Export Information</h2>
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
          <div v-if="exportData.file_size" class="info-item">
            <label>File Size</label>
            <div class="value">{{ formatFileSize(exportData.file_size) }}</div>
          </div>
          <div v-if="exportData.file_path" class="info-item">
            <label>File Location</label>
            <div class="value file-path">{{ exportData.file_path }}</div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="loading" class="loading">
      <font-awesome-icon icon="spinner" spin /> Loading export details...
    </div>

    <div v-else-if="error" class="error-message-block">
      {{ error }}
      <button class="btn btn-secondary" @click="loadExport">
        <font-awesome-icon icon="refresh" /> Retry
      </button>
    </div>

    <form v-else class="restore-form" @submit.prevent="createRestore">
      <!-- Restore Description -->
      <div class="form-section">
        <div class="card-header">
          <h2>Restore Description</h2>
        </div>
        <div class="card-body">
          <div class="form-group">
            <label for="description">Description</label>
            <textarea
              id="description"
              v-model="form.description"
              placeholder="Enter a description for this restore operation..."
              rows="3"
              maxlength="500"
              required
              :class="{ 'is-invalid': formErrors.description }"
            ></textarea>
            <div v-if="formErrors.description" class="error-message">{{ formErrors.description }}</div>
            <div class="form-help">Describe what this restore operation will accomplish</div>
          </div>
        </div>
      </div>

      <!-- Restore Strategy -->
      <div class="form-section">
        <div class="card-header">
          <h2>Restore Strategy</h2>
        </div>
        <div class="card-body">
          <div class="strategy-options">
            <div class="strategy-option" :class="{ selected: form.options.strategy === 'merge_add' }">
              <RadioButton
                v-model="form.options.strategy"
                inputId="strategy-merge-add"
                value="merge_add"
              />
              <label for="strategy-merge-add" class="strategy-label">
                <strong>Merge Add</strong>
                <span class="strategy-description">
                  Only add data from backup that is missing in current database
                </span>
              </label>
            </div>

            <div class="strategy-option" :class="{ selected: form.options.strategy === 'merge_update' }">
              <RadioButton
                v-model="form.options.strategy"
                inputId="strategy-merge-update"
                value="merge_update"
              />
              <label for="strategy-merge-update" class="strategy-label">
                <strong>Merge Update</strong>
                <span class="strategy-description">
                  Create if missing, update if exists, leave other records untouched
                </span>
              </label>
            </div>

            <div class="strategy-option" :class="{ selected: form.options.strategy === 'full_replace' }">
              <RadioButton
                v-model="form.options.strategy"
                inputId="strategy-full-replace"
                value="full_replace"
              />
              <label for="strategy-full-replace" class="strategy-label">
                <strong>Full Replace</strong>
                <span class="strategy-description">
                  Clear all existing data and restore everything from backup
                </span>
              </label>
            </div>
          </div>
          <div v-if="formErrors.strategy" class="error-message">{{ formErrors.strategy }}</div>
        </div>
      </div>

      <!-- Options -->
      <div class="form-section">
        <div class="card-header">
          <h2>Options</h2>
        </div>
        <div class="card-body">
          <div class="form-group">
            <label class="checkbox-label">
              <Checkbox
                v-model="form.options.include_file_data"
                inputId="include-file-data"
                :binary="true"
              />
              <span>Include file data (images, invoices, manuals)</span>
            </label>
            <div class="form-help">
              When enabled, restores binary file data along with database records
            </div>
          </div>

          <div class="form-group">
            <label class="checkbox-label">
              <Checkbox
                v-model="form.options.dry_run"
                inputId="dry-run"
                :binary="true"
              />
              <span>Dry run (preview changes without applying them)</span>
            </label>
            <div class="form-help">
              When enabled, shows what would be restored without making actual changes
            </div>
          </div>
        </div>
      </div>

      <!-- Form Actions -->
      <div class="form-actions">
        <router-link :to="`/exports/${exportId}`" class="btn btn-secondary">
          Cancel
        </router-link>
        <button
          type="submit"
          class="btn btn-restore"
          :disabled="!canSubmit || creating"
        >
          <font-awesome-icon :icon="creating ? 'spinner' : 'upload'" :spin="creating" />
          {{ creating ? 'Starting Restore...' : (form.options.dry_run ? 'Preview Restore' : 'Start Restore') }}
        </button>
      </div>
    </form>

    <!-- Form Error Display -->
    <div v-if="formError" class="form-error">
      <h3>Validation Errors:</h3>
      <ul>
        <li v-for="(error, field) in formError" :key="field">
          <strong>{{ field }}:</strong> {{ error }}
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import RadioButton from 'primevue/radiobutton'
import Checkbox from 'primevue/checkbox'
import { useToast } from 'primevue/usetoast'
import exportService from '@/services/exportService'
import type { Export, RestoreRequest, RestoreOptions } from '@/types'

const route = useRoute()
const router = useRouter()
const toast = useToast()

const exportId = route.params.id as string
const exportData = ref<Export | null>(null)
const loading = ref(true)
const error = ref('')
const creating = ref(false)
const formError = ref<Record<string, string> | null>(null)

const form = ref<RestoreRequest>({
  description: '',
  source_file_path: '', // Will be set from export data
  options: {
    strategy: 'merge_add',
    include_file_data: true,
    dry_run: false,
  } as RestoreOptions,
})

const formErrors = ref<Record<string, string>>({})

const canSubmit = computed(() => {
  return exportData.value &&
         form.value.description.trim() &&
         exportData.value.file_path &&
         !creating.value
})

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

const formatDate = (dateString?: string) => {
  if (!dateString) return '-'
  try {
    return new Date(dateString).toLocaleString()
  } catch {
    return dateString
  }
}

const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const loadExport = async () => {
  try {
    loading.value = true
    error.value = ''
    const response = await exportService.getExport(exportId)

    if (response.data && response.data.data) {
      exportData.value = {
        id: response.data.data.id,
        ...response.data.data.attributes
      }

      // Set the source file path from export data
      form.value.source_file_path = exportData.value.file_path || ''

      // Set default description
      if (!form.value.description) {
        form.value.description = `Restore from "${exportData.value.description}"`
      }
    }
  } catch (err: any) {
    error.value = err.response?.data?.errors?.[0]?.detail || 'Failed to load export'
    console.error('Error loading export:', err)
  } finally {
    loading.value = false
  }
}

const validateForm = (): boolean => {
  formErrors.value = {}

  if (!form.value.description.trim()) {
    formErrors.value.description = 'Description is required'
  }

  if (!form.value.options.strategy) {
    formErrors.value.strategy = 'Restore strategy is required'
  }

  return Object.keys(formErrors.value).length === 0
}

const scrollToFirstError = () => {
  const firstErrorElement = document.querySelector('.error-message')
  if (firstErrorElement) {
    firstErrorElement.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }
}

const createRestore = async () => {
  if (!validateForm()) {
    scrollToFirstError()
    return
  }

  try {
    creating.value = true
    error.value = ''
    formError.value = null

    // Create restore operation for this export
    const requestData = {
      data: {
        type: 'restores',
        attributes: {
          description: form.value.description,
          options: form.value.options
        }
      }
    }

    const response = await exportService.createRestore(exportId, requestData)
    const restore = response.data.data.attributes

    toast.add({
      severity: 'success',
      summary: 'Restore Started',
      detail: `Restore operation "${form.value.description}" has been started and is running in the background`,
      life: 5000
    })

    // Start polling for restore status updates
    exportService.pollRestoreStatus(
      exportId,
      restore.id,
      (updatedRestore) => {
        console.log('Restore status update:', updatedRestore.status)
        // You could show progress updates here if needed
      }
    ).then((finalRestore) => {
      // Show completion notification
      if (finalRestore.status === 'completed') {
        toast.add({
          severity: 'success',
          summary: 'Restore Completed',
          detail: `Restore operation "${form.value.description}" completed successfully`,
          life: 8000
        })
      } else if (finalRestore.status === 'failed') {
        toast.add({
          severity: 'error',
          summary: 'Restore Failed',
          detail: finalRestore.error_message || 'Restore operation failed',
          life: 10000
        })
      }
    }).catch((error) => {
      console.error('Error polling restore status:', error)
      toast.add({
        severity: 'warn',
        summary: 'Restore Monitoring Lost',
        detail: 'Lost connection to restore status updates. Check the export details page for current status.',
        life: 8000
      })
    })

    // Navigate back to export detail view immediately
    router.push(`/exports/${exportId}`)
  } catch (err: any) {
    console.error('Error creating restore:', err)

    if (err.response?.data?.errors) {
      // Handle validation errors from API
      const apiErrors = err.response.data.errors
      const errorObj: Record<string, string> = {}

      apiErrors.forEach((error: any) => {
        if (error.source?.pointer) {
          const field = error.source.pointer.replace('/data/attributes/', '')
          errorObj[field] = error.detail
        }
      })

      if (Object.keys(errorObj).length > 0) {
        formError.value = errorObj
        scrollToFirstError()
        return
      }
    }

    // Extract user-friendly error message
    const apiError = err.response?.data?.errors?.[0]
    if (apiError?.error?.msg) {
      // Use the detailed error message from the API
      error.value = apiError.error.msg
    } else if (apiError?.detail) {
      // Use the detail field if available
      error.value = apiError.detail
    } else {
      // Fallback to generic message
      error.value = 'Failed to create restore operation'
    }
  } finally {
    creating.value = false
  }
}

onMounted(() => {
  loadExport()
})
</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;
@use '@/assets/export-detail-styles' as *;

// Removed .restore-create container style, now using .export-detail-page for unified layout

h1 {
  margin-bottom: 1.5rem;
  color: $text-color;
}

// Removed .export-info-card, .export-summary, .summary-item styles as they are now unified
// in export-detail-styles.scss

.form-section {
  background: white;
  border: 1px solid $border-color;
  border-radius: $default-radius;
  margin-bottom: 1.5rem;
  box-shadow: $box-shadow;
}

.form-group {
  margin-bottom: 1.5rem;

  &:last-child {
    margin-bottom: 0;
  }
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 600;
  color: $text-color;
}

.form-group textarea {
  resize: vertical;
  min-height: 80px;
  font-family: inherit;
}

.form-group input,
.form-group select,
.form-group textarea {
  width: 100%;
  padding: 10px;
  border: 1px solid $border-color;
  border-radius: $default-radius;
  font-size: 1rem;
  transition: border-color 0.2s ease;

  &:focus {
    outline: none;
    border-color: $primary-color;
    box-shadow: 0 0 0 2px rgba($primary-color, 0.2);
  }

  &.is-invalid {
    border-color: $error-color;
  }
}

.form-help {
  font-size: 0.85rem;
  color: $text-secondary-color;
  margin-top: 0.5rem;
  line-height: 1.4;
}

.strategy-options {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.strategy-option {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  padding: 1rem;
  border: 2px solid $border-color;
  border-radius: $default-radius;
  cursor: pointer;
  transition: all 0.2s ease;

  &:hover {
    border-color: $primary-color;
    background-color: rgba($primary-color, 0.05);
  }

  &.selected {
    border-color: $primary-color;
    background-color: rgba($primary-color, 0.1);
  }
}

.strategy-label {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  cursor: pointer;
  flex: 1;

  strong {
    color: $text-color;
    font-size: 1rem;
  }
}

.strategy-description {
  color: $text-secondary-color;
  font-size: 0.875rem;
  line-height: 1.4;
}

// Checkbox styles are now imported globally from assets/primevue-checkbox.scss

.form-actions {
  display: flex;
  gap: 1rem;
  justify-content: flex-end;
  margin-top: 2rem;
  padding-top: 1rem;
  border-top: 1px solid $border-color;
}

.loading {
  text-align: center;
  padding: 2rem;
  color: $text-secondary-color;
}

.form-error {
  margin-top: 1rem;
  padding: 1rem;
  background-color: rgba($error-color, 0.1);
  border: 1px solid $error-color;
  border-radius: $default-radius;

  h3 {
    margin: 0 0 0.5rem;
    color: $error-color;
  }

  ul {
    margin: 0;
    padding-left: 1.5rem;

    li {
      color: $error-color;
      margin-bottom: 0.25rem;
    }
  }
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: $default-radius;
  font-size: 1rem;
  font-weight: 500;
  text-decoration: none;
  cursor: pointer;
  transition: all 0.2s ease;

  &:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
}

.btn-secondary {
  background-color: $secondary-color;
  color: white;
  border: 1px solid $secondary-color;

  &:hover:not(:disabled) {
    background-color: $secondary-hover-color;
    border-color: $secondary-hover-color;
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

.error-message {
  color: $error-color;
  font-size: 0.875rem;
  margin-top: 0.25rem;
}
</style>
