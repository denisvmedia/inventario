<template>
  <div class="file-list">
    <div v-if="loading" class="loading">Loading files...</div>
    <div v-else-if="files.length === 0" class="no-files">
      No {{ fileType }} uploaded yet.
    </div>
    <div v-else class="files-container">
      <div v-for="file in files" :key="file.id" class="file-item">
        <div v-if="isImageFile(file)" class="file-preview" @click="openViewer(file)">
          <img :src="getFileUrl(file)" alt="Preview" class="preview-image" />
        </div>
        <div v-else class="file-icon" @click="openViewer(file)">
          <font-awesome-icon :icon="getFileIcon(file)" size="3x" />
        </div>
        <div class="file-info">
          <div class="file-name-container">
            <div v-if="editingFile !== file.id" class="file-name" @click="startEditing(file)">
              {{ getFileName(file) }}
              <font-awesome-icon icon="pencil-alt" class="edit-icon" />
            </div>
            <div v-else class="file-name-edit">
              <input
                ref="fileNameInput"
                v-model="editedFileName"
                type="text"
                @keyup.enter="saveFileName(file)"
                @keyup.esc="cancelEditing"
              />
              <div class="edit-actions">
                <button class="btn btn-sm btn-success" @click="saveFileName(file)">
                  <font-awesome-icon icon="check" />
                </button>
                <button class="btn btn-sm btn-secondary" @click="cancelEditing">
                  <font-awesome-icon icon="times" />
                </button>
              </div>
            </div>
          </div>
          <div class="file-actions">
            <button class="btn btn-sm btn-primary" @click="downloadFile(file)">
              <font-awesome-icon icon="download" /> Download
            </button>
            <button class="btn btn-sm btn-danger" @click="confirmDelete(file)">
              <font-awesome-icon icon="trash" /> Delete
            </button>
            <button class="btn btn-sm btn-info" @click="viewFileDetails(file)">
              <font-awesome-icon icon="info-circle" /> Details
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick } from 'vue'

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

const emit = defineEmits(['delete', 'download', 'update', 'view-details', 'open-viewer'])

const getFileUrl = (file: any) => {
  if (props.fileType === 'images') {
    return `/api/v1/commodities/${props.commodityId}/images/${file.id}${file.ext}`
  } else if (props.fileType === 'manuals') {
    return `/api/v1/commodities/${props.commodityId}/manuals/${file.id}${file.ext}`
  } else if (props.fileType === 'invoices') {
    return `/api/v1/commodities/${props.commodityId}/invoices/${file.id}${file.ext}`
  }
  return ''
}

const getFileName = (file: any) => {
  // Use the Path field directly (it's now just the filename without extension)
  // and add the extension from the ext field
  if (file.path) {
    return file.path + file.ext
  }
  // Fallback to ID with extension if path is not available
  return `${file.id}${file.ext}`
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

const downloadFile = (file: any) => {
  // Only emit the event, let parent handle the actual download
  emit('download', file)
}

const confirmDelete = (file: any) => {
  // Only emit the event, let parent handle the confirmation and deletion
  emit('delete', file)
}

// File name editing functionality
const editingFile = ref<string | null>(null)
const editedFileName = ref('')
const fileNameInput = ref<HTMLInputElement | null>(null)

const startEditing = (file: any) => {
  editingFile.value = file.id
  // Set the initial value to the path (without extension)
  editedFileName.value = file.path

  // Focus the input field after the DOM updates
  nextTick(() => {
    if (fileNameInput.value) {
      fileNameInput.value.focus()
    }
  })
}

const cancelEditing = () => {
  editingFile.value = null
  editedFileName.value = ''
}

const saveFileName = async (file: any) => {
  if (!editedFileName.value.trim()) {
    alert('File name cannot be empty')
    return
  }

  try {
    // Emit the update event with the file ID and new path
    emit('update', {
      id: file.id,
      type: props.fileType,
      path: editedFileName.value
    })

    // Reset the editing state
    editingFile.value = null
    editedFileName.value = ''
  } catch (error) {
    console.error('Error updating file name:', error)
    alert('Failed to update file name')
  }
}

const viewFileDetails = (file: any) => {
  emit('view-details', file)
}

const openViewer = (file: any) => {
  emit('open-viewer', file)
}

// Helper functions to detect file types
const isImageFile = (file: any) => {
  if (!file) return false
  const imageExtensions = ['jpg', 'jpeg', 'png', 'gif', 'webp']

  // Check file extension
  if (file.ext) {
    const ext = file.ext.toLowerCase().replace('.', '')
    return imageExtensions.includes(ext)
  }

  // Check mime type if available
  if (file.mime_type && file.mime_type.startsWith('image/')) {
    return true
  }

  return false
}

const isPdfFile = (file: any) => {
  if (!file) return false

  // Check file extension
  if (file.ext) {
    return file.ext.toLowerCase() === '.pdf' || file.ext.toLowerCase() === 'pdf'
  }

  // Check mime type if available
  if (file.mime_type && file.mime_type === 'application/pdf') {
    return true
  }

  return false
}
</script>

<style lang="scss" scoped>
@import '../assets/main.scss';

.file-list {
  margin-bottom: 1.5rem;
}

.loading, .no-files {
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
    box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
  }
}

.file-preview {
  height: 150px;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: $light-bg-color;
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
  background-color: $light-bg-color;
  font-size: 3rem;
  color: $secondary-color;
}

.file-info {
  padding: 0.75rem;
}

.file-name-container {
  margin-bottom: 0.5rem;
}

.file-name {
  font-weight: 500;
  word-break: break-word;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
  display: flex;
  justify-content: space-between;
  align-items: center;

  &:hover .edit-icon {
    opacity: 1;
  }
}

.edit-icon {
  font-size: 0.8rem;
  margin-left: 0.5rem;
  color: $secondary-color;
  opacity: 0;
  transition: opacity 0.2s ease;
}

.file-name-edit {
  display: flex;
  align-items: center;

  input {
    flex: 1;
    padding: 0.25rem 0.5rem;
    border: 1px solid $border-color;
    border-radius: $default-radius;
    font-size: 0.9rem;
  }
}

.edit-actions {
  display: flex;
  gap: 0.25rem;
  margin-left: 0.5rem;
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
