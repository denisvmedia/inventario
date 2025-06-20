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
          Back to File
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
import fileService, { type FileEntity, type FileUpdateData } from '@/services/fileService'

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
  path: ''
})

const tagsInput = ref('')
const errors = ref<Record<string, string>>({})

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

const isFormValid = computed(() => {
  return form.value.path.trim() // Only path is required, title is optional
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
      path: file.value.path
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
    router.push(`/files/${fileId.value}`)
  } catch (err: any) {
    saveError.value = err.response?.data?.message || 'Failed to save changes'
    console.error('Error updating file:', err)
  } finally {
    saving.value = false
  }
}



const goBack = () => {
  router.push(`/files/${fileId.value}`)
}

// Lifecycle
onMounted(() => {
  loadFile()
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

.file-preview-card {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 2rem;

  .file-preview {
    width: 80px;
    height: 80px;
    border-radius: $default-radius;
    overflow: hidden;
    background: $light-bg-color;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 1px solid $border-color;

    .preview-image {
      width: 100%;
      height: 100%;
      object-fit: cover;
    }

    .file-icon {
      font-size: 2.5rem;
      color: $text-secondary-color;
    }
  }

  .file-info {
    flex: 1;

    h3 {
      margin: 0 0 0.5rem;
      color: $text-color;
      font-size: 1.125rem;
    }

    .file-meta {
      display: flex;
      gap: 0.5rem;

      span {
        font-size: 0.875rem;
        padding: 0.25rem 0.5rem;
        border-radius: 4px;
        background: $light-bg-color;
        color: $text-secondary-color;
        border: 1px solid $border-color;
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
</style>
