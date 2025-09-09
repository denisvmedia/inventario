<template>
  <div class="commodity-detail">
    <div v-if="loading" class="loading">Loading...</div>
    <ResourceNotFound
      v-else-if="is404Error"
      resource-type="commodity"
      :title="get404Title('commodity')"
      :message="get404Message('commodity')"
      :go-back-text="sourceIsArea ? 'Back to Area' : 'Back to Commodities'"
      @go-back="navigateBack"
      @try-again="loadCommodity"
    />
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="!commodity" class="not-found">Commodity not found</div>
    <div v-else>
      <div class="breadcrumb-nav">
        <a href="#" class="breadcrumb-link" @click.prevent="navigateBack">
          <font-awesome-icon icon="arrow-left" />
          <span v-if="sourceIsArea">Back to Area</span>
          <span v-else>Back to Commodities</span>
        </a>
      </div>
      <div class="header">
        <h1>{{ commodity.attributes.name }}</h1>
        <div class="actions">
          <button class="btn btn-secondary" @click="editCommodity">Edit</button>
          <button class="btn btn-danger" @click="confirmDelete">Delete</button>
          <button class="btn btn-primary" @click="printCommodity">
            <font-awesome-icon icon="print" /> Print
          </button>
        </div>
      </div>

      <div class="commodity-info">
        <div class="info-card">
          <h2>Basic Information</h2>
          <div class="info-row commodity-short-name">
            <span class="label">Short Name:</span>
            <span>{{ commodity.attributes.short_name }}</span>
          </div>
          <div class="info-row commodity-type">
            <span class="label">Type:</span>
            <span class="type-with-icon">
              <font-awesome-icon :icon="getTypeIcon(commodity.attributes.type)" />
              {{ getTypeName(commodity.attributes.type) }}
            </span>
          </div>
          <div class="info-row commodity-count">
            <span class="label">Count:</span>
            <span>{{ commodity.attributes.count || 1 }}</span>
          </div>
          <div class="info-row commodity-status">
            <span class="label">Status:</span>
            <span class="status" :class="commodity.attributes.status">
              {{ getStatusName(commodity.attributes.status) }}
            </span>
          </div>
          <div class="info-row commodity-purchase-date">
            <span class="label">Purchase Date:</span>
            <span>{{ formatDate(commodity.attributes.purchase_date) }}</span>
          </div>
        </div>

        <div class="info-card commodity-price-information">
          <h2>Price Information</h2>
          <div class="info-row commodity-original-price">
            <span class="label">Original Price:</span>
            <span>{{ formatPrice(parseFloat(commodity.attributes.original_price), commodity.attributes.original_price_currency) }}</span>
          </div>
          <div v-if="(commodity.attributes.original_price_currency !== getMainCurrency()) && parseFloat(commodity.attributes.converted_original_price) > 0" class="info-row commodity-converted-original-price">
            <span class="label">Converted Original Price:</span>
            <span>{{ formatPrice(parseFloat(commodity.attributes.converted_original_price)) }}</span>
          </div>
          <div v-if="parseFloat(commodity.attributes.current_price) > 0" class="info-row commodity-current-price">
            <span class="label">Current Price:</span>
            <span>{{ formatPrice(parseFloat(commodity.attributes.current_price)) }}</span>
          </div>
          <div v-if="(commodity.attributes.count || 1) > 1" class="info-row commodity-price-per-unit">
            <span class="label">Price Per Unit:</span>
            <span>{{ formatPrice(calculatePricePerUnit(commodity)) }}</span>
          </div>
        </div>

        <div v-if="commodity.attributes.serial_number || (commodity.attributes.extra_serial_numbers && commodity.attributes.extra_serial_numbers.length > 0) || (commodity.attributes.part_numbers && commodity.attributes.part_numbers.length > 0)" class="info-card commodity-serial-and-part-numbers">
          <h2>Serial Numbers and Part Numbers</h2>
          <div v-if="commodity.attributes.serial_number" class="info-row commodity-serial-number">
            <span class="label">Serial Number:</span>
            <span>{{ commodity.attributes.serial_number }}</span>
          </div>
          <div v-if="commodity.attributes.extra_serial_numbers && commodity.attributes.extra_serial_numbers.length > 0" class="commodity-extra-serial-numbers">
            <h3>Extra Serial Numbers</h3>
            <ul>
              <li v-for="(serial, index) in commodity.attributes.extra_serial_numbers" :key="index">
                {{ serial }}
              </li>
            </ul>
          </div>
          <div v-if="commodity.attributes.part_numbers && commodity.attributes.part_numbers.length > 0" class="commodity-part-numbers">
            <h3>Part Numbers</h3>
            <ul>
              <li v-for="(part, index) in commodity.attributes.part_numbers" :key="index">
                {{ part }}
              </li>
            </ul>
          </div>
        </div>

        <div v-if="commodity.attributes.tags && commodity.attributes.tags.length > 0" class="info-card commodity-tags">
          <h2>Tags</h2>
          <div class="tags">
            <span v-for="(tag, index) in commodity.attributes.tags" :key="index" class="tag">
              {{ tag }}
            </span>
          </div>
        </div>

        <div v-if="commodity.attributes.urls && commodity.attributes.urls.length > 0" class="info-card commodity-urls">
          <h2>URLs</h2>
          <ul>
            <li v-for="(url, index) in commodity.attributes.urls" :key="index">
              <a :href="url" target="_blank">{{ url }}</a>
            </li>
          </ul>
        </div>

        <!-- Images Section -->
        <div class="info-card full-width commodity-images">
          <div class="section-header">
            <h2>Images</h2>
            <button
              class="btn btn-sm"
              :class="showImageUploader ? 'btn-secondary-alt' : 'btn-primary'"
              @click="showImageUploader = !showImageUploader"
            >
              {{ showImageUploader ? 'Cancel' : 'Add Images' }}
            </button>
          </div>

          <Transition name="file-uploader" mode="out-in">
            <FileUploader
              v-if="showImageUploader"
              ref="imageUploaderRef"
              :multiple="true"
              accept=".gif,.jpg,.jpeg,.png,.webp,image/gif,image/jpeg,image/png,image/webp"
              upload-prompt="Drag and drop images here"
              upload-hint="Supports GIF, JPG, JPEG, PNG, and WebP image formats"
              @upload="uploadImages"
              @files-selected="onFilesSelected('images')"
              @files-cleared="onFilesCleared('images')"
            />
          </Transition>

          <div v-if="loadingImages" class="loading">Loading images...</div>
          <FileViewer
            v-else
            :files="images"
            :signed-urls="imagesSignedUrls"
            file-type="images"
            :entity-id="commodity.id"
            entity-type="commodities"
            @delete="deleteImage"
            @update="updateImage"
            @download="downloadImage"
          />
        </div>

        <!-- Manuals Section -->
        <div class="info-card full-width commodity-manuals">
          <div class="section-header">
            <h2>Manuals</h2>
            <button
              class="btn btn-sm"
              :class="showManualUploader ? 'btn-secondary-alt' : 'btn-primary'"
              @click="showManualUploader = !showManualUploader"
            >
              {{ showManualUploader ? 'Cancel' : 'Add Manuals' }}
            </button>
          </div>

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

          <div v-if="loadingManuals" class="loading">Loading manuals...</div>
          <FileViewer
            v-else
            :files="manuals"
            :signed-urls="manualsSignedUrls"
            file-type="manuals"
            :entity-id="commodity.id"
            entity-type="commodities"
            @delete="deleteManual"
            @update="updateManual"
            @download="downloadManual"
          />
        </div>

        <!-- Invoices Section -->
        <div class="info-card full-width commodity-invoices">
          <div class="section-header">
            <h2>Invoices</h2>
            <button
              class="btn btn-sm"
              :class="showInvoiceUploader ? 'btn-secondary-alt' : 'btn-primary'"
              @click="showInvoiceUploader = !showInvoiceUploader"
            >
              {{ showInvoiceUploader ? 'Cancel' : 'Add Invoices' }}
            </button>
          </div>

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

          <div v-if="loadingInvoices" class="loading">Loading invoices...</div>
          <FileViewer
            v-else
            :files="invoices"
            :signed-urls="invoicesSignedUrls"
            file-type="invoices"
            :entity-id="commodity.id"
            entity-type="commodities"
            @delete="deleteInvoice"
            @update="updateInvoice"
            @download="downloadInvoice"
          />
        </div>
      </div>

      <!-- Commodity Delete Confirmation Dialog -->
      <Confirmation
        v-model:visible="showDeleteDialog"
        title="Confirm Delete"
        message="Are you sure you want to delete this commodity?"
        confirm-label="Delete"
        cancel-label="Cancel"
        confirm-button-class="danger"
        confirmationIcon="exclamation-triangle"
        @confirm="onConfirmDelete"
      />

      <!-- Focus Overlay for Upload Reminder -->
      <FocusOverlay
        :show="showFocusOverlay"
        :target-element="focusTargetElement"
        message="Don't forget to upload your selected files!"
        @close="closeFocusOverlay"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import commodityService from '@/services/commodityService'
