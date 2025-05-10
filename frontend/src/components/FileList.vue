<template>
  <div class="file-list">
    <div v-if="loading" class="loading">Loading files...</div>
    <div v-else-if="files.length === 0" class="no-files">
      No {{ fileType }} uploaded yet.
    </div>
    <div v-else class="files-container">
      <div v-for="file in files" :key="file.id" class="file-item">
        <div class="file-preview" v-if="fileType === 'images' && file.path">
          <img :src="getFileUrl(file)" alt="Preview" class="preview-image" />
        </div>
        <div class="file-icon" v-else>
          <i :class="getFileIcon(file)"></i>
        </div>
        <div class="file-info">
          <div class="file-name">{{ getFileName(file) }}</div>
          <div class="file-actions">
            <button class="btn btn-sm btn-primary" @click="downloadFile(file)">
              <i class="fas fa-download"></i> Download
            </button>
            <button class="btn btn-sm btn-danger" @click="confirmDelete(file)">
              <i class="fas fa-trash"></i> Delete
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { defineProps, defineEmits } from 'vue'

const props = defineProps({
  files: {
    type: Array,
    required: true,
    default: () => []
  },
  fileType: {
    type: String,
    required: true,
    validator: (value: string) => ['images', 'manuals', 'invoices'].includes(value)
  },
  commodityId: {
    type: String,
    required: true
  },
  loading: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['delete', 'download'])

const getFileUrl = (file: any) => {
  if (props.fileType === 'images') {
    return `/api/v1/commodities/${props.commodityId}/images/${file.id}.${file.ext}`
  } else if (props.fileType === 'manuals') {
    return `/api/v1/commodities/${props.commodityId}/manuals/${file.id}.${file.ext}`
  } else if (props.fileType === 'invoices') {
    return `/api/v1/commodities/${props.commodityId}/invoices/${file.id}.${file.ext}`
  }
  return ''
}

const getFileName = (file: any) => {
  // Extract filename from path or use ID if not available
  const pathParts = file.path ? file.path.split('/') : []
  return pathParts.length > 0 ? pathParts[pathParts.length - 1] : `${file.id}.${file.ext}`
}

const getFileIcon = (file: any) => {
  if (props.fileType === 'manuals') {
    return 'fas fa-file-pdf'
  } else if (props.fileType === 'invoices') {
    return 'fas fa-file-invoice-dollar'
  }
  return 'fas fa-file'
}

const downloadFile = (file: any) => {
  emit('download', file)

  // Create a link and trigger download
  const link = document.createElement('a')
  link.href = getFileUrl(file)
  link.download = getFileName(file)
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

const confirmDelete = (file: any) => {
  if (confirm(`Are you sure you want to delete this ${props.fileType.slice(0, -1)}?`)) {
    emit('delete', file)
  }
}
</script>

<style scoped>
.file-list {
  margin-bottom: 1.5rem;
}

.loading, .no-files {
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
}

.preview-image {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.file-icon {
  height: 150px;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #f8f9fa;
  font-size: 3rem;
  color: #6c757d;
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

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.75rem;
}
</style>
