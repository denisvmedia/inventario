<template>
  <div class="file-edit">
    <!-- Loading State -->
    <div v-if="loading" class="loading">Loading...</div>

    <!-- Error State -->
    <div v-else-if="error" class="error">{{ error }}</div>

    <!-- Edit Form -->
    <div v-else-if="file">
      <!-- Header with Back Link -->
      <div class="breadcrumb-nav">
        <a href="#" class="breadcrumb-link" @click.prevent="goBack">
          <FontAwesomeIcon icon="arrow-left" />
          {{ backLinkText }}
        </a>
      </div>

      <div class="header">
        <h1>Edit File</h1>
      </div>

      <!-- File Preview Card -->
      <div class="info-card file-preview-card">
        <div class="file-preview">
          <img
            v-if="file.type === 'image'"
            :src="getFileUrl(file)"
            :alt="file.title"
            class="preview-image"
            @error="handleImageError"
          />
          <div v-else class="file-icon">
            <font-awesome-icon :icon="getFileIcon(file)" size="2x" />
          </div>
        </div>

        <div class="file-info">
          <h3>{{ file.original_path }}</h3>
          <div class="file-meta">
            <span class="file-type">{{ getFileTypeLabel(file.type) }}</span>
            <span class="file-ext">{{ file.ext }}</span>
          </div>

          <!-- Show current link if exists -->
          <div v-if="file && isLinked(file)" class="current-link-info">
            <a
              v-if="getLinkedEntityUrl(file)"
              :href="getLinkedEntityUrl(file)"
              class="info-badge"
              title="View linked entity"
            >
              <FontAwesomeIcon icon="link" />
              Currently linked: {{ getLinkedEntityDisplay(file) }}
              <FontAwesomeIcon icon="external-link-alt" class="link-nav-icon" />
              View
            </a>
          </div>
        </div>
      </div>



      <!-- Edit Form Card -->
      <form class="form" @submit.prevent="updateFile">
        <!-- 1. Filename and Extension (editable) -->
        <div class="form-group">
          <label for="path" class="required">Filename</label>
          <div class="filename-input-group" :class="{ 'is-invalid': errors.path }">
            <input
              id="path"
              v-model="form.path"
              type="text"
              class="form-control filename-input"
              placeholder="Enter filename (without extension)"
              required
            />
            <span class="file-extension">{{ file.ext }}</span>
          </div>
          <div v-if="errors.path" class="error-message">{{ errors.path }}</div>
          <div class="form-help">This will be the filename when downloaded (extension will be added automatically)</div>
        </div>

        <!-- 2. All Editable Fields -->
        <div class="form-group">
          <label for="title">Title</label>
          <input
            id="title"
            v-model="form.title"
            type="text"
            class="form-control"
            :class="{ 'is-invalid': errors.title }"
            placeholder="Enter a title for this file (optional)"
          />
          <div v-if="errors.title" class="error-message">{{ errors.title }}</div>
          <div class="form-help">If left empty, the filename will be used as the title</div>
        </div>

        <div class="form-group">
          <label for="description">Description</label>
          <textarea
            id="description"
            v-model="form.description"
            class="form-control"
            :class="{ 'is-invalid': errors.description }"
            placeholder="Optional description"
            rows="3"
          ></textarea>
          <div v-if="errors.description" class="error-message">{{ errors.description }}</div>
        </div>

        <div class="form-group">
          <label for="tags">Tags</label>
          <input
            id="tags"
            v-model="tagsInput"
            type="text"
            class="form-control"
            placeholder="Enter tags separated by commas"
            @input="updateTags"
          />
          <div class="form-help">Separate multiple tags with commas</div>

          <div v-if="form.tags.length > 0" class="tags-preview">
            <span v-for="tag in form.tags" :key="tag" class="tag">
              {{ tag }}
              <button type="button" class="tag-remove" @click="removeTag(tag)">×</button>
            </span>
          </div>
        </div>

        <!-- Entity Linking Section -->
        <div class="form-section">
          <h3>Entity Linking</h3>
          <div class="form-help">Link this file to a commodity or export for better organization</div>

          <div class="form-group">
            <label for="linked_entity_type">Link Type</label>
            <Select
              id="linked_entity_type"
              v-model="form.linked_entity_type"
              :options="entityTypeOptions"
              option-label="label"
              option-value="value"
              placeholder="Choose entity type"
              :disabled="isExportFile"
              @change="onEntityTypeChange"
            />
            <div class="form-help">
              <span v-if="isExportFile">Export files cannot be manually relinked</span>
              <span v-else>Choose what type of entity to link this file to</span>
            </div>
          </div>

          <div v-if="form.linked_entity_type === 'commodity'" class="form-group">
            <label for="linked_entity_id">Commodity</label>
            <Select
              id="linked_entity_id"
              v-model="form.linked_entity_id"
              :options="commodityOptions"
              option-label="label"
              option-value="value"
              option-group-label="label"
              option-group-children="items"
              :placeholder="loadingCommodities ? 'Loading commodities...' : 'Select commodity'"
              filter
              :loading="loadingCommodities"
              :disabled="isExportFile || loadingCommodities"
              @show="loadCommodities"
            />
            <div class="form-help">
              <span v-if="isExportFile">Export file entity ID cannot be changed</span>
              <span v-else-if="loadingCommodities">Loading commodities...</span>
              <span v-else>Choose the commodity to link this file to</span>
            </div>
          </div>

          <div v-if="form.linked_entity_type === 'export'" class="form-group">
            <label for="linked_entity_id">Export ID</label>
            <input
              id="linked_entity_id"
              v-model="form.linked_entity_id"
              type="text"
              class="form-control"
              disabled
              readonly
            />
            <div class="form-help">Export file entity ID cannot be changed</div>
          </div>

          <div v-if="form.linked_entity_type === 'commodity'" class="form-group">
            <label for="linked_entity_meta">File Category</label>
            <Select
              id="linked_entity_meta"
              v-model="form.linked_entity_meta"
              :options="commodityMetaOptions"
              option-label="label"
              option-value="value"
              placeholder="Select category"
            />
            <div class="form-help">What type of commodity file this is</div>
          </div>

          <div v-if="form.linked_entity_type === 'export'" class="form-group">
            <label for="linked_entity_meta">Export Version</label>
            <input
              id="linked_entity_meta"
              v-model="form.linked_entity_meta"
              type="text"
              class="form-control"
              disabled
              readonly
            />
            <div class="form-help">Export file version cannot be changed</div>
          </div>

        </div>

        <!-- 3. Read-only File Information Fields -->
        <div class="form-group">
          <label>File Type</label>
          <div class="form-control-readonly">
            <span class="type-badge" :class="`type-${file.type}`">
              <font-awesome-icon :icon="getFileIcon(file)" />
              {{ getFileTypeLabel(file.type) }}
            </span>
          </div>
        </div>

        <div class="form-group">
          <label>MIME Type</label>
          <div class="form-control-readonly">{{ file.mime_type }}</div>
        </div>

        <div class="form-group">
          <label>Original Filename</label>
          <div class="form-control-readonly file-path">{{ file.original_path }}</div>
        </div>

        <div class="form-actions">
          <button type="button" class="btn btn-secondary" :disabled="saving" @click="goBack">
            Cancel
          </button>
          <button type="submit" class="btn btn-primary" :disabled="saving || !isFormValid">
            <span v-if="saving">
              <FontAwesomeIcon icon="spinner" spin />
              Saving...
            </span>
            <span v-else>
              <FontAwesomeIcon icon="save" />
              Save Changes
            </span>
          </button>
        </div>
      </form>
    </div>

    <!-- Error Display -->
    <div v-if="saveError" class="form-error">{{ saveError }}</div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Select from 'primevue/select'
