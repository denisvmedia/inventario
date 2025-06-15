<template>
  <div class="file-create-view">
    <div class="page-header">
      <div class="header-nav">
        <button class="btn btn-secondary" @click="goBack">
          <FontAwesomeIcon icon="arrow-left" />
          Back to Files
        </button>
      </div>
      
      <div class="header-content">
        <h1>Upload Files</h1>
        <p class="page-description">Upload files - you can edit metadata after upload</p>
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

      <!-- Upload Actions -->
      <div v-if="selectedFile" class="upload-actions">
        <p class="upload-info">
          File will be uploaded with auto-detected metadata. You can edit the details after upload.
        </p>
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

    <!-- Error Display -->
    <div v-if="error" class="error-section">
      <div class="error-card">
        <div class="error-icon">
          <FontAwesomeIcon icon="exclamation-circle" />
        </div>
        <div class="error-content">
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
  .upload-section {
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

  .upload-actions {
    background: $light-bg-color;
    border-radius: 8px;
    padding: 2rem;
    margin-bottom: 2rem;
    border: 1px solid $border-color;
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

// Font Awesome spin animation is handled by the spin prop

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}
</style>
