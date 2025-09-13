<template>
  <div class="file-viewer">
    <FileList
      :files="files"
      :file-type="fileType"
      :commodity-id="entityId"
      :loading="false"
      :file-urls="fileUrls"
      @delete="confirmDeleteFile"
      @download="downloadFile"
      @update="updateFile"
      @view-details="openFileDetails"
      @open-viewer="handleOpenViewer"
    />

    <!-- File Details Modal -->
    <FileDetails
      v-if="selectedFile"
      :file="selectedFile"
      :file-type="fileType"
      :commodity-id="entityId"
      @close="closeFileDetails"
      @delete="confirmDeleteFile"
      @download="downloadFile"
    />

    <!-- File Viewer Modal -->
    <div v-if="showViewer" class="file-modal" @click="handleModalClick">
      <div class="modal-content" @click.stop>
        <div class="modal-header">
          <h3 :title="currentFileName">{{ currentFileName }}</h3>
          <button class="close-button" title="Close" @click="closeViewer">&times;</button>
        </div>
        <div class="modal-body">
          <!-- Image viewer -->
          <template v-if="isImageFile(currentFile)">
            <button v-if="files.length > 1" class="nav-button prev" title="Previous file" @click="prevFile">&lt;</button>
            <div class="image-container">
              <img
                v-if="currentFileUrl"
                ref="fullImage"
                :src="currentFileUrl"
                :alt="currentFileName"
                class="full-image"
                :class="{ 'zoomed': isZoomed }"
                :style="imageStyle"
                @click="handleImageClick"
                @mousedown="startPan"
                @mousemove="pan"
                @mouseup="endPan"
                @mouseleave="endPan"
              />
              <div v-else class="loading-placeholder">
                Loading preview...
              </div>
            </div>
            <button v-if="files.length > 1" class="nav-button next" title="Next file" @click="nextFile">&gt;</button>
          </template>

          <!-- PDF viewer -->
          <template v-else-if="isPdfFile(currentFile)">
            <button v-if="files.length > 1" class="nav-button prev" title="Previous file" @click="prevFile">&lt;</button>
            <div class="pdf-container">
              <template v-if="!pdfViewerError && currentFileUrl">
                <PDFViewerCanvas
                  :url="currentFileUrl"
                  @error="handlePdfError"
                  @loading="(isLoading) => pdfLoading = isLoading"
                />
              </template>
              <div v-else-if="!currentFileUrl" class="loading-placeholder">
                Loading preview...
              </div>
              <div v-else class="pdf-error-container">
                <div class="file-icon large">
                  <font-awesome-icon icon="file-pdf" size="3x" />
                </div>
                <p>{{ pdfErrorMessage }}</p>
                <button class="btn btn-primary" @click="downloadCurrentFile">
                  <font-awesome-icon icon="download" /> Download PDF
                </button>
              </div>
            </div>
            <button v-if="files.length > 1" class="nav-button next" title="Next file" @click="nextFile">&gt;</button>
          </template>

          <!-- Fallback for other file types -->
          <div v-else class="unsupported-file">
            <div class="file-icon large">
              <font-awesome-icon :icon="getFileIcon(currentFile)" size="3x" />
            </div>
            <p>This file type cannot be previewed. Please download the file to view it.</p>
          </div>
        </div>
        <div class="modal-footer">
          <span class="file-counter">{{ currentIndex + 1 }} / {{ files.length }}</span>
          <div class="file-actions">
            <button class="btn btn-sm btn-primary" @click="downloadCurrentFile">
              <font-awesome-icon icon="download" /> Download
            </button>
            <button v-if="allowDelete" class="btn btn-sm btn-danger" @click="confirmDeleteCurrentFile">
              <font-awesome-icon icon="trash" /> Delete
            </button>
            <button class="btn btn-sm btn-secondary" @click="closeViewer">
              <font-awesome-icon icon="times" /> Close
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Delete Confirmation Dialog -->
    <Confirmation
      v-model:visible="showDeleteConfirmation"
      :title="'Confirm Delete'"
      :message="`Are you sure you want to delete this ${fileType.slice(0, -1)}?`"
      :confirm-label="'Delete'"
      :cancel-label="'Cancel'"
      :confirm-button-class="'danger'"
      :confirmation-icon="'exclamation-triangle'"
      @confirm="confirmDelete"
      @cancel="cancelDelete"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import PDFViewerCanvas from './PDFViewerCanvas.vue'
