<script setup lang="ts">
/**
 * FileListView — migrated to the design system in Phase 4 of Epic
 * #1324 (issue #1329).
 *
 * Page chrome (header, filter bar, file grid, empty state,
 * pagination) is composed from `@design/*` patterns. Per-card
 * markup is kept inline because file cards have a unique preview /
 * metadata / linked-entity layout that is not shared with the
 * area / commodity grids — we'll extract a `FileCard` pattern when
 * a second consumer materialises.
 *
 * Legacy DOM anchors (`.file-list`, `.file-card`, `.file-thumbnail`,
 * `.file-upload-button`, `.files-grid`) are preserved as no-op
 * markers so existing Playwright selectors keep resolving — see
 * devdocs/frontend/migration-conventions.md.
 */
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Box,
  Download,
  ExternalLink,
  FileArchive,
  FileAudio,
  FileText,
  FileType,
  FileVideo,
  Image as ImageIcon,
  Lock,
  MapPin,
  Package,
  Pencil,
  Plus,
  Trash2,
  X,
} from 'lucide-vue-next'

import fileService, { type FileEntity } from '@/services/fileService'
import { useGroupStore } from '@/stores/groupStore'
import { getErrorMessage } from '@/utils/errorUtils'

import { Badge } from '@design/ui/badge'
import { Button } from '@design/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@design/ui/select'
import Banner from '@design/patterns/Banner.vue'
import EmptyState from '@design/patterns/EmptyState.vue'
import FilterBar from '@design/patterns/FilterBar.vue'
import IconButton from '@design/patterns/IconButton.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import SearchInput from '@design/patterns/SearchInput.vue'
import emptyFilesIllustration from '@design/illustrations/empty-files.svg'
import { useAppToast } from '@design/composables/useAppToast'
import { useConfirm } from '@design/composables/useConfirm'

import PaginationControls from '@/components/PaginationControls.vue'

interface URLData {
  url: string
  thumbnails?: { small?: string; medium?: string }
}

const router = useRouter()
const route = useRoute()
const groupStore = useGroupStore()
const toast = useAppToast()
const { confirmDelete } = useConfirm()

const files = ref<FileEntity[]>([])
const loading = ref<boolean>(false)
const error = ref<string | null>(null)
const fileUrls = ref<Record<string, string>>({})

const currentPage = ref(1)
const pageSize = ref(20)
const totalFiles = ref(0)
const totalPages = computed(() => Math.ceil(totalFiles.value / pageSize.value))

const SEARCH_DEBOUNCE_MS = 500

const filters = ref({
  search: '',
  type: '',
  tags: '',
})

const TYPE_FILTER_ALL = '__all__'
const typeFilter = computed<string>({
  get: () => filters.value.type || TYPE_FILTER_ALL,
  set: (v: string) => {
    filters.value.type = v === TYPE_FILTER_ALL ? '' : v
    currentPage.value = 1
    loadFiles()
  },
})

const fileTypeOptions = fileService.getFileTypeOptions()

const hasActiveFilters = computed(
  () => !!(filters.value.search || filters.value.type || filters.value.tags),
)

const FILE_TYPE_ICON = {
  image: ImageIcon,
  document: FileText,
  video: FileVideo,
  audio: FileAudio,
  archive: FileArchive,
  other: FileType,
} as const

function getFileIconComp(file: FileEntity) {
  return FILE_TYPE_ICON[file.type] ?? FileType
}

function getEntityIconComp(file: FileEntity) {
  if (file.linked_entity_type === 'commodity') return Package
  if (file.linked_entity_type === 'location') return MapPin
  if (file.linked_entity_type === 'export') return Box
  return ExternalLink
}

function isLinked(file: FileEntity): boolean {
  return fileService.isLinked(file)
}
function getLinkedEntityDisplay(file: FileEntity): string {
  return fileService.getLinkedEntityDisplay(file)
}
function getLinkedEntityUrl(file: FileEntity) {
  return fileService.getLinkedEntityUrl(file, route)
}
function getDisplayTitle(file: FileEntity): string {
  return fileService.getDisplayTitle(file)
}
function getFileTypeLabel(type: string): string {
  return fileTypeOptions.find((opt) => opt.value === type)?.label ?? type
}
function getFileUrl(file: FileEntity): string {
  return fileUrls.value[file.id] ?? ''
}
function canDeleteFile(file: FileEntity): boolean {
  return fileService.canDelete(file)
}
function getDeleteRestrictionReason(file: FileEntity): string {
  return fileService.getDeleteRestrictionReason(file)
}