import { COMMODITY_TYPES } from '@/constants/commodityTypes'
import { COMMODITY_STATUSES } from '@/constants/commodityStatuses'
import ResourceNotFound from '@/components/ResourceNotFound.vue'
import { is404Error as checkIs404Error, get404Message, get404Title } from '@/utils/errorUtils'
import { formatPrice, calculatePricePerUnit, getMainCurrency } from '@/services/currencyService'
import FileUploader from '@/components/FileUploader.vue'
import FileViewer from '@/components/FileViewer.vue'
import Confirmation from "@/components/Confirmation.vue"
import FocusOverlay from '@/components/FocusOverlay.vue'

const router = useRouter()
const route = useRoute()
const commodity = ref<any>(null)
const loading = ref<boolean>(true)
const error = ref<string | null>(null)
const lastError = ref<any>(null) // Store the last error object for 404 detection

// Navigation source tracking
const sourceIsArea = computed(() => route.query.source === 'area')
const areaId = computed(() => route.query.areaId as string || '')

// Error state computed properties
const is404Error = computed(() => lastError.value && checkIs404Error(lastError.value))

// File states
const images = ref<any[]>([])
const manuals = ref<any[]>([])
const invoices = ref<any[]>([])

// Signed URLs from API responses
const imagesSignedUrls = ref<Record<string, any>>({})
const manualsSignedUrls = ref<Record<string, any>>({})
const invoicesSignedUrls = ref<Record<string, any>>({})

