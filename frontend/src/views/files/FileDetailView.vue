<template>
  <div class="file-detail">
    <!-- Loading State -->
    <div v-if="loading" class="loading">
      <div class="spinner"></div>
      <p>Loading file...</p>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="error">
      <div class="error-icon">
        <font-awesome-icon icon="exclamation-triangle" />
      </div>
      <h3>Error Loading File</h3>
      <p>{{ error }}</p>
      <div class="error-actions">
        <button class="btn btn-secondary" @click="goBack">
          <font-awesome-icon icon="arrow-left" />
          Go Back
        </button>
        <button class="btn btn-primary" @click="loadFile">
          <font-awesome-icon icon="redo" />
          Try Again
        </button>
      </div>
    </div>

    <!-- File Content -->
    <div v-else-if="file">
      <!-- Breadcrumb Navigation -->
      <div class="breadcrumb-nav">
        <button class="breadcrumb-link" @click="goBack">
          <font-awesome-icon icon="arrow-left" />
          Back to Files
        </button>
      </div>

      <!-- Header -->
      <div class="header">
        <div class="header-title">
          <h1>{{ getDisplayTitle(file) }}</h1>
          <div class="file-meta">
            <span class="file-type">{{ getFileTypeLabel(file.type) }}</span>
            <span class="file-ext">{{ file.ext }}</span>
            <span v-if="fileSize" class="file-size">{{ fileSize }}</span>
          </div>
        </div>

        <div class="header-actions">
          <button class="btn btn-secondary" @click="downloadFile">
            <font-awesome-icon icon="download" />
            Download
          </button>
          <button class="btn btn-primary" @click="editFile">
            <font-awesome-icon icon="edit" />
            Edit
          </button>
          <button class="btn btn-danger" @click="confirmDelete">
            <font-awesome-icon icon="trash" />
            Delete
          </button>
        </div>
      </div>

      <!-- File Preview -->
      <div class="file-preview-card">
        <!-- Image Preview -->
        <div v-if="file.type === 'image'" class="image-preview">
          <img
            :src="getFileUrl(file)"
            :alt="getDisplayTitle(file)"
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
            <font-awesome-icon :icon="getFileIcon(file)" size="4x" />
          </div>
          <p>Preview not available for this file type</p>
          <button class="btn btn-primary" @click="downloadFile">
            <font-awesome-icon icon="download" />
            Download to View
          </button>
        </div>
      </div>

      <!-- File Information -->
      <div class="file-info">
        <div class="info-grid">
          <div class="info-card">
            <h2>Description</h2>
            <p v-if="file.description">{{ file.description }}</p>
            <p v-else class="no-description">No description provided</p>
          </div>

          <div class="info-card">
            <h2>Tags</h2>
            <div v-if="file.tags && file.tags.length > 0" class="tags-list">
              <span v-for="tag in file.tags" :key="tag" class="tag">
                {{ tag }}
              </span>
            </div>
            <p v-else class="no-tags">No tags</p>
          </div>

          <div v-if="isLinked(file)" class="info-card">
            <h2>Linked Entity</h2>
            <div class="linked-entity-info">
              <div class="entity-badge">
                <FontAwesomeIcon :icon="getEntityIcon(file)" />
                <span class="entity-text">{{ getLinkedEntityDisplay(file) }}</span>
                <router-link
                  :to="getLinkedEntityUrl(file)"
                  class="entity-link"
                  title="View linked entity"
                >
                  <FontAwesomeIcon icon="external-link-alt" />
                </router-link>
              </div>
            </div>
          </div>

          <div class="info-card">
            <h2>File Details</h2>
            <div class="file-details">
              <div class="detail-row">
                <span class="label">Original Name:</span>
                <span class="value">{{ file.original_path }}</span>
              </div>
              <div class="detail-row">
                <span class="label">MIME Type:</span>
                <span class="value">{{ file.mime_type }}</span>
              </div>
              <div v-if="file.created_at" class="detail-row">
                <span class="label">Uploaded:</span>
                <span class="value">{{ formatDate(file.created_at) }}</span>
              </div>
              <div v-if="file.updated_at && file.updated_at !== file.created_at" class="detail-row">
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
          <p>Are you sure you want to delete <strong>{{ file ? getDisplayTitle(file) : '' }}</strong>?</p>
          <p class="warning-text">This action cannot be undone. The file will be permanently deleted.</p>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="cancelDelete">Cancel</button>
          <button class="btn btn-danger" :disabled="deleting" @click="deleteFile">
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

// Make fileService methods available in template
const { isLinked, getLinkedEntityDisplay, getLinkedEntityUrl } = fileService

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

const getDisplayTitle = (file: FileEntity) => {
  return fileService.getDisplayTitle(file)
}

const getEntityIcon = (file: FileEntity) => {
  if (file.linked_entity_type === 'commodity') {
    return 'box'
  } else if (file.linked_entity_type === 'export') {
    return 'file-export'
  }
  return 'link'
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
          <i class="fas fa-image" style="font-size: 4rem; color: var(--text-secondary-color); margin-bottom: 1rem;"></i>
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
@use '@/assets/main' as *;

.file-detail {
  max-width: $container-max-width;
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

// Header styles are now in shared _header.scss

.file-meta {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
  margin-top: 0.5rem;

  @media (width <= 768px) {
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

.file-preview-card {
  background: white;
  border-radius: $default-radius;
  padding: 2rem;
  margin-bottom: 2rem;
  box-shadow: $box-shadow;

  .image-preview {
    text-align: center;

    .preview-image {
      max-width: 100%;
      max-height: 600px;
      border-radius: $default-radius;
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
      font-size: 4rem;
      color: $text-secondary-color;
      margin-bottom: 1rem;
    }

    p {
      margin: 0 0 1.5rem;
      color: $text-secondary-color;
    }
  }
}

.file-info {
  .info-grid {
    display: grid;
    grid-template-columns: 1fr;
    gap: 1.5rem;

    @media (width >= 768px) {
      grid-template-columns: 1fr 1fr;
    }
  }

  .info-card {
    background: white;
    border-radius: $default-radius;
    padding: 1.5rem;
    box-shadow: $box-shadow;

    h2 {
      margin: 0 0 1rem;
      padding-bottom: 0.5rem;
      border-bottom: 1px solid #eee;
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
          word-break: break-all;
        }
      }
    }

    .linked-entity-info {
      .entity-badge {
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

        &:hover {
          background-color: #e1f5fe;
          border-color: #90caf9;
        }

        .entity-text {
          flex: 1;
        }

        .entity-link {
          color: inherit;
          text-decoration: none;
          font-weight: 600;
          display: inline-flex;
          align-items: center;
          padding: 0.25rem;
          border-radius: 4px;
          transition: background-color 0.2s ease;

          &:hover {
            background-color: rgba(255, 255, 255, 0.3);
            text-decoration: none;
          }

          svg {
            font-size: 0.75rem;
          }
        }
      }
    }
  }
}

// Loading and error states use shared styles from _components.scss
.loading,
.error {
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

  .error-actions {
    display: flex;
    gap: 1rem;
    justify-content: center;
  }
}

.modal-overlay {
  position: fixed;
  inset: 0;
  background: $mask-background-color;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;

  .modal-content {
    background: white;
    border-radius: $default-radius;
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
        margin: 0 0 1rem;
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
