<template>
  <div class="pdf-viewer-container">
    <div v-if="loading" class="pdf-loading">
      Loading PDF...
    </div>
    <div v-else-if="error" class="pdf-error">
      {{ error }}
      <div class="error-actions">
        <button class="btn btn-primary" @click="downloadPDF">Download PDF</button>
      </div>
    </div>
    <div v-else>
      <!-- Fallback to iframe for more reliable rendering -->
      <iframe
        :src="pdfViewerUrl"
        class="pdf-iframe"
        frameborder="0"
        title="PDF Viewer"
      ></iframe>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, watch, computed } from 'vue'

const props = defineProps({
  url: {
    type: String,
    required: true
  }
})

// PDF state
const loading = ref(true)
const error = ref(null)

// Use PDF.js viewer URL with our PDF URL as a parameter
const pdfViewerUrl = computed(() => {
  if (!props.url) return ''

  // Use PDF.js viewer from CDN
  const pdfJsViewerUrl = 'https://mozilla.github.io/pdf.js/web/viewer.html'
  const encodedPdfUrl = encodeURIComponent(props.url)
  return `${pdfJsViewerUrl}?file=${encodedPdfUrl}`
})

// Check if the URL is valid
const checkUrl = () => {
  loading.value = true
  error.value = null

  if (!props.url || props.url === '') {
    error.value = 'Invalid PDF URL'
    loading.value = false
    return false
  }

  // Simple check to see if the URL ends with .pdf
  if (!props.url.toLowerCase().endsWith('.pdf')) {
    console.warn('URL does not end with .pdf, but will try to load anyway:', props.url)
  }

  loading.value = false
  return true
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
watch(() => props.url, (newUrl) => {
  if (newUrl) {
    checkUrl()
  }
}, { immediate: true })

// Initialize
onMounted(() => {
  checkUrl()
})
</script>

<style lang="scss" scoped>
@import '../assets/main.scss';

.pdf-viewer-container {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  background-color: $light-bg-color;
  border-radius: $default-radius;
  overflow: hidden;
}

.pdf-loading, .pdf-error {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  height: 200px;
  color: $secondary-color;
  padding: 1rem;
  text-align: center;
}

.error-actions {
  margin-top: 1rem;
}

.pdf-iframe {
  width: 100%;
  height: 600px;
  border: none;
}

.btn {
  background-color: $primary-color;
  color: white;
  border: none;
  cursor: pointer;
  border-radius: $default-radius;
  padding: 0.5rem 1rem;
  font-size: 0.875rem;

  &:hover {
    background-color: $primary-hover-color;
  }
}
</style>
