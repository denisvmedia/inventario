<template>
  <div class="image-viewer">
    <div v-if="images.length === 0" class="no-images">
      No images to display
    </div>
    <div v-else class="files-container">
      <div v-for="(image, index) in images" :key="image.id" class="file-item">
        <div class="file-preview" @click="openGallery(index)">
          <img
            :src="getImageUrl(image)"
            :alt="getImageName(image)"
            class="preview-image"
            :title="getFileName(image)"
          />
        </div>
        <div class="file-info">
          <div class="file-name" :title="getFileName(image)">{{ getFileName(image) }}</div>
          <div class="file-actions">
            <button class="btn btn-sm btn-primary" @click="downloadImage(image)">
              <i class="fas fa-download"></i> Download
            </button>
            <button v-if="allowDelete" class="btn btn-sm btn-danger" @click="confirmDeleteImage(image)">
              <i class="fas fa-trash"></i> Delete
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Image Gallery Modal -->
    <div v-if="showGallery" class="image-modal" @click="closeGallery">
      <div class="modal-content" @click.stop>
        <div class="modal-header">
          <h3 :title="currentImageName">{{ currentImageName }}</h3>
          <button class="close-button" @click="closeGallery" title="Close">&times;</button>
        </div>
        <div class="modal-body">
          <button v-if="images.length > 1" class="nav-button prev" @click="prevImage">&lt;</button>
          <div class="image-container">
            <img
              :src="currentImageUrl"
              :alt="currentImageName"
              class="full-image"
              :class="{ 'zoomed': isZoomed }"
              :style="imageStyle"
              ref="fullImage"
              @click="handleImageClick"
              @mousedown="startPan"
              @mousemove="pan"
              @mouseup="endPan"
              @mouseleave="endPan"
            />
          </div>
          <button v-if="images.length > 1" class="nav-button next" @click="nextImage">&gt;</button>
        </div>
        <div class="modal-footer">
          <span class="image-counter">{{ currentIndex + 1 }} / {{ images.length }}</span>
          <div class="image-actions">
            <button class="btn btn-sm btn-primary" @click="downloadCurrentImage">
              <i class="fas fa-download"></i> Download
            </button>
            <button v-if="allowDelete" class="btn btn-sm btn-danger" @click="confirmDeleteCurrentImage">
              <i class="fas fa-trash"></i> Delete
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, defineProps, defineEmits, onMounted, onBeforeUnmount } from 'vue'

