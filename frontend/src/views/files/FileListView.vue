<template>
  <div class="file-list">
    <div class="header">
      <div class="header-title">
        <h1>Files</h1>
        <div v-if="!loading && files.length > 0" class="item-count">
          {{ totalFiles }} file{{ totalFiles !== 1 ? 's' : '' }}
        </div>
      </div>
      <div class="header-actions">
        <router-link to="/files/create" class="btn btn-primary">
          <FontAwesomeIcon icon="plus" />
          Upload File
        </router-link>
      </div>
    </div>

    <!-- Filters -->
    <div class="filters-card">
      <div class="filters-row">
        <div class="filter-group">
          <label for="search">Search</label>
          <input
            id="search"
            v-model="filters.search"
            type="text"
            placeholder="Search files..."
            class="form-control"
            @input="debouncedSearch"
          />
        </div>

        <div class="filter-group">
          <label for="type">Type</label>
          <select id="type" v-model="filters.type" class="form-control" @change="loadFiles">
            <option value="">All Types</option>
            <option v-for="option in fileTypeOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </div>

        <div class="filter-group">
          <label for="tags">Tags</label>
          <input
            id="tags"
            v-model="filters.tags"
            type="text"
            placeholder="Comma-separated tags"
            class="form-control"
            @input="debouncedSearch"
          />
        </div>

        <div class="filter-group">
          <button class="btn btn-secondary" @click="clearFilters">
            <FontAwesomeIcon icon="times" />
            Clear
          </button>
        </div>
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="loading">
      <div class="spinner"></div>
      <p>Loading files...</p>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="error">
      <div class="error-icon">
        <FontAwesomeIcon icon="exclamation-circle" />
      </div>
      <h3>Error Loading Files</h3>
      <p>{{ error }}</p>
      <button class="btn btn-primary" @click="loadFiles">
        <FontAwesomeIcon icon="redo" />
        Try Again
      </button>
    </div>

    <!-- Files Grid -->
    <div v-else-if="files.length > 0" class="files-grid">
      <div
        v-for="file in files"
        :key="file.id"
        class="file-card"
        @click="viewFile(file)"
      >
          <div class="file-preview">
            <img
              v-if="file.type === 'image'"
              :src="getFileUrl(file)"
              :alt="getDisplayTitle(file)"
              :data-file-id="file.id"
              class="file-thumbnail"
              @error="onImageError"
            />
            <div v-else class="file-icon">
              <FontAwesomeIcon :icon="getFileIcon(file)" />
            </div>
          </div>

          <div class="file-info">
            <h3 class="file-title" :title="getDisplayTitle(file)">{{ getDisplayTitle(file) }}</h3>
            <p class="file-description" :title="file.description">{{ file.description || 'No description' }}</p>

            <div class="file-meta">
              <span class="file-type">{{ getFileTypeLabel(file.type) }}</span>
              <span class="file-ext">{{ file.ext }}</span>
            </div>

            <div v-if="file.tags && file.tags.length > 0" class="file-tags">
              <span v-for="tag in file.tags.slice(0, 3)" :key="tag" class="tag">
                {{ tag }}
              </span>
              <span v-if="file.tags.length > 3" class="tag-more">
                +{{ file.tags.length - 3 }} more
              </span>
            </div>

            <div v-if="isLinked(file)" class="file-linked-entity">
              <router-link
                :to="getLinkedEntityUrl(file)"
                class="entity-badge-small"
                title="View linked entity"
                @click.stop
              >
                <FontAwesomeIcon :icon="getEntityIcon(file)" />
                <span class="entity-text">{{ getLinkedEntityDisplay(file) }}</span>
                <FontAwesomeIcon icon="external-link-alt" class="entity-link-icon" />
              </router-link>
            </div>
          </div>

          <div class="file-actions" @click.stop>
            <button
              class="btn-icon"
              title="Download"
              @click="downloadFile(file)"
            >
              <FontAwesomeIcon icon="download" />
            </button>
            <button
              class="btn-icon"
              title="Edit"
              @click="editFile(file)"
            >
              <FontAwesomeIcon icon="edit" />
            </button>
            <button
              v-if="canDeleteFile(file)"
              class="btn-icon btn-danger"
              title="Delete"
              @click="confirmDelete(file)"
            >
              <FontAwesomeIcon icon="trash" />
            </button>
            <button
              v-else
              class="btn-icon btn-disabled"
              :title="getDeleteRestrictionReason(file)"
              disabled
            >
              <FontAwesomeIcon icon="lock" />
            </button>
          </div>
        </div>

      <!-- Pagination -->
      <div v-if="totalPages > 1" class="pagination-card">
        <div class="pagination-info">
          Showing {{ (currentPage - 1) * pageSize + 1 }} to {{ Math.min(currentPage * pageSize, totalFiles) }} of {{ totalFiles }} files
        </div>
        <div class="pagination-controls">
          <router-link
            v-if="currentPage > 1"
            :to="getPaginationUrl(currentPage - 1)"
            class="btn btn-secondary pagination-link"
          >
            <font-awesome-icon icon="chevron-left" />
            Previous
          </router-link>
          <span
            v-else
            class="btn btn-secondary pagination-link disabled"
          >
            <font-awesome-icon icon="chevron-left" />
            Previous
          </span>

          <div class="page-numbers">
            <router-link
              v-for="page in visiblePages"
              :key="page"
              :to="getPaginationUrl(page)"
              class="btn pagination-link"
              :class="{ 'btn-primary': page === currentPage, 'btn-secondary': page !== currentPage }"
            >
              {{ page }}
            </router-link>
          </div>

          <router-link
            v-if="currentPage < totalPages"
            :to="getPaginationUrl(currentPage + 1)"
            class="btn btn-secondary pagination-link"
          >
            Next
            <font-awesome-icon icon="chevron-right" />
          </router-link>
          <span
            v-else
            class="btn btn-secondary pagination-link disabled"
          >
            Next
            <font-awesome-icon icon="chevron-right" />
          </span>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div v-else class="empty">
      <div class="empty-message">
        <div class="empty-icon">
          <font-awesome-icon icon="file" size="4x" />
        </div>
        <h3>No Files Found</h3>
        <p v-if="hasActiveFilters">No files match your current filters. Try adjusting your search criteria.</p>
        <p v-else>You haven't uploaded any files yet. Upload your first file to get started.</p>
        <div class="action-button">
          <router-link to="/files/create" class="btn btn-primary">
            <font-awesome-icon icon="plus" />
            Upload File
          </router-link>
        </div>
      </div>
    </div>

    <!-- Delete Confirmation Modal -->
    <Confirmation
      v-model:visible="showDeleteModal"
      :title="'Delete File'"
      :message="`Are you sure you want to delete <strong>${fileToDelete ? getDisplayTitle(fileToDelete) : ''}</strong>?<br><br><span class='warning-text'>This action cannot be undone. The file will be permanently deleted.</span>`"
      :confirm-label="deleting ? 'Deleting...' : 'Delete'"
      :cancel-label="'Cancel'"
      :confirm-button-class="'danger'"
      :confirm-disabled="deleting"
      :confirmation-icon="'exclamation-triangle'"
      @confirm="deleteFile"
      @cancel="cancelDelete"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import fileService, { type FileEntity } from '@/services/fileService'
