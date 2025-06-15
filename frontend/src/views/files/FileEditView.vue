<template>
  <div class="file-edit-view">
    <!-- Loading State -->
    <div v-if="loading" class="loading-state">
      <div class="spinner"></div>
      <p>Loading file...</p>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="error-state">
      <div class="error-icon">
        <i class="bx bx-error"></i>
      </div>
      <h3>Error Loading File</h3>
      <p>{{ error }}</p>
      <div class="error-actions">
        <button class="btn btn-secondary" @click="goBack">
          <i class="bx bx-arrow-back"></i>
          Go Back
        </button>
        <button class="btn btn-primary" @click="loadFile">
          <i class="bx bx-refresh"></i>
          Try Again
        </button>
      </div>
    </div>

    <!-- Edit Form -->
    <div v-else-if="file" class="edit-content">
      <div class="page-header">
        <div class="header-nav">
          <button class="btn btn-secondary" @click="goBack">
            <i class="bx bx-arrow-back"></i>
            Back to File
          </button>
        </div>
        
        <div class="header-content">
          <h1>Edit File</h1>
          <p class="page-description">Update file information and metadata</p>
        </div>
      </div>

      <!-- File Preview -->
      <div class="file-preview-section">
        <div class="file-preview">
          <img
            v-if="file.type === 'image'"
            :src="getFileUrl(file)"
            :alt="file.title"
            class="preview-image"
            @error="handleImageError"
          />
          <div v-else class="file-icon">
            <i :class="getFileIcon(file)"></i>
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

      <!-- Edit Form -->
      <div class="edit-form-section">
        <form @submit.prevent="updateFile" class="edit-form">
          <div class="form-group">
            <label for="title" class="required">Title</label>
            <input
              id="title"
              v-model="form.title"
              type="text"
              class="form-control"
              :class="{ 'error': errors.title }"
              placeholder="Enter a title for this file"
              required
            />
            <div v-if="errors.title" class="error-message">{{ errors.title }}</div>
          </div>

          <div class="form-group">
            <label for="path" class="required">Filename</label>
            <input
              id="path"
              v-model="form.path"
              type="text"
              class="form-control"
              :class="{ 'error': errors.path }"
              placeholder="Enter filename (without extension)"
              required
            />
            <div v-if="errors.path" class="error-message">{{ errors.path }}</div>
            <div class="form-help">This will be the filename when downloaded (extension will be added automatically)</div>
          </div>

          <div class="form-group">
            <label for="description">Description</label>
            <textarea
              id="description"
              v-model="form.description"
              class="form-control"
              :class="{ 'error': errors.description }"
              placeholder="Optional description"
              rows="3"
            ></textarea>
            <div v-if="errors.description" class="error-message">{{ errors.description }}</div>
          </div>

          <div class="form-group">
            <label for="type" class="required">File Type</label>
            <select
              id="type"
              v-model="form.type"
              class="form-control"
              :class="{ 'error': errors.type }"
              required
            >
              <option v-for="option in fileTypeOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
            <div v-if="errors.type" class="error-message">{{ errors.type }}</div>
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
                <button type="button" @click="removeTag(tag)" class="tag-remove">×</button>
              </span>
            </div>
          </div>

          <div class="form-actions">
            <button type="button" class="btn btn-secondary" @click="goBack" :disabled="saving">
              Cancel
            </button>
            <button type="button" class="btn btn-primary" :disabled="saving || !isFormValid" @click="updateFile">
              <span v-if="saving">
                <i class="bx bx-loader-alt bx-spin"></i>
                Saving...
              </span>
              <span v-else>
                <i class="bx bx-save"></i>
                Save Changes
              </span>
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Error Display -->
    <div v-if="saveError" class="error-section">
      <div class="error-card">
        <div class="error-icon">
          <i class="bx bx-error"></i>
        </div>
        <div class="error-content">
          <h3>Save Failed</h3>
          <p>{{ saveError }}</p>
        </div>
        <button class="btn btn-secondary" @click="clearSaveError">
          <i class="bx bx-x"></i>
          Dismiss
        </button>
      </div>
    </div>
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
  type: 'other',
  tags: [],
  path: ''
})

const tagsInput = ref('')
const errors = ref<Record<string, string>>({})

// File type options
const fileTypeOptions = fileService.getFileTypeOptions()