async function onImageError(event: Event) {
  const img = event.target as HTMLImageElement
  const fileId = img.dataset.fileId
  if (!fileId) return
  const file = files.value.find((f) => f.id === fileId)
  if (!file) return
  try {
    const response = await fileService.generateSignedUrlWithThumbnails(file)
    if (response.thumbnails?.medium) {
      img.src = response.thumbnails.medium
    } else if (response.thumbnails?.small) {
      img.src = response.thumbnails.small
    } else {
      img.src = response.url
    }
    fileUrls.value[fileId] = img.src
  } catch {
    img.style.display = 'none'
  }
}

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

    const response = await fileService.getFiles(params)
    files.value = response.data.data
    totalFiles.value = response.data.meta.total

    const signed = response.data.meta.signed_urls as
      | Record<string, URLData>
      | undefined
    if (signed) {
      const extracted: Record<string, string> = {}
      for (const [fileId, urlData] of Object.entries(signed)) {
        if (urlData.thumbnails?.medium) {
          extracted[fileId] = urlData.thumbnails.medium
        } else if (urlData.thumbnails?.small) {
          extracted[fileId] = urlData.thumbnails.small
        } else {
          extracted[fileId] = urlData.url
        }
      }
      fileUrls.value = extracted
    } else {
      fileUrls.value = {}
    }
  } catch (err) {
    error.value = getErrorMessage(err as never, 'file', 'Failed to load files')
  } finally {
    loading.value = false
  }
}

let searchTimeout: number | undefined
function debouncedSearch() {
  if (searchTimeout !== undefined) window.clearTimeout(searchTimeout)
  searchTimeout = window.setTimeout(() => {
    currentPage.value = 1
    loadFiles()
  }, SEARCH_DEBOUNCE_MS)
}

function clearFilters() {
  filters.value = { search: '', type: '', tags: '' }
  currentPage.value = 1
  loadFiles()
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
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'file', 'Failed to download file'))
  }
}

async function onDeleteFile(file: FileEntity) {
  if (!canDeleteFile(file)) return
  const ok = await confirmDelete('file', {
    title: 'Delete File',
    message: `Are you sure you want to delete "${getDisplayTitle(file)}"? This action cannot be undone.`,
  })
  if (!ok) return
  try {
    await fileService.deleteFile(file.id)
    await loadFiles()
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'file', 'Failed to delete file'))
  }
}

function goToCreate() {
  router.push(groupStore.groupPath('/files/create'))
}