import Confirmation from '@/components/Confirmation.vue'


// Type for signed URLs structure
interface URLData {
  url: string
  thumbnails?: {
    small?: string
    medium?: string
  }
}

const router = useRouter()
const route = useRoute()

// Image error handling with automatic URL refresh
const onImageError = async (event: Event) => {
  const img = event.target as HTMLImageElement
  const fileId = img.dataset.fileId

  if (!fileId) {
    console.warn('Image load error: no file ID found', event)
    return
  }

  console.warn('Image load error for file:', fileId, 'attempting to refresh URL')

  try {
    // Find the file in our current list
    const file = files.value.find(f => f.id === fileId)
    if (!file) {
      console.warn('File not found in current list:', fileId)
      return
    }

    // Generate new signed URL with thumbnails
    const response = await fileService.generateSignedUrlWithThumbnails(file)

    // Update the image source with new thumbnail URL
    if (response.thumbnails?.medium) {
      img.src = response.thumbnails.medium
    } else if (response.thumbnails?.small) {
      img.src = response.thumbnails.small
    } else {
      img.src = response.url
    }

    console.log('Successfully refreshed URL for file:', fileId)
  } catch (error) {
    console.error('Failed to refresh URL for file:', fileId, error)
    // Hide the broken image
    img.style.display = 'none'
  }
}