// Loading states for files
const loadingImages = ref<boolean>(false)
const loadingManuals = ref<boolean>(false)
const loadingInvoices = ref<boolean>(false)

// Toggle states for file uploaders
const showImageUploader = ref<boolean>(false)
const showManualUploader = ref<boolean>(false)
const showInvoiceUploader = ref<boolean>(false)

// Focus overlay state
const showFocusOverlay = ref<boolean>(false)
const focusTargetElement = ref<HTMLElement | null>(null)
const activeUploader = ref<string | null>(null)

// File uploader refs
const imageUploaderRef = ref<InstanceType<typeof FileUploader> | null>(null)
const manualUploaderRef = ref<InstanceType<typeof FileUploader> | null>(null)
const invoiceUploaderRef = ref<InstanceType<typeof FileUploader> | null>(null)

const loadCommodity = async () => {
  const id = route.params.id as string
  loading.value = true
  error.value = null
  lastError.value = null

  try {
    const response = await commodityService.getCommodity(id)
    commodity.value = response.data.data
    loading.value = false

    // Load files after commodity is loaded
    loadFiles()
  } catch (err: any) {
    lastError.value = err
    if (checkIs404Error(err)) {
      // 404 errors will be handled by the ResourceNotFound component
      loading.value = false
    } else {
      error.value = 'Failed to load commodity: ' + (err.message || 'Unknown error')
      loading.value = false
    }
  }
}

onMounted(() => {
  loadCommodity()
})

const loadFiles = async () => {
  if (!commodity.value) return

  // Load images
  loadingImages.value = true
  try {
    console.log('CommodityDetailView: Loading images for commodity', commodity.value.id)
    const response = await commodityService.getImages(commodity.value.id)
    images.value = response.data?.data || []
    // Extract signed URLs from the response
    imagesSignedUrls.value = response.data?.meta?.signed_urls || {}
    console.log('CommodityDetailView: Images signed URLs:', imagesSignedUrls.value)
  } catch (err: any) {
    console.error('Failed to load images:', err)
  } finally {
    loadingImages.value = false
  }

  // Load manuals
  loadingManuals.value = true
  try {
    const response = await commodityService.getManuals(commodity.value.id)
    manuals.value = response.data?.data || []
    // Extract signed URLs from the response
    manualsSignedUrls.value = response.data?.meta?.signed_urls || {}
  } catch (err: any) {
    console.error('Failed to load manuals:', err)
  } finally {
    loadingManuals.value = false
  }

  // Load invoices
  loadingInvoices.value = true
  try {
    const response = await commodityService.getInvoices(commodity.value.id)
    invoices.value = response.data?.data || []
    // Extract signed URLs from the response
    invoicesSignedUrls.value = response.data?.meta?.signed_urls || {}
  } catch (err: any) {
    console.error('Failed to load invoices:', err)
  } finally {
    loadingInvoices.value = false
  }
}

