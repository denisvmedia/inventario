<script setup lang="ts">
/**
 * FileDetailView — migrated to the design system in Phase 4 of
 * Epic #1324 (issue #1329).
 *
 * Standalone file detail page: image / PDF preview, metadata grid,
 * tags, linked-entity badge and the Download / Edit / Delete action
 * row. PDFViewerCanvas is reused as-is (out of scope for the
 * design-system migration). The delete confirmation switched from
 * the legacy `Confirmation` component to the shared `useConfirm`
 * composable so the modal stack matches every other Phase 4 view.
 *
 * Legacy DOM anchors (`.file-detail`, `.breadcrumb-link`,
 * `.file-name`) are preserved as no-op markers so existing
 * Playwright selectors keep resolving — see
 * devdocs/frontend/migration-conventions.md.
 */
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowLeft,
  Box,
  Download,
  ExternalLink,
  FileArchive,
  FileAudio,
  FileText,
  FileVideo,
  FileType,
  Image as ImageIcon,
  Lock,
  MapPin,
  Package,
  Pencil,
  Trash2,
} from 'lucide-vue-next'

import PDFViewerCanvas from '@/components/PDFViewerCanvas.vue'
import fileService, { type FileEntity } from '@/services/fileService'
import {
  is404Error as checkIs404Error,
  get404Message,
  get404Title,
  getErrorMessage,
} from '@/utils/errorUtils'
import { useGroupStore } from '@/stores/groupStore'

import { Button } from '@design/ui/button'
import { Badge } from '@design/ui/badge'
import Banner from '@design/patterns/Banner.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import PageSection from '@design/patterns/PageSection.vue'
import { useAppToast } from '@design/composables/useAppToast'
import { useConfirm } from '@design/composables/useConfirm'

import ResourceNotFound from '@/components/ResourceNotFound.vue'

const route = useRoute()
const router = useRouter()
const groupStore = useGroupStore()
const toast = useAppToast()
const { confirmDelete } = useConfirm()

const file = ref<FileEntity | null>(null)
const loading = ref<boolean>(false)
const error = ref<string | null>(null)
const lastError = ref<unknown>(null)
const fileUrl = ref<string | null>(null)
const fileSize = ref<string | null>(null)

const fileId = computed<string>(() => route.params.id as string)
const is404 = computed<boolean>(
  () => !!lastError.value && checkIs404Error(lastError.value as never),
)

const backLinkText = computed<string>(() =>
  route.query.from === 'export' ? 'Back to Export' : 'Back to Files',
)

const canDeleteFile = computed<boolean>(() =>
  file.value ? fileService.canDelete(file.value) : false,
)
const deleteRestrictionReason = computed<string>(() =>
  file.value ? fileService.getDeleteRestrictionReason(file.value) : '',
)

const fileTypeOptions = fileService.getFileTypeOptions()

function isLinked(f: FileEntity): boolean {
  return fileService.isLinked(f)
}
function getLinkedEntityDisplay(f: FileEntity): string {
  return fileService.getLinkedEntityDisplay(f)
}
function getLinkedEntityUrl(f: FileEntity): string {
  return fileService.getLinkedEntityUrl(f, route)
}
function getDisplayTitle(f: FileEntity): string {
  return fileService.getDisplayTitle(f)
}
function getFileTypeLabel(type: string): string {
  return fileTypeOptions.find((opt) => opt.value === type)?.label ?? type
}
function formatDate(value: string): string {
  return new Date(value).toLocaleString()
}

const FILE_TYPE_ICON = {
  image: ImageIcon,
  document: FileText,
  video: FileVideo,
  audio: FileAudio,
  archive: FileArchive,
  other: FileType,
} as const

function getFileIcon(f: FileEntity) {
  return FILE_TYPE_ICON[f.type] ?? FileType
}

function getEntityIcon(f: FileEntity) {
  if (f.linked_entity_type === 'commodity') return Package
  if (f.linked_entity_type === 'location') return MapPin
  if (f.linked_entity_type === 'export') return Box
  return ExternalLink
}

async function refreshSignedUrl() {
  if (!file.value) return
  try {
    const response = await fileService.generateSignedUrlWithThumbnails(file.value)
    fileUrl.value = response.url
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'file', 'Failed to refresh preview'))
  }
}