// State
const files = ref<FileEntity[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const deleting = ref(false)
const fileUrls = ref<Record<string, string>>({})

// Pagination
const currentPage = ref(1)
const pageSize = ref(20)
const totalFiles = ref(0)

// Filters
const filters = ref({
  search: '',
  type: '',
  tags: ''
})

// Delete modal
const showDeleteModal = ref(false)
const fileToDelete = ref<FileEntity | null>(null)

// File type options
const fileTypeOptions = fileService.getFileTypeOptions()

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

// Computed
const totalPages = computed(() => Math.ceil(totalFiles.value / pageSize.value))

const hasActiveFilters = computed(() => {
  return filters.value.search || filters.value.type || filters.value.tags
})

const visiblePages = computed(() => {
  const pages = []
  const start = Math.max(1, currentPage.value - 2)
  const end = Math.min(totalPages.value, currentPage.value + 2)

  for (let i = start; i <= end; i++) {
    pages.push(i)
  }

  return pages
})

// Methods
const loadFiles = async () => {
  loading.value = true
  error.value = null

  try {
    const params = {
      page: currentPage.value,
      limit: pageSize.value,
      ...(filters.value.search && { search: filters.value.search }),
      ...(filters.value.type && { type: filters.value.type }),
      ...(filters.value.tags && { tags: filters.value.tags })
    }

    const response = await fileService.getFiles(params)
    files.value = response.data.data
    totalFiles.value = response.data.meta.total

    // Use signed URLs from response metadata if available
    if (response.data.meta.signed_urls) {
      console.log('FileListView: Using pre-generated signed URLs')
      // Extract URLs from the signed URLs structure
      const extractedUrls: Record<string, string> = {}
      for (const [fileId, urlData] of Object.entries(response.data.meta.signed_urls as Record<string, URLData>)) {
        // For images, prefer medium thumbnail if available, otherwise use original URL
        if (urlData.thumbnails?.medium) {
          extractedUrls[fileId] = urlData.thumbnails.medium
        } else if (urlData.thumbnails?.small) {
          extractedUrls[fileId] = urlData.thumbnails.small
        } else {
          extractedUrls[fileId] = urlData.url
        }
      }
      fileUrls.value = extractedUrls
    } else {
      console.log('FileListView: No signed URLs in response, URLs will be empty')
      fileUrls.value = {}
    }
  } catch (err: any) {
    error.value = err.response?.data?.message || 'Failed to load files'
    console.error('Error loading files:', err)
  } finally {
    loading.value = false
  }
}

// Debounced search
let searchTimeout: number
const debouncedSearch = () => {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    currentPage.value = 1
    loadFiles()
  }, 500)
}

const clearFilters = () => {
  filters.value = {
    search: '',
    type: '',
    tags: ''
  }
  currentPage.value = 1
  loadFiles()
}



// Generate pagination URL for a specific page
const getPaginationUrl = (page: number) => {
  const query = { ...route.query }
  if (page > 1) {
    query.page = page.toString()
  } else {
    delete query.page
  }
  return {
    path: route.path,
    query
  }
}



// No more image ref tracking needed (thumbnails generated during upload)

