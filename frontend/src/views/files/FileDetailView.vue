<template>
  <div class="file-detail-view">
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

    <!-- File Content -->
    <div v-else-if="file" class="file-content">
      <!-- Header -->
      <div class="file-header">
        <div class="header-nav">
          <button class="btn btn-secondary" @click="goBack">
            <i class="bx bx-arrow-back"></i>
            Back to Files
          </button>
        </div>
        
        <div class="header-info">
          <h1>{{ file.title }}</h1>
          <div class="file-meta">
            <span class="file-type">{{ getFileTypeLabel(file.type) }}</span>
            <span class="file-ext">{{ file.ext }}</span>
            <span class="file-size" v-if="fileSize">{{ fileSize }}</span>
          </div>
        </div>
        
        <div class="header-actions">
          <button class="btn btn-secondary" @click="downloadFile">
            <i class="bx bx-download"></i>
            Download
          </button>
          <button class="btn btn-primary" @click="editFile">
            <i class="bx bx-edit"></i>
            Edit
          </button>
          <button class="btn btn-danger" @click="confirmDelete">
            <i class="bx bx-trash"></i>
            Delete
          </button>
        </div>
      </div>

      <!-- File Preview -->
      <div class="file-preview-section">
        <!-- Image Preview -->
        <div v-if="file.type === 'image'" class="image-preview">
          <img
            :src="getFileUrl(file)"
            :alt="file.title"
            class="preview-image"
            @error="handleImageError"
          />
        </div>

        <!-- PDF Preview -->
        <div v-else-if="file.mime_type === 'application/pdf'" class="pdf-preview">
          <PDFViewerCanvas
            :url="getFileUrl(file)"
            @error="handlePdfError"
          />
        </div>

        <!-- Other File Types -->
        <div v-else class="file-placeholder">
          <div class="file-icon">
            <i :class="getFileIcon(file)"></i>
          </div>
          <p>Preview not available for this file type</p>
          <button class="btn btn-primary" @click="downloadFile">
            <i class="bx bx-download"></i>
            Download to View
          </button>
        </div>
      </div>

      <!-- File Information -->
      <div class="file-info-section">
        <div class="info-grid">
          <div class="info-card">
            <h3>Description</h3>
            <p v-if="file.description">{{ file.description }}</p>
            <p v-else class="no-description">No description provided</p>
          </div>

          <div class="info-card">
            <h3>Tags</h3>
            <div v-if="file.tags && file.tags.length > 0" class="tags-list">
              <span v-for="tag in file.tags" :key="tag" class="tag">
                {{ tag }}
              </span>
            </div>
            <p v-else class="no-tags">No tags</p>
          </div>

          <div class="info-card">
            <h3>File Details</h3>
            <div class="file-details">
              <div class="detail-row">
                <span class="label">Original Name:</span>
                <span class="value">{{ file.original_path }}</span>
              </div>
              <div class="detail-row">
                <span class="label">MIME Type:</span>
                <span class="value">{{ file.mime_type }}</span>
              </div>
              <div class="detail-row" v-if="file.created_at">
                <span class="label">Uploaded:</span>
                <span class="value">{{ formatDate(file.created_at) }}</span>
              </div>
              <div class="detail-row" v-if="file.updated_at && file.updated_at !== file.created_at">
                <span class="label">Modified:</span>
                <span class="value">{{ formatDate(file.updated_at) }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Delete Confirmation Modal -->
    <div v-if="showDeleteModal" class="modal-overlay" @click="cancelDelete">
      <div class="modal-content" @click.stop>
        <div class="modal-header">
          <h3>Delete File</h3>
          <button class="btn-close" @click="cancelDelete">&times;</button>
        </div>
        <div class="modal-body">
          <p>Are you sure you want to delete <strong>{{ file?.title }}</strong>?</p>
          <p class="warning-text">This action cannot be undone. The file will be permanently deleted.</p>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="cancelDelete">Cancel</button>
          <button class="btn btn-danger" @click="deleteFile" :disabled="deleting">
            <span v-if="deleting">Deleting...</span>
            <span v-else>Delete</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import PDFViewerCanvas from '@/components/PDFViewerCanvas.vue'
import fileService, { type FileEntity } from '@/services/fileService'

const route = useRoute()
const router = useRouter()

// State
const file = ref<FileEntity | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)
const deleting = ref(false)
const fileSize = ref<string | null>(null)

// Delete modal
const showDeleteModal = ref(false)

// File type options for labels
const fileTypeOptions = fileService.getFileTypeOptions()

// Computed
const fileId = computed(() => route.params.id as string)

