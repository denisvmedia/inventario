<script setup lang="ts">
/**
 * ExportListView — migrated to the design system in Phase 4 of
 * Epic #1324 (issue #1329).
 *
 * Lists all exports for the current group with filter (deleted / live),
 * inline download / delete row actions, and 3 s auto-refresh while any
 * export is pending or in progress.
 *
 * Legacy DOM anchors preserved for Playwright stability:
 *   - `.export-list`, `.export-row`, `.export-row.deleted`
 *   - `.status-badge.export-status--<status>` on the row pill
 *   - `.type-badge.type-<type>` on the type pill
 *   - `.bool-badge` on simple yes/no pills (used by other views)
 *   - `<table>` markup with `<tr>`/`<td>` per `exports-crud.spec.ts`
 */
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { Download, Eye, Loader2, Plus, Trash2, Upload } from 'lucide-vue-next'

import Confirmation from '@/components/Confirmation.vue'
import exportService from '@/services/exportService'
import {
  isExportDeleted,
  canPerformOperations,
  getExportDisplayStatus,
} from '@/utils/exportUtils'
import { getErrorMessage } from '@/utils/errorUtils'
import type { Export, ResourceObject } from '@/types'
import { useGroupStore } from '@/stores/groupStore'

import { Button } from '@design/ui/button'
import { Label } from '@design/ui/label'
import { Switch } from '@design/ui/switch'
import Banner from '@design/patterns/Banner.vue'
import EmptyState from '@design/patterns/EmptyState.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import { EXPORT_STATUS_LABELS, type ExportStatus } from '@design/patterns/ExportStatusPill.vue'

const router = useRouter()
const groupStore = useGroupStore()

const exports = ref<Export[]>([])
const loading = ref<boolean>(true)
const error = ref<string>('')
const deleting = ref<string | null>(null)
const downloading = ref<string | null>(null)
const showDeleteDialog = ref<boolean>(false)
const exportToDelete = ref<string | null>(null)
const showDeleted = ref<boolean>(false)

let refreshTimer: ReturnType<typeof setInterval> | null = null

const TYPE_LABELS: Record<string, string> = {
  full_database: 'Full Database',
  selected_items: 'Selected Items',
  locations: 'Locations',
  areas: 'Areas',
  commodities: 'Commodities',
  imported: 'Imported',
}

function formatExportType(type: string) {
  return TYPE_LABELS[type] || type
}

function formatDate(dateString?: string | null) {
  if (!dateString) return '-'
  try {
    return new Date(dateString).toLocaleString()
  } catch {
    return dateString
  }
}

function rowStatusClass(item: Export): string {
  const status = isExportDeleted(item)
    ? 'deleted'
    : ((item.status || 'pending') as ExportStatus)
  return `status-badge export-status export-status--${status}`
}

function rowStatusLabel(item: Export): string {
  if (isExportDeleted(item)) return EXPORT_STATUS_LABELS.deleted
  return getExportDisplayStatus(item)
}

async function loadExports() {
  try {
    loading.value = true
    error.value = ''
    const response = await exportService.getExports(showDeleted.value)
    if (response.data?.data) {
      exports.value = response.data.data.map((item: ResourceObject<Export>) => ({
        id: item.id,
        ...item.attributes,
      }))
    }
  } catch (err) {
    error.value = getErrorMessage(err as never, 'export', 'Failed to load exports')
  } finally {
    loading.value = false
  }
}

function viewExport(exportId: string) {
  router.push(groupStore.groupPath(`/exports/${exportId}`))
}

function deleteExport(exportId: string) {
  exportToDelete.value = exportId
  showDeleteDialog.value = true
}

async function onConfirmDelete() {
  if (!exportToDelete.value) return
  try {
    deleting.value = exportToDelete.value
    await exportService.deleteExport(exportToDelete.value)
    await loadExports()
  } catch (err) {
    error.value = getErrorMessage(err as never, 'export', 'Failed to delete export')
  } finally {
    deleting.value = null
    exportToDelete.value = null
    showDeleteDialog.value = false
  }
}

function onCancelDelete() {
  exportToDelete.value = null
  showDeleteDialog.value = false
}