import fileService, { type FileEntity, type FileUpdateData } from '@/services/fileService'
import commodityService from '@/services/commodityService'
import locationService from '@/services/locationService'
import areaService from '@/services/areaService'

const route = useRoute()
const router = useRouter()

// State
const file = ref<FileEntity | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)
const saving = ref(false)
const saveError = ref<string | null>(null)

// Form data
const form = ref<FileUpdateData>({
  title: '',
  description: '',
  tags: [],
  path: '',
  linked_entity_type: '',
  linked_entity_id: '',
  linked_entity_meta: ''
})

const tagsInput = ref('')
const errors = ref<Record<string, string>>({})

// Commodity selection state
const loadingCommodities = ref(false)
const commodityOptions = ref<any[]>([])

// Options for select components
const entityTypeOptions = [
  { label: 'No link (standalone file)', value: '' },
  { label: 'Commodity', value: 'commodity' }
]

const commodityMetaOptions = [
  { label: 'Images', value: 'images' },
  { label: 'Invoices', value: 'invoices' },
  { label: 'Manuals', value: 'manuals' }
]

// Wrapper functions to maintain proper context
const isLinked = (file: FileEntity) => {
  return fileService.isLinked(file)
}

const getLinkedEntityDisplay = (file: FileEntity) => {
  return fileService.getLinkedEntityDisplay(file)
}

