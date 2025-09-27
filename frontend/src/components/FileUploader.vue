<template>
  <div class="file-uploader">
    <div class="upload-area"
         :class="{ 'drag-over': isDragOver, 'has-file': selectedFiles.length > 0 }"
         @dragover.prevent="onDragOver"
         @dragleave.prevent="onDragLeave"
         @drop.prevent="onDrop"
         @click="triggerFileInput">
      <input
        ref="fileInput"
        type="file"
        :multiple="multiple"
        :accept="accept"
        class="file-input"
        @change="onFileSelected"
      />

      <div v-if="selectedFiles.length === 0" class="upload-content-inner">
        <div class="upload-icon">
          <FontAwesomeIcon icon="cloud-upload-alt" />
        </div>
        <p class="upload-text">
          <span class="upload-prompt">{{ uploadPrompt }}</span>
          <span class="upload-or">or</span>
          <span class="browse-button">click to browse</span>
        </p>
        <p class="upload-hint">{{ uploadHint }}</p>
      </div>

      <div v-else class="selected-files-content">
        <div class="selected-files-list">
          <div v-for="(file, index) in selectedFiles" :key="index" class="selected-file">
            <div class="file-preview">
              <img
                v-if="getFilePreview(file) && file.type.startsWith('image/')"
                :src="getFilePreview(file)"
                :alt="file.name"
                class="file-thumbnail"
              />
              <div v-else class="file-icon">
                <font-awesome-icon :icon="getFileIcon(file)" />
              </div>
            </div>
            <div class="file-info">
              <h3>{{ file.name }}</h3>
              <p>{{ formatFileSize(file.size) }} • {{ file.type || 'Unknown type' }}</p>
            </div>
            <button class="btn-remove" title="Remove file" @click.stop="removeFile(index)">
              <font-awesome-icon icon="times" />
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Upload Capacity Error -->
    <div v-if="uploadCapacityError" class="upload-capacity-error">
      <font-awesome-icon icon="exclamation-triangle" />
      {{ uploadCapacityError }}
    </div>

    <!-- Upload actions outside the file block -->
    <transition name="upload-actions" mode="out-in">
      <div v-if="selectedFiles.length > 0 && !uploadCompleted && !hideUploadButton" class="upload-actions">
        <!-- Upload Capacity Status -->
        <div v-if="requireSlots && hasUploadCapacity" class="slot-status">
          <font-awesome-icon icon="check-circle" />
          Upload capacity available
        </div>

        <!-- Upload Progress Bar -->
        <div v-if="isUploading && uploadProgress.total > 0" class="upload-progress">
          <div class="progress-info">
            <span class="progress-text">
              Uploading {{ uploadProgress.current }} of {{ uploadProgress.total }} files...
            </span>
            <span class="progress-percentage">{{ Math.round(uploadProgress.percentage) }}%</span>
          </div>
          <div class="progress-bar">
            <div
              class="progress-fill"
              :style="{ width: uploadProgress.percentage + '%' }"
            ></div>
          </div>
          <div v-if="uploadProgress.currentFile" class="current-file">
            {{ uploadProgress.currentFile }}
          </div>
        </div>

        <!-- Upload Button -->
        <button
          ref="uploadButton"
          type="button"
          class="btn btn-primary"
          :disabled="isUploading"
          @click="uploadFiles"
        >
          <font-awesome-icon v-if="isUploading" icon="spinner" spin />
          <font-awesome-icon v-else icon="upload" />
          {{ isUploading ? 'Uploading...' : 'Upload Files' }}
        </button>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import uploadSlotService from '@/services/uploadSlotService'

