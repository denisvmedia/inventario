<template>
  <div class="file-create-view">
    <div class="page-header">
      <div class="header-nav">
        <button class="btn btn-secondary" @click="goBack">
          <i class="bx bx-arrow-back"></i>
          Back to Files
        </button>
      </div>
      
      <div class="header-content">
        <h1>Upload File</h1>
        <p class="page-description">Upload and manage a new file</p>
      </div>
    </div>

    <div class="upload-form">
      <!-- File Upload Section -->
      <div class="upload-section">
        <h2>Select File</h2>
        
        <div class="file-uploader">
          <div
            class="upload-area"
            :class="{ 'drag-over': isDragOver, 'has-file': selectedFile }"
            @dragover.prevent="onDragOver"
            @dragleave.prevent="onDragLeave"
            @drop.prevent="onDrop"
            @click="triggerFileInput"
          >
            <input
              ref="fileInput"
              type="file"
              class="file-input"
              @change="onFileSelected"
            />
            
            <div v-if="!selectedFile" class="upload-content">
              <div class="upload-icon">
                <i class="bx bx-cloud-upload"></i>
              </div>
              <p class="upload-text">
                <span class="upload-prompt">Drag and drop a file here</span>
                <span class="upload-or">or</span>
                <span class="browse-button">click to browse</span>
              </p>
              <p class="upload-hint">Supports images, documents, videos, audio files, and archives</p>
            </div>
            
            <div v-else class="selected-file">
              <div class="file-preview">
                <img
                  v-if="filePreview && detectedType === 'image'"
                  :src="filePreview"
                  :alt="selectedFile.name"
                  class="file-thumbnail"
                />
                <div v-else class="file-icon">
                  <i :class="getFileIconForType(detectedType)"></i>
                </div>
              </div>
              
              <div class="file-info">
                <h3>{{ selectedFile.name }}</h3>
                <p>{{ formatFileSize(selectedFile.size) }} • {{ detectedType }}</p>
              </div>
              
              <button class="btn-remove" @click.stop="removeFile" title="Remove file">
                <i class="bx bx-x"></i>
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Metadata Form -->
      <div v-if="selectedFile" class="metadata-section">
        <h2>File Information</h2>
        
        <form @submit.prevent="uploadFile" class="metadata-form">
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
            <div class="form-help">Auto-detected: {{ getFileTypeLabel(detectedType) }}</div>
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
            <button type="button" class="btn btn-secondary" @click="goBack" :disabled="uploading">
              Cancel
            </button>
            <button type="submit" class="btn btn-primary" :disabled="uploading || !isFormValid">
              <span v-if="uploading">
                <i class="bx bx-loader-alt bx-spin"></i>
                Uploading...
              </span>
              <span v-else>
                <i class="bx bx-upload"></i>
                Upload File
              </span>
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Error Display -->
    <div v-if="error" class="error-section">
      <div class="error-card">
        <div class="error-icon">
          <i class="bx bx-error"></i>
        </div>
        <div class="error-content">
          <h3>Upload Failed</h3>
          <p>{{ error }}</p>
        </div>
        <button class="btn btn-secondary" @click="clearError">
          <i class="bx bx-x"></i>
          Dismiss
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import fileService, { type FileCreateData } from '@/services/fileService'

const router = useRouter()

// State
const selectedFile = ref<File | null>(null)
const filePreview = ref<string | null>(null)
const isDragOver = ref(false)
const uploading = ref(false)
const error = ref<string | null>(null)

// Form data
const form = ref<FileCreateData>({
  title: '',
  description: '',
  type: 'other',
  tags: []
})

const tagsInput = ref('')
const errors = ref<Record<string, string>>({})

// File input ref
const fileInput = ref<HTMLInputElement | null>(null)

// File type options
const fileTypeOptions = fileService.getFileTypeOptions()

// Computed
const detectedType = computed(() => {
  if (!selectedFile.value) return 'other'
  
  const mimeType = selectedFile.value.type
  if (mimeType.startsWith('image/')) return 'image'
  if (mimeType.startsWith('video/')) return 'video'
  if (mimeType.startsWith('audio/')) return 'audio'
  if (mimeType === 'application/zip' || mimeType === 'application/x-zip-compressed') return 'archive'
  if (mimeType === 'application/pdf' || 
      mimeType === 'text/plain' || 
      mimeType === 'text/csv' ||
      mimeType.includes('document') ||
      mimeType.includes('spreadsheet') ||
      mimeType.includes('presentation')) return 'document'
  
  return 'other'
})

const isFormValid = computed(() => {
  return selectedFile.value && form.value.title.trim() && form.value.type
})

// Methods
const triggerFileInput = () => {
  fileInput.value?.click()
}

const onDragOver = () => {
  isDragOver.value = true
}

const onDragLeave = () => {
  isDragOver.value = false
}

const onDrop = (event: DragEvent) => {
  isDragOver.value = false
  if (event.dataTransfer?.files && event.dataTransfer.files.length > 0) {
    handleFileSelection(event.dataTransfer.files[0])
  }
}

const onFileSelected = (event: Event) => {
  const input = event.target as HTMLInputElement
  if (input.files && input.files.length > 0) {
    handleFileSelection(input.files[0])
  }
}