// Wrapper function to pass current route context
const getLinkedEntityUrl = (file: any) => {
  return fileService.getLinkedEntityUrl(file, route)
}

// Helper function to get file type label
const getFileTypeLabel = (type: string): string => {
  const typeMap: Record<string, string> = {
    'image': 'Image',
    'document': 'Document',
    'video': 'Video',
    'audio': 'Audio',
    'archive': 'Archive',
    'other': 'Other'
  }
  return typeMap[type] || 'Other'
}

// Computed
const fileId = computed(() => route.params.id as string)

const backLinkText = computed(() => {
  const from = route.query.from as string
  if (from === 'export') {
    return 'Back to Export File'
  }
  return 'Back to File'
})

const isFormValid = computed(() => {
  return form.value.path.trim() // Only path is required, title is optional
})

const isExportFile = computed(() => {
  return form.value.linked_entity_type === 'export'
})

// Methods
const loadFile = async () => {
  loading.value = true
  error.value = null

  try {
    const response = await fileService.getFile(fileId.value)
    file.value = response.data.attributes

    // Populate form with current values
    form.value = {
      title: file.value.title,
      description: file.value.description || '',
      tags: [...file.value.tags],
      path: file.value.path,
      linked_entity_type: file.value.linked_entity_type || '',
      linked_entity_id: file.value.linked_entity_id || '',
      linked_entity_meta: file.value.linked_entity_meta || ''
    }

    tagsInput.value = file.value.tags.join(', ')
  } catch (err: any) {
    error.value = err.response?.data?.message || 'Failed to load file'
    console.error('Error loading file:', err)
  } finally {
    loading.value = false
  }
}

const getFileUrl = (file: FileEntity) => {
  return fileService.getDownloadUrl(file)
}

const getFileIcon = (file: FileEntity) => {
  return fileService.getFileIcon(file)
}





const handleImageError = (event: Event) => {
  const img = event.target as HTMLImageElement
  img.style.display = 'none'
  const parent = img.parentElement
  if (parent) {
    parent.innerHTML = '<div class="file-icon"><i class="fas fa-image" style="font-size: 2.5rem; color: var(--text-secondary-color);"></i></div>'
  }
}

const updateTags = () => {
  const tags = tagsInput.value
    .split(',')
    .map(tag => tag.trim())
    .filter(tag => tag.length > 0)

  form.value.tags = [...new Set(tags)] // Remove duplicates
}

const removeTag = (tagToRemove: string) => {
  form.value.tags = form.value.tags.filter(tag => tag !== tagToRemove)
  tagsInput.value = form.value.tags.join(', ')
}

const onEntityTypeChange = () => {
  // Clear entity ID and meta when type changes (but not for export files)
  if (!isExportFile.value) {
    form.value.linked_entity_id = ''
    form.value.linked_entity_meta = ''
  }
}

const loadCommodities = async () => {
  if (commodityOptions.value.length > 0) {
    return // Already loaded
  }

  loadingCommodities.value = true

  try {
    // Load all data in parallel
    const [locationsResponse, areasResponse, commoditiesResponse] = await Promise.all([
      locationService.getLocations(),
      areaService.getAreas(),
      commodityService.getCommodities()
    ])

    const locations = locationsResponse.data.data.map((item: any) => ({
      id: item.id,
      ...item.attributes
    }))

    const areas = areasResponse.data.data.map((item: any) => ({
      id: item.id,
      ...item.attributes
    }))

    const commodities = commoditiesResponse.data.data.map((item: any) => ({
      id: item.id,
      ...item.attributes
    }))

    // Create maps for quick lookups
    const locationMap = new Map(locations.map(loc => [loc.id, loc]))
    const areaMap = new Map(areas.map(area => [area.id, area]))

    // Group commodities by location and area (flat structure for PrimeVue)
    const groupedOptions: any[] = []
    const areaGroups = new Map<string, any>()

    // Group commodities by their areas
    commodities.forEach(commodity => {
      const area = areaMap.get(commodity.area_id)
      if (!area) return

      const location = locationMap.get(area.location_id)
      if (!location) return

      const commodityOption = {
        label: `${commodity.name} (${commodity.short_name})`,
        value: commodity.id
      }

      const areaKey = `${location.id}-${area.id}`
      if (!areaGroups.has(areaKey)) {
        areaGroups.set(areaKey, {
          label: `${location.name} - ${area.name}`,
          items: []
        })
      }

      areaGroups.get(areaKey).items.push(commodityOption)
    })

    // Convert to array and sort
    areaGroups.forEach(group => {
      if (group.items.length > 0) {
        // Sort commodities within group
        group.items.sort((a: any, b: any) => a.label.localeCompare(b.label))
        groupedOptions.push(group)
      }
    })

    // Sort groups by location and area name
    groupedOptions.sort((a, b) => a.label.localeCompare(b.label))

    commodityOptions.value = groupedOptions
  } catch (err: any) {
    console.error('Error loading commodities:', err)
    // Fallback: load all commodities without grouping
    try {
      const commoditiesResponse = await commodityService.getCommodities()
      const commodities = commoditiesResponse.data.data.map((item: any) => ({
        label: `${item.attributes.name} (${item.attributes.short_name})`,
        value: item.id
      }))

      commodityOptions.value = [{
        label: 'All Commodities',
        items: commodities
      }]
    } catch (fallbackErr: any) {
      console.error('Error loading commodities fallback:', fallbackErr)
    }
  } finally {
    loadingCommodities.value = false
  }
}

