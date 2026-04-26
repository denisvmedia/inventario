<script setup lang="ts">
/**
 * FileViewerDialog — modal preview for files in a media gallery.
 *
 * This pattern replaces the legacy FileViewer modal while preserving the
 * Playwright class anchors used during the design-system migration window.
 */
import { computed, defineAsyncComponent, onBeforeUnmount, ref, watch } from 'vue'
import { ChevronLeft, ChevronRight, Download, File as FileIcon, FileText, Trash2, X } from 'lucide-vue-next'

import type { FileEntity } from '@/services/fileService'
import fileService from '@/services/fileService'
import { Button } from '@design/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@design/ui/dialog'

const PDFViewerCanvas = defineAsyncComponent(() => import('@/components/PDFViewerCanvas.vue'))

interface SignedUrlData {
  url?: string
  thumbnails?: Record<string, string>
}

interface Props {
  files: FileEntity[]
  open?: boolean
  selectedIndex?: number
  signedUrls?: Record<string, SignedUrlData>
  allowDelete?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  open: false,
  selectedIndex: 0,
  signedUrls: () => ({}),
  allowDelete: true,
})

const emit = defineEmits<{
  'update:open': [value: boolean]
  'update:selectedIndex': [value: number]
  download: [file: FileEntity]
  delete: [file: FileEntity]
}>()

const currentIndex = computed(() => {
  if (props.files.length === 0) return 0
  return Math.min(Math.max(props.selectedIndex, 0), props.files.length - 1)
})

const currentFile = computed(() => props.files[currentIndex.value] ?? null)
const currentFileName = computed(() => {
  const file = currentFile.value
  if (!file) return ''
  const base = file.path || file.id
  return `${base}${file.ext || ''}`
})

const currentFileUrl = computed(() => {
  const file = currentFile.value
  if (!file) return ''
  const urlData = props.signedUrls[file.id]
  if (urlData?.url) return urlData.url
  return ''
})

const hasMultipleFiles = computed(() => props.files.length > 1)
const isImage = computed(() => currentFile.value ? fileService.isImageFile(currentFile.value) : false)
const isPdf = computed(() => currentFile.value?.mime_type === 'application/pdf' || currentFile.value?.ext?.toLowerCase().replace('.', '') === 'pdf')

function setOpen(value: boolean) {
  emit('update:open', value)
}

function closeDialog() {
  setOpen(false)
}

function setIndex(index: number) {
  if (props.files.length === 0) return
  const next = (index + props.files.length) % props.files.length
  emit('update:selectedIndex', next)
}

function previousFile() {
  setIndex(currentIndex.value - 1)
}

function nextFile() {
  setIndex(currentIndex.value + 1)
}

function downloadCurrentFile() {
  if (currentFile.value) emit('download', currentFile.value)
}

function deleteCurrentFile() {
  if (currentFile.value) emit('delete', currentFile.value)
}

const isZoomed = ref(false)
const panX = ref(0)
const panY = ref(0)
const isPanning = ref(false)
const isDragging = ref(false)
const isGlobalDragging = ref(false)
const startX = ref(0)
const startY = ref(0)
const clickStartPos = ref({ x: 0, y: 0 })
const ZOOM_SCALE = 2
const DEFAULT_PDF_ERROR_MESSAGE = 'Unable to display PDF. Please download the file to view it.'

const pdfHasError = ref(false)
const pdfErrorMessage = ref(DEFAULT_PDF_ERROR_MESSAGE)

const imageStyle = computed(() => {
  if (!isZoomed.value) {
    return { transform: 'none', cursor: 'zoom-in' }
  }
  return {
    transform: `translate(${panX.value}px, ${panY.value}px) scale(${ZOOM_SCALE})`,
    cursor: isPanning.value ? 'grabbing' : 'grab',
  }
})