const getFileUrl = (file: FileEntity) => {
  return fileUrls.value[file.id] || ''
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



const viewFile = (file: FileEntity) => {
  router.push(`/files/${file.id}`)
}

const editFile = (file: FileEntity) => {
  router.push(`/files/${file.id}/edit`)
}

const downloadFile = async (file: FileEntity) => {
  try {
    await fileService.downloadFile(file)
  } catch (error) {
    console.error('Failed to download file:', error)
    // You might want to show a user-friendly error message here
  }
}

const canDeleteFile = (file: FileEntity) => {
  return fileService.canDelete(file)
}

const getDeleteRestrictionReason = (file: FileEntity) => {
  return fileService.getDeleteRestrictionReason(file)
}

const confirmDelete = (file: FileEntity) => {
  if (!canDeleteFile(file)) {
    return // Don't allow deletion of restricted files
  }
  fileToDelete.value = file
  showDeleteModal.value = true
}

const cancelDelete = () => {
  fileToDelete.value = null
  showDeleteModal.value = false
}

const deleteFile = async () => {
  if (!fileToDelete.value) return

  deleting.value = true

  try {
    await fileService.deleteFile(fileToDelete.value.id)
    await loadFiles() // Reload the list
    cancelDelete()
  } catch (err: any) {
    error.value = err.response?.data?.message || 'Failed to delete file'
    console.error('Error deleting file:', err)
  } finally {
    deleting.value = false
  }
}

// Initialize current page from URL parameters
const initializeFromUrl = () => {
  const pageParam = route.query.page
  if (pageParam && typeof pageParam === 'string') {
    const page = parseInt(pageParam, 10)
    if (page > 0) {
      currentPage.value = page
    }
  }
}

// Watch for route changes to update pagination
watch(() => route.query.page, (newPage) => {
  const page = newPage && typeof newPage === 'string' ? parseInt(newPage, 10) : 1
  if (page > 0 && page !== currentPage.value) {
    currentPage.value = page
    loadFiles()
  }
}, { immediate: true })

// Lifecycle
onMounted(() => {
  initializeFromUrl()
  loadFiles()
})
</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;

.file-list {
  max-width: $container-max-width;
  margin: 0 auto;
  padding: 20px;
}

// Header styles are now in shared _header.scss

.filters-card {
  background: white;
  border-radius: $default-radius;
  padding: 1.5rem;
  margin-bottom: 2rem;
  box-shadow: $box-shadow;

  .filters-row {
    display: grid;
    grid-template-columns: 2fr 1fr 1.5fr auto;
    gap: 1rem;
    align-items: end;

    @media (width <= 768px) {
      grid-template-columns: 1fr;
    }
  }

  .filter-group {
    label {
      display: block;
      margin-bottom: 0.5rem;
      font-weight: 500;
      color: $text-color;
    }
  }
}

.files-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 1.5rem;
}