const uploadImages = async (files: File[]) => {
  if (!commodity.value || files.length === 0) return

  try {
    await commodityService.uploadImages(commodity.value.id, files)
    showImageUploader.value = false
    closeFocusOverlay()
    // Mark upload as completed in the uploader
    imageUploaderRef.value?.markUploadCompleted()
    // Reload images after upload
    loadingImages.value = true
    const response = await commodityService.getImages(commodity.value.id)
    images.value = response.data?.data || []
    // Extract signed URLs from the response
    imagesSignedUrls.value = response.data?.meta?.signed_urls || {}
    loadingImages.value = false
  } catch (err: any) {
    error.value = 'Failed to upload images: ' + (err.message || 'Unknown error')
    imageUploaderRef.value?.markUploadFailed()
  }
}

const uploadManuals = async (files: File[]) => {
  if (!commodity.value || files.length === 0) return

  try {
    await commodityService.uploadManuals(commodity.value.id, files)
    showManualUploader.value = false
    closeFocusOverlay()
    // Mark upload as completed in the uploader
    manualUploaderRef.value?.markUploadCompleted()
    // Reload manuals after upload
    loadingManuals.value = true
    const response = await commodityService.getManuals(commodity.value.id)
    manuals.value = response.data?.data || []
    // Extract signed URLs from the response
    manualsSignedUrls.value = response.data?.meta?.signed_urls || {}
    loadingManuals.value = false
  } catch (err: any) {
    error.value = 'Failed to upload manuals: ' + (err.message || 'Unknown error')
    manualUploaderRef.value?.markUploadFailed()
  }
}

const uploadInvoices = async (files: File[]) => {
  if (!commodity.value || files.length === 0) return

  try {
    await commodityService.uploadInvoices(commodity.value.id, files)
    showInvoiceUploader.value = false
    closeFocusOverlay()
    // Mark upload as completed in the uploader
    invoiceUploaderRef.value?.markUploadCompleted()
    // Reload invoices after upload
    loadingInvoices.value = true
    const response = await commodityService.getInvoices(commodity.value.id)
    invoices.value = response.data?.data || []
    // Extract signed URLs from the response
    invoicesSignedUrls.value = response.data?.meta?.signed_urls || {}
    loadingInvoices.value = false
  } catch (err: any) {
    error.value = 'Failed to upload invoices: ' + (err.message || 'Unknown error')
    invoiceUploaderRef.value?.markUploadFailed()
  }
}

// Focus overlay event handlers
const onFilesSelected = (uploaderType: string) => {
  activeUploader.value = uploaderType

  // Get the upload button from the appropriate uploader
  let uploaderRef: any = null
  switch (uploaderType) {
    case 'images':
      uploaderRef = imageUploaderRef.value
      break
    case 'manuals':
      uploaderRef = manualUploaderRef.value
      break
    case 'invoices':
      uploaderRef = invoiceUploaderRef.value
      break
  }

  if (uploaderRef) {
    // Wait for next tick to ensure the upload button is rendered
    setTimeout(() => {
      const uploadButton = uploaderRef.getUploadButton()
      if (uploadButton) {
        focusTargetElement.value = uploadButton
        showFocusOverlay.value = true
      }
    }, 100)
  }
}

const onFilesCleared = (uploaderType: string) => {
  if (activeUploader.value === uploaderType) {
    closeFocusOverlay()
  }
}

const closeFocusOverlay = () => {
  showFocusOverlay.value = false
  focusTargetElement.value = null
  activeUploader.value = null
}