const validateForm = (): boolean => {
  errors.value = {}

  // Title is now optional, no validation needed

  if (!form.value.path.trim()) {
    errors.value.path = 'Filename is required'
  }

  return Object.keys(errors.value).length === 0
}

const updateFile = async () => {
  if (!validateForm()) return

  saving.value = true
  saveError.value = null

  try {
    await fileService.updateFile(fileId.value, form.value)

    // Preserve context when redirecting after save
    const from = route.query.from as string
    const exportId = route.query.exportId as string

    if (from === 'export' && exportId) {
      router.push(`/files/${fileId.value}?from=export&exportId=${exportId}`)
    } else {
      router.push(`/files/${fileId.value}`)
    }
  } catch (err: any) {
    saveError.value = err.response?.data?.message || 'Failed to save changes'
    console.error('Error updating file:', err)
  } finally {
    saving.value = false
  }
}



const goBack = () => {
  const from = route.query.from as string
  const exportId = route.query.exportId as string

  if (from === 'export' && exportId) {
    router.push(`/files/${fileId.value}?from=export&exportId=${exportId}`)
  } else {
    router.push(`/files/${fileId.value}`)
  }
}

// Lifecycle
onMounted(async () => {
  await loadFile()

  // If the file has a linked commodity, load commodities to show the selection
  if (form.value.linked_entity_type === 'commodity' && form.value.linked_entity_id) {
    await loadCommodities()
  }
})
</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;

.file-edit {
  max-width: 800px;
  margin: 0 auto;
  padding: 20px;
}

.header {
  margin-bottom: 2rem;

  h1 {
    margin: 0;
    font-size: 2rem;
  }
}

h3 {
  margin: 0 0 0.5rem;
  font-size: 1.125rem;
  color: $text-color;
}

.file-preview-card {
  display: flex;
  align-items: flex-start;
  gap: 1.5rem;
  margin-bottom: 2rem;
  padding: 1.5rem;
  background: white;
  border-radius: $default-radius;
  border: 1px solid $border-color;

  .file-preview {
    width: 120px;
    height: 120px;
    border-radius: $default-radius;
    overflow: hidden;
    background: $light-bg-color;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 1px solid $border-color;
    flex-shrink: 0;

    .preview-image {
      width: 100%;
      height: 100%;
      object-fit: cover;
    }

    .file-icon {
      font-size: 3rem;
      color: $text-secondary-color;
    }
  }

  .file-info {
    flex: 1;
    min-width: 0;

    h3 {
      margin: 0 0 1rem;
      color: $text-color;
      font-size: 1.25rem;
      font-weight: 600;
      word-break: break-all;
    }

    .file-meta {
      display: flex;
      gap: 0.75rem;
      margin-bottom: 1rem;
      flex-wrap: wrap;

      .file-type {
        font-size: 0.875rem;
        padding: 0.375rem 0.75rem;
        border-radius: 12px;
        background: $primary-color;
        color: white;
        font-weight: 500;
        text-transform: capitalize;
      }

      .file-ext {
        font-size: 0.875rem;
        padding: 0.375rem 0.75rem;
        border-radius: 12px;
        background: $text-secondary-color;
        color: white;
        font-weight: 500;
        text-transform: uppercase;
      }
    }


  }

  // Responsive design
  @media (width <= 768px) {
    flex-direction: column;
    gap: 1rem;
    padding: 1rem;

    .file-preview {
      align-self: center;
      width: 100px;
      height: 100px;
    }

    .file-info {
      text-align: center;

      .file-meta {
        justify-content: center;
      }

      .current-link-info {
        text-align: left;
      }
    }
  }
}


.file-path {
  word-break: break-all;
  word-wrap: break-word;
  overflow-wrap: break-word;
}