const props = defineProps({
  images: {
    type: Array,
    required: true
  },
  entityId: {
    type: String,
    required: true
  },
  entityType: {
    type: String,
    required: true,
    default: 'commodities'
  },
  allowDelete: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits(['delete', 'download'])

const showGallery = ref(false)
const currentIndex = ref(0)
const fullImage = ref(null)

// Zoom and pan state
const isZoomed = ref(false)
const panX = ref(0)
const panY = ref(0)
const isPanning = ref(false)
const startX = ref(0)
const startY = ref(0)

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

const currentImage = computed(() => {
  if (props.images.length === 0) return null
  return props.images[currentIndex.value]
})

const currentImageUrl = computed(() => {
  if (!currentImage.value) return ''
  return getImageUrl(currentImage.value)
})

const currentImageName = computed(() => {
  if (!currentImage.value) return ''
  return getImageName(currentImage.value)
})

const getImageUrl = (image) => {
  return `/api/v1/${props.entityType}/${props.entityId}/images/${image.id}.${image.attributes?.ext || 'jpg'}`
}

const getImageName = (image) => {
  // Extract filename from path or use ID if not available
  if (!image.attributes?.path) return `Image ${image.id}`

  const pathParts = image.attributes.path.split('/')
  return pathParts.length > 0 ? pathParts[pathParts.length - 1] : `Image ${image.id}`
}

const getFileName = (image) => {
  return getImageName(image)
}

const openGallery = (index) => {
  currentIndex.value = index
  showGallery.value = true
  // Reset zoom and pan when opening gallery
  resetZoom()
  // Prevent scrolling when modal is open
  document.body.style.overflow = 'hidden'
}

const closeGallery = () => {
  showGallery.value = false
  // Restore scrolling
  document.body.style.overflow = 'auto'
}

const nextImage = () => {
  if (currentIndex.value < props.images.length - 1) {
    currentIndex.value++
  } else {
    currentIndex.value = 0 // Loop back to the first image
  }
  // Reset zoom and pan when changing images
  resetZoom()
}

const prevImage = () => {
  if (currentIndex.value > 0) {
    currentIndex.value--
  } else {
    currentIndex.value = props.images.length - 1 // Loop to the last image
  }
  // Reset zoom and pan when changing images
  resetZoom()
}

// Variables to track click vs. drag
const isDragging = ref(false)
const clickStartTime = ref(0)
const clickStartPos = ref({ x: 0, y: 0 })

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
const handleImageClick = (event) => {
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
}

// Pan functions - only active when zoomed
const startPan = (event) => {
  if (isZoomed.value) {
    event.preventDefault()
    isPanning.value = true
    startX.value = event.clientX - panX.value
    startY.value = event.clientY - panY.value

    // Track for click vs. drag detection
    clickStartTime.value = Date.now()
    clickStartPos.value = { x: event.clientX, y: event.clientY }
    isDragging.value = false
  }
}

const pan = (event) => {
  if (!isPanning.value) return
  event.preventDefault()

  // Calculate distance moved
  const dx = Math.abs(event.clientX - clickStartPos.value.x)
  const dy = Math.abs(event.clientY - clickStartPos.value.y)

  // If moved more than 5px, consider it a drag
  if (dx > 5 || dy > 5) {
    isDragging.value = true
  }

  panX.value = event.clientX - startX.value
  panY.value = event.clientY - startY.value
}

const endPan = () => {
  isPanning.value = false
}

// Download functions
const downloadImage = (image) => {
  // Only emit the event, let parent handle the actual download
  emit('download', image)
}

const downloadCurrentImage = () => {
  if (!currentImage.value) return
  downloadImage(currentImage.value)
}

// Delete functions
const confirmDeleteImage = (image) => {
  // Only emit the event, let parent handle the confirmation and deletion
  emit('delete', image)
}

const confirmDeleteCurrentImage = () => {
  if (!currentImage.value) return
  confirmDeleteImage(currentImage.value)
}

// Keyboard navigation
const handleKeyDown = (event) => {
  if (!showGallery.value) return

  switch (event.key) {
    case 'Escape':
      closeGallery()
      break
    case 'ArrowLeft':
      prevImage()
      break
    case 'ArrowRight':
      nextImage()
      break
    case ' ': // Space key
      toggleZoom(event)
      break
  }
}

// Add keyboard event listener
onMounted(() => {
  window.addEventListener('keydown', handleKeyDown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeyDown)
})
</script>

<style scoped>
.image-viewer {
  margin-bottom: 1.5rem;
}

.no-images {
  padding: 1rem;
  text-align: center;
  color: #6c757d;
  background-color: #f8f9fa;
  border-radius: 8px;
}

.files-container {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 1rem;
}

.file-item {
  border: 1px solid #dee2e6;
  border-radius: 8px;
  overflow: hidden;
  background-color: white;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05);
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.file-item:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
}

.file-preview {
  height: 150px;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #f8f9fa;
  overflow: hidden;
  cursor: pointer;
}

.preview-image {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.file-info {
  padding: 0.75rem;
}

.file-name {
  font-weight: 500;
  margin-bottom: 0.5rem;
  word-break: break-word;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-actions {
  display: flex;
  gap: 0.5rem;
}

/* Modal styles */
.image-modal {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background-color: rgba(0, 0, 0, 0.9);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1000;
}

.modal-content {
  background-color: white;
  border-radius: 8px;
  width: 90%;
  max-width: 1200px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  border-bottom: 1px solid #dee2e6;
}

.modal-header h3 {
  margin: 0;
  font-size: 1.25rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 70%;
}

.modal-controls {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.close-button {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: #6c757d;
  margin-left: 0.5rem;
}

.close-button:hover {
  color: #343a40;
}

.modal-body {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
  flex: 1;
  overflow: hidden;
  position: relative;
  background-color: #f8f9fa;
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
}

.full-image.zoomed {
  max-width: none;
  max-height: none;
  width: 200%;
  height: auto;
  object-fit: cover;
}

.nav-button {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  background-color: rgba(255, 255, 255, 0.5);
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
}

.nav-button:hover {
  background-color: rgba(255, 255, 255, 0.8);
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
  border-top: 1px solid #dee2e6;
}

.image-counter {
  color: #6c757d;
}

.image-actions {
  display: flex;
  gap: 0.5rem;
}

.btn-primary {
  background-color: #4CAF50;
  color: white;
  border: none;
  cursor: pointer;
}

.btn-danger {
  background-color: #dc3545;
  color: white;
  border: none;
  cursor: pointer;
}

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
  border-radius: 4px;
}
</style>
