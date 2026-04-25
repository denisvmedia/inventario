<script setup lang="ts">
/**
 * CommodityDetailView — migrated to the design system in Phase 4 of
 * Epic #1324 (issue #1329).
 *
 * Page chrome (header, info cards, status pill, tags, URLs, error
 * notifications, delete confirmation) is built from `@design/*`
 * patterns and `vee-validate` / `useConfirm` / `useAppToast` helpers.
 * The file panes still embed the legacy `FileViewer` / `FileUploader`
 * components — those will be replaced by a dedicated `MediaGallery` +
 * `FileViewerDialog` pattern in a later commit on the same branch
 * (`design-system/phase-4-detail-form-views`). The legacy
 * `FocusOverlay` reminder was deleted in #1330 PR 5.7 and replaced
 * with a transient `toast.info` nudge.
 *
 * Legacy CSS class anchors (`.commodity-detail`, `.commodity-short-name`,
 * `.commodity-original-price`, `.commodity-serial-number`,
 * `.commodity-extra-serial-numbers`, `.commodity-part-numbers`,
 * `.commodity-tags`, `.commodity-urls`, `.commodity-images`,
 * `.commodity-manuals`, `.commodity-invoices`, …) are preserved as
 * no-op markers so existing Playwright selectors continue to resolve
 * through the strangler-fig window — see
 * devdocs/frontend/migration-conventions.md.
 */
import { computed, onMounted, ref, type FunctionalComponent } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowLeft,
  Box,
  CookingPot,
  Laptop,
  Pencil,
  Printer,
  Shirt,
  Sofa,
  Trash2,
  Wrench,
  type LucideProps,
} from 'lucide-vue-next'

import commodityService from '@/services/commodityService'
import { COMMODITY_TYPES } from '@/constants/commodityTypes'
import { COMMODITY_STATUSES } from '@/constants/commodityStatuses'
import {
  is404Error as checkIs404Error,
  get404Message,
  get404Title,
  getErrorMessage,
} from '@/utils/errorUtils'
import {
  formatPrice,
  calculatePricePerUnit,
  getMainCurrency,
} from '@/services/currencyService'
import { useGroupStore } from '@/stores/groupStore'

import { Button } from '@design/ui/button'
import { Card } from '@design/ui/card'
import Banner from '@design/patterns/Banner.vue'
import CommodityStatusPill, {
  COMMODITY_STATUS_LABELS,
  type CommodityStatus,
} from '@design/patterns/CommodityStatusPill.vue'
import EmptyState from '@design/patterns/EmptyState.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import PageSection from '@design/patterns/PageSection.vue'
import { useAppToast } from '@design/composables/useAppToast'
import { useConfirm } from '@design/composables/useConfirm'

import FileUploader from '@/components/FileUploader.vue'
import FileViewer from '@/components/FileViewer.vue'

type AnyRecord = Record<string, unknown>
type ApiResource = { id: string; attributes: AnyRecord }
type FileUploaderInstance = InstanceType<typeof FileUploader>

const router = useRouter()
const route = useRoute()
const groupStore = useGroupStore()
const toast = useAppToast()
const { confirmDelete } = useConfirm()

const commodity = ref<ApiResource | null>(null)
const loading = ref<boolean>(true)
const lastError = ref<unknown>(null)

const sourceIsArea = computed(() => route.query.source === 'area')
const areaId = computed(() => (route.query.areaId as string) || '')
const is404 = computed(() => !!lastError.value && checkIs404Error(lastError.value as never))

const images = ref<ApiResource[]>([])
const manuals = ref<ApiResource[]>([])
const invoices = ref<ApiResource[]>([])

const imagesSignedUrls = ref<Record<string, unknown>>({})
const manualsSignedUrls = ref<Record<string, unknown>>({})
const invoicesSignedUrls = ref<Record<string, unknown>>({})

const loadingImages = ref<boolean>(false)
const loadingManuals = ref<boolean>(false)
const loadingInvoices = ref<boolean>(false)