const handleFileSelection = (file: File) => {
  selectedFile.value = file
  
  // Auto-fill title from filename
  if (!form.value.title) {
    const nameWithoutExt = file.name.replace(/\.[^/.]+$/, '')
    form.value.title = nameWithoutExt
  }
  
  // Auto-detect and set file type
  form.value.type = detectedType.value as any
  
  // Generate preview for images
  if (file.type.startsWith('image/')) {
    const reader = new FileReader()
    reader.onload = (e) => {
      filePreview.value = e.target?.result as string
    }
    reader.readAsDataURL(file)
  } else {
    filePreview.value = null
  }
  
  // Clear file input
  if (fileInput.value) {
    fileInput.value.value = ''
  }
}

const removeFile = () => {
  selectedFile.value = null
  filePreview.value = null
  form.value.title = ''
  form.value.description = ''
  form.value.type = 'other'
  form.value.tags = []
  tagsInput.value = ''
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

const getFileIconForType = (type: string): string => {
  switch (type) {
    case 'image': return 'bx-image'
    case 'document': return 'bx-file-doc'
    case 'video': return 'bx-video'
    case 'audio': return 'bx-music'
    case 'archive': return 'bx-archive'
    default: return 'bx-file'
  }
}

const getFileTypeLabel = (type: string) => {
  const option = fileTypeOptions.find(opt => opt.value === type)
  return option?.label || type
}

const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return '0 Bytes'
  
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const validateForm = (): boolean => {
  errors.value = {}
  
  if (!form.value.title.trim()) {
    errors.value.title = 'Title is required'
  }
  
  if (!form.value.type) {
    errors.value.type = 'File type is required'
  }
  
  return Object.keys(errors.value).length === 0
}

const uploadFile = async () => {
  if (!selectedFile.value || !validateForm()) return
  
  uploading.value = true
  error.value = null
  
  try {
    const response = await fileService.createFile(form.value, selectedFile.value)
    const fileId = response.data.id
    router.push(`/files/${fileId}`)
  } catch (err: any) {
    error.value = err.response?.data?.message || 'Failed to upload file'
    console.error('Error uploading file:', err)
  } finally {
    uploading.value = false
  }
}

const clearError = () => {
  error.value = null
}

const goBack = () => {
  router.push('/files')
}

// Watch for file type changes to update form
watch(detectedType, (newType) => {
  if (selectedFile.value && !form.value.type) {
    form.value.type = newType as any
  }
})
</script>

<style lang="scss" scoped>
@use '@/assets/variables' as *;

.file-create-view {
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

.upload-form {
  .upload-section,
  .metadata-section {
    background: $light-bg-color;
    border-radius: 8px;
    padding: 2rem;
    margin-bottom: 2rem;
    border: 1px solid $border-color;

    h2 {
      margin: 0 0 1.5rem 0;
      color: $text-color;
      font-size: 1.25rem;
    }
  }
}

.file-uploader {
  .upload-area {
    border: 2px dashed $border-color;
    border-radius: 8px;
    padding: 3rem 2rem;
    text-align: center;
    cursor: pointer;
    transition: all 0.2s;
    
    &:hover,
    &.drag-over {
      border-color: $primary-color;
      background: rgba($primary-color, 0.05);
    }

    &.has-file {
      border-style: solid;
      padding: 1.5rem;
    }

    .file-input {
      display: none;
    }

    .upload-content {
      .upload-icon {
        i {
          font-size: 3rem;
          color: $text-secondary-color;
          margin-bottom: 1rem;
        }
      }

      .upload-text {
        margin: 0 0 0.5rem 0;
        color: $text-color;

        .upload-prompt {
          display: block;
          font-weight: 500;
        }

        .upload-or {
          display: block;
          margin: 0.5rem 0;
          color: $text-secondary-color;
        }

        .browse-button {
          color: $primary-color;
          font-weight: 500;
        }
      }

      .upload-hint {
        margin: 0;
        font-size: 0.875rem;
        color: $text-secondary-color;
      }
    }
    
    .selected-file {
      display: flex;
      align-items: center;
      gap: 1rem;
      text-align: left;
      
      .file-preview {
        width: 60px;
        height: 60px;
        border-radius: 8px;
        overflow: hidden;
        background: white;
        display: flex;
        align-items: center;
        justify-content: center;
        border: 1px solid $border-color;

        .file-thumbnail {
          width: 100%;
          height: 100%;
          object-fit: cover;
        }

        .file-icon {
          i {
            font-size: 2rem;
            color: $text-secondary-color;
          }
        }
      }

      .file-info {
        flex: 1;

        h3 {
          margin: 0 0 0.25rem 0;
          color: $text-color;
          font-size: 1rem;
        }

        p {
          margin: 0;
          color: $text-secondary-color;
          font-size: 0.875rem;
        }
      }

      .btn-remove {
        width: 32px;
        height: 32px;
        border-radius: 50%;
        border: none;
        background: $error-color;
        color: white;
        display: flex;
        align-items: center;
        justify-content: center;
        cursor: pointer;

        &:hover {
          opacity: 0.8;
        }
      }
    }
  }
}

.metadata-form {
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

.error-section {
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
