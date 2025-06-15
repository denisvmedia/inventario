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
        <p class="upload-hint">Supports XML files and other document formats</p>
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
            <button class="btn-remove" @click.stop="removeFile(index)" title="Remove file">
              <font-awesome-icon icon="times" />
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Upload actions outside the file block -->
    <transition name="upload-actions" mode="out-in">
      <div v-if="selectedFiles.length > 0 && !uploadCompleted" class="upload-actions">
        <button type="button" class="btn btn-primary" :disabled="isUploading" @click="uploadFiles">
          <font-awesome-icon v-if="isUploading" icon="spinner" spin />
          <font-awesome-icon v-else icon="upload" />
          {{ isUploading ? 'Uploading...' : 'Upload Files' }}
        </button>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'

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
  }
})

const emit = defineEmits(['upload', 'filesCleared'])

const fileInput = ref<HTMLInputElement | null>(null)
const selectedFiles = ref<File[]>([])
const filePreviews = ref<{ [key: string]: string }>({}) // Store file previews by file name + size
const isDragOver = ref(false)
const isUploading = ref(false)
const uploadCompleted = ref(false)

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

const clearFiles = () => {
  selectedFiles.value = []
  filePreviews.value = {} // Clear all previews
  uploadCompleted.value = false
  isUploading.value = false
  emit('filesCleared')
}

const markUploadCompleted = () => {
  isUploading.value = false
  uploadCompleted.value = true
}

const markUploadFailed = () => {
  isUploading.value = false
  uploadCompleted.value = false
}

// Expose methods for parent component
defineExpose({
  clearFiles,
  markUploadCompleted,
  markUploadFailed
})

const uploadFiles = async () => {
  if (selectedFiles.value.length === 0) return

  isUploading.value = true
  try {
    emit('upload', selectedFiles.value)
    // Don't set uploadCompleted here - wait for parent to signal completion
  } catch (error) {
    console.error('Upload failed:', error)
    isUploading.value = false
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
      margin: 0 0 0.5rem 0;
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
      margin: 0 0 0.25rem 0;
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
</style>
