<script setup lang="ts">
/**
 * FileUploader — drag/drop upload pattern with capacity gating and progress UI.
 *
 * Migrated from the legacy uploader in Phase 4 of
 * Epic #1324 (issue #1354). Public API (props / emits / exposed methods)
 * is preserved so the consuming views (CommodityDetailView,
 * LocationDetailView, FileCreateView, ExportImportView) can swap the
 * import without further refactor.
 *
 * Class anchors preserved for the Playwright suite during the
 * strangler-fig window: `.file-uploader`, `.upload-area`, `.drag-over`,
 * `.has-file`, `.file-input`, `.upload-prompt`, `.browse-button`,
 * `.selected-files-content`, `.file-info`, `.btn-remove`,
 * `.upload-actions`, and `.btn-primary` on the upload button. See
 * `e2e/tests/includes/uploads.ts`.
 */
import { computed, ref } from 'vue'
import {
  AlertTriangle,
  Archive,
  CheckCircle,
  CloudUpload,
  File as FileIcon,
  FileText,
  Image as ImageIcon,
  Loader2,
  Music,
  Upload,
  Video,
  X,
} from 'lucide-vue-next'

import uploadSlotService from '@/services/uploadSlotService'

interface Props {
  multiple?: boolean
  accept?: string
  uploadPrompt?: string
  uploadHint?: string
  hideUploadButton?: boolean
  operationName?: string
  requireSlots?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  multiple: true,
  accept: '*/*',
  uploadPrompt: 'Drag and drop files here',
  uploadHint: 'Supports XML files and other document formats',
  hideUploadButton: false,
  operationName: 'file_upload',
  requireSlots: true,
})

const emit = defineEmits<{
  upload: [files: File[], rejected: File[]]
  filesCleared: []
  filesSelected: [files: File[]]
  uploadCapacityFailed: [error: unknown]
}>()

const fileInput = ref<HTMLInputElement | null>(null)
const uploadButton = ref<HTMLButtonElement | null>(null)
const selectedFiles = ref<File[]>([])
const filePreviews = ref<Record<string, string>>({})
const isDragOver = ref(false)
const isUploading = ref(false)
const uploadCompleted = ref(false)

const uploadProgress = ref({
  current: 0,
  total: 0,
  percentage: 0,
  currentFile: '',
})

const uploadCapacityError = ref<string | null>(null)
const isCheckingCapacity = ref(false)

const hasUploadCapacity = computed(
  () => !uploadCapacityError.value && !isCheckingCapacity.value,
)

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
    input.value = ''
  }
}

const addFiles = (files: File[]) => {
  if (files.length === 0) return

  if (props.multiple) {
    selectedFiles.value = [...selectedFiles.value, ...files]
  } else {
    selectedFiles.value = [files[0]]
  }

  files.forEach((file) => {
    if (file.type.startsWith('image/')) {
      const fileKey = `${file.name}_${file.size}`
      const reader = new FileReader()
      reader.onload = (e) => {
        filePreviews.value[fileKey] = e.target?.result as string
      }
      reader.readAsDataURL(file)
    }
  })

  uploadCompleted.value = false
  emit('filesSelected', selectedFiles.value)
}

const removeFile = (index: number) => {
  const file = selectedFiles.value[index]
  if (file) {
    const fileKey = `${file.name}_${file.size}`
    delete filePreviews.value[fileKey]
  }
  selectedFiles.value.splice(index, 1)
  uploadCompleted.value = false
  if (selectedFiles.value.length === 0) {
    emit('filesCleared')
  }
}

const markUploadCompleted = () => {
  isUploading.value = false
  uploadCompleted.value = true
  uploadProgress.value = { current: 0, total: 0, percentage: 0, currentFile: '' }
}

const markUploadFailed = () => {
  isUploading.value = false
  uploadCompleted.value = false
  uploadProgress.value = { current: 0, total: 0, percentage: 0, currentFile: '' }
}

