<template>
  <div class="file-uploader">
    <div class="upload-area" 
         @dragover.prevent="onDragOver" 
         @dragleave.prevent="onDragLeave" 
         @drop.prevent="onDrop"
         :class="{ 'drag-over': isDragOver }">
      <input 
        type="file" 
        ref="fileInput" 
        @change="onFileSelected" 
        :multiple="multiple"
        :accept="accept"
        class="file-input"
      />
      <div class="upload-content">
        <div class="upload-icon">
          <i class="fas fa-upload"></i>
        </div>
        <p class="upload-text">
          <span class="upload-prompt">{{ uploadPrompt }}</span>
          <span class="upload-or">or</span>
          <button class="browse-button" @click="triggerFileInput">Browse Files</button>
        </p>
      </div>
    </div>
    
    <div v-if="selectedFiles.length > 0" class="selected-files">
      <div v-for="(file, index) in selectedFiles" :key="index" class="selected-file">
        <span class="file-name">{{ file.name }}</span>
        <button class="remove-file" @click="removeFile(index)">×</button>
      </div>
      <div class="upload-actions">
        <button class="btn btn-primary" @click="uploadFiles" :disabled="isUploading">
          {{ isUploading ? 'Uploading...' : 'Upload Files' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, defineProps, defineEmits } from 'vue'

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

const emit = defineEmits(['upload'])

const fileInput = ref<HTMLInputElement | null>(null)
const selectedFiles = ref<File[]>([])
const isDragOver = ref(false)
const isUploading = ref(false)

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
}

const removeFile = (index: number) => {
  selectedFiles.value.splice(index, 1)
}

const uploadFiles = async () => {
  if (selectedFiles.value.length === 0) return
  
  isUploading.value = true
  try {
    emit('upload', selectedFiles.value)
    selectedFiles.value = [] // Clear selected files after successful upload
  } catch (error) {
    console.error('Upload failed:', error)
  } finally {
    isUploading.value = false
  }
}
</script>

<style scoped>
.file-uploader {
  margin-bottom: 1.5rem;
}

.upload-area {
  position: relative;
  border: 2px dashed #ccc;
  border-radius: 8px;
  padding: 2rem;
  text-align: center;
  transition: all 0.3s ease;
  background-color: #f9f9f9;
  cursor: pointer;
}

.upload-area:hover {
  border-color: #4CAF50;
}

.drag-over {
  border-color: #4CAF50;
  background-color: #f0f9f0;
}

.file-input {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  opacity: 0;
  cursor: pointer;
}

.upload-content {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.upload-icon {
  font-size: 2rem;
  color: #6c757d;
  margin-bottom: 1rem;
}

.upload-text {
  margin: 0;
  color: #555;
}

.upload-prompt {
  display: block;
  margin-bottom: 0.5rem;
}

.upload-or {
  display: block;
  margin: 0.5rem 0;
  color: #888;
}

.browse-button {
  background-color: #4CAF50;
  color: white;
  border: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  cursor: pointer;
  font-weight: 500;
}

.browse-button:hover {
  background-color: #45a049;
}

.selected-files {
  margin-top: 1rem;
}

.selected-file {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background-color: #f0f0f0;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  margin-bottom: 0.5rem;
}

.file-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.remove-file {
  background: none;
  border: none;
  color: #dc3545;
  font-size: 1.25rem;
  cursor: pointer;
  padding: 0 0.5rem;
}

.upload-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 1rem;
}
</style>