const showImageUploader = ref<boolean>(false)
const showManualUploader = ref<boolean>(false)
const showInvoiceUploader = ref<boolean>(false)

// Upload-reminder hooks (#1330 PR 5.7). Replaces the legacy
// FocusOverlay: when the user picks files, a toast nudges them to
// click Upload; clearing the picker or actually uploading dismisses
// it. The toast id is stashed so we dismiss exactly the toast we
// raised — concurrent toasts from elsewhere stay untouched.
const uploadReminderToastId = ref<string | number | null>(null)
const activeUploader = ref<string | null>(null)

const imageUploaderRef = ref<FileUploaderInstance | null>(null)
const manualUploaderRef = ref<FileUploaderInstance | null>(null)
const invoiceUploaderRef = ref<FileUploaderInstance | null>(null)

const typeIcons: Record<string, FunctionalComponent<LucideProps>> = {
  white_goods: CookingPot,
  electronics: Laptop,
  equipment: Wrench,
  furniture: Sofa,
  clothes: Shirt,
  other: Box,
}

const attrs = computed<AnyRecord>(() => (commodity.value?.attributes ?? {}) as AnyRecord)
const typeId = computed(() => (attrs.value.type as string) ?? 'other')
const typeIcon = computed(() => typeIcons[typeId.value] ?? Box)
const typeName = computed(() => {
  const t = COMMODITY_TYPES.find((x) => x.id === typeId.value)
  return t ? t.name : typeId.value
})
const statusName = computed(() => {
  const s = COMMODITY_STATUSES.find((x) => x.id === attrs.value.status)
  return s ? s.name : (attrs.value.status as string) ?? ''
})
const statusPill = computed<CommodityStatus | null>(() => {
  const s = attrs.value.status as string | undefined
  if (!s) return null
  return (s as CommodityStatus) in COMMODITY_STATUS_LABELS ? (s as CommodityStatus) : null
})
const purchaseDateLabel = computed(() => {
  const d = attrs.value.purchase_date as string | undefined
  if (!d) return ''
  return new Date(d).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })
})

const originalPriceLabel = computed(() => {
  const v = parseFloat((attrs.value.original_price as string) ?? '0')
  const cur = (attrs.value.original_price_currency as string) ?? getMainCurrency()
  return formatPrice(v, cur)
})
const showConvertedOriginalPrice = computed(() => {
  const cur = attrs.value.original_price_currency as string | undefined
  const conv = parseFloat((attrs.value.converted_original_price as string) ?? '0')
  return cur && cur !== getMainCurrency() && conv > 0
})
const convertedOriginalPriceLabel = computed(() =>
  formatPrice(parseFloat((attrs.value.converted_original_price as string) ?? '0')),
)
const showCurrentPrice = computed(
  () => parseFloat((attrs.value.current_price as string) ?? '0') > 0,
)
const currentPriceLabel = computed(() =>
  formatPrice(parseFloat((attrs.value.current_price as string) ?? '0')),
)
const count = computed(() => (attrs.value.count as number) ?? 1)
const showPricePerUnit = computed(() => count.value > 1)
const pricePerUnitLabel = computed(() =>
  commodity.value ? formatPrice(calculatePricePerUnit(commodity.value as never)) : '',
)

const extraSerialNumbers = computed(
  () => (attrs.value.extra_serial_numbers as string[] | undefined) ?? [],
)
const partNumbers = computed(() => (attrs.value.part_numbers as string[] | undefined) ?? [])
const tags = computed(() => (attrs.value.tags as string[] | undefined) ?? [])
const urls = computed(() => (attrs.value.urls as string[] | undefined) ?? [])

const showSerialAndPartCard = computed(
  () =>
    !!attrs.value.serial_number ||
    extraSerialNumbers.value.length > 0 ||
    partNumbers.value.length > 0,
)

onMounted(() => {
  loadCommodity()
})