// PDFViewer is not used, using PDFViewerCanvas instead
import FileList from './FileList.vue'
import FileDetails from './FileDetails.vue'
import Confirmation from './Confirmation.vue'
import fileService from '@/services/fileService'

const props = defineProps({
  files: {
    type: Array,
    required: true
  },
  signedUrls: {
    type: Object,
    default: () => ({})
  },
  entityId: {
    type: String,
    required: true
  },
  entityType: {
    type: String,
    default: 'commodities'
  },
  fileType: {
    type: String,
    required: true,
    validator: (value: string) => ['images', 'manuals', 'invoices'].includes(value)
  },
  allowDelete: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits(['delete', 'download', 'update'])

const showViewer = ref(false)
const currentIndex = ref(0)
const selectedFile = ref(null)
const fullImage = ref(null)
const pdfViewerError = ref(false)
const pdfLoading = ref(true)
const pdfErrorMessage = ref('Unable to display PDF. Please download the file to view it.')
const fileUrls = ref<Record<string, string>>({})

// Zoom and pan state
const isZoomed = ref(false)
const panX = ref(0)
const panY = ref(0)
const isPanning = ref(false)
const startX = ref(0)
const startY = ref(0)

// Variables to track click vs. drag
const isDragging = ref(false)
const isGlobalDragging = ref(false) // Track dragging at the document level
const clickStartTime = ref(0)
const clickStartPos = ref({ x: 0, y: 0 })

// Computed style for the image with zoom and pan
const imageStyle = computed(() => {
  if (!isZoomed.value) {
    return {
      transform: 'none',
      cursor: 'zoom-in'
    }
  } else {
    return {
      transform: `translate(${panX.value}px, ${panY.value}px)`,
      cursor: isPanning.value ? 'grabbing' : 'grab'
    }
  }
})

const currentFile = computed(() => {
  if (props.files.length === 0) return null
  return props.files[currentIndex.value]
})

const currentFileUrl = ref<string>('')

const currentFileName = computed(() => {
  if (!currentFile.value) return ''
  return getFileName(currentFile.value)
})

// Generate signed URL for the current file
const generateCurrentFileUrl = async () => {
  if (!currentFile.value) {
    currentFileUrl.value = ''
    return
  }

  // Use provided signed URLs if available
  const urlData = props.signedUrls[currentFile.value.id]
  if (urlData && urlData.url) {
    currentFileUrl.value = urlData.url
    return
  }

  // Fallback to API call
  try {
    const signedUrl = await fileService.getDownloadUrl(currentFile.value)
    currentFileUrl.value = signedUrl
  } catch (error) {
    console.error('Failed to generate signed URL for file viewer:', error)
    currentFileUrl.value = ''
  }
}

// Watch for changes to current file and generate new signed URL
watch(currentFile, () => {
  generateCurrentFileUrl()
}, { immediate: true })

// Generate signed URLs for all files (for FileList previews and downloads)
const generateFileUrls = async () => {
  const newUrls: Record<string, string> = {}

  // Don't process if no files
  if (!props.files || props.files.length === 0) {
    fileUrls.value = newUrls
    return
  }

  // Use provided signed URLs if available, otherwise fall back to API calls
  console.log('FileViewer generateFileUrls - fileType:', props.fileType, 'files count:', props.files.length)
  console.log('FileViewer generateFileUrls - signedUrls:', props.signedUrls)
  console.log('FileViewer generateFileUrls - signedUrls keys:', Object.keys(props.signedUrls))
  if (Object.keys(props.signedUrls).length > 0) {
    console.log('FileViewer', props.fileType, ': Using pre-generated signed URLs')
    // Use pre-generated signed URLs from the API response
    props.files.forEach((file: any) => {
      const urlData = props.signedUrls[file.id]
      if (urlData) {
        // For image files, prefer medium thumbnail for previews, fallback to original
        if (urlData.thumbnails && urlData.thumbnails.medium && fileService.isImageFile(file)) {
          newUrls[file.id] = urlData.thumbnails.medium
        } else {
          newUrls[file.id] = urlData.url
        }
      }
    })
  } else {
    console.log('FileViewer', props.fileType, ': Falling back to individual API calls')
    // Fallback to individual API calls (for backward compatibility)
    const urlPromises = props.files
      .map(async (file: any) => {
        try {
          const response = await fileService.generateSignedUrlWithThumbnails(file)
          return {
            fileId: file.id,
            file: file,
            url: response.url,
            thumbnails: response.thumbnails
          }
        } catch (error) {
          console.error(`Failed to generate URL for file ${file.id}:`, error)
          return { fileId: file.id, file: file, url: null, thumbnails: null }
        }
      })

    const results = await Promise.all(urlPromises)
    results.forEach(({ fileId, file, url, thumbnails }) => {
      if (url) {
        // For image files, prefer medium thumbnail for previews, fallback to original
        if (thumbnails && thumbnails.medium && fileService.isImageFile(file)) {
          newUrls[fileId] = thumbnails.medium
        } else {
          newUrls[fileId] = url
        }
      }
    })
  }

  fileUrls.value = newUrls
}

// Watch for changes to files or signed URLs and generate new signed URLs
watch([() => props.files, () => props.signedUrls], () => {
  generateFileUrls()
}, { immediate: true })



const getFileName = (file: any) => {
  // Use the Path field directly (it's now just the filename without extension)
  // and add the extension from the ext field
  if (file.path) {
    return file.path + file.ext
  }

  // Check for attributes if using JSON API format
  if (file.attributes) {
    if (file.attributes.path) {
      return file.attributes.path + (file.attributes.ext || '')
    }
  }

  // Fallback to ID with extension if path is not available
  const ext = file.ext || (file.attributes && file.attributes.ext) || ''
  return `${file.id}${ext}`
}

const getFileIcon = (file: any) => {
  if (isPdfFile(file)) {
    return 'file-pdf'
  } else if (isImageFile(file)) {
    return 'file-image'
  } else if (props.fileType === 'manuals') {
    return 'book'
  } else if (props.fileType === 'invoices') {
    return 'file-invoice-dollar'
  }
  return 'file'
}

const isImageFile = (file: any) => {
  if (!file) return false
  const imageExtensions = ['jpg', 'jpeg', 'png', 'gif', 'webp']
  const imageMimeTypes = ['image/jpeg', 'image/png', 'image/gif', 'image/webp']

  // Check for extension - this is now the primary way to identify file types
  const ext = file.ext || (file.attributes && file.attributes.ext)
  if (ext) {
    // Remove the dot if present for comparison
    let extLower = ext.toLowerCase()
    if (extLower.startsWith('.')) {
      extLower = extLower.substring(1)
    }
    if (imageExtensions.includes(extLower)) {
      return true
    }
  }

  // Check for content_type
  const contentType = file.content_type || (file.attributes && file.attributes.content_type) || file.mime_type
  if (contentType && imageMimeTypes.includes(contentType.toLowerCase())) {
    return true
  }

  return false
}

const isPdfFile = (file: any) => {
  console.log('Checking if file is PDF:', file)
  if (!file) return false

  // Check for extension - this is now the primary way to identify file types
  const ext = file.ext || (file.attributes && file.attributes.ext)
  if (ext && (ext.toLowerCase() === '.pdf' || ext.toLowerCase() === 'pdf')) {
    console.log('PDF detected from ext property')
    return true
  }

  // Check for content_type
  const contentType = file.content_type || (file.attributes && file.attributes.content_type) || file.mime_type
  if (contentType && contentType.toLowerCase() === 'application/pdf') {
    console.log('PDF detected from content_type')
    return true
  }

  console.log('Not a PDF file')
  return false
}

const openViewer = (index) => {
  currentIndex.value = index
  showViewer.value = true
  // Reset zoom and pan when opening viewer
  resetZoom()
  // Reset PDF viewer state
  pdfViewerError.value = false
  pdfLoading.value = true
  pdfErrorMessage.value = 'Unable to display PDF. Please download the file to view it.'
  // Prevent scrolling when modal is open
  document.body.style.overflow = 'hidden'
}

const handleOpenViewer = (file) => {
  // Find the index of the file in the files array
  const index = props.files.findIndex(f => f.id === file.id)
  if (index !== -1) {
    openViewer(index)
  }
}

// Handle PDF rendering errors
const handlePdfError = (error) => {
  console.error('PDF rendering error:', error)
  pdfViewerError.value = true

  // Set a more specific error message if available
  if (error && error.message) {
    if (error.message.includes('timeout')) {
      pdfErrorMessage.value = 'PDF loading timed out. Please try downloading the file instead.'
    } else if (error.message.includes('canvas')) {
      pdfErrorMessage.value = 'PDF viewer is not available. Please download the file to view it.'
    } else {
      pdfErrorMessage.value = 'Unable to display PDF. Please download the file to view it.'
    }
  }
}

const handleModalClick = () => {
  // Only close if we're not in a dragging operation
  if (!isGlobalDragging.value) {
    closeViewer()
  }
}

const closeViewer = () => {
  showViewer.value = false
  // Restore scrolling
  document.body.style.overflow = 'auto'
}

const nextFile = () => {
  if (currentIndex.value < props.files.length - 1) {
    currentIndex.value++
  } else {
    currentIndex.value = 0 // Loop back to the first file
  }
  // Reset zoom and pan when changing files
  resetZoom()
}

const prevFile = () => {
  if (currentIndex.value > 0) {
    currentIndex.value--
  } else {
    currentIndex.value = props.files.length - 1 // Loop to the last file
  }
  // Reset zoom and pan when changing files
  resetZoom()
}

// Toggle zoom on click
const toggleZoom = () => {
  if (isZoomed.value) {
    // If already zoomed, reset to fit view
    resetZoom()
  } else {
    // If not zoomed, zoom in
    isZoomed.value = true
    // Reset pan position to center initially
    panX.value = 0
    panY.value = 0
  }
}

// Handle image click - differentiates between click and drag
const handleImageClick = () => {
  // Only toggle zoom if it was a click, not a drag
  if (!isDragging.value) {
    toggleZoom()
  }

  // Reset drag state
  isDragging.value = false
}

const resetZoom = () => {
  isZoomed.value = false
  panX.value = 0
  panY.value = 0
  isPanning.value = false
  isDragging.value = false
  isGlobalDragging.value = false

  // Remove any global event listeners that might be active
  document.removeEventListener('mousemove', handleGlobalMouseMove)
  document.removeEventListener('mouseup', handleGlobalMouseUp)
}

// Pan functions - only active when zoomed
const startPan = (event) => {
  if (isZoomed.value) {
    event.preventDefault()
    isPanning.value = true
    isGlobalDragging.value = false // Reset global dragging state
    startX.value = event.clientX - panX.value
    startY.value = event.clientY - panY.value

    // Track for click vs. drag detection
    clickStartTime.value = Date.now()
    clickStartPos.value = { x: event.clientX, y: event.clientY }
    isDragging.value = false

    // Add global event listeners to track mouse movement outside the image
    document.addEventListener('mousemove', handleGlobalMouseMove)
    document.addEventListener('mouseup', handleGlobalMouseUp)
  }
}

// Global mouse move handler - works even when mouse is outside the image
const handleGlobalMouseMove = (event) => {
  if (!isPanning.value) return

  // Calculate distance moved
  const dx = Math.abs(event.clientX - clickStartPos.value.x)
  const dy = Math.abs(event.clientY - clickStartPos.value.y)

  // If moved more than 5px, consider it a drag
  if (dx > 5 || dy > 5) {
    isDragging.value = true
    isGlobalDragging.value = true // Set global dragging state
  }

  panX.value = event.clientX - startX.value
  panY.value = event.clientY - startY.value
}

// Global mouse up handler
const handleGlobalMouseUp = () => {
  if (isPanning.value) {
    isPanning.value = false

    // Keep the global dragging state true for a short time
    // This prevents the modal from closing when releasing after a drag
    setTimeout(() => {
      isGlobalDragging.value = false
    }, 50) // Short delay to handle the click event that might follow

    // Remove global event listeners
    document.removeEventListener('mousemove', handleGlobalMouseMove)
    document.removeEventListener('mouseup', handleGlobalMouseUp)
  }
}

// These local handlers are still needed for the image element
const pan = (event) => {
  if (!isPanning.value) return
  event.preventDefault()
}

const endPan = () => {
  // Local handler - the actual end of panning is handled by the global handler
}

// Download functions
const downloadFile = (file: any) => {
  // Only pass through the download event to parent
  emit('download', file)
}

const downloadCurrentFile = () => {
  if (!currentFile.value) return
  downloadFile(currentFile.value)
}

// Delete functions
const fileToDelete = ref<any>(null)
const showDeleteConfirmation = ref(false)

const confirmDeleteFile = (file: any) => {
  // Store the file to delete and show the confirmation dialog
  fileToDelete.value = file
  showDeleteConfirmation.value = true
}

const confirmDelete = () => {
  if (!fileToDelete.value) return

  // Emit delete event to parent
  emit('delete', fileToDelete.value)

  // Handle UI updates after deletion
  handlePostDeleteUI(fileToDelete.value)

  // Close the dialog
  showDeleteConfirmation.value = false
  fileToDelete.value = null
}

const cancelDelete = () => {
  showDeleteConfirmation.value = false
  fileToDelete.value = null
}

// Handle UI updates after a file is deleted
const handlePostDeleteUI = (file: any) => {
  // If we're deleting the current file in the viewer, adjust accordingly
  if (showViewer.value && currentFile.value && currentFile.value.id === file.id) {
    if (props.files.length <= 1) {
      closeViewer()
    } else {
      // Stay on the same index unless it's the last file
      if (currentIndex.value === props.files.length - 1) {
        currentIndex.value--
      }
    }
  }

  // Close file details if we're deleting the selected file
  if (selectedFile.value && selectedFile.value.id === file.id) {
    closeFileDetails()
  }
}

const updateFile = (data: any) => {
  emit('update', data)
}

const openFileDetails = (file: any) => {
  selectedFile.value = file
}

const closeFileDetails = () => {
  selectedFile.value = null
}

const confirmDeleteCurrentFile = () => {
  if (!currentFile.value) return
  confirmDeleteFile(currentFile.value)
}

// Keyboard navigation
const handleKeyDown = (event) => {
  if (!showViewer.value) return

  switch (event.key) {
    case 'Escape':
      closeViewer()
      break
    case 'ArrowLeft':
      prevFile()
      break
    case 'ArrowRight':
      nextFile()
      break
    case ' ': // Space key
      if (isImageFile(currentFile.value)) {
        toggleZoom()
      }
      break
  }
}

// Debug: Log files when they change
watch(() => props.files, (newFiles) => {
  console.log('Files changed:', newFiles)
  console.log('File types:', newFiles.map(file => {
    const isPdf = isPdfFile(file)
    const isImage = isImageFile(file)
    return { id: file.id, isPdf, isImage }
  }))
}, { immediate: true })

// Add keyboard event listener
onMounted(() => {
  window.addEventListener('keydown', handleKeyDown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeyDown)

  // Clean up any remaining global event listeners
  document.removeEventListener('mousemove', handleGlobalMouseMove)
  document.removeEventListener('mouseup', handleGlobalMouseUp)
})
</script>

<style lang="scss" scoped>
@use 'sass:color';
@use '@/assets/variables' as *;

.file-viewer {
  margin-bottom: 1.5rem;
}

.no-files {
  padding: 1rem;
  text-align: center;
  color: $secondary-color;
  background-color: $light-bg-color;
  border-radius: $default-radius;
}

.files-container {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 1rem;
}

.file-item {
  border: 1px solid $border-color;
  border-radius: $default-radius;
  overflow: hidden;
  background-color: white;
  box-shadow: $box-shadow;
  transition: transform 0.2s ease, box-shadow 0.2s ease;

  &:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 8px rgb(0 0 0 / 10%);
  }
}

.file-preview {
  height: 150px;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: $light-bg-color;
  overflow: hidden;
  cursor: pointer;
}

.preview-image {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.file-icon {
  font-size: 3rem;
  color: $secondary-color;

  &.large {
    font-size: 5rem;
    margin-bottom: 1rem;
  }
}

.file-info {
  padding: 0.75rem;
}

.file-name {
  font-weight: 500;
  margin-bottom: 0.5rem;
  word-break: normal;
  overflow-wrap: break-word;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-actions {
  display: flex;
  gap: 0.5rem;
}

/* Modal styles */
.file-modal {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background-color: rgb(0 0 0 / 50%);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1000;
}

.modal-content {
  background-color: white;
  border-radius: $default-radius;
  box-shadow: 0 4px 20px rgb(0 0 0 / 15%);
  width: 90%;
  max-width: 1200px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  position: relative; /* Ensure proper positioning for navigation buttons */
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  border-bottom: 1px solid $border-color;

  h3 {
    margin: 0;
    font-size: 1.25rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 70%;
  }
}

.close-button {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: $secondary-color;
  margin-left: 0.5rem;

  &:hover {
    color: color.adjust($secondary-color, $lightness: -10%);
  }
}

.modal-body {
  display: flex;
  justify-content: center;
  padding: 1rem;
  flex: 1;
  overflow: hidden;
  position: relative;
  background-color: $light-bg-color;
  min-height: 300px; /* Ensure minimum height for content */
}

.image-container {
  width: 100%;
  height: 100%;
  display: flex;
  justify-content: center;
  align-items: center;
  overflow: hidden;
}

.full-image {
  max-width: 100%;
  max-height: 70vh;
  object-fit: contain;
  transition: transform 0.3s ease;
  transform-origin: center center;

  &.zoomed {
    max-width: none;
    max-height: none;
    width: 200%;
    height: auto;
    object-fit: cover;
  }
}

.pdf-container {
  width: 100%;
  display: flex;
  justify-content: center;
  align-items: center;
  position: relative; /* Ensure proper positioning */
}

.pdf-viewer {
  width: 100%;
  height: 70vh;
  border: none;
}

.pdf-loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 2rem;
  width: 100%;
  height: 300px;

  p {
    margin-top: 1rem;
    color: $secondary-color;
  }
}

.spinner {
  width: 40px;
  height: 40px;
  border: 4px solid rgb(0 0 0 / 10%);
  border-radius: 50%;
  border-top-color: $primary-color;
  animation: spin 1s ease-in-out infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.pdf-error-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 2rem;
  background-color: $light-bg-color;
  border-radius: $default-radius;
  width: 100%;
  max-width: 500px;
  margin: 0 auto;

  p {
    margin: 1rem 0;
    color: $secondary-color;
  }

  .btn {
    margin-top: 1rem;
  }
}

.unsupported-file {
  text-align: center;
  padding: 2rem;
}

.nav-button {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  background-color: rgb(255 255 255 / 50%);
  color: $text-color;
  border: none;
  border-radius: 50%;
  width: 40px;
  height: 40px;
  font-size: 1.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  z-index: 10;
  transition: background-color 0.2s ease;
  box-shadow: $box-shadow;

  &:hover {
    background-color: rgb(255 255 255 / 80%);
  }

  @media (width <= 768px) {
    width: 30px;
    height: 30px;
    font-size: 1.2rem;
  }
}

.prev {
  left: 10px;
}

.next {
  right: 10px;
}

.modal-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  border-top: 1px solid $border-color;
}

.file-counter {
  color: $secondary-color;
}

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
  border-radius: $default-radius;
}
</style>