.file-card {
  background: white;
  border-radius: $default-radius;
  overflow: hidden;
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
  position: relative;
  border: 1px solid $border-color;
  box-shadow: $box-shadow;

  &:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 8px rgb(0 0 0 / 10%);
  }

  .file-preview {
    height: 160px;
    background: $light-bg-color;
    display: flex;
    align-items: center;
    justify-content: center;

    .file-thumbnail {
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
    padding: 1rem;

    .file-title {
      margin: 0 0 0.5rem;
      font-size: 1rem;
      font-weight: 600;
      color: $text-color;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .file-description {
      margin: 0 0 0.75rem;
      font-size: 0.875rem;
      color: $text-secondary-color;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .file-meta {
      display: flex;
      gap: 0.5rem;
      margin-bottom: 0.75rem;

      .file-type,
      .file-ext {
        font-size: 0.75rem;
        padding: 0.25rem 0.5rem;
        border-radius: 4px;
        background: $light-bg-color;
        color: $text-secondary-color;
        border: 1px solid $border-color;
      }
    }

    .file-tags {
      display: flex;
      flex-wrap: wrap;
      gap: 0.25rem;

      .tag {
        font-size: 0.75rem;
        padding: 0.125rem 0.375rem;
        border-radius: 12px;
        background: $primary-color;
        color: white;
      }

      .tag-more {
        font-size: 0.75rem;
        color: $text-secondary-color;
      }
    }

    .file-linked-entity {
      margin-top: 0.75rem;

      .entity-badge-small {
        display: inline-flex;
        align-items: center;
        gap: 0.5rem;
        padding: 0.5rem 0.75rem;
        background-color: #e3f2fd;
        color: #1565c0;
        border-radius: $default-radius;
        font-size: 0.75rem;
        font-weight: 500;
        border: 1px solid #bbdefb;
        transition: all 0.2s ease;
        max-width: 100%;
        text-decoration: none;
        cursor: pointer;

        &:hover {
          background-color: #e1f5fe;
          border-color: #90caf9;
          text-decoration: none;
        }

        .entity-text {
          flex: 1;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
          min-width: 0;
        }

        .entity-link-icon {
          flex-shrink: 0;
          font-size: 0.625rem;
          opacity: 0.8;
          transition: opacity 0.2s ease;
        }

        &:hover .entity-link-icon {
          opacity: 1;
        }
      }
    }
  }

  .file-actions {
    position: absolute;
    top: 0.5rem;
    right: 0.5rem;
    display: flex;
    gap: 0.25rem;
    opacity: 0;
    transition: opacity 0.2s;

    .btn-icon {
      width: 32px;
      height: 32px;
      border-radius: 50%;
      border: none;
      background: rgb(255 255 255 / 90%);
      color: $text-color;
      display: flex;
      align-items: center;
      justify-content: center;
      cursor: pointer;
      transition: background-color 0.2s;

      &:hover {
        background: white;
      }

      &.btn-danger {
        color: $danger-color;
      }

      &.btn-disabled {
        color: $text-secondary-color;
        cursor: not-allowed;
        opacity: 0.5;

        &:hover {
          background: rgb(255 255 255 / 90%);
        }
      }
    }
  }

  &:hover .file-actions {
    opacity: 1;
  }
}

.pagination-card {
  background: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
  margin-top: 2rem;
  padding: 1.5rem;
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;

  @media (width <= 768px) {
    flex-direction: column;
    gap: 1rem;
    align-items: center;
    padding: 1rem;
  }

  .pagination-info {
    color: $text-secondary-color;
    font-size: 0.875rem;
    font-weight: 500;
    margin: 0;

    @media (width <= 768px) {
      text-align: center;
    }
  }

  .pagination-controls {
    display: flex;
    align-items: center;
    gap: 0.5rem;

    .pagination-link {
      text-decoration: none;
      min-width: 2.5rem;
      height: 2.5rem;
      display: flex;
      align-items: center;
      justify-content: center;
      border-radius: $default-radius;
      font-weight: 500;
      transition: all 0.2s ease;
      border: 1px solid transparent;
      font-size: 0.875rem;

      &.btn-primary {
        background-color: $primary-color;
        color: white;
        border-color: $primary-color;
        box-shadow: 0 1px 3px rgba($primary-color, 0.3);

        &:hover {
          background-color: $primary-hover-color;
          border-color: $primary-hover-color;
          text-decoration: none;
          transform: translateY(-1px);
          box-shadow: 0 2px 6px rgba($primary-color, 0.4);
        }
      }

      &.btn-secondary {
        background-color: white;
        color: $text-color;
        border-color: $border-color;
        box-shadow: 0 1px 3px rgba($border-color, 0.3);

        &:hover {
          background-color: $light-bg-color;
          border-color: $primary-color;
          color: $primary-color;
          text-decoration: none;
          transform: translateY(-1px);
          box-shadow: 0 2px 6px rgba($primary-color, 0.2);
        }
      }

      &.disabled {
        background-color: $light-bg-color;
        color: $text-secondary-color;
        border-color: $border-color;
        cursor: not-allowed;
        opacity: 0.6;
        box-shadow: none;

        &:hover {
          background-color: $light-bg-color;
          color: $text-secondary-color;
          border-color: $border-color;
          transform: none;
          box-shadow: none;
        }
      }

      // Icon spacing
      .fa-chevron-left {
        margin-right: 0.5rem;
      }

      .fa-chevron-right {
        margin-left: 0.5rem;
      }
    }

    .page-numbers {
      display: flex;
      gap: 0.25rem;
      margin: 0 0.75rem;

      @media (width <= 768px) {
        margin: 0 0.5rem;
      }

      .pagination-link {
        min-width: 2.5rem;
        padding: 0;
      }
    }
  }
}

// Loading, error, and empty states use shared styles from _components.scss
.loading,
.error,
.empty {
  .spinner {
    width: 40px;
    height: 40px;
    border: 4px solid $light-bg-color;
    border-top: 4px solid $primary-color;
    border-radius: 50%;
    animation: spin 1s linear infinite;
    margin: 0 auto 1rem;
  }

  .error-icon,
  .empty-icon {
    font-size: 4rem;
    color: $text-secondary-color;
    margin-bottom: 1rem;
  }

  .warning-text {
    color: $error-color;
  }
}

.empty-message {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1.5rem;

  p {
    margin-bottom: 0;
    font-size: 1.1rem;
  }
}

.action-button {
  margin-top: 0.5rem;
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
