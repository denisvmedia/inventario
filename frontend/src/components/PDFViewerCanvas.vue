<template>
  <div class="pdf-viewer-container">
    <div v-if="loading" class="pdf-loading">
      <div class="spinner"></div>
      <p>Loading PDF...</p>
    </div>
    <div v-else-if="error" class="pdf-error">
      {{ error }}
      <div class="error-actions">
        <button class="btn btn-primary" @click="downloadPDF">Download PDF</button>
      </div>
    </div>
    <div v-else class="pdf-view">
      <div class="pdf-controls">
        <div class="pdf-navigation">
          <button
            class="btn btn-sm"
            @click="prevPage"
            :disabled="currentPage <= 1"
            title="Previous Page"
          >
            <i class="fas fa-chevron-left"></i>
          </button>
          <span class="page-info">{{ currentPage }} / {{ numPages }}</span>
          <button
            class="btn btn-sm"
            @click="nextPage"
            :disabled="currentPage >= numPages"
            title="Next Page"
          >
            <i class="fas fa-chevron-right"></i>
          </button>
        </div>
        <div class="pdf-zoom">
          <button class="btn btn-sm" @click="zoomOut" title="Zoom Out">
            <i class="fas fa-search-minus"></i>
          </button>
          <span class="zoom-level">{{ Math.round(scale * 100) }}%</span>
          <button class="btn btn-sm" @click="zoomIn" title="Zoom In">
            <i class="fas fa-search-plus"></i>
          </button>
        </div>
        <button class="btn btn-sm btn-primary" @click="downloadPDF" title="Download PDF">
          <i class="fas fa-download"></i>
        </button>
      </div>
      <div class="pdf-container" ref="pdfContainer">
        <div v-for="n in renderedPages" :key="n" class="pdf-page-container">
          <img v-if="pageImages[n]" :src="pageImages[n]" class="pdf-page" />
          <div v-else class="pdf-page-loading">
            <div class="spinner small"></div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, watch, markRaw, computed } from 'vue'
import { pdfjsLib } from '../utils/pdfjs-init.js'

const props = defineProps({
  url: {
    type: String,
    required: true
  }
})

const emit = defineEmits(['error', 'loading'])

// PDF state
const loading = ref(true)
const error = ref(null)
const pdfContainer = ref(null)
const pdfDoc = ref(null)
const currentPage = ref(1)
const numPages = ref(0)
const scale = ref(1.5)
const pageImages = ref({})
const renderedPages = ref(0)
const isRendering = ref(false)
const isMounted = ref(false)

// Load the PDF document
const loadPDF = async () => {
  if (!props.url || props.url === '') {
    error.value = 'Invalid PDF URL'
    loading.value = false
    emit('loading', false)
    return
  }

  // Reset state
  loading.value = true
  emit('loading', true)
  error.value = null
  isRendering.value = false
  pdfDoc.value = null
  numPages.value = 0
  pageImages.value = {}
  renderedPages.value = 0

  try {
    console.log('Loading PDF from URL:', props.url)

    // Add a timeout to ensure we don't get stuck in loading state
    const timeoutPromise = new Promise((_, reject) => {
      setTimeout(() => reject(new Error('PDF loading timeout')), 15000)
    })

    // Load the PDF document
    const loadingTask = pdfjsLib.getDocument({
      url: props.url,
      cMapUrl: '/cmaps/',
      cMapPacked: true,
    })

    // Race between loading and timeout
    const pdf = await Promise.race([
      loadingTask.promise,
      timeoutPromise
    ])

    // Use markRaw to prevent Vue from making the PDF object reactive
    pdfDoc.value = markRaw(pdf)
    numPages.value = pdf.numPages

    // Render the first page
    renderedPages.value = 1
    await renderPage(currentPage.value)

    loading.value = false
    emit('loading', false)
  } catch (err) {
    console.error('Error loading PDF:', err)
    error.value = 'Failed to load PDF. Please try downloading the file instead.'
    loading.value = false
    emit('loading', false)
    // Emit error to parent component
    emit('error', err)
  }
}

// Render a specific page and convert it to an image
const renderPage = async (pageNum) => {
  if (!pdfDoc.value) return

  // If already rendering, don't start another render operation
  if (isRendering.value) {
    console.log('Render operation already in progress, skipping')
    return
  }

  try {
    isRendering.value = true

    // Get the page
    const page = markRaw(await pdfDoc.value.getPage(pageNum))
    const viewport = markRaw(page.getViewport({ scale: scale.value }))

    // Create a canvas for rendering
    const canvas = document.createElement('canvas')
    canvas.width = viewport.width
    canvas.height = viewport.height

    const context = canvas.getContext('2d')

    const renderContext = {
      canvasContext: context,
      viewport: viewport
    }

    // Render the page to the canvas
    const renderTask = markRaw(page.render(renderContext))
    await renderTask.promise

    // Convert the canvas to an image data URL
    const imageUrl = canvas.toDataURL('image/png')

    // Store the image URL
    pageImages.value[pageNum] = imageUrl

    isRendering.value = false
  } catch (err) {
    console.error('Error rendering page:', err)
    error.value = 'Failed to render PDF page. Please try downloading the file instead.'
    isRendering.value = false
    // Emit error to parent component
    emit('error', err)
  }
}