// Methods
const loadFile = async () => {
  loading.value = true
  error.value = null
  
  try {
    const response = await fileService.getFile(fileId.value)
    file.value = response.data.attributes
    
    // Try to get file size (this would need to be added to the API response)
    // For now, we'll skip this or implement it later
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

const formatDate = (dateString: string) => {
  return new Date(dateString).toLocaleString()
}

const handleImageError = (event: Event) => {
  const img = event.target as HTMLImageElement
  img.style.display = 'none'
  const parent = img.parentElement
  if (parent) {
    parent.innerHTML = `
      <div class="file-placeholder">
        <div class="file-icon">
          <i class="bx bx-image"></i>
        </div>
        <p>Image could not be loaded</p>
      </div>
    `
  }
}

const handlePdfError = () => {
  // PDF viewer will handle its own error display
}

const goBack = () => {
  router.push('/files')
}

const downloadFile = () => {
  if (file.value) {
    fileService.downloadFile(file.value)
  }
}

const editFile = () => {
  router.push(`/files/${fileId.value}/edit`)
}

const confirmDelete = () => {
  showDeleteModal.value = true
}

const cancelDelete = () => {
  showDeleteModal.value = false
}

const deleteFile = async () => {
  if (!file.value) return
  
  deleting.value = true
  
  try {
    await fileService.deleteFile(file.value.id)
    router.push('/files')
  } catch (err: any) {
    error.value = err.response?.data?.message || 'Failed to delete file'
    console.error('Error deleting file:', err)
  } finally {
    deleting.value = false
    showDeleteModal.value = false
  }
}

// Lifecycle
onMounted(() => {
  loadFile()
})
</script>

<style lang="scss" scoped>
@use '@/assets/variables' as *;

.file-detail-view {
  padding: 2rem;
  max-width: 1200px;
  margin: 0 auto;
}

.file-header {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 2rem;
  align-items: center;
  margin-bottom: 2rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid $border-color;

  @media (max-width: 768px) {
    grid-template-columns: 1fr;
    gap: 1rem;
    text-align: center;
  }

  .header-info {
    h1 {
      margin: 0 0 0.5rem 0;
      color: $text-color;
    }
    
    .file-meta {
      display: flex;
      gap: 0.5rem;
      flex-wrap: wrap;

      @media (max-width: 768px) {
        justify-content: center;
      }

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

  .header-actions {
    display: flex;
    gap: 0.5rem;

    @media (max-width: 768px) {
      justify-content: center;
    }
  }
}

.file-preview-section {
  background: $light-bg-color;
  border-radius: 8px;
  padding: 2rem;
  margin-bottom: 2rem;
  border: 1px solid $border-color;

  .image-preview {
    text-align: center;

    .preview-image {
      max-width: 100%;
      max-height: 600px;
      border-radius: 8px;
      box-shadow: $box-shadow;
    }
  }

  .pdf-preview {
    min-height: 600px;
  }

  .file-placeholder {
    text-align: center;
    padding: 3rem 1rem;

    .file-icon {
      i {
        font-size: 4rem;
        color: $text-secondary-color;
        margin-bottom: 1rem;
      }
    }

    p {
      margin: 0 0 1.5rem 0;
      color: $text-secondary-color;
    }
  }
}

.file-info-section {
  .info-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
    gap: 1.5rem;
  }
  
  .info-card {
    background: $light-bg-color;
    border-radius: 8px;
    padding: 1.5rem;
    border: 1px solid $border-color;

    h3 {
      margin: 0 0 1rem 0;
      color: $text-color;
      font-size: 1.125rem;
    }

    p {
      margin: 0;
      color: $text-color;

      &.no-description,
      &.no-tags {
        color: $text-secondary-color;
        font-style: italic;
      }
    }

    .tags-list {
      display: flex;
      flex-wrap: wrap;
      gap: 0.5rem;

      .tag {
        font-size: 0.875rem;
        padding: 0.25rem 0.75rem;
        border-radius: 12px;
        background: $primary-color;
        color: white;
      }
    }

    .file-details {
      .detail-row {
        display: flex;
        justify-content: space-between;
        margin-bottom: 0.75rem;

        &:last-child {
          margin-bottom: 0;
        }

        .label {
          font-weight: 500;
          color: $text-secondary-color;
        }

        .value {
          color: $text-color;
          word-break: break-word;
        }
      }
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

.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: $mask-background-color;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;

  .modal-content {
    background: white;
    border-radius: 8px;
    width: 90%;
    max-width: 500px;
    box-shadow: $box-shadow;

    .modal-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 1.5rem;
      border-bottom: 1px solid $border-color;

      h3 {
        margin: 0;
        color: $text-color;
      }

      .btn-close {
        background: none;
        border: none;
        font-size: 1.5rem;
        cursor: pointer;
        color: $text-secondary-color;

        &:hover {
          color: $text-color;
        }
      }
    }

    .modal-body {
      padding: 1.5rem;

      p {
        margin: 0 0 1rem 0;
        color: $text-color;

        &:last-child {
          margin-bottom: 0;
        }

        &.warning-text {
          color: $error-color;
        }
      }
    }

    .modal-footer {
      display: flex;
      justify-content: flex-end;
      gap: 1rem;
      padding: 1.5rem;
      border-top: 1px solid $border-color;
    }
  }
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}
</style>
