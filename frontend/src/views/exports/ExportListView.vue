<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Eye, Plus, Trash2, Upload } from 'lucide-vue-next'

import { Button } from '@design/ui/button'
import { Switch } from '@design/ui/switch'
import { Label } from '@design/ui/label'
import ExportStatusPill from '@design/patterns/ExportStatusPill.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import { useAppToast } from '@design/composables/useAppToast'

import Confirmation from '@/components/Confirmation.vue'

import exportService from '@/services/exportService'
import {
  canPerformOperations,
  getExportDisplayStatus,
  isExportDeleted,
} from '@/utils/exportUtils'
import type { Export, ResourceObject } from '@/types'
import { useGroupStore } from '@/stores/groupStore'

const router = useRouter()
const groupStore = useGroupStore()
const toast = useAppToast()

const exports = ref<Export[]>([])
const loading = ref(true)
const error = ref('')
const deleting = ref<string | null>(null)
const downloading = ref<string | null>(null)
const showDeleteDialog = ref(false)
const exportToDelete = ref<string | null>(null)
const showDeleted = ref(false)

const newExportHref = computed(() => groupStore.groupPath('/exports/new'))
const importHref = computed(() => groupStore.groupPath('/exports/import'))

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
  } catch (err: any) {
    error.value =
      err?.response?.data?.errors?.[0]?.detail ?? err?.message ?? 'Failed to load exports'
  } finally {
    loading.value = false
  }
}

function formatExportType(type: string): string {
  const typeMap: Record<string, string> = {
    full_database: 'Full Database',
    selected_items: 'Selected Items',
    locations: 'Locations',
    areas: 'Areas',
    commodities: 'Commodities',
    imported: 'Imported',
  }
  return typeMap[type] ?? type
}

function formatDate(dateString?: string | null): string {
  if (!dateString) return '-'
  try {
    return new Date(dateString).toLocaleString()
  } catch {
    return dateString
  }
}

function viewExport(id: string) {
  router.push(groupStore.groupPath(`/exports/${id}`))
}

function effectiveStatus(item: Export): 'pending' | 'in_progress' | 'completed' | 'failed' | 'deleted' {
  if (isExportDeleted(item)) return 'deleted'
  return item.status as 'pending' | 'in_progress' | 'completed' | 'failed'
}

function statusClasses(item: Export): string[] {
  const status = effectiveStatus(item)
  return ['status-badge', `export-status--${status.replace(/_/g, '-')}`]
}

function typeClasses(item: Export): string[] {
  return ['type-badge', `type-${item.type}`]
}

function deleteExport(id: string) {
  exportToDelete.value = id
  showDeleteDialog.value = true
}

async function onConfirmDelete() {
  if (!exportToDelete.value) return
  const id = exportToDelete.value
  showDeleteDialog.value = false
  exportToDelete.value = null
  try {
    deleting.value = id
    await exportService.deleteExport(id)
    await loadExports()
  } catch (err: any) {
    toast.error(err?.message ?? 'Failed to delete export')
  } finally {
    deleting.value = null
  }
}

function onCancelDelete() {
  showDeleteDialog.value = false
  exportToDelete.value = null
}