watch(
  () => route.query.page,
  (newPage) => {
    const page = newPage && typeof newPage === 'string' ? parseInt(newPage, 10) : 1
    if (page > 0 && page !== currentPage.value) {
      currentPage.value = page
      loadFiles()
    }
  },
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
  <PageContainer as="div" class="file-list">
    <PageHeader title="Files">
      <template #description>
        <span v-if="!loading && totalFiles > 0" class="item-count text-sm text-muted-foreground">
          {{ totalFiles }} file{{ totalFiles !== 1 ? 's' : '' }}
        </span>
      </template>
      <template #actions>
        <Button class="file-upload-button" @click="goToCreate">
          <Plus class="size-4" aria-hidden="true" />
          Upload File
        </Button>
      </template>
    </PageHeader>

    <FilterBar class="mb-6">
      <template #search>
        <SearchInput
          v-model="filters.search"
          placeholder="Search files..."
          aria-label="Search files"
          @update:model-value="debouncedSearch"
        />
      </template>

      <Select v-model="typeFilter">
        <SelectTrigger class="w-44" aria-label="Filter by type">
          <SelectValue placeholder="All Types" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem :value="TYPE_FILTER_ALL">All Types</SelectItem>
          <SelectItem
            v-for="option in fileTypeOptions"
            :key="option.value"
            :value="option.value"
          >
            {{ option.label }}
          </SelectItem>
        </SelectContent>
      </Select>

      <input
        v-model="filters.tags"
        type="text"
        placeholder="Comma-separated tags"
        aria-label="Filter by tags"
        class="h-9 w-56 rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs transition-[color,box-shadow] outline-none focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]"
        @input="debouncedSearch"
      />

      <template #actions>
        <Button v-if="hasActiveFilters" variant="ghost" size="sm" @click="clearFilters">
          <X class="size-4" aria-hidden="true" />
          Clear
        </Button>
      </template>
    </FilterBar>

    <Banner v-if="error" variant="error" class="mb-4">{{ error }}</Banner>

    <div v-if="loading" class="py-12 text-center text-sm text-muted-foreground">
      Loading files...
    </div>

    <template v-else-if="files.length > 0">
      <div class="files-grid grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        <div
          v-for="file in files"
          :key="file.id"
          class="file-card group relative flex cursor-pointer flex-col overflow-hidden rounded-md border border-border bg-card shadow-sm transition hover:border-primary/50 hover:shadow-md"
          @click="viewFile(file)"
        >
          <div class="file-preview flex h-40 items-center justify-center overflow-hidden bg-muted/40">
            <img
              v-if="file.type === 'image' && getFileUrl(file)"
              :src="getFileUrl(file)"
              :alt="getDisplayTitle(file)"
              :data-file-id="file.id"
              class="file-thumbnail h-full w-full object-cover"
              @error="onImageError"
            />
            <component
              :is="getFileIconComp(file)"
              v-else
              class="file-icon size-14 text-muted-foreground"
              aria-hidden="true"
            />
          </div>

          <div class="file-info flex min-w-0 flex-1 flex-col gap-2 p-3">
            <h3
              class="file-title m-0 truncate text-sm font-semibold text-foreground"
              :title="getDisplayTitle(file)"
            >
              {{ getDisplayTitle(file) }}
            </h3>
            <p
              class="file-description line-clamp-2 m-0 text-xs text-muted-foreground"
              :title="file.description || ''"
            >
              {{ file.description || 'No description' }}
            </p>

            <div class="file-meta flex flex-wrap items-center gap-1.5">
              <Badge variant="secondary">{{ getFileTypeLabel(file.type) }}</Badge>
              <Badge v-if="file.ext" variant="outline" class="uppercase">{{ file.ext }}</Badge>
            </div>

            <div
              v-if="file.tags && file.tags.length > 0"
              class="file-tags flex flex-wrap items-center gap-1"
            >
              <Badge
                v-for="tag in file.tags.slice(0, 3)"
                :key="tag"
                variant="outline"
                class="tag"
              >
                {{ tag }}
              </Badge>
              <span
                v-if="file.tags.length > 3"
                class="tag-more text-xs text-muted-foreground"
              >
                +{{ file.tags.length - 3 }} more
              </span>
            </div>

            <div v-if="isLinked(file)" class="file-linked-entity">
              <router-link
                :to="getLinkedEntityUrl(file)"
                class="entity-badge-small inline-flex max-w-full items-center gap-1.5 rounded-md border border-border bg-muted/40 px-2 py-1 text-xs text-foreground hover:bg-muted"
                title="View linked entity"
                @click.stop
              >
                <component :is="getEntityIconComp(file)" class="size-3.5 shrink-0" aria-hidden="true" />
                <span class="entity-text truncate">{{ getLinkedEntityDisplay(file) }}</span>
                <ExternalLink class="entity-link-icon size-3 shrink-0 text-muted-foreground" aria-hidden="true" />
              </router-link>
            </div>
          </div>

          <div
            class="file-actions absolute right-2 top-2 flex items-center gap-1 rounded-md bg-background/80 p-1 opacity-0 shadow-sm backdrop-blur-sm transition group-hover:opacity-100 focus-within:opacity-100"
            @click.stop
          >
            <IconButton aria-label="Download" @click="downloadFile(file)">
              <Download class="size-4" aria-hidden="true" />
            </IconButton>
            <IconButton aria-label="Edit" @click="editFile(file)">
              <Pencil class="size-4" aria-hidden="true" />
            </IconButton>
            <IconButton
              v-if="canDeleteFile(file)"
              aria-label="Delete"
              variant="ghost"
              class="text-destructive hover:bg-destructive/10 hover:text-destructive"
              @click="onDeleteFile(file)"
            >
              <Trash2 class="size-4" aria-hidden="true" />
            </IconButton>
            <IconButton
              v-else
              aria-label="Delete (locked)"
              :title="getDeleteRestrictionReason(file)"
              disabled
            >
              <Lock class="size-4" aria-hidden="true" />
            </IconButton>
          </div>
        </div>
      </div>

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
      class="empty"
      test-id="files-empty-state"
      :title="hasActiveFilters ? 'No files match your filters' : 'No Files Found'"
      :description="hasActiveFilters
        ? 'No files match your current filters. Try adjusting your search criteria.'
        : `You haven't uploaded any files yet. Upload your first file to get started.`"
      :illustration-src="emptyFilesIllustration"
      illustration-alt=""
    >
      <template #actions>
        <Button class="file-upload-button" @click="goToCreate">
          <Plus class="size-4" aria-hidden="true" />
          Upload File
        </Button>
      </template>
    </EmptyState>
  </PageContainer>
</template>