.type-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  border-radius: $default-radius;
  font-size: 0.875rem;
  font-weight: 500;

  &.type-image {
    background-color: #e8f5e8;
    color: #2e7d32;
  }

  &.type-document {
    background-color: #e3f2fd;
    color: #1565c0;
  }

  &.type-video {
    background-color: #fce4ec;
    color: #c2185b;
  }

  &.type-audio {
    background-color: #fff3e0;
    color: #ef6c00;
  }

  &.type-archive {
    background-color: #f3e5f5;
    color: #7b1fa2;
  }

  &.type-other {
    background-color: #f5f5f5;
    color: #616161;
  }

  svg {
    font-size: 1rem;
  }
}

// Custom form styles for file edit
.form-control-readonly {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid $border-color;
  border-radius: $default-radius;
  background-color: #f8f9fa;
  color: $text-color;
  font-size: 1rem;
  word-break: break-all;
  min-height: 48px;
  display: flex;
  align-items: center;
}

.filename-input-group {
  display: flex;
  align-items: center;
  border: 1px solid $border-color;
  border-radius: $default-radius;
  background: white;
  overflow: hidden;

  .filename-input {
    flex: 1;
    border: none;
    border-radius: 0;
    margin: 0;
    padding: 0.75rem;

    &:focus {
      outline: none;
      border: none;
      box-shadow: none;
    }
  }

  .file-extension {
    padding: 0.75rem;
    background-color: #f8f9fa;
    color: $text-secondary-color;
    font-size: 1rem;
    font-weight: 500;
    border-left: 1px solid $border-color;
    white-space: nowrap;
  }

  &:focus-within {
    border-color: $primary-color;
    box-shadow: 0 0 0 2px rgba($primary-color, 0.2);
  }

  &.is-invalid {
    border-color: $danger-color;
  }
}

.form-help {
  margin-top: 0.25rem;
  font-size: 0.875rem;
  color: $text-secondary-color;
}

.error-message {
  margin-top: 0.25rem;
  font-size: 0.875rem;
  color: $danger-color;
}

.form-section {
  margin-top: 2rem;
  padding-top: 1.5rem;
  border-top: 1px solid $border-color;

  > .form-help {
    margin-bottom: 1rem;
    color: $text-secondary-color;
  }
}

.current-link-info {
  margin-top: 1.25rem;
  margin-bottom: 0.5rem;

  .info-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem 1rem;
    background-color: #e3f2fd;
    color: #1565c0;
    border-radius: $default-radius;
    font-size: 0.875rem;
    font-weight: 500;
    border: 1px solid #bbdefb;
    transition: all 0.2s ease;
    text-decoration: none;
    cursor: pointer;

    &:hover {
      background-color: #e1f5fe;
      border-color: #90caf9;
      text-decoration: none;
    }

    .link-nav-icon {
      font-size: 0.75rem;
      opacity: 0.8;
      transition: opacity 0.2s ease;
    }

    &:hover .link-nav-icon {
      opacity: 1;
    }
  }
}

.tags-preview {
  margin-top: 0.75rem;
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;

  .tag {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    font-size: 0.875rem;
    padding: 0.25rem 0.75rem;
    border-radius: 12px;
    background: $primary-color;
    color: white;

    .tag-remove {
      background: none;
      border: none;
      color: white;
      cursor: pointer;
      font-size: 1rem;
      line-height: 1;

      &:hover {
        opacity: 0.7;
      }
    }
  }
}

.required::after {
  content: ' *';
  color: $danger-color;
}

// Breadcrumb navigation styling
.breadcrumb-nav {
  margin-bottom: 1rem;
}

.breadcrumb-link {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  color: $text-secondary-color;
  text-decoration: none;
  font-size: 0.875rem;

  &:hover {
    color: $primary-color;
    text-decoration: none;
  }

  svg {
    font-size: 0.75rem;
  }
}

// PrimeVue Select styling to match form controls
:deep(.p-select) {
  width: 100%;

  .p-select-label {
    padding: 0.75rem;
    font-size: 1rem;
    border: 1px solid $border-color;
    border-radius: $default-radius;
    background: white;
    color: $text-color;
    min-height: 48px;
    display: flex;
    align-items: center;

    &.p-placeholder {
      color: $text-secondary-color;
    }
  }

  &.p-disabled .p-select-label {
    background-color: #f8f9fa;
    color: $text-secondary-color;
    cursor: not-allowed;
  }

  &.p-focus .p-select-label {
    border-color: $primary-color;
    box-shadow: 0 0 0 2px rgba($primary-color, 0.2);
  }

  &:not(.p-disabled):hover .p-select-label {
    border-color: #c4c4c4;
  }
}
</style>