async function loadCommodity() {
  const id = route.params.id as string
  loading.value = true
  lastError.value = null

  try {
    const response = await commodityService.getCommodity(id)
    commodity.value = response.data.data
    loading.value = false
    loadFiles()
  } catch (err) {
    lastError.value = err
    if (!checkIs404Error(err as never)) {
      toast.error(getErrorMessage(err as never, 'commodity', 'Failed to load commodity'))
    }
    loading.value = false
  }
}

async function loadFiles() {
  if (!commodity.value) return
  const id = commodity.value.id

  loadingImages.value = true
  try {
    const response = await commodityService.getImages(id)
    images.value = response.data?.data || []
    imagesSignedUrls.value = response.data?.meta?.signed_urls || {}
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to load images'))
  } finally {
    loadingImages.value = false
  }

  loadingManuals.value = true
  try {
    const response = await commodityService.getManuals(id)
    manuals.value = response.data?.data || []
    manualsSignedUrls.value = response.data?.meta?.signed_urls || {}
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to load manuals'))
  } finally {
    loadingManuals.value = false
  }

  loadingInvoices.value = true
  try {
    const response = await commodityService.getInvoices(id)
    invoices.value = response.data?.data || []
    invoicesSignedUrls.value = response.data?.meta?.signed_urls || {}
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to load invoices'))
  } finally {
    loadingInvoices.value = false
  }
}

async function uploadImages(files: File[]) {
  if (!commodity.value || files.length === 0) return
  closeFocusOverlay()
  try {
    const onProgress = (current: number, total: number, currentFile: string) => {
      imageUploaderRef.value?.updateProgress(current, total, currentFile)
    }
    await commodityService.uploadImages(commodity.value.id, files, onProgress)
    imageUploaderRef.value?.markUploadCompleted()
    showImageUploader.value = false
    loadingImages.value = true
    const response = await commodityService.getImages(commodity.value.id)
    images.value = response.data?.data || []
    imagesSignedUrls.value = response.data?.meta?.signed_urls || {}
    loadingImages.value = false
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to upload images'))
    imageUploaderRef.value?.markUploadFailed()
  }
}

async function uploadManuals(files: File[]) {
  if (!commodity.value || files.length === 0) return
  closeFocusOverlay()
  try {
    const onProgress = (current: number, total: number, currentFile: string) => {
      manualUploaderRef.value?.updateProgress(current, total, currentFile)
    }
    await commodityService.uploadManuals(commodity.value.id, files, onProgress)
    manualUploaderRef.value?.markUploadCompleted()
    showManualUploader.value = false
    loadingManuals.value = true
    const response = await commodityService.getManuals(commodity.value.id)
    manuals.value = response.data?.data || []
    manualsSignedUrls.value = response.data?.meta?.signed_urls || {}
    loadingManuals.value = false
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to upload manuals'))
    manualUploaderRef.value?.markUploadFailed()
  }
}

async function uploadInvoices(files: File[]) {
  if (!commodity.value || files.length === 0) return
  closeFocusOverlay()
  try {
    const onProgress = (current: number, total: number, currentFile: string) => {
      invoiceUploaderRef.value?.updateProgress(current, total, currentFile)
    }
    await commodityService.uploadInvoices(commodity.value.id, files, onProgress)
    invoiceUploaderRef.value?.markUploadCompleted()
    showInvoiceUploader.value = false
    loadingInvoices.value = true
    const response = await commodityService.getInvoices(commodity.value.id)
    invoices.value = response.data?.data || []
    invoicesSignedUrls.value = response.data?.meta?.signed_urls || {}
    loadingInvoices.value = false
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to upload invoices'))
    invoiceUploaderRef.value?.markUploadFailed()
  }
}

function onUploadCapacityFailed(err: unknown) {
  toast.error(getErrorMessage(err as never, 'commodity', 'Upload capacity unavailable'))
}

function onFilesSelected(uploaderType: 'images' | 'manuals' | 'invoices') {
  activeUploader.value = uploaderType
  if (uploadReminderToastId.value !== null) {
    toast.dismiss(uploadReminderToastId.value)
  }
  uploadReminderToastId.value = toast.info(
    "Don't forget to upload your selected files!",
  )
}

