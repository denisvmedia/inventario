<template>
  <div class="file-create-view">
    <!-- Breadcrumb Navigation -->
    <div class="breadcrumb-nav">
      <a href="#" class="breadcrumb-link" @click.prevent="goBack">
        <FontAwesomeIcon icon="arrow-left" />
        Back to Files
      </a>
    </div>

    <!-- Page Header -->
    <div class="header">
      <div class="header-title">
        <h1>Upload Files</h1>
      </div>
    </div>

    <!-- Upload Content -->
    <div class="upload-content">
      <!-- File Upload Card -->
      <div class="upload-card">
        <div class="card-header">
          <h2>Select File</h2>
        </div>
        <div class="card-body">
          <p class="form-description">
            File will be uploaded with auto-detected metadata. You can edit the details after upload.
          </p>
          <FileUploader
            ref="fileUploader"
            :multiple="false"
            accept="*/*"
            upload-prompt="Drag and drop a file here"
            upload-hint="Supports images, documents, videos, audio files, and archives"
            operation-name="file_upload"
            :require-slots="true"
            :hide-upload-button="true"
            @filesCleared="handleFilesCleared"
            @filesSelected="handleFilesSelected"
            @upload-capacity-failed="onUploadCapacityFailed"
          />

          <!-- Upload Actions -->
          <div v-if="hasSelectedFiles" class="upload-actions">
            <div class="action-buttons">
              <button
                type="button"
                class="btn btn-primary"
                :disabled="uploading"
                @click="uploadFile"
              >
                <FontAwesomeIcon v-if="uploading" icon="spinner" spin />
                <FontAwesomeIcon v-else icon="upload" />
                {{ uploading ? 'Uploading...' : 'Upload File' }}
              </button>
              <button type="button" class="btn btn-secondary" :disabled="uploading" @click="goBack">
                Cancel
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Error Display Card -->
      <div v-if="error" class="error-card">
        <div class="card-body">
          <div class="error-content">
            <div class="error-icon">
              <FontAwesomeIcon icon="exclamation-circle" />
            </div>
            <div class="error-text">
              <h3>Upload Failed</h3>
              <p>{{ error }}</p>
            </div>
            <button class="btn btn-secondary" @click="clearError">
              <FontAwesomeIcon icon="times" />
              Dismiss
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import FileUploader from '@/components/FileUploader.vue'
import fileService from '@/services/fileService'

const router = useRouter()

// State
const uploading = ref(false)
const error = ref<string | null>(null)
const hasSelectedFiles = ref(false)

// File uploader ref
const fileUploader = ref<InstanceType<typeof FileUploader> | null>(null)

// Methods
const handleFilesCleared = () => {
  hasSelectedFiles.value = false
  error.value = null
}

const handleFilesSelected = () => {
  hasSelectedFiles.value = true
}

const uploadFile = async () => {
  console.log('Upload button clicked!')

  // Get the selected files from the FileUploader component
  const selectedFiles = fileUploader.value?.selectedFiles
  console.log('Selected files:', selectedFiles)

  if (!selectedFiles || selectedFiles.length === 0) {
    console.log('No files selected')
    return
  }

  const file = selectedFiles[0] // Since we're using single file mode
  console.log('Uploading file:', file.name)

  uploading.value = true
  error.value = null

  try {
    // Setup progress tracking
    // TODO: Add support for cancellation of file upload in progress
    // - Implement AbortController to allow cancelling the upload
    // - Add cancel button in UI during upload
    // - Handle cancellation gracefully with proper cleanup
    const onProgress = (current: number, total: number, currentFile: string) => {
      fileUploader.value?.updateProgress(current, total, currentFile)
    }

    const response = await fileService.uploadFile(file, onProgress)

    // Mark upload as completed in the FileUploader component
    fileUploader.value?.markUploadCompleted()

    // The upload creates the file entity automatically
    // Check if we got a single file or multiple files
    const responseFiles = response.data.data
    if (Array.isArray(responseFiles) && responseFiles.length > 0) {
      // Redirect to the first uploaded file's detail view
      const fileId = responseFiles[0].id
      router.push(`/files/${fileId}`)
    } else if (responseFiles && responseFiles.id) {
      // Single file response
      router.push(`/files/${responseFiles.id}`)
    } else {
      // Fallback to files list
      router.push('/files')
    }
  } catch (err: any) {
    error.value = err.response?.data?.message || 'Failed to upload file'
    console.error('Error uploading file:', err)

    // Mark upload as failed in the FileUploader component
    fileUploader.value?.markUploadFailed()
  } finally {
    uploading.value = false
  }
}

const onUploadCapacityFailed = (capacityError: any) => {
  console.error('Upload capacity failed:', capacityError)
  error.value = 'Upload capacity unavailable: ' + (capacityError.message || 'Try again later')
}

const clearError = () => {
  error.value = null
}

const goBack = () => {
  router.push('/files')
}
</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;

.file-create-view {
  max-width: 800px;
  margin: 0 auto;
  padding: 20px;
}

.breadcrumb-nav {
  margin-bottom: 1rem;
}

.breadcrumb-link {
  color: $secondary-color;
  font-size: 0.9rem;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  transition: color 0.2s;

  &:hover {
    color: $primary-color;
    text-decoration: none;
  }
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 2rem;

  .header-title {
    display: flex;
    flex-direction: column;
    align-items: flex-start;

    h1 {
      margin: 0 0 5px;
      font-size: 2rem;
    }
  }
}

.upload-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.upload-card,
.error-card {
  background: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
  overflow: hidden;
}

.error-card {
  border-left: 4px solid $error-color;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px;
  background-color: $light-bg-color;
  border-bottom: 1px solid $border-color;

  h2 {
    margin: 0;
    font-size: 1.25rem;
  }
}

.card-body {
  padding: 20px;
}

.form-description {
  margin: 0 0 1.5rem;
  color: $text-secondary-color;
  font-size: 1rem;
  line-height: 1.5;
  padding: 1rem;
  background-color: rgba($primary-color, 0.05);
  border-left: 4px solid $primary-color;
  border-radius: $default-radius;
}

.upload-actions {
  margin-top: 1.5rem;
  padding-top: 1.5rem;
  border-top: 1px solid $border-color;
  text-align: center;

  .action-buttons {
    display: flex;
    gap: 1rem;
    justify-content: center;
  }
}

.error-content {
  display: flex;
  align-items: center;
  gap: 1rem;

  .error-icon {
    font-size: 1.5rem;
    color: $error-color;
  }

  .error-text {
    flex: 1;

    h3 {
      margin: 0 0 0.25rem;
      color: $error-color;
      font-size: 1rem;
    }

    p {
      margin: 0;
      color: $text-color;
    }
  }
}
</style>