async function downloadExport(id: string) {
  try {
    downloading.value = id
    const response = await exportService.downloadExport(id)
    const blob = new Blob([response.data], { type: 'application/xml' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url

    const contentDisposition = response.headers['content-disposition']
    let filename = 'export.xml'
    if (contentDisposition) {
      const match = contentDisposition.match(/filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/)
      if (match) filename = match[1].replace(/['"]/g, '')
    }

    link.download = filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)
  } catch (err: any) {
    toast.error(err?.message ?? 'Failed to download export')
  } finally {
    downloading.value = null
  }
}

let refreshTimer: number | null = null

onMounted(() => {
  loadExports()
  refreshTimer = window.setInterval(() => {
    if (exports.value.some((e) => e.status === 'pending' || e.status === 'in_progress')) {
      loadExports().catch((err) => {
        console.error('Error refreshing exports:', err)
      })
    }
  }, 3000)
})

onBeforeUnmount(() => {
  if (refreshTimer !== null) {
    window.clearInterval(refreshTimer)
    refreshTimer = null
  }
})
</script>

<template>
  <PageContainer>
    <PageHeader title="Exports">
      <template #description>
        <span v-if="!loading && exports.length > 0">
          {{ exports.length }} export{{ exports.length !== 1 ? 's' : '' }}
        </span>
      </template>
      <template #actions>
        <div class="filter-toggle flex items-center gap-2">
          <Switch
            id="show-deleted-exports"
            v-model="showDeleted"
            aria-label="Show deleted exports"
            @update:model-value="loadExports"
          />
          <Label
            for="show-deleted-exports"
            class="cursor-pointer text-sm text-muted-foreground"
          >
            Show deleted exports
          </Label>
        </div>

        <Button as-child variant="outline">
          <router-link :to="importHref">
            <Upload class="size-4" aria-hidden="true" />
            Import
          </router-link>
        </Button>

        <Button as-child>
          <router-link :to="newExportHref">
            <Plus class="size-4" aria-hidden="true" />
            New
          </router-link>
        </Button>
      </template>
    </PageHeader>

    <div
      v-if="loading"
      class="rounded-md border border-border bg-card p-12 text-center text-muted-foreground shadow-sm"
    >
      Loading...
    </div>

    <div
      v-else-if="error"
      class="rounded-md border border-destructive/50 bg-destructive/10 p-12 text-center text-destructive shadow-sm"
    >
      {{ error }}
    </div>

    <div
      v-else-if="exports.length === 0"
      class="flex flex-col items-center gap-4 rounded-md border border-border bg-card p-12 text-center shadow-sm"
    >
      <p class="text-base">No exports found. Create your first export!</p>
      <Button as-child>
        <router-link :to="newExportHref">Create Export</router-link>
      </Button>
    </div>

    <div
      v-else
      class="overflow-hidden rounded-md border border-border bg-card shadow-sm"
    >
      <table class="w-full border-collapse">
        <thead class="bg-muted/50">
          <tr>
            <th class="border-b border-border p-3 text-left text-sm font-semibold text-foreground">
              Description
            </th>
            <th class="border-b border-border p-3 text-left text-sm font-semibold text-foreground">
              Type
            </th>
            <th class="border-b border-border p-3 text-left text-sm font-semibold text-foreground">
              Status
            </th>
            <th class="border-b border-border p-3 text-left text-sm font-semibold text-foreground">
              Created
            </th>
            <th class="border-b border-border p-3 text-left text-sm font-semibold text-foreground">
              Completed
            </th>
            <th class="border-b border-border p-3 text-left text-sm font-semibold text-foreground">
              Actions
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="exportItem in exports"
            :key="exportItem.id"
            :class="[
              'export-row cursor-pointer motion-safe:transition-colors hover:bg-muted/40',
              isExportDeleted(exportItem) && 'opacity-60 bg-muted/30',
            ]"
            @click="viewExport(exportItem.id!)"
          >
            <td class="border-b border-border p-3 align-top">
              <div class="font-medium text-foreground">
                {{ exportItem.description || 'No description' }}
              </div>
              <div
                v-if="exportItem.error_message"
                class="mt-1 text-xs text-destructive"
              >
                Error: {{ exportItem.error_message }}
              </div>
            </td>
            <td class="border-b border-border p-3 align-top">
              <span
                :class="[
                  ...typeClasses(exportItem),
                  'inline-flex items-center rounded-md border border-border bg-muted px-2 py-0.5 text-xs font-semibold uppercase tracking-wide text-muted-foreground',
                ]"
              >
                {{ formatExportType(exportItem.type) }}
              </span>
            </td>
            <td class="border-b border-border p-3 align-top">
              <ExportStatusPill
                :status="effectiveStatus(exportItem)"
                :label="getExportDisplayStatus(exportItem)"
                :class="statusClasses(exportItem)"
              />
            </td>
            <td class="border-b border-border p-3 align-top text-sm text-muted-foreground">
              {{ formatDate(exportItem.created_date) }}
            </td>
            <td class="border-b border-border p-3 align-top text-sm text-muted-foreground">
              {{ exportItem.completed_date ? formatDate(exportItem.completed_date) : '-' }}
            </td>
            <td class="border-b border-border p-3 align-top">
              <div class="flex flex-wrap items-center gap-2" @click.stop>
                <Button as-child variant="outline" size="sm">
                  <router-link :to="groupStore.groupPath(`/exports/${exportItem.id}`)">
                    <Eye class="size-4" aria-hidden="true" />
                    View
                  </router-link>
                </Button>
                <Button
                  v-if="exportItem.status === 'completed' && canPerformOperations(exportItem)"
                  size="sm"
                  :disabled="downloading === exportItem.id"
                  @click="downloadExport(exportItem.id!)"
                >
                  {{ downloading === exportItem.id ? 'Downloading...' : 'Download' }}
                </Button>
                <Button
                  v-if="canPerformOperations(exportItem)"
                  variant="destructive"
                  size="sm"
                  :disabled="deleting === exportItem.id"
                  @click="deleteExport(exportItem.id!)"
                >
                  <Trash2 class="size-4" aria-hidden="true" />
                  {{ deleting === exportItem.id ? 'Deleting...' : 'Delete' }}
                </Button>
                <span
                  v-else-if="isExportDeleted(exportItem)"
                  class="text-xs italic text-muted-foreground"
                >
                  <Trash2 class="inline size-3" aria-hidden="true" />
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