function onFilesCleared(uploaderType: 'images' | 'manuals' | 'invoices') {
  if (activeUploader.value === uploaderType) closeFocusOverlay()
}

function closeFocusOverlay() {
  if (uploadReminderToastId.value !== null) {
    toast.dismiss(uploadReminderToastId.value)
    uploadReminderToastId.value = null
  }
  activeUploader.value = null
}

async function deleteImage(image: ApiResource) {
  if (!commodity.value) return
  try {
    await commodityService.deleteImage(commodity.value.id, image.id)
    images.value = images.value.filter((i) => i.id !== image.id)
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to delete image'))
  }
}

async function deleteManual(manual: ApiResource) {
  if (!commodity.value) return
  try {
    await commodityService.deleteManual(commodity.value.id, manual.id)
    manuals.value = manuals.value.filter((m) => m.id !== manual.id)
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to delete manual'))
  }
}

async function deleteInvoice(invoice: ApiResource) {
  if (!commodity.value) return
  try {
    await commodityService.deleteInvoice(commodity.value.id, invoice.id)
    invoices.value = invoices.value.filter((i) => i.id !== invoice.id)
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to delete invoice'))
  }
}

function downloadFile(kind: 'images' | 'manuals' | 'invoices', file: ApiResource) {
  if (!commodity.value) return
  const a = file.attributes as AnyRecord
  const ext = (a.ext as string) ?? ''
  const path = (a.path as string) ?? ''
  const link = document.createElement('a')
  link.href = `/api/v1/commodities/${commodity.value.id}/${kind}/${file.id}${ext}`
  link.download = `${path}${ext}`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}
const downloadImage = (file: ApiResource) => downloadFile('images', file)
const downloadManual = (file: ApiResource) => downloadFile('manuals', file)
const downloadInvoice = (file: ApiResource) => downloadFile('invoices', file)

async function updateImage(data: { id: string; path: string }) {
  if (!commodity.value) return
  try {
    await commodityService.updateImage(commodity.value.id, data.id, { path: data.path })
    const i = images.value.findIndex((x) => x.id === data.id)
    if (i !== -1) (images.value[i].attributes as AnyRecord).path = data.path
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to update image'))
  }
}

async function updateManual(data: { id: string; path: string }) {
  if (!commodity.value) return
  try {
    await commodityService.updateManual(commodity.value.id, data.id, { path: data.path })
    const i = manuals.value.findIndex((x) => x.id === data.id)
    if (i !== -1) (manuals.value[i].attributes as AnyRecord).path = data.path
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to update manual'))
  }
}

async function updateInvoice(data: { id: string; path: string }) {
  if (!commodity.value) return
  try {
    await commodityService.updateInvoice(commodity.value.id, data.id, { path: data.path })
    const i = invoices.value.findIndex((x) => x.id === data.id)
    if (i !== -1) (invoices.value[i].attributes as AnyRecord).path = data.path
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to update invoice'))
  }
}

function navigateBack() {
  if (sourceIsArea.value && areaId.value) {
    router.push(groupStore.groupPath(`/areas/${areaId.value}`))
  } else {
    router.push(groupStore.groupPath('/commodities'))
  }
}

function editCommodity() {
  if (!commodity.value) return
  router.push({
    path: groupStore.groupPath(`/commodities/${commodity.value.id}/edit`),
    query: { source: route.query.source, areaId: route.query.areaId },
  })
}

function printCommodity() {
  if (!commodity.value) return
  window.open(groupStore.groupPath(`/commodities/${commodity.value.id}/print`), '_blank')
}

async function onDeleteCommodity() {
  if (!commodity.value) return
  const confirmed = await confirmDelete('commodity')
  if (!confirmed) return
  try {
    await commodityService.deleteCommodity(commodity.value.id)
    if (sourceIsArea.value && areaId.value) {
      router.push(groupStore.groupPath(`/areas/${areaId.value}`))
    } else {
      router.push(groupStore.groupPath('/commodities'))
    }
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to delete commodity'))
  }
}
</script>