const props = defineProps({
  multiple: {
    type: Boolean,
    default: true
  },
  accept: {
    type: String,
    default: '*/*'
  },
  uploadPrompt: {
    type: String,
    default: 'Drag and drop files here'
  },
  uploadHint: {
    type: String,
    default: 'Supports XML files and other document formats'
  },
  hideUploadButton: {
    type: Boolean,
    default: false
  },
  operationName: {
    type: String,
    default: 'file_upload'
  },
  requireSlots: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits(['upload', 'filesCleared', 'filesSelected', 'uploadCapacityFailed'])

const fileInput = ref<HTMLInputElement | null>(null)
const uploadButton = ref<HTMLButtonElement | null>(null)
const selectedFiles = ref<File[]>([])
const filePreviews = ref<{ [key: string]: string }>({}) // Store file previews by file name + size
const isDragOver = ref(false)
const isUploading = ref(false)
const uploadCompleted = ref(false)

// Upload progress tracking
// TODO: Implement per-file progress tracking with the following features:
// - Individual progress for each file in the queue
// - Ability to cancel files that are still in queue (not yet started)
// - Automatic removal of files from the list once they are successfully uploaded
// - Visual indicators for: queued, uploading, completed, failed, cancelled states
// - Cancel button for each file that's still in queue or currently uploading
// - Retry functionality for failed uploads
const uploadProgress = ref({
  current: 0,
  total: 0,
  percentage: 0,
  currentFile: ''
})

// Upload capacity tracking
const uploadCapacityError = ref<string | null>(null)
const isCheckingCapacity = ref(false)

// Computed properties
const hasUploadCapacity = computed(() => {
  return !uploadCapacityError.value && !isCheckingCapacity.value
})

const triggerFileInput = () => {
  fileInput.value?.click()
}

const onDragOver = () => {
  isDragOver.value = true
}

const onDragLeave = () => {
  isDragOver.value = false
}

const onDrop = (event: DragEvent) => {
  isDragOver.value = false
  if (event.dataTransfer?.files) {
    addFiles(Array.from(event.dataTransfer.files))
  }
}

const onFileSelected = (event: Event) => {
  const input = event.target as HTMLInputElement
  if (input.files) {
    addFiles(Array.from(input.files))
    // Reset the input so the same file can be selected again
    input.value = ''
  }
}

const addFiles = (files: File[]) => {
  if (props.multiple) {
    selectedFiles.value = [...selectedFiles.value, ...files]
  } else {
    selectedFiles.value = [files[0]]
  }

  // Generate previews for image files
  files.forEach(file => {
    if (file.type.startsWith('image/')) {
      const fileKey = `${file.name}_${file.size}`
      const reader = new FileReader()
      reader.onload = (e) => {
        filePreviews.value[fileKey] = e.target?.result as string
      }
      reader.readAsDataURL(file)
    }
  })

  // Reset upload completed state when new files are added
  uploadCompleted.value = false

  // Emit filesSelected event when files are added
  emit('filesSelected', selectedFiles.value)
}

const removeFile = (index: number) => {
  const file = selectedFiles.value[index]
  if (file) {
    // Remove preview for this file
    const fileKey = `${file.name}_${file.size}`
    delete filePreviews.value[fileKey]
  }

  selectedFiles.value.splice(index, 1)
  // Reset upload completed state when files are removed
  uploadCompleted.value = false

  // Emit filesCleared event when all files are removed
  if (selectedFiles.value.length === 0) {
    emit('filesCleared')
  }
}



const markUploadCompleted = () => {
  isUploading.value = false
  uploadCompleted.value = true
  // Reset progress
  uploadProgress.value = {
    current: 0,
    total: 0,
    percentage: 0,
    currentFile: ''
  }
}

const markUploadFailed = () => {
  isUploading.value = false
  uploadCompleted.value = false
  // Reset progress
  uploadProgress.value = {
    current: 0,
    total: 0,
    percentage: 0,
    currentFile: ''
  }
}

// Progress tracking methods
const updateProgress = (current: number, total: number, currentFile: string = '') => {
  uploadProgress.value = {
    current,
    total,
    percentage: total > 0 ? (current / total) * 100 : 0,
    currentFile
  }
}

const resetProgress = () => {
  uploadProgress.value = {
    current: 0,
    total: 0,
    percentage: 0,
    currentFile: ''
  }
}

// Clear upload capacity error when clearing files
const clearFiles = () => {
  selectedFiles.value = []
  filePreviews.value = {} // Clear all previews
  uploadCompleted.value = false
  isUploading.value = false
  uploadCapacityError.value = null
  emit('filesCleared')
}

// Expose methods for parent component
defineExpose({
  clearFiles,
  markUploadCompleted,
  markUploadFailed,
  updateProgress,
  resetProgress,
  getUploadButton: () => uploadButton.value,
  selectedFiles
})

const uploadFiles = async () => {
  if (selectedFiles.value.length === 0) return

  isUploading.value = true
  uploadCapacityError.value = null

  try {
    // Check upload capacity if required
    if (props.requireSlots) {
      console.log(`🎫 Checking upload capacity for ${props.operationName}`)
      isCheckingCapacity.value = true

      try {
        const capacityResponse = await uploadSlotService.waitForCapacity(
          props.operationName,
          3, // maxRetries
          1000 // baseDelay
        )

        if (!capacityResponse.data.attributes.can_start_upload) {
          throw new Error('Upload capacity not available')
        }

        console.log(`✅ Upload capacity available for ${props.operationName}`)
      } catch (capacityError: unknown) {
        console.error('❌ Failed to get upload capacity:', capacityError)
        const errorMessage = capacityError && typeof capacityError === 'object' && 'message' in capacityError ? (capacityError as Error).message : 'Upload capacity not available'
        uploadCapacityError.value = errorMessage
        isUploading.value = false
        isCheckingCapacity.value = false
        emit('uploadCapacityFailed', capacityError)
        return
      } finally {
        isCheckingCapacity.value = false
      }
    }

    // Proceed with upload
    emit('upload', selectedFiles.value, [])
    // Don't set uploadCompleted here - wait for parent to signal completion
  } catch (error) {
    console.error('Upload failed:', error)
    isUploading.value = false
    uploadCapacityError.value = null
  }
}

const getFilePreview = (file: File): string | null => {
  const fileKey = `${file.name}_${file.size}`
  return filePreviews.value[fileKey] || null
}

const getFileIcon = (file: File): string => {
  const type = file.type
  if (type.startsWith('image/')) return 'image'
  if (type.startsWith('video/')) return 'video'
  if (type.startsWith('audio/')) return 'music'
  if (type === 'application/zip' || type === 'application/x-zip-compressed') return 'archive'
  if (type === 'application/pdf' || type.includes('document')) return 'file-alt'
  // For XML files, use file-alt since file-code is not registered
  if (type.includes('xml')) return 'file-alt'
  return 'file'
}

const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return '0 Bytes'

  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}
</script>

<style lang="scss" scoped>
@use 'sass:color';
@use '@/assets/variables' as *;

.file-uploader {
  margin-bottom: 1.5rem;
}

.upload-capacity-error {
  background-color: #fef2f2;
  border: 1px solid #fecaca;
  color: #dc2626;
  padding: 0.75rem 1rem;
  border-radius: 6px;
  margin-bottom: 1rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
}

.slot-status {
  background-color: #f0f9ff;
  border: 1px solid #bae6fd;
  color: #0369a1;
  padding: 0.5rem 0.75rem;
  border-radius: 6px;
  margin-bottom: 0.75rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  font-weight: 500;
}

.upload-area {
  border: 2px dashed $border-color;
  border-radius: 8px;
  padding: 3rem 2rem;
  text-align: center;
  cursor: pointer;
  transition: all 0.2s;
  background-color: $light-bg-color;

  &:hover,
  &.drag-over {
    border-color: $primary-color;
    background: rgba($primary-color, 0.05);
  }

  &.has-file {
    border-style: solid;
    padding: 1.5rem;
  }

  .file-input {
    display: none;
  }

  .upload-content-inner {
    .upload-icon {
      font-size: 3rem;
      color: $text-secondary-color;
      margin-bottom: 1rem;
    }

    .upload-text {
      margin: 0 0 0.5rem;
      color: $text-color;

      .upload-prompt {
        display: block;
        font-weight: 500;
      }

      .upload-or {
        display: block;
        margin: 0.5rem 0;
        color: $text-secondary-color;
      }

      .browse-button {
        color: $primary-color;
        font-weight: 500;
      }
    }

    .upload-hint {
      margin: 0;
      font-size: 0.875rem;
      color: $text-secondary-color;
    }
  }
}

.selected-files-content {
  text-align: left;
}

.selected-files-list {
  position: relative;
}

.selected-file {
  display: flex;
  align-items: center;
  gap: 1rem;
  text-align: left;

  .file-preview {
    width: 60px;
    height: 60px;
    border-radius: 8px;
    overflow: hidden;
    background: white;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 1px solid $border-color;

    .file-thumbnail {
      width: 100%;
      height: 100%;
      object-fit: cover;
    }

    .file-icon {
      font-size: 2rem;
      color: $text-secondary-color;
    }
  }

  .file-info {
    flex: 1;

    h3 {
      margin: 0 0 0.25rem;
      color: $text-color;
      font-size: 1rem;
    }

    p {
      margin: 0;
      color: $text-secondary-color;
      font-size: 0.875rem;
    }
  }

  .btn-remove {
    width: 32px;
    height: 32px;
    border-radius: 50%;
    border: none;
    background: $error-color;
    color: white;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;

    &:hover {
      opacity: 0.8;
    }
  }
}

.upload-actions {
  background: $light-bg-color;
  border-radius: 8px;
  padding: 1rem;
  margin-top: 1rem;
  border: 1px solid $border-color;
  text-align: center;
}

// Upload actions transition - only animate when hiding
.upload-actions-enter-active {
  transition: none; // No transition when showing
}

.upload-actions-leave-active {
  transition: all 0.4s ease;
  transform-origin: top;
}

.upload-actions-enter-from {
  // No styles needed since we're not animating the enter
}

.upload-actions-leave-to {
  opacity: 0;
  transform: translateY(-10px) scaleY(0.8);
  max-height: 0;
  margin-top: 0;
  padding-top: 0;
  padding-bottom: 0;
}

// Upload progress styles
.upload-progress {
  margin-bottom: 1rem;
  padding: 1rem;
  background: $light-bg-color;
  border-radius: $default-radius;
  border: 1px solid $border-color;

  .progress-info {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.5rem;
    font-size: 0.9rem;

    .progress-text {
      color: $text-color;
      font-weight: 500;
    }

    .progress-percentage {
      color: $primary-color;
      font-weight: 600;
    }
  }

  .progress-bar {
    width: 100%;
    height: 8px;
    background: $border-color;
    border-radius: 4px;
    overflow: hidden;
    margin-bottom: 0.5rem;

    .progress-fill {
      height: 100%;
      background: linear-gradient(90deg, $primary-color, color.adjust($primary-color, $lightness: 10%));
      border-radius: 4px;
      transition: width 0.3s ease;
      position: relative;

      &::after {
        content: '';
        position: absolute;
        inset: 0;
        background: linear-gradient(
          90deg,
          transparent,
          rgb(255 255 255 / 30%),
          transparent
        );
        animation: shimmer 2s infinite;
      }
    }
  }

  .current-file {
    font-size: 0.8rem;
    color: $text-secondary-color;
    font-style: italic;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
}

@keyframes shimmer {
  0% {
    transform: translateX(-100%);
  }

  100% {
    transform: translateX(100%);
  }
}
</style>