// Navigation functions
const prevPage = async () => {
  if (isRendering.value) return // Don't navigate if already rendering

  if (currentPage.value > 1) {
    currentPage.value--

    // If this page hasn't been rendered yet, render it
    if (!pageImages.value[currentPage.value]) {
      await renderPage(currentPage.value)
    }
  }
}

const nextPage = async () => {
  if (isRendering.value) return // Don't navigate if already rendering

  if (currentPage.value < numPages.value) {
    currentPage.value++

    // If we're showing a new page that hasn't been rendered yet
    if (!pageImages.value[currentPage.value]) {
      // Update rendered pages count if needed
      if (currentPage.value > renderedPages.value) {
        renderedPages.value = currentPage.value
      }

      // Render the page
      await renderPage(currentPage.value)
    }
  }
}

// Zoom functions
const zoomIn = async () => {
  if (isRendering.value) return // Don't zoom if already rendering

  // Calculate new scale
  scale.value = Math.min(scale.value + 0.25, 3.0) // Max zoom 3x

  // Clear all rendered pages and re-render at new scale
  pageImages.value = {}
  await renderPage(currentPage.value)
}

const zoomOut = async () => {
  if (isRendering.value) return // Don't zoom if already rendering

  // Calculate new scale
  scale.value = Math.max(scale.value - 0.25, 0.75) // Min zoom 0.75x

  // Clear all rendered pages and re-render at new scale
  pageImages.value = {}
  await renderPage(currentPage.value)
}

// Download PDF function
const downloadPDF = () => {
  const link = document.createElement('a')
  link.href = props.url
  link.download = props.url.split('/').pop() || 'document.pdf'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

// Watch for URL changes
watch(() => props.url, (newUrl, oldUrl) => {
  if (newUrl && newUrl !== oldUrl) {
    // Cancel any ongoing rendering
    isRendering.value = false
    // Reset state before loading new PDF
    pdfDoc.value = null
    numPages.value = 0
    currentPage.value = 1
    scale.value = 1.5
    pageImages.value = {}
    renderedPages.value = 0

    // Only load if component is mounted
    if (isMounted.value) {
      // Load the new PDF
      loadPDF()
    } else {
      // Delay loading until next tick to give component time to initialize
      setTimeout(() => {
        if (isMounted.value) {
          loadPDF()
        } else {
          console.error('Component not mounted for URL change')
          error.value = 'PDF viewer is not available. Please try downloading the file instead.'
          loading.value = false
          emit('loading', false)
          emit('error', new Error('Component not mounted for URL change'))
        }
      }, 200)
    }
  }
}, { immediate: false })

// Initialize
onMounted(() => {
  // Mark component as mounted
  isMounted.value = true

  // Delay loading slightly to ensure component is properly set up
  setTimeout(() => {
    if (props.url && isMounted.value) {
      loadPDF()
    }
  }, 200) // Increased delay for more reliable mounting
})

// Clean up when component is unmounted
onBeforeUnmount(() => {
  // Mark component as unmounted
  isMounted.value = false

  // Cancel any ongoing rendering
  isRendering.value = false

  // Clear references
  pdfDoc.value = null
  pageImages.value = {}
  renderedPages.value = 0
})
</script>

<style scoped>
.pdf-viewer-container {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  background-color: #f8f9fa;
  border-radius: 4px;
  overflow: hidden;
}

.pdf-loading, .pdf-error {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  height: 200px;
  color: #6c757d;
  padding: 1rem;
  text-align: center;
}

.pdf-loading {
  height: 300px;
}

.spinner {
  width: 40px;
  height: 40px;
  border: 4px solid rgba(0, 0, 0, 0.1);
  border-radius: 50%;
  border-top-color: #4CAF50;
  animation: spin 1s ease-in-out infinite;
}

.spinner.small {
  width: 20px;
  height: 20px;
  border-width: 2px;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.pdf-loading p {
  margin-top: 1rem;
}

.error-actions {
  margin-top: 1rem;
}

.pdf-view {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 100%;
  height: 100%;
}

.pdf-controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
  padding: 0.75rem;
  background-color: #e9ecef;
  border-bottom: 1px solid #dee2e6;
}

.pdf-navigation, .pdf-zoom {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.page-info, .zoom-level {
  font-size: 0.875rem;
  color: #495057;
  min-width: 60px;
  text-align: center;
}

.pdf-container {
  overflow: auto;
  max-height: 600px;
  margin: 1rem 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  background-color: #e9ecef;
  box-shadow: inset 0 0 10px rgba(0, 0, 0, 0.1);
  padding: 1rem;
  width: 100%;
}

.pdf-page-container {
  margin-bottom: 1rem;
  background-color: white;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  position: relative;
}

.pdf-page {
  display: block;
  max-width: 100%;
  height: auto;
}

.pdf-page-loading {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 200px;
  background-color: white;
  width: 100%;
}

.btn {
  background-color: #fff;
  border: 1px solid #ced4da;
  color: #495057;
  cursor: pointer;
  border-radius: 4px;
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
}

.btn:hover:not(:disabled) {
  background-color: #e9ecef;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
}

.btn-primary {
  background-color: #4CAF50;
  color: white;
  border: none;
}

.btn-primary:hover {
  background-color: #45a049;
}
</style>