<template>
  <PageContainer as="div" class="commodity-detail">
    <div v-if="loading" class="py-12 text-center text-sm text-muted-foreground">Loading...</div>

    <!-- `resource-not-found` is a strangler-fig anchor preserved for
         `e2e/tests/file-deletion-cascade.spec.ts`, which navigates to a
         deleted commodity URL and waits for `.resource-not-found` to
         confirm the 404 view. -->
    <EmptyState
      v-else-if="is404"
      class="resource-not-found"
      :title="get404Title('commodity')"
      :description="get404Message('commodity')"
    >
      <template #actions>
        <Button variant="outline" @click="navigateBack">
          <ArrowLeft class="size-4" aria-hidden="true" />
          {{ sourceIsArea ? 'Back to Area' : 'Back to Commodities' }}
        </Button>
        <Button @click="loadCommodity">Try Again</Button>
      </template>
    </EmptyState>

    <Banner v-else-if="!commodity" variant="warning">Commodity not found</Banner>

    <template v-else>
      <!-- `header` is a strangler-fig anchor preserved for
           `e2e/tests/includes/user-isolation-auth.ts:393`, which waits
           for `.header` to confirm successful access to a commodity
           detail page (legacy template wrapped the title block in
           `<div class="header">`). -->
      <PageHeader class="header" :title="(commodity.attributes.name as string)">
        <template #breadcrumbs>
          <a
            href="#"
            class="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
            @click.prevent="navigateBack"
          >
            <ArrowLeft class="size-4" aria-hidden="true" />
            {{ sourceIsArea ? 'Back to Area' : 'Back to Commodities' }}
          </a>
        </template>
        <template #actions>
          <Button variant="outline" @click="editCommodity">
            <Pencil class="size-4" aria-hidden="true" />
            Edit
          </Button>
          <Button variant="destructive" @click="onDeleteCommodity">
            <Trash2 class="size-4" aria-hidden="true" />
            Delete
          </Button>
          <Button @click="printCommodity">
            <Printer class="size-4" aria-hidden="true" />
            Print
          </Button>
        </template>
      </PageHeader>

      <div class="commodity-info grid grid-cols-1 gap-4 md:grid-cols-2">
        <Card class="p-5">
          <h2 class="text-lg font-semibold tracking-tight text-foreground">Basic Information</h2>
          <dl class="mt-3 space-y-2 text-sm">
            <div class="commodity-short-name flex justify-between gap-3">
              <dt class="text-muted-foreground">Short Name:</dt>
              <dd class="text-foreground">{{ attrs.short_name }}</dd>
            </div>
            <div class="commodity-type flex justify-between gap-3">
              <dt class="text-muted-foreground">Type:</dt>
              <dd class="inline-flex items-center gap-1.5 text-foreground">
                <component :is="typeIcon" class="size-4" aria-hidden="true" />
                {{ typeName }}
              </dd>
            </div>
            <div class="commodity-count flex justify-between gap-3">
              <dt class="text-muted-foreground">Count:</dt>
              <dd class="tabular-nums text-foreground">{{ count }}</dd>
            </div>
            <div class="commodity-status flex items-center justify-between gap-3">
              <dt class="text-muted-foreground">Status:</dt>
              <dd>
                <CommodityStatusPill v-if="statusPill" :status="statusPill" />
                <span v-else class="text-foreground">{{ statusName }}</span>
              </dd>
            </div>
            <div v-if="purchaseDateLabel" class="commodity-purchase-date flex justify-between gap-3">
              <dt class="text-muted-foreground">Purchase Date:</dt>
              <dd class="text-foreground">{{ purchaseDateLabel }}</dd>
            </div>
          </dl>
        </Card>

        <Card class="commodity-price-information p-5">
          <h2 class="text-lg font-semibold tracking-tight text-foreground">Price Information</h2>
          <dl class="mt-3 space-y-2 text-sm">
            <div class="commodity-original-price flex justify-between gap-3">
              <dt class="text-muted-foreground">Original Price:</dt>
              <dd class="tabular-nums text-foreground">{{ originalPriceLabel }}</dd>
            </div>
            <div
              v-if="showConvertedOriginalPrice"
              class="commodity-converted-original-price flex justify-between gap-3"
            >
              <dt class="text-muted-foreground">Converted Original Price:</dt>
              <dd class="tabular-nums text-foreground">{{ convertedOriginalPriceLabel }}</dd>
            </div>
            <div v-if="showCurrentPrice" class="commodity-current-price flex justify-between gap-3">
              <dt class="text-muted-foreground">Current Price:</dt>
              <dd class="tabular-nums text-foreground">{{ currentPriceLabel }}</dd>
            </div>
            <div v-if="showPricePerUnit" class="commodity-price-per-unit flex justify-between gap-3">
              <dt class="text-muted-foreground">Price Per Unit:</dt>
              <dd class="tabular-nums text-foreground">{{ pricePerUnitLabel }}</dd>
            </div>
          </dl>
        </Card>
      </div>

      <Card v-if="showSerialAndPartCard" class="commodity-serial-and-part-numbers mt-4 p-5">
        <h2 class="text-lg font-semibold tracking-tight text-foreground">
          Serial Numbers and Part Numbers
        </h2>
        <div v-if="attrs.serial_number" class="commodity-serial-number mt-3 flex justify-between gap-3 text-sm">
          <dt class="text-muted-foreground">Serial Number:</dt>
          <dd class="text-foreground">{{ attrs.serial_number }}</dd>
        </div>
        <div v-if="extraSerialNumbers.length > 0" class="commodity-extra-serial-numbers mt-3">
          <h3 class="text-sm font-medium text-foreground">Extra Serial Numbers</h3>
          <ul class="mt-1 list-inside list-disc text-sm text-muted-foreground">
            <li v-for="(serial, index) in extraSerialNumbers" :key="index">{{ serial }}</li>
          </ul>
        </div>
        <div v-if="partNumbers.length > 0" class="commodity-part-numbers mt-3">
          <h3 class="text-sm font-medium text-foreground">Part Numbers</h3>
          <ul class="mt-1 list-inside list-disc text-sm text-muted-foreground">
            <li v-for="(part, index) in partNumbers" :key="index">{{ part }}</li>
          </ul>
        </div>
      </Card>

      <Card v-if="tags.length > 0" class="commodity-tags mt-4 p-5">
        <h2 class="text-lg font-semibold tracking-tight text-foreground">Tags</h2>
        <div class="tags mt-3 flex flex-wrap gap-1.5">
          <span
            v-for="(tag, index) in tags"
            :key="index"
            class="tag inline-flex items-center rounded-full border border-border bg-muted px-2.5 py-0.5 text-xs text-muted-foreground"
          >
            {{ tag }}
          </span>
        </div>
      </Card>

      <Card v-if="urls.length > 0" class="commodity-urls mt-4 p-5">
        <h2 class="text-lg font-semibold tracking-tight text-foreground">URLs</h2>
        <ul class="mt-3 list-inside list-disc text-sm">
          <li v-for="(url, index) in urls" :key="index">
            <a :href="url" target="_blank" class="text-primary underline-offset-4 hover:underline">{{ url }}</a>
          </li>
        </ul>
      </Card>

      <PageSection title="Images" class="commodity-images mt-6">
        <template #actions>
          <!-- `section-header` and `btn-primary` are strangler-fig anchors
               preserved for the e2e upload helper
               (`e2e/tests/includes/uploads.ts:14`), which selects
               `${selectorBase} .section-header .btn-primary` to open the
               uploader pane. The wrapper sits inside the PageSection
               actions slot so the legacy selector still resolves. -->
          <div class="section-header">
            <Button
              size="sm"
              class="btn-primary"
              :variant="showImageUploader ? 'outline' : 'default'"
              @click="showImageUploader = !showImageUploader"
            >
              {{ showImageUploader ? 'Cancel' : 'Add Images' }}
            </Button>
          </div>
        </template>

        <Transition name="file-uploader" mode="out-in">
          <FileUploader
            v-if="showImageUploader"
            ref="imageUploaderRef"
            :multiple="true"
            accept=".gif,.jpg,.jpeg,.png,.webp,image/gif,image/jpeg,image/png,image/webp"
            upload-prompt="Drag and drop images here"
            upload-hint="Supports GIF, JPG, JPEG, PNG, and WebP image formats"
            operation-name="image_upload"
            :require-slots="true"
            @upload="uploadImages"
            @files-selected="onFilesSelected('images')"
            @files-cleared="onFilesCleared('images')"
            @upload-capacity-failed="onUploadCapacityFailed"
          />
        </Transition>

        <div v-if="loadingImages" class="py-8 text-center text-sm text-muted-foreground">
          Loading images...
        </div>
        <FileViewer
          v-else
          :files="(images as never)"
          :signed-urls="imagesSignedUrls"
          file-type="images"
          :entity-id="commodity.id"
          entity-type="commodities"
          @delete="deleteImage"
          @update="updateImage"
          @download="downloadImage"
        />
      </PageSection>
      <PageSection title="Manuals" class="commodity-manuals mt-6">
        <template #actions>
          <div class="section-header">
            <Button
              size="sm"
              class="btn-primary"
              :variant="showManualUploader ? 'outline' : 'default'"
              @click="showManualUploader = !showManualUploader"
            >
              {{ showManualUploader ? 'Cancel' : 'Add Manuals' }}
            </Button>
          </div>
        </template>

        <Transition name="file-uploader" mode="out-in">
          <FileUploader
            v-if="showManualUploader"
            ref="manualUploaderRef"
            :multiple="true"
            accept=".pdf,.gif,.jpg,.jpeg,.png,.webp,application/pdf,image/gif,image/jpeg,image/png,image/webp"
            upload-prompt="Drag and drop manuals here"
            upload-hint="Supports PDF documents and image formats (GIF, JPG, PNG, WebP)"
            @upload="uploadManuals"
            @files-selected="onFilesSelected('manuals')"
            @files-cleared="onFilesCleared('manuals')"
          />
        </Transition>

        <div v-if="loadingManuals" class="py-8 text-center text-sm text-muted-foreground">
          Loading manuals...
        </div>
        <FileViewer
          v-else
          :files="(manuals as never)"
          :signed-urls="manualsSignedUrls"
          file-type="manuals"
          :entity-id="commodity.id"
          entity-type="commodities"
          @delete="deleteManual"
          @update="updateManual"
          @download="downloadManual"
        />
      </PageSection>

      <PageSection title="Invoices" class="commodity-invoices mt-6">
        <template #actions>
          <div class="section-header">
            <Button
              size="sm"
              class="btn-primary"
              :variant="showInvoiceUploader ? 'outline' : 'default'"
              @click="showInvoiceUploader = !showInvoiceUploader"
            >
              {{ showInvoiceUploader ? 'Cancel' : 'Add Invoices' }}
            </Button>
          </div>
        </template>

        <Transition name="file-uploader" mode="out-in">
          <FileUploader
            v-if="showInvoiceUploader"
            ref="invoiceUploaderRef"
            :multiple="true"
            accept=".pdf,.gif,.jpg,.jpeg,.png,.webp,application/pdf,image/gif,image/jpeg,image/png,image/webp"
            upload-prompt="Drag and drop invoices here"
            upload-hint="Supports PDF documents and image formats (GIF, JPG, PNG, WebP)"
            @upload="uploadInvoices"
            @files-selected="onFilesSelected('invoices')"
            @files-cleared="onFilesCleared('invoices')"
          />
        </Transition>

        <div v-if="loadingInvoices" class="py-8 text-center text-sm text-muted-foreground">
          Loading invoices...
        </div>
        <FileViewer
          v-else
          :files="(invoices as never)"
          :signed-urls="invoicesSignedUrls"
          file-type="invoices"
          :entity-id="commodity.id"
          entity-type="commodities"
          @delete="deleteInvoice"
          @update="updateInvoice"
          @download="downloadInvoice"
        />
      </PageSection>

    </template>
  </PageContainer>
</template>