function handleImageError() {
  void refreshSignedUrl()
}

function handlePdfError() {
  // PDFViewerCanvas surfaces its own error UI.
}

async function loadFile() {
  loading.value = true
  error.value = null
  lastError.value = null
  fileUrl.value = null
  try {
    const response = await fileService.getFile(fileId.value)
    file.value = response.data.attributes as FileEntity
    if (file.value) {
      const signedUrls = response.data.meta?.signed_urls as
        | Record<string, { url: string }>
        | undefined
      if (signedUrls && signedUrls[file.value.id]) {
        fileUrl.value = signedUrls[file.value.id].url
      } else {
        try {
          fileUrl.value = await fileService.getDownloadUrl(file.value)
        } catch {
          // Download still works without a preview URL.
        }
      }
    }
  } catch (err) {
    lastError.value = err
    if (!checkIs404Error(err as never)) {
      error.value = getErrorMessage(err as never, 'file', 'Failed to load file')
    }
  } finally {
    loading.value = false
  }
}

function goBack() {
  const from = route.query.from as string | undefined
  const exportId = route.query.exportId as string | undefined
  if (from === 'export' && exportId) {
    router.push(groupStore.groupPath(`/exports/${exportId}`))
  } else {
    router.push(groupStore.groupPath('/files'))
  }
}

async function downloadFile() {
  if (!file.value) return
  try {
    await fileService.downloadFile(file.value)
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'file', 'Failed to download file'))
  }
}

function editFile() {
  const from = route.query.from as string | undefined
  const exportId = route.query.exportId as string | undefined
  const base = `/files/${fileId.value}/edit`
  if (from === 'export' && exportId) {
    router.push(groupStore.groupPath(`${base}?from=export&exportId=${exportId}`))
  } else {
    router.push(groupStore.groupPath(base))
  }
}

async function onDelete() {
  if (!file.value || !canDeleteFile.value) return
  const title = getDisplayTitle(file.value)
  const ok = await confirmDelete('file', {
    title: 'Delete File',
    message: `Are you sure you want to delete "${title}"? This action cannot be undone.`,
  })
  if (!ok) return
  try {
    await fileService.deleteFile(file.value.id)
    router.push(groupStore.groupPath('/files'))
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'file', 'Failed to delete file'))
  }
}

onMounted(loadFile)
</script>


