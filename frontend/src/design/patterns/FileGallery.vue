<script setup lang="ts">
/**
 * FileGallery — design-system replacement for the legacy FileViewer.
 *
 * Internally composes MediaGallery, FilePreview, and FileViewerDialog while
 * preserving legacy class anchors used by Playwright during migration.
 */
import { computed, ref } from 'vue'
import { FileText, X, Download, Trash2 } from 'lucide-vue-next'

import type { FileEntity } from '@/services/fileService'
import { Button } from '@design/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@design/ui/dialog'
import { useConfirm } from '@design/composables/useConfirm'

import MediaGallery from './MediaGallery.vue'
import FilePreview from './FilePreview.vue'
import FileViewerDialog from './FileViewerDialog.vue'
import type { FileGallerySignedUrls } from './fileGalleryTypes'

type AnyRecord = Record<string, unknown>
type ApiResource = { id: string; attributes?: AnyRecord } & AnyRecord

interface Props {
  files: ApiResource[]
  signedUrls?: FileGallerySignedUrls
  entityId: string
  entityType?: string
  fileType: 'images' | 'manuals' | 'invoices' | 'files'
  allowDelete?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  signedUrls: () => ({}),
  entityType: 'commodities',
  allowDelete: true,
})

const emit = defineEmits<{
  delete: [file: ApiResource]
  download: [file: ApiResource]
  update: [data: { id: string; path: string }]
}>()

const { confirmDelete } = useConfirm()
const viewerOpen = ref(false)
const viewerIndex = ref(0)
const detailsFile = ref<ApiResource | null>(null)

function attrs(file: ApiResource): AnyRecord {
  return (file.attributes ?? file) as AnyRecord
}

function fileName(file: ApiResource): string {
  const a = attrs(file)
  const path = (a.path as string) || file.id
  const ext = (a.ext as string) || ''
  return `${path}${ext}`
}

function fileKind(file: ApiResource): FileEntity['type'] {
  const a = attrs(file)
  const ext = String(a.ext ?? '').toLowerCase().replace('.', '')
  const mime = String(a.mime_type ?? a.content_type ?? '').toLowerCase()
  if (mime.startsWith('image/') || ['jpg', 'jpeg', 'png', 'gif', 'webp'].includes(ext)) return 'image'
  if (mime.startsWith('video/')) return 'video'
  if (mime.startsWith('audio/')) return 'audio'
  if (['zip', 'rar', '7z', 'tar', 'gz'].includes(ext)) return 'archive'
  if (mime === 'application/pdf' || ext === 'pdf' || props.fileType === 'manuals' || props.fileType === 'invoices') return 'document'
  return 'other'
}

function toFileEntity(file: ApiResource): FileEntity {
  const a = attrs(file)
  const path = (a.path as string) || file.id
  const ext = (a.ext as string) || ''
  return {
    id: file.id,
    title: fileName(file),
    description: (a.description as string) || '',
    type: fileKind(file),
    tags: (a.tags as string[]) || [],
    path,
    original_path: (a.original_path as string) || fileName(file),
    ext,
    mime_type: (a.mime_type as string) || (a.content_type as string) || '',
    linked_entity_type: (a.linked_entity_type as string) || props.entityType,
    linked_entity_id: (a.linked_entity_id as string) || props.entityId,
    linked_entity_meta: (a.linked_entity_meta as string) || props.fileType,
    created_at: a.created_at as string | undefined,
    updated_at: a.updated_at as string | undefined,
  }
}

const galleryFiles = computed(() => props.files.map(toFileEntity))
const currentDetailsEntity = computed(() => detailsFile.value ? toFileEntity(detailsFile.value) : null)
const detailsOpen = computed({
  get: () => detailsFile.value !== null,
  set: (open: boolean) => {
    if (!open) detailsFile.value = null
  },
})
const detailsUrl = computed(() => detailsFile.value ? thumbnailUrl(detailsFile.value) : '')
const detailsObjectType = computed(() => {
  if (!currentDetailsEntity.value) return 'File'
  if (currentDetailsEntity.value.type === 'image') return 'Image'
  if (currentDetailsEntity.value.mime_type === 'application/pdf' || currentDetailsEntity.value.ext.toLowerCase().replace('.', '') === 'pdf') return 'PDF'
  return 'File'
})

function thumbnailUrl(file: ApiResource): string {
  const data = props.signedUrls[file.id]
  if (data?.thumbnails?.medium && fileKind(file) === 'image') return data.thumbnails.medium
  return data?.url || ''
}

function openViewer(index: number) {
  viewerIndex.value = index
  viewerOpen.value = true
}

async function requestDelete(file: ApiResource) {
  if (!props.allowDelete) return
  const singular = props.fileType.endsWith('s') ? props.fileType.slice(0, -1) : props.fileType
  if (!(await confirmDelete(singular))) return
  if (detailsFile.value?.id === file.id) detailsFile.value = null
  if (props.files.length <= 1) viewerOpen.value = false
  emit('delete', file)
}

