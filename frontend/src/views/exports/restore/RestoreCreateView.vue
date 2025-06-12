<template>
  <div class="restore-create">
    <div class="breadcrumb-nav">
      <router-link :to="`/exports/${exportId}`" class="breadcrumb-link">
        <font-awesome-icon icon="arrow-left" /> Back to Export Details
      </router-link>
    </div>

    <div class="header">
      <h1>Restore from Export</h1>
      <div v-if="exportData" class="export-info">
        <span class="export-description">{{ exportData.description }}</span>
        <span class="export-type">{{ formatExportType(exportData.type) }}</span>
      </div>
    </div>

    <div v-if="loading" class="loading">
      <font-awesome-icon icon="spinner" spin /> Loading export details...
    </div>

    <div v-else-if="error" class="error-message">
      {{ error }}
      <button @click="loadExport" class="btn btn-secondary">
        <font-awesome-icon icon="refresh" /> Retry
      </button>
    </div>

    <form v-else @submit.prevent="createRestore" class="restore-form">
      <!-- Step 1: Export Information -->
      <div class="form-section">
        <h2>1. Export Information</h2>
        <div class="export-details">
          <div class="detail-item">
            <label>Export File:</label>
            <span>{{ exportData?.file_path || 'Not available' }}</span>
          </div>
          <div class="detail-item">
            <label>Created:</label>
            <span>{{ formatDate(exportData?.created_date) }}</span>
          </div>
          <div class="detail-item">
            <label>File Size:</label>
            <span>{{ formatFileSize(exportData?.file_size || 0) }}</span>
          </div>
        </div>
      </div>

      <!-- Step 2: Restore Description -->
      <div class="form-section">
        <h2>2. Restore Description</h2>
        <div class="form-group">
          <label for="description">Description</label>
          <input
            id="description"
            v-model="form.description"
            type="text"
            placeholder="Enter a description for this restore operation"
            required
          />
          <div v-if="formErrors.description" class="error-message">{{ formErrors.description }}</div>
          <div class="form-help">Describe what this restore operation will accomplish</div>
        </div>
      </div>

      <!-- Step 3: Restore Strategy -->
      <div class="form-section">
        <h2>3. Restore Strategy</h2>
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

      <!-- Step 4: Options -->
      <div class="form-section">
        <h2>4. Options</h2>
        
        <div class="form-group">
          <div class="checkbox-group">
            <Checkbox
              v-model="form.options.include_file_data"
              inputId="include-file-data"
              :binary="true"
            />
            <label for="include-file-data">Include file data (images, invoices, manuals)</label>
          </div>
          <div class="form-help">
            When enabled, restores binary file data along with database records
          </div>
        </div>

        <div class="form-group">
          <div class="checkbox-group">
            <Checkbox
              v-model="form.options.dry_run"
              inputId="dry-run"
              :binary="true"
            />
            <label for="dry-run">Dry run (preview changes without applying them)</label>
          </div>
          <div class="form-help">
            When enabled, shows what would be restored without making actual changes
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
          class="btn btn-primary"
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
    backup_existing: false,
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

    const restore = await exportService.createRestore(exportId, requestData)

    toast.add({
      severity: 'success',
      summary: 'Restore Started',
      detail: `Restore operation "${form.value.description}" has been started`,
      life: 5000
    })

    // Navigate back to export detail view
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
    
    error.value = err.response?.data?.errors?.[0]?.detail || 'Failed to create restore operation'
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

.restore-create {
  max-width: 800px;
  margin: 0 auto;
}

.breadcrumb-nav {
  margin-bottom: 1rem;
}

.breadcrumb-link {
  color: $primary-color;
  text-decoration: none;
  font-size: 0.9rem;
  
  &:hover {
    text-decoration: underline;
  }
}

.export-info {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  margin-top: 0.5rem;
  
  .export-description {
    font-size: 1rem;
    color: $text-color;
  }
  
  .export-type {
    font-size: 0.875rem;
    color: $text-secondary-color;
    text-transform: uppercase;
  }
}

.export-details {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 1rem;
  padding: 1rem;
  background: $light-bg-color;
  border-radius: $default-radius;
  border: 1px solid $border-color;
}

.detail-item {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  
  label {
    font-weight: 600;
    color: $text-secondary-color;
    font-size: 0.875rem;
  }
  
  span {
    color: $text-color;
  }
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

.checkbox-group {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  
  label {
    cursor: pointer;
    color: $text-color;
  }
}

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
</style>
