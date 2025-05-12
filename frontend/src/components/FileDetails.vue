<template>
  <div class="file-details-overlay" v-if="file" @click.self="close">
    <div class="file-details-modal">
      <div class="file-details-header">
        <h3>File Details</h3>
        <button class="close-button" @click="close">
          <font-awesome-icon icon="times" />
        </button>
      </div>

      <div class="file-details-content">
        <!-- Preview section -->
        <div class="file-preview-section">
          <div v-if="isImageFile" class="image-preview">
            <img :src="fileUrl" alt="Image preview" />
          </div>
          <div v-else class="file-icon-preview">
            <font-awesome-icon :icon="getFileIcon()" size="5x" />
          </div>
        </div>

        <!-- Details section -->
        <div class="file-info-section">
          <div class="file-info-item">
            <div class="info-label">ID:</div>
            <div class="info-value">{{ file.id }}</div>
          </div>

          <div class="file-info-item">
            <div class="info-label">File Name:</div>
            <div class="info-value">{{ file.path }}{{ file.ext }}</div>
          </div>

          <div class="file-info-item">
            <div class="info-label">Original Name:</div>
            <div class="info-value">{{ file.original_path }}</div>
          </div>

          <div class="file-info-item">
            <div class="info-label">Object Type:</div>
            <div class="info-value">{{ objectType }}</div>
          </div>

          <div class="file-info-item">
            <div class="info-label">File Type:</div>
            <div class="info-value">{{ file.mime_type }}</div>
          </div>

          <div class="file-info-item">
            <div class="info-label">Extension:</div>
            <div class="info-value">{{ file.ext }}</div>
          </div>
        </div>
      </div>

      <div class="file-details-actions">
        <button class="btn btn-primary" @click="downloadFile">
          <font-awesome-icon icon="download" /> Download
        </button>
        <button class="btn btn-danger" @click="confirmDelete">
          <font-awesome-icon icon="trash" /> Delete
        </button>
        <button class="btn btn-secondary" @click="close">
          <font-awesome-icon icon="times" /> Close
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount } from 'vue'

const props = defineProps({
  file: {
    type: Object,
    required: true
  },
  fileType: {
    type: String,
    required: true,
    validator: (value: string) => ['images', 'manuals', 'invoices'].includes(value)
  },
  commodityId: {
    type: String,
    required: true
  }
})

const emit = defineEmits(['close', 'delete', 'download'])

const fileUrl = computed(() => {
  if (!props.file) return ''

  if (props.fileType === 'images') {
    return `/api/v1/commodities/${props.commodityId}/images/${props.file.id}${props.file.ext}`
  } else if (props.fileType === 'manuals') {
    return `/api/v1/commodities/${props.commodityId}/manuals/${props.file.id}${props.file.ext}`
  } else if (props.fileType === 'invoices') {
    return `/api/v1/commodities/${props.commodityId}/invoices/${props.file.id}${props.file.ext}`
  }
  return ''
})

const isImageFile = computed(() => {
  if (!props.file) return false
  const imageExtensions = ['jpg', 'jpeg', 'png', 'gif', 'webp']

  // Check file extension
  if (props.file.ext) {
    const ext = props.file.ext.toLowerCase().replace('.', '')
    return imageExtensions.includes(ext)
  }

  // Check mime type if available
  if (props.file.mime_type && props.file.mime_type.startsWith('image/')) {
    return true
  }

  return false
})

const isPdfFile = computed(() => {
  if (!props.file) return false

  // Check file extension
  if (props.file.ext) {
    return props.file.ext.toLowerCase() === '.pdf' || props.file.ext.toLowerCase() === 'pdf'
  }

  // Check mime type if available
  if (props.file.mime_type && props.file.mime_type === 'application/pdf') {
    return true
  }

  return false
})

const objectType = computed(() => {
  if (isImageFile.value) return 'Image'
  if (isPdfFile.value) return 'PDF'
  return 'File'
})

const getFileIcon = () => {
  if (isPdfFile.value) {
    return 'file-pdf'
  } else if (isImageFile.value) {
    return 'file-image'
  } else if (props.fileType === 'manuals') {
    return 'book'
  } else if (props.fileType === 'invoices') {
    return 'file-invoice-dollar'
  }
  return 'file'
}

const close = () => {
  emit('close')
}

// Handle keyboard events
const handleKeyDown = (event) => {
  if (event.key === 'Escape') {
    close()
  }
}

// Add keyboard event listener when component is mounted
onMounted(() => {
  window.addEventListener('keydown', handleKeyDown)
})

// Remove keyboard event listener when component is unmounted
onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeyDown)
})

const downloadFile = () => {
  // Only emit the event, let parent handle the actual download
  emit('download', props.file)
}

const confirmDelete = () => {
  // Only emit the event, let parent handle the confirmation and deletion
  emit('delete', props.file)
  // Don't close immediately, let the parent decide when to close
}
</script>

<style lang="scss" scoped>
@import '../assets/main.scss';

.file-details-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.file-details-modal {
  background-color: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
  width: 90%;
  max-width: 800px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.file-details-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  border-bottom: 1px solid $border-color;

  h3 {
    margin: 0;
    font-size: 1.25rem;
  }
}

.close-button {
  background: none;
  border: none;
  font-size: 1.25rem;
  cursor: pointer;
  color: $secondary-color;

  &:hover {
    color: darken($secondary-color, 10%);
  }
}

.file-details-content {
  display: flex;
  flex-direction: column;
  padding: 1rem;
  overflow-y: auto;
  flex: 1;

  @media (min-width: 768px) {
    flex-direction: row;
  }
}

.file-preview-section {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
  background-color: $light-bg-color;
  border-radius: $default-radius;
  margin-bottom: 1rem;

  @media (min-width: 768px) {
    margin-right: 1rem;
    margin-bottom: 0;
  }
}

.image-preview {
  img {
    max-width: 100%;
    max-height: 300px;
    object-fit: contain;
  }
}

.pdf-preview {
  width: 100%;
  height: 400px;

  iframe {
    width: 100%;
    height: 100%;
    border: none;
  }
}

.file-icon-preview {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 200px;
  color: $secondary-color;
}

.file-info-section {
  flex: 1;
}

.file-info-item {
  margin-bottom: 1rem;
  display: flex;
  flex-direction: column;
}

.info-label {
  font-weight: 600;
  color: $text-color;
  margin-bottom: 0.25rem;
}

.info-value {
  word-break: break-all;
}

.file-details-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  padding: 1rem;
  border-top: 1px solid $border-color;
}

.btn {
  padding: 0.375rem 0.75rem;
  border-radius: $default-radius;
  font-weight: 500;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  border: none;

  &-primary {
    background-color: $primary-color;
    color: white;

    &:hover {
      background-color: $primary-hover-color;
    }
  }

  &-danger {
    background-color: $danger-color;
    color: white;

    &:hover {
      background-color: $danger-hover-color;
    }
  }

  &-secondary {
    background-color: $secondary-color;
    color: white;

    &:hover {
      background-color: darken($secondary-color, 10%);
    }
  }
}
</style>
