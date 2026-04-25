<script setup lang="ts">
/**
 * ExportDetailView — migrated to the design system in Phase 4 of
 * Epic #1324 (issue #1329).
 *
 * Renders all the metadata, statistics, selected-item hierarchy and
 * restore operation history for a single export, plus the download /
 * retry / delete / restore actions. Replaces the legacy SCSS / PrimeVue
 * markup with design-system primitives (PageContainer / PageHeader /
 * PageSection / Banner / Button / ExportStatusPill) and Tailwind v4
 * utilities. Lucide icons replace FontAwesome.
 *
 * Legacy DOM anchors preserved verbatim because Playwright suites
 * (`exports-crud.spec.ts`, `e2e/tests/includes/exports.ts`) drive the
 * page by class names and headings:
 *   .export-detail-page, .breadcrumb-link,
 *   .card-header, .status-badge.export-status--<status>,
 *   .info-item > label, .file-path, .bool-badge,
 *   .type-badge, .count-badge, .selected-items-hierarchy,
 *   .location-item / .area-item / .commodity-item, .item-name,
 *   .item-type, .confirmation-modal,
 *   h1 "Export Details", h2 "Export Information",
 *   h2 "Selected Items", h2 "Restore Operations",
 *   button "Download" / "Delete" / "Download Export".
 */
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowLeft,
  ChevronDown,
  ChevronUp,
  Download,
  ExternalLink,
  FileDown,
  Loader2,
  RotateCcw,
  Trash2,
  TriangleAlert,
  Upload,
} from 'lucide-vue-next'

import exportService from '@/services/exportService'
import {
  isExportDeleted,
  canPerformOperations,
  getExportDisplayStatus,
  getExportStatusClasses,
} from '@/utils/exportUtils'
import type { Export } from '@/types'
import AppConfirmDialog from '@design/patterns/AppConfirmDialog.vue'
import { useGroupStore } from '@/stores/groupStore'

import { Button } from '@design/ui/button'
import Banner from '@design/patterns/Banner.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import PageSection from '@design/patterns/PageSection.vue'

const route = useRoute()
const router = useRouter()
const groupStore = useGroupStore()

const exportData = ref<Export | null>(null)
const loading = ref(true)
const error = ref('')
const retrying = ref(false)
const deleting = ref(false)
const downloading = ref(false)
const showDeleteDialog = ref(false)
const loadingItems = ref(false)
const selectedItemsDetails = ref<Array<{ id: string; name: string; type: string }>>([])
const restoreOperations = ref<Array<any>>([])
const expandedRestoreOperations = ref<Record<number, boolean>>({})
const hierarchicalItems = ref<{
  locations: Array<{
    id: string
    name: string
    includeAll: boolean
    areas: Array<{
      id: string
      name: string
      includeAll: boolean
      commodities: Array<{ id: string; name: string }>
    }>
  }>
  standaloneAreas: Array<{
    id: string
    name: string
    includeAll: boolean
    commodities: Array<{ id: string; name: string }>
  }>
  standaloneCommodities: Array<{ id: string; name: string }>
}>({
  locations: [],
  standaloneAreas: [],
  standaloneCommodities: [],
})

const hasStatistics = computed(() => {
  if (!exportData.value) return false
  return exportData.value.location_count !== undefined ||
         exportData.value.area_count !== undefined ||
         exportData.value.commodity_count !== undefined ||
         exportData.value.image_count !== undefined ||
         exportData.value.invoice_count !== undefined ||
         exportData.value.manual_count !== undefined ||
         exportData.value.binary_data_size !== undefined
})

const backLinkText = computed(() => {
  const from = route.query.from as string
  const fileId = route.query.fileId as string
  if (from && fileId) {
    switch (from) {
      case 'file-list': return 'Back to Files'
      case 'file-edit': return 'Back to File Edit'
      case 'file-view': return 'Back to File'
      default: return 'Back to File'
    }
  }
  return 'Back to Exports'
})

const statusBadgeClass = computed(() => [
  'status-badge',
  'inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium whitespace-nowrap',
  exportData.value ? getExportStatusClasses(exportData.value) : '',
])

