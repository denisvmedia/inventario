<template>
  <div class="file-create-view">
    <!-- Breadcrumb Navigation -->
    <div class="breadcrumb-nav">
      <button class="breadcrumb-link" @click="goBack">
        <FontAwesomeIcon icon="arrow-left" />
        Back to Files
      </button>
    </div>

    <!-- Page Header -->
    <div class="header">
      <div class="header-title">
        <h1>Upload Files</h1>
      </div>
    </div>

    <!-- Upload Content -->
    <div class="upload-content">
      <!-- File Upload Card -->
      <div class="upload-card">
        <div class="card-header">
          <h2>Select File</h2>
        </div>
        <div class="card-body">
          <p class="form-description">
            File will be uploaded with auto-detected metadata. You can edit the details after upload.
          </p>
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

              <div v-if="!selectedFile" class="upload-content-inner">
                <div class="upload-icon">
                  <FontAwesomeIcon icon="cloud-upload-alt" />
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
                    <FontAwesomeIcon :icon="getFileIcon(selectedFile)" />
                  </div>
                </div>

                <div class="file-info">
                  <h3>{{ selectedFile.name }}</h3>
                  <p>{{ formatFileSize(selectedFile.size) }} • {{ selectedFile.type || 'Unknown type' }}</p>
                </div>

                <button class="btn-remove" @click.stop="removeFile" title="Remove file">
                  <FontAwesomeIcon icon="times" />
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Upload Actions Card -->
      <div v-if="selectedFile" class="upload-actions-card">
        <div class="card-body">
          <div class="action-buttons">
            <button
              type="button"
              class="btn btn-primary"
              :disabled="uploading"
              @click="uploadFile"
            >
              <FontAwesomeIcon v-if="uploading" icon="spinner" spin />
              <FontAwesomeIcon v-else icon="upload" />
              {{ uploading ? 'Uploading...' : 'Upload File' }}
            </button>
            <button type="button" class="btn btn-secondary" @click="goBack" :disabled="uploading">
              Cancel
            </button>
          </div>
        </div>
      </div>

      <!-- Error Display Card -->
      <div v-if="error" class="error-card">
        <div class="card-body">
          <div class="error-content">
            <div class="error-icon">
              <FontAwesomeIcon icon="exclamation-circle" />
            </div>
            <div class="error-text">
              <h3>Upload Failed</h3>
              <p>{{ error }}</p>
            </div>
            <button class="btn btn-secondary" @click="clearError">
              <FontAwesomeIcon icon="times" />
              Dismiss
            </button>
          </div>
        </div>
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

// File input ref
const fileInput = ref<HTMLInputElement | null>(null)

// File type options (for display purposes)
const fileTypeOptions = fileService.getFileTypeOptions()

// Computed
const detectedType = computed(() => {
  if (!selectedFile.value) return null
  const type = selectedFile.value.type
  if (type.startsWith('image/')) return 'image'
  if (type.startsWith('video/')) return 'video'
  if (type.startsWith('audio/')) return 'audio'
  if (type === 'application/pdf') return 'pdf'
  if (type.includes('document') || type.includes('text')) return 'document'
  if (type.includes('zip') || type.includes('archive')) return 'archive'
  return 'file'
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
}

const getFileIcon = (file: File): string => {
  const type = file.type
  if (type.startsWith('image/')) return 'image'
  if (type.startsWith('video/')) return 'video'
  if (type.startsWith('audio/')) return 'music'
  if (type === 'application/zip' || type === 'application/x-zip-compressed') return 'archive'
  if (type === 'application/pdf' || type.includes('document')) return 'file-alt'
  return 'file'
}

const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return '0 Bytes'

  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}



const uploadFile = async () => {
  if (!selectedFile.value) return

  uploading.value = true
  error.value = null

  try {
    const response = await fileService.uploadFile(selectedFile.value)

    // The upload creates the file entity automatically
    // Check if we got a single file or multiple files
    const files = response.data.data
    if (Array.isArray(files) && files.length > 0) {
      // Redirect to the first uploaded file's detail view
      const fileId = files[0].id
      router.push(`/files/${fileId}`)
    } else if (files && files.id) {
      // Single file response
      router.push(`/files/${files.id}`)
    } else {
      // Fallback to files list
      router.push('/files')
    }
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


</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;

.file-create-view {
  max-width: 800px;
  margin: 0 auto;
  padding: 20px;
}

.breadcrumb-nav {
  margin-bottom: 1rem;
}

.breadcrumb-link {
  color: $secondary-color;
  font-size: 0.9rem;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  transition: color 0.2s;
  background: none;
  border: none;
  cursor: pointer;

  &:hover {
    color: $primary-color;
    text-decoration: none;
  }
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 2rem;

  .header-title {
    display: flex;
    flex-direction: column;
    align-items: flex-start;

    h1 {
      margin: 0 0 5px;
      font-size: 2rem;
    }
  }
}

.upload-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.upload-card,
.upload-actions-card,
.error-card {
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

  h2 {
    margin: 0;
    font-size: 1.25rem;
  }
}

.card-body {
  padding: 20px;
}

.form-description {
  margin: 0 0 1.5rem;
  color: $text-secondary-color;
  font-size: 1rem;
  line-height: 1.5;
  padding: 1rem;
  background-color: rgba($primary-color, 0.05);
  border-left: 4px solid $primary-color;
  border-radius: $default-radius;
}

.file-uploader {
  .upload-area {
    border: 2px dashed $border-color;
    border-radius: 8px;
    padding: 3rem 2rem;
    text-align: center;
    cursor: pointer;
    transition: all 0.2s;
    background-color: $light-bg-color;

    &:hover,
    &.drag-over {
      border-color: $primary-color;
      background: rgba($primary-color, 0.05);
    }

    &.has-file {
      border-style: solid;
      padding: 1.5rem;
      cursor: pointer;
    }

    .file-input {
      display: none;
    }

    .upload-content-inner {
      display: flex;
      flex-direction: column;
      align-items: center;

      .upload-icon {
        font-size: 3rem;
        color: $text-secondary-color;
        margin-bottom: 1rem;
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
          font-size: 2rem;
          color: $text-secondary-color;
          line-height: 1;
          display: flex;
          align-items: center;
          justify-content: center;
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

.upload-actions-card {
  text-align: center;

  .upload-info {
    margin: 0 0 1.5rem 0;
    color: $text-secondary-color;
    font-size: 0.875rem;
  }

  .action-buttons {
    display: flex;
    gap: 1rem;
    justify-content: center;
  }
}

.error-content {
  display: flex;
  align-items: center;
  gap: 1rem;

  .error-icon {
    font-size: 1.5rem;
    color: $error-color;
  }

  .error-text {
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

// Loading and error states use shared styles from _components.scss
.loading,
.error {
  text-align: center;
  padding: 2rem;
  background: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;

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
    font-size: 4rem;
    color: $error-color;
    margin-bottom: 1rem;
  }
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}
</style>