const updateProgress = (current: number, total: number, currentFile = '') => {
  uploadProgress.value = {
    current,
    total,
    percentage: total > 0 ? (current / total) * 100 : 0,
    currentFile,
  }
}

const resetProgress = () => {
  uploadProgress.value = { current: 0, total: 0, percentage: 0, currentFile: '' }
}

const clearFiles = () => {
  selectedFiles.value = []
  filePreviews.value = {}
  uploadCompleted.value = false
  isUploading.value = false
  uploadCapacityError.value = null
  emit('filesCleared')
}

const uploadFiles = async () => {
  if (selectedFiles.value.length === 0) return

  isUploading.value = true
  uploadCapacityError.value = null

  try {
    if (props.requireSlots) {
      console.log(`🎫 Checking upload capacity for ${props.operationName}`)
      isCheckingCapacity.value = true
      try {
        const capacityResponse = await uploadSlotService.waitForCapacity(
          props.operationName,
          3,
          1000,
        )
        if (!capacityResponse.data.attributes.can_start_upload) {
          throw new Error('Upload capacity not available')
        }
        console.log(`✅ Upload capacity available for ${props.operationName}`)
      } catch (capacityError: unknown) {
        console.error('❌ Failed to get upload capacity:', capacityError)
        const message =
          capacityError && typeof capacityError === 'object' && 'message' in capacityError
            ? (capacityError as Error).message
            : 'Upload capacity not available'
        uploadCapacityError.value = message
        isUploading.value = false
        isCheckingCapacity.value = false
        emit('uploadCapacityFailed', capacityError)
        return
      } finally {
        isCheckingCapacity.value = false
      }
    }

    emit('upload', selectedFiles.value, [])
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

const getFileIcon = (file: File) => {
  const type = file.type
  if (type.startsWith('image/')) return ImageIcon
  if (type.startsWith('video/')) return Video
  if (type.startsWith('audio/')) return Music
  if (type === 'application/zip' || type === 'application/x-zip-compressed') return Archive
  if (type === 'application/pdf' || type.includes('document') || type.includes('xml')) return FileText
  return FileIcon
}

const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

defineExpose({
  clearFiles,
  markUploadCompleted,
  markUploadFailed,
  updateProgress,
  resetProgress,
  getUploadButton: () => uploadButton.value,
  selectedFiles,
  // Internal helpers exposed for the existing vitest spec.
  fileInput,
  addFiles,
  triggerFileInput,
  uploadFiles,
  isUploading,
})
</script>


<template>
  <div class="file-uploader mb-6">
    <div
      class="upload-area cursor-pointer rounded-lg border-2 border-dashed border-border bg-muted/40 px-8 py-12 text-center transition-colors hover:border-primary hover:bg-primary/5"
      :class="{
        'drag-over border-primary bg-primary/5': isDragOver,
        'has-file border-solid p-6': selectedFiles.length > 0,
      }"
      @dragover.prevent="onDragOver"
      @dragleave.prevent="onDragLeave"
      @drop.prevent="onDrop"
      @click="triggerFileInput"
    >
      <input
        ref="fileInput"
        type="file"
        :multiple="multiple"
        :accept="accept"
        class="file-input hidden"
        @change="onFileSelected"
      />

      <div v-if="selectedFiles.length === 0" class="upload-content-inner">
        <CloudUpload class="mx-auto mb-4 size-12 text-muted-foreground" aria-hidden="true" />
        <p class="m-0 mb-2 text-foreground">
          <span class="upload-prompt block font-medium">{{ uploadPrompt }}</span>
          <span class="upload-or my-2 block text-muted-foreground">or</span>
          <span class="browse-button font-medium text-primary">click to browse</span>
        </p>
        <p class="upload-hint m-0 text-sm text-muted-foreground">{{ uploadHint }}</p>
      </div>

      <div v-else class="selected-files-content text-left">
        <div class="selected-files-list relative">
          <div
            v-for="(file, index) in selectedFiles"
            :key="index"
            class="selected-file flex items-center gap-4 text-left"
          >
            <div
              class="file-preview flex size-[60px] items-center justify-center overflow-hidden rounded-lg border border-border bg-background"
            >
              <img
                v-if="getFilePreview(file) && file.type.startsWith('image/')"
                :src="getFilePreview(file) ?? ''"
                :alt="file.name"
                class="file-thumbnail size-full object-cover"
              />
              <component
                :is="getFileIcon(file)"
                v-else
                class="file-icon size-8 text-muted-foreground"
                aria-hidden="true"
              />
            </div>
            <div class="file-info flex-1 min-w-0">
              <h3 class="m-0 mb-1 truncate text-base text-foreground">{{ file.name }}</h3>
              <p class="m-0 truncate text-sm text-muted-foreground">
                {{ formatFileSize(file.size) }} • {{ file.type || 'Unknown type' }}
              </p>
            </div>
            <button
              type="button"
              class="btn-remove inline-flex size-8 items-center justify-center rounded-full border-0 bg-destructive text-destructive-foreground transition-opacity hover:opacity-80"
              title="Remove file"
              @click.stop="removeFile(index)"
            >
              <X class="size-4" aria-hidden="true" />
              <span class="sr-only">Remove file</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <div
      v-if="uploadCapacityError"
      class="upload-capacity-error mt-4 flex items-center gap-2 rounded-md border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive"
      role="alert"
    >
      <AlertTriangle class="size-4" aria-hidden="true" />
      {{ uploadCapacityError }}
    </div>

    <transition
      enter-active-class=""
      leave-active-class="motion-safe:transition-all motion-safe:duration-300 ease-in-out origin-top"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0 -translate-y-2 scale-y-75 max-h-0 mt-0 py-0"
      mode="out-in"
    >
      <div
        v-if="selectedFiles.length > 0 && !uploadCompleted && !hideUploadButton"
        class="upload-actions mt-4 rounded-lg border border-border bg-muted/40 p-4 text-center"
      >
        <div
          v-if="requireSlots && hasUploadCapacity"
          class="slot-status mb-3 flex items-center gap-2 rounded-md border border-sky-200 bg-sky-50 px-3 py-2 text-sm font-medium text-sky-700 dark:border-sky-900/60 dark:bg-sky-950/40 dark:text-sky-200"
        >
          <CheckCircle class="size-4" aria-hidden="true" />
          Upload capacity available
        </div>

        <div
          v-if="isUploading && uploadProgress.total > 0"
          class="upload-progress mb-4 rounded-md border border-border bg-background p-4 text-left"
        >
          <div class="progress-info mb-2 flex items-center justify-between text-sm">
            <span class="progress-text font-medium text-foreground">
              Uploading {{ uploadProgress.current }} of {{ uploadProgress.total }} files...
            </span>
            <span class="progress-percentage font-semibold text-primary">
              {{ Math.round(uploadProgress.percentage) }}%
            </span>
          </div>
          <div class="progress-bar mb-2 h-2 w-full overflow-hidden rounded-full bg-muted">
            <div
              class="progress-fill h-full rounded-full bg-primary motion-safe:transition-[width] motion-safe:duration-300"
              :style="{ width: uploadProgress.percentage + '%' }"
            />
          </div>
          <div
            v-if="uploadProgress.currentFile"
            class="current-file truncate text-xs italic text-muted-foreground"
          >
            {{ uploadProgress.currentFile }}
          </div>
        </div>

        <button
          ref="uploadButton"
          type="button"
          class="btn btn-primary inline-flex items-center justify-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50"
          :disabled="isUploading"
          @click="uploadFiles"
        >
          <Loader2 v-if="isUploading" class="size-4 animate-spin" aria-hidden="true" />
          <Upload v-else class="size-4" aria-hidden="true" />
          {{ isUploading ? 'Uploading...' : 'Upload Files' }}
        </button>
      </div>
    </transition>
  </div>
</template>