const loadExport = async () => {
  try {
    loading.value = true
    error.value = ''
    const exportId = route.params.id as string
    const response = await exportService.getExport(exportId)
    if (response.data && response.data.data) {
      exportData.value = {
        id: response.data.data.id,
        ...response.data.data.attributes,
      }
      if (exportData.value?.selected_items && exportData.value.selected_items.length > 0) {
        await loadSelectedItemsDetails(exportData.value.selected_items)
      }
      await loadRestoreOperations()
    }
  } catch (err: any) {
    error.value = err.response?.data?.errors?.[0]?.detail || 'Failed to load export'
    console.error('Error loading export:', err)
    throw err
  } finally {
    loading.value = false
  }
}

const loadRestoreOperations = async () => {
  try {
    if (!exportData.value?.id) return
    const response = await exportService.getRestoreOperations(exportData.value.id)
    if (response.data && response.data.data) {
      restoreOperations.value = response.data.data.map((item: any) => ({
        id: item.id,
        ...item.attributes,
      }))
    } else {
      restoreOperations.value = []
    }
  } catch (err: any) {
    console.error('Error loading restore operations:', err)
    restoreOperations.value = []
  }
}

const loadSelectedItemsDetails = async (
  items: Array<{ id: string; type: string; name?: string; include_all?: boolean; location_id?: string; area_id?: string }>,
) => {
  try {
    loadingItems.value = true
    selectedItemsDetails.value = []
    hierarchicalItems.value = { locations: [], standaloneAreas: [], standaloneCommodities: [] }

    const locationItems = new Map<string, { name: string; includeAll: boolean; locationId: string | null; areaId: string | null }>()
    const areaItems = new Map<string, { name: string; includeAll: boolean; locationId: string | null; areaId: string | null }>()
    const commodityItems = new Map<string, { name: string; includeAll: boolean; locationId: string | null; areaId: string | null }>()

    items.forEach(item => {
      const itemData = {
        name: item.name || `[Unknown ${item.type} ${item.id}]`,
        includeAll: item.include_all || false,
        locationId: item.location_id || null,
        areaId: item.area_id || null,
      }
      if (item.type === 'location') locationItems.set(item.id, itemData)
      else if (item.type === 'area') areaItems.set(item.id, itemData)
      else if (item.type === 'commodity') commodityItems.set(item.id, itemData)
    })

    const processedAreaIds = new Set<string>()

    for (const [locationId, locationData] of locationItems) {
      const locationAreasData: Array<{ id: string; name: string; includeAll: boolean; commodities: Array<{ id: string; name: string }> }> = []
      if (!locationData.includeAll) {
        for (const [areaId, areaData] of areaItems) {
          if (areaData.locationId === locationId) {
            processedAreaIds.add(areaId)
            const areaCommoditiesData: Array<{ id: string; name: string }> = []
            if (!areaData.includeAll) {
              for (const [commodityId, commodityData] of commodityItems) {
                if (commodityData.areaId === areaId) {
                  areaCommoditiesData.push({ id: commodityId, name: commodityData.name })
                }
              }
            }
            locationAreasData.push({ id: areaId, name: areaData.name, includeAll: areaData.includeAll, commodities: areaCommoditiesData })
          }
        }
      }
      hierarchicalItems.value.locations.push({
        id: locationId,
        name: locationData.name,
        includeAll: locationData.includeAll,
        areas: locationAreasData,
      })
    }

    for (const [areaId, areaData] of areaItems) {
      if (processedAreaIds.has(areaId)) continue
      const parentLocationSelected = areaData.locationId && locationItems.has(areaData.locationId)
      if (parentLocationSelected) continue
      const areaCommoditiesData: Array<{ id: string; name: string }> = []
      if (!areaData.includeAll) {
        for (const [commodityId, commodityData] of commodityItems) {
          if (commodityData.areaId === areaId) {
            areaCommoditiesData.push({ id: commodityId, name: commodityData.name })
          }
        }
      }
      hierarchicalItems.value.standaloneAreas.push({
        id: areaId,
        name: areaData.name,
        includeAll: areaData.includeAll,
        commodities: areaCommoditiesData,
      })
    }

    for (const [commodityId, commodityData] of commodityItems) {
      const parentAreaSelected = commodityData.areaId && areaItems.has(commodityData.areaId)
      if (parentAreaSelected) continue
      let parentLocationIncludesAll = false
      if (commodityData.areaId) {
        for (const [, areaData] of areaItems) {
          if (areaData.locationId && locationItems.has(areaData.locationId)) {
            const locationData = locationItems.get(areaData.locationId)
            if (locationData?.includeAll) { parentLocationIncludesAll = true; break }
          }
        }
      }
      if (parentLocationIncludesAll) continue
      hierarchicalItems.value.standaloneCommodities.push({ id: commodityId, name: commodityData.name })
    }
  } catch (err: any) {
    console.error('Error loading selected items details:', err)
  } finally {
    loadingItems.value = false
  }
}