const deleteImage = async (image: any) => {
  if (!commodity.value) return

  try {
    await commodityService.deleteImage(commodity.value.id, image.id)
    // Remove the deleted image from the list
    images.value = images.value.filter(img => img.id !== image.id)
  } catch (err: any) {
    error.value = 'Failed to delete image: ' + (err.message || 'Unknown error')
  }
}

const deleteManual = async (manual: any) => {
  if (!commodity.value) return

  try {
    await commodityService.deleteManual(commodity.value.id, manual.id)
    // Remove the deleted manual from the list
    manuals.value = manuals.value.filter(m => m.id !== manual.id)
  } catch (err: any) {
    error.value = 'Failed to delete manual: ' + (err.message || 'Unknown error')
  }
}

const deleteInvoice = async (invoice: any) => {
  if (!commodity.value) return

  try {
    await commodityService.deleteInvoice(commodity.value.id, invoice.id)
    // Remove the deleted invoice from the list
    invoices.value = invoices.value.filter(inv => inv.id !== invoice.id)
  } catch (err: any) {
    error.value = 'Failed to delete invoice: ' + (err.message || 'Unknown error')
  }
}

// Download functions
const downloadImage = (image: any) => {
  if (!commodity.value) return

  // Create a link and trigger download
  const link = document.createElement('a')
  const imageUrl = `/api/v1/commodities/${commodity.value.id}/images/${image.id}${image.ext}`
  link.href = imageUrl
  link.download = image.path + image.ext
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

const downloadManual = (manual: any) => {
  if (!commodity.value) return

  // Create a link and trigger download
  const link = document.createElement('a')
  const manualUrl = `/api/v1/commodities/${commodity.value.id}/manuals/${manual.id}${manual.ext}`
  link.href = manualUrl
  link.download = manual.path + manual.ext
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

const downloadInvoice = (invoice: any) => {
  if (!commodity.value) return

  // Create a link and trigger download
  const link = document.createElement('a')
  const invoiceUrl = `/api/v1/commodities/${commodity.value.id}/invoices/${invoice.id}${invoice.ext}`
  link.href = invoiceUrl
  link.download = invoice.path + invoice.ext
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

// File update functions
const updateImage = async (data: any) => {
  if (!commodity.value) return

  try {
    await commodityService.updateImage(commodity.value.id, data.id, { path: data.path })
    // Update the image in the list
    const index = images.value.findIndex(img => img.id === data.id)
    if (index !== -1) {
      images.value[index].path = data.path
    }
  } catch (err: any) {
    error.value = 'Failed to update image: ' + (err.message || 'Unknown error')
  }
}

const updateManual = async (data: any) => {
  if (!commodity.value) return

  try {
    await commodityService.updateManual(commodity.value.id, data.id, { path: data.path })
    // Update the manual in the list
    const index = manuals.value.findIndex(m => m.id === data.id)
    if (index !== -1) {
      manuals.value[index].path = data.path
    }
  } catch (err: any) {
    error.value = 'Failed to update manual: ' + (err.message || 'Unknown error')
  }
}

const updateInvoice = async (data: any) => {
  if (!commodity.value) return

  try {
    await commodityService.updateInvoice(commodity.value.id, data.id, { path: data.path })
    // Update the invoice in the list
    const index = invoices.value.findIndex(inv => inv.id === data.id)
    if (index !== -1) {
      invoices.value[index].path = data.path
    }
  } catch (err: any) {
    error.value = 'Failed to update invoice: ' + (err.message || 'Unknown error')
  }
}

const editCommodity = () => {
  // Preserve the navigation source when going to edit view
  router.push({
    path: `/commodities/${commodity.value.id}/edit`,
    query: {
      source: route.query.source,
      areaId: route.query.areaId
    }
  })
}

const showDeleteDialog = ref(false)

const confirmDelete = () => {
  showDeleteDialog.value = true
}

const onConfirmDelete = () => {
  deleteCommodity()
  showDeleteDialog.value = false
}

const printCommodity = () => {
  // Open the print view in a new tab/window
  window.open(`/commodities/${commodity.value.id}/print`, '_blank')
}

const deleteCommodity = async () => {
  try {
    await commodityService.deleteCommodity(commodity.value.id)
    // Navigate based on the source
    if (sourceIsArea.value && areaId.value) {
      router.push(`/areas/${areaId.value}`)
    } else {
      router.push('/commodities')
    }
  } catch (err: any) {
    error.value = 'Failed to delete commodity: ' + (err.message || 'Unknown error')
  }
}

const navigateBack = () => {
  if (sourceIsArea.value && areaId.value) {
    // Navigate back to the area detail view
    router.push(`/areas/${areaId.value}`)
  } else {
    // Navigate back to the commodities list
    router.push('/commodities')
  }
}

const getTypeIcon = (typeId: string): string => {
  switch(typeId) {
    case 'white_goods':
      return 'blender'
    case 'electronics':
      return 'laptop'
    case 'equipment':
      return 'tools'
    case 'furniture':
      return 'couch'
    case 'clothes':
      return 'tshirt'
    case 'other':
      return 'box'
    default:
      return 'box'
  }
}

const getTypeName = (typeId: string): string => {
  const type = COMMODITY_TYPES.find(t => t.id === typeId)
  return type ? type.name : typeId
}

const getStatusName = (statusId: string): string => {
  const status = COMMODITY_STATUSES.find(s => s.id === statusId)
  return status ? status.name : statusId
}

const formatDate = (date: string): string => {
  const options: Intl.DateTimeFormatOptions = { year: 'numeric', month: 'long', day: 'numeric' }
  return new Date(date).toLocaleDateString('en-US', options)
}
</script>

<style lang="scss" scoped>
@use '@/assets/main.scss' as *;

.commodity-detail {
  max-width: $container-max-width;
  margin: 0 auto;
  padding: 20px;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2rem;
}

.actions {
  display: flex;
  gap: 0.5rem;
}

.loading, .error, .not-found {
  text-align: center;
  padding: 2rem;
  background: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
}

.error {
  color: $danger-color;
}

.commodity-info {
  display: grid;
  grid-template-columns: 1fr;
  gap: 1.5rem;

  @media (width >= 768px) {
    grid-template-columns: 1fr 1fr;
  }
}

.info-card {
  background: white;
  border-radius: $default-radius;
  padding: 1.5rem;
  box-shadow: $box-shadow;

  h2 {
    margin-bottom: 1rem;
    padding-bottom: 0.5rem;
    border-bottom: 1px solid #eee;
  }
}

.info-row {
  display: flex;
  margin-bottom: 0.75rem;
  align-items: center;
}

.label {
  font-weight: 500;
  width: 120px;
  color: $text-color;
}

.status {
  font-weight: 500;
  padding: 0.25rem 0.5rem;
  border-radius: $default-radius;

  &.in_use {
    background-color: #d4edda;
    color: #155724;
  }

  &.sold {
    background-color: #cce5ff;
    color: #004085;
  }

  &.lost {
    background-color: #fff3cd;
    color: #856404;
  }

  &.disposed {
    background-color: #f8d7da;
    color: #721c24;
  }

  &.written_off {
    background-color: #e2e3e5;
    color: #383d41;
  }
}

.type-with-icon {
  display: flex;
  align-items: center;
  gap: 0.5rem;

  i {
    font-size: 1.2rem;
  }
}

.tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.tag {
  background-color: $light-bg-color;
  color: $text-color;
  padding: 0.25rem 0.5rem;
  border-radius: $default-radius;
  margin-right: 0.5rem;
}

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
  margin-top: 0;
  border-radius: $default-radius;
}

/* New styles for file upload sections */
.full-width {
  grid-column: 1 / -1; /* Make the card span all columns */
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid #eee;

  h2 {
    margin-bottom: 0;
    padding-bottom: 0;
    border-bottom: none;
  }
}

/* File uploader transition animations */
.file-uploader-enter-active,
.file-uploader-leave-active {
  transition: all 0.3s ease;
  transform-origin: top;
}

.file-uploader-enter-from {
  opacity: 0;
  transform: translateY(-20px) scaleY(0.8);
  max-height: 0;
}

.file-uploader-enter-to {
  opacity: 1;
  transform: translateY(0) scaleY(1);
  max-height: 500px; /* Adjust based on your content */
}

.file-uploader-leave-from {
  opacity: 1;
  transform: translateY(0) scaleY(1);
  max-height: 500px;
}

.file-uploader-leave-to {
  opacity: 0;
  transform: translateY(-20px) scaleY(0.8);
  max-height: 0;
}
</style>