// Computed
const fileId = computed(() => route.params.id as string)

const isFormValid = computed(() => {
  return form.value.title.trim() && form.value.path.trim() && form.value.type
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
      type: file.value.type,
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

const getFileTypeLabel = (type: string) => {
  const option = fileTypeOptions.find(opt => opt.value === type)
  return option?.label || type
}

const handleImageError = (event: Event) => {
  const img = event.target as HTMLImageElement
  img.style.display = 'none'
  const parent = img.parentElement
  if (parent) {
    parent.innerHTML = '<div class="file-icon"><i class="bx bx-image"></i></div>'
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
  
  if (!form.value.title.trim()) {
    errors.value.title = 'Title is required'
  }
  
  if (!form.value.path.trim()) {
    errors.value.path = 'Filename is required'
  }
  
  if (!form.value.type) {
    errors.value.type = 'File type is required'
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

const clearSaveError = () => {
  saveError.value = null
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
@use '@/assets/variables' as *;

.file-edit-view {
  padding: 2rem;
  max-width: 800px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 2rem;
  
  .header-nav {
    margin-bottom: 1rem;
  }
  
  .header-content {
    h1 {
      margin: 0 0 0.5rem 0;
      color: $text-color;
    }

    .page-description {
      margin: 0;
      color: $text-secondary-color;
    }
  }
}

.file-preview-section {
  background: $light-bg-color;
  border-radius: 8px;
  padding: 1.5rem;
  margin-bottom: 2rem;
  display: flex;
  align-items: center;
  gap: 1rem;
  border: 1px solid $border-color;

  .file-preview {
    width: 80px;
    height: 80px;
    border-radius: 8px;
    overflow: hidden;
    background: white;
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
      i {
        font-size: 2.5rem;
        color: $text-secondary-color;
      }
    }
  }

  .file-info {
    flex: 1;

    h3 {
      margin: 0 0 0.5rem 0;
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
        background: white;
        color: $text-secondary-color;
        border: 1px solid $border-color;
      }
    }
  }
}

.edit-form-section {
  background: $light-bg-color;
  border-radius: 8px;
  padding: 2rem;
  border: 1px solid $border-color;

  .edit-form {
    .form-group {
      margin-bottom: 1.5rem;

      label {
        display: block;
        margin-bottom: 0.5rem;
        font-weight: 500;
        color: $text-color;

        &.required::after {
          content: ' *';
          color: $error-color;
        }
      }

      .form-control {
        width: 100%;
        padding: 0.75rem;
        border: 1px solid $border-color;
        border-radius: 4px;
        background: white;
        color: $text-color;

        &:focus {
          outline: none;
          border-color: $primary-color;
        }

        &.error {
          border-color: $error-color;
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
        color: $error-color;
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
    }
    
    .form-actions {
      display: flex;
      gap: 1rem;
      justify-content: flex-end;
      margin-top: 2rem;
      padding-top: 1rem;
      border-top: 1px solid $border-color;
    }
  }
}

.loading-state,
.error-state {
  text-align: center;
  padding: 3rem 1rem;

  .spinner {
    width: 40px;
    height: 40px;
    border: 4px solid $light-bg-color;
    border-top: 4px solid $primary-color;
    border-radius: 50%;
    animation: spin 1s linear infinite;
    margin: 0 auto 1rem;
  }

  .error-icon {
    i {
      font-size: 4rem;
      color: $error-color;
      margin-bottom: 1rem;
    }
  }

  h3 {
    margin: 0 0 1rem 0;
    color: $text-color;
  }

  p {
    margin: 0 0 1.5rem 0;
    color: $text-secondary-color;
  }

  .error-actions {
    display: flex;
    gap: 1rem;
    justify-content: center;
  }
}

.error-section {
  margin-top: 2rem;

  .error-card {
    background: $light-bg-color;
    border: 1px solid $error-color;
    border-radius: 8px;
    padding: 1.5rem;
    display: flex;
    align-items: center;
    gap: 1rem;

    .error-icon {
      i {
        font-size: 1.5rem;
        color: $error-color;
      }
    }

    .error-content {
      flex: 1;

      h3 {
        margin: 0 0 0.25rem 0;
        color: $error-color;
        font-size: 1rem;
      }

      p {
        margin: 0;
        color: $text-color;
      }
    }
  }
}

.bx-spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}
</style>