const formatExportType = (type: string) => {
  const typeMap = {
    full_database: 'Full Database',
    selected_items: 'Selected Items',
    locations: 'Locations',
    areas: 'Areas',
    commodities: 'Commodities',
  }
  return typeMap[type as keyof typeof typeMap] || type
}

const formatDateTime = (dateString: string) => {
  if (!dateString) return '-'
  try { return new Date(dateString).toLocaleString() } catch { return dateString }
}

const formatFileSize = (bytes: number) => {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const formatRestoreStrategy = (strategy: string) => {
  const strategyMap = {
    merge_add: 'Merge Add',
    merge_update: 'Merge Update',
    full_replace: 'Full Replace',
  }
  return strategyMap[strategy as keyof typeof strategyMap] || strategy
}

const getRestoreStatusClasses = (restore: any) => {
  const status = restore.status || 'pending'
  return `status-${status.replace('_', '-')}`
}

const getRestoreDisplayStatus = (restore: any) => {
  const statusMap = { pending: 'Pending', running: 'Running', completed: 'Completed', failed: 'Failed' }
  return statusMap[restore.status as keyof typeof statusMap] || restore.status
}

const getStepEmoji = (result: string) => {
  const emojiMap: Record<string, string> = {
    todo: '📝', in_progress: '🔄', success: '✅', error: '❌', skipped: '⏭️',
  }
  return emojiMap[result] || '📝'
}

const formatStepResult = (result: string) => {
  const resultMap: Record<string, string> = {
    todo: 'To Do', in_progress: 'In Progress', success: 'Success', error: 'Error', skipped: 'Skipped',
  }
  return resultMap[result] || result
}

const formatDuration = (duration: number) => {
  if (duration < 1000) return `${duration}ms`
  if (duration < 60000) return `${(duration / 1000).toFixed(1)}s`
  return `${(duration / 60000).toFixed(1)}m`
}

const toggleRestoreOperation = (index: number) => {
  expandedRestoreOperations.value[index] = !expandedRestoreOperations.value[index]
}

const navigateToRestore = () => {
  if (exportData.value?.id) {
    router.push(groupStore.groupPath(`/exports/${exportData.value.id}/restore`))
  }
}

const getExportFileUrl = (exportItem: any) => {
  if (!exportItem.file_id) return ''
  return groupStore.groupPath(`/files/${exportItem.file_id}?from=export&exportId=${exportItem.id}`)
}

const goBack = () => {
  const from = route.query.from as string
  const fileId = route.query.fileId as string
  if (from && fileId) {
    switch (from) {
      case 'file-list': router.push(groupStore.groupPath('/files')); break
      case 'file-edit': router.push(groupStore.groupPath(`/files/${fileId}/edit`)); break
      case 'file-view': router.push(groupStore.groupPath(`/files/${fileId}`)); break
      default: router.push(groupStore.groupPath(`/files/${fileId}`)); break
    }
  } else {
    router.push(groupStore.groupPath('/exports'))
  }
}

const retryExport = async () => {
  if (!exportData.value?.id) return
  try {
    retrying.value = true
    const requestData = {
      data: {
        type: 'exports',
        attributes: {
          ...exportData.value,
          status: 'pending',
          error_message: '',
          completed_date: null,
          file_path: '',
        },
      },
    }
    await exportService.updateExport(exportData.value.id, requestData)
    await loadExport()
  } catch (err: any) {
    console.error('Error retrying export:', err)
    alert('Failed to retry export')
  } finally {
    retrying.value = false
  }
}

const confirmDelete = () => { showDeleteDialog.value = true }
const onConfirmDelete = () => { deleteExport(); showDeleteDialog.value = false }

const deleteExport = async () => {
  if (!exportData.value?.id) return
  try {
    deleting.value = true
    await exportService.deleteExport(exportData.value.id)
    router.push(groupStore.groupPath('/exports'))
  } catch (err: any) {
    console.error('Error deleting export:', err)
    alert('Failed to delete export')
  } finally {
    deleting.value = false
  }
}

const downloadExport = async () => {
  if (!exportData.value?.id) return
  try {
    downloading.value = true
    const response = await exportService.downloadExport(exportData.value.id)
    const blob = new Blob([response.data], { type: 'application/xml' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    const contentDisposition = response.headers['content-disposition']
    let filename = 'export.xml'
    if (contentDisposition) {
      const filenameMatch = contentDisposition.match(/filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/)
      if (filenameMatch) filename = filenameMatch[1].replace(/['"]/g, '')
    }
    link.download = filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)
  } catch (err: any) {
    console.error('Error downloading export:', err)
    alert('Failed to download export')
  } finally {
    downloading.value = false
  }
}

let exportInterval: ReturnType<typeof setInterval> | undefined
let restoreInterval: ReturnType<typeof setInterval> | undefined

onMounted(() => {
  loadExport()
  exportInterval = setInterval(() => {
    if (exportData.value?.status === 'pending' || exportData.value?.status === 'in_progress') {
      loadExport().catch(err => {
        console.error('Error refreshing export:', err)
        if (exportInterval) clearInterval(exportInterval)
      })
    } else if (exportInterval) {
      clearInterval(exportInterval)
    }
  }, 5000)
  restoreInterval = setInterval(() => {
    if (exportData.value?.restore_operations) {
      const runningRestores = exportData.value.restore_operations.filter(
        restore => restore.status === 'pending' || restore.status === 'running',
      )
      if (runningRestores.length > 0) {
        loadExport().catch(err => {
          console.error('Error refreshing restore operations:', err)
        })
      }
    }
  }, 3000)
})

onBeforeUnmount(() => {
  if (exportInterval) clearInterval(exportInterval)
  if (restoreInterval) clearInterval(restoreInterval)
})
</script>

<template>
  <PageContainer
    as="div"
    :class="['export-detail-page mx-auto max-w-3xl', { 'opacity-80': exportData && isExportDeleted(exportData) }]"
  >
    <div class="breadcrumb-nav mb-2 text-sm">
      <a
        href="#"
        class="breadcrumb-link inline-flex items-center gap-1 text-primary hover:underline"
        @click.prevent="goBack"
      >
        <ArrowLeft class="size-4" aria-hidden="true" />
        <span>{{ backLinkText }}</span>
      </a>
    </div>

    <PageHeader title="Export Details">
      <template #actions>
        <template v-if="exportData">
          <Button
            v-if="exportData.status === 'completed' && canPerformOperations(exportData)"
            :disabled="downloading"
            @click="downloadExport"
          >
            <Loader2 v-if="downloading" class="size-4 motion-safe:animate-spin" aria-hidden="true" />
            <Download v-else class="size-4" aria-hidden="true" />
            {{ downloading ? 'Downloading...' : 'Download' }}
          </Button>

          <Button
            v-if="exportData.status === 'failed'"
            variant="outline"
            :disabled="retrying"
            @click="retryExport"
          >
            <Loader2 v-if="retrying" class="size-4 motion-safe:animate-spin" aria-hidden="true" />
            <RotateCcw v-else class="size-4" aria-hidden="true" />
            {{ retrying ? 'Retrying...' : 'Retry' }}
          </Button>

          <Button
            v-if="canPerformOperations(exportData)"
            variant="destructive"
            :disabled="deleting"
            @click="confirmDelete"
          >
            <Loader2 v-if="deleting" class="size-4 motion-safe:animate-spin" aria-hidden="true" />
            <Trash2 v-else class="size-4" aria-hidden="true" />
            {{ deleting ? 'Deleting...' : 'Delete' }}
          </Button>

          <div
            v-else-if="isExportDeleted(exportData)"
            class="deleted-status inline-flex items-center gap-1 text-sm text-muted-foreground"
          >
            <Trash2 class="size-4" aria-hidden="true" />
            <span>This export has been deleted</span>
          </div>
        </template>
      </template>
    </PageHeader>

    <div v-if="loading" class="loading mt-4 text-sm text-muted-foreground">Loading export details...</div>
    <Banner v-else-if="error" variant="error" class="mt-4">{{ error }}</Banner>

    <div v-else-if="exportData" class="export-content mt-4 flex flex-col gap-4">
      <div class="export-card rounded-md border bg-card p-6 shadow-sm">
        <div class="card-header mb-4 flex items-center justify-between">
          <h2 class="text-lg font-semibold tracking-tight sm:text-xl">Export Information</h2>
          <span :class="statusBadgeClass">{{ getExportDisplayStatus(exportData) }}</span>
        </div>

        <div class="card-body">
          <div class="info-grid grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div class="info-item">
              <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">Description</label>
              <div class="value mt-1 text-sm">{{ exportData.description || 'No description' }}</div>
            </div>

            <div class="info-item">
              <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">Type</label>
              <div class="value mt-1">
                <span
                  :class="[
                    'type-badge inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium',
                    `type-${exportData.type}`,
                  ]"
                >
                  {{ formatExportType(exportData.type) }}
                </span>
              </div>
            </div>

            <div class="info-item">
              <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">Include File Data</label>
              <div class="value mt-1">
                <span
                  :class="[
                    'bool-badge inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium',
                    exportData.include_file_data
                      ? 'border-success/40 bg-success/10 text-success yes'
                      : 'border-muted-foreground/30 bg-muted text-muted-foreground no',
                  ]"
                >
                  {{ exportData.include_file_data ? 'Yes' : 'No' }}
                </span>
              </div>
            </div>

            <div class="info-item">
              <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">Created</label>
              <div class="value mt-1 text-sm">{{ formatDateTime(exportData.created_date) }}</div>
            </div>

            <div v-if="exportData.completed_date" class="info-item">
              <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">Completed</label>
              <div class="value mt-1 text-sm">{{ formatDateTime(exportData.completed_date) }}</div>
            </div>

            <div v-if="exportData.deleted_at" class="info-item">
              <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">Deleted</label>
              <div class="value deleted-date mt-1 text-sm text-destructive">{{ formatDateTime(exportData.deleted_at) }}</div>
            </div>

            <div v-if="exportData.file_path" class="info-item sm:col-span-2">
              <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">File Location</label>
              <div class="value file-path mt-1 break-all rounded bg-muted/40 p-2 font-mono text-xs">
                {{ exportData.file_path }}
              </div>
            </div>

            <div v-if="exportData.file_size" class="info-item">
              <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">File Size</label>
              <div class="value mt-1 text-sm">{{ formatFileSize(exportData.file_size) }}</div>
            </div>
          </div>
        </div>
      </div>

      <PageSection
        v-if="exportData.file_id && exportData.status === 'completed'"
        title="Export File"
        class="export-card rounded-md border bg-card p-6 shadow-sm"
      >
        <div class="card-body">
          <div class="linked-entity-info">
            <router-link
              :to="getExportFileUrl(exportData)"
              class="entity-badge inline-flex items-center gap-2 rounded-md border bg-muted/40 px-3 py-2 text-sm hover:bg-muted"
              title="View export file"
            >
              <FileDown class="size-4" aria-hidden="true" />
              <span class="entity-text">Export File (xml-1.0)</span>
              <ExternalLink class="entity-link-icon size-3.5 opacity-70" aria-hidden="true" />
            </router-link>
          </div>
        </div>
      </PageSection>

      <PageSection
        v-if="exportData.status === 'completed' && hasStatistics"
        title="Export Statistics"
        class="export-card rounded-md border bg-card p-6 shadow-sm"
      >
        <div class="info-grid grid grid-cols-2 gap-4 sm:grid-cols-3">
          <div v-if="exportData.location_count !== undefined" class="info-item">
            <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">Locations</label>
            <div class="value mt-1 text-sm">{{ exportData.location_count.toLocaleString() }}</div>
          </div>
          <div v-if="exportData.area_count !== undefined" class="info-item">
            <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">Areas</label>
            <div class="value mt-1 text-sm">{{ exportData.area_count.toLocaleString() }}</div>
          </div>
          <div v-if="exportData.commodity_count !== undefined" class="info-item">
            <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">Commodities</label>
            <div class="value mt-1 text-sm">{{ exportData.commodity_count.toLocaleString() }}</div>
          </div>
          <div v-if="exportData.image_count !== undefined" class="info-item">
            <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">Images</label>
            <div class="value mt-1 text-sm">{{ exportData.image_count.toLocaleString() }}</div>
          </div>
          <div v-if="exportData.invoice_count !== undefined" class="info-item">
            <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">Invoices</label>
            <div class="value mt-1 text-sm">{{ exportData.invoice_count.toLocaleString() }}</div>
          </div>
          <div v-if="exportData.manual_count !== undefined" class="info-item">
            <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">Manuals</label>
            <div class="value mt-1 text-sm">{{ exportData.manual_count.toLocaleString() }}</div>
          </div>
          <div v-if="exportData.binary_data_size !== undefined && exportData.binary_data_size > 0" class="info-item">
            <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">Binary Data Size</label>
            <div class="value mt-1 text-sm">{{ formatFileSize(exportData.binary_data_size) }}</div>
          </div>
        </div>
      </PageSection>

      <div
        v-if="exportData.selected_items && exportData.selected_items.length > 0"
        class="export-card rounded-md border bg-card p-6 shadow-sm"
      >
        <div class="card-header mb-4 flex items-center justify-between">
          <h2 class="text-lg font-semibold tracking-tight sm:text-xl">Selected Items</h2>
          <span class="count-badge inline-flex items-center rounded-full border bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
            {{ exportData.selected_items.length }} items
          </span>
        </div>
        <div class="card-body">
          <div v-if="loadingItems" class="loading-items text-sm text-muted-foreground">Loading item details...</div>
          <div v-else class="selected-items-hierarchy flex flex-col gap-2">
            <div
              v-for="location in hierarchicalItems.locations"
              :key="location.id"
              class="hierarchy-item location-item rounded-md border-l-[3px] border-l-blue-500 bg-muted/20 p-3"
            >
              <div class="item-header flex items-center justify-between gap-2">
                <div class="item-info flex items-center gap-2">
                  <span class="item-name font-semibold">{{ location.name }}</span>
                  <span class="item-type rounded bg-muted px-1.5 py-0.5 text-xs uppercase text-muted-foreground">Location</span>
                </div>
                <div v-if="location.includeAll" class="inclusion-badge text-xs italic text-muted-foreground">
                  includes all areas and commodities
                </div>
              </div>
              <div v-if="location.areas.length > 0" class="sub-items mt-3 flex flex-col gap-2 pl-4">
                <div
                  v-for="area in location.areas"
                  :key="area.id"
                  class="hierarchy-item area-item rounded-md border-l-[3px] border-l-orange-500 bg-muted/10 p-2"
                >
                  <div class="item-header flex items-center justify-between gap-2">
                    <div class="item-info flex items-center gap-2">
                      <span class="item-name font-semibold">{{ area.name }}</span>
                      <span class="item-type rounded bg-muted px-1.5 py-0.5 text-xs uppercase text-muted-foreground">Area</span>
                    </div>
                    <div v-if="area.includeAll" class="inclusion-badge text-xs italic text-muted-foreground">
                      includes all commodities
                    </div>
                  </div>
                  <div v-if="area.commodities.length > 0" class="sub-items mt-2 flex flex-col gap-2 pl-4">
                    <div
                      v-for="commodity in area.commodities"
                      :key="commodity.id"
                      class="hierarchy-item commodity-item rounded-md border-l-[3px] border-l-green-500 bg-muted/10 p-2"
                    >
                      <div class="item-header flex items-center gap-2">
                        <span class="item-name font-semibold">{{ commodity.name }}</span>
                        <span class="item-type rounded bg-muted px-1.5 py-0.5 text-xs uppercase text-muted-foreground">Commodity</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div
              v-for="area in hierarchicalItems.standaloneAreas"
              :key="area.id"
              class="hierarchy-item area-item rounded-md border-l-[3px] border-l-orange-500 bg-muted/10 p-2"
            >
              <div class="item-header flex items-center justify-between gap-2">
                <div class="item-info flex items-center gap-2">
                  <span class="item-name font-semibold">{{ area.name }}</span>
                  <span class="item-type rounded bg-muted px-1.5 py-0.5 text-xs uppercase text-muted-foreground">Area</span>
                </div>
                <div v-if="area.includeAll" class="inclusion-badge text-xs italic text-muted-foreground">
                  includes all commodities
                </div>
              </div>
              <div v-if="area.commodities.length > 0" class="sub-items mt-2 flex flex-col gap-2 pl-4">
                <div
                  v-for="commodity in area.commodities"
                  :key="commodity.id"
                  class="hierarchy-item commodity-item rounded-md border-l-[3px] border-l-green-500 bg-muted/10 p-2"
                >
                  <div class="item-header flex items-center gap-2">
                    <span class="item-name font-semibold">{{ commodity.name }}</span>
                    <span class="item-type rounded bg-muted px-1.5 py-0.5 text-xs uppercase text-muted-foreground">Commodity</span>
                  </div>
                </div>
              </div>
            </div>

            <div
              v-for="commodity in hierarchicalItems.standaloneCommodities"
              :key="commodity.id"
              class="hierarchy-item commodity-item rounded-md border-l-[3px] border-l-green-500 bg-muted/10 p-2"
            >
              <div class="item-header flex items-center gap-2">
                <span class="item-name font-semibold">{{ commodity.name }}</span>
                <span class="item-type rounded bg-muted px-1.5 py-0.5 text-xs uppercase text-muted-foreground">Commodity</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="exportData.error_message"
        class="export-card error-card rounded-md border border-destructive/40 bg-destructive/5 p-6 shadow-sm"
      >
        <div class="card-header mb-3">
          <h2 class="text-lg font-semibold tracking-tight text-destructive sm:text-xl">Error Details</h2>
        </div>
        <div class="card-body">
          <div class="error-message whitespace-pre-wrap break-words rounded bg-destructive/10 p-3 text-sm text-destructive">
            {{ exportData.error_message }}
          </div>
        </div>
      </div>

      <div
        v-if="restoreOperations.length > 0"
        class="export-card rounded-md border bg-card p-6 shadow-sm"
      >
        <div class="card-header mb-4 flex items-center justify-between">
          <h2 class="text-lg font-semibold tracking-tight sm:text-xl">Restore Operations</h2>
          <span class="count-badge inline-flex items-center rounded-full border bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
            {{ restoreOperations.length }} operation{{ restoreOperations.length !== 1 ? 's' : '' }}
          </span>
        </div>
        <div class="card-body">
          <div class="restore-operations flex flex-col gap-3">
            <div
              v-for="(restore, index) in restoreOperations"
              :key="restore.id"
              class="restore-operation rounded-md border bg-muted/10"
            >
              <button
                type="button"
                class="restore-header flex w-full items-center justify-between gap-3 px-3 py-3 text-left hover:bg-muted/20"
                @click="toggleRestoreOperation(index)"
              >
                <div class="restore-info min-w-0 flex-1">
                  <div class="restore-description truncate text-sm font-semibold">{{ restore.description }}</div>
                  <div class="restore-meta mt-0.5 flex flex-wrap gap-x-3 gap-y-0.5 text-xs text-muted-foreground">
                    <span class="restore-date">{{ formatDateTime(restore.created_date) }}</span>
                    <span class="restore-strategy">{{ formatRestoreStrategy(restore.options?.strategy) }}</span>
                  </div>
                </div>
                <div class="restore-status flex items-center gap-2">
                  <span
                    :class="[
                      'status-badge inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium',
                      getRestoreStatusClasses(restore),
                    ]"
                  >
                    <Loader2
                      v-if="restore.status === 'running' || restore.status === 'pending'"
                      class="status-icon size-3 motion-safe:animate-spin"
                      aria-hidden="true"
                    />
                    {{ getRestoreDisplayStatus(restore) }}
                  </span>
                  <span
                    :class="[
                      'collapse-toggle inline-flex size-6 items-center justify-center rounded text-muted-foreground hover:bg-muted',
                      { expanded: expandedRestoreOperations[index] },
                    ]"
                  >
                    <ChevronUp v-if="expandedRestoreOperations[index]" class="size-4" aria-hidden="true" />
                    <ChevronDown v-else class="size-4" aria-hidden="true" />
                  </span>
                </div>
              </button>

              <div
                v-if="expandedRestoreOperations[index] && restore.steps && restore.steps.length > 0"
                class="restore-steps border-t px-3 py-3"
              >
                <div class="steps-header mb-2 flex items-center justify-between">
                  <h4 class="text-sm font-semibold">Restore Steps</h4>
                  <span class="steps-count text-xs text-muted-foreground">{{ restore.steps.length }} steps</span>
                </div>
                <div class="steps-list flex flex-col gap-2">
                  <div
                    v-for="step in restore.steps"
                    :key="step.id"
                    class="restore-step flex items-start gap-2 rounded border bg-card p-2 text-sm"
                  >
                    <div class="step-icon">
                      <span class="step-emoji text-base leading-none">{{ getStepEmoji(step.result) }}</span>
                    </div>
                    <div class="step-content min-w-0 flex-1">
                      <div class="step-name font-medium">{{ step.name }}</div>
                      <div class="step-details mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs text-muted-foreground">
                        <span v-if="step.duration" class="step-duration">{{ formatDuration(step.duration) }}</span>
                        <span
                          :class="[
                            'step-result inline-flex items-center rounded-full border px-1.5 py-0.5 text-xs font-medium',
                            `result-${step.result}`,
                          ]"
                        >
                          {{ formatStepResult(step.result) }}
                        </span>
                      </div>
                      <div v-if="step.reason" class="step-reason mt-1 text-xs text-muted-foreground">{{ step.reason }}</div>
                    </div>
                  </div>
                </div>
              </div>

              <div
                v-if="expandedRestoreOperations[index] && restore.error_message"
                class="restore-error border-t px-3 py-3"
              >
                <div class="error-message whitespace-pre-wrap break-words rounded bg-destructive/10 p-2 text-sm text-destructive">
                  {{ restore.error_message }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="export-card rounded-md border bg-card p-6 shadow-sm">
        <div class="card-header mb-4">
          <h2 class="text-lg font-semibold tracking-tight sm:text-xl">Actions</h2>
        </div>
        <div class="card-body">
          <div class="actions right-aligned flex flex-wrap items-center justify-end gap-2">
            <Button
              v-if="exportData.status === 'completed' && canPerformOperations(exportData)"
              variant="outline"
              @click="navigateToRestore"
            >
              <Upload class="size-4" aria-hidden="true" />
              Restore from Export
            </Button>

            <Button
              v-if="exportData.status === 'completed'"
              :disabled="downloading"
              @click="downloadExport"
            >
              <Loader2 v-if="downloading" class="size-4 motion-safe:animate-spin" aria-hidden="true" />
              <Download v-else class="size-4" aria-hidden="true" />
              {{ downloading ? 'Downloading...' : 'Download Export' }}
            </Button>

            <Button
              v-if="exportData.status === 'failed'"
              variant="outline"
              :disabled="retrying"
              @click="retryExport"
            >
              <Loader2 v-if="retrying" class="size-4 motion-safe:animate-spin" aria-hidden="true" />
              <TriangleAlert v-else class="size-4" aria-hidden="true" />
              {{ retrying ? 'Retrying...' : 'Retry Export' }}
            </Button>

            <Button
              variant="destructive"
              :disabled="deleting"
              @click="confirmDelete"
            >
              <Loader2 v-if="deleting" class="size-4 motion-safe:animate-spin" aria-hidden="true" />
              <Trash2 v-else class="size-4" aria-hidden="true" />
              {{ deleting ? 'Deleting...' : 'Delete Export' }}
            </Button>
          </div>
        </div>
      </div>
    </div>

    <AppConfirmDialog
      v-model:open="showDeleteDialog"
      title="Confirm Delete"
      message="Are you sure you want to delete this export?"
      confirm-label="Delete"
      cancel-label="Cancel"
      variant="danger"
      @confirm="onConfirmDelete"
    />
  </PageContainer>
</template>