function resetZoom() {
  isZoomed.value = false
  panX.value = 0
  panY.value = 0
  isPanning.value = false
  isDragging.value = false
  isGlobalDragging.value = false
  document.removeEventListener('mousemove', handleGlobalMouseMove)
  document.removeEventListener('mouseup', handleGlobalMouseUp)
}

function toggleZoom() {
  if (isZoomed.value) resetZoom()
  else {
    isZoomed.value = true
    panX.value = 0
    panY.value = 0
  }
}

function handleImageClick() {
  if (!isDragging.value) toggleZoom()
  isDragging.value = false
}

function startPan(event: MouseEvent) {
  if (!isZoomed.value) return
  event.preventDefault()
  isPanning.value = true
  isGlobalDragging.value = false
  startX.value = event.clientX - panX.value
  startY.value = event.clientY - panY.value
  clickStartPos.value = { x: event.clientX, y: event.clientY }
  isDragging.value = false
  document.addEventListener('mousemove', handleGlobalMouseMove)
  document.addEventListener('mouseup', handleGlobalMouseUp)
}

function handleGlobalMouseMove(event: MouseEvent) {
  if (!isPanning.value) return
  const dx = Math.abs(event.clientX - clickStartPos.value.x)
  const dy = Math.abs(event.clientY - clickStartPos.value.y)
  if (dx > 5 || dy > 5) {
    isDragging.value = true
    isGlobalDragging.value = true
  }
  panX.value = event.clientX - startX.value
  panY.value = event.clientY - startY.value
}

function handleGlobalMouseUp() {
  if (!isPanning.value) return
  isPanning.value = false
  setTimeout(() => { isGlobalDragging.value = false }, 50)
  document.removeEventListener('mousemove', handleGlobalMouseMove)
  document.removeEventListener('mouseup', handleGlobalMouseUp)
}

function handlePdfError(error: unknown) {
  pdfHasError.value = true
  pdfErrorMessage.value = DEFAULT_PDF_ERROR_MESSAGE
  if (error && typeof error === 'object' && 'message' in error) {
    const message = String((error as { message: unknown }).message)
    if (message.includes('timeout')) {
      pdfErrorMessage.value = 'PDF loading timed out. Please try downloading the file instead.'
    } else if (message.includes('canvas')) {
      pdfErrorMessage.value = 'PDF viewer is not available. Please download the file to view it.'
    }
  }
}

function handleKeydown(event: KeyboardEvent) {
  if (!props.open) return
  if (event.key === 'ArrowLeft') previousFile()
  else if (event.key === 'ArrowRight') nextFile()
  else if (event.key === ' ' && isImage.value) {
    event.preventDefault()
    toggleZoom()
  }
}

function handleInteractOutside(event: Event) {
  if (isGlobalDragging.value) event.preventDefault()
}

watch(() => props.open, (open) => {
  if (open) {
    resetZoom()
    pdfHasError.value = false
    pdfErrorMessage.value = 'Unable to display PDF. Please download the file to view it.'
    window.addEventListener('keydown', handleKeydown)
  } else {
    window.removeEventListener('keydown', handleKeydown)
    resetZoom()
  }
}, { immediate: true })

watch(currentFile, () => {
  resetZoom()
  pdfHasError.value = false
  pdfErrorMessage.value = DEFAULT_PDF_ERROR_MESSAGE
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown)
  document.removeEventListener('mousemove', handleGlobalMouseMove)
  document.removeEventListener('mouseup', handleGlobalMouseUp)
})
</script>