function renameFile(file: ApiResource) {
  const a = attrs(file)
  const currentPath = (a.path as string) || ''
  const nextPath = window.prompt('Enter file name', currentPath)?.trim()
  if (!nextPath || nextPath === currentPath) return
  emit('update', { id: file.id, path: nextPath })
}

function downloadDetailsFile() {
  if (detailsFile.value) emit('download', detailsFile.value)
}

function deleteDetailsFile() {
  if (detailsFile.value) void requestDelete(detailsFile.value)
}
</script>

<template>
  <div class="file-viewer">
    <div class="file-list">
      <div v-if="files.length === 0" class="no-files rounded-md bg-muted p-4 text-center text-sm text-muted-foreground">
        No {{ fileType }} uploaded yet.
      </div>

      <MediaGallery v-else class="files-container" density="default">
        <FilePreview
          v-for="(file, index) in files"
          :key="file.id"
          :file="toFileEntity(file)"
          :thumbnail-url="thumbnailUrl(file)"
          :show-details-action="true"
          @view="openViewer(index)"
          @download="emit('download', file)"
          @edit="renameFile(file)"
          @details="detailsFile = file"
          @delete="requestDelete(file)"
        />
      </MediaGallery>
    </div>

    <FileViewerDialog
      v-model:open="viewerOpen"
      v-model:selected-index="viewerIndex"
      :files="galleryFiles"
      :signed-urls="signedUrls"
      :allow-delete="allowDelete"
      @download="(file) => emit('download', files.find((raw) => raw.id === file.id) ?? (file as unknown as ApiResource))"
      @delete="(file) => { const raw = files.find((candidate) => candidate.id === file.id); if (raw) requestDelete(raw) }"
    />

    <Dialog v-model:open="detailsOpen">
      <DialogContent
        v-if="currentDetailsEntity"
        :show-close-button="false"
        class="file-details-overlay file-details-modal flex max-h-[90vh] w-full max-w-3xl flex-col overflow-hidden rounded-lg border border-border bg-card p-0 shadow-xl"
      >
        <div class="file-details-header flex items-center justify-between border-b border-border p-4">
          <DialogTitle class="text-lg font-semibold">File Details</DialogTitle>
          <DialogDescription class="sr-only">
            View file metadata, download the file, or delete it.
          </DialogDescription>
          <button class="close-button inline-flex size-8 items-center justify-center rounded-md text-muted-foreground hover:bg-muted" @click="detailsFile = null">
            <X class="size-4" />
            <span class="sr-only">Close</span>
          </button>
        </div>

        <div class="file-details-content grid gap-4 overflow-y-auto p-4 md:grid-cols-2">
          <div class="file-preview-section flex min-h-48 items-center justify-center rounded-md bg-muted p-4">
            <div v-if="currentDetailsEntity.type === 'image' && detailsUrl" class="image-preview">
              <img :src="detailsUrl" alt="Image preview" class="max-h-72 max-w-full object-contain" />
            </div>
            <div v-else class="file-icon-preview text-muted-foreground">
              <FileText class="fa-file-pdf size-20" aria-hidden="true" />
            </div>
          </div>

          <div class="file-info-section space-y-3 text-sm">
            <div class="file-info-item file-id"><div class="info-label font-semibold">ID:</div><div class="info-value break-all">{{ currentDetailsEntity.id }}</div></div>
            <div class="file-info-item file-name"><div class="info-label font-semibold">File Name:</div><div class="info-value break-all">{{ currentDetailsEntity.path }}{{ currentDetailsEntity.ext }}</div></div>
            <div class="file-info-item file-original-name"><div class="info-label font-semibold">Original Name:</div><div class="info-value break-all">{{ currentDetailsEntity.original_path }}</div></div>
            <div class="file-info-item file-object-type"><div class="info-label font-semibold">Object Type:</div><div class="info-value">{{ detailsObjectType }}</div></div>
            <div class="file-info-item file-mime-type"><div class="info-label font-semibold">File Type:</div><div class="info-value break-all">{{ currentDetailsEntity.mime_type }}</div></div>
            <div class="file-info-item file-extension"><div class="info-label font-semibold">Extension:</div><div class="info-value">{{ currentDetailsEntity.ext }}</div></div>
          </div>
        </div>

        <div class="file-details-actions flex justify-end gap-2 border-t border-border p-4">
          <Button class="btn btn-primary action-download" @click="downloadDetailsFile"><Download class="size-4" /> Download</Button>
          <Button v-if="allowDelete" class="btn btn-danger action-delete" variant="destructive" @click="deleteDetailsFile"><Trash2 class="size-4" /> Delete</Button>
          <Button class="btn btn-secondary action-close" variant="secondary" @click="detailsFile = null">Close</Button>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>