<template>
  <PageContainer as="div" class="file-detail mx-auto max-w-5xl">
    <div v-if="loading" class="py-12 text-center text-sm text-muted-foreground">
      Loading file...
    </div>

    <ResourceNotFound
      v-else-if="is404"
      resource-type="file"
      :title="get404Title('file')"
      :message="get404Message('file')"
      :go-back-text="backLinkText"
      @go-back="goBack"
      @try-again="loadFile"
    />

    <Banner v-else-if="error" variant="error" class="mb-4">{{ error }}</Banner>

    <template v-else-if="file">
      <div class="mb-2 text-sm">
        <a
          href="#"
          class="breadcrumb-link inline-flex items-center gap-1 text-primary hover:underline"
          @click.prevent="goBack"
        >
          <ArrowLeft class="size-4" aria-hidden="true" />
          <span>{{ backLinkText }}</span>
        </a>
      </div>

      <PageHeader :title="getDisplayTitle(file)">
        <template #description>
          <div class="file-meta flex flex-wrap items-center gap-2">
            <span class="file-name sr-only">{{ getDisplayTitle(file) }}</span>
            <Badge variant="secondary">{{ getFileTypeLabel(file.type) }}</Badge>
            <Badge v-if="file.ext" variant="outline" class="uppercase">{{ file.ext }}</Badge>
            <span v-if="fileSize" class="text-xs text-muted-foreground">{{ fileSize }}</span>
          </div>
        </template>
        <template #actions>
          <Button variant="outline" @click="downloadFile">
            <Download class="size-4" aria-hidden="true" />
            Download
          </Button>
          <Button @click="editFile">
            <Pencil class="size-4" aria-hidden="true" />
            Edit
          </Button>
          <Button
            v-if="canDeleteFile"
            variant="destructive"
            @click="onDelete"
          >
            <Trash2 class="size-4" aria-hidden="true" />
            Delete
          </Button>
          <Button
            v-else
            variant="outline"
            disabled
            :title="deleteRestrictionReason"
          >
            <Lock class="size-4" aria-hidden="true" />
            Delete
          </Button>
        </template>
      </PageHeader>

      <!-- Preview -->
      <PageSection class="mb-6">
        <div class="flex min-h-[16rem] items-center justify-center rounded-md border border-border bg-muted/30 p-4">
          <div v-if="file.type === 'image'" class="image-preview w-full">
            <img
              v-if="fileUrl"
              :src="fileUrl"
              :alt="getDisplayTitle(file)"
              :data-file-id="file.id"
              class="preview-image mx-auto max-h-[60vh] max-w-full object-contain"
              @error="handleImageError"
            />
            <div v-else class="py-12 text-center text-sm text-muted-foreground">
              Loading preview...
            </div>
          </div>
          <div v-else-if="file.mime_type === 'application/pdf'" class="pdf-preview w-full">
            <PDFViewerCanvas
              v-if="fileUrl"
              :url="fileUrl"
              @error="handlePdfError"
            />
            <div v-else class="py-12 text-center text-sm text-muted-foreground">
              Loading preview...
            </div>
          </div>
          <div v-else class="file-placeholder flex flex-col items-center gap-3 py-8 text-center">
            <component :is="getFileIcon(file)" class="size-16 text-muted-foreground" aria-hidden="true" />
            <p class="text-sm text-muted-foreground">Preview not available for this file type</p>
            <Button @click="downloadFile">
              <Download class="size-4" aria-hidden="true" />
              Download to View
            </Button>
          </div>
        </div>
      </PageSection>

      <!-- Information cards -->
      <div class="grid gap-4 sm:grid-cols-2">
        <PageSection title="Description">
          <p v-if="file.description" class="text-sm text-foreground">{{ file.description }}</p>
          <p v-else class="text-sm italic text-muted-foreground">No description provided</p>
        </PageSection>

        <PageSection title="Tags">
          <div v-if="file.tags && file.tags.length > 0" class="tags-list flex flex-wrap gap-1.5">
            <Badge v-for="tag in file.tags" :key="tag" variant="secondary" class="tag">
              {{ tag }}
            </Badge>
          </div>
          <p v-else class="text-sm italic text-muted-foreground">No tags</p>
        </PageSection>

        <PageSection v-if="isLinked(file)" title="Linked Entity">
          <router-link
            :to="getLinkedEntityUrl(file)"
            class="entity-badge inline-flex items-center gap-2 rounded-md border border-border bg-muted/40 px-3 py-2 text-sm text-foreground hover:bg-muted"
            title="View linked entity"
          >
            <component :is="getEntityIcon(file)" class="size-4" aria-hidden="true" />
            <span class="entity-text">{{ getLinkedEntityDisplay(file) }}</span>
            <ExternalLink class="size-4 text-muted-foreground" aria-hidden="true" />
          </router-link>
        </PageSection>

        <PageSection title="File Details">
          <dl class="grid gap-2 text-sm">
            <div class="flex flex-wrap items-baseline gap-2">
              <dt class="font-medium text-muted-foreground">Original Name:</dt>
              <dd class="break-all text-foreground">{{ file.original_path }}</dd>
            </div>
            <div class="flex flex-wrap items-baseline gap-2">
              <dt class="font-medium text-muted-foreground">MIME Type:</dt>
              <dd class="break-all text-foreground">{{ file.mime_type }}</dd>
            </div>
            <div v-if="file.created_at" class="flex flex-wrap items-baseline gap-2">
              <dt class="font-medium text-muted-foreground">Uploaded:</dt>
              <dd class="text-foreground">{{ formatDate(file.created_at) }}</dd>
            </div>
            <div
              v-if="file.updated_at && file.updated_at !== file.created_at"
              class="flex flex-wrap items-baseline gap-2"
            >
              <dt class="font-medium text-muted-foreground">Modified:</dt>
              <dd class="text-foreground">{{ formatDate(file.updated_at) }}</dd>
            </div>
          </dl>
        </PageSection>
      </div>
    </template>
  </PageContainer>
</template>