<template>
  <Dialog :open="open" @update:open="setOpen">
    <DialogContent
      :show-close-button="false"
      class="file-modal modal-content max-h-[90vh] max-w-[min(1200px,calc(100vw-2rem))] grid-rows-[auto_minmax(300px,1fr)_auto] gap-0 overflow-hidden p-0"
      @interact-outside="handleInteractOutside"
    >
      <div class="modal-header flex items-center justify-between border-b border-border p-4">
        <DialogTitle as="h3" :title="currentFileName" class="truncate pr-4">
          {{ currentFileName || 'File preview' }}
        </DialogTitle>
        <DialogDescription class="sr-only">
          Preview the selected file, navigate between files, download, or delete it.
        </DialogDescription>
        <button class="close-button inline-flex size-8 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground" title="Close" @click="closeDialog">
          <X class="size-4" aria-hidden="true" />
          <span class="sr-only">Close</span>
        </button>
      </div>

      <div class="modal-body relative flex min-h-[300px] justify-center overflow-hidden bg-muted p-4">
        <button v-if="hasMultipleFiles" class="nav-button prev absolute left-3 top-1/2 z-10 inline-flex size-10 -translate-y-1/2 items-center justify-center rounded-full bg-background/80 text-foreground shadow hover:bg-background" title="Previous file" @click="previousFile">
          <ChevronLeft class="size-5" aria-hidden="true" />
        </button>

        <slot name="body" :file="currentFile" :url="currentFileUrl" :is-image="isImage" :is-pdf="isPdf">
          <div v-if="isImage" class="image-container flex size-full items-center justify-center overflow-hidden">
            <img
              v-if="currentFileUrl"
              :src="currentFileUrl"
              :alt="currentFileName"
              :data-file-id="currentFile?.id"
              class="full-image max-h-[70vh] max-w-full object-contain select-none"
              :class="{ zoomed: isZoomed }"
              :style="imageStyle"
              draggable="false"
              @click="handleImageClick"
              @mousedown="startPan"
            />
            <div v-else class="loading-placeholder text-sm text-muted-foreground">Loading preview...</div>
          </div>
          <div v-else-if="isPdf" class="pdf-container flex w-full flex-col">
            <template v-if="!pdfHasError && currentFileUrl">
              <PDFViewerCanvas :url="currentFileUrl" @error="handlePdfError" />
            </template>
            <div v-else-if="!currentFileUrl" class="loading-placeholder m-auto text-sm text-muted-foreground">Loading preview...</div>
            <div v-else class="pdf-error-container m-auto flex flex-col items-center gap-3 text-center text-muted-foreground">
              <FileText class="size-12" aria-hidden="true" />
              <p>{{ pdfErrorMessage }}</p>
              <Button class="btn btn-primary" size="sm" @click="downloadCurrentFile">
                <Download class="size-4" /> Download PDF
              </Button>
            </div>
          </div>
          <div v-else class="unsupported-file m-auto flex flex-col items-center gap-3 text-center text-muted-foreground">
            <FileText v-if="currentFile?.mime_type === 'application/pdf'" class="size-12" aria-hidden="true" />
            <FileIcon v-else class="size-12" aria-hidden="true" />
            <p>This file type cannot be previewed. Please download the file to view it.</p>
          </div>
        </slot>

        <button v-if="hasMultipleFiles" class="nav-button next absolute right-3 top-1/2 z-10 inline-flex size-10 -translate-y-1/2 items-center justify-center rounded-full bg-background/80 text-foreground shadow hover:bg-background" title="Next file" @click="nextFile">
          <ChevronRight class="size-5" aria-hidden="true" />
        </button>
      </div>

      <div class="modal-footer flex items-center justify-between gap-4 border-t border-border p-4">
        <span class="file-counter text-sm text-muted-foreground">{{ currentIndex + 1 }} / {{ files.length }}</span>
        <div class="file-actions flex items-center gap-2">
          <Button class="btn btn-sm btn-primary" size="sm" @click="downloadCurrentFile"><Download class="size-4" /> Download</Button>
          <Button v-if="allowDelete" class="btn btn-sm btn-danger" size="sm" variant="destructive" @click="deleteCurrentFile"><Trash2 class="size-4" /> Delete</Button>
          <Button class="btn btn-sm btn-secondary" size="sm" variant="secondary" @click="closeDialog">Close</Button>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
