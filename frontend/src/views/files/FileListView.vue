<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Plus, X } from 'lucide-vue-next'

import { Button } from '@design/ui/button'
import { Input } from '@design/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@design/ui/select'
import EmptyState from '@design/patterns/EmptyState.vue'
import FilePreview from '@design/patterns/FilePreview.vue'
import FilterBar from '@design/patterns/FilterBar.vue'
import MediaGallery from '@design/patterns/MediaGallery.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import SearchInput from '@design/patterns/SearchInput.vue'
import { useAppToast } from '@design/composables/useAppToast'
import emptyFilesIllustration from '@design/illustrations/empty-files.svg'

import Confirmation from '@/components/Confirmation.vue'
import PaginationControls from '@/components/PaginationControls.vue'

import fileService, { type FileEntity } from '@/services/fileService'
import { useGroupStore } from '@/stores/groupStore'

interface URLData {
  url: string
  thumbnails?: { small?: string; medium?: string }
}

type EntityIconName = 'commodity' | 'location' | 'export' | 'link'

const router = useRouter()
const route = useRoute()
const groupStore = useGroupStore()
const toast = useAppToast()

const files = ref<FileEntity[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const deleting = ref(false)
const fileUrls = ref<Record<string, string>>({})

const currentPage = ref(1)
const pageSize = ref(20)
const totalFiles = ref(0)
const totalPages = computed(() =>
  Math.ceil(totalFiles.value / pageSize.value),
)

const filters = ref({ search: '', type: '', tags: '' })

const showDeleteModal = ref(false)
const fileToDelete = ref<FileEntity | null>(null)

const fileTypeOptions = fileService.getFileTypeOptions()

const hasActiveFilters = computed(
  () => !!(filters.value.search || filters.value.type || filters.value.tags),
)

const filesEmptyTitle = computed(() =>
  hasActiveFilters.value ? 'No files match your filters' : 'No Files Found',
)

const filesEmptyDescription = computed(() =>
  hasActiveFilters.value
    ? 'No files match your current filters. Try adjusting your search criteria.'
    : "You haven't uploaded any files yet. Upload your first file to get started.",
)

const newFileHref = computed(() => groupStore.groupPath('/files/create'))

async function loadFiles() {
  loading.value = true
  error.value = null
  try {
    const params: Record<string, unknown> = {
      page: currentPage.value,
      limit: pageSize.value,
    }
    if (filters.value.search) params.search = filters.value.search
    if (filters.value.type) params.type = filters.value.type
    if (filters.value.tags) params.tags = filters.value.tags

    const resp = await fileService.getFiles(params)
    files.value = resp.data.data
    totalFiles.value = resp.data.meta.total

    const signed = resp.data.meta.signed_urls as
      | Record<string, URLData>
      | undefined
    const extracted: Record<string, string> = {}
    if (signed) {
      for (const [fileId, urlData] of Object.entries(signed)) {
        if (urlData.thumbnails?.medium) extracted[fileId] = urlData.thumbnails.medium
        else if (urlData.thumbnails?.small) extracted[fileId] = urlData.thumbnails.small
        else extracted[fileId] = urlData.url
      }
    }
    fileUrls.value = extracted
  } catch (err: any) {
    error.value = err?.response?.data?.message ?? err?.message ?? 'Failed to load files'
  } finally {
    loading.value = false
  }
}

let searchTimeout: number | null = null
function debounceReload() {
  if (searchTimeout !== null) clearTimeout(searchTimeout)
  searchTimeout = window.setTimeout(() => {
    currentPage.value = 1
    loadFiles()
  }, 500)
}

function clearFilters() {
  filters.value = { search: '', type: '', tags: '' }
  currentPage.value = 1
  loadFiles()
}

function getThumbnailUrl(file: FileEntity): string | undefined {
  if (file.type !== 'image') return undefined
  return fileUrls.value[file.id] || undefined
}

function entityIconName(file: FileEntity): EntityIconName {
  switch (file.linked_entity_type) {
    case 'commodity':
      return 'commodity'
    case 'location':
      return 'location'
    case 'export':
      return 'export'
    default:
      return 'link'
  }
}

function linkedEntityFor(file: FileEntity) {
  if (!fileService.isLinked(file)) return undefined
  const url = fileService.getLinkedEntityUrl(file, route)
  if (!url) return undefined
  return {
    display: fileService.getLinkedEntityDisplay(file),
    url,
    icon: entityIconName(file),
  }
}

async function onImageError(file: FileEntity, event: Event) {
  const img = event.target as HTMLImageElement
  try {
    const response = await fileService.generateSignedUrlWithThumbnails(file)
    if (response.thumbnails?.medium) img.src = response.thumbnails.medium
    else if (response.thumbnails?.small) img.src = response.thumbnails.small
    else img.src = response.url
    fileUrls.value = { ...fileUrls.value, [file.id]: img.src }
  } catch (err) {
    console.error('Failed to refresh URL for file:', file.id, err)
    img.style.display = 'none'
  }
}

function viewFile(file: FileEntity) {
  router.push(groupStore.groupPath(`/files/${file.id}`))
}
function editFile(file: FileEntity) {
  router.push(groupStore.groupPath(`/files/${file.id}/edit`))
}

async function downloadFile(file: FileEntity) {
  try {
    await fileService.downloadFile(file)
  } catch (err: any) {
    toast.error(err?.message ?? 'Failed to download file')
  }
}

function confirmDelete(file: FileEntity) {
  if (!fileService.canDelete(file)) return
  fileToDelete.value = file
  showDeleteModal.value = true
}

function cancelDelete() {
  fileToDelete.value = null
  showDeleteModal.value = false
}

async function deleteFile() {
  const file = fileToDelete.value
  if (!file) return
  deleting.value = true
  try {
    await fileService.deleteFile(file.id)
    await loadFiles()
    cancelDelete()
  } catch (err: any) {
    error.value = err?.response?.data?.message ?? err?.message ?? 'Failed to delete file'
    toast.error(error.value)
  } finally {
    deleting.value = false
  }
}

watch(
  () => route.query.page,
  (np) => {
    const page = np && typeof np === 'string' ? parseInt(np, 10) : 1
    if (page > 0 && page !== currentPage.value) {
      currentPage.value = page
      loadFiles()
    }
  },
  { immediate: true },
)

onMounted(() => {
  const pageParam = route.query.page
  if (pageParam && typeof pageParam === 'string') {
    const page = parseInt(pageParam, 10)
    if (page > 0) currentPage.value = page
  }
  loadFiles()
})
</script>

<template>
  <PageContainer>
    <PageHeader title="Files">
      <template #description>
        <span v-if="!loading && files.length > 0">
          {{ totalFiles }} file{{ totalFiles !== 1 ? 's' : '' }}
        </span>
      </template>
      <template #actions>
        <Button as-child>
          <router-link :to="newFileHref">
            <Plus class="size-4" aria-hidden="true" />
            Upload File
          </router-link>
        </Button>
      </template>
    </PageHeader>

    <FilterBar class="mb-6">
      <template #search>
        <SearchInput
          v-model="filters.search"
          placeholder="Search files..."
          aria-label="Search files"
          @update:model-value="debounceReload"
        />
      </template>

      <Select v-model="filters.type" @update:model-value="loadFiles">
        <SelectTrigger
          aria-label="Filter by file type"
          class="w-44"
        >
          <SelectValue placeholder="All Types" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="">All Types</SelectItem>
          <SelectItem
            v-for="opt in fileTypeOptions"
            :key="opt.value"
            :value="opt.value"
          >
            {{ opt.label }}
          </SelectItem>
        </SelectContent>
      </Select>

      <Input
        v-model="filters.tags"
        placeholder="Comma-separated tags"
        aria-label="Filter by tags"
        class="w-56"
        @input="debounceReload"
      />

      <template #actions>
        <Button variant="outline" @click="clearFilters">
          <X class="size-4" aria-hidden="true" />
          Clear
        </Button>
      </template>
    </FilterBar>

    <div
      v-if="loading"
      class="rounded-md border border-border bg-card p-12 text-center text-muted-foreground shadow-sm"
    >
      <p>Loading files...</p>
    </div>

    <div
      v-else-if="error"
      class="flex flex-col items-center gap-3 rounded-md border border-destructive/50 bg-destructive/10 p-12 text-center shadow-sm"
    >
      <h3 class="text-lg font-semibold text-destructive">
        Error Loading Files
      </h3>
      <p class="text-sm text-destructive">{{ error }}</p>
      <Button @click="loadFiles">Try Again</Button>
    </div>

    <template v-else-if="files.length > 0">
      <MediaGallery>
        <FilePreview
          v-for="file in files"
          :key="file.id"
          :file="file"
          :thumbnail-url="getThumbnailUrl(file)"
          :linked-entity="linkedEntityFor(file)"
          :can-delete="fileService.canDelete(file)"
          :delete-restriction-reason="fileService.getDeleteRestrictionReason(file)"
          @view="viewFile(file)"
          @download="downloadFile(file)"
          @edit="editFile(file)"
          @delete="confirmDelete(file)"
          @image-error="onImageError(file, $event)"
        />
      </MediaGallery>

      <PaginationControls
        :current-page="currentPage"
        :total-pages="totalPages"
        :page-size="pageSize"
        :total-items="totalFiles"
        item-label="files"
      />
    </template>

    <EmptyState
      v-else
      test-id="files-empty-state"
      :title="filesEmptyTitle"
      :description="filesEmptyDescription"
      :illustration-src="emptyFilesIllustration"
      illustration-alt=""
    >
      <template #actions>
        <Button as-child>
          <router-link :to="newFileHref">
            <Plus class="size-4" aria-hidden="true" />
            Upload File
          </router-link>
        </Button>
      </template>
    </EmptyState>

    <Confirmation
      v-model:visible="showDeleteModal"
      title="Delete File"
      :message="
        `Are you sure you want to delete <strong>${
          fileToDelete ? fileToDelete.title || fileToDelete.path || 'this file' : ''
        }</strong>?<br><br><span class='warning-text'>This action cannot be undone. The file will be permanently deleted.</span>`
      "
      :confirm-label="deleting ? 'Deleting...' : 'Delete'"
      cancel-label="Cancel"
      confirm-button-class="danger"
      :confirm-disabled="deleting"
      confirmation-icon="exclamation-triangle"
      @confirm="deleteFile"
      @cancel="cancelDelete"
    />
  </PageContainer>
</template>