async function downloadExport(exportId: string) {
  try {
    downloading.value = exportId
    const response = await exportService.downloadExport(exportId)

    const blob = new Blob([response.data], { type: 'application/xml' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url

    const contentDisposition = response.headers['content-disposition']
    let filename = 'export.xml'
    if (contentDisposition) {
      const filenameMatch = contentDisposition.match(/filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/)
      if (filenameMatch) {
        filename = filenameMatch[1].replace(/['"]/g, '')
      }
    }

    link.download = filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)
  } catch (err) {
    error.value = getErrorMessage(err as never, 'export', 'Failed to download export')
  } finally {
    downloading.value = null
  }
}

onMounted(() => {
  loadExports()
  refreshTimer = setInterval(() => {
    const inProgress = exports.value.filter(
      e => e.status === 'pending' || e.status === 'in_progress',
    )
    if (inProgress.length > 0) {
      loadExports().catch(() => {
        /* swallow — error already surfaced by loadExports */
      })
    }
  }, 3000)
})

onBeforeUnmount(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
})
</script>

<template>
  <PageContainer as="div" class="export-list">
    <PageHeader title="Exports">
      <template #description>
        <span v-if="!loading && exports.length > 0" class="export-count text-sm text-muted-foreground">
          {{ exports.length }} export{{ exports.length !== 1 ? 's' : '' }}
        </span>
      </template>
      <template #actions>
        <div class="filter-toggle flex items-center gap-2">
          <Switch
            id="export-list-show-deleted"
            v-model="showDeleted"
            data-testid="export-list-show-deleted"
            @update:model-value="loadExports"
          />
          <Label
            for="export-list-show-deleted"
            class="toggle-label cursor-pointer text-sm text-muted-foreground"
          >
            Show deleted exports
          </Label>
        </div>
        <Button
          variant="outline"
          as-child
          class="new-import-button"
        >
          <router-link :to="groupStore.groupPath('/exports/import')">
            <Upload class="size-4" aria-hidden="true" />
            Import
          </router-link>
        </Button>
        <Button as-child class="new-export-button">
          <router-link :to="groupStore.groupPath('/exports/new')">
            <Plus class="size-4" aria-hidden="true" />
            New
          </router-link>
        </Button>
      </template>
    </PageHeader>

    <div v-if="loading" class="loading py-12 text-center text-sm text-muted-foreground">
      Loading...
    </div>

    <Banner v-else-if="error" variant="error" class="mt-2">{{ error }}</Banner>

    <EmptyState
      v-else-if="exports.length === 0"
      class="empty mt-2"
      title="No exports yet"
      description="Create your first export to capture a snapshot of your inventory."
    >
      <template #actions>
        <Button as-child>
          <router-link :to="groupStore.groupPath('/exports/new')">
            <Plus class="size-4" aria-hidden="true" />
            Create Export
          </router-link>
        </Button>
      </template>
    </EmptyState>

    <div v-else class="exports-table mt-2 overflow-hidden rounded-md border bg-card shadow-sm">
      <table class="table w-full border-collapse text-sm">
        <thead class="bg-muted/50">
          <tr>
            <th class="px-3 py-3 text-left font-semibold">Description</th>
            <th class="px-3 py-3 text-left font-semibold">Type</th>
            <th class="px-3 py-3 text-left font-semibold">Status</th>
            <th class="px-3 py-3 text-left font-semibold">Created</th>
            <th class="px-3 py-3 text-left font-semibold">Completed</th>
            <th class="px-3 py-3 text-left font-semibold">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="exportItem in exports"
            :key="exportItem.id"
            :class="[
              'export-row cursor-pointer border-t transition-colors hover:bg-muted/40',
              { deleted: isExportDeleted(exportItem) },
              isExportDeleted(exportItem) ? 'opacity-60 bg-muted/30' : '',
            ]"
            @click="viewExport(exportItem.id!)"
          >
            <td class="export-description px-3 py-3">
              <div class="description-text font-medium text-foreground">
                {{ exportItem.description || 'No description' }}
              </div>
              <div v-if="exportItem.error_message" class="error-message mt-1 text-xs text-destructive">
                Error: {{ exportItem.error_message }}
              </div>
            </td>
            <td class="export-type px-3 py-3">
              <span
                :class="[
                  'type-badge inline-flex items-center rounded-full bg-muted px-2 py-0.5 text-xs font-medium uppercase',
                  `type-${exportItem.type}`,
                ]"
              >
                {{ formatExportType(exportItem.type) }}
              </span>
            </td>
            <td class="export-status px-3 py-3">
              <span
                :class="[
                  rowStatusClass(exportItem),
                  'inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium',
                ]"
              >
                {{ rowStatusLabel(exportItem) }}
              </span>
            </td>
            <td class="export-date px-3 py-3 text-muted-foreground">
              {{ formatDate(exportItem.created_date) }}
            </td>
            <td class="export-date px-3 py-3 text-muted-foreground">
              {{ exportItem.completed_date ? formatDate(exportItem.completed_date) : '-' }}
            </td>
            <td class="export-actions whitespace-nowrap px-3 py-3">
              <div class="flex flex-wrap items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  as-child
                  @click.stop
                >
                  <router-link :to="groupStore.groupPath(`/exports/${exportItem.id}`)">
                    <Eye class="size-4" aria-hidden="true" />
                    View
                  </router-link>
                </Button>
                <Button
                  v-if="exportItem.status === 'completed' && canPerformOperations(exportItem)"
                  size="sm"
                  :disabled="downloading === exportItem.id"
                  @click.stop="downloadExport(exportItem.id!)"
                >
                  <Loader2
                    v-if="downloading === exportItem.id"
                    class="size-4 motion-safe:animate-spin"
                    aria-hidden="true"
                  />
                  <Download v-else class="size-4" aria-hidden="true" />
                  {{ downloading === exportItem.id ? 'Downloading...' : 'Download' }}
                </Button>
                <Button
                  v-if="canPerformOperations(exportItem)"
                  variant="destructive"
                  size="sm"
                  :disabled="deleting === exportItem.id"
                  @click.stop="deleteExport(exportItem.id!)"
                >
                  <Trash2 class="size-4" aria-hidden="true" />
                  {{ deleting === exportItem.id ? 'Deleting...' : 'Delete' }}
                </Button>
                <span
                  v-else-if="isExportDeleted(exportItem)"
                  class="deleted-indicator inline-flex items-center gap-1 text-xs italic text-muted-foreground"
                >
                  <Trash2 class="size-3.5" aria-hidden="true" />
                  Deleted
                </span>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <Confirmation
      v-model:visible="showDeleteDialog"
      title="Confirm Delete"
      message="Are you sure you want to delete this export?"
      confirm-label="Delete"
      cancel-label="Cancel"
      confirm-button-class="danger"
      confirmation-icon="exclamation-triangle"
      @confirm="onConfirmDelete"
      @cancel="onCancelDelete"
    />
  </PageContainer>
</template>

